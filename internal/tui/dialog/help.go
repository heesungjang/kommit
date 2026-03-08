package dialog

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nicholascross/opengit/internal/tui/keys"
	"github.com/nicholascross/opengit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// HelpCloseMsg is sent when the help overlay should be closed.
type HelpCloseMsg struct{}

// ---------------------------------------------------------------------------
// Help model
// ---------------------------------------------------------------------------

// Help displays keybindings organised by context. The displayed bindings
// change depending on the active page when the help dialog was opened.
type Help struct {
	ctx    keys.Context
	scroll int
	width  int
	height int
}

// NewHelp creates a new help overlay showing bindings relevant to ctx.
func NewHelp(ctx keys.Context, width, height int) Help {
	return Help{ctx: ctx, width: width, height: height}
}

func (h Help) Init() tea.Cmd { return nil }

func (h Help) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("?", "esc", "q"))):
			return h, func() tea.Msg { return HelpCloseMsg{} }
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			h.scroll++
			return h, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			if h.scroll > 0 {
				h.scroll--
			}
			return h, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+d", "pgdown"))):
			h.scroll += 10
			return h, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+u", "pgup"))):
			h.scroll -= 10
			if h.scroll < 0 {
				h.scroll = 0
			}
			return h, nil
		}
	}
	return h, nil
}

// sectionNames labels each group returned by keys.FullHelp for display.
var sectionNames = []string{
	"Global",
	"Tabs",
	"Panels",
	"Navigation",
}

// contextSectionName returns a human-readable label for the context-specific
// binding group (the 5th group in FullHelp output).
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
	t := theme.Active

	borderSize := 2
	dialogWidth := 70
	if dialogWidth+borderSize > h.width-2 {
		dialogWidth = h.width - borderSize - 2
	}
	if dialogWidth < 30 {
		dialogWidth = 30
	}
	dialogHeight := h.height - borderSize - 2
	if dialogHeight < 10 {
		dialogHeight = 10
	}

	title := lipgloss.NewStyle().
		Foreground(t.Blue).
		Background(t.Surface0).
		Bold(true).
		Padding(0, 0, 1, 0).
		Render("Keyboard Shortcuts")

	keyStyle := lipgloss.NewStyle().Foreground(t.Mauve).Background(t.Surface0).Bold(true).Width(16)
	descStyle := lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface0)
	sectionStyle := lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Surface0).Bold(true).Padding(1, 0, 0, 0)

	groups := keys.FullHelp(h.ctx)

	var allLines []string
	for i, group := range groups {
		// Determine section label.
		var label string
		if i < len(sectionNames) {
			label = sectionNames[i]
		} else {
			label = contextSectionName(h.ctx)
		}

		// Convert bindings to entries, skip empty groups.
		entries := keys.EntriesFromBindings(group)
		if len(entries) == 0 {
			continue
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

	// Scrolling — visible lines = dialogHeight minus vertical padding (2) minus title (~2) minus footer (~2)
	visibleCount := dialogHeight - 6
	if visibleCount < 1 {
		visibleCount = 1
	}
	maxScroll := len(allLines) - visibleCount
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := h.scroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + visibleCount
	if end > len(allLines) {
		end = len(allLines)
	}

	visible := allLines[scroll:end]
	content := lipgloss.JoinVertical(lipgloss.Left, visible...)

	footer := lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Surface0).
		Padding(1, 0, 0, 0).
		Render("Press ? or esc to close")

	body := lipgloss.JoinVertical(lipgloss.Left, title, content, footer)

	return lipgloss.NewStyle().
		Width(dialogWidth).
		Height(dialogHeight).
		Padding(1, 2).
		Background(t.Surface0).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Blue).
		BorderBackground(t.Surface0).
		Render(body)
}

var _ tea.Model = Help{}
