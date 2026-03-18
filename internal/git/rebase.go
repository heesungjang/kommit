package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Rebase rebases the current branch onto the given branch.
func (r *Repository) Rebase(branch string) error {
	_, err := r.run("rebase", branch)
	return err
}

// RebaseAbort aborts a rebase in progress.
func (r *Repository) RebaseAbort() error {
	_, err := r.run("rebase", "--abort")
	return err
}

// RebaseContinue continues a rebase after resolving conflicts.
func (r *Repository) RebaseContinue() error {
	_, err := r.run("rebase", "--continue")
	return err
}

// IsRebasing returns true if a rebase is in progress.
// Uses filesystem checks (fast, no git subprocess).
func (r *Repository) IsRebasing() bool {
	if _, err := os.Stat(filepath.Join(r.gitDir, "rebase-merge")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(r.gitDir, "rebase-apply")); err == nil {
		return true
	}
	return false
}

// RebaseInteractiveAction performs a one-shot interactive rebase action on a
// single commit. The action can be "squash", "fixup", "drop", "edit", or "reword".
// It uses a cross-platform temp script as GIT_SEQUENCE_EDITOR.
func (r *Repository) RebaseInteractiveAction(hash, action string) error {
	// Validate action
	switch action {
	case "squash", "fixup", "drop", "edit", "reword":
	default:
		return fmt.Errorf("unsupported rebase action: %s", action)
	}

	// Use short hash prefix for matching — git's todo file may use various
	// abbreviation lengths. We match on the first 7 chars and also try the
	// full hash to be safe.
	shortHash := hash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}

	// Check if this is a root commit (has no parent)
	isRoot := false
	if _, err := r.run("rev-parse", hash+"^"); err != nil {
		isRoot = true
	}

	// Create a cross-platform sequence editor script that replaces
	// "pick <hash>" with "<action> <hash>" in the rebase TODO file.
	seqEditor, cleanup, err := r.makeSequenceEditor(shortHash, action)
	if err != nil {
		return err
	}
	defer cleanup()

	var args []string
	if isRoot {
		args = []string{"rebase", "-i", "--root"}
	} else {
		args = []string{"rebase", "-i", hash + "^"}
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = r.path
	cmd.Env = append(os.Environ(), "GIT_SEQUENCE_EDITOR="+seqEditor)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// makeSequenceEditor creates a temporary script that replaces "pick <hash>"
// with "<action> <hash>" in the rebase TODO file. Returns the editor command
// string and a cleanup function.
func (r *Repository) makeSequenceEditor(shortHash, action string) (editorCmd string, cleanup func(), err error) {
	// Write a temp shell script that does the replacement.
	// This avoids sed -i portability issues between macOS and Linux.
	var ext string
	if runtime.GOOS == "windows" {
		ext = ".bat"
	} else {
		ext = ".sh"
	}

	tmpFile, err := os.CreateTemp("", "opengit-seqedit-*"+ext)
	if err != nil {
		return "", nil, err
	}
	tmpPath := tmpFile.Name()
	cleanup = func() { os.Remove(tmpPath) }

	if runtime.GOOS == "windows" {
		// Windows batch script (basic; interactive rebase on Windows is rare)
		script := fmt.Sprintf(`@echo off
powershell -Command "(Get-Content '%%1') -replace '^pick %s', '%s %s' | Set-Content '%%1'"`,
			shortHash, action, shortHash)
		if _, wErr := tmpFile.WriteString(script); wErr != nil {
			tmpFile.Close()
			cleanup()
			return "", nil, wErr
		}
	} else {
		// Unix shell script — portable across macOS and Linux.
		// Use a temp file for the replacement to avoid sed -i differences.
		script := fmt.Sprintf(`#!/bin/sh
tmpf=$(mktemp)
sed 's/^pick %s/%s %s/' "$1" > "$tmpf"
mv "$tmpf" "$1"
`, shortHash, action, shortHash)
		if _, wErr := tmpFile.WriteString(script); wErr != nil {
			tmpFile.Close()
			cleanup()
			return "", nil, wErr
		}
	}
	tmpFile.Close()
	if chErr := os.Chmod(tmpPath, 0o755); chErr != nil {
		cleanup()
		return "", nil, chErr
	}

	return tmpPath, cleanup, nil
}

// RebaseReword changes the message of a past commit using interactive rebase.
func (r *Repository) RebaseReword(hash, message string) error {
	shortHash := hash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}

	// Check if root commit
	isRoot := false
	if _, err := r.run("rev-parse", hash+"^"); err != nil {
		isRoot = true
	}

	// Create sequence editor to change pick → reword
	seqEditor, seqCleanup, err := r.makeSequenceEditor(shortHash, "reword")
	if err != nil {
		return err
	}
	defer seqCleanup()

	// Create a temp file with the new commit message
	msgFile, err := os.CreateTemp("", "opengit-reword-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(msgFile.Name())
	if _, wErr := msgFile.WriteString(message); wErr != nil {
		msgFile.Close()
		return wErr
	}
	msgFile.Close()

	// Create an editor script that copies our message over the commit message file
	editorScript, err := os.CreateTemp("", "opengit-editor-*.sh")
	if err != nil {
		return err
	}
	defer os.Remove(editorScript.Name())
	fmt.Fprintf(editorScript, "#!/bin/sh\ncp '%s' \"$1\"\n", msgFile.Name())
	editorScript.Close()
	if chErr := os.Chmod(editorScript.Name(), 0o755); chErr != nil {
		return chErr
	}

	var args []string
	if isRoot {
		args = []string{"rebase", "-i", "--root"}
	} else {
		args = []string{"rebase", "-i", hash + "^"}
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = r.path
	cmd.Env = append(os.Environ(),
		"GIT_SEQUENCE_EDITOR="+seqEditor,
		"GIT_EDITOR="+editorScript.Name(),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
