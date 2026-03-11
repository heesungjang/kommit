package config

import (
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
