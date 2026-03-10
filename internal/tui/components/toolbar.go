package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nicholascross/opengit/internal/tui/theme"
)

// ToolbarAction represents a single action button in the toolbar.
type ToolbarAction struct {
	Icon    string // Unicode icon (e.g., "↓", "↑")
	Label   string // Short label (e.g., "Pull", "Push")
	Key     string // Keyboard shortcut (e.g., "P", "p")
	Enabled bool   // Whether the action is currently available
}

// Toolbar renders a horizontal action bar showing available operations
// and their keyboard shortcuts, similar to GitKraken's top toolbar.
type Toolbar struct {
	width   int
	actions []ToolbarAction
}

// NewToolbar creates a new toolbar with default actions.
func NewToolbar() Toolbar {
	return Toolbar{
		width: 80,
		actions: []ToolbarAction{
			{Icon: "↓", Label: "Pull", Key: "P", Enabled: true},
			{Icon: "↑", Label: "Push", Key: "p", Enabled: true},
			{Icon: "⟳", Label: "Fetch", Key: "f", Enabled: true},
			{Icon: "⎇", Label: "Branch", Key: "n", Enabled: true},
			{Icon: "⊞", Label: "Stash", Key: "S", Enabled: true},
			{Icon: "⊟", Label: "Pop", Key: "", Enabled: true},
			{Icon: "⌫", Label: "Discard", Key: "d", Enabled: true},
		},
	}
}

// SetWidth sets the toolbar width.
func (tb Toolbar) SetWidth(w int) Toolbar {
	tb.width = w
	return tb
}

// View renders the toolbar.
func (tb Toolbar) View() string {
	t := theme.Active
	bg := t.Mantle

	iconStyle := lipgloss.NewStyle().
		Foreground(t.Subtext1).
		Background(bg)

	labelStyle := lipgloss.NewStyle().
		Foreground(t.Subtext0).
		Background(bg)

	keyStyle := lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(bg)

	dimIconStyle := lipgloss.NewStyle().
		Foreground(t.Surface2).
		Background(bg)

	dimLabelStyle := lipgloss.NewStyle().
		Foreground(t.Surface2).
		Background(bg)

	dividerStyle := lipgloss.NewStyle().
		Foreground(t.Surface2).
		Background(bg)

	divider := dividerStyle.Render("│")

	var parts []string
	for _, a := range tb.actions {
		var item string
		if a.Enabled {
			item = iconStyle.Render(a.Icon) + labelStyle.Render(" "+a.Label)
			if a.Key != "" {
				item += keyStyle.Render(" [" + a.Key + "]")
			}
		} else {
			item = dimIconStyle.Render(a.Icon) + dimLabelStyle.Render(" "+a.Label)
		}
		parts = append(parts, item)
	}

	content := strings.Join(parts, dividerStyle.Render(" ")+divider+dividerStyle.Render(" "))

	// Build the final line manually to avoid transparency bleed-through.
	// Skip barStyle.Render() and handle padding ourselves.
	bgStyle := lipgloss.NewStyle().Background(bg)
	line := bgStyle.Render(" ") + content + bgStyle.Render(" ")

	bgSeq := hexToBgSeq(string(bg))
	if bgSeq != "" {
		reset := "\x1b[0m"
		line = ansiBgRe.ReplaceAllString(line, "")
		line = strings.ReplaceAll(line, reset, reset+bgSeq)
		w := lipgloss.Width(line)
		if w < tb.width {
			line += strings.Repeat(" ", tb.width-w)
		}
		line = bgSeq + line + reset
	}

	return line
}
