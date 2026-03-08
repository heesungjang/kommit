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
	ID      string
	Title   string
	Message string
	focused bool // true = "Yes" is focused, false = "No"

	width  int
	height int
}

// NewConfirm creates a new confirmation dialog.
func NewConfirm(id, title, message string, width, height int) Confirm {
	return Confirm{
		ID:      id,
		Title:   title,
		Message: message,
		focused: false, // default to "No" for safety
		width:   width,
		height:  height,
	}
}

func (c Confirm) Init() tea.Cmd { return nil }

func (c Confirm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
	t := theme.Active

	dialogWidth := 50
	if dialogWidth+2 > c.width-2 { // +2 for border, -2 for centering margin
		dialogWidth = c.width - 4
	}

	title := lipgloss.NewStyle().
		Foreground(t.Blue).
		Background(t.Surface0).
		Bold(true).
		Padding(0, 0, 1, 0).
		Render(c.Title)

	message := lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Surface0).
		Width(dialogWidth - 4).
		Render(c.Message)

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

	btnSep := lipgloss.NewStyle().Background(t.Surface0).Render("  ")
	buttons := lipgloss.JoinHorizontal(lipgloss.Top,
		yesStyle.Render("Yes"),
		btnSep,
		noStyle.Render("No"),
	)
	buttons = lipgloss.NewStyle().Background(t.Surface0).Padding(1, 0, 0, 0).Render(buttons)

	content := lipgloss.JoinVertical(lipgloss.Left, title, message, buttons)

	return lipgloss.NewStyle().
		Width(dialogWidth).
		Padding(1, 2).
		Background(t.Surface0).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Blue).
		BorderBackground(t.Surface0).
		Render(content)
}

func (c Confirm) confirm(yes bool) tea.Cmd {
	id := c.ID
	return func() tea.Msg {
		return ConfirmResultMsg{ID: id, Confirmed: yes}
	}
}

var _ tea.Model = Confirm{}
