package auth

import (
	"testing"
)

func TestHostFromRemoteURL(t *testing.T) {
	tests := []struct {
		name     string
		remote   string
		expected string
	}{
		{
			name:     "HTTPS GitHub",
			remote:   "https://github.com/heesungjang/kommit.git",
			expected: "github.com",
		},
		{
			name:     "SSH GitHub",
			remote:   "git@github.com:heesungjang/kommit.git",
			expected: "github.com",
		},
		{
			name:     "HTTPS GitLab",
			remote:   "https://gitlab.com/user/repo.git",
			expected: "gitlab.com",
		},
		{
			name:     "SSH GitLab",
			remote:   "git@gitlab.com:user/repo.git",
			expected: "gitlab.com",
		},
		{
			name:     "Azure DevOps HTTPS",
			remote:   "https://dev.azure.com/org/project/_git/repo",
			expected: "dev.azure.com",
		},
		{
			name:     "Bitbucket HTTPS",
			remote:   "https://bitbucket.org/user/repo.git",
			expected: "bitbucket.org",
		},
		{
			name:     "SSH Bitbucket",
			remote:   "git@bitbucket.org:user/repo.git",
			expected: "bitbucket.org",
		},
		{
			name:     "empty string",
			remote:   "",
			expected: "",
		},
		{
			name:     "HTTPS with port",
			remote:   "https://gitlab.example.com:8443/user/repo.git",
			expected: "gitlab.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HostFromRemoteURL(tt.remote)
			if got != tt.expected {
				t.Errorf("HostFromRemoteURL(%q) = %q, want %q", tt.remote, got, tt.expected)
			}
		})
	}
}

func TestProviderForHost(t *testing.T) {
	tests := []struct {
		host     string
		expected HostProvider
	}{
		{"github.com", ProviderGitHub},
		{"gitlab.com", ProviderGitLab},
		{"gitlab.example.com", ProviderGitLab},
		{"dev.azure.com", ProviderAzureDevOps},
		{"heesungjang.visualstudio.com", ProviderAzureDevOps},
		{"bitbucket.org", ProviderBitbucket},
		{"unknown.example.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := ProviderForHost(tt.host)
			if got != tt.expected {
				t.Errorf("ProviderForHost(%q) = %q, want %q", tt.host, got, tt.expected)
			}
		})
	}
}

func TestGitUserForProvider(t *testing.T) {
	tests := []struct {
		provider HostProvider
		expected string
	}{
		{ProviderGitHub, "x-access-token"},
		{ProviderGitLab, "oauth2"},
		{ProviderAzureDevOps, "kommit"},
		{ProviderBitbucket, "x-token-auth"},
		{"unknown", "oauth2"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			got := GitUserForProvider(tt.provider)
			if got != tt.expected {
				t.Errorf("GitUserForProvider(%q) = %q, want %q", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestDeviceFlowConfigs(t *testing.T) {
	t.Run("CopilotDeviceFlow", func(t *testing.T) {
		cfg := CopilotDeviceFlow()
		if cfg.ClientID != "Iv1.b507a08c87ecfe98" {
			t.Errorf("unexpected client ID: %s", cfg.ClientID)
		}
		if cfg.Scopes != "read:user" {
			t.Errorf("unexpected scopes: %s", cfg.Scopes)
		}
	})

	t.Run("GitHubDeviceFlow", func(t *testing.T) {
		cfg := GitHubDeviceFlow("test-client-id")
		if cfg.ClientID != "test-client-id" {
			t.Errorf("unexpected client ID: %s", cfg.ClientID)
		}
		if cfg.Scopes != "repo read:user user:email" {
			t.Errorf("unexpected scopes: %s", cfg.Scopes)
		}
	})

	t.Run("GitLabDeviceFlow", func(t *testing.T) {
		cfg := GitLabDeviceFlow("https://gitlab.com", "gl-client-id")
		if cfg.DeviceCodeURL != "https://gitlab.com/oauth/authorize_device" {
			t.Errorf("unexpected device code URL: %s", cfg.DeviceCodeURL)
		}
		if cfg.TokenURL != "https://gitlab.com/oauth/token" {
			t.Errorf("unexpected token URL: %s", cfg.TokenURL)
		}
	})

	t.Run("GitLabDeviceFlow trailing slash", func(t *testing.T) {
		cfg := GitLabDeviceFlow("https://gitlab.example.com/", "gl-client-id")
		if cfg.DeviceCodeURL != "https://gitlab.example.com/oauth/authorize_device" {
			t.Errorf("unexpected device code URL: %s", cfg.DeviceCodeURL)
		}
	})
}

func TestAccountSaveLoadRemove(t *testing.T) {
	// Override the path to use a temp dir.
	origEnv := t.TempDir()
	t.Setenv("XDG_DATA_HOME", origEnv)

	// No accounts initially.
	accounts, err := LoadAccounts()
	if err != nil {
		t.Fatalf("LoadAccounts: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("expected empty accounts, got %d", len(accounts))
	}

	// Save an account.
	acct := Account{
		Token:       "gho_test123",
		Username:    "testuser",
		DisplayName: "Test User",
		Provider:    ProviderGitHub,
		GitUser:     "x-access-token",
	}
	if saveErr := SaveAccount("github.com", acct); saveErr != nil {
		t.Fatalf("SaveAccount: %v", saveErr)
	}

	// Load and verify.
	accounts, err = LoadAccounts()
	if err != nil {
		t.Fatalf("LoadAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	got := accounts["github.com"]
	if got.Username != "testuser" {
		t.Errorf("username = %q, want %q", got.Username, "testuser")
	}
	if got.Host != "github.com" {
		t.Errorf("host = %q, want %q", got.Host, "github.com")
	}

	// GetAccount.
	single := GetAccount("github.com")
	if single == nil {
		t.Fatal("GetAccount returned nil")
	}
	if single.Token != "gho_test123" {
		t.Errorf("token = %q, want %q", single.Token, "gho_test123")
	}

	// GetAccount for non-existent host.
	if acct := GetAccount("gitlab.com"); acct != nil {
		t.Error("expected nil for non-existent host")
	}

	// Remove account.
	if removeErr := RemoveAccount("github.com"); removeErr != nil {
		t.Fatalf("RemoveAccount: %v", removeErr)
	}
	accounts, err = LoadAccounts()
	if err != nil {
		t.Fatalf("LoadAccounts after remove: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("expected 0 accounts after remove, got %d", len(accounts))
	}
}

func TestAccountForRemote(t *testing.T) {
	origEnv := t.TempDir()
	t.Setenv("XDG_DATA_HOME", origEnv)

	// Save a GitHub account.
	acct := Account{
		Token:    "gho_test456",
		Username: "remoteuser",
		Provider: ProviderGitHub,
		GitUser:  "x-access-token",
	}
	if err := SaveAccount("github.com", acct); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}

	// Match by HTTPS remote.
	found := AccountForRemote("https://github.com/user/repo.git")
	if found == nil {
		t.Fatal("expected account for HTTPS remote")
	}
	if found.Username != "remoteuser" {
		t.Errorf("username = %q, want %q", found.Username, "remoteuser")
	}

	// Match by SSH remote.
	found = AccountForRemote("git@github.com:user/repo.git")
	if found == nil {
		t.Fatal("expected account for SSH remote")
	}

	// No match for different host.
	found = AccountForRemote("https://gitlab.com/user/repo.git")
	if found != nil {
		t.Error("expected nil for gitlab remote")
	}
}
