package context

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// Styles holds pre-computed lipgloss styles derived from the active theme.
// Styles are computed once via InitStyles and stored on the ProgramContext so
// that View() methods can read them without allocating new styles per frame.
type Styles struct {
	// Panel styles
	PanelFocused   lipgloss.Style
	PanelUnfocused lipgloss.Style

	// Title styles
	TitleFocused   lipgloss.Style
	TitleUnfocused lipgloss.Style

	// List item styles
	Selected lipgloss.Style
	Normal   lipgloss.Style
	Dim      lipgloss.Style

	// Component styles
	StatusBar lipgloss.Style
	TabActive lipgloss.Style
	TabNormal lipgloss.Style

	// Hint styles
	KeyHint lipgloss.Style
	KeyName lipgloss.Style

	// Feedback styles
	Error   lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style

	// Diff styles
	DiffAdded       lipgloss.Style
	DiffRemoved     lipgloss.Style
	DiffContext     lipgloss.Style
	DiffHunkHeader  lipgloss.Style
	DiffCurrentHunk lipgloss.Style
	DiffFileHeader  lipgloss.Style
	DiffMeta        lipgloss.Style
}

// InitStyles pre-computes all styles from the given theme.
func InitStyles(t theme.Theme) Styles {
	var s Styles

	// -- Panels ---
	s.PanelUnfocused = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Surface1).
		BorderBackground(t.Base).
		Background(t.Base).
		Padding(0, 1)
	s.PanelFocused = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Blue).
		BorderBackground(t.Base).
		Background(t.Base).
		Padding(0, 1)

	// -- Titles ---
	s.TitleUnfocused = lipgloss.NewStyle().
		Foreground(t.Subtext0).
		Background(t.Base).
		Bold(true).
		Padding(0, 1)
	s.TitleFocused = lipgloss.NewStyle().
		Foreground(t.Blue).
		Background(t.Base).
		Bold(true).
		Padding(0, 1)

	// -- List items ---
	s.Selected = lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Surface1).
		Bold(true)
	s.Normal = lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Base)
	s.Dim = lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Base)

	// -- Components ---
	s.StatusBar = lipgloss.NewStyle().
		Foreground(t.Subtext1).
		Background(t.Surface0).
		Padding(0, 1)
	s.TabActive = lipgloss.NewStyle().
		Foreground(t.Blue).
		Background(t.Mantle).
		Bold(true).
		Padding(0, 1)
	s.TabNormal = lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Mantle).
		Padding(0, 1)

	// -- Hints ---
	s.KeyHint = lipgloss.NewStyle().Foreground(t.Subtext0)
	s.KeyName = lipgloss.NewStyle().Foreground(t.Mauve).Bold(true)

	// -- Feedback ---
	s.Error = lipgloss.NewStyle().Foreground(t.Red).Background(t.Base).Bold(true)
	s.Success = lipgloss.NewStyle().Foreground(t.Green).Background(t.Base)
	s.Warning = lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base)

	// -- Diff ---
	s.DiffAdded = lipgloss.NewStyle().Foreground(t.DiffAdded()).Background(t.DiffAddedBg())
	s.DiffRemoved = lipgloss.NewStyle().Foreground(t.DiffRemoved()).Background(t.DiffRemovedBg())
	s.DiffContext = lipgloss.NewStyle().Foreground(t.DiffContext()).Background(t.Base)
	s.DiffHunkHeader = lipgloss.NewStyle().Foreground(t.DiffHunkHeader()).Background(t.DiffHunkBg()).Bold(true)
	s.DiffCurrentHunk = lipgloss.NewStyle().Foreground(t.Blue).Background(t.DiffHunkBg()).Bold(true)
	s.DiffFileHeader = lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Bold(true)
	s.DiffMeta = lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base)

	return s
}
