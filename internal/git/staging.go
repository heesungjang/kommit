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
