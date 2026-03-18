package git

import (
	"os"
	"path/filepath"
	"strings"
)

// BisectStart begins a bisect session.
func (r *Repository) BisectStart() error {
	_, err := r.run("bisect", "start")
	return err
}

// BisectBad marks the given ref (or HEAD if empty) as bad.
func (r *Repository) BisectBad(ref string) (string, error) {
	args := []string{"bisect", "bad"}
	if ref != "" {
		args = append(args, ref)
	}
	return r.run(args...)
}

// BisectGood marks the given ref (or HEAD if empty) as good.
func (r *Repository) BisectGood(ref string) (string, error) {
	args := []string{"bisect", "good"}
	if ref != "" {
		args = append(args, ref)
	}
	return r.run(args...)
}

// BisectSkip skips the current commit.
func (r *Repository) BisectSkip() (string, error) {
	return r.run("bisect", "skip")
}

// BisectReset ends the bisect session and returns to the original branch.
func (r *Repository) BisectReset() error {
	_, err := r.run("bisect", "reset")
	return err
}

// BisectLog returns the bisect log output.
func (r *Repository) BisectLog() (string, error) {
	return r.run("bisect", "log")
}

// IsBisecting returns true if a bisect session is in progress.
// Uses filesystem check (fast, no git subprocess).
func (r *Repository) IsBisecting() bool {
	_, err := os.Stat(filepath.Join(r.gitDir, "BISECT_LOG"))
	return err == nil
}

// BisectVisualize returns a short log of remaining suspects.
func (r *Repository) BisectVisualize() (string, error) {
	return r.run("bisect", "visualize", "--oneline")
}

// BisectState represents the current state of a bisect session.
type BisectState struct {
	Active  bool
	Good    []string // hashes marked as good
	Bad     string   // hash marked as bad (only one)
	Current string   // current test commit hash
}

// GetBisectState returns the current bisect state by parsing the bisect log.
func (r *Repository) GetBisectState() BisectState {
	state := BisectState{}
	logOut, err := r.run("bisect", "log")
	if err != nil {
		return state
	}
	state.Active = true
	for _, line := range strings.Split(logOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# bad:") {
			// Extract hash: "# bad: [hash] ..."
			if parts := strings.Fields(line); len(parts) >= 3 {
				h := strings.TrimPrefix(parts[2], "[")
				h = strings.TrimSuffix(h, "]")
				state.Bad = h
			}
		}
		if strings.HasPrefix(line, "# good:") {
			if parts := strings.Fields(line); len(parts) >= 3 {
				h := strings.TrimPrefix(parts[2], "[")
				h = strings.TrimSuffix(h, "]")
				state.Good = append(state.Good, h)
			}
		}
	}
	// Get current HEAD as the test commit
	if head, err := r.Head(); err == nil {
		state.Current = head
	}
	return state
}
