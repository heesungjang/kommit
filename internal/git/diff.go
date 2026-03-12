package git

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DiffFile represents a changed file in a diff.
type DiffFile struct {
	OldPath string
	NewPath string
	Status  string // added, deleted, modified, renamed
	Hunks   []Hunk
	Binary  bool
}

// Stats returns the number of added and removed lines in the file.
func (f DiffFile) Stats() (added, removed int) {
	for _, h := range f.Hunks {
		for _, line := range h.Lines {
			if line == "" {
				continue
			}
			switch line[0] {
			case '+':
				added++
			case '-':
				removed++
			}
		}
	}
	return
}

// DiffResult holds a complete diff output.
type DiffResult struct {
	Files []DiffFile
}

// DiffUnstaged returns the diff of unstaged changes.
func (r *Repository) DiffUnstaged() (*DiffResult, error) {
	out, err := r.run("diff", "--no-color")
	if err != nil {
		return nil, err
	}
	return parseDiff(out), nil
}

// DiffStaged returns the diff of staged changes.
func (r *Repository) DiffStaged() (*DiffResult, error) {
	out, err := r.run("diff", "--cached", "--no-color")
	if err != nil {
		return nil, err
	}
	return parseDiff(out), nil
}

// DiffStagedRaw returns the raw diff output string for staged changes.
// This is useful when the caller needs the unparsed text (e.g. for AI prompts).
func (r *Repository) DiffStagedRaw() (string, error) {
	return r.run("diff", "--cached", "--no-color")
}

// DiffFile returns the diff for a specific file (unstaged).
func (r *Repository) DiffFileUnstaged(path string) (*DiffResult, error) {
	out, err := r.run("diff", "--no-color", "--", path)
	if err != nil {
		return nil, err
	}
	return parseDiff(out), nil
}

// DiffFileStaged returns the diff for a specific file (staged).
func (r *Repository) DiffFileStaged(path string) (*DiffResult, error) {
	out, err := r.run("diff", "--cached", "--no-color", "--", path)
	if err != nil {
		return nil, err
	}
	return parseDiff(out), nil
}

// DiffCommit returns the diff for a specific commit.
func (r *Repository) DiffCommit(hash string) (*DiffResult, error) {
	out, err := r.run("diff", "--no-color", hash+"^!", "--")
	if err != nil {
		// Try for root commit
		out, err = r.run("diff", "--no-color", "--root", hash, "--")
		if err != nil {
			return nil, err
		}
	}
	return parseDiff(out), nil
}

// DiffCommitFile returns the raw diff text for a single file in a commit.
func (r *Repository) DiffCommitFile(hash, path string) (string, error) {
	out, err := r.run("diff", "--no-color", hash+"^!", "--", path)
	if err != nil {
		// Try for root commit
		out, err = r.run("diff", "--no-color", "--root", hash, "--", path)
		if err != nil {
			return "", fmt.Errorf("getting diff for %s in commit %s: %w", path, hash, err)
		}
	}
	return out, nil
}

// DiffBranch returns the diff between two branches/refs.
func (r *Repository) DiffBranch(base, target string) (*DiffResult, error) {
	out, err := r.run("diff", "--no-color", base+"..."+target, "--")
	if err != nil {
		return nil, err
	}
	return parseDiff(out), nil
}

func parseDiff(raw string) *DiffResult {
	result := &DiffResult{}
	if strings.TrimSpace(raw) == "" {
		return result
	}

	sections := splitDiffSections(raw)
	for _, section := range sections {
		file := parseDiffSection(section)
		if file != nil {
			result.Files = append(result.Files, *file)
		}
	}
	return result
}

func splitDiffSections(raw string) []string {
	var sections []string
	lines := strings.Split(raw, "\n")
	var current []string

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			if len(current) > 0 {
				sections = append(sections, strings.Join(current, "\n"))
			}
			current = []string{line}
		} else {
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n"))
	}
	return sections
}

func parseDiffSection(section string) *DiffFile {
	lines := strings.Split(section, "\n")
	if len(lines) == 0 {
		return nil
	}

	file := &DiffFile{}

	i := 0
	// Parse header
	for i < len(lines) {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "diff --git"):
			// Extract paths from "diff --git a/path b/path"
			parts := strings.SplitN(line, " ", 4)
			if len(parts) >= 4 {
				file.OldPath = strings.TrimPrefix(parts[2], "a/")
				file.NewPath = strings.TrimPrefix(parts[3], "b/")
			}
		case strings.HasPrefix(line, "--- "):
			path := strings.TrimPrefix(line, "--- ")
			if path != "/dev/null" {
				file.OldPath = strings.TrimPrefix(path, "a/")
			}
		case strings.HasPrefix(line, "+++ "):
			path := strings.TrimPrefix(line, "+++ ")
			if path != "/dev/null" {
				file.NewPath = strings.TrimPrefix(path, "b/")
			}
		case strings.HasPrefix(line, "new file"):
			file.Status = "added"
		case strings.HasPrefix(line, "deleted file"):
			file.Status = "deleted"
		case strings.HasPrefix(line, "rename from"):
			file.Status = "renamed"
		case strings.HasPrefix(line, "Binary files"):
			file.Binary = true
		case strings.HasPrefix(line, "@@"):
			// Start of hunks — break out of header parsing
			goto parseHunks
		}
		i++
	}

