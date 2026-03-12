// Package hosting provides API clients for git hosting platforms (GitHub, etc.)
// to list, create, and inspect pull requests and issues.
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
// Data types
// --------------------------------------------------------------------------

// PullRequest represents a pull request from a hosting provider.
type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"` // "open", "closed", "merged"
	Draft     bool      `json:"draft"`
	Author    string    `json:"author"`
	HeadRef   string    `json:"headRef"` // source branch
	BaseRef   string    `json:"baseRef"` // target branch
	URL       string    `json:"url"`     // web URL
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Review / CI summary
	ReviewDecision string   `json:"reviewDecision"` // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, ""
	Labels         []string `json:"labels"`
	Mergeable      bool     `json:"mergeable"`
	Additions      int      `json:"additions"`
	Deletions      int      `json:"deletions"`
}

// CreatePRRequest contains the fields needed to create a new pull request.
type CreatePRRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"` // source branch
	Base  string `json:"base"` // target branch
	Draft bool   `json:"draft"`
}

// RepoRef identifies a repository on a hosting platform.
type RepoRef struct {
	Owner string
	Repo  string
}

// --------------------------------------------------------------------------
// GitHub REST API client
// --------------------------------------------------------------------------

const githubAPIBase = "https://api.github.com"

// GitHubClient provides access to GitHub's REST API for PR operations.
type GitHubClient struct {
	token      string
	httpClient *http.Client
}

// NewGitHubClient creates a new GitHub API client with the given token.
func NewGitHubClient(token string) *GitHubClient {
	return &GitHubClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ListPullRequests fetches open pull requests for the given repository.
func (c *GitHubClient) ListPullRequests(ref RepoRef, state string) ([]PullRequest, error) {
	if state == "" {
		state = "open"
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/pulls?state=%s&per_page=50&sort=updated&direction=desc",
		githubAPIBase, url.PathEscape(ref.Owner), url.PathEscape(ref.Repo), url.QueryEscape(state))

	body, err := c.doGet(endpoint)
	if err != nil {
		return nil, fmt.Errorf("listing PRs: %w", err)
	}
	defer body.Close()

	var ghPRs []ghPullRequest
	if decErr := json.NewDecoder(body).Decode(&ghPRs); decErr != nil {
		return nil, fmt.Errorf("decoding PR list: %w", decErr)
	}

	prs := make([]PullRequest, 0, len(ghPRs))
	for _, gh := range ghPRs {
		prs = append(prs, gh.toPullRequest())
	}
	return prs, nil
}

// GetPullRequest fetches a single pull request by number.
func (c *GitHubClient) GetPullRequest(ref RepoRef, number int) (*PullRequest, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/pulls/%d",
		githubAPIBase, url.PathEscape(ref.Owner), url.PathEscape(ref.Repo), number)

	body, err := c.doGet(endpoint)
	if err != nil {
		return nil, fmt.Errorf("getting PR #%d: %w", number, err)
	}
	defer body.Close()

	var gh ghPullRequest
	if decErr := json.NewDecoder(body).Decode(&gh); decErr != nil {
		return nil, fmt.Errorf("decoding PR #%d: %w", number, decErr)
	}

	pr := gh.toPullRequest()
	return &pr, nil
}

// CreatePullRequest creates a new pull request.
func (c *GitHubClient) CreatePullRequest(ref RepoRef, req CreatePRRequest) (*PullRequest, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/pulls",
		githubAPIBase, url.PathEscape(ref.Owner), url.PathEscape(ref.Repo))

	payload := ghCreatePR(req)
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling create PR request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	c.setHeaders(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("creating PR: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create PR failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var gh ghPullRequest
	if decErr := json.NewDecoder(resp.Body).Decode(&gh); decErr != nil {
		return nil, fmt.Errorf("decoding created PR: %w", decErr)
	}

	pr := gh.toPullRequest()
	return &pr, nil
}

// GetDefaultBranch returns the default branch of the repository.
func (c *GitHubClient) GetDefaultBranch(ref RepoRef) (string, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s",
		githubAPIBase, url.PathEscape(ref.Owner), url.PathEscape(ref.Repo))

	body, err := c.doGet(endpoint)
	if err != nil {
		return "", fmt.Errorf("getting repo info: %w", err)
	}
	defer body.Close()

	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if decErr := json.NewDecoder(body).Decode(&repo); decErr != nil {
		return "", fmt.Errorf("decoding repo info: %w", decErr)
	}

	if repo.DefaultBranch == "" {
		return "main", nil
	}
	return repo.DefaultBranch, nil
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func (c *GitHubClient) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func (c *GitHubClient) doGet(endpoint string) (io.ReadCloser, error) {
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
// GitHub API response types (internal)
// --------------------------------------------------------------------------

type ghPullRequest struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	Draft     bool   `json:"draft"`
	HTMLURL   string `json:"html_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Mergeable *bool  `json:"mergeable"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`

	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (gh ghPullRequest) toPullRequest() PullRequest {
	pr := PullRequest{
		Number:    gh.Number,
		Title:     gh.Title,
		Body:      gh.Body,
		State:     gh.State,
		Draft:     gh.Draft,
		Author:    gh.User.Login,
		HeadRef:   gh.Head.Ref,
		BaseRef:   gh.Base.Ref,
		URL:       gh.HTMLURL,
		Additions: gh.Additions,
		Deletions: gh.Deletions,
	}

	if gh.Mergeable != nil {
		pr.Mergeable = *gh.Mergeable
	}

	labels := make([]string, 0, len(gh.Labels))
	for _, l := range gh.Labels {
		labels = append(labels, l.Name)
	}
	pr.Labels = labels

	if t, err := time.Parse(time.RFC3339, gh.CreatedAt); err == nil {
		pr.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, gh.UpdatedAt); err == nil {
		pr.UpdatedAt = t
	}

	return pr
}

type ghCreatePR struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Draft bool   `json:"draft"`
}

// --------------------------------------------------------------------------
// Remote URL parsing — extract owner/repo from a git remote URL
// --------------------------------------------------------------------------

// RepoRefFromRemoteURL extracts the owner and repo name from a git remote URL.
// Supports HTTPS (https://github.com/owner/repo.git) and
// SSH (git@github.com:owner/repo.git) formats.
func RepoRefFromRemoteURL(remoteURL string) (RepoRef, error) {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return RepoRef{}, fmt.Errorf("empty remote URL")
	}

	var path string

	// SSH format: git@github.com:owner/repo.git
	if strings.Contains(remoteURL, "@") && strings.Contains(remoteURL, ":") && !strings.Contains(remoteURL, "://") {
		idx := strings.Index(remoteURL, ":")
		path = remoteURL[idx+1:]
	} else {
		// HTTPS format
		u, err := url.Parse(remoteURL)
		if err != nil {
			return RepoRef{}, fmt.Errorf("parsing remote URL: %w", err)
		}
		path = strings.TrimPrefix(u.Path, "/")
	}

	// Remove trailing .git
	path = strings.TrimSuffix(path, ".git")

	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return RepoRef{}, fmt.Errorf("cannot extract owner/repo from %q", remoteURL)
	}

	return RepoRef{
		Owner: parts[0],
		Repo:  parts[1],
	}, nil
}

// StatusIcon returns a compact icon for a PR based on its state and draft status.
func (pr PullRequest) StatusIcon() string {
	if pr.Draft {
		return "◌" // draft
	}
	switch pr.State {
	case "open":
		return "●" // open
	case "closed":
		return "✕" // closed (not merged)
	case "merged":
		return "◆" // merged
	default:
		return "○"
	}
}
