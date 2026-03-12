package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repository represents a git repository and provides operations on it.
type Repository struct {
	path   string // Absolute path to the repository root
	gitDir string // Path to .git directory
}

// Open opens a git repository at the given path.
func Open(path string) (*Repository, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	repo := &Repository{path: absPath}

	// Verify this is a git repository and find the root
	root, err := repo.run("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %s", absPath)
	}
	repo.path = strings.TrimSpace(root)

	gitDir, err := repo.run("rev-parse", "--git-dir")
	if err != nil {
		return nil, fmt.Errorf("finding .git directory: %w", err)
	}
	repo.gitDir = strings.TrimSpace(gitDir)
	if !filepath.IsAbs(repo.gitDir) {
		repo.gitDir = filepath.Join(repo.path, repo.gitDir)
	}

	return repo, nil
}

// Path returns the repository root path.
func (r *Repository) Path() string {
	return r.path
}

// run executes a git command and returns stdout.
func (r *Repository) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.path

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Return stdout alongside the error so callers like FileDiffUntracked
		// can inspect output even when the exit code is non-zero (e.g. git
		// diff --no-index exits 1 when files differ).
		return stdout.String(), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), stderr.String(), err)
	}

	return stdout.String(), nil
}

// runWithStdin executes a git command with stdin input.
func (r *Repository) runWithStdin(input string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.path
	cmd.Stdin = strings.NewReader(input)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), stderr.String(), err)
	}

	return nil
}

// runWithEnv executes a git command with additional environment variables.
// The extra env vars are appended to the current process's environment.
func (r *Repository) runWithEnv(env []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.path
	cmd.Env = append(os.Environ(), env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), stderr.String(), err)
	}

	return stdout.String(), nil
}

// RunAuthenticated executes a git command with GIT_ASKPASS configured to
// provide credentials from the given username and token. This is used for
// push/pull/fetch operations that need authentication.
func (r *Repository) RunAuthenticated(username, token string, args ...string) (string, error) {
	if username == "" || token == "" {
		return r.run(args...)
	}

	askpassScript, cleanup, err := createAskpassScript(username, token)
	if err != nil {
		// Fall back to unauthenticated if we can't create the script.
		return r.run(args...)
	}
	defer cleanup()

	env := []string{
		"GIT_ASKPASS=" + askpassScript,
		"GIT_TERMINAL_PROMPT=0",
	}
	return r.runWithEnv(env, args...)
}

// Head returns the current HEAD reference (branch name or commit hash).
func (r *Repository) Head() (string, error) {
	ref, err := r.run("symbolic-ref", "--short", "HEAD")
	if err != nil {
		// Detached HEAD — return commit hash
		hash, err2 := r.run("rev-parse", "--short", "HEAD")
		if err2 != nil {
			return "", fmt.Errorf("getting HEAD: %w", err2)
		}
		return strings.TrimSpace(hash), nil
	}
	return strings.TrimSpace(ref), nil
}

// IsClean returns true if the working tree has no changes.
func (r *Repository) IsClean() (bool, error) {
	out, err := r.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "", nil
}

// RemoteURL returns the URL of the given remote.
func (r *Repository) RemoteURL(name string) (string, error) {
	out, err := r.run("remote", "get-url", name)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Remotes returns the list of remote names.
func (r *Repository) Remotes() ([]string, error) {
	out, err := r.run("remote")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	return strings.Split(strings.TrimSpace(out), "\n"), nil
}

// TrackingBranch returns the upstream tracking branch for the current branch.
func (r *Repository) TrackingBranch() (string, error) {
	out, runErr := r.run("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if runErr != nil {
		return "", nil //nolint:nilerr // No tracking branch is not an error
	}
	return strings.TrimSpace(out), nil
}

// StatusFingerprint returns a lightweight fingerprint of the working tree state.
// It runs `git status --porcelain=v2` and returns the raw output, which can be
// compared against a previous value to detect changes without parsing.
func (r *Repository) StatusFingerprint() string {
	out, err := r.run("status", "--porcelain=v2", "--branch")
	if err != nil {
		return ""
	}
	return out
}

// AheadBehind returns how many commits the current branch is ahead/behind its upstream.
func (r *Repository) AheadBehind() (ahead, behind int, err error) {
	out, runErr := r.run("rev-list", "--left-right", "--count", "HEAD...@{u}")
	if runErr != nil {
		return 0, 0, nil //nolint:nilerr // No upstream is not an error
	}
	parts := strings.Fields(strings.TrimSpace(out))
	if len(parts) == 2 {
		_, _ = fmt.Sscanf(parts[0], "%d", &ahead)
		_, _ = fmt.Sscanf(parts[1], "%d", &behind)
	}
	return ahead, behind, nil
}
