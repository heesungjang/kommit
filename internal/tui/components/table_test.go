package components

import (
	"strings"
	"testing"
)

func TestNewTable(t *testing.T) {
	cols := []Column{
		{Title: "Name", Width: 20},
		{Title: "Age", Width: 5, Align: AlignRight},
	}
	tbl := NewTable(cols, 80, 10)

	if len(tbl.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(tbl.Columns))
	}
	if tbl.Cursor != 0 {
		t.Errorf("Expected cursor 0, got %d", tbl.Cursor)
	}
	if !tbl.ShowHdr {
		t.Error("ShowHdr should default to true")
	}
}

func TestTable_SetRows(t *testing.T) {
	cols := []Column{{Title: "Name", Width: 20}}
	tbl := NewTable(cols, 80, 10)
	tbl.Cursor = 5 // set cursor to something

	tbl = tbl.SetRows([]Row{{"Alice"}, {"Bob"}, {"Charlie"}})

	if len(tbl.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(tbl.Rows))
	}
	if tbl.Cursor != 0 {
		t.Error("Cursor should be reset to 0 after SetRows")
	}
}

func TestTable_MoveDown(t *testing.T) {
	cols := []Column{{Title: "Name", Width: 20}}
	tbl := NewTable(cols, 80, 10)
	tbl = tbl.SetRows([]Row{{"A"}, {"B"}, {"C"}})

	tbl.MoveDown(1)
	if tbl.Cursor != 1 {
		t.Errorf("Cursor = %d, want 1", tbl.Cursor)
	}

	tbl.MoveDown(5) // overshoot
	if tbl.Cursor != 2 {
		t.Errorf("Cursor = %d, want 2 (clamped)", tbl.Cursor)
	}
}

func TestTable_MoveUp(t *testing.T) {
	cols := []Column{{Title: "Name", Width: 20}}
	tbl := NewTable(cols, 80, 10)
	tbl = tbl.SetRows([]Row{{"A"}, {"B"}, {"C"}})
	tbl.Cursor = 2

	tbl.MoveUp(1)
	if tbl.Cursor != 1 {
		t.Errorf("Cursor = %d, want 1", tbl.Cursor)
	}

	tbl.MoveUp(5) // overshoot
	if tbl.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0 (clamped)", tbl.Cursor)
	}
}

func TestTable_GoToTopBottom(t *testing.T) {
	cols := []Column{{Title: "Name", Width: 20}}
	tbl := NewTable(cols, 80, 5)
	tbl = tbl.SetRows([]Row{{"A"}, {"B"}, {"C"}, {"D"}, {"E"}})

	tbl.GoToBottom()
	if tbl.Cursor != 4 {
		t.Errorf("GoToBottom: Cursor = %d, want 4", tbl.Cursor)
	}

	tbl.GoToTop()
	if tbl.Cursor != 0 {
		t.Errorf("GoToTop: Cursor = %d, want 0", tbl.Cursor)
	}
}

func TestTable_SelectedRow(t *testing.T) {
	cols := []Column{{Title: "Name", Width: 20}}
	tbl := NewTable(cols, 80, 10)
	tbl = tbl.SetRows([]Row{{"Alice"}, {"Bob"}})

	row := tbl.SelectedRow()
	if row == nil || row[0] != "Alice" {
		t.Errorf("SelectedRow() = %v, want [Alice]", row)
	}

	tbl.MoveDown(1)
	row = tbl.SelectedRow()
	if row == nil || row[0] != "Bob" {
		t.Errorf("SelectedRow() = %v, want [Bob]", row)
	}
}

func TestTable_SelectedRow_Empty(t *testing.T) {
	cols := []Column{{Title: "Name", Width: 20}}
	tbl := NewTable(cols, 80, 10)

	row := tbl.SelectedRow()
	if row != nil {
		t.Errorf("SelectedRow() on empty table = %v, want nil", row)
	}
}

func TestTable_View_HasHeader(t *testing.T) {
	cols := []Column{
		{Title: "Name", Width: 10},
		{Title: "Value", Width: 10},
	}
	tbl := NewTable(cols, 25, 5)
	tbl = tbl.SetRows([]Row{{"foo", "bar"}})

	view := tbl.View()
	lines := strings.Split(view, "\n")

	// Should have at least 2 lines (header + 1 row)
	if len(lines) < 2 {
		t.Errorf("View has %d lines, want >= 2", len(lines))
	}
}

func TestTable_View_NoHeader(t *testing.T) {
	cols := []Column{{Title: "Name", Width: 10}}
	tbl := NewTable(cols, 25, 5)
	tbl.ShowHdr = false
	tbl = tbl.SetRows([]Row{{"foo"}})

	view := tbl.View()
	lines := strings.Split(view, "\n")

	// With height=5 and no header, should have exactly 5 lines (1 data + 4 padding)
	if len(lines) != 5 {
		t.Errorf("View has %d lines, want 5", len(lines))
	}
}

func TestTable_Scrolling(t *testing.T) {
	cols := []Column{{Title: "Name", Width: 10}}
	tbl := NewTable(cols, 15, 3) // header + 2 visible rows
	rows := make([]Row, 10)
	for i := range rows {
		rows[i] = Row{strings.Repeat("x", i+1)}
	}
	tbl = tbl.SetRows(rows)

	tbl.GoToBottom()
	if tbl.Offset == 0 {
		t.Error("After GoToBottom on 10 rows with 2 visible, offset should be > 0")
	}
	if tbl.Cursor != 9 {
		t.Errorf("Cursor = %d, want 9", tbl.Cursor)
	}
}

func TestAlignText(t *testing.T) {
	tests := []struct {
		text  string
		width int
		align Align
		want  string
	}{
		{"hello", 10, AlignLeft, "hello"},
		{"hello", 10, AlignRight, "     hello"},
		{"hi", 6, AlignCenter, "  hi  "},
		{"toolong", 4, AlignLeft, "too~"},
		{"ab", 2, AlignLeft, "ab"},
	}
	for _, tt := range tests {
		got := alignText(tt.text, tt.width, tt.align)
		if got != tt.want {
			t.Errorf("alignText(%q, %d, %d) = %q, want %q", tt.text, tt.width, tt.align, got, tt.want)
		}
	}
}

func TestComputeColumnWidths_AllFixed(t *testing.T) {
	cols := []Column{
		{Title: "A", Width: 10},
		{Title: "B", Width: 20},
	}
	tbl := NewTable(cols, 50, 5)
	widths := tbl.computeColumnWidths()

	if widths[0] != 10 {
		t.Errorf("widths[0] = %d, want 10", widths[0])
	}
	if widths[1] != 20 {
		t.Errorf("widths[1] = %d, want 20", widths[1])
	}
}

func TestComputeColumnWidths_FlexColumn(t *testing.T) {
	cols := []Column{
		{Title: "Fixed", Width: 10},
		{Title: "Flex", Width: 0, MinWidth: 5},
	}
	tbl := NewTable(cols, 50, 5)
	widths := tbl.computeColumnWidths()

	if widths[0] != 10 {
		t.Errorf("widths[0] = %d, want 10", widths[0])
	}
	// Flex column should get remaining: 50 - 10 - 1(separator) = 39
	if widths[1] != 39 {
		t.Errorf("widths[1] = %d, want 39", widths[1])
	}
}
