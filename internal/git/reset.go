package git

// ResetSoft resets the current HEAD to the given ref, keeping all changes staged.
func (r *Repository) ResetSoft(ref string) error {
	_, err := r.run("reset", "--soft", ref)
	return err
}

// ResetMixed resets the current HEAD to the given ref, keeping changes as unstaged.
func (r *Repository) ResetMixed(ref string) error {
	_, err := r.run("reset", "--mixed", ref)
	return err
}

// ResetHard resets the current HEAD to the given ref, discarding all changes.
func (r *Repository) ResetHard(ref string) error {
	_, err := r.run("reset", "--hard", ref)
	return err
}

// NukeWorkingTree resets hard to HEAD and removes all untracked files and directories.
func (r *Repository) NukeWorkingTree() error {
	if _, err := r.run("reset", "--hard", "HEAD"); err != nil {
		return err
	}
	_, err := r.run("clean", "-fd")
	return err
}
