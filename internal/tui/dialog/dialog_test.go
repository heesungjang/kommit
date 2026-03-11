package dialog

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/heesungjang/kommit/internal/config"
	tuictx "github.com/heesungjang/kommit/internal/tui/context"
)

// ---------------------------------------------------------------------------
// fuzzyMatch tests
// ---------------------------------------------------------------------------

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		name     string
		haystack string
		query    string
		want     bool
	}{
		{
			name:     "exact match",
			haystack: "hello",
			query:    "hello",
			want:     true,
		},
		{
			name:     "subsequence across words",
			haystack: "hello world",
			query:    "hwd",
			want:     true,
		},
		{
			name:     "case sensitive mismatch (caller must lowercase)",
			haystack: "hello",
			query:    "HELLO",
			want:     false,
		},
		{
			name:     "lowercased input matches",
			haystack: "hello",
			query:    "hello",
			want:     true,
		},
		{
			name:     "no match",
			haystack: "hello",
			query:    "xyz",
			want:     false,
		},
		{
			name:     "empty query matches everything",
			haystack: "anything",
			query:    "",
			want:     true,
		},
		{
			name:     "query longer than haystack",
			haystack: "hi",
			query:    "hello",
			want:     false,
		},
		{
			name:     "single char present",
			haystack: "abc",
			query:    "b",
			want:     true,
		},
		{
			name:     "out of order fails",
			haystack: "abc",
			query:    "ca",
			want:     false,
		},
		{
			name:     "empty haystack with empty query",
			haystack: "",
			query:    "",
			want:     true,
		},
		{
			name:     "empty haystack with non-empty query",
			haystack: "",
			query:    "a",
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fuzzyMatch(tc.haystack, tc.query)
			if got != tc.want {
				t.Errorf("fuzzyMatch(%q, %q) = %v, want %v",
					tc.haystack, tc.query, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FlattenLines tests
// ---------------------------------------------------------------------------

func TestFlattenLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single line",
			input: "hello",
			want:  []string{"hello"},
		},
		{
			name:  "multiple lines",
			input: "one\ntwo\nthree",
			want:  []string{"one", "two", "three"},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{""},
		},
		{
			name:  "trailing newline produces empty last element",
			input: "hello\n",
			want:  []string{"hello", ""},
		},
		{
			name:  "only newlines",
			input: "\n\n",
			want:  []string{"", "", ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FlattenLines(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("FlattenLines(%q): got %d lines, want %d",
					tc.input, len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("FlattenLines(%q)[%d] = %q, want %q",
						tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Base.DialogWidth tests
// ---------------------------------------------------------------------------

func TestDialogWidth(t *testing.T) {
	tests := []struct {
		name         string
		defaultWidth int
		minWidth     int
		termWidth    int
		want         int
	}{
		{
			name:         "normal case fits",
			defaultWidth: 50,
			minWidth:     20,
			termWidth:    100,
			want:         50,
		},
		{
			name:         "terminal too narrow clamps to termWidth-4",
			defaultWidth: 50,
			minWidth:     20,
			termWidth:    30,
			want:         26, // 30-4
		},
		{
			name:         "minWidth enforced when default is smaller",
			defaultWidth: 10,
			minWidth:     20,
			termWidth:    100,
			want:         20,
		},
		{
			name:         "minWidth wins even when terminal is narrow",
			defaultWidth: 50,
			minWidth:     30,
			termWidth:    20,
			want:         30, // termWidth-4=16 < minWidth=30, so 30
		},
		{
			name:         "exact boundary: defaultWidth+2 == termWidth-2",
			defaultWidth: 48,
			minWidth:     10,
			termWidth:    100,
			want:         48, // 48+2=50 <= 100-2=98, fits
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBase("test", "hint", tc.defaultWidth, tc.minWidth, tc.termWidth, 50)
			got := b.DialogWidth()
			if got != tc.want {
				t.Errorf("DialogWidth() = %d, want %d", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Base.InnerWidth tests
// ---------------------------------------------------------------------------

func TestInnerWidth(t *testing.T) {
	tests := []struct {
		name         string
		defaultWidth int
		minWidth     int
		termWidth    int
		want         int
	}{
		{
			name:         "normal case",
			defaultWidth: 50,
			minWidth:     20,
			termWidth:    100,
			want:         46, // 50 - 4
		},
		{
			name:         "clamped dialog width",
			defaultWidth: 50,
			minWidth:     20,
			termWidth:    30,
			want:         22, // (30-4) - 4 = 22
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBase("test", "hint", tc.defaultWidth, tc.minWidth, tc.termWidth, 50)
			got := b.InnerWidth()
			if got != tc.want {
				t.Errorf("InnerWidth() = %d, want %d", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Base.ContentWidth tests
// ---------------------------------------------------------------------------

func TestContentWidth(t *testing.T) {
	tests := []struct {
		name         string
		defaultWidth int
		minWidth     int
		termWidth    int
		needsScroll  bool
		want         int
	}{
		{
			name:         "without scroll",
			defaultWidth: 50,
			minWidth:     20,
			termWidth:    100,
			needsScroll:  false,
			want:         46, // InnerWidth = 50-4 = 46
		},
		{
			name:         "with scroll subtracts gutter",
			defaultWidth: 50,
			minWidth:     20,
			termWidth:    100,
			needsScroll:  true,
			want:         44, // 46 - 2
		},
		{
			name:         "minimum 10 enforced",
			defaultWidth: 14,
			minWidth:     14,
			termWidth:    100,
			needsScroll:  true,
			want:         10, // InnerWidth=10, 10-2=8 < 10, so clamped to 10
		},
		{
			name:         "minimum 10 without scroll",
			defaultWidth: 12,
			minWidth:     12,
			termWidth:    100,
			needsScroll:  false,
			want:         10, // InnerWidth=8 < 10, but actually let's check...
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBase("test", "hint", tc.defaultWidth, tc.minWidth, tc.termWidth, 50)
			got := b.ContentWidth(tc.needsScroll)
			if got != tc.want {
				t.Errorf("ContentWidth(%v) = %d, want %d",
					tc.needsScroll, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Base.MaxContentHeight tests
// ---------------------------------------------------------------------------

func TestMaxContentHeight(t *testing.T) {
	// chrome = borderOverhead(4) + title(2) + footer(2) + margin(2) = 10
	tests := []struct {
		name       string
		termHeight int
		want       int
	}{
		{
			name:       "normal terminal",
			termHeight: 50,
			want:       40, // 50-10
		},
		{
			name:       "small terminal clamps to minimum 3",
			termHeight: 10,
			want:       3, // 10-10=0 < 3, so 3
		},
		{
			name:       "very small terminal clamps to minimum 3",
			termHeight: 5,
			want:       3, // 5-10=-5 < 3, so 3
		},
		{
			name:       "exactly at boundary",
			termHeight: 13,
			want:       3, // 13-10=3
		},
		{
			name:       "one above boundary",
			termHeight: 14,
			want:       4, // 14-10=4
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBase("test", "hint", 50, 20, 100, tc.termHeight)
			got := b.MaxContentHeight()
			if got != tc.want {
				t.Errorf("MaxContentHeight() = %d, want %d", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Base scroll method tests
// ---------------------------------------------------------------------------

func TestClampScroll(t *testing.T) {
	tests := []struct {
		name         string
		termHeight   int
		scrollOffset int
		totalLines   int
		wantOffset   int
	}{
		{
			name:         "offset within bounds stays",
			termHeight:   50,
			scrollOffset: 5,
			totalLines:   100,
			wantOffset:   5,
		},
		{
			name:         "offset beyond max is clamped",
			termHeight:   50, // MaxContentHeight=40
			scrollOffset: 80,
			totalLines:   100, // maxScroll=100-40=60
			wantOffset:   60,
		},
		{
			name:         "negative offset clamped to zero",
			termHeight:   50,
			scrollOffset: -5,
			totalLines:   100,
			wantOffset:   0,
		},
		{
			name:         "total lines less than visible",
			termHeight:   50, // MaxContentHeight=40
			scrollOffset: 10,
			totalLines:   20, // maxScroll=max(20-40,0)=0
			wantOffset:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBase("test", "hint", 50, 20, 100, tc.termHeight)
			b.ScrollOffset = tc.scrollOffset
			b.ClampScroll(tc.totalLines)
			if b.ScrollOffset != tc.wantOffset {
				t.Errorf("ClampScroll(%d): ScrollOffset = %d, want %d",
					tc.totalLines, b.ScrollOffset, tc.wantOffset)
			}
		})
	}
}

func TestScrollDown(t *testing.T) {
	// termHeight=50 → MaxContentHeight=40
	b := NewBase("test", "hint", 50, 20, 100, 50)
	b.ScrollOffset = 0

	// Scroll down by 5 with 100 total lines
	b.ScrollDown(5, 100)
	if b.ScrollOffset != 5 {
		t.Errorf("after ScrollDown(5, 100): got %d, want 5", b.ScrollOffset)
	}

	// Scroll down by 100 should clamp to maxScroll=60
	b.ScrollDown(100, 100)
	if b.ScrollOffset != 60 {
		t.Errorf("after ScrollDown(100, 100): got %d, want 60", b.ScrollOffset)
	}
}

func TestScrollUp(t *testing.T) {
	b := NewBase("test", "hint", 50, 20, 100, 50)
	b.ScrollOffset = 20

	b.ScrollUp(5)
	if b.ScrollOffset != 15 {
		t.Errorf("after ScrollUp(5): got %d, want 15", b.ScrollOffset)
	}

	// Scroll up past zero should floor at 0
	b.ScrollUp(100)
	if b.ScrollOffset != 0 {
		t.Errorf("after ScrollUp(100): got %d, want 0", b.ScrollOffset)
	}
}

func TestEnsureVisible(t *testing.T) {
	// termHeight=50 → MaxContentHeight=40
	tests := []struct {
		name         string
		scrollOffset int
		idx          int
		totalLines   int
		wantOffset   int
	}{
		{
			name:         "already visible no change",
			scrollOffset: 0,
			idx:          10,
			totalLines:   100,
			wantOffset:   0,
		},
		{
			name:         "item above viewport scrolls up",
			scrollOffset: 20,
			idx:          5,
			totalLines:   100,
			wantOffset:   5,
		},
		{
			name:         "item below viewport scrolls down",
			scrollOffset: 0,
			idx:          45,
			totalLines:   100,
			wantOffset:   6, // 45 - 40 + 1 = 6
		},
		{
			name:         "totalLines <= maxVisible resets to 0",
			scrollOffset: 10,
			idx:          5,
			totalLines:   30,
			wantOffset:   0,
		},
		{
			name:         "item at bottom edge of viewport",
			scrollOffset: 0,
			idx:          39, // last visible index in viewport [0..39]
			totalLines:   100,
			wantOffset:   0,
		},
		{
			name:         "item just past bottom edge",
			scrollOffset: 0,
			idx:          40, // first index beyond viewport
			totalLines:   100,
			wantOffset:   1, // 40 - 40 + 1 = 1
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBase("test", "hint", 50, 20, 100, 50)
			b.ScrollOffset = tc.scrollOffset
			b.EnsureVisible(tc.idx, tc.totalLines)
			if b.ScrollOffset != tc.wantOffset {
				t.Errorf("EnsureVisible(%d, %d): ScrollOffset = %d, want %d",
					tc.idx, tc.totalLines, b.ScrollOffset, tc.wantOffset)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ListPicker tests
// ---------------------------------------------------------------------------

func TestListPickerNew(t *testing.T) {
	opts := []ListPickerOption{
		{Label: "Alpha", Value: "a"},
		{Label: "Beta", Value: "b"},
		{Label: "Gamma", Value: "c"},
	}
	lp := NewListPicker("test-id", "Pick", opts, "b", testCtx())
	if lp.ID != "test-id" {
		t.Errorf("ID = %q, want %q", lp.ID, "test-id")
	}
	if lp.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (matching activeValue 'b')", lp.cursor)
	}
	if lp.activeValue != "b" {
		t.Errorf("activeValue = %q, want %q", lp.activeValue, "b")
	}
	if len(lp.Options) != 3 {
		t.Errorf("len(Options) = %d, want 3", len(lp.Options))
	}
}

func TestListPickerNew_NoMatch(t *testing.T) {
	opts := []ListPickerOption{
		{Label: "One", Value: "1"},
		{Label: "Two", Value: "2"},
	}
	lp := NewListPicker("x", "Pick", opts, "missing", testCtx())
	if lp.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (no match defaults to first)", lp.cursor)
	}
}

func TestListPickerNavigation(t *testing.T) {
	opts := []ListPickerOption{
		{Label: "A", Value: "a"},
		{Label: "B", Value: "b"},
		{Label: "C", Value: "c"},
	}
	lp := NewListPicker("nav", "Pick", opts, "a", testCtx())

	// Move down
	m, _ := lp.Update(keyMsg("j"))
	lp = m.(ListPicker)
	if lp.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", lp.cursor)
	}

	// Move down again
	m, _ = lp.Update(keyMsg("j"))
	lp = m.(ListPicker)
	if lp.cursor != 2 {
		t.Errorf("after j j: cursor = %d, want 2", lp.cursor)
	}

	// Move down at bottom — stays at 2
	m, _ = lp.Update(keyMsg("j"))
	lp = m.(ListPicker)
	if lp.cursor != 2 {
		t.Errorf("at bottom j: cursor = %d, want 2", lp.cursor)
	}

	// Move up
	m, _ = lp.Update(keyMsg("k"))
	lp = m.(ListPicker)
	if lp.cursor != 1 {
		t.Errorf("after k: cursor = %d, want 1", lp.cursor)
	}
}

func TestListPickerSelect(t *testing.T) {
	opts := []ListPickerOption{
		{Label: "A", Value: "a"},
		{Label: "B", Value: "b"},
	}
	lp := NewListPicker("sel", "Pick", opts, "a", testCtx())

	// Move to second option and select
	m, _ := lp.Update(keyMsg("j"))
	lp = m.(ListPicker)
	_, cmd := lp.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("enter should produce a cmd")
	}
	msg := cmd()
	result, ok := msg.(ListPickerResultMsg)
	if !ok {
		t.Fatalf("expected ListPickerResultMsg, got %T", msg)
	}
	if result.ID != "sel" {
		t.Errorf("result.ID = %q, want %q", result.ID, "sel")
	}
	if result.Value != "b" {
		t.Errorf("result.Value = %q, want %q", result.Value, "b")
	}
	if result.Index != 1 {
		t.Errorf("result.Index = %d, want 1", result.Index)
	}
}

func TestListPickerCancel(t *testing.T) {
	opts := []ListPickerOption{
		{Label: "X", Value: "x"},
	}
	lp := NewListPicker("can", "Pick", opts, "x", testCtx())
	_, cmd := lp.Update(keyMsg("esc"))
	if cmd == nil {
		t.Fatal("esc should produce a cmd")
	}
	msg := cmd()
	cancel, ok := msg.(ListPickerCancelMsg)
	if !ok {
		t.Fatalf("expected ListPickerCancelMsg, got %T", msg)
	}
	if cancel.ID != "can" {
		t.Errorf("cancel.ID = %q, want %q", cancel.ID, "can")
	}
}

func TestListPickerPreview(t *testing.T) {
	opts := []ListPickerOption{
		{Label: "A", Value: "a"},
		{Label: "B", Value: "b"},
	}
	lp := NewListPicker("prev", "Pick", opts, "a", testCtx())

	// Move down — should emit preview message
	_, cmd := lp.Update(keyMsg("j"))
	if cmd == nil {
		t.Fatal("j should produce a preview cmd")
	}
	msg := cmd()
	preview, ok := msg.(ListPickerPreviewMsg)
	if !ok {
		t.Fatalf("expected ListPickerPreviewMsg, got %T", msg)
	}
	if preview.Value != "b" {
		t.Errorf("preview.Value = %q, want %q", preview.Value, "b")
	}
}

func TestListPickerView(t *testing.T) {
	opts := []ListPickerOption{
		{Label: "Alpha", Value: "a"},
		{Label: "Beta", Value: "b"},
	}
	lp := NewListPicker("view", "Pick", opts, "a", testCtx())
	view := lp.View()
	if view == "" {
		t.Error("View() should not be empty")
	}
}

// ---------------------------------------------------------------------------
// Settings dialog tests
// ---------------------------------------------------------------------------

func TestSettingsNew(t *testing.T) {
	cfg := testConfig()
	s := NewSettings(cfg, testCtx())
	if len(s.defs) == 0 {
		t.Fatal("Settings should have at least one setting definition")
	}
	if s.cursor != 0 {
		t.Errorf("cursor = %d, want 0", s.cursor)
	}
}

func TestSettingsNavigation(t *testing.T) {
	cfg := testConfig()
	s := NewSettings(cfg, testCtx())
	total := len(s.defs)

	// Move down through all settings
	var m tea.Model = s
	for i := 0; i < total-1; i++ {
		m, _ = m.Update(keyMsg("j"))
	}
	s = m.(Settings)
	if s.cursor != total-1 {
		t.Errorf("cursor = %d, want %d", s.cursor, total-1)
	}

	// Move down at bottom — stays
	m, _ = s.Update(keyMsg("j"))
	s = m.(Settings)
	if s.cursor != total-1 {
		t.Errorf("at bottom j: cursor = %d, want %d", s.cursor, total-1)
	}

	// Move up
	m, _ = s.Update(keyMsg("k"))
	s = m.(Settings)
	if s.cursor != total-2 {
		t.Errorf("after k: cursor = %d, want %d", s.cursor, total-2)
	}
}

func TestSettingsToggle(t *testing.T) {
	cfg := testConfig()
	cfg.Appearance.ShowGraph = true
	s := NewSettings(cfg, testCtx())

	// Find the showGraph setting index
	graphIdx := -1
	for i, def := range s.defs {
		if def.Key == "appearance.showGraph" {
			graphIdx = i
			break
		}
	}
	if graphIdx < 0 {
		t.Fatal("showGraph setting not found")
	}

	// Navigate to it
	var m tea.Model = s
	for i := 0; i < graphIdx; i++ {
		m, _ = m.Update(keyMsg("j"))
	}
	s = m.(Settings)

	// Toggle it
	m, cmd := s.Update(keyMsg("enter"))
	s = m.(Settings)
	if cmd == nil {
		t.Fatal("enter on toggle should produce a cmd")
	}
	msg := cmd()
	change, ok := msg.(SettingsChangeMsg)
	if !ok {
		t.Fatalf("expected SettingsChangeMsg, got %T", msg)
	}
	if change.Key != "appearance.showGraph" {
		t.Errorf("change.Key = %q, want %q", change.Key, "appearance.showGraph")
	}
	if change.Value != "false" {
		t.Errorf("change.Value = %q, want %q (toggled from true)", change.Value, "false")
	}

	// Toggle again
	_, cmd = s.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("second enter should produce a cmd")
	}
	msg = cmd()
	change = msg.(SettingsChangeMsg)
	if change.Value != "true" {
		t.Errorf("second toggle: change.Value = %q, want %q", change.Value, "true")
	}
}

func TestSettingsListPicker(t *testing.T) {
	cfg := testConfig()
	s := NewSettings(cfg, testCtx())

	// First setting should be Theme (a list type)
	if s.defs[0].Key != "theme" {
		t.Fatalf("first setting is %q, expected 'theme'", s.defs[0].Key)
	}

	// Enter on theme should open a sub-picker
	m, _ := s.Update(keyMsg("enter"))
	s = m.(Settings)
	if s.subPicker == nil {
		t.Fatal("enter on list setting should open a sub-picker")
	}
}

func TestSettingsClose(t *testing.T) {
	cfg := testConfig()
	s := NewSettings(cfg, testCtx())

	_, cmd := s.Update(keyMsg("esc"))
	if cmd == nil {
		t.Fatal("esc should produce a close cmd")
	}
	msg := cmd()
	if _, ok := msg.(SettingsCloseMsg); !ok {
		t.Fatalf("expected SettingsCloseMsg, got %T", msg)
	}
}

func TestSettingsCloseWithComma(t *testing.T) {
	cfg := testConfig()
	s := NewSettings(cfg, testCtx())

	_, cmd := s.Update(keyMsg(","))
	if cmd == nil {
		t.Fatal("comma should produce a close cmd")
	}
	msg := cmd()
	if _, ok := msg.(SettingsCloseMsg); !ok {
		t.Fatalf("expected SettingsCloseMsg, got %T", msg)
	}
}

func TestSettingsView(t *testing.T) {
	cfg := testConfig()
	s := NewSettings(cfg, testCtx())
	view := s.View()
	if view == "" {
		t.Error("View() should not be empty")
	}
}

func TestSettingsCurrentValue(t *testing.T) {
	cfg := testConfig()
	cfg.Theme = "catppuccin-latte"
	cfg.Appearance.ShowGraph = false
	s := NewSettings(cfg, testCtx())

	// Theme value
	v := s.currentValue(s.defs[0])
	if v != "catppuccin-latte" {
		t.Errorf("theme value = %q, want %q", v, "catppuccin-latte")
	}

	// ShowGraph value (should show "off")
	for _, def := range s.defs {
		if def.Key == "appearance.showGraph" {
			v = s.currentValue(def)
			if v != "off" {
				t.Errorf("showGraph value = %q, want %q", v, "off")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func testCtx() *tuictx.ProgramContext {
	cfg := testConfig()
	return &tuictx.ProgramContext{
		ScreenWidth:  120,
		ScreenHeight: 40,
		Config:       cfg,
	}
}

func testConfig() *config.Config {
	c := config.DefaultConfig()
	return &c
}

func keyMsg(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}
