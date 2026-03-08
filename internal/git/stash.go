package git

import (
	"fmt"
	"strings"
)

// StashEntry represents a single stash entry.
type StashEntry struct {
	Index   int
	Ref     string // e.g., stash@{0}
	Branch  string
	Message string
}

// StashList returns all stash entries.
func (r *Repository) StashList() ([]StashEntry, error) {
	out, err := r.run("stash", "list", "--format=%gd\t%gs")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	var entries []StashEntry
	for i, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		entry := StashEntry{
			Index: i,
			Ref:   strings.TrimSpace(parts[0]),
		}
		if len(parts) >= 2 {
			entry.Message = strings.TrimSpace(parts[1])
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// StashSave creates a new stash entry.
func (r *Repository) StashSave(message string) error {
	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := r.run(args...)
	return err
}

// StashPop pops the top stash entry (or a specific one).
func (r *Repository) StashPop(index int) error {
	ref := fmt.Sprintf("stash@{%d}", index)
	_, err := r.run("stash", "pop", ref)
	return err
}

// StashApply applies a stash entry without removing it.
func (r *Repository) StashApply(index int) error {
	ref := fmt.Sprintf("stash@{%d}", index)
	_, err := r.run("stash", "apply", ref)
	return err
}

// StashDrop removes a stash entry.
func (r *Repository) StashDrop(index int) error {
	ref := fmt.Sprintf("stash@{%d}", index)
	_, err := r.run("stash", "drop", ref)
	return err
}

// StashClear removes all stash entries.
func (r *Repository) StashClear() error {
	_, err := r.run("stash", "clear")
	return err
}

// StashShow returns the diff of a stash entry.
func (r *Repository) StashShow(index int) (string, error) {
	ref := fmt.Sprintf("stash@{%d}", index)
	out, err := r.run("stash", "show", "-p", "--no-color", ref)
	if err != nil {
		return "", err
	}
	return out, nil
}
