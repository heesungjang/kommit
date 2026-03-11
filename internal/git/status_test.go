package git

import (
	"testing"
)

func TestFileStatusIsStaged(t *testing.T) {
	tests := []struct {
		name       string
		stagedCode byte
		want       bool
	}{
		{"modified staged", 'M', true},
		{"added staged", 'A', true},
		{"deleted staged", 'D', true},
		{"renamed staged", 'R', true},
		{"copied staged", 'C', true},
		{"type change staged", 'T', true},
		{"unmerged staged", 'U', true},
		{"space not staged", ' ', false},
		{"dot not staged", '.', false},
		{"untracked not staged", '?', false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := FileStatus{StagedCode: tc.stagedCode}
			if got := f.IsStaged(); got != tc.want {
				t.Errorf("IsStaged() = %v, want %v (code=%c)", got, tc.want, tc.stagedCode)
			}
		})
	}
}

func TestFileStatusIsUnstaged(t *testing.T) {
	tests := []struct {
		name         string
		unstagedCode byte
		want         bool
	}{
		{"modified unstaged", 'M', true},
		{"added unstaged", 'A', true},
		{"deleted unstaged", 'D', true},
		{"space not unstaged", ' ', false},
		{"dot not unstaged", '.', false},
		{"zero not unstaged", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := FileStatus{UnstagedCode: tc.unstagedCode}
			if got := f.IsUnstaged(); got != tc.want {
				t.Errorf("IsUnstaged() = %v, want %v (code=%c)", got, tc.want, tc.unstagedCode)
			}
		})
	}
}

func TestFileStatusIsUntracked(t *testing.T) {
	t.Run("untracked", func(t *testing.T) {
		f := FileStatus{StagedCode: '?', UnstagedCode: '?'}
		if !f.IsUntracked() {
			t.Error("expected IsUntracked() = true")
		}
	})
	t.Run("not untracked", func(t *testing.T) {
		f := FileStatus{StagedCode: 'M', UnstagedCode: '.'}
		if f.IsUntracked() {
			t.Error("expected IsUntracked() = false")
		}
	})
}

