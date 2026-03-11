package git

import (
	"strconv"
	"strings"
)

// ReflogEntry represents a single entry in the git reflog.
type ReflogEntry struct {
	Hash      string
	ShortHash string
	Action    string // e.g. "commit", "checkout", "rebase", "merge", "reset", "pull"
	Message   string // full reflog message
}

// Reflog returns the most recent reflog entries.
func (r *Repository) Reflog(limit int) ([]ReflogEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	out, err := r.run("reflog", "--format=%H\x1f%h\x1f%gs", "-n", strconv.Itoa(limit))
	if err != nil {
		return nil, err
	}
	return parseReflog(out), nil
}

func parseReflog(out string) []ReflogEntry {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	entries := make([]ReflogEntry, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 3)
		if len(parts) < 3 {
			continue
		}
		message := parts[2]
		action := message
		// Extract the action keyword before the first colon
		if idx := strings.Index(message, ":"); idx > 0 {
			action = strings.TrimSpace(message[:idx])
		}
		entries = append(entries, ReflogEntry{
			Hash:      parts[0],
			ShortHash: parts[1],
			Action:    action,
			Message:   message,
		})
	}
	return entries
}
