package git

import "strings"

// TagInfo represents a git tag.
type TagInfo struct {
	Name string
	Hash string
}

// Tags returns all tags sorted by most recent first.
func (r *Repository) Tags() ([]TagInfo, error) {
	out, err := r.run("tag", "--sort=-creatordate", "--format=%(refname:short)\t%(objectname:short)")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	tags := make([]TagInfo, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		tag := TagInfo{Name: strings.TrimSpace(parts[0])}
		if len(parts) >= 2 {
			tag.Hash = strings.TrimSpace(parts[1])
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// CreateTag creates a lightweight tag at the given commit (or HEAD if hash is empty).
func (r *Repository) CreateTag(name, hash string) error {
	args := []string{"tag", name}
	if hash != "" {
		args = append(args, hash)
	}
	_, err := r.run(args...)
	return err
}

// CreateAnnotatedTag creates an annotated tag with a message.
func (r *Repository) CreateAnnotatedTag(name, hash, message string) error {
	args := []string{"tag", "-a", name, "-m", message}
	if hash != "" {
		args = append(args, hash)
	}
	_, err := r.run(args...)
	return err
}

// DeleteTag deletes a local tag.
func (r *Repository) DeleteTag(name string) error {
	_, err := r.run("tag", "-d", name)
	return err
}
