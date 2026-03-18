package hosting

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// --------------------------------------------------------------------------
// GitLab REST API client
// --------------------------------------------------------------------------

const gitlabAPIBase = "https://gitlab.com/api/v4"

// GitLabClient provides access to GitLab's REST API for MR (merge request) operations.
// GitLab calls them "merge requests" but we map them to the common PullRequest type.
type GitLabClient struct {
	token      string
	baseURL    string // defaults to gitlabAPIBase; set for self-hosted instances
	httpClient *http.Client
}

// NewGitLabClient creates a new GitLab API client with the given personal access token.
func NewGitLabClient(token string) *GitLabClient {
	return &GitLabClient{
		token:   token,
		baseURL: gitlabAPIBase,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// NewGitLabClientWithHost creates a GitLab client for a self-hosted instance.
func NewGitLabClientWithHost(token, host string) *GitLabClient {
	c := NewGitLabClient(token)
	if host != "" && host != "gitlab.com" {
		c.baseURL = "https://" + host + "/api/v4"
	}
	return c
}

// projectPath encodes the owner/repo as GitLab's URL-encoded project path.
func projectPath(ref RepoRef) string {
	return url.PathEscape(ref.Owner + "/" + ref.Repo)
}

// ListPullRequests fetches merge requests for the given repository.
func (c *GitLabClient) ListPullRequests(ref RepoRef, state string) ([]PullRequest, error) {
	glState := "opened" // GitLab uses "opened" not "open"
	if state == "closed" {
		glState = "closed"
	} else if state == "merged" {
		glState = "merged"
	} else if state == "all" {
		glState = "all"
	}

	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests?state=%s&per_page=50&order_by=updated_at&sort=desc",
		c.baseURL, projectPath(ref), glState)

	body, err := c.doGet(endpoint)
	if err != nil {
		return nil, fmt.Errorf("listing MRs: %w", err)
	}
	defer body.Close()

	var glMRs []glMergeRequest
	if decErr := json.NewDecoder(body).Decode(&glMRs); decErr != nil {
		return nil, fmt.Errorf("decoding MR list: %w", decErr)
	}

	prs := make([]PullRequest, 0, len(glMRs))
	for _, mr := range glMRs {
		prs = append(prs, mr.toPullRequest())
	}
	return prs, nil
}

// GetPullRequest fetches a single merge request by IID (project-scoped ID).
func (c *GitLabClient) GetPullRequest(ref RepoRef, number int) (*PullRequest, error) {
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%d",
		c.baseURL, projectPath(ref), number)

	body, err := c.doGet(endpoint)
	if err != nil {
		return nil, fmt.Errorf("getting MR !%d: %w", number, err)
	}
	defer body.Close()

	var mr glMergeRequest
	if decErr := json.NewDecoder(body).Decode(&mr); decErr != nil {
		return nil, fmt.Errorf("decoding MR !%d: %w", number, decErr)
	}

	pr := mr.toPullRequest()
	return &pr, nil
}

// CreatePullRequest creates a new merge request.
func (c *GitLabClient) CreatePullRequest(ref RepoRef, req CreatePRRequest) (*PullRequest, error) {
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests", c.baseURL, projectPath(ref))

	payload := glCreateMR{
		Title:        req.Title,
		Description:  req.Body,
		SourceBranch: req.Head,
		TargetBranch: req.Base,
	}
	// GitLab doesn't have a "draft" field on create — use "Draft:" prefix convention.
	if req.Draft {
		payload.Title = "Draft: " + payload.Title
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling create MR request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	c.setHeaders(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("creating MR: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create MR failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var mr glMergeRequest
	if decErr := json.NewDecoder(resp.Body).Decode(&mr); decErr != nil {
		return nil, fmt.Errorf("decoding created MR: %w", decErr)
	}

	pr := mr.toPullRequest()
	return &pr, nil
}

// GetDefaultBranch returns the default branch of the repository.
func (c *GitLabClient) GetDefaultBranch(ref RepoRef) (string, error) {
	endpoint := fmt.Sprintf("%s/projects/%s", c.baseURL, projectPath(ref))

	body, err := c.doGet(endpoint)
	if err != nil {
		return "", fmt.Errorf("getting project info: %w", err)
	}
	defer body.Close()

	var project struct {
		DefaultBranch string `json:"default_branch"`
	}
	if decErr := json.NewDecoder(body).Decode(&project); decErr != nil {
		return "", fmt.Errorf("decoding project info: %w", decErr)
	}

	if project.DefaultBranch == "" {
		return "main", nil
	}
	return project.DefaultBranch, nil
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func (c *GitLabClient) setHeaders(req *http.Request) {
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}
}

func (c *GitLabClient) doGet(endpoint string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return resp.Body, nil
}

// --------------------------------------------------------------------------
// GitLab API response types (internal)
// --------------------------------------------------------------------------

type glMergeRequest struct {
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	State        string `json:"state"` // "opened", "closed", "merged"
	Draft        bool   `json:"draft"`
	WebURL       string `json:"web_url"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	MergeStatus  string `json:"merge_status"` // "can_be_merged", "cannot_be_merged", etc.
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	Author       struct {
		Username string `json:"username"`
	} `json:"author"`
	Labels []string `json:"labels"`
	// GitLab includes changes stat in a separate endpoint — we skip for now.
}

func (mr glMergeRequest) toPullRequest() PullRequest {
	pr := PullRequest{
		Number:  mr.IID,
		Title:   mr.Title,
		Body:    mr.Description,
		Draft:   mr.Draft,
		Author:  mr.Author.Username,
		HeadRef: mr.SourceBranch,
		BaseRef: mr.TargetBranch,
		URL:     mr.WebURL,
		Labels:  mr.Labels,
	}

	// Map GitLab states to our canonical states
	switch mr.State {
	case "opened":
		pr.State = "open"
	case "closed":
		pr.State = "closed"
	case "merged":
		pr.State = "merged"
	default:
		pr.State = mr.State
	}

	pr.Mergeable = mr.MergeStatus == "can_be_merged"

	if t, err := time.Parse(time.RFC3339Nano, mr.CreatedAt); err == nil {
		pr.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, mr.UpdatedAt); err == nil {
		pr.UpdatedAt = t
	}

	return pr
}

type glCreateMR struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
}

// IsGitLabRemote checks if a remote URL points to a GitLab instance.
func IsGitLabRemote(remoteURL string) bool {
	remoteURL = strings.ToLower(strings.TrimSpace(remoteURL))
	return strings.Contains(remoteURL, "gitlab.com") ||
		strings.Contains(remoteURL, "gitlab.")
}