parseHunks:
	if file.Status == "" {
		file.Status = "modified"
	}

	// Parse hunks
	var currentHunk *Hunk
	for ; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				file.Hunks = append(file.Hunks, *currentHunk)
			}
			currentHunk = parseHunkHeader(line)
		} else if currentHunk != nil {
			currentHunk.Lines = append(currentHunk.Lines, line)
		}
	}
	if currentHunk != nil {
		file.Hunks = append(file.Hunks, *currentHunk)
	}

	return file
}

func parseHunkHeader(line string) *Hunk {
	hunk := &Hunk{Header: line}

	// Parse @@ -start,count +start,count @@
	// Find the range info between @@ markers
	if idx := strings.Index(line, "@@"); idx >= 0 {
		rest := line[idx+2:]
		if idx2 := strings.Index(rest, "@@"); idx2 > 0 {
			rangeInfo := strings.TrimSpace(rest[:idx2])
			parts := strings.Fields(rangeInfo)
			for _, part := range parts {
				if strings.HasPrefix(part, "-") {
					parseRange(part[1:], &hunk.StartOld, &hunk.CountOld)
				} else if strings.HasPrefix(part, "+") {
					parseRange(part[1:], &hunk.StartNew, &hunk.CountNew)
				}
			}
		}
	}

	return hunk
}

func parseRange(s string, start, count *int) {
	parts := strings.SplitN(s, ",", 2)
	if len(parts) >= 1 {
		n, err := strconv.Atoi(parts[0])
		if err == nil {
			*start = n
		}
	}
	if len(parts) >= 2 {
		n, err := strconv.Atoi(parts[1])
		if err == nil {
			*count = n
		}
	} else {
		*count = 1
	}
}

// DiffStatEntry represents a single file in diff stat output.
type DiffStatEntry struct {
	Path    string
	Added   int
	Removed int
}

// DiffStat returns file change statistics for unstaged changes.
func (r *Repository) DiffStat() ([]DiffStatEntry, error) {
	out, err := r.run("diff", "--numstat")
	if err != nil {
		return nil, err
	}
	return parseDiffStat(out), nil
}

// DiffStatStaged returns file change statistics for staged changes.
func (r *Repository) DiffStatStaged() ([]DiffStatEntry, error) {
	out, err := r.run("diff", "--cached", "--numstat")
	if err != nil {
		return nil, err
	}
	return parseDiffStat(out), nil
}

// DiffStatStagedRaw returns the raw --stat output for staged changes as a
// human-readable string (used for AI prompts).
func (r *Repository) DiffStatStagedRaw() (string, error) {
	return r.run("diff", "--cached", "--stat")
}

// DiffBranchRaw returns the raw diff between a base ref and HEAD.
// This is used for PR description generation (e.g. diff from main..HEAD).
func (r *Repository) DiffBranchRaw(baseRef string) (string, error) {
	return r.run("diff", "--no-color", baseRef+"...HEAD")
}

// DiffStatBranchRaw returns the raw --stat output between a base ref and HEAD.
func (r *Repository) DiffStatBranchRaw(baseRef string) (string, error) {
	return r.run("diff", "--stat", baseRef+"...HEAD")
}

// LogBranchOneline returns the one-line log of commits between base and HEAD.
func (r *Repository) LogBranchOneline(baseRef string) (string, error) {
	return r.run("log", "--oneline", baseRef+"..HEAD")
}

func parseDiffStat(out string) []DiffStatEntry {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	entries := make([]DiffStatEntry, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		removed, _ := strconv.Atoi(parts[1])
		entries = append(entries, DiffStatEntry{
			Path:    parts[2],
			Added:   added,
			Removed: removed,
		})
	}
	return entries
}

// CountFileLines returns the number of lines in a file relative to the repo root.
// This is used to generate diff stats for untracked files, where all lines are "added".
func (r *Repository) CountFileLines(relPath string) (int, error) {
	fullPath := filepath.Join(r.path, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	count := bytes.Count(data, []byte{'\n'})
	// If the file doesn't end with a newline, count the last line too
	if data[len(data)-1] != '\n' {
		count++
	}
	return count, nil
}

// FileDiff returns the diff for a file, handling both staged and unstaged context.
func (r *Repository) FileDiff(path string, staged bool) (string, error) {
	var args []string
	if staged {
		args = []string{"diff", "--cached", "--no-color", "--", path}
	} else {
		args = []string{"diff", "--no-color", "--", path}
	}
	out, err := r.run(args...)
	if err != nil {
		return "", fmt.Errorf("getting diff for %s: %w", path, err)
	}
	return out, nil
}

// FileDiffUntracked returns the diff for an untracked file by comparing
// /dev/null against the file contents. Regular `git diff` returns nothing
// for untracked files since they have no tracked version.
func (r *Repository) FileDiffUntracked(path string) (string, error) {
	// git diff --no-index exits with status 1 when there are differences,
	// which is the expected case for untracked files. We treat exit code 1
	// as success here.
	out, err := r.run("diff", "--no-index", "--no-color", "--", "/dev/null", path)
	if err != nil {
		// git diff --no-index exits 1 when files differ (expected).
		// Only treat it as an error if there's no output at all.
		if out != "" {
			return out, nil
		}
		return "", fmt.Errorf("getting diff for untracked %s: %w", path, err)
	}
	return out, nil
}
