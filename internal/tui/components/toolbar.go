package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/heesungjang/kommit/internal/tui/keys"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// HintBar renders a context-sensitive key hint bar that shows the most
// relevant keybindings for the current context. It replaces the old static
// toolbar and adapts to panel focus, diff mode, WIP state, etc.
type HintBar struct {
	width int
	// Extra is an optional contextual message shown on the right side,
	// e.g. "BISECTING: press B for actions" or "Select a commit to compare".
	Extra string
}

// NewHintBar creates a new context-sensitive hint bar.
func NewHintBar() HintBar {
	return HintBar{width: 80}
}

// SetWidth sets the bar width.
func (hb HintBar) SetWidth(w int) HintBar {
	hb.width = w
	return hb
}

// SetExtra sets an optional contextual message.
func (hb HintBar) SetExtra(msg string) HintBar {
	hb.Extra = msg
	return hb
}

// View renders the hint bar using the current keys.ActiveContext.
func (hb HintBar) View() string {
	t := theme.Active
	bg := t.Mantle

	// Render context-sensitive bindings on the left
	leftContent := keys.RenderHelp(keys.ActiveContext, hb.width-4)

	// Build right content: optional extra message
	rightContent := ""
	if hb.Extra != "" {
		rightContent = lipgloss.NewStyle().
			Foreground(t.Yellow).
			Bold(true).
			Render(hb.Extra)
	}

	// Measure widths
	leftWidth := lipgloss.Width(leftContent)
	rightWidth := lipgloss.Width(rightContent)

	// Calculate padding
	padding := hb.width - leftWidth - rightWidth - 1
	if padding < 1 {
		padding = 1
	}

	var content string
	if rightContent != "" {
		content = leftContent + strings.Repeat(" ", padding) + rightContent + " "
	} else {
		content = leftContent
	}

	// Apply background using ANSI patching (same technique as statusbar)
	bgSeq := hexToBgSeq(string(bg))
	if bgSeq != "" {
		reset := "\x1b[0m"
		content = ansiBgRe.ReplaceAllString(content, "")
		content = strings.ReplaceAll(content, reset, reset+bgSeq)
		w := lipgloss.Width(content)
		if w < hb.width {
			content += strings.Repeat(" ", hb.width-w)
		}
		content = bgSeq + content + reset
	}

	return content
}

// hexToBgSeq is defined in statusbar.go (same package).
