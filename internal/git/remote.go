package git

import (
	"strings"
)

// RemoteInfo represents a git remote.
type RemoteInfo struct {
	Name     string
	FetchURL string
	PushURL  string
}

// RemoteList returns detailed info about all remotes.
func (r *Repository) RemoteList() ([]RemoteInfo, error) {
	out, err := r.run("remote", "-v")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	remoteMap := make(map[string]*RemoteInfo)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		name := parts[0]
		url := parts[1]
		typ := strings.Trim(parts[2], "()")

		if _, ok := remoteMap[name]; !ok {
			remoteMap[name] = &RemoteInfo{Name: name}
		}
		if typ == "fetch" {
			remoteMap[name].FetchURL = url
		} else if typ == "push" {
			remoteMap[name].PushURL = url
		}
	}

	var remotes []RemoteInfo
	for _, info := range remoteMap {
		remotes = append(remotes, *info)
	}
	return remotes, nil
}

// Push pushes the current branch to the remote.
func (r *Repository) Push(remote, branch string) error {
	args := []string{"push"}
	if remote != "" {
		args = append(args, remote)
	}
	if branch != "" {
		args = append(args, branch)
	}
	_, err := r.run(args...)
	return err
}

// PushSetUpstream pushes and sets the upstream tracking branch.
func (r *Repository) PushSetUpstream(remote, branch string) error {
	_, err := r.run("push", "-u", remote, branch)
	return err
}

// Pull pulls changes from the remote.
func (r *Repository) Pull(remote, branch string) error {
	args := []string{"pull"}
	if remote != "" {
		args = append(args, remote)
	}
	if branch != "" {
		args = append(args, branch)
	}
	_, err := r.run(args...)
	return err
}

// Fetch fetches from all remotes.
func (r *Repository) Fetch() error {
	_, err := r.run("fetch", "--all", "--prune")
	return err
}

// FetchRemote fetches from a specific remote.
func (r *Repository) FetchRemote(remote string) error {
	_, err := r.run("fetch", remote, "--prune")
	return err
}

// AddRemote adds a new remote.
func (r *Repository) AddRemote(name, url string) error {
	_, err := r.run("remote", "add", name, url)
	return err
}

// RemoveRemote removes a remote.
func (r *Repository) RemoveRemote(name string) error {
	_, err := r.run("remote", "remove", name)
	return err
}
