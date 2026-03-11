package config

import (
	"os"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Theme != "catppuccin-mocha" {
		t.Errorf("Default theme = %q, want %q", cfg.Theme, "catppuccin-mocha")
	}
	if cfg.Debug {
		t.Error("Default Debug should be false")
	}
	if cfg.AI.Provider != "anthropic" {
		t.Errorf("Default AI.Provider = %q, want %q", cfg.AI.Provider, "anthropic")
	}
	if cfg.Hosting.Provider != "auto" {
		t.Errorf("Default Hosting.Provider = %q, want %q", cfg.Hosting.Provider, "auto")
	}
	if cfg.Appearance.DiffMode != "inline" {
		t.Errorf("Default Appearance.DiffMode = %q, want %q", cfg.Appearance.DiffMode, "inline")
	}
	if !cfg.Appearance.ShowGraph {
		t.Error("Default Appearance.ShowGraph should be true")
	}
	if cfg.Appearance.CompactLog {
		t.Error("Default Appearance.CompactLog should be false")
	}
	if cfg.Keybinds.Custom == nil {
		t.Error("Default Keybinds.Custom should not be nil")
	}
}

func TestDefaultConfig_PanelWidths(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Appearance.SidebarWidth != 0 {
		t.Errorf("Default SidebarWidth = %d, want 0 (auto)", cfg.Appearance.SidebarWidth)
	}
	if cfg.Appearance.SidebarMaxPct != 0 {
		t.Errorf("Default SidebarMaxPct = %d, want 0 (default)", cfg.Appearance.SidebarMaxPct)
	}
	if cfg.Appearance.CenterPct != 0 {
		t.Errorf("Default CenterPct = %d, want 0 (default)", cfg.Appearance.CenterPct)
	}
}

func TestThemeOverrides_ZeroValue(t *testing.T) {
	var o ThemeOverrides
	// All fields should be empty strings (zero value).
	if o.Base != "" || o.Text != "" || o.Blue != "" {
		t.Error("Zero-value ThemeOverrides should have empty strings")
	}
}

// ---------------------------------------------------------------------------
// Save tests
// ---------------------------------------------------------------------------

func TestSave_CreatesFile(t *testing.T) {
	// Override HOME so we don't touch the real config.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := DefaultConfig()
	cfg.Theme = "catppuccin-latte"
	cfg.Appearance.ShowGraph = false
	cfg.Appearance.DiffMode = "side-by-side"

	err := Save(&cfg)
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify the file was created.
	path := tmpDir + "/.config/kommit/config.yaml"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config file not found: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "catppuccin-latte") {
		t.Errorf("config file should contain theme name, got:\n%s", content)
	}
	if !strings.Contains(content, "side-by-side") {
		t.Errorf("config file should contain diff mode, got:\n%s", content)
	}
}

func TestSave_StripsSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := DefaultConfig()
	cfg.AI.APIKey = "sk-supersecret-12345"
	cfg.Hosting.Token = "ghp_secret_token"

	err := Save(&cfg)
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	path := tmpDir + "/.config/kommit/config.yaml"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config file not found: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "supersecret") {
		t.Error("config file should NOT contain API key")
	}
	if strings.Contains(content, "ghp_secret") {
		t.Error("config file should NOT contain hosting token")
	}
}

func TestSave_ShowGraphPersists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := DefaultConfig()
	cfg.Appearance.ShowGraph = false

	err := Save(&cfg)
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	path := tmpDir + "/.config/kommit/config.yaml"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config file not found: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "showgraph: false") && !strings.Contains(content, "showGraph: false") {
		t.Errorf("config file should contain showGraph: false, got:\n%s", content)
	}
}
