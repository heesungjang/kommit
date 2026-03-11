package git

import (
	"strings"
	"testing"
)

func TestParseDiff(t *testing.T) {
	t.Run("real looking diff with two files", func(t *testing.T) {
		raw := `diff --git a/file1.go b/file1.go
index abc1234..def5678 100644
--- a/file1.go
+++ b/file1.go
@@ -1,4 +1,5 @@
 package main
 
+import "fmt"
 func main() {
 }
diff --git a/file2.go b/file2.go
new file mode 100644
--- /dev/null
+++ b/file2.go
@@ -0,0 +1,3 @@
+package util
+
+func Helper() {}
`
		result := parseDiff(raw)
		if len(result.Files) != 2 {
			t.Fatalf("expected 2 files, got %d", len(result.Files))
		}
		f1 := result.Files[0]
		if f1.NewPath != "file1.go" {
			t.Errorf("file1 NewPath = %q, want %q", f1.NewPath, "file1.go")
		}
		if f1.Status != "modified" {
			t.Errorf("file1 Status = %q, want %q", f1.Status, "modified")
		}
		if len(f1.Hunks) != 1 {
			t.Fatalf("file1 hunks count = %d, want 1", len(f1.Hunks))
		}

		f2 := result.Files[1]
		if f2.NewPath != "file2.go" {
			t.Errorf("file2 NewPath = %q, want %q", f2.NewPath, "file2.go")
		}
		if f2.Status != "added" {
			t.Errorf("file2 Status = %q, want %q", f2.Status, "added")
		}
	})

	t.Run("empty diff", func(t *testing.T) {
		result := parseDiff("")
		if len(result.Files) != 0 {
			t.Errorf("expected 0 files for empty diff, got %d", len(result.Files))
		}
	})
}

func TestSplitDiffSections(t *testing.T) {
	raw := `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1 +1 @@
-old
+new
diff --git a/b.txt b/b.txt
--- a/b.txt
+++ b/b.txt
@@ -1 +1 @@
-foo
+bar`
	sections := splitDiffSections(raw)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if !strings.HasPrefix(sections[0], "diff --git a/a.txt") {
		t.Errorf("section[0] should start with a.txt diff header")
	}
	if !strings.HasPrefix(sections[1], "diff --git a/b.txt") {
		t.Errorf("section[1] should start with b.txt diff header")
	}
}

func TestParseDiffSection(t *testing.T) {
	t.Run("added file", func(t *testing.T) {
		section := "diff --git a/new.go b/new.go\nnew file mode 100644\n--- /dev/null\n+++ b/new.go\n@@ -0,0 +1,2 @@\n+package new\n+func New() {}"
		file := parseDiffSection(section)
		if file == nil {
			t.Fatal("expected non-nil file")
		}
		if file.Status != "added" {
			t.Errorf("Status = %q, want %q", file.Status, "added")
		}
		if file.NewPath != "new.go" {
			t.Errorf("NewPath = %q, want %q", file.NewPath, "new.go")
		}
	})

	t.Run("deleted file", func(t *testing.T) {
		section := "diff --git a/old.go b/old.go\ndeleted file mode 100644\n--- a/old.go\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-package old\n-func Old() {}"
		file := parseDiffSection(section)
		if file == nil {
			t.Fatal("expected non-nil file")
		}
		if file.Status != "deleted" {
			t.Errorf("Status = %q, want %q", file.Status, "deleted")
		}
	})

	t.Run("modified file", func(t *testing.T) {
		section := "diff --git a/mod.go b/mod.go\nindex abc..def 100644\n--- a/mod.go\n+++ b/mod.go\n@@ -1,3 +1,3 @@\n package mod\n-func Old() {}\n+func New() {}"
		file := parseDiffSection(section)
		if file == nil {
			t.Fatal("expected non-nil file")
		}
		if file.Status != "modified" {
			t.Errorf("Status = %q, want %q", file.Status, "modified")
		}
	})

	t.Run("renamed file", func(t *testing.T) {
		section := "diff --git a/old.go b/new.go\nsimilarity index 95%\nrename from old.go\nrename to new.go\n--- a/old.go\n+++ b/new.go\n@@ -1 +1 @@\n-package old\n+package new"
		file := parseDiffSection(section)
		if file == nil {
			t.Fatal("expected non-nil file")
		}
		if file.Status != "renamed" {
			t.Errorf("Status = %q, want %q", file.Status, "renamed")
		}
	})
}

