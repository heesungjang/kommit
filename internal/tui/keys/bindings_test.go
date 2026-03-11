package keys

import (
	"testing"
)

func TestActionNames_NonEmpty(t *testing.T) {
	names := ActionNames()
	if len(names) == 0 {
		t.Fatal("ActionNames() returned empty slice")
	}
}

func TestActionNames_Sorted(t *testing.T) {
	names := ActionNames()
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Errorf("ActionNames not sorted: %q >= %q at index %d", names[i-1], names[i], i)
		}
	}
}

func TestActionNames_ContainsExpected(t *testing.T) {
	names := ActionNames()
	expected := []string{
		"global.quit",
		"global.help",
		"nav.up",
		"nav.down",
		"status.stage",
		"commit.revert",
		"diff.nextHunk",
		"stash.pop",
		"remote.push",
		"branch.checkout",
	}
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for _, e := range expected {
		if !nameSet[e] {
			t.Errorf("ActionNames() missing expected action: %q", e)
		}
	}
}

func TestApplyOverrides_ChangesKey(t *testing.T) {
	// Reset singletons to defaults first.
	Navigation = NewNavigationKeys()

	original := Navigation.Up.Keys()
	if len(original) == 0 {
		t.Fatal("Navigation.Up has no keys initially")
	}

	ApplyOverrides(map[string]string{
		"nav.up": "w",
	})

	keys := Navigation.Up.Keys()
	if len(keys) != 1 || keys[0] != "w" {
		t.Errorf("After override, nav.up keys = %v, want [w]", keys)
	}

	// Restore defaults.
	Navigation = NewNavigationKeys()
}

func TestApplyOverrides_MultipleKeys(t *testing.T) {
	Navigation = NewNavigationKeys()

	ApplyOverrides(map[string]string{
		"nav.down": "s down",
	})

	keys := Navigation.Down.Keys()
	if len(keys) != 2 || keys[0] != "s" || keys[1] != "down" {
		t.Errorf("After override, nav.down keys = %v, want [s down]", keys)
	}

	// Restore defaults.
	Navigation = NewNavigationKeys()
}

func TestApplyOverrides_PreservesHelpDesc(t *testing.T) {
	Navigation = NewNavigationKeys()
	desc := Navigation.Up.Help().Desc

	ApplyOverrides(map[string]string{
		"nav.up": "w",
	})

	if Navigation.Up.Help().Desc != desc {
		t.Errorf("Help desc changed: got %q, want %q", Navigation.Up.Help().Desc, desc)
	}

	Navigation = NewNavigationKeys()
}

func TestApplyOverrides_UnknownAction(t *testing.T) {
	// Should not panic.
	ApplyOverrides(map[string]string{
		"nonexistent.action": "x",
	})
}

func TestApplyOverrides_EmptyKey(t *testing.T) {
	Navigation = NewNavigationKeys()
	original := Navigation.Up.Keys()

	ApplyOverrides(map[string]string{
		"nav.up": "",
	})

	// Empty key should be a no-op.
	keys := Navigation.Up.Keys()
	if len(keys) != len(original) {
		t.Errorf("Empty key override changed keys: got %v, want %v", keys, original)
	}

	Navigation = NewNavigationKeys()
}

func TestSplitKeys(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"k", []string{"k"}},
		{"up k", []string{"up", "k"}},
		{"ctrl+c", []string{"ctrl+c"}},
		{"ctrl+shift+a up", []string{"ctrl+shift+a", "up"}},
		{"", nil},
		{"  a  b  ", []string{"a", "b"}},
	}
	for _, tt := range tests {
		got := splitKeys(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitKeys(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitKeys(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestShortHelp_AllContexts(t *testing.T) {
	contexts := []Context{
		ContextGlobal, ContextStatus, ContextLog, ContextBranches,
		ContextDiff, ContextStash, ContextRemotes, ContextPR,
		ContextCI, ContextWorkspace, ContextDialog,
	}
	for _, ctx := range contexts {
		bindings := ShortHelp(ctx)
		if len(bindings) == 0 {
			t.Errorf("ShortHelp(%d) returned empty", ctx)
		}
	}
}

func TestFullHelp_AllContexts(t *testing.T) {
	contexts := []Context{
		ContextGlobal, ContextStatus, ContextLog, ContextBranches,
		ContextDiff, ContextStash, ContextRemotes, ContextPR,
		ContextCI, ContextWorkspace,
	}
	for _, ctx := range contexts {
		groups := FullHelp(ctx)
		if len(groups) == 0 {
			t.Errorf("FullHelp(%d) returned empty", ctx)
		}
		// Each group should have at least one binding.
		for i, g := range groups {
			if len(g) == 0 {
				t.Errorf("FullHelp(%d) group %d is empty", ctx, i)
			}
		}
	}
}
