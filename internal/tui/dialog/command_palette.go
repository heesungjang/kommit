package dialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuictx "github.com/heesungjang/kommit/internal/tui/context"
)

// CommandPaletteResultMsg is sent when the user selects a command from the palette.
type CommandPaletteResultMsg struct {
	Action string // canonical action name, e.g. "global.quit" or "nav.up"
}

// CommandPaletteCloseMsg is sent when the user dismisses the palette.
type CommandPaletteCloseMsg struct{}

// PaletteEntry represents a single item in the command palette.
type PaletteEntry struct {
	Action      string // canonical action name
	Label       string // display label
	Description string // optional description
	Key         string // keyboard shortcut hint
}

// CommandPalette is a fuzzy-filtered action picker, similar to VS Code's ctrl+p.
type CommandPalette struct {
	Base     Base
	input    textinput.Model
	entries  []PaletteEntry // all available entries
	filtered []int          // indices into entries that match the current query
	cursor   int            // selected index within filtered
}

// NewCommandPalette creates a new command palette with the given entries.
func NewCommandPalette(entries []PaletteEntry, pctx *tuictx.ProgramContext) CommandPalette {
	t := pctx.Theme

	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.PromptStyle = lipgloss.NewStyle().Foreground(t.Blue).Background(t.Surface0)
	ti.TextStyle = lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface0)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Surface0)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(t.Mauve).Background(t.Surface0)
	ti.Focus()

	// Build initial filtered list (all entries).
	filtered := make([]int, len(entries))
	for i := range entries {
		filtered[i] = i
	}

	return CommandPalette{
		Base:     NewBaseWithContext("Command Palette", "enter: run  esc: close  ↑↓/ctrl+j,k: navigate", 60, 40, pctx),
		input:    ti,
		entries:  entries,
		filtered: filtered,
		cursor:   0,
	}
}

func (cp CommandPalette) Init() tea.Cmd {
	return textinput.Blink
}

func (cp CommandPalette) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if kmsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(kmsg, key.NewBinding(key.WithKeys("esc"))):
			return cp, func() tea.Msg { return CommandPaletteCloseMsg{} }

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("enter"))):
			if len(cp.filtered) > 0 && cp.cursor < len(cp.filtered) {
				entry := cp.entries[cp.filtered[cp.cursor]]
				action := entry.Action
				return cp, func() tea.Msg {
					return CommandPaletteResultMsg{Action: action}
				}
			}
			return cp, nil

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("up", "ctrl+k"))):
			if cp.cursor > 0 {
				cp.cursor--
				cp.Base.EnsureVisible(cp.cursor, len(cp.filtered))
			}
			return cp, nil

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("down", "ctrl+j"))):
			if cp.cursor < len(cp.filtered)-1 {
				cp.cursor++
				cp.Base.EnsureVisible(cp.cursor, len(cp.filtered))
			}
			return cp, nil

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("ctrl+d", "pgdown"))):
			cp.cursor += 10
			if cp.cursor >= len(cp.filtered) {
				cp.cursor = len(cp.filtered) - 1
			}
			if cp.cursor < 0 {
				cp.cursor = 0
			}
			cp.Base.EnsureVisible(cp.cursor, len(cp.filtered))
			return cp, nil

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("ctrl+u", "pgup"))):
			cp.cursor -= 10
			if cp.cursor < 0 {
				cp.cursor = 0
			}
			cp.Base.EnsureVisible(cp.cursor, len(cp.filtered))
			return cp, nil
		}
	}

	// Update the text input.
	oldValue := cp.input.Value()
	var cmd tea.Cmd
	cp.input, cmd = cp.input.Update(msg)

	// Re-filter if the query changed.
	if cp.input.Value() != oldValue {
		cp.refilter()
	}

	return cp, cmd
}

// refilter updates the filtered indices based on the current input value.
func (cp *CommandPalette) refilter() {
	query := strings.ToLower(cp.input.Value())
	if query == "" {
		cp.filtered = make([]int, len(cp.entries))
		for i := range cp.entries {
			cp.filtered[i] = i
		}
	} else {
		cp.filtered = cp.filtered[:0]
		for i, e := range cp.entries {
			// Simple substring match on action name, label, and description.
			haystack := strings.ToLower(e.Action + " " + e.Label + " " + e.Description)
			if fuzzyMatch(haystack, query) {
				cp.filtered = append(cp.filtered, i)
			}
		}
	}
	cp.cursor = 0
	cp.Base.ScrollOffset = 0
}

// fuzzyMatch returns true if all characters in query appear in haystack in order.
func fuzzyMatch(haystack, query string) bool {
	hi := 0
	for qi := 0; qi < len(query); qi++ {
		found := false
		for hi < len(haystack) {
			if haystack[hi] == query[qi] {
				hi++
				found = true
				break
			}
			hi++
		}
		if !found {
			return false
		}
	}
	return true
}

func (cp CommandPalette) View() string {
	return cp.Base.Render(cp.buildContentLines())
}

// buildContentLines produces the content for the command palette.
func (cp CommandPalette) buildContentLines() []string {
	t := cp.Base.ctx.Theme
	iw := cp.Base.ContentWidth(len(cp.filtered) > cp.Base.MaxContentHeight())

	lines := make([]string, 0, len(cp.filtered)+2)

	// Text input at the top.
	inputView := cp.input.View()
	inputLine := lipgloss.NewStyle().
		Background(t.Surface0).
		Width(iw).
		MaxWidth(iw).
		Render(inputView)
	lines = append(lines, inputLine)

	// Separator.
	sep := lipgloss.NewStyle().
		Foreground(t.Surface2).
		Background(t.Surface0).
		Width(iw).
		Render(strings.Repeat("─", iw))
	lines = append(lines, sep)

	if len(cp.filtered) == 0 {
		noMatch := lipgloss.NewStyle().
			Foreground(t.Overlay0).
			Background(t.Surface0).
			Width(iw).
			Render("  No matching commands")
		lines = append(lines, noMatch)
		return lines
	}

	// Action list.
	for i, idx := range cp.filtered {
		entry := cp.entries[idx]
		selected := i == cp.cursor

		bg := t.Surface0
		if selected {
			bg = t.Surface1
		}

		// Build the label + key hint.
		label := entry.Label
		if label == "" {
			label = entry.Action
		}

		keyHint := ""
		if entry.Key != "" {
			keyHint = entry.Key
		}

		// Render label and key hint.
		labelStyle := lipgloss.NewStyle().Foreground(t.Text).Background(bg)
		keyStyle := lipgloss.NewStyle().Foreground(t.Subtext0).Background(bg)

		if selected {
			labelStyle = labelStyle.Bold(true).Foreground(t.Blue)
		}

		labelRendered := labelStyle.Render("  " + label)
		if keyHint != "" {
			// Right-align the key hint.
			labelWidth := lipgloss.Width(labelRendered)
			gap := iw - labelWidth - lipgloss.Width(keyHint) - 2
			if gap < 1 {
				gap = 1
			}
			labelRendered += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", gap)) +
				keyStyle.Render(keyHint) +
				lipgloss.NewStyle().Background(bg).Render(" ")
		}

		// Pad to full width.
		lineWidth := lipgloss.Width(labelRendered)
		if lineWidth < iw {
			labelRendered += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", iw-lineWidth))
		}

		lines = append(lines, labelRendered)
	}

	return lines
}
