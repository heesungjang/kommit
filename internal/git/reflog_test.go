package git

import (
	"testing"
)

func TestParseReflog(t *testing.T) {
	t.Run("multiple entries", func(t *testing.T) {
		out := "abc123full\x1fabc123\x1fcommit: add new feature\n" +
			"def456full\x1fdef456\x1fcheckout: moving from main to feature\n" +
			"ghi789full\x1fghi789\x1frebase: checkout main"

		entries := parseReflog(out)
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}

		// Entry 1
		if entries[0].Hash != "abc123full" {
			t.Errorf("entries[0].Hash = %q, want %q", entries[0].Hash, "abc123full")
		}
		if entries[0].ShortHash != "abc123" {
			t.Errorf("entries[0].ShortHash = %q, want %q", entries[0].ShortHash, "abc123")
		}
		if entries[0].Action != "commit" {
			t.Errorf("entries[0].Action = %q, want %q", entries[0].Action, "commit")
		}
		if entries[0].Message != "commit: add new feature" {
			t.Errorf("entries[0].Message = %q, want %q", entries[0].Message, "commit: add new feature")
		}

		// Entry 2
		if entries[1].Action != "checkout" {
			t.Errorf("entries[1].Action = %q, want %q", entries[1].Action, "checkout")
		}
		if entries[1].Message != "checkout: moving from main to feature" {
			t.Errorf("entries[1].Message = %q", entries[1].Message)
		}

		// Entry 3
		if entries[2].Action != "rebase" {
			t.Errorf("entries[2].Action = %q, want %q", entries[2].Action, "rebase")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		entries := parseReflog("")
		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("message without colon", func(t *testing.T) {
		out := "aaa\x1faa\x1freset HEAD"
		entries := parseReflog(out)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		// No colon in message, so action == message
		if entries[0].Action != "reset HEAD" {
			t.Errorf("Action = %q, want %q", entries[0].Action, "reset HEAD")
		}
	})
}
