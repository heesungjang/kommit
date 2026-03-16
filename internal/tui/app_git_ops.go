package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/heesungjang/kommit/internal/auth"
)

// doCommit runs the commit asynchronously.
func (a App) doCommit(message string) tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		err := repo.Commit(message)
		return commitDoneMsg{err: err}
	}
}

// doCommitAmend runs an amend commit asynchronously.
func (a App) doCommitAmend(message string) tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		err := repo.CommitAmend(message)
		return commitDoneMsg{err: err}
	}
}

// doGitOp runs a push/pull/fetch operation asynchronously.
func (a App) doGitOp(op string, force bool) tea.Cmd {
	repo := a.repo

	// Resolve credentials for the origin remote.
	gitUser, gitToken := a.resolveGitCredentials()

	return func() tea.Msg {
		var err error
		switch op {
		case "push":
			// Best-effort fetch to update tracking refs before push.
			// Errors are intentionally ignored — the push itself will
			// produce a clear error if refs are out of date.
			if gitUser != "" {
				_ = repo.FetchAuth(gitUser, gitToken) //nolint:errcheck
			} else {
				_ = repo.Fetch() //nolint:errcheck
			}

			// Auto-detect missing upstream and set it (for both normal and force push).
			branch, brErr := repo.CurrentBranch()
			needsUpstream := false
			if brErr == nil && branch != "" && branch != "HEAD" {
				hasUp, _ := repo.HasUpstream(branch)
				needsUpstream = !hasUp
			}
			if needsUpstream {
				var upErr error
				if gitUser != "" {
					upErr = repo.PushSetUpstreamAuth("origin", branch, gitUser, gitToken)
				} else {
					upErr = repo.PushSetUpstream("origin", branch)
				}
				if upErr == nil {
					return gitOpDoneMsg{op: op, force: force, err: nil}
				}
				// PushSetUpstream failed — fall through to normal/force push.
			}
			if force {
				if gitUser != "" {
					err = repo.ForcePushAuth("", "", gitUser, gitToken)
				} else {
					err = repo.ForcePush("", "")
				}
			} else {
				if gitUser != "" {
					err = repo.PushAuth("", "", gitUser, gitToken)
				} else {
					err = repo.Push("", "")
				}
			}
		case "pull":
			if gitUser != "" {
				err = repo.PullAuth("", "", gitUser, gitToken)
			} else {
				err = repo.Pull("", "")
			}
		case "fetch":
			if gitUser != "" {
				err = repo.FetchAuth(gitUser, gitToken)
			} else {
				err = repo.Fetch()
			}
		}
		return gitOpDoneMsg{op: op, force: force, err: err}
	}
}

// resolveGitCredentials looks up the saved account matching the current repo's
// origin remote and returns the git username and token for authentication.
// Returns empty strings if no matching account is found.
func (a App) resolveGitCredentials() (username, token string) {
	if a.repo == nil {
		return "", ""
	}
	remoteURL, err := a.repo.RemoteURL("origin")
	if err != nil || remoteURL == "" {
		return "", ""
	}
	acct := auth.AccountForRemote(remoteURL)
	if acct == nil || acct.Token == "" {
		return "", ""
	}
	return acct.GitUser, acct.Token
}

