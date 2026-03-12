package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// gitHubClientID is the public OAuth App client ID for kommit's GitHub
// integration. This is NOT a secret — device flow (RFC 8628) is designed
// for native/CLI apps where the client ID is public. No client_secret is
// used. Every token still requires explicit user approval in the browser.
//
// Registered at: https://github.com/settings/applications
const gitHubClientID = "Ov23lidHZ1Au9HZfKVpF"

// GitHubUser holds the profile info returned by the GitHub API.
type GitHubUser struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubDeviceLogin returns a DeviceFlowConfig for GitHub account login
// using kommit's registered OAuth App.
func GitHubDeviceLogin() DeviceFlowConfig {
	return GitHubDeviceFlow(gitHubClientID)
}

// FetchGitHubUser fetches the authenticated user's profile from GitHub.
// Works with both OAuth tokens and Personal Access Tokens.
func FetchGitHubUser(ctx context.Context, token string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "kommit/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching user profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, body)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decoding user profile: %w", err)
	}

	// If the primary email isn't public, fetch from the emails endpoint.
	if user.Email == "" {
		email, _ := fetchGitHubPrimaryEmail(ctx, token)
		user.Email = email
	}

	return &user, nil
}

// fetchGitHubPrimaryEmail fetches the user's primary email from the
// /user/emails endpoint (requires user:email scope).
func fetchGitHubPrimaryEmail(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "kommit/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("emails API error (%d)", resp.StatusCode)
	}

	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary {
			return e.Email, nil
		}
	}
	if len(emails) > 0 {
		return emails[0].Email, nil
	}
	return "", nil
}

// LoginGitHub fetches the GitHub profile for the given token (PAT or OAuth)
// and saves the account. This is the high-level helper used after either
// PAT entry or device flow completion.
func LoginGitHub(ctx context.Context, token string) (*Account, error) {
	user, err := FetchGitHubUser(ctx, token)
	if err != nil {
		return nil, err
	}

	acct := Account{
		Token:       token,
		Username:    user.Login,
		DisplayName: user.Name,
		AvatarURL:   user.AvatarURL,
		Email:       user.Email,
		Provider:    ProviderGitHub,
		GitUser:     GitUserForProvider(ProviderGitHub),
		Host:        "github.com",
	}

	if err := SaveAccount("github.com", acct); err != nil {
		return nil, fmt.Errorf("saving account: %w", err)
	}

	return &acct, nil
}
