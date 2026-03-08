package dialog

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

// Help displays all keybindings organised by context.
type Help struct {
	scroll int
	width  int
	height int
}

// NewHelp creates a new help overlay.
func NewHelp(width, height int) Help {
	return Help{width: width, height: height}
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
		}
	}
	return h, nil
}

func (h Help) View() string {
	t := theme.Active

	// Border adds 2 to the outer width/height beyond Width()/Height().
	// Leave at least 2 cols/rows margin for centering within Place().
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
		Bold(true).
		Padding(0, 0, 1, 0).
		Render("Keyboard Shortcuts")

	keyStyle := lipgloss.NewStyle().Foreground(t.Mauve).Background(t.Surface0).Bold(true).Width(16)
	descStyle := lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface0)
	sectionStyle := lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Surface0).Bold(true).Padding(1, 0, 0, 0)

	sections := []struct {
		name     string
		bindings [][2]string
	}{
		{
			name: "Global",
			bindings: [][2]string{
				{"q", "Quit"},
				{"ctrl+c", "Force quit"},
				{"?", "Toggle help"},
				{"1-8", "Switch tab"},
				{"tab", "Next panel"},
				{"shift+tab", "Previous panel"},
			},
		},
		{
			name: "Navigation",
			bindings: [][2]string{
				{"j/down", "Move down"},
				{"k/up", "Move up"},
				{"h/left", "Move left"},
				{"l/right", "Move right"},
				{"ctrl+d/pgdn", "Page down"},
				{"ctrl+u/pgup", "Page up"},
				{"g/home", "Go to top"},
				{"G/end", "Go to bottom"},
				{"enter", "Select"},
			},
		},
		{
			name: "Status Page",
			bindings: [][2]string{
				{"space/s", "Stage/unstage file"},
				{"a", "Stage all"},
				{"c", "Commit"},
				{"A", "Amend commit"},
				{"d", "Discard changes"},
				{"p", "Push"},
				{"P", "Pull"},
				{"f", "Fetch"},
				{"r", "Refresh"},
			},
		},
		{
			name: "Branches Page",
			bindings: [][2]string{
				{"enter/o", "Checkout branch"},
				{"n", "New branch"},
				{"D", "Delete branch"},
				{"m", "Merge into current"},
				{"R", "Rename branch"},
			},
		},
		{
			name: "Stash Page",
			bindings: [][2]string{
				{"s", "Stash save"},
				{"p", "Stash pop"},
				{"a", "Stash apply"},
				{"D", "Stash drop"},
			},
		},
		{
			name: "Remotes Page",
			bindings: [][2]string{
				{"p", "Push"},
				{"P", "Pull"},
				{"f", "Fetch remote"},
				{"F", "Fetch all"},
			},
		},
	}

	var allLines []string
	for _, section := range sections {
		allLines = append(allLines, sectionStyle.Render(section.name))
		for _, b := range section.bindings {
			line := lipgloss.JoinHorizontal(lipgloss.Top,
				keyStyle.Render(b[0]),
				descStyle.Render(b[1]),
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
		Padding(1, 0, 0, 0).
		Render("Press ? or esc to close")

	body := lipgloss.JoinVertical(lipgloss.Left, title, content, footer)

	return lipgloss.NewStyle().
		Width(dialogWidth).
		Height(dialogHeight).
		MaxHeight(dialogHeight).
		Padding(1, 2).
		Background(t.Surface0).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Blue).
		BorderBackground(t.Surface0).
		Render(body)
}

var _ tea.Model = Help{}