func TestFileStatusIsConflict(t *testing.T) {
	tests := []struct {
		name         string
		stagedCode   byte
		unstagedCode byte
		want         bool
	}{
		{"U in staged", 'U', 'M', true},
		{"U in unstaged", 'M', 'U', true},
		{"both A (add/add conflict)", 'A', 'A', true},
		{"both D (delete/delete conflict)", 'D', 'D', true},
		{"no conflict", 'M', '.', false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := FileStatus{StagedCode: tc.stagedCode, UnstagedCode: tc.unstagedCode}
			if got := f.IsConflict(); got != tc.want {
				t.Errorf("IsConflict() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFileStatusStatusIcon(t *testing.T) {
	tests := []struct {
		name         string
		stagedCode   byte
		unstagedCode byte
		want         string
	}{
		{"conflict", 'U', 'U', "!"},
		{"untracked", '?', '?', "?"},
		{"modified staged", 'M', '.', "M"},
		{"added staged", 'A', '.', "A"},
		{"deleted staged", 'D', '.', "D"},
		{"renamed staged", 'R', '.', "R"},
		{"copied staged", 'C', '.', "C"},
		{"type changed staged", 'T', '.', "T"},
		{"modified unstaged", '.', 'M', "M"},
		{"space with modified unstaged", ' ', 'M', "M"},
		{"unknown", '.', '.', " "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := FileStatus{StagedCode: tc.stagedCode, UnstagedCode: tc.unstagedCode}
			if got := f.StatusIcon(); got != tc.want {
				t.Errorf("StatusIcon() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStagedLabel(t *testing.T) {
	f := FileStatus{StagedCode: 'M'}
	if got := f.StagedLabel(); got != "modified" {
		t.Errorf("StagedLabel() = %q, want %q", got, "modified")
	}
}

func TestUnstagedLabel(t *testing.T) {
	f := FileStatus{UnstagedCode: 'D'}
	if got := f.UnstagedLabel(); got != "deleted" {
		t.Errorf("UnstagedLabel() = %q, want %q", got, "deleted")
	}
}

func TestStatusLabel(t *testing.T) {
	tests := []struct {
		code byte
		want string
	}{
		{'M', "modified"},
		{'A', "added"},
		{'D', "deleted"},
		{'R', "renamed"},
		{'C', "copied"},
		{'T', "type changed"},
		{'U', "unmerged"},
		{'?', "untracked"},
		{'!', "ignored"},
		{' ', ""},
		{'.', ""},
	}
	for _, tc := range tests {
		t.Run(string(tc.code), func(t *testing.T) {
			got := statusLabel(tc.code)
			if got != tc.want {
				t.Errorf("statusLabel(%c) = %q, want %q", tc.code, got, tc.want)
			}
		})
	}
}

func TestParseOrdinaryEntry(t *testing.T) {
	t.Run("valid entry", func(t *testing.T) {
		// 1 XY sub mH mI mW hH hI path
		line := "1 M. N... 100644 100644 100644 abc123 def456 src/main.go"
		f := parseOrdinaryEntry(line)
		if f == nil {
			t.Fatal("expected non-nil result")
		}
		if f.Path != "src/main.go" {
			t.Errorf("Path = %q, want %q", f.Path, "src/main.go")
		}
		if f.StagedCode != 'M' {
			t.Errorf("StagedCode = %c, want M", f.StagedCode)
		}
		if f.UnstagedCode != '.' {
			t.Errorf("UnstagedCode = %c, want .", f.UnstagedCode)
		}
	})

	t.Run("invalid entry too few parts", func(t *testing.T) {
		line := "1 M. N... 100644"
		f := parseOrdinaryEntry(line)
		if f != nil {
			t.Errorf("expected nil for invalid entry, got %+v", f)
		}
	})

	t.Run("invalid XY too short", func(t *testing.T) {
		line := "1 M N... 100644 100644 100644 abc123 def456 src/main.go"
		f := parseOrdinaryEntry(line)
		if f != nil {
			t.Errorf("expected nil for short XY, got %+v", f)
		}
	})
}

func TestParseRenameEntry(t *testing.T) {
	t.Run("valid rename entry", func(t *testing.T) {
		// 2 XY sub mH mI mW hH hI Xscore path\torigPath
		line := "2 R. N... 100644 100644 100644 abc123 def456 R100 new/path.go\told/path.go"
		f := parseRenameEntry(line)
		if f == nil {
			t.Fatal("expected non-nil result")
		}
		if f.Path != "new/path.go" {
			t.Errorf("Path = %q, want %q", f.Path, "new/path.go")
		}
		if f.OrigPath != "old/path.go" {
			t.Errorf("OrigPath = %q, want %q", f.OrigPath, "old/path.go")
		}
		if f.StagedCode != 'R' {
			t.Errorf("StagedCode = %c, want R", f.StagedCode)
		}
		if f.UnstagedCode != '.' {
			t.Errorf("UnstagedCode = %c, want .", f.UnstagedCode)
		}
	})

	t.Run("invalid rename entry too few parts", func(t *testing.T) {
		line := "2 R. N... 100644"
		f := parseRenameEntry(line)
		if f != nil {
			t.Errorf("expected nil for invalid entry, got %+v", f)
		}
	})
}

func TestParseUnmergedEntry(t *testing.T) {
	t.Run("valid unmerged entry", func(t *testing.T) {
		// u XY sub m1 m2 m3 mW h1 h2 h3 path
		line := "u UU N... 100644 100644 100644 100644 abc123 def456 ghi789 conflict.go"
		f := parseUnmergedEntry(line)
		if f == nil {
			t.Fatal("expected non-nil result")
		}
		if f.Path != "conflict.go" {
			t.Errorf("Path = %q, want %q", f.Path, "conflict.go")
		}
		if f.StagedCode != 'U' {
			t.Errorf("StagedCode = %c, want U", f.StagedCode)
		}
		if f.UnstagedCode != 'U' {
			t.Errorf("UnstagedCode = %c, want U", f.UnstagedCode)
		}
	})

	t.Run("invalid unmerged entry too few parts", func(t *testing.T) {
		line := "u UU N..."
		f := parseUnmergedEntry(line)
		if f != nil {
			t.Errorf("expected nil for invalid entry, got %+v", f)
		}
	})
}

func TestStatusResultFilters(t *testing.T) {
	result := &StatusResult{
		Files: []FileStatus{
			{Path: "staged.go", StagedCode: 'M', UnstagedCode: '.'},
			{Path: "unstaged.go", StagedCode: '.', UnstagedCode: 'M'},
			{Path: "both.go", StagedCode: 'M', UnstagedCode: 'M'},
			{Path: "untracked.go", StagedCode: '?', UnstagedCode: '?'},
			{Path: "conflict.go", StagedCode: 'U', UnstagedCode: 'U'},
		},
	}

	t.Run("StagedFiles", func(t *testing.T) {
		staged := result.StagedFiles()
		// M., M+M, and U+U (U is staged per IsStaged)
		if len(staged) != 3 {
			t.Errorf("StagedFiles count = %d, want 3", len(staged))
			for _, f := range staged {
				t.Logf("  %s (staged=%c unstaged=%c)", f.Path, f.StagedCode, f.UnstagedCode)
			}
		}
	})

	t.Run("UnstagedFiles", func(t *testing.T) {
		unstaged := result.UnstagedFiles()
		// .M, M+M, ??, U+U
		if len(unstaged) != 4 {
			t.Errorf("UnstagedFiles count = %d, want 4", len(unstaged))
			for _, f := range unstaged {
				t.Logf("  %s (staged=%c unstaged=%c)", f.Path, f.StagedCode, f.UnstagedCode)
			}
		}
	})

	t.Run("ConflictFiles", func(t *testing.T) {
		conflicts := result.ConflictFiles()
		if len(conflicts) != 1 {
			t.Errorf("ConflictFiles count = %d, want 1", len(conflicts))
		}
		if len(conflicts) > 0 && conflicts[0].Path != "conflict.go" {
			t.Errorf("ConflictFiles[0].Path = %q, want %q", conflicts[0].Path, "conflict.go")
		}
	})
}
