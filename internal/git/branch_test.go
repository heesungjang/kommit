package git

import (
	"testing"
)

func TestParseBranches(t *testing.T) {
	t.Run("local branches with current", func(t *testing.T) {
		// Format: %(HEAD)%(refname:short)\t%(objectname:short)\t%(upstream:short)\t%(subject)
		// Current branch has * prefix, others have space prefix
		out := "*main\tabc1234\torigin/main\tInitial commit\n feature\tdef5678\torigin/feature\tAdd feature"
		branches := parseBranches(out, false)
		if len(branches) != 2 {
			t.Fatalf("expected 2 branches, got %d", len(branches))
		}

		if branches[0].Name != "main" {
			t.Errorf("branches[0].Name = %q, want %q", branches[0].Name, "main")
		}
		if !branches[0].IsCurrent {
			t.Error("branches[0] should be current")
		}
		if branches[0].Hash != "abc1234" {
			t.Errorf("branches[0].Hash = %q, want %q", branches[0].Hash, "abc1234")
		}
		if branches[0].Upstream != "origin/main" {
			t.Errorf("branches[0].Upstream = %q, want %q", branches[0].Upstream, "origin/main")
		}
		if branches[0].Subject != "Initial commit" {
			t.Errorf("branches[0].Subject = %q, want %q", branches[0].Subject, "Initial commit")
		}
		if branches[0].IsRemote {
			t.Error("branches[0] should NOT be remote")
		}

		if branches[1].Name != "feature" {
			t.Errorf("branches[1].Name = %q, want %q", branches[1].Name, "feature")
		}
		if branches[1].IsCurrent {
			t.Error("branches[1] should NOT be current")
		}
	})

	t.Run("remote branches", func(t *testing.T) {
		// Remote branches don't have HEAD indicator prefix
		out := "origin/main\tabc1234\t\tInitial commit\norigin/feature\tdef5678\t\tAdd feature"
		branches := parseBranches(out, true)
		if len(branches) != 2 {
			t.Fatalf("expected 2 branches, got %d", len(branches))
		}

		if branches[0].Name != "origin/main" {
			t.Errorf("branches[0].Name = %q, want %q", branches[0].Name, "origin/main")
		}
		if !branches[0].IsRemote {
			t.Error("branches[0] should be remote")
		}
		if branches[0].IsCurrent {
			t.Error("remote branches should NOT be current")
		}
		if branches[1].Name != "origin/feature" {
			t.Errorf("branches[1].Name = %q, want %q", branches[1].Name, "origin/feature")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		branches := parseBranches("", false)
		if len(branches) != 0 {
			t.Errorf("expected 0 branches, got %d", len(branches))
		}
	})
}
