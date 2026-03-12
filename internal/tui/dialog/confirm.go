package dialog

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// ConfirmResultMsg is sent when the user answers the confirm dialog.
type ConfirmResultMsg struct {
	ID        string // caller-defined identifier so the receiver knows which confirm this was
	Confirmed bool
}

// ---------------------------------------------------------------------------
// Confirm model
// ---------------------------------------------------------------------------

// Confirm is a simple yes/no dialog that overlays the current view.
type Confirm struct {
	Base    Base
	ID      string
	Message string
	focused bool // true = "Yes" is focused, false = "No"
}

// NewConfirm creates a new confirmation dialog using a shared ProgramContext.
func NewConfirm(id, title, message string, ctx *tuictx.ProgramContext) Confirm {
	return Confirm{
		Base:    NewBaseWithContext(title, "y/n: answer  ←/→: switch  enter: confirm  esc: cancel", 50, 20, ctx),
		ID:      id,
		Message: message,
		focused: false, // default to "No" for safety
	}
}

func (c Confirm) Init() tea.Cmd { return nil }

func (c Confirm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		// Let base handle page-scroll keys first.
		if c.Base.HandleScrollKeys(msg, len(c.buildContentLines())) {
			return c, nil
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("left", "h", "tab"))):
			c.focused = !c.focused
			return c, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("right", "l"))):
			c.focused = !c.focused
			return c, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("y", "Y"))):
			return c, c.confirm(true)

		case key.Matches(msg, key.NewBinding(key.WithKeys("n", "N", "esc"))):
			return c, c.confirm(false)

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			return c, c.confirm(c.focused)
		}
	}
	return c, nil
}

func (c Confirm) View() string {
	return c.Base.Render(c.buildContentLines())
}

// buildContentLines produces the scrollable content lines for the confirm
// dialog. Every entry is a single terminal line (no embedded newlines).
func (c Confirm) buildContentLines() []string {
	t := theme.Active

	// Render the message with word wrapping.
	message := lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Surface0).
		Width(c.Base.InnerWidth()).
		Render(c.Message)

	// Build buttons.
	yesStyle := lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Surface0).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Surface2).
		BorderBackground(t.Surface0)
	noStyle := yesStyle

	if c.focused {
		yesStyle = yesStyle.
			Foreground(t.Base).
			Background(t.Green).
			Bold(true).
			BorderForeground(t.Green).
			BorderBackground(t.Surface0)
	} else {
		noStyle = noStyle.
			Foreground(t.Base).
			Background(t.Red).
			Bold(true).
			BorderForeground(t.Red).
			BorderBackground(t.Surface0)
	}

	// The separator must match the button height (3 lines due to borders)
	// to avoid JoinHorizontal padding artifacts.
	btnSep := lipgloss.NewStyle().Background(t.Surface0).Render("  \n  \n  ")
	buttons := lipgloss.JoinHorizontal(lipgloss.Top,
		yesStyle.Render("Yes"),
		btnSep,
		noStyle.Render("No"),
	)

	// Flatten everything into individual lines. Add a blank line separator
	// between the message and buttons.
	emptyLine := lipgloss.NewStyle().Background(t.Surface0).Render("")

	var lines []string
	lines = append(lines, FlattenLines(message)...)
	lines = append(lines, emptyLine)
	lines = append(lines, FlattenLines(buttons)...)
	return lines
}

func (c Confirm) confirm(yes bool) tea.Cmd {
	id := c.ID
	return func() tea.Msg {
		return ConfirmResultMsg{ID: id, Confirmed: yes}
	}
}

var _ tea.Model = Confirm{}
