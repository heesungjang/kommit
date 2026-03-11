package git

import (
	"testing"
	"time"
)

func TestParseCommitInfo(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		out := "abc123def456789\n" +
			"abc123d\n" +
			"John Doe\n" +
			"john@example.com\n" +
			"2024-03-15T14:30:00+00:00\n" +
			"Fix the thing\n"

		info, err := parseCommitInfo(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info == nil {
			t.Fatal("expected non-nil CommitInfo")
		}
		if info.Hash != "abc123def456789" {
			t.Errorf("Hash = %q, want %q", info.Hash, "abc123def456789")
		}
		if info.ShortHash != "abc123d" {
			t.Errorf("ShortHash = %q, want %q", info.ShortHash, "abc123d")
		}
		if info.Author != "John Doe" {
			t.Errorf("Author = %q, want %q", info.Author, "John Doe")
		}
		if info.AuthorEmail != "john@example.com" {
			t.Errorf("AuthorEmail = %q, want %q", info.AuthorEmail, "john@example.com")
		}
		expectedDate, _ := time.Parse(time.RFC3339, "2024-03-15T14:30:00+00:00")
		if !info.Date.Equal(expectedDate) {
			t.Errorf("Date = %v, want %v", info.Date, expectedDate)
		}
		if info.Subject != "Fix the thing" {
			t.Errorf("Subject = %q, want %q", info.Subject, "Fix the thing")
		}
		if info.Body != "" {
			t.Errorf("Body = %q, want empty", info.Body)
		}
	})

	t.Run("short input returns nil", func(t *testing.T) {
		out := "abc123\nabc\nJohn\n"
		info, err := parseCommitInfo(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info != nil {
			t.Errorf("expected nil for short input, got %+v", info)
		}
	})

	t.Run("input with body", func(t *testing.T) {
		out := "abc123def456789\n" +
			"abc123d\n" +
			"Jane Smith\n" +
			"jane@example.com\n" +
			"2024-06-01T09:00:00Z\n" +
			"Add cool feature\n" +
			"This is the body of the commit message.\nWith multiple lines."

		info, err := parseCommitInfo(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info == nil {
			t.Fatal("expected non-nil CommitInfo")
		}
		if info.Subject != "Add cool feature" {
			t.Errorf("Subject = %q, want %q", info.Subject, "Add cool feature")
		}
		if info.Body != "This is the body of the commit message.\nWith multiple lines." {
			t.Errorf("Body = %q", info.Body)
		}
	})

	t.Run("exactly 6 lines", func(t *testing.T) {
		out := "hash\nshort\nauthor\nemail\n2024-01-01T00:00:00Z\nsubject"
		info, err := parseCommitInfo(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info == nil {
			t.Fatal("expected non-nil CommitInfo")
		}
		if info.Body != "" {
			t.Errorf("Body = %q, want empty", info.Body)
		}
	})
}
