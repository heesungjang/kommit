package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/heesungjang/kommit/internal/tui/keys"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// actionButton describes a single button in the action bar.
type actionButton struct {
	Icon     string         // unicode icon, e.g. "↺"
	Label    string         // display label, e.g. "Undo"
	Key      string         // shortcut key, e.g. "z"
	Contexts []keys.Context // contexts where this action is available
}

// actionGroup is a logical group of buttons separated by a group divider.
type actionGroup struct {
	buttons []actionButton
}

// actionGroups defines the toolbar layout: grouped by workflow.
var actionGroups = []actionGroup{
	// History operations
	{buttons: []actionButton{
		{Icon: "↺", Label: "Undo", Key: "z", Contexts: []keys.Context{
			keys.ContextStatus, keys.ContextLog,
		}},
		{Icon: "↻", Label: "Redo", Key: "Z", Contexts: []keys.Context{
			keys.ContextLog,
		}},
	}},
	// Remote operations
	{buttons: []actionButton{
		{Icon: "↓", Label: "Pull", Key: "P", Contexts: []keys.Context{
			keys.ContextStatus, keys.ContextLog, keys.ContextRemotes, keys.ContextDetail,
		}},
		{Icon: "↑", Label: "Push", Key: "p", Contexts: []keys.Context{
			keys.ContextStatus, keys.ContextLog, keys.ContextRemotes, keys.ContextDetail,
		}},
		{Icon: "⟳", Label: "Fetch", Key: "f", Contexts: []keys.Context{
			keys.ContextStatus, keys.ContextLog, keys.ContextRemotes, keys.ContextDetail,
		}},
	}},
	// Branch
	{buttons: []actionButton{
		{Icon: "⑂", Label: "Branch", Key: "n", Contexts: []keys.Context{
			keys.ContextBranches,
		}},
	}},
	// Stash operations — available from WIP panel
	{buttons: []actionButton{
		{Icon: "⊞", Label: "Stash", Key: "W", Contexts: []keys.Context{
			keys.ContextStatus,
		}},
		{Icon: "⊟", Label: "Pop", Key: "X", Contexts: []keys.Context{
			keys.ContextStatus,
		}},
	}},
}

// ActionBar renders a top-level action toolbar with common git operations.
// Buttons are dimmed when not available in the current key context, making
// it clear which actions can be triggered at any given moment.
type ActionBar struct {
	width    int
	context  keys.Context
	ahead    int    // commits ahead of upstream (badge on Push button)
	behind   int    // commits behind upstream (badge on Pull button)
	username string // logged-in username (shown next to branding)
}

// NewActionBar creates a new action bar.
func NewActionBar() ActionBar {
	return ActionBar{width: 80, context: keys.ContextLog}
}

// SetWidth sets the bar width.
func (ab ActionBar) SetWidth(w int) ActionBar {
	ab.width = w
	return ab
}

// SetContext updates the current key context so the bar can dim
// unavailable actions.
func (ab ActionBar) SetContext(ctx keys.Context) ActionBar {
	ab.context = ctx
	return ab
}

// SetAheadBehind updates the ahead/behind counts displayed as badges
// on the Push and Pull buttons respectively.
func (ab ActionBar) SetAheadBehind(ahead, behind int) ActionBar {
	ab.ahead = ahead
	ab.behind = behind
	return ab
}

// SetUsername sets the logged-in username to display in the branding area.
func (ab ActionBar) SetUsername(username string) ActionBar {
	ab.username = username
	return ab
}

// isActive returns true if the button is usable in the current context.
func (ab ActionBar) isActive(btn actionButton) bool {
	for _, c := range btn.Contexts {
		if c == ab.context {
			return true
		}
	}
	return false
}

// View renders the action bar as a single line with icons, grouped buttons,
// context-aware dimming, and right-aligned branding.
func (ab ActionBar) View() string {
	t := theme.Active
	bg := t.Surface0

	// Active styles — foreground only; ANSI patching applies bg uniformly.
	iconActive := lipgloss.NewStyle().Foreground(t.Blue).Bold(true)
	labelActive := lipgloss.NewStyle().Foreground(t.Text)
	keyActive := lipgloss.NewStyle().Foreground(t.Overlay0)

	// Dimmed styles for unavailable actions.
	iconDim := lipgloss.NewStyle().Foreground(t.Surface2)
	labelDim := lipgloss.NewStyle().Foreground(t.Surface2)
	keyDim := lipgloss.NewStyle().Foreground(t.Surface2)

	groupSep := lipgloss.NewStyle().Foreground(t.Surface2).Render("  │  ")
	btnSep := "   "

	// Badge style for ahead/behind counts on Push/Pull buttons.
	badgeStyle := lipgloss.NewStyle().Foreground(t.Peach).Bold(true)
	badgeDim := lipgloss.NewStyle().Foreground(t.Surface2)

	// Build groups.
	groups := make([]string, 0, len(actionGroups))
	for _, g := range actionGroups {
		btns := make([]string, 0, len(g.buttons))
		for _, btn := range g.buttons {
			// Determine if this button gets an ahead/behind badge.
			badge := ""
			if btn.Label == "Pull" && ab.behind > 0 {
				badge = " " + fmt.Sprintf("%d", ab.behind)
			} else if btn.Label == "Push" && ab.ahead > 0 {
				badge = " " + fmt.Sprintf("%d", ab.ahead)
			}

			var b string
			if ab.isActive(btn) {
				b = iconActive.Render(btn.Icon) + " " +
					labelActive.Render(btn.Label) + " " +
					keyActive.Render(btn.Key)
				if badge != "" {
					b += badgeStyle.Render(badge)
				}
			} else {
				b = iconDim.Render(btn.Icon) + " " +
					labelDim.Render(btn.Label) + " " +
					keyDim.Render(btn.Key)
				if badge != "" {
					b += badgeDim.Render(badge)
				}
			}
			btns = append(btns, b)
		}
		groups = append(groups, strings.Join(btns, btnSep))
	}

	left := "  " + strings.Join(groups, groupSep)

	// Right-aligned branding: "@username · kommit" or just "kommit".
	var brand string
	if ab.username != "" {
		userPart := lipgloss.NewStyle().Foreground(t.Green).Bold(true).Render("@" + ab.username)
		sep := lipgloss.NewStyle().Foreground(t.Surface2).Render(" · ")
		namePart := lipgloss.NewStyle().Foreground(t.Mauve).Bold(true).Render("kommit")
		brand = userPart + sep + namePart
	} else {
		brand = lipgloss.NewStyle().Foreground(t.Mauve).Bold(true).Render("kommit")
	}
	rightPad := " "

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(brand) + len(rightPad)
	gap := ab.width - leftW - rightW - 1
	if gap < 2 {
		gap = 2
	}
	content := left + strings.Repeat(" ", gap) + brand + rightPad

	// Force background on every cell using ANSI patching (same as statusbar).
	bgSeq := hexToBgSeq(string(bg))
	if bgSeq != "" {
		reset := "\x1b[0m"
		content = ansiBgRe.ReplaceAllString(content, "")
		content = strings.ReplaceAll(content, reset, reset+bgSeq)
		w := lipgloss.Width(content)
		if w < ab.width {
			content += strings.Repeat(" ", ab.width-w)
		}
		content = bgSeq + content + reset
	}

	return content
}
