package git

import (
	"strings"
	"time"
)

// CommitInfo represents information about a single commit.
type CommitInfo struct {
	Hash        string
	ShortHash   string
	Author      string
	AuthorEmail string
	Date        time.Time
	Subject     string
	Body        string
	Parents     []string
	Refs        []string
}

// Commit creates a new commit with the given message.
func (r *Repository) Commit(message string) error {
	_, err := r.run("commit", "-m", message)
	return err
}

// CommitAmend amends the last commit with the current staged changes and message.
func (r *Repository) CommitAmend(message string) error {
	if message != "" {
		_, err := r.run("commit", "--amend", "-m", message)
		return err
	}
	_, err := r.run("commit", "--amend", "--no-edit")
	return err
}

// LastCommit returns info about the most recent commit.
func (r *Repository) LastCommit() (*CommitInfo, error) {
	out, err := r.run("log", "-1", "--format=%H%n%h%n%an%n%ae%n%aI%n%s%n%b")
	if err != nil {
		return nil, err
	}
	return parseCommitInfo(out)
}

// CommitBody returns the body text of a commit (everything after the subject line).
// This is fetched separately from the log list to avoid parsing complexity with
// multi-line bodies in the bulk log format.
func (r *Repository) CommitBody(hash string) (string, error) {
	out, err := r.run("log", "-1", "--format=%b", hash)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// RevertCommit reverts the given commit.
func (r *Repository) RevertCommit(hash string) error {
	_, err := r.run("revert", "--no-edit", hash)
	return err
}

// CherryPick cherry-picks the given commit onto the current branch.
func (r *Repository) CherryPick(hash string) error {
	_, err := r.run("cherry-pick", hash)
	return err
}

func parseCommitInfo(out string) (*CommitInfo, error) {
	lines := strings.SplitN(out, "\n", 7)
	if len(lines) < 6 {
		return nil, nil
	}

	date, _ := time.Parse(time.RFC3339, strings.TrimSpace(lines[4]))

	return &CommitInfo{
		Hash:        strings.TrimSpace(lines[0]),
		ShortHash:   strings.TrimSpace(lines[1]),
		Author:      strings.TrimSpace(lines[2]),
		AuthorEmail: strings.TrimSpace(lines[3]),
		Date:        date,
		Subject:     strings.TrimSpace(lines[5]),
		Body: func() string {
			if len(lines) > 6 {
				return strings.TrimSpace(lines[6])
			}
			return ""
		}(),
	}, nil
}
