package keys

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Styles for the help bar
// ---------------------------------------------------------------------------

var (
	helpKeyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Bold(true)
	helpDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	helpSepStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	helpBarStyle  = lipgloss.NewStyle().Padding(0, 1)
)

// ---------------------------------------------------------------------------
// HelpEntry
// ---------------------------------------------------------------------------

// HelpEntry represents a single key → description mapping for display.
type HelpEntry struct {
	Key         string
	Description string
}

// EntriesFromBindings converts a slice of key.Binding into HelpEntry values,
// skipping any bindings that are disabled.
func EntriesFromBindings(bindings []key.Binding) []HelpEntry {
	entries := make([]HelpEntry, 0, len(bindings))
	for _, b := range bindings {
		if !b.Enabled() {
			continue
		}
		h := b.Help()
		entries = append(entries, HelpEntry{
			Key:         h.Key,
			Description: h.Desc,
		})
	}
	return entries
}

// ---------------------------------------------------------------------------
// RenderHelp – compact one-line help bar
// ---------------------------------------------------------------------------

// RenderHelp renders a compact help bar showing the most relevant keybindings
// for the given context. The output is truncated to fit within width columns.
func RenderHelp(ctx Context, width int) string {
	bindings := ShortHelp(ctx)
	return renderBar(bindings, width)
}

// RenderFullHelp renders an expanded multi-column help view for the given
// context. Each group of bindings is separated by a blank line.
func RenderFullHelp(ctx Context, width int) string {
	groups := FullHelp(ctx)
	if len(groups) == 0 {
		return ""
	}

	var sections []string
	for _, group := range groups {
		section := renderSection(group)
		if section != "" {
			sections = append(sections, section)
		}
	}

	return lipgloss.NewStyle().
		Width(width).
		Render(strings.Join(sections, "\n\n"))
}

// ---------------------------------------------------------------------------
// Internal rendering helpers
// ---------------------------------------------------------------------------

// separator used between key/desc pairs in the compact bar.
const barSep = " · "

// renderBar renders a single-line help bar from a set of bindings, ensuring
// the result does not exceed the given width.
func renderBar(bindings []key.Binding, width int) string {
	if width <= 0 {
		return ""
	}

	entries := EntriesFromBindings(bindings)
	if len(entries) == 0 {
		return ""
	}

	sep := helpSepStyle.Render(barSep)
	sepLen := len(barSep)

	parts := make([]string, 0, len(entries))
	remaining := width - 2 // account for padding

	for i, e := range entries {
		rendered := helpKeyStyle.Render(e.Key) + " " + helpDescStyle.Render(e.Description)
		plainLen := len(e.Key) + 1 + len(e.Description)

		if i > 0 {
			plainLen += sepLen
		}

		if remaining-plainLen < 0 {
			break
		}

		parts = append(parts, rendered)
		remaining -= plainLen
	}

	joined := strings.Join(parts, sep)
	return helpBarStyle.Render(joined)
}

// renderSection renders a group of bindings as aligned key/description rows.
func renderSection(bindings []key.Binding) string {
	entries := EntriesFromBindings(bindings)
	if len(entries) == 0 {
		return ""
	}

	// Determine the widest key string for alignment.
	maxKeyLen := 0
	for _, e := range entries {
		if len(e.Key) > maxKeyLen {
			maxKeyLen = len(e.Key)
		}
	}

	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteByte('\n')
		}
		padded := e.Key + strings.Repeat(" ", maxKeyLen-len(e.Key))
		b.WriteString(helpKeyStyle.Render(padded))
		b.WriteString("  ")
		b.WriteString(helpDescStyle.Render(e.Description))
	}
	return b.String()
}
