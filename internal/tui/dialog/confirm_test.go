package dialog

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Confirm – construction
// ---------------------------------------------------------------------------

func TestConfirmNew(t *testing.T) {
	c := NewConfirm("del", "Delete?", "Are you sure?", testCtx())
	if c.ID != "del" {
		t.Errorf("ID = %q, want %q", c.ID, "del")
	}
	if c.Message != "Are you sure?" {
		t.Errorf("Message = %q, want %q", c.Message, "Are you sure?")
	}
	if c.focused != false {
		t.Error("focused should default to false (No)")
	}
}

// ---------------------------------------------------------------------------
// Confirm – tab toggles focus
// ---------------------------------------------------------------------------

func TestConfirmTabToggle(t *testing.T) {
	c := NewConfirm("t", "Title", "msg", testCtx())
	// Default focused = false (No)
	if c.focused {
		t.Fatal("precondition: focused should start false")
	}

	// Tab should toggle to Yes
	m, _ := c.Update(keyMsg("tab"))
	c = m.(Confirm)
	if !c.focused {
		t.Error("after tab: focused should be true (Yes)")
	}

	// Tab again should toggle back to No
	m, _ = c.Update(keyMsg("tab"))
	c = m.(Confirm)
	if c.focused {
		t.Error("after second tab: focused should be false (No)")
	}
}

// ---------------------------------------------------------------------------
// Confirm – left/right toggles focus
// ---------------------------------------------------------------------------

func TestConfirmArrowToggle(t *testing.T) {
	c := NewConfirm("t", "Title", "msg", testCtx())

	// Left toggles
	m, _ := c.Update(keyMsg("h"))
	c = m.(Confirm)
	if !c.focused {
		t.Error("after h: focused should be true")
	}

	// Right toggles back
	m, _ = c.Update(keyMsg("l"))
	c = m.(Confirm)
	if c.focused {
		t.Error("after l: focused should be false")
	}
}

// ---------------------------------------------------------------------------
// Confirm – enter returns ConfirmResultMsg with correct Confirmed
// ---------------------------------------------------------------------------

func TestConfirmEnter(t *testing.T) {
	tests := []struct {
		name        string
		toggleToYes bool
		wantConfirm bool
	}{
		{
			name:        "enter on No (default)",
			toggleToYes: false,
			wantConfirm: false,
		},
		{
			name:        "enter on Yes (after toggle)",
			toggleToYes: true,
			wantConfirm: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewConfirm("enter-test", "Confirm?", "msg", testCtx())
			if tc.toggleToYes {
				m, _ := c.Update(keyMsg("tab"))
				c = m.(Confirm)
			}

			_, cmd := c.Update(keyMsg("enter"))
			if cmd == nil {
				t.Fatal("enter should produce a cmd")
			}
			msg := cmd()
			result, ok := msg.(ConfirmResultMsg)
			if !ok {
				t.Fatalf("expected ConfirmResultMsg, got %T", msg)
			}
			if result.ID != "enter-test" {
				t.Errorf("result.ID = %q, want %q", result.ID, "enter-test")
			}
			if result.Confirmed != tc.wantConfirm {
				t.Errorf("result.Confirmed = %v, want %v", result.Confirmed, tc.wantConfirm)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Confirm – y key confirms, n/esc cancels
// ---------------------------------------------------------------------------

func TestConfirmShortcutKeys(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		wantConfirm bool
	}{
		{name: "y confirms", key: "y", wantConfirm: true},
		{name: "Y confirms", key: "Y", wantConfirm: true},
		{name: "n cancels", key: "n", wantConfirm: false},
		{name: "N cancels", key: "N", wantConfirm: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewConfirm("sc", "Title", "msg", testCtx())
			_, cmd := c.Update(keyMsg(tc.key))
			if cmd == nil {
				t.Fatalf("key %q should produce a cmd", tc.key)
			}
			msg := cmd()
			result, ok := msg.(ConfirmResultMsg)
			if !ok {
				t.Fatalf("expected ConfirmResultMsg, got %T", msg)
			}
			if result.Confirmed != tc.wantConfirm {
				t.Errorf("key %q: Confirmed = %v, want %v", tc.key, result.Confirmed, tc.wantConfirm)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Confirm – escape returns ConfirmResultMsg{Confirmed: false}
// ---------------------------------------------------------------------------

func TestConfirmEscape(t *testing.T) {
	c := NewConfirm("esc-test", "Delete?", "really?", testCtx())

	// Even after toggling to Yes, esc should cancel
	m, _ := c.Update(keyMsg("tab"))
	c = m.(Confirm)
	if !c.focused {
		t.Fatal("precondition: should be focused on Yes")
	}

	_, cmd := c.Update(keyMsg("esc"))
	if cmd == nil {
		t.Fatal("esc should produce a cmd")
	}
	msg := cmd()
	result, ok := msg.(ConfirmResultMsg)
	if !ok {
		t.Fatalf("expected ConfirmResultMsg, got %T", msg)
	}
	if result.ID != "esc-test" {
		t.Errorf("result.ID = %q, want %q", result.ID, "esc-test")
	}
	if result.Confirmed {
		t.Error("esc should always set Confirmed = false")
	}
}

// ---------------------------------------------------------------------------
// Confirm – View returns non-empty string
// ---------------------------------------------------------------------------

func TestConfirmView(t *testing.T) {
	c := NewConfirm("v", "Title", "Some message", testCtx())
	view := c.View()
	if view == "" {
		t.Error("View() should not be empty")
	}
}
