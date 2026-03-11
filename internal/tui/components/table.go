// Package components provides reusable TUI building blocks.
//
// Table is a simple, theme-aware table component with cursor navigation,
// scrolling, and configurable columns. Unlike bubbles/table, it:
//   - Integrates with opengit's theme system (no external styles needed)
//   - Supports per-column alignment and truncation
//   - Renders with a compact, borderless design suitable for panel embedding
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nicholascross/opengit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Column definition
// ---------------------------------------------------------------------------

// Align specifies column text alignment.
type Align int

const (
	AlignLeft Align = iota
	AlignCenter
	AlignRight
)

// Column defines a table column.
type Column struct {
	Title    string
	Width    int   // fixed width; 0 = flexible (takes remaining space)
	MinWidth int   // minimum width when flexible (default 4)
	Align    Align // text alignment within the column
}

// ---------------------------------------------------------------------------
// Row
// ---------------------------------------------------------------------------

// Row is a table row — a slice of cell strings parallel to the column list.
type Row []string

// ---------------------------------------------------------------------------
// Table
// ---------------------------------------------------------------------------

// Table is a cursor-navigable, scrollable table component.
type Table struct {
	Columns []Column
	Rows    []Row
	Cursor  int  // selected row index
	Offset  int  // scroll offset (first visible row)
	ShowHdr bool // show column header row

	width  int
	height int // visible rows (excluding header)
}

// NewTable creates a table with the given columns and visible area.
func NewTable(cols []Column, width, height int) Table {
	return Table{
		Columns: cols,
		ShowHdr: true,
		width:   width,
		height:  height,
	}
}

// SetSize updates the available width and height.
func (t Table) SetSize(width, height int) Table {
	t.width = width
	t.height = height
	return t
}

// SetRows replaces all rows and resets the cursor.
func (t Table) SetRows(rows []Row) Table {
	t.Rows = rows
	t.Cursor = 0
	t.Offset = 0
	return t
}

// MoveUp moves the cursor up by n rows, clamping to bounds.
func (t *Table) MoveUp(n int) {
	t.Cursor -= n
	if t.Cursor < 0 {
		t.Cursor = 0
	}
	t.ensureVisible()
}

// MoveDown moves the cursor down by n rows, clamping to bounds.
func (t *Table) MoveDown(n int) {
	t.Cursor += n
	if t.Cursor >= len(t.Rows) {
		t.Cursor = len(t.Rows) - 1
	}
	if t.Cursor < 0 {
		t.Cursor = 0
	}
	t.ensureVisible()
}

// GoToTop moves the cursor to the first row.
func (t *Table) GoToTop() {
	t.Cursor = 0
	t.Offset = 0
}

// GoToBottom moves the cursor to the last row.
func (t *Table) GoToBottom() {
	if len(t.Rows) > 0 {
		t.Cursor = len(t.Rows) - 1
	}
	t.ensureVisible()
}

// SelectedRow returns the currently selected row, or nil.
func (t Table) SelectedRow() Row {
	if t.Cursor >= 0 && t.Cursor < len(t.Rows) {
		return t.Rows[t.Cursor]
	}
	return nil
}

// ensureVisible adjusts Offset so the Cursor is within the visible window.
func (t *Table) ensureVisible() {
	visibleRows := t.visibleRows()
	if visibleRows <= 0 {
		return
	}
	if t.Cursor < t.Offset {
		t.Offset = t.Cursor
	}
	if t.Cursor >= t.Offset+visibleRows {
		t.Offset = t.Cursor - visibleRows + 1
	}
	if t.Offset < 0 {
		t.Offset = 0
	}
}

