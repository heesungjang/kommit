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

// ListPickerResultMsg is sent when the user selects an option from the list picker.
type ListPickerResultMsg struct {
	ID    string // caller-defined identifier (e.g. "theme-picker")
	Index int    // which option was selected (0-based)
	Value string // value of the selected option
}

// ListPickerCancelMsg is sent when the user cancels the list picker.
type ListPickerCancelMsg struct {
	ID string
}

// ---------------------------------------------------------------------------
// ListPickerOption
// ---------------------------------------------------------------------------

// ListPickerOption represents a single option in the list picker.
type ListPickerOption struct {
	Label       string // display text
	Value       string // underlying value returned in the result
	Description string // optional description shown when selected
}

// ---------------------------------------------------------------------------
// ListPicker model
// ---------------------------------------------------------------------------

// ListPicker is a single-select list overlay dialog. It is a lighter
// alternative to Menu, designed for settings pickers (theme, diff mode, etc.).
// The currently-active value is highlighted with a check mark.
type ListPicker struct {
	Base         Base
	ID           string
	Options      []ListPickerOption
	cursor       int
	activeValue  string // the currently-active value (shown with check mark)
	previewValue string // value being previewed (cursor position) — used for live preview
}

// NewListPicker creates a new list picker dialog.
func NewListPicker(id, title string, options []ListPickerOption, activeValue string, ctx *tuictx.ProgramContext) ListPicker {
	// Find the cursor position matching the active value.
	cursor := 0
	for i, opt := range options {
		if opt.Value == activeValue {
			cursor = i
			break
		}
	}

	return ListPicker{
		Base:         NewBaseWithContext(title, "j/k: navigate  enter: select  esc: cancel", 44, 20, ctx),
		ID:           id,
		Options:      options,
		cursor:       cursor,
		activeValue:  activeValue,
		previewValue: activeValue,
	}
}

func (lp ListPicker) Init() tea.Cmd { return nil }

func (lp ListPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if kmsg, ok := msg.(tea.KeyMsg); ok {
		totalLines := len(lp.buildContentLines())
		if lp.Base.HandleScrollKeys(kmsg, totalLines) {
			return lp, nil
		}

		switch {
		case key.Matches(kmsg, key.NewBinding(key.WithKeys("up", "k"))):
			if lp.cursor > 0 {
				lp.cursor--
			}
			lp.previewValue = lp.Options[lp.cursor].Value
			lp.ensureCursorVisible()
			return lp, lp.emitPreview()

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("down", "j"))):
			if lp.cursor < len(lp.Options)-1 {
				lp.cursor++
			}
			lp.previewValue = lp.Options[lp.cursor].Value
			lp.ensureCursorVisible()
			return lp, lp.emitPreview()

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("enter"))):
			return lp, lp.selectOption(lp.cursor)

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("esc", "q"))):
			return lp, lp.cancel()
		}
	}
	return lp, nil
}

func (lp ListPicker) View() string {
	return lp.Base.Render(lp.buildContentLines())
}

// PreviewValue returns the value currently being previewed (cursor position).
func (lp ListPicker) PreviewValue() string {
	return lp.previewValue
}

// buildContentLines produces the scrollable content lines.
func (lp ListPicker) buildContentLines() []string {
	t := theme.Active
	w := lp.Base.InnerWidth()

	lines := make([]string, 0, len(lp.Options)*2)
	for i, opt := range lp.Options {
		selected := i == lp.cursor
		isActive := opt.Value == lp.activeValue

		bg := t.Surface0
		fg := t.Text
		prefix := "  "
		if isActive {
			prefix = "* " // check mark for active
		}
		if selected {
			bg = t.Blue
			fg = t.Base
			if isActive {
				prefix = "* "
			} else {
				prefix = "> "
			}
		}

		label := prefix + opt.Label

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

		lines = append(lines, FlattenLines(line)...)
	}
	return lines
}

// ensureCursorVisible adjusts the scroll offset so the cursor option is visible.
func (lp *ListPicker) ensureCursorVisible() {
	t := theme.Active
	w := lp.Base.InnerWidth()

	lineIdx := 0
	for i, opt := range lp.Options {
		if i == lp.cursor {
			break
		}
		label := "  " + opt.Label
		style := lipgloss.NewStyle().
			Foreground(t.Text).
			Background(t.Surface0).
			Width(w)
		rendered := style.Render(label)
		lineIdx += strings.Count(rendered, "\n") + 1
	}

	totalLines := len(lp.buildContentLines())
	lp.Base.EnsureVisible(lineIdx, totalLines)
}

func (lp ListPicker) selectOption(idx int) tea.Cmd {
	if idx < 0 || idx >= len(lp.Options) {
		return nil
	}
	id := lp.ID
	value := lp.Options[idx].Value
	return func() tea.Msg {
		return ListPickerResultMsg{ID: id, Index: idx, Value: value}
	}
}

func (lp ListPicker) cancel() tea.Cmd {
	id := lp.ID
	return func() tea.Msg {
		return ListPickerCancelMsg{ID: id}
	}
}

// emitPreview sends a ListPickerPreviewMsg for live preview of the hovered option.
func (lp ListPicker) emitPreview() tea.Cmd {
	id := lp.ID
	value := lp.previewValue
	idx := lp.cursor
	return func() tea.Msg {
		return ListPickerPreviewMsg{ID: id, Index: idx, Value: value}
	}
}

// ListPickerPreviewMsg is sent when the cursor moves in the list picker,
// allowing the parent to apply a live preview (e.g. theme preview).
type ListPickerPreviewMsg struct {
	ID    string
	Index int
	Value string
}

var _ tea.Model = ListPicker{}
