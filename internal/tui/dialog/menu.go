package dialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/theme"
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
	Base    Base
	ID      string
	Options []MenuOption
	cursor  int
}

// NewMenu creates a new menu dialog using a shared ProgramContext.
func NewMenu(id, title string, options []MenuOption, ctx *tuictx.ProgramContext) Menu {
	return Menu{
		Base:    NewBaseWithContext(title, "j/k: navigate  enter: select  esc: cancel", 50, 20, ctx),
		ID:      id,
		Options: options,
		cursor:  0,
	}
}

func (m Menu) Init() tea.Cmd { return nil }

func (m Menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		// Let base handle page-scroll keys first.
		totalLines := len(m.buildContentLines())
		if m.Base.HandleScrollKeys(msg, totalLines) {
			return m, nil
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.cursor > 0 {
				m.cursor--
			}
			m.ensureCursorVisible()
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if m.cursor < len(m.Options)-1 {
				m.cursor++
			}
			m.ensureCursorVisible()
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			return m, m.selectOption(m.cursor)

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q"))):
			return m, m.cancel()
		}

		// Check for shortcut keys.
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
	return m.Base.Render(m.buildContentLines())
}

// buildContentLines produces the scrollable content lines for the menu dialog.
// Every entry is a single terminal line (no embedded newlines).
func (m Menu) buildContentLines() []string {
	t := theme.Active
	w := m.Base.InnerWidth()

	var lines []string
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
			Width(w).
			Bold(selected)
		line := style.Render(label)

		if opt.Description != "" && selected {
			descStyle := lipgloss.NewStyle().
				Foreground(t.Subtext0).
				Background(bg).
				Width(w).
				Padding(0, 0, 0, 2)
			line = lipgloss.JoinVertical(lipgloss.Left, line, descStyle.Render(opt.Description))
		}

		// Flatten: each option may produce multiple rendered lines
		// (if description is shown).
		lines = append(lines, FlattenLines(line)...)
	}
	return lines
}

// ensureCursorVisible adjusts the scroll offset so the cursor option is visible.
func (m *Menu) ensureCursorVisible() {
	t := theme.Active
	w := m.Base.InnerWidth()

	lineIdx := 0
	for i, opt := range m.Options {
		if i == m.cursor {
			break
		}
		// Count how many lines this option occupies.
		label := "  " + opt.Label
		if opt.Key != "" {
			label += " [" + opt.Key + "]"
		}
		style := lipgloss.NewStyle().
			Foreground(t.Text).
			Background(t.Surface0).
			Width(w)
		rendered := style.Render(label)
		lineIdx += strings.Count(rendered, "\n") + 1

		// If this was the cursor position, description would be showing too.
		// But since we break when i == m.cursor, non-cursor items never have
		// descriptions expanded, so this is correct.
	}

	totalLines := len(m.buildContentLines())
	m.Base.EnsureVisible(lineIdx, totalLines)
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
