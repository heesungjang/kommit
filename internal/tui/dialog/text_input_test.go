package dialog

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TextInput – construction
// ---------------------------------------------------------------------------

func TestTextInputNew(t *testing.T) {
	ti := NewTextInput("branch", "Branch name", "feature/...", "", testCtx())
	if ti.ID != "branch" {
		t.Errorf("ID = %q, want %q", ti.ID, "branch")
	}
}

func TestTextInputNewWithInitialValue(t *testing.T) {
	ti := NewTextInput("rename", "Rename", "new name", "old-name", testCtx())
	if ti.ID != "rename" {
		t.Errorf("ID = %q, want %q", ti.ID, "rename")
	}
	// The underlying textinput model should have the initial value set.
	if ti.input.Value() != "old-name" {
		t.Errorf("input.Value() = %q, want %q", ti.input.Value(), "old-name")
	}
}

// ---------------------------------------------------------------------------
// TextInput – escape returns TextInputCancelMsg
// ---------------------------------------------------------------------------

func TestTextInputCancel(t *testing.T) {
	ti := NewTextInput("esc-test", "Title", "placeholder", "", testCtx())
	_, cmd := ti.Update(keyMsg("esc"))
	if cmd == nil {
		t.Fatal("esc should produce a cmd")
	}
	msg := cmd()
	cancel, ok := msg.(TextInputCancelMsg)
	if !ok {
		t.Fatalf("expected TextInputCancelMsg, got %T", msg)
	}
	if cancel.ID != "esc-test" {
		t.Errorf("cancel.ID = %q, want %q", cancel.ID, "esc-test")
	}
}

// ---------------------------------------------------------------------------
// TextInput – enter with empty value does nothing
// ---------------------------------------------------------------------------

func TestTextInputEnterEmpty(t *testing.T) {
	ti := NewTextInput("empty", "Title", "placeholder", "", testCtx())
	_, cmd := ti.Update(keyMsg("enter"))
	if cmd != nil {
		t.Error("enter on empty input should not produce a cmd")
	}
}

// ---------------------------------------------------------------------------
// TextInput – enter with value returns TextInputResultMsg
// ---------------------------------------------------------------------------

func TestTextInputEnterWithValue(t *testing.T) {
	ti := NewTextInput("submit", "Title", "placeholder", "hello", testCtx())
	_, cmd := ti.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("enter with value should produce a cmd")
	}
	msg := cmd()
	result, ok := msg.(TextInputResultMsg)
	if !ok {
		t.Fatalf("expected TextInputResultMsg, got %T", msg)
	}
	if result.ID != "submit" {
		t.Errorf("result.ID = %q, want %q", result.ID, "submit")
	}
	if result.Value != "hello" {
		t.Errorf("result.Value = %q, want %q", result.Value, "hello")
	}
}

// ---------------------------------------------------------------------------
// TextInput – View returns non-empty string
// ---------------------------------------------------------------------------

func TestTextInputView(t *testing.T) {
	ti := NewTextInput("v", "Title", "placeholder", "", testCtx())
	view := ti.View()
	if view == "" {
		t.Error("View() should not be empty")
	}
}
