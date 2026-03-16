package dialog

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Menu – construction
// ---------------------------------------------------------------------------

func TestMenuNew(t *testing.T) {
	opts := []MenuOption{
		{Label: "Squash", Key: "s"},
		{Label: "Merge", Key: "m"},
		{Label: "Rebase", Key: "r"},
	}
	m := NewMenu("merge-menu", "Strategy", opts, testCtx())
	if m.ID != "merge-menu" {
		t.Errorf("ID = %q, want %q", m.ID, "merge-menu")
	}
	if len(m.Options) != 3 {
		t.Errorf("len(Options) = %d, want 3", len(m.Options))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

// ---------------------------------------------------------------------------
// Menu – cursor navigation with j/k and down/up
// ---------------------------------------------------------------------------

func TestMenuNavigation(t *testing.T) {
	opts := []MenuOption{
		{Label: "A"},
		{Label: "B"},
		{Label: "C"},
	}

	tests := []struct {
		name       string
		keys       []string
		wantCursor int
	}{
		{name: "j moves down", keys: []string{"j"}, wantCursor: 1},
		{name: "k at top stays", keys: []string{"k"}, wantCursor: 0},
		{name: "j j moves to 2", keys: []string{"j", "j"}, wantCursor: 2},
		{name: "j j j clamps at bottom", keys: []string{"j", "j", "j"}, wantCursor: 2},
		{name: "j j k moves to 1", keys: []string{"j", "j", "k"}, wantCursor: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			menu := NewMenu("nav", "Pick", opts, testCtx())
			var model tea.Model = menu
			for _, k := range tc.keys {
				model, _ = model.Update(keyMsg(k))
			}
			got := model.(Menu).cursor
			if got != tc.wantCursor {
				t.Errorf("cursor = %d, want %d", got, tc.wantCursor)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Menu – enter returns MenuResultMsg with correct Index
// ---------------------------------------------------------------------------

func TestMenuSelect(t *testing.T) {
	opts := []MenuOption{
		{Label: "Alpha"},
		{Label: "Beta"},
		{Label: "Gamma"},
	}

	tests := []struct {
		name      string
		movesDown int
		wantIdx   int
		wantLabel string
	}{
		{name: "select first", movesDown: 0, wantIdx: 0, wantLabel: "Alpha"},
		{name: "select second", movesDown: 1, wantIdx: 1, wantLabel: "Beta"},
		{name: "select third", movesDown: 2, wantIdx: 2, wantLabel: "Gamma"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			menu := NewMenu("sel", "Pick", opts, testCtx())
			var model tea.Model = menu
			for i := 0; i < tc.movesDown; i++ {
				model, _ = model.Update(keyMsg("j"))
			}
			_, cmd := model.Update(keyMsg("enter"))
			if cmd == nil {
				t.Fatal("enter should produce a cmd")
			}
			msg := cmd()
			result, ok := msg.(MenuResultMsg)
			if !ok {
				t.Fatalf("expected MenuResultMsg, got %T", msg)
			}
			if result.ID != "sel" {
				t.Errorf("result.ID = %q, want %q", result.ID, "sel")
			}
			if result.Index != tc.wantIdx {
				t.Errorf("result.Index = %d, want %d", result.Index, tc.wantIdx)
			}
			if result.Label != tc.wantLabel {
				t.Errorf("result.Label = %q, want %q", result.Label, tc.wantLabel)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Menu – shortcut keys select directly
// ---------------------------------------------------------------------------

func TestMenuShortcutKeys(t *testing.T) {
	opts := []MenuOption{
		{Label: "Squash", Key: "s"},
		{Label: "Merge", Key: "m"},
		{Label: "Rebase", Key: "r"},
	}
	menu := NewMenu("sc", "Strategy", opts, testCtx())

	// Press "m" — should select Merge (index 1)
	_, cmd := menu.Update(keyMsg("m"))
	if cmd == nil {
		t.Fatal("shortcut key should produce a cmd")
	}
	msg := cmd()
	result, ok := msg.(MenuResultMsg)
	if !ok {
		t.Fatalf("expected MenuResultMsg, got %T", msg)
	}
	if result.Index != 1 {
		t.Errorf("result.Index = %d, want 1", result.Index)
	}
	if result.Label != "Merge" {
		t.Errorf("result.Label = %q, want %q", result.Label, "Merge")
	}
}

// ---------------------------------------------------------------------------
// Menu – escape returns MenuCancelMsg
// ---------------------------------------------------------------------------

func TestMenuCancel(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "esc cancels", key: "esc"},
		{name: "q cancels", key: "q"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := []MenuOption{{Label: "X"}}
			menu := NewMenu("can", "Title", opts, testCtx())
			_, cmd := menu.Update(keyMsg(tc.key))
			if cmd == nil {
				t.Fatalf("%s should produce a cmd", tc.key)
			}
			msg := cmd()
			cancel, ok := msg.(MenuCancelMsg)
			if !ok {
				t.Fatalf("expected MenuCancelMsg, got %T", msg)
			}
			if cancel.ID != "can" {
				t.Errorf("cancel.ID = %q, want %q", cancel.ID, "can")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Menu – View returns non-empty string
// ---------------------------------------------------------------------------

func TestMenuView(t *testing.T) {
	opts := []MenuOption{
		{Label: "Alpha", Description: "First letter"},
		{Label: "Beta"},
	}
	menu := NewMenu("view", "Pick", opts, testCtx())
	view := menu.View()
	if view == "" {
		t.Error("View() should not be empty")
	}
}
