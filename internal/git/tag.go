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

	var tags []TagInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
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
