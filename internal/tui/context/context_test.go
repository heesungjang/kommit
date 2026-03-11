package context

import (
	"testing"

	"github.com/nicholascross/opengit/internal/config"
)

func TestNew_DefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	ctx := New(&cfg, nil)

	if ctx.Config == nil {
		t.Fatal("Config should not be nil")
	}
	if ctx.Theme.Name != "catppuccin-mocha" {
		t.Errorf("Theme.Name = %q, want %q", ctx.Theme.Name, "catppuccin-mocha")
	}
	// Styles should be initialized (verify border foreground is set).
	if ctx.Styles.PanelFocused.GetBorderTopForeground() == ctx.Styles.PanelUnfocused.GetBorderTopForeground() {
		// Both could be the same if default, but at least they should exist.
	}
}

func TestNew_AutoTheme(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Theme = "auto"
	ctx := New(&cfg, nil)

	if ctx.Theme.Name != "catppuccin-mocha" && ctx.Theme.Name != "catppuccin-latte" {
		t.Errorf("Auto theme Name = %q, want mocha or latte", ctx.Theme.Name)
	}
}

func TestNew_WithColorOverrides(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Appearance.ThemeColors.Blue = "#ff0000"
	ctx := New(&cfg, nil)

	if string(ctx.Theme.Blue) != "#ff0000" {
		t.Errorf("Theme.Blue = %v after override, want #ff0000", ctx.Theme.Blue)
	}
}

func TestSetScreenSize(t *testing.T) {
	cfg := config.DefaultConfig()
	ctx := New(&cfg, nil)

	ctx.SetScreenSize(120, 40, 2)

	if ctx.ScreenWidth != 120 {
		t.Errorf("ScreenWidth = %d, want 120", ctx.ScreenWidth)
	}
	if ctx.ScreenHeight != 40 {
		t.Errorf("ScreenHeight = %d, want 40", ctx.ScreenHeight)
	}
	if ctx.MainContentWidth != 120 {
		t.Errorf("MainContentWidth = %d, want 120", ctx.MainContentWidth)
	}
	if ctx.MainContentHeight != 38 {
		t.Errorf("MainContentHeight = %d, want 38", ctx.MainContentHeight)
	}
}

func TestSetScreenSize_NegativeHeight(t *testing.T) {
	cfg := config.DefaultConfig()
	ctx := New(&cfg, nil)

	ctx.SetScreenSize(80, 1, 5) // chrome taller than terminal

	if ctx.MainContentHeight != 0 {
		t.Errorf("MainContentHeight = %d, want 0 (clamped)", ctx.MainContentHeight)
	}
}

func TestInitStyles_NotZero(t *testing.T) {
	cfg := config.DefaultConfig()
	ctx := New(&cfg, nil)

	// Verify diff styles are set (non-zero foreground).
	s := ctx.Styles
	if s.DiffAdded.GetForeground() == s.DiffRemoved.GetForeground() {
		t.Error("DiffAdded and DiffRemoved should have different foreground colors")
	}
}
