// Package auth provides multi-provider OAuth device flow and credential
// storage for git hosting providers (GitHub, GitLab, Azure DevOps, Bitbucket).
package auth

import (
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

// DeviceFlowConfig holds the OAuth application and endpoint info needed to
// run an RFC 8628 device authorization flow.
type DeviceFlowConfig struct {
	ClientID      string
	Scopes        string
	DeviceCodeURL string // e.g. https://github.com/login/device/code
	TokenURL      string // e.g. https://github.com/login/oauth/access_token
	GrantType     string // default: urn:ietf:params:oauth:grant-type:device_code
}

// DeviceCodeResponse is returned by the device code endpoint.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse holds the access (and optional refresh) token returned after
// the user completes the device authorization flow.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// ErrAuthorizationPending indicates the user hasn't completed the flow yet.
var ErrAuthorizationPending = errors.New("authorization pending")

// ---------- Pre-defined configs -------------------------------------------

// GitHubDeviceFlow returns a DeviceFlowConfig for GitHub OAuth.
// The clientID should be a registered GitHub OAuth App with device flow enabled.
func GitHubDeviceFlow(clientID string) DeviceFlowConfig {
	return DeviceFlowConfig{
		ClientID:      clientID,
		Scopes:        "repo read:user user:email",
		DeviceCodeURL: "https://github.com/login/device/code",
		TokenURL:      "https://github.com/login/oauth/access_token",
	}
}

// CopilotDeviceFlow returns the config used by the existing Copilot AI feature.
func CopilotDeviceFlow() DeviceFlowConfig {
	return DeviceFlowConfig{
		ClientID:      "Iv1.b507a08c87ecfe98",
		Scopes:        "read:user",
		DeviceCodeURL: "https://github.com/login/device/code",
		TokenURL:      "https://github.com/login/oauth/access_token",
	}
}

// GitLabDeviceFlow returns a DeviceFlowConfig for GitLab (>=17.2).
// baseURL should be the GitLab instance URL, e.g. "https://gitlab.com".
func GitLabDeviceFlow(baseURL, clientID string) DeviceFlowConfig {
	base := strings.TrimRight(baseURL, "/")
	return DeviceFlowConfig{
		ClientID:      clientID,
		Scopes:        "api read_user",
		DeviceCodeURL: base + "/oauth/authorize_device",
		TokenURL:      base + "/oauth/token",
		GrantType:     "urn:ietf:params:oauth:grant-type:device_code",
	}
}

// ---------- Device flow steps ---------------------------------------------

// RequestDeviceCode initiates the device authorization flow by requesting a
// device code from the authorization server.
func (c DeviceFlowConfig) RequestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	data := url.Values{
		"client_id": {c.ClientID},
		"scope":     {c.Scopes},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.DeviceCodeURL, strings.NewReader(data.Encode()))
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

// PollForToken polls the token endpoint until the user completes authorization,
// the context is canceled, or the device code expires.
func (c DeviceFlowConfig) PollForToken(ctx context.Context, dc *DeviceCodeResponse) (*TokenResponse, error) {
	interval := time.Duration(dc.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	for {
		if time.Now().After(deadline) {
			return nil, errors.New("device code expired — please try again")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		tok, err := c.tryExchange(ctx, dc.DeviceCode)
		if err != nil {
			if errors.Is(err, ErrAuthorizationPending) {
				continue
			}
			return nil, err
		}
		return tok, nil
	}
}

// tryExchange performs a single token exchange attempt.
func (c DeviceFlowConfig) tryExchange(ctx context.Context, deviceCode string) (*TokenResponse, error) {
	grantType := c.GrantType
	if grantType == "" {
		grantType = "urn:ietf:params:oauth:grant-type:device_code"
	}

	data := url.Values{
		"client_id":   {c.ClientID},
		"device_code": {deviceCode},
		"grant_type":  {grantType},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		TokenResponse
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	switch result.Error {
	case "":
		if result.AccessToken == "" {
			return nil, errors.New("empty access token in response")
		}
		return &result.TokenResponse, nil
	case "authorization_pending":
		return nil, ErrAuthorizationPending
	case "slow_down":
		return nil, ErrAuthorizationPending // treat as pending, interval handles delay
	case "expired_token":
		return nil, errors.New("device code expired — please try again")
	case "access_denied":
		return nil, errors.New("authorization denied by user")
	default:
		return nil, fmt.Errorf("OAuth error: %s", result.Error)
	}
}
