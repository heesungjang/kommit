package git

import (
	"testing"
	"time"
)

func TestParseLog(t *testing.T) {
	t.Run("single commit with all fields", func(t *testing.T) {
		input := "abc123def456\x1fabc123d\x1fJohn Doe\x1fjohn@example.com\x1f2024-01-15T10:30:00+00:00\x1fInitial commit\x1fparent1\x1fHEAD -> main, origin/main, tag: v1.0\x1e"
		commits := parseLog(input)
		if len(commits) != 1 {
			t.Fatalf("expected 1 commit, got %d", len(commits))
		}
		c := commits[0]
		if c.Hash != "abc123def456" {
			t.Errorf("Hash = %q, want %q", c.Hash, "abc123def456")
		}
		if c.ShortHash != "abc123d" {
			t.Errorf("ShortHash = %q, want %q", c.ShortHash, "abc123d")
		}
		if c.Author != "John Doe" {
			t.Errorf("Author = %q, want %q", c.Author, "John Doe")
		}
		if c.AuthorEmail != "john@example.com" {
			t.Errorf("AuthorEmail = %q, want %q", c.AuthorEmail, "john@example.com")
		}
		expectedDate, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00+00:00")
		if !c.Date.Equal(expectedDate) {
			t.Errorf("Date = %v, want %v", c.Date, expectedDate)
		}
		if c.Subject != "Initial commit" {
			t.Errorf("Subject = %q, want %q", c.Subject, "Initial commit")
		}
		if len(c.Parents) != 1 || c.Parents[0] != "parent1" {
			t.Errorf("Parents = %v, want [parent1]", c.Parents)
		}
		if len(c.Refs) != 3 {
			t.Fatalf("Refs = %v, want 3 refs", c.Refs)
		}
		if c.Refs[0] != "HEAD -> main" {
			t.Errorf("Refs[0] = %q, want %q", c.Refs[0], "HEAD -> main")
		}
		if c.Refs[1] != "origin/main" {
			t.Errorf("Refs[1] = %q, want %q", c.Refs[1], "origin/main")
		}
		if c.Refs[2] != "tag: v1.0" {
			t.Errorf("Refs[2] = %q, want %q", c.Refs[2], "tag: v1.0")
		}
	})

	t.Run("multiple commits", func(t *testing.T) {
		input := "hash1\x1fh1\x1fAlice\x1falice@ex.com\x1f2024-01-01T00:00:00Z\x1fFirst\x1f\x1f\x1e" +
			"hash2\x1fh2\x1fBob\x1fbob@ex.com\x1f2024-01-02T00:00:00Z\x1fSecond\x1fhash1\x1f\x1e"
		commits := parseLog(input)
		if len(commits) != 2 {
			t.Fatalf("expected 2 commits, got %d", len(commits))
		}
		if commits[0].Hash != "hash1" {
			t.Errorf("commits[0].Hash = %q, want %q", commits[0].Hash, "hash1")
		}
		if commits[1].Hash != "hash2" {
			t.Errorf("commits[1].Hash = %q, want %q", commits[1].Hash, "hash2")
		}
		if len(commits[0].Parents) != 0 {
			t.Errorf("commits[0].Parents = %v, want empty", commits[0].Parents)
		}
		if len(commits[1].Parents) != 1 || commits[1].Parents[0] != "hash1" {
			t.Errorf("commits[1].Parents = %v, want [hash1]", commits[1].Parents)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		commits := parseLog("")
		if len(commits) != 0 {
			t.Errorf("expected 0 commits, got %d", len(commits))
		}
	})

	t.Run("root commit no parents", func(t *testing.T) {
		input := "roothash\x1froot\x1fDev\x1fdev@ex.com\x1f2024-06-01T12:00:00Z\x1fRoot commit\x1f\x1f\x1e"
		commits := parseLog(input)
		if len(commits) != 1 {
			t.Fatalf("expected 1 commit, got %d", len(commits))
		}
		if len(commits[0].Parents) != 0 {
			t.Errorf("Parents = %v, want empty for root commit", commits[0].Parents)
		}
	})

	t.Run("merge commit with multiple parents", func(t *testing.T) {
		input := "mergehash\x1fmerge\x1fDev\x1fdev@ex.com\x1f2024-06-01T12:00:00Z\x1fMerge branch\x1fparentA parentB\x1f\x1e"
		commits := parseLog(input)
		if len(commits) != 1 {
			t.Fatalf("expected 1 commit, got %d", len(commits))
		}
		if len(commits[0].Parents) != 2 {
			t.Fatalf("Parents = %v, want 2 parents", commits[0].Parents)
		}
		if commits[0].Parents[0] != "parentA" {
			t.Errorf("Parents[0] = %q, want %q", commits[0].Parents[0], "parentA")
		}
		if commits[0].Parents[1] != "parentB" {
			t.Errorf("Parents[1] = %q, want %q", commits[0].Parents[1], "parentB")
		}
	})

	t.Run("commit with refs containing HEAD and tags", func(t *testing.T) {
		input := "hashX\x1fhX\x1fDev\x1fdev@ex.com\x1f2024-06-01T12:00:00Z\x1fTagged\x1fparent1\x1fHEAD -> feature, origin/feature, tag: v2.0\x1e"
		commits := parseLog(input)
		if len(commits) != 1 {
			t.Fatalf("expected 1 commit, got %d", len(commits))
		}
		refs := commits[0].Refs
		if len(refs) != 3 {
			t.Fatalf("Refs = %v, want 3 refs", refs)
		}
		if refs[0] != "HEAD -> feature" {
			t.Errorf("Refs[0] = %q, want %q", refs[0], "HEAD -> feature")
		}
		if refs[1] != "origin/feature" {
			t.Errorf("Refs[1] = %q, want %q", refs[1], "origin/feature")
		}
		if refs[2] != "tag: v2.0" {
			t.Errorf("Refs[2] = %q, want %q", refs[2], "tag: v2.0")
		}
	})
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{100, "100"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := formatInt(tc.input)
			if got != tc.want {
				t.Errorf("formatInt(%d) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
