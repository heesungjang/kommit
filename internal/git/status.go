package git

import (
	"fmt"
	"strings"
)

// FileStatus represents the status of a single file.
type FileStatus struct {
	Path         string
	StagedCode   byte   // Status code in the index (staged)
	UnstagedCode byte   // Status code in the worktree (unstaged)
	OrigPath     string // Original path for renamed files
}

// StatusResult holds the complete status of the working tree.
type StatusResult struct {
	Files    []FileStatus
	Branch   string
	Upstream string
	Ahead    int
	Behind   int
}

// StagedLabel returns a human-readable label for the staged status.
func (f FileStatus) StagedLabel() string {
	return statusLabel(f.StagedCode)
}

// UnstagedLabel returns a human-readable label for the unstaged status.
func (f FileStatus) UnstagedLabel() string {
	return statusLabel(f.UnstagedCode)
}

// IsStaged returns true if this file has staged changes.
// In porcelain v2 format, '.' means "not modified in index" (same as ' ' in v1).
func (f FileStatus) IsStaged() bool {
	return f.StagedCode != ' ' && f.StagedCode != '?' && f.StagedCode != '.'
}

// IsUnstaged returns true if this file has unstaged changes.
// In porcelain v2 format, '.' means "not modified in worktree" (same as ' ' in v1).
func (f FileStatus) IsUnstaged() bool {
	return f.UnstagedCode != ' ' && f.UnstagedCode != 0 && f.UnstagedCode != '.'
}

// IsUntracked returns true if this file is untracked.
func (f FileStatus) IsUntracked() bool {
	return f.StagedCode == '?' && f.UnstagedCode == '?'
}

// IsConflict returns true if this file has merge conflicts.
func (f FileStatus) IsConflict() bool {
	return f.StagedCode == 'U' || f.UnstagedCode == 'U' ||
		(f.StagedCode == 'A' && f.UnstagedCode == 'A') ||
		(f.StagedCode == 'D' && f.UnstagedCode == 'D')
}

// StatusIcon returns a display icon for the file status.
func (f FileStatus) StatusIcon() string {
	if f.IsConflict() {
		return "!"
	}
	if f.IsUntracked() {
		return "?"
	}
	code := f.StagedCode
	if code == ' ' || code == '.' {
		code = f.UnstagedCode
	}
	switch code {
	case 'M':
		return "M"
	case 'A':
		return "A"
	case 'D':
		return "D"
	case 'R':
		return "R"
	case 'C':
		return "C"
	case 'T':
		return "T"
	default:
		return " "
	}
}

func statusLabel(code byte) string {
	switch code {
	case 'M':
		return "modified"
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	case 'T':
		return "type changed"
	case 'U':
		return "unmerged"
	case '?':
		return "untracked"
	case '!':
		return "ignored"
	default:
		return ""
	}
}

// Status returns the current working tree status.
func (r *Repository) Status() (*StatusResult, error) {
	out, err := r.run("status", "--porcelain=v2", "--branch")
	if err != nil {
		return nil, err
	}

	result := &StatusResult{}
	lines := strings.Split(out, "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "# branch.head "):
			result.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.upstream "):
			result.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			ab := strings.TrimPrefix(line, "# branch.ab ")
			parts := strings.Fields(ab)
			if len(parts) >= 2 {
				_, _ = fmt.Sscanf(parts[0], "+%d", &result.Ahead)
				_, _ = fmt.Sscanf(parts[1], "-%d", &result.Behind)
			}
		case strings.HasPrefix(line, "1 "):
			// Ordinary changed entry: 1 XY sub mH mI mW hH hI path
			f := parseOrdinaryEntry(line)
			if f != nil {
				result.Files = append(result.Files, *f)
			}
		case strings.HasPrefix(line, "2 "):
			// Rename/copy entry: 2 XY sub mH mI mW hH hI X\tscore path\torigPath
			f := parseRenameEntry(line)
			if f != nil {
				result.Files = append(result.Files, *f)
			}
		case strings.HasPrefix(line, "u "):
			// Unmerged entry
			f := parseUnmergedEntry(line)
			if f != nil {
				result.Files = append(result.Files, *f)
			}
		case strings.HasPrefix(line, "? "):
			// Untracked file
			path := strings.TrimPrefix(line, "? ")
			result.Files = append(result.Files, FileStatus{
				Path:         path,
				StagedCode:   '?',
				UnstagedCode: '?',
			})
		}
	}

	return result, nil
}

func parseOrdinaryEntry(line string) *FileStatus {
	// 1 XY sub mH mI mW hH hI path
	parts := strings.SplitN(line, " ", 9)
	if len(parts) < 9 {
		return nil
	}
	xy := parts[1]
	if len(xy) < 2 {
		return nil
	}
	return &FileStatus{
		Path:         parts[8],
		StagedCode:   xy[0],
		UnstagedCode: xy[1],
	}
}

func parseRenameEntry(line string) *FileStatus {
	// 2 XY sub mH mI mW hH hI Xscore path\torigPath
	parts := strings.SplitN(line, " ", 10)
	if len(parts) < 10 {
		return nil
	}
	xy := parts[1]
	if len(xy) < 2 {
		return nil
	}
	// path\torigPath
	paths := strings.SplitN(parts[9], "\t", 2)
	f := &FileStatus{
		Path:         paths[0],
		StagedCode:   xy[0],
		UnstagedCode: xy[1],
	}
	if len(paths) == 2 {
		f.OrigPath = paths[1]
	}
	return f
}

func parseUnmergedEntry(line string) *FileStatus {
	// u XY sub m1 m2 m3 mW h1 h2 h3 path
	parts := strings.SplitN(line, " ", 11)
	if len(parts) < 11 {
		return nil
	}
	xy := parts[1]
	if len(xy) < 2 {
		return nil
	}
	return &FileStatus{
		Path:         parts[10],
		StagedCode:   xy[0],
		UnstagedCode: xy[1],
	}
}

// StagedFiles returns only files that have staged changes.
func (r *StatusResult) StagedFiles() []FileStatus {
	var files []FileStatus
	for _, f := range r.Files {
		if f.IsStaged() {
			files = append(files, f)
		}
	}
	return files
}

// UnstagedFiles returns only files that have unstaged changes (including untracked).
func (r *StatusResult) UnstagedFiles() []FileStatus {
	var files []FileStatus
	for _, f := range r.Files {
		if f.IsUnstaged() || f.IsUntracked() {
			files = append(files, f)
		}
	}
	return files
}

// ConflictFiles returns only files with merge conflicts.
func (r *StatusResult) ConflictFiles() []FileStatus {
	var files []FileStatus
	for _, f := range r.Files {
		if f.IsConflict() {
			files = append(files, f)
		}
	}
	return files
}

// IsDirty returns true if there are any tracked changes (staged or unstaged).
// Untracked files alone do not count as dirty.
func (r *Repository) IsDirty() (bool, error) {
	out, err := r.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}
