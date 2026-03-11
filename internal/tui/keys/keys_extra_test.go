package keys

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

// ---------------------------------------------------------------------------
// LookupBinding tests
// (bindings_test.go does not test LookupBinding directly)
// ---------------------------------------------------------------------------

func TestLookupBinding_Existing(t *testing.T) {
	b := LookupBinding("global.quit")
	if b == nil {
		t.Fatal("LookupBinding(\"global.quit\") returned nil")
	}
	// The binding should have at least one key.
	if len(b.Keys()) == 0 {
		t.Error("global.quit binding has no keys")
	}
}

func TestLookupBinding_Nonexistent(t *testing.T) {
	b := LookupBinding("does.not.exist")
	if b != nil {
		t.Errorf("LookupBinding(\"does.not.exist\") = %v, want nil", b)
	}
}

func TestLookupBinding_AllRegistered(t *testing.T) {
	for _, name := range ActionNames() {
		b := LookupBinding(name)
		if b == nil {
			t.Errorf("LookupBinding(%q) returned nil for registered action", name)
		}
	}
}

// ---------------------------------------------------------------------------
// ActionNames supplementary tests
// (bindings_test.go already tests non-empty, sorted, contains expected;
//  here we add a count sanity check and verify no duplicates)
// ---------------------------------------------------------------------------

func TestActionNames_NoDuplicates(t *testing.T) {
	names := ActionNames()
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		if seen[n] {
			t.Errorf("duplicate action name: %q", n)
		}
		seen[n] = true
	}
}

func TestActionNames_CountMatchesRegistry(t *testing.T) {
	names := ActionNames()
	// We know the registry has entries for global, nav, status, branch,
	// commit, diff, stash, remote, and pr. Total should be > 40.
	if len(names) < 40 {
		t.Errorf("ActionNames() returned %d names, expected at least 40", len(names))
	}
}

// ---------------------------------------------------------------------------
// splitKeys supplementary tests
// (bindings_test.go covers basic cases; we add edge cases here)
// ---------------------------------------------------------------------------

func TestSplitKeys_OnlySpaces(t *testing.T) {
	got := splitKeys("   ")
	if len(got) != 0 {
		t.Errorf("splitKeys(\"   \") = %v, want nil/empty", got)
	}
}

func TestSplitKeys_ModifierCombination(t *testing.T) {
	got := splitKeys("ctrl+shift+a")
	if len(got) != 1 || got[0] != "ctrl+shift+a" {
		t.Errorf("splitKeys(\"ctrl+shift+a\") = %v, want [ctrl+shift+a]", got)
	}
}

func TestSplitKeys_TabSpaceMix(t *testing.T) {
	// Tabs are not spaces, so "a\tb" should be a single token.
	got := splitKeys("a\tb")
	if len(got) != 1 || got[0] != "a\tb" {
		t.Errorf("splitKeys(\"a\\tb\") = %v, want [a\\tb]", got)
	}
}

// ---------------------------------------------------------------------------
// EntriesFromBindings tests
// ---------------------------------------------------------------------------

func TestEntriesFromBindings_Basic(t *testing.T) {
	bindings := []key.Binding{
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
	entries := EntriesFromBindings(bindings)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Key != "q" || entries[0].Description != "quit" {
		t.Errorf("entry[0] = {%q, %q}, want {q, quit}", entries[0].Key, entries[0].Description)
	}
	if entries[1].Key != "?" || entries[1].Description != "help" {
		t.Errorf("entry[1] = {%q, %q}, want {?, help}", entries[1].Key, entries[1].Description)
	}
}

func TestEntriesFromBindings_SkipsDisabled(t *testing.T) {
	enabled := key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "action"))
	disabled := key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "disabled"))
	disabled.SetEnabled(false)

	entries := EntriesFromBindings([]key.Binding{enabled, disabled})
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (disabled should be skipped)", len(entries))
	}
	if entries[0].Key != "a" {
		t.Errorf("entry[0].Key = %q, want \"a\"", entries[0].Key)
	}
}

func TestEntriesFromBindings_EmptySlice(t *testing.T) {
	entries := EntriesFromBindings(nil)
	if len(entries) != 0 {
		t.Errorf("got %d entries for nil input, want 0", len(entries))
	}
}

func TestEntriesFromBindings_AllDisabled(t *testing.T) {
	d1 := key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "x"))
	d1.SetEnabled(false)
	d2 := key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "y"))
	d2.SetEnabled(false)

	entries := EntriesFromBindings([]key.Binding{d1, d2})
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0 (all disabled)", len(entries))
	}
}
