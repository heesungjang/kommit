package dialog

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/keys"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// HelpCloseMsg is sent when the help overlay should be closed.
type HelpCloseMsg struct{}

// ---------------------------------------------------------------------------
// Help model
// ---------------------------------------------------------------------------

// Help displays keybindings organized by context. The displayed bindings
// change depending on the active page when the help dialog was opened.
type Help struct {
	Base Base
	ctx  keys.Context
}

// NewHelp creates a new help overlay showing bindings relevant to kctx.
func NewHelp(kctx keys.Context, pctx *tuictx.ProgramContext) Help {
	return Help{
		Base: NewBaseWithContext("Keyboard Shortcuts", "j/k: scroll  pgup/pgdn: page  ?: close", 70, 30, pctx),
		ctx:  kctx,
	}
}

func (h Help) Init() tea.Cmd { return nil }

func (h Help) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		totalLines := len(h.buildContentLines())

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("?", "esc", "q"))):
			return h, func() tea.Msg { return HelpCloseMsg{} }
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			h.Base.ScrollDown(1, totalLines)
			return h, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			h.Base.ScrollUp(1)
			return h, nil
		}

		// Let base handle pgup/pgdn/ctrl+d/ctrl+u.
		if h.Base.HandleScrollKeys(msg, totalLines) {
			return h, nil
		}
	}
	return h, nil
}

// sectionNames labels each base group returned by keys.FullHelp for display.
// The order must match the base groups in keys.FullHelp: global, panels, navigation.
var sectionNames = []string{
	"Global",
	"Panels",
	"Navigation",
}

// contextSectionNames returns labels for all context-specific groups.
// Most contexts have 1 extra group; ContextLog has 2 (commit ops + remote ops).
func contextSectionNames(ctx keys.Context) []string {
	switch ctx {
	case keys.ContextLog:
		return []string{"Commit Operations", "Remote"}
	default:
		return []string{contextSectionName(ctx)}
	}
}

// contextSectionName returns a human-readable label for the context-specific
// binding group (groups beyond the 3 base groups in FullHelp output).
func contextSectionName(ctx keys.Context) string {
	switch ctx {
	case keys.ContextStatus:
		return "Status"
	case keys.ContextLog:
		return "Log"
	case keys.ContextBranches:
		return "Branches"
	case keys.ContextDiff:
		return "Diff"
	case keys.ContextStash:
		return "Stash"
	case keys.ContextRemotes:
		return "Remotes"
	case keys.ContextPR:
		return "Pull Requests"
	case keys.ContextCI:
		return "CI/CD"
	case keys.ContextWorkspace:
		return "Workspace"
	default:
		return "Context"
	}
}

func (h Help) View() string {
	return h.Base.Render(h.buildContentLines())
}

// buildContentLines produces all keybinding lines for scrollable rendering.
// Every returned entry is a single terminal line (no embedded newlines).
func (h Help) buildContentLines() []string {
	t := h.Base.ctx.Theme

	emptyLine := lipgloss.NewStyle().Background(t.Surface0).Render("")
	keyStyle := lipgloss.NewStyle().Foreground(t.Mauve).Background(t.Surface0).Bold(true).Width(16)
	descStyle := lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface0)
	sectionStyle := lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Surface0).Bold(true)

	groups := keys.FullHelp(h.ctx)
	contextLabels := contextSectionNames(h.ctx)

	allLines := make([]string, 0, 64)
	for i, group := range groups {
		// Determine section label.
		var label string
		if i < len(sectionNames) {
			label = sectionNames[i]
		} else {
			ci := i - len(sectionNames) // index into context-specific groups
			if ci < len(contextLabels) {
				label = contextLabels[ci]
			} else {
				label = contextSectionName(h.ctx)
			}
		}

		// Convert bindings to entries, skip empty groups.
		entries := keys.EntriesFromBindings(group)
		if len(entries) == 0 {
			continue
		}

		// Blank line before each section (except the very first).
		if len(allLines) > 0 {
			allLines = append(allLines, emptyLine)
		}
		allLines = append(allLines, sectionStyle.Render(label))

		for _, e := range entries {
			line := lipgloss.JoinHorizontal(lipgloss.Top,
				keyStyle.Render(e.Key),
				descStyle.Render(e.Description),
			)
			allLines = append(allLines, line)
		}
	}

	return allLines
}

var _ tea.Model = Help{}
