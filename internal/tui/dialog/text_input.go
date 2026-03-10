package dialog

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nicholascross/opengit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// TextInputResultMsg is sent when the user submits text input.
type TextInputResultMsg struct {
	ID    string // caller-defined identifier
	Value string
}

// TextInputCancelMsg is sent when the user cancels the text input.
type TextInputCancelMsg struct {
	ID string
}

// ---------------------------------------------------------------------------
// TextInput model
// ---------------------------------------------------------------------------

// TextInput is a single-line text input dialog.
type TextInput struct {
	ID     string
	title  string
	input  textinput.Model
	width  int
	height int
}

// NewTextInput creates a new text input dialog.
func NewTextInput(id, title, placeholder, initialValue string, width, height int) TextInput {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 40
	if initialValue != "" {
		ti.SetValue(initialValue)
	}

	return TextInput{
		ID:     id,
		title:  title,
		input:  ti,
		width:  width,
		height: height,
	}
}

func (t TextInput) Init() tea.Cmd {
	return textinput.Blink
}

func (t TextInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			id := t.ID
			return t, func() tea.Msg { return TextInputCancelMsg{ID: id} }

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			value := t.input.Value()
			if value == "" {
				return t, nil
			}
			id := t.ID
			return t, func() tea.Msg {
				return TextInputResultMsg{ID: id, Value: value}
			}
		}
	}

	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	return t, cmd
}

func (t TextInput) View() string {
	th := theme.Active

	dialogWidth := 50
	if dialogWidth+2 > t.width-2 {
		dialogWidth = t.width - 4
	}
	if dialogWidth < 20 {
		dialogWidth = 20
	}

	title := lipgloss.NewStyle().
		Foreground(th.Blue).
		Background(th.Surface0).
		Bold(true).
		Padding(0, 0, 1, 0).
		Render(t.title)

	input := t.input.View()

	hints := lipgloss.NewStyle().
		Foreground(th.Overlay0).
		Background(th.Surface0).
		Padding(1, 0, 0, 0).
		Render("enter: confirm  esc: cancel")

	emptyLine := lipgloss.NewStyle().Background(th.Surface0).Render("")
	content := lipgloss.JoinVertical(lipgloss.Left, title, input, emptyLine, hints)

	return lipgloss.NewStyle().
		Width(dialogWidth).
		Padding(1, 2).
		Background(th.Surface0).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Blue).
		BorderBackground(th.Surface0).
		Render(content)
}

var _ tea.Model = TextInput{}
