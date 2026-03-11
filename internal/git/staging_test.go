package git

import (
	"strings"
	"testing"
)

func TestBuildPatch(t *testing.T) {
	t.Run("simple hunk", func(t *testing.T) {
		hunk := Hunk{
			Header:   "@@ -1,3 +1,4 @@",
			StartOld: 1,
			CountOld: 3,
			StartNew: 1,
			CountNew: 4,
			Lines: []string{
				" context line",
				"+added line",
				" another context",
			},
		}
		patch := buildPatch("file.go", "file.go", hunk)
		if !strings.Contains(patch, "--- a/file.go") {
			t.Error("patch should contain --- a/file.go")
		}
		if !strings.Contains(patch, "+++ b/file.go") {
			t.Error("patch should contain +++ b/file.go")
		}
		if !strings.Contains(patch, "@@ -1,3 +1,4 @@") {
			t.Error("patch should contain hunk header")
		}
		if !strings.Contains(patch, "+added line") {
			t.Error("patch should contain the added line")
		}
		if !strings.Contains(patch, " context line") {
			t.Error("patch should contain the context line")
		}
	})
}

func TestBuildPartialPatch(t *testing.T) {
	hunk := Hunk{
		Header:   "@@ -1,4 +1,4 @@",
		StartOld: 1,
		CountOld: 4,
		StartNew: 1,
		CountNew: 4,
		Lines: []string{
			" context",
			"-removed line 1",
			"-removed line 2",
			"+added line 1",
			"+added line 2",
			" trailing context",
		},
	}

	t.Run("staging forward select one add", func(t *testing.T) {
		// In forward (staging) mode: selected "+" lines are included, unselected are dropped
		// All "-" lines are included
		selected := map[int]bool{3: true} // only "added line 1"
		patch := buildPartialPatch("file.go", hunk, selected, false)
		if patch == "" {
			t.Fatal("expected non-empty patch")
		}
		if !strings.Contains(patch, "--- a/file.go") {
			t.Error("patch should contain --- a/file.go")
		}
		if !strings.Contains(patch, "+++ b/file.go") {
			t.Error("patch should contain +++ b/file.go")
		}
		// Should include +added line 1 but not +added line 2
		if !strings.Contains(patch, "+added line 1") {
			t.Error("patch should contain selected add line")
		}
		if strings.Contains(patch, "+added line 2") {
			t.Error("patch should NOT contain unselected add line")
		}
		// All remove lines should be present
		if !strings.Contains(patch, "-removed line 1") {
			t.Error("patch should contain -removed line 1")
		}
		if !strings.Contains(patch, "-removed line 2") {
			t.Error("patch should contain -removed line 2")
		}
	})

	t.Run("staging forward select all adds", func(t *testing.T) {
		selected := map[int]bool{3: true, 4: true}
		patch := buildPartialPatch("file.go", hunk, selected, false)
		if !strings.Contains(patch, "+added line 1") {
			t.Error("missing +added line 1")
		}
		if !strings.Contains(patch, "+added line 2") {
			t.Error("missing +added line 2")
		}
	})

	t.Run("unstaging reverse select one remove", func(t *testing.T) {
		// In reverse (unstaging) mode: selected "-" lines are included, unselected are context
		// All "+" lines are included
		selected := map[int]bool{1: true} // only "removed line 1"
		patch := buildPartialPatch("file.go", hunk, selected, true)
		if patch == "" {
			t.Fatal("expected non-empty patch")
		}
		// Selected "-" line should remain as "-"
		if !strings.Contains(patch, "-removed line 1") {
			t.Error("patch should contain selected remove line")
		}
		// Unselected "-" line should be converted to context
		if strings.Contains(patch, "-removed line 2") {
			t.Error("unselected remove line should be converted to context")
		}
		// All "+" lines should be present
		if !strings.Contains(patch, "+added line 1") {
			t.Error("patch should contain +added line 1")
		}
		if !strings.Contains(patch, "+added line 2") {
			t.Error("patch should contain +added line 2")
		}
	})

	t.Run("staging with no adds selected returns nothing staged", func(t *testing.T) {
		// No "+" lines selected, all "-" lines are still included
		selected := map[int]bool{}
		patch := buildPartialPatch("file.go", hunk, selected, false)
		// Still has remove lines and context, so patch should not be empty
		if patch == "" {
			t.Fatal("expected non-empty patch (remove lines are always included)")
		}
		// But no add lines
		if strings.Contains(patch, "+added") {
			t.Error("patch should NOT contain any add lines")
		}
	})
}
