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

// MenuResultMsg is sent when the user selects an option from the menu.
type MenuResultMsg struct {
	ID    string // caller-defined identifier
	Index int    // which option was selected (0-based)
	Label string // label of the selected option
}

// MenuCancelMsg is sent when the user cancels the menu.
type MenuCancelMsg struct {
	ID string
}

// MenuOption represents a single option in the menu dialog.
type MenuOption struct {
	Label       string // display text
	Description string // optional description shown below the label
	Key         string // optional shortcut key hint (e.g. "s", "m", "h")
}

// ---------------------------------------------------------------------------
// Menu model
// ---------------------------------------------------------------------------

// Menu is a multi-option selector dialog that overlays the current view.
type Menu struct {
	ID      string
	Title   string
	Options []MenuOption
	cursor  int

	width  int
	height int
}

// NewMenu creates a new menu dialog.
func NewMenu(id, title string, options []MenuOption, width, height int) Menu {
	return Menu{
		ID:      id,
		Title:   title,
		Options: options,
		cursor:  0,
		width:   width,
		height:  height,
	}
}

func (m Menu) Init() tea.Cmd { return nil }

func (m Menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if m.cursor < len(m.Options)-1 {
				m.cursor++
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			return m, m.selectOption(m.cursor)

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q"))):
			return m, m.cancel()
		}

		// Check for shortcut keys
		if len(msg.Runes) == 1 {
			ch := string(msg.Runes[0])
			for i, opt := range m.Options {
				if opt.Key != "" && opt.Key == ch {
					return m, m.selectOption(i)
				}
			}
		}
	}
	return m, nil
}

func (m Menu) View() string {
	t := theme.Active

	dialogWidth := 50
	if dialogWidth+2 > m.width-2 {
		dialogWidth = m.width - 4
	}
	if dialogWidth < 20 {
		dialogWidth = 20
	}

	title := lipgloss.NewStyle().
		Foreground(t.Blue).
		Background(t.Surface0).
		Bold(true).
		Padding(0, 0, 1, 0).
		Render(m.Title)

	var optLines []string
	for i, opt := range m.Options {
		selected := i == m.cursor
		bg := t.Surface0
		fg := t.Text
		prefix := "  "
		if selected {
			bg = t.Blue
			fg = t.Base
			prefix = "▸ "
		}

		label := prefix + opt.Label
		if opt.Key != "" {
			keyHint := " [" + opt.Key + "]"
			label += keyHint
		}

		style := lipgloss.NewStyle().
			Foreground(fg).
			Background(bg).
			Width(dialogWidth - 4).
			Bold(selected)
		line := style.Render(label)

		if opt.Description != "" && selected {
			descStyle := lipgloss.NewStyle().
				Foreground(t.Subtext0).
				Background(bg).
				Width(dialogWidth-4).
				Padding(0, 0, 0, 2)
			line = lipgloss.JoinVertical(lipgloss.Left, line, descStyle.Render(opt.Description))
		}

		optLines = append(optLines, line)
	}

	optContent := lipgloss.JoinVertical(lipgloss.Left, optLines...)

	hint := lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Surface0).
		Padding(1, 0, 0, 0).
		Render("j/k:navigate  enter:select  esc:cancel")

	content := lipgloss.JoinVertical(lipgloss.Left, title, optContent, hint)

	return lipgloss.NewStyle().
		Width(dialogWidth).
		Padding(1, 2).
		Background(t.Surface0).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Blue).
		BorderBackground(t.Surface0).
		Render(content)
}

func (m Menu) selectOption(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.Options) {
		return nil
	}
	id := m.ID
	label := m.Options[idx].Label
	return func() tea.Msg {
		return MenuResultMsg{ID: id, Index: idx, Label: label}
	}
}

func (m Menu) cancel() tea.Cmd {
	id := m.ID
	return func() tea.Msg {
		return MenuCancelMsg{ID: id}
	}
}

var _ tea.Model = Menu{}
