package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// credentials stores API keys per provider, persisted to disk at
// ~/.local/share/kommit/auth.json. This is separate from the main config
// to avoid accidentally committing secrets to version control.
type credentials struct {
	Providers map[string]providerCred `json:"providers"`
}

// providerCred holds the API key for a single provider.
type providerCred struct {
	APIKey string `json:"apiKey"`
}

// credentialsPath returns the path to the auth.json file.
func credentialsPath() (string, error) {
	// Prefer XDG_DATA_HOME, fall back to ~/.local/share.
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

// LoadCredentials reads saved API credentials from disk.
// Returns an empty credentials struct if the file doesn't exist or is invalid.
func LoadCredentials() (*credentials, error) {
	emptyCreds := func() *credentials {
		return &credentials{Providers: make(map[string]providerCred)}
	}

	path, err := credentialsPath()
	if err != nil {
		// Can't determine path (no home dir) — degrade gracefully.
		return emptyCreds(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyCreds(), nil
		}
		return nil, err
	}

	var creds credentials
	if unmarshalErr := json.Unmarshal(data, &creds); unmarshalErr != nil {
		// Corrupt file — return empty creds with the error.
		return emptyCreds(), unmarshalErr
	}
	if creds.Providers == nil {
		creds.Providers = make(map[string]providerCred)
	}
	return &creds, nil
}

// SaveCredentials writes API credentials to disk.
func SaveCredentials(creds *credentials) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if mkdirErr := os.MkdirAll(dir, 0o700); mkdirErr != nil {
		return mkdirErr
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	// Write with restrictive permissions (owner read/write only).
	return os.WriteFile(path, data, 0o600)
}

// GetAPIKey returns the API key for a provider, checking multiple sources
// in priority order:
//  1. Config file / env var (cfg.AI.APIKey, already resolved by config.Load)
//  2. Saved credentials (~/.local/share/kommit/auth.json)
//
// Returns empty string if no key is found anywhere.
func GetAPIKey(provider, cfgKey string) string {
	// Config/env var takes priority.
	if cfgKey != "" {
		return cfgKey
	}

	// Try saved credentials.
	creds, err := LoadCredentials()
	if err != nil {
		return ""
	}
	if pc, ok := creds.Providers[provider]; ok {
		return pc.APIKey
	}
	return ""
}

// SetAPIKey saves an API key for a provider to the credentials file.
func SetAPIKey(provider, apiKey string) error {
	creds, err := LoadCredentials()
	if err != nil {
		creds = &credentials{Providers: make(map[string]providerCred)}
	}
	creds.Providers[provider] = providerCred{APIKey: apiKey}
	return SaveCredentials(creds)
}
