package styles

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/nicholascross/opengit/internal/tui/theme"
)

// Shared style helpers that reference the active theme.

// PanelBorderWidth is the horizontal overhead added by PanelStyle's border.
// lipgloss Width() includes padding but excludes border, so the outer rendered
// width = Width(w) + PanelBorderWidth.
const PanelBorderWidth = 2

// PanelBorderHeight is the vertical overhead added by PanelStyle's border.
const PanelBorderHeight = 2

// PanelPaddingWidth is the horizontal padding inside PanelStyle (Padding(0, 1) = 1 left + 1 right).
const PanelPaddingWidth = 2

// PanelStyle returns the base style for a panel/pane.
func PanelStyle(focused bool) lipgloss.Style {
	t := theme.Active
	borderColor := t.Surface1
	if focused {
		borderColor = t.Blue
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(t.Base).
		Background(t.Base).
		Padding(0, 1)
}

// TitleStyle returns the style for panel titles.
func TitleStyle(focused bool) lipgloss.Style {
	t := theme.Active
	fg := t.Subtext0
	if focused {
		fg = t.Blue
	}
	return lipgloss.NewStyle().
		Foreground(fg).
		Background(t.Base).
		Bold(true).
		Padding(0, 1)
}

// SelectedStyle returns the style for selected list items.
func SelectedStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Surface1).
		Bold(true)
}

// NormalStyle returns the style for normal list items.
func NormalStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Base)
}

// DimStyle returns the style for dimmed/secondary text.
func DimStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Base)
}

// StatusBarStyle returns the style for the status bar.
func StatusBarStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Subtext1).
		Background(t.Surface0).
		Padding(0, 1)
}

// TabStyle returns the style for tab items.
// No Background set — the parent tab bar container handles it so that
// space separators between tabs also get filled.
func TabStyle(active bool) lipgloss.Style {
	t := theme.Active
	if active {
		return lipgloss.NewStyle().
			Foreground(t.Blue).
			Bold(true).
			Padding(0, 1)
	}
	return lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Padding(0, 1)
}

// KeyHintStyle returns the style for keybinding hints.
func KeyHintStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Subtext0)
}

// KeyStyle returns the style for the key itself in a hint.
func KeyStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Mauve).
		Bold(true)
}

// ErrorStyle returns the style for error messages.
func ErrorStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Red).
		Bold(true)
}

// SuccessStyle returns the style for success messages.
func SuccessStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Green)
}

// WarningStyle returns the style for warning messages.
func WarningStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Yellow)
}

// FileStatusColor returns the color for a file based on its status code.
func FileStatusColor(code byte) lipgloss.Color {
	t := theme.Active
	switch code {
	case 'M':
		return t.StatusModified()
	case 'A':
		return t.StatusAdded()
	case 'D':
		return t.StatusDeleted()
	case 'R':
		return t.StatusRenamed()
	case 'U':
		return t.StatusConflict()
	case '?':
		return t.StatusUntracked()
	default:
		return t.Text
	}
}

// DiffLineStyle returns the style for a diff line.
func DiffLineStyle(lineType byte) lipgloss.Style {
	t := theme.Active
	switch lineType {
	case '+':
		return lipgloss.NewStyle().Foreground(t.DiffAdded()).Background(t.Base)
	case '-':
		return lipgloss.NewStyle().Foreground(t.DiffRemoved()).Background(t.Base)
	case '@':
		return lipgloss.NewStyle().Foreground(t.DiffHunkHeader()).Background(t.Base).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(t.DiffContext()).Background(t.Base)
	}
}
