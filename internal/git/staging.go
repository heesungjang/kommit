package git

import (
	"fmt"
	"strings"
)

// StageFile adds a file to the staging area.
func (r *Repository) StageFile(path string) error {
	_, err := r.run("add", "--", path)
	return err
}

// StageAll stages all changes.
func (r *Repository) StageAll() error {
	_, err := r.run("add", "-A")
	return err
}

// UnstageFile removes a file from the staging area.
func (r *Repository) UnstageFile(path string) error {
	_, err := r.run("reset", "HEAD", "--", path)
	return err
}

// UnstageAll removes all files from the staging area.
func (r *Repository) UnstageAll() error {
	_, err := r.run("reset", "HEAD")
	return err
}

// DiscardFile discards unstaged changes in a file.
func (r *Repository) DiscardFile(path string) error {
	_, err := r.run("checkout", "--", path)
	return err
}

// DiscardAll discards all unstaged changes.
func (r *Repository) DiscardAll() error {
	_, err := r.run("checkout", "--", ".")
	return err
}

// CleanFile removes an untracked file from the working tree.
func (r *Repository) CleanFile(path string) error {
	_, err := r.run("clean", "-f", "--", path)
	return err
}

// Hunk represents a diff hunk that can be staged.
type Hunk struct {
	Header   string   // @@ line
	StartOld int      // Starting line in old file
	CountOld int      // Line count in old file
	StartNew int      // Starting line in new file
	CountNew int      // Line count in new file
	Lines    []string // All lines including context
}

// StageHunk stages a specific diff hunk for a file.
func (r *Repository) StageHunk(path string, hunk Hunk) error {
	patch := buildPatch(path, path, hunk)
	_, err := r.runWithStdin(patch, "apply", "--cached", "--unidiff-zero", "-")
	return err
}

// UnstageHunk unstages a specific diff hunk.
func (r *Repository) UnstageHunk(path string, hunk Hunk) error {
	patch := buildPatch(path, path, hunk)
	_, err := r.runWithStdin(patch, "apply", "--cached", "--reverse", "--unidiff-zero", "-")
	return err
}

func buildPatch(oldPath, newPath string, hunk Hunk) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- a/%s\n", oldPath))
	b.WriteString(fmt.Sprintf("+++ b/%s\n", newPath))
	b.WriteString(hunk.Header + "\n")
	for _, line := range hunk.Lines {
		b.WriteString(line + "\n")
	}
	return b.String()
}

// StageLines stages only the selected lines from a hunk.
// selectedIndices is a set of line indices within hunk.Lines that the user selected.
// For staging from unstaged diff:
//   - Selected "+" lines are included as "+" (added)
//   - Unselected "+" lines are dropped (treated as not yet added)
//   - All "-" lines are included as "-" (removed)
//   - Context lines are always included
func (r *Repository) StageLines(path string, hunk Hunk, selectedIndices map[int]bool) error {
	partial := buildPartialPatch(path, hunk, selectedIndices, false)
	if partial == "" {
		return nil // nothing to stage
	}
	_, err := r.runWithStdin(partial, "apply", "--cached", "--unidiff-zero", "-")
	return err
}

// UnstageLines unstages only the selected lines from a staged hunk.
// For unstaging from staged diff:
//   - Selected "-" lines are included (will be un-removed)
//   - Unselected "-" lines are dropped
//   - All "+" lines are included
//   - Context lines are always included
func (r *Repository) UnstageLines(path string, hunk Hunk, selectedIndices map[int]bool) error {
	partial := buildPartialPatch(path, hunk, selectedIndices, true)
	if partial == "" {
		return nil
	}
	_, err := r.runWithStdin(partial, "apply", "--cached", "--reverse", "--unidiff-zero", "-")
	return err
}

// buildPartialPatch constructs a unified diff patch containing only the selected
// add/remove lines from a hunk. Unselected add lines are converted to context
// for staging; unselected remove lines are converted to context for unstaging.
func buildPartialPatch(path string, hunk Hunk, selectedIndices map[int]bool, reverse bool) string {
	var patchLines []string
	oldCount := 0
	newCount := 0

	for i, line := range hunk.Lines {
		if len(line) == 0 {
			// Empty line = context
			patchLines = append(patchLines, " ")
			oldCount++
			newCount++
			continue
		}

		ch := line[0]
		rest := line[1:]

		switch ch {
		case '+':
			if reverse {
				// For unstage (reverse apply): "+" lines are always kept
				patchLines = append(patchLines, line)
				newCount++
			} else {
				// For stage: only include selected "+" lines; drop unselected
				if selectedIndices[i] {
					patchLines = append(patchLines, line)
					newCount++
				} else {
					// Unselected add: skip it (don't stage this addition)
					// Don't add to patch at all
				}
			}
		case '-':
			if reverse {
				// For unstage (reverse apply): only include selected "-" lines
				if selectedIndices[i] {
					patchLines = append(patchLines, line)
					oldCount++
				} else {
					// Unselected remove: convert to context (keep the line)
					patchLines = append(patchLines, " "+rest)
					oldCount++
					newCount++
				}
			} else {
				// For stage: "-" lines are always kept
				patchLines = append(patchLines, line)
				oldCount++
			}
		default:
			// Context line
			patchLines = append(patchLines, line)
			oldCount++
			newCount++
		}
	}

	if len(patchLines) == 0 || (oldCount == 0 && newCount == 0) {
		return ""
	}

	// Build the patch with corrected hunk header
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", hunk.StartOld, oldCount, hunk.StartNew, newCount)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- a/%s\n", path))
	b.WriteString(fmt.Sprintf("+++ b/%s\n", path))
	b.WriteString(header + "\n")
	for _, pl := range patchLines {
		b.WriteString(pl + "\n")
	}
	return b.String()
}
