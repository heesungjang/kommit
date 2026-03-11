package git

import (
	"strings"
)

// BranchInfo represents a git branch.
type BranchInfo struct {
	Name      string
	Hash      string
	Upstream  string
	IsCurrent bool
	IsRemote  bool
	Subject   string
}

// Branches returns all local branches.
func (r *Repository) Branches() ([]BranchInfo, error) {
	out, err := r.run("branch", "--format=%(HEAD)%(refname:short)\t%(objectname:short)\t%(upstream:short)\t%(subject)")
	if err != nil {
		return nil, err
	}
	return parseBranches(out, false), nil
}

// RemoteBranches returns all remote branches.
func (r *Repository) RemoteBranches() ([]BranchInfo, error) {
	out, err := r.run("branch", "-r", "--format=%(refname:short)\t%(objectname:short)\t\t%(subject)")
	if err != nil {
		return nil, err
	}
	return parseBranches(out, true), nil
}

func parseBranches(out string, remote bool) []BranchInfo {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	branches := make([]BranchInfo, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}

		isCurrent := false
		if !remote && line != "" && line[0] == '*' {
			isCurrent = true
			line = line[1:]
		} else if !remote && line != "" {
			line = line[1:]
		}

		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 2 {
			continue
		}

		b := BranchInfo{
			Name:      strings.TrimSpace(parts[0]),
			Hash:      strings.TrimSpace(parts[1]),
			IsCurrent: isCurrent,
			IsRemote:  remote,
		}
		if len(parts) >= 3 {
			b.Upstream = strings.TrimSpace(parts[2])
		}
		if len(parts) >= 4 {
			b.Subject = strings.TrimSpace(parts[3])
		}
		branches = append(branches, b)
	}
	return branches
}

// CreateBranch creates a new branch at the current HEAD.
func (r *Repository) CreateBranch(name string) error {
	_, err := r.run("branch", name)
	return err
}

// CreateBranchAt creates a new branch at the given ref.
func (r *Repository) CreateBranchAt(name, ref string) error {
	_, err := r.run("branch", name, ref)
	return err
}

// DeleteBranch deletes a local branch.
func (r *Repository) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.run("branch", flag, name)
	return err
}

// RenameBranch renames a branch.
func (r *Repository) RenameBranch(oldName, newName string) error {
	_, err := r.run("branch", "-m", oldName, newName)
	return err
}

// Checkout switches to the given branch or ref.
func (r *Repository) Checkout(ref string) error {
	_, err := r.run("checkout", ref)
	return err
}

// CheckoutNewBranch creates and switches to a new branch.
func (r *Repository) CheckoutNewBranch(name string) error {
	_, err := r.run("checkout", "-b", name)
	return err
}

// Merge merges the given branch into the current branch.
func (r *Repository) Merge(branch string) error {
	_, err := r.run("merge", branch)
	return err
}

// MergeAbort aborts a merge in progress.
func (r *Repository) MergeAbort() error {
	_, err := r.run("merge", "--abort")
	return err
}

// CurrentBranch returns the current branch name.
func (r *Repository) CurrentBranch() (string, error) {
	return r.Head()
}