func TestParseHunkHeader(t *testing.T) {
	t.Run("standard header", func(t *testing.T) {
		hunk := parseHunkHeader("@@ -10,5 +20,8 @@ func foo()")
		if hunk == nil {
			t.Fatal("expected non-nil hunk")
		}
		if hunk.StartOld != 10 {
			t.Errorf("StartOld = %d, want 10", hunk.StartOld)
		}
		if hunk.CountOld != 5 {
			t.Errorf("CountOld = %d, want 5", hunk.CountOld)
		}
		if hunk.StartNew != 20 {
			t.Errorf("StartNew = %d, want 20", hunk.StartNew)
		}
		if hunk.CountNew != 8 {
			t.Errorf("CountNew = %d, want 8", hunk.CountNew)
		}
		if hunk.Header != "@@ -10,5 +20,8 @@ func foo()" {
			t.Errorf("Header = %q", hunk.Header)
		}
	})

	t.Run("no count (single line)", func(t *testing.T) {
		hunk := parseHunkHeader("@@ -1 +1 @@")
		if hunk == nil {
			t.Fatal("expected non-nil hunk")
		}
		if hunk.StartOld != 1 {
			t.Errorf("StartOld = %d, want 1", hunk.StartOld)
		}
		if hunk.CountOld != 1 {
			t.Errorf("CountOld = %d, want 1 (default)", hunk.CountOld)
		}
		if hunk.StartNew != 1 {
			t.Errorf("StartNew = %d, want 1", hunk.StartNew)
		}
		if hunk.CountNew != 1 {
			t.Errorf("CountNew = %d, want 1 (default)", hunk.CountNew)
		}
	})

	t.Run("zero start new file", func(t *testing.T) {
		hunk := parseHunkHeader("@@ -0,0 +1,3 @@")
		if hunk == nil {
			t.Fatal("expected non-nil hunk")
		}
		if hunk.StartOld != 0 {
			t.Errorf("StartOld = %d, want 0", hunk.StartOld)
		}
		if hunk.CountOld != 0 {
			t.Errorf("CountOld = %d, want 0", hunk.CountOld)
		}
		if hunk.StartNew != 1 {
			t.Errorf("StartNew = %d, want 1", hunk.StartNew)
		}
		if hunk.CountNew != 3 {
			t.Errorf("CountNew = %d, want 3", hunk.CountNew)
		}
	})
}

func TestParseRange(t *testing.T) {
	t.Run("start and count", func(t *testing.T) {
		var start, count int
		parseRange("10,5", &start, &count)
		if start != 10 {
			t.Errorf("start = %d, want 10", start)
		}
		if count != 5 {
			t.Errorf("count = %d, want 5", count)
		}
	})

	t.Run("start only defaults count to 1", func(t *testing.T) {
		var start, count int
		parseRange("10", &start, &count)
		if start != 10 {
			t.Errorf("start = %d, want 10", start)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})

	t.Run("zero start", func(t *testing.T) {
		var start, count int
		parseRange("0", &start, &count)
		if start != 0 {
			t.Errorf("start = %d, want 0", start)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})
}

func TestParseDiffStat(t *testing.T) {
	t.Run("multi-line numstat output", func(t *testing.T) {
		input := "10\t5\tfile1.go\n3\t0\tfile2.go\n0\t7\tfile3.go"
		entries := parseDiffStat(input)
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}
		if entries[0].Path != "file1.go" || entries[0].Added != 10 || entries[0].Removed != 5 {
			t.Errorf("entry[0] = %+v", entries[0])
		}
		if entries[1].Path != "file2.go" || entries[1].Added != 3 || entries[1].Removed != 0 {
			t.Errorf("entry[1] = %+v", entries[1])
		}
		if entries[2].Path != "file3.go" || entries[2].Added != 0 || entries[2].Removed != 7 {
			t.Errorf("entry[2] = %+v", entries[2])
		}
	})

	t.Run("empty input", func(t *testing.T) {
		entries := parseDiffStat("")
		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})
}

func TestDiffFileStats(t *testing.T) {
	t.Run("mixed hunks", func(t *testing.T) {
		file := DiffFile{
			Hunks: []Hunk{
				{
					Lines: []string{
						" context line",
						"+added line 1",
						"+added line 2",
						"-removed line 1",
						" another context",
					},
				},
				{
					Lines: []string{
						"+added line 3",
						"-removed line 2",
						"-removed line 3",
					},
				},
			},
		}
		added, removed := file.Stats()
		if added != 3 {
			t.Errorf("added = %d, want 3", added)
		}
		if removed != 3 {
			t.Errorf("removed = %d, want 3", removed)
		}
	})

	t.Run("empty hunks", func(t *testing.T) {
		file := DiffFile{}
		added, removed := file.Stats()
		if added != 0 || removed != 0 {
			t.Errorf("Stats() = (%d, %d), want (0, 0)", added, removed)
		}
	})

	t.Run("hunk with empty lines", func(t *testing.T) {
		file := DiffFile{
			Hunks: []Hunk{
				{
					Lines: []string{
						"",
						"+added",
						"",
					},
				},
			},
		}
		added, removed := file.Stats()
		if added != 1 {
			t.Errorf("added = %d, want 1", added)
		}
		if removed != 0 {
			t.Errorf("removed = %d, want 0", removed)
		}
	})
}