// visibleRows returns the number of data rows that fit in the height.
func (t Table) visibleRows() int {
	h := t.height
	if t.ShowHdr {
		h-- // header takes one row
	}
	if h < 0 {
		h = 0
	}
	return h
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

// View renders the table.
func (t Table) View() string {
	th := theme.Active
	colWidths := t.computeColumnWidths()

	var lines []string

	// Header
	if t.ShowHdr {
		hdr := t.renderHeader(colWidths, th)
		lines = append(lines, hdr)
	}

	// Rows
	visibleCount := t.visibleRows()
	end := t.Offset + visibleCount
	if end > len(t.Rows) {
		end = len(t.Rows)
	}

	for i := t.Offset; i < end; i++ {
		row := t.Rows[i]
		selected := i == t.Cursor
		line := t.renderRow(row, colWidths, selected, th)
		lines = append(lines, line)
	}

	// Pad remaining lines if needed
	for len(lines) < t.height {
		lines = append(lines, lipgloss.NewStyle().
			Background(th.Base).
			Width(t.width).
			Render(""))
	}

	return strings.Join(lines, "\n")
}

// computeColumnWidths distributes width among columns.
func (t Table) computeColumnWidths() []int {
	widths := make([]int, len(t.Columns))
	totalFixed := 0
	flexCount := 0
	for i, col := range t.Columns {
		if col.Width > 0 {
			widths[i] = col.Width
			totalFixed += col.Width
		} else {
			flexCount++
		}
	}

	// Remaining space for flexible columns (minus separators: 1 char between each).
	separators := 0
	if len(t.Columns) > 1 {
		separators = len(t.Columns) - 1
	}
	remaining := t.width - totalFixed - separators
	if remaining < 0 {
		remaining = 0
	}

	if flexCount > 0 {
		perFlex := remaining / flexCount
		for i, col := range t.Columns {
			if col.Width == 0 {
				w := perFlex
				minW := col.MinWidth
				if minW <= 0 {
					minW = 4
				}
				if w < minW {
					w = minW
				}
				widths[i] = w
			}
		}
	}

	return widths
}

// renderHeader renders the column header row.
func (t Table) renderHeader(colWidths []int, th theme.Theme) string {
	var cells []string
	for i, col := range t.Columns {
		w := colWidths[i]
		text := alignText(col.Title, w, col.Align)
		cell := lipgloss.NewStyle().
			Foreground(th.Subtext0).
			Background(th.Surface0).
			Bold(true).
			Width(w).
			MaxWidth(w).
			Render(text)
		cells = append(cells, cell)
	}
	sep := lipgloss.NewStyle().Background(th.Surface0).Render(" ")
	row := strings.Join(cells, sep)

	// Pad to full width
	rw := lipgloss.Width(row)
	if rw < t.width {
		pad := lipgloss.NewStyle().Background(th.Surface0).Width(t.width - rw).Render("")
		row += pad
	}
	return row
}

// renderRow renders a single data row.
func (t Table) renderRow(row Row, colWidths []int, selected bool, th theme.Theme) string {
	bg := th.Base
	fg := th.Text
	if selected {
		bg = th.Surface1
	}

	var cells []string
	for i, col := range t.Columns {
		w := colWidths[i]
		text := ""
		if i < len(row) {
			text = row[i]
		}
		text = alignText(text, w, col.Align)
		cell := lipgloss.NewStyle().
			Foreground(fg).
			Background(bg).
			Width(w).
			MaxWidth(w).
			Render(text)
		cells = append(cells, cell)
	}
	sep := lipgloss.NewStyle().Background(bg).Render(" ")
	line := strings.Join(cells, sep)

	// Pad to full width
	lw := lipgloss.Width(line)
	if lw < t.width {
		pad := lipgloss.NewStyle().Background(bg).Width(t.width - lw).Render("")
		line += pad
	}
	return line
}

// alignText aligns text within the given width.
func alignText(text string, width int, align Align) string {
	// Truncate if too long.
	if len(text) > width {
		if width <= 3 {
			return text[:width]
		}
		return text[:width-1] + "~"
	}

	switch align {
	case AlignRight:
		pad := width - len(text)
		if pad > 0 {
			return strings.Repeat(" ", pad) + text
		}
	case AlignCenter:
		pad := width - len(text)
		if pad > 0 {
			left := pad / 2
			right := pad - left
			return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
		}
	}
	return text
}
