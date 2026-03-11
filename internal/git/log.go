package git

import (
	"strconv"
	"strings"
	"time"
)

// LogOptions controls how commit log is retrieved.
type LogOptions struct {
	MaxCount int
	Skip     int
	All      bool   // Show all branches
	Author   string // Filter by author
	Since    string // Date filter
	Until    string // Date filter
	Path     string // Filter by file path
	Grep     string // Filter by commit message
}

// Log returns the commit log.
func (r *Repository) Log(opts LogOptions) ([]CommitInfo, error) {
	args := []string{"log", "--format=%H\x1f%h\x1f%an\x1f%ae\x1f%aI\x1f%s\x1f%P\x1f%D\x1e"}

	if opts.MaxCount > 0 {
		args = append(args, "-n", formatInt(opts.MaxCount))
	} else {
		args = append(args, "-n", "200") // Default limit
	}
	if opts.Skip > 0 {
		args = append(args, "--skip", formatInt(opts.Skip))
	}
	if opts.All {
		args = append(args, "--all")
	}
	if opts.Author != "" {
		args = append(args, "--author", opts.Author)
	}
	if opts.Since != "" {
		args = append(args, "--since", opts.Since)
	}
	if opts.Until != "" {
		args = append(args, "--until", opts.Until)
	}
	if opts.Grep != "" {
		args = append(args, "--grep", opts.Grep)
	}
	if opts.Path != "" {
		args = append(args, "--", opts.Path)
	}

	out, err := r.run(args...)
	if err != nil {
		return nil, err
	}

	return parseLog(out), nil
}

func parseLog(out string) []CommitInfo {
	entries := strings.Split(out, "\x1e")
	commits := make([]CommitInfo, 0, len(entries))

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		fields := strings.Split(entry, "\x1f")
		if len(fields) < 6 {
			continue
		}

		date, _ := time.Parse(time.RFC3339, strings.TrimSpace(fields[4]))

		commit := CommitInfo{
			Hash:        strings.TrimSpace(fields[0]),
			ShortHash:   strings.TrimSpace(fields[1]),
			Author:      strings.TrimSpace(fields[2]),
			AuthorEmail: strings.TrimSpace(fields[3]),
			Date:        date,
			Subject:     strings.TrimSpace(fields[5]),
		}

		if len(fields) > 6 {
			parents := strings.TrimSpace(fields[6])
			if parents != "" {
				commit.Parents = strings.Fields(parents)
			}
		}

		if len(fields) > 7 {
			refs := strings.TrimSpace(fields[7])
			if refs != "" {
				for _, ref := range strings.Split(refs, ", ") {
					commit.Refs = append(commit.Refs, strings.TrimSpace(ref))
				}
			}
		}

		commits = append(commits, commit)
	}

	return commits
}

func formatInt(n int) string {
	return strconv.Itoa(n)
}

// LogOneline returns a simplified one-line log.
func (r *Repository) LogOneline(count int) ([]CommitInfo, error) {
	return r.Log(LogOptions{MaxCount: count})
}

// LogForFile returns the commit history for a specific file.
func (r *Repository) LogForFile(path string, count int) ([]CommitInfo, error) {
	return r.Log(LogOptions{MaxCount: count, Path: path})
}

// LogAll returns the commit log for all branches.
func (r *Repository) LogAll(count int) ([]CommitInfo, error) {
	return r.Log(LogOptions{MaxCount: count, All: true})
}
