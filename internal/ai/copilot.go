package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHub OAuth device flow constants.
const (
	// copilotClientID is the public OAuth client ID used by Copilot CLI tools.
	copilotClientID     = "Iv1.b507a08c87ecfe98"
	deviceCodeURL       = "https://github.com/login/device/code"
	accessTokenURL      = "https://github.com/login/oauth/access_token"
	copilotTokenURL     = "https://api.github.com/copilot_internal/v2/token"
	copilotAPIBaseURL   = "https://api.githubcopilot.com"
	copilotDefaultModel = "gpt-4o"
)

// DeviceCodeResponse is returned by the GitHub device code endpoint.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// CopilotToken holds the exchanged Copilot bearer token.
type CopilotToken struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// RequestDeviceCode initiates the GitHub OAuth device code flow.
// Returns a DeviceCodeResponse containing the user_code and verification_uri
// that should be shown to the user.
func RequestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	data := url.Values{
		"client_id": {copilotClientID},
		"scope":     {"read:user"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed (%d): %s", resp.StatusCode, body)
	}

	var result DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding device code response: %w", err)
	}
	return &result, nil
}

// ErrAuthorizationPending indicates the user hasn't completed the auth flow yet.
var ErrAuthorizationPending = errors.New("authorization pending")

// PollForGitHubToken polls the GitHub access token endpoint until the user
// completes authorization. It blocks until a token is received, the context
// is canceled, or the device code expires.
func PollForGitHubToken(ctx context.Context, dc *DeviceCodeResponse) (string, error) {
	interval := time.Duration(dc.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	for {
		if time.Now().After(deadline) {
			return "", errors.New("device code expired — please try again")
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		token, err := tryExchangeDeviceCode(ctx, dc.DeviceCode)
		if err != nil {
			if errors.Is(err, ErrAuthorizationPending) {
				continue
			}
			return "", err
		}
		return token, nil
	}
}

func tryExchangeDeviceCode(ctx context.Context, deviceCode string) (string, error) {
	data := url.Values{
		"client_id":   {copilotClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, accessTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}

	switch result.Error {
	case "":
		if result.AccessToken == "" {
			return "", errors.New("empty access token in response")
		}
		return result.AccessToken, nil
	case "authorization_pending":
		return "", ErrAuthorizationPending
	case "slow_down":
		return "", ErrAuthorizationPending // treat as pending, interval handles delay
	case "expired_token":
		return "", errors.New("device code expired — please try again")
	case "access_denied":
		return "", errors.New("authorization denied by user")
	default:
		return "", fmt.Errorf("GitHub OAuth error: %s", result.Error)
	}
}

// ExchangeForCopilotToken exchanges a GitHub OAuth token for a short-lived
// Copilot bearer token that can be used to call the Copilot API.
func ExchangeForCopilotToken(ctx context.Context, githubToken string) (*CopilotToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, copilotTokenURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating copilot token request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+githubToken)
	req.Header.Set("User-Agent", "kommit/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchanging for copilot token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("copilot token exchange failed (%d): %s", resp.StatusCode, body)
	}

	var token CopilotToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decoding copilot token: %w", err)
	}
	return &token, nil
}

// CopilotProvider implements the Provider interface using GitHub Copilot's
// OpenAI-compatible API at api.githubcopilot.com.
type CopilotProvider struct {
	bearerToken string
	model       string
}

// NewCopilotProvider creates a CopilotProvider with the given bearer token.
func NewCopilotProvider(bearerToken, model string) *CopilotProvider {
	if model == "" {
		model = copilotDefaultModel
	}
	return &CopilotProvider{
		bearerToken: bearerToken,
		model:       model,
	}
}

// GenerateCommitMessage implements the Provider interface.
func (c *CopilotProvider) GenerateCommitMessage(ctx context.Context, diff, stat string) (*CommitMessage, error) {
	prompt := buildCommitPrompt(diff, stat)

	reqBody := openAIRequest{
		Model: c.model,
		Messages: []openAIMessage{
			{Role: "system", Content: commitMessageSystemPrompt},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   256,
		Temperature: 0.3,
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	apiURL := copilotAPIBaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	req.Header.Set("Editor-Version", "kommit/1.0")
	req.Header.Set("Editor-Plugin-Version", "kommit/1.0")
	req.Header.Set("Copilot-Integration-Id", "vscode-chat")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("copilot API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading copilot response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copilot API error (%d): %s", resp.StatusCode, respBody)
	}

	var chatResp openAIResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("decoding copilot response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, errors.New("copilot returned no choices")
	}

	return parseCommitMessage(chatResp.Choices[0].Message.Content), nil
}