// schedulePoll returns a Cmd that sends pollTickMsg after pollInterval.
func (a App) schedulePoll() tea.Cmd {
	return tea.Tick(pollInterval, func(_ time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

// scheduleAutoFetch returns a Cmd that sends autoFetchTickMsg after autoFetchInterval.
func (a App) scheduleAutoFetch() tea.Cmd {
	return tea.Tick(autoFetchInterval, func(_ time.Time) tea.Msg {
		return autoFetchTickMsg{}
	})
}

// doBackgroundFetch runs a silent fetch from all remotes.
func (a App) doBackgroundFetch() tea.Cmd {
	repo := a.repo
	if repo == nil {
		return nil
	}
	gitUser, gitToken := a.resolveGitCredentials()
	return func() tea.Msg {
		var err error
		if gitUser != "" {
			err = repo.FetchAuth(gitUser, gitToken)
		} else {
			err = repo.Fetch()
		}
		return autoFetchDoneMsg{err: err}
	}
}

// checkFingerprint runs a lightweight git status check in the background.
func (a App) checkFingerprint() tea.Cmd {
	repo := a.repo
	if repo == nil {
		return nil
	}
	prev := a.lastFingerprint
	return func() tea.Msg {
		fp := repo.StatusFingerprint()
		return pollResultMsg{
			fingerprint: fp,
			changed:     prev != "" && fp != prev,
		}
	}
}

// capitalize returns s with the first letter uppercased.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// friendlyGitError returns a user-friendly message for common git errors.
// If the error is not recognized, the original message is returned.
func friendlyGitError(op string, err error) string {
	msg := err.Error()
	lower := strings.ToLower(msg)

	switch op {
	case "push":
		switch {
		case strings.Contains(lower, "workflow") && strings.Contains(lower, "scope"):
			return "Push rejected — token lacks 'workflow' scope for CI file changes."
		case strings.Contains(lower, "oauth") && strings.Contains(lower, "scope"):
			return "Push rejected — OAuth token is missing required scopes."
		case strings.Contains(lower, "rejected") &&
			!strings.Contains(lower, "workflow") &&
			!strings.Contains(lower, "scope") &&
			!strings.Contains(lower, "permission"):
			return "Push rejected — remote has new commits. Pull first."
		case strings.Contains(lower, "no upstream"):
			return "No upstream branch configured."
		case strings.Contains(lower, "authentication") || strings.Contains(lower, "permission denied"):
			return "Authentication failed. Check your credentials."
		case strings.Contains(lower, "could not resolve host"):
			return "Cannot reach remote. Check your network connection."
		case strings.Contains(lower, "does not match any"):
			return "Branch not found on remote."
		}
	case "pull":
		switch {
		case strings.Contains(lower, "not possible because you have unmerged"):
			return "Pull failed — resolve merge conflicts first."
		case strings.Contains(lower, "uncommitted changes"):
			return "Pull failed — commit or stash your changes first."
		case strings.Contains(lower, "authentication") || strings.Contains(lower, "permission denied"):
			return "Authentication failed. Check your credentials."
		case strings.Contains(lower, "could not resolve host"):
			return "Cannot reach remote. Check your network connection."
		case strings.Contains(lower, "conflict"):
			return "Pull completed with merge conflicts. Resolve them to continue."
		}
	case "fetch":
		switch {
		case strings.Contains(lower, "authentication") || strings.Contains(lower, "permission denied"):
			return "Authentication failed. Check your credentials."
		case strings.Contains(lower, "could not resolve host"):
			return "Cannot reach remote. Check your network connection."
		}
	}
	return msg
}

// isPushRejected returns true if the push error indicates a non-fast-forward
// rejection, which means force push could resolve the issue. It excludes
// permission/auth errors that also contain generic "failed to push" text.
func isPushRejected(err error) bool {
	lower := strings.ToLower(err.Error())
	// Exclude auth/permission errors — these can't be fixed by force push.
	if isAuthError(err) ||
		strings.Contains(lower, "permission") ||
		strings.Contains(lower, "oauth") ||
		strings.Contains(lower, "workflow") ||
		strings.Contains(lower, "scope") {
		return false
	}
	return strings.Contains(lower, "non-fast-forward") ||
		strings.Contains(lower, "fetch first") ||
		(strings.Contains(lower, "rejected") && strings.Contains(lower, "failed to push"))
}

// isAuthError returns true if a git error indicates an authentication failure.
func isAuthError(err error) bool {
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "permission denied") ||
		strings.Contains(lower, "could not read username") ||
		strings.Contains(lower, "invalid credentials") ||
		strings.Contains(lower, "401") ||
		strings.Contains(lower, "403")
}

// hasAccountForOrigin returns true if there's a saved account matching the
// current repo's origin remote.
func (a App) hasAccountForOrigin() bool {
	if a.repo == nil {
		return false
	}
	remoteURL, err := a.repo.RemoteURL("origin")
	if err != nil || remoteURL == "" {
		return false
	}
	return auth.AccountForRemote(remoteURL) != nil
}

// detectOriginProvider detects the hosting provider for the origin remote.
func (a App) detectOriginProvider() auth.HostProvider {
	if a.repo == nil {
		return ""
	}
	remoteURL, err := a.repo.RemoteURL("origin")
	if err != nil || remoteURL == "" {
		return ""
	}
	host := auth.HostFromRemoteURL(remoteURL)
	return auth.ProviderForHost(host)
}

// ---------------------------------------------------------------------------
// Accounts
// ---------------------------------------------------------------------------

// resolveUsername returns the username for the hosting provider matching the
// current repo's origin remote. Returns empty string if no account matches.
func (a App) resolveUsername() string {
	if a.repo == nil {
		return ""
	}
	remoteURL, err := a.repo.RemoteURL("origin")
	if err != nil || remoteURL == "" {
		return ""
	}
	acct := auth.AccountForRemote(remoteURL)
	if acct == nil {
		return ""
	}
	return acct.Username
}
