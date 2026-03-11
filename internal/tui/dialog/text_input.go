package dialog

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuictx "github.com/nicholascross/opengit/internal/tui/context"
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
	Base  Base
	ID    string
	input textinput.Model
}

// NewTextInput creates a new text input dialog using a shared ProgramContext.
func NewTextInput(id, title, placeholder, initialValue string, ctx *tuictx.ProgramContext) TextInput {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 40

	if initialValue != "" {
		ti.SetValue(initialValue)
	}

	return TextInput{
		Base:  NewBaseWithContext(title, "enter: confirm  esc: cancel", 50, 20, ctx),
		ID:    id,
		input: ti,
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
	return t.Base.Render(t.buildContentLines())
}

// buildContentLines produces the scrollable content lines for the text input
// dialog. Every entry is a single terminal line.
func (t TextInput) buildContentLines() []string {
	th := theme.Active
	emptyLine := lipgloss.NewStyle().Background(th.Surface0).Render("")

	var lines []string
	lines = append(lines, FlattenLines(t.input.View())...)
	lines = append(lines, emptyLine)
	return lines
}

var _ tea.Model = TextInput{}
