// Package context provides a shared ProgramContext that is passed as a pointer
// to all TUI components. This allows every component to read the latest
// terminal dimensions, theme, and config without message forwarding.
package context

import (
	"github.com/heesungjang/kommit/internal/config"
	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/icons"
	"github.com/heesungjang/kommit/internal/tui/keys"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ProgramContext holds shared state that every TUI component needs. It is
// allocated once and passed by pointer so that updates (e.g. terminal resize)
// are immediately visible to all components without message propagation.
type ProgramContext struct {
	// ScreenWidth and ScreenHeight are the raw terminal dimensions, updated
	// on every tea.WindowSizeMsg.
	ScreenWidth  int
	ScreenHeight int

	// MainContentWidth and MainContentHeight are the dimensions available
	// for the main page view (after subtracting toolbar + status bar).
	MainContentWidth  int
	MainContentHeight int

	// Config is the loaded application configuration.
	Config *config.Config

	// Theme is the active color theme.
	Theme theme.Theme

	// Styles holds pre-computed lipgloss styles derived from the theme.
	Styles Styles

	// Repo is the git repository being displayed.
	Repo *git.Repository

	// ActiveKeyContext tracks the currently active keybinding context.
	// It is updated whenever the user switches pages, panels, or opens a dialog.
	ActiveKeyContext keys.Context

	// Icons holds the active icon set (plain Unicode or Nerd Font glyphs).
	Icons icons.Icons
}

// New creates a ProgramContext with the given config and repository.
// Dimensions start at zero and are set on the first WindowSizeMsg.
// If the config specifies theme color overrides, they are applied on top of
// the base theme before computing styles.
func New(cfg *config.Config, repo *git.Repository) *ProgramContext {
	t := theme.Get(cfg.Theme)

	// Apply user color overrides from config.
	o := cfg.Appearance.ThemeColors
	t.ApplyOverrides(theme.ColorOverrides{
		Base:      o.Base,
		Mantle:    o.Mantle,
		Crust:     o.Crust,
		Surface0:  o.Surface0,
		Surface1:  o.Surface1,
		Surface2:  o.Surface2,
		Overlay0:  o.Overlay0,
		Overlay1:  o.Overlay1,
		Text:      o.Text,
		Subtext0:  o.Subtext0,
		Subtext1:  o.Subtext1,
		Red:       o.Red,
		Green:     o.Green,
		Yellow:    o.Yellow,
		Blue:      o.Blue,
		Mauve:     o.Mauve,
		Pink:      o.Pink,
		Teal:      o.Teal,
		Sky:       o.Sky,
		Peach:     o.Peach,
		Maroon:    o.Maroon,
		Lavender:  o.Lavender,
		Flamingo:  o.Flamingo,
		Rosewater: o.Rosewater,
		Sapphire:  o.Sapphire,
	})

	return &ProgramContext{
		Config:           cfg,
		Theme:            t,
		Styles:           InitStyles(t),
		Repo:             repo,
		ActiveKeyContext: keys.ContextLog,
		Icons:            icons.ForConfig(cfg.Appearance.NerdFonts),
	}
}

// SetScreenSize updates the raw terminal dimensions and recomputes the main
// content area. chromeHeight is the vertical space consumed by the toolbar,
// status bar, and any other fixed chrome.
func (ctx *ProgramContext) SetScreenSize(width, height, chromeHeight int) {
	ctx.ScreenWidth = width
	ctx.ScreenHeight = height
	ctx.MainContentWidth = width
	h := height - chromeHeight
	if h < 0 {
		h = 0
	}
	ctx.MainContentHeight = h
}
