package auth

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// HostProvider identifies a git hosting provider.
type HostProvider string

const (
	ProviderGitHub      HostProvider = "github"
	ProviderGitLab      HostProvider = "gitlab"
	ProviderAzureDevOps HostProvider = "azure-devops"
	ProviderBitbucket   HostProvider = "bitbucket"
)

// Account holds the saved credentials and profile for a hosting provider.
type Account struct {
	Token       string       `json:"token"`                 // OAuth token or PAT
	Username    string       `json:"username"`              // e.g. "heesungjang"
	DisplayName string       `json:"displayName,omitempty"` // e.g. "Joseph Jang"
	AvatarURL   string       `json:"avatarUrl,omitempty"`
	Email       string       `json:"email,omitempty"`
	Provider    HostProvider `json:"provider"`  // github, gitlab, azure-devops, bitbucket
	GitUser     string       `json:"gitUser"`   // HTTPS username for git credential (e.g. "x-access-token")
	Host        string       `json:"host"`      // e.g. "github.com"
	ExpiresAt   int64        `json:"expiresAt"` // Unix timestamp, 0 = no expiry (PAT)
}

// authFile is the on-disk JSON structure stored at auth.json.
// It holds both AI provider credentials (existing) and hosting accounts (new).
type authFile struct {
	Providers map[string]providerCred `json:"providers"`
	Accounts  map[string]Account      `json:"accounts"` // keyed by host (e.g. "github.com")
}

// providerCred matches the existing ai.providerCred structure so we can
// read/write the same file without breaking existing AI credentials.
type providerCred struct {
	APIKey string `json:"apiKey"`
}

// authFilePath returns the path to auth.json.
func authFilePath() (string, error) {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "kommit", "auth.json"), nil
}

// loadAuthFile reads and parses the auth.json file.
func loadAuthFile() (*authFile, error) {
	empty := func() *authFile {
		return &authFile{
			Providers: make(map[string]providerCred),
			Accounts:  make(map[string]Account),
		}
	}

	path, err := authFilePath()
	if err != nil {
		return empty(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return empty(), nil
		}
		return nil, err
	}

	var af authFile
	if unmarshalErr := json.Unmarshal(data, &af); unmarshalErr != nil {
		return empty(), unmarshalErr
	}
	if af.Providers == nil {
		af.Providers = make(map[string]providerCred)
	}
	if af.Accounts == nil {
		af.Accounts = make(map[string]Account)
	}
	return &af, nil
}

// saveAuthFile writes the auth file to disk with restrictive permissions.
func saveAuthFile(af *authFile) error {
	path, err := authFilePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if mkdirErr := os.MkdirAll(dir, 0o700); mkdirErr != nil {
		return mkdirErr
	}

	data, err := json.MarshalIndent(af, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ---------- Public API ----------------------------------------------------

// LoadAccounts returns all saved hosting accounts, keyed by host.
func LoadAccounts() (map[string]Account, error) {
	af, err := loadAuthFile()
	if err != nil {
		return make(map[string]Account), err
	}
	return af.Accounts, nil
}

// GetAccount returns the account for a specific host, or nil if not found.
func GetAccount(host string) *Account {
	af, err := loadAuthFile()
	if err != nil {
		return nil
	}
	acct, ok := af.Accounts[host]
	if !ok {
		return nil
	}
	return &acct
}

// SaveAccount persists an account for the given host.
func SaveAccount(host string, acct Account) error {
	af, err := loadAuthFile()
	if err != nil {
		af = &authFile{
			Providers: make(map[string]providerCred),
			Accounts:  make(map[string]Account),
		}
	}
	acct.Host = host
	af.Accounts[host] = acct
	return saveAuthFile(af)
}

// RemoveAccount removes the account for a host.
func RemoveAccount(host string) error {
	af, err := loadAuthFile()
	if err != nil {
		return err
	}
	delete(af.Accounts, host)
	return saveAuthFile(af)
}

// AccountForRemote looks up the saved account that matches a remote URL.
// It extracts the host from the URL and returns the matching account, if any.
func AccountForRemote(remoteURL string) *Account {
	host := HostFromRemoteURL(remoteURL)
	if host == "" {
		return nil
	}
	return GetAccount(host)
}

// HostFromRemoteURL extracts the hostname from a git remote URL.
// Supports both HTTPS (https://github.com/user/repo.git) and
// SSH (git@github.com:user/repo.git) formats.
func HostFromRemoteURL(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}

	// SSH format: git@github.com:user/repo.git
	if strings.Contains(remote, "@") && strings.Contains(remote, ":") && !strings.Contains(remote, "://") {
		at := strings.Index(remote, "@")
		colon := strings.Index(remote[at:], ":")
		if colon > 0 {
			return remote[at+1 : at+colon]
		}
	}

	// HTTPS format
	u, err := url.Parse(remote)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// ProviderForHost returns the HostProvider for a known host.
func ProviderForHost(host string) HostProvider {
	h := strings.ToLower(host)
	switch {
	case strings.Contains(h, "github.com"):
		return ProviderGitHub
	case strings.Contains(h, "gitlab.com") || strings.Contains(h, "gitlab"):
		return ProviderGitLab
	case strings.Contains(h, "dev.azure.com") || strings.Contains(h, "visualstudio.com"):
		return ProviderAzureDevOps
	case strings.Contains(h, "bitbucket.org") || strings.Contains(h, "bitbucket"):
		return ProviderBitbucket
	default:
		return ""
	}
}

// GitUserForProvider returns the git HTTPS username for a hosting provider.
func GitUserForProvider(p HostProvider) string {
	switch p {
	case ProviderGitHub:
		return "x-access-token"
	case ProviderGitLab:
		return "oauth2"
	case ProviderAzureDevOps:
		return "kommit" // Azure DevOps accepts any username with a PAT
	case ProviderBitbucket:
		return "x-token-auth"
	default:
		return "oauth2"
	}
}
