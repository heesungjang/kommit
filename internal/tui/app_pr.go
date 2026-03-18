package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heesungjang/kommit/internal/auth"
	"github.com/heesungjang/kommit/internal/hosting"
	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/dialog"
	"github.com/heesungjang/kommit/internal/tui/pages"
)

// Pull request operations

// loadPullRequests fetches open PRs from the hosting provider in the background.
// It checks the origin remote, resolves the hosting account, and calls the API.
// The result is a SidebarPRsLoadedMsg which the LogPage routes to the sidebar.
func (a App) loadPullRequests() tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		if repo == nil {
			return pages.SidebarPRsLoadedMsg{}
		}

		remoteURL, err := repo.RemoteURL("origin")
		if err != nil || remoteURL == "" {
			return pages.SidebarPRsLoadedMsg{}
		}

		host := auth.HostFromRemoteURL(remoteURL)
		provider := auth.ProviderForHost(host)

		acct := auth.GetAccount(host)
		if acct == nil {
			return pages.SidebarPRsLoadedMsg{}
		}

		ref, err := hosting.RepoRefFromRemoteURL(remoteURL)
		if err != nil {
			return pages.SidebarPRsLoadedMsg{Err: err}
		}

		var prs []hosting.PullRequest
		switch provider {
		case auth.ProviderGitHub:
			client := hosting.NewGitHubClient(acct.Token)
			prs, err = client.ListPullRequests(ref, "open")
		case auth.ProviderGitLab:
			client := hosting.NewGitLabClientWithHost(acct.Token, host)
			prs, err = client.ListPullRequests(ref, "open")
		default:
			return pages.SidebarPRsLoadedMsg{}
		}
		return pages.SidebarPRsLoadedMsg{PRs: prs, Err: err}
	}
}

// openCreatePRDialog opens the Create PR dialog pre-filled with the current
// branch, remote branches, and the repository's default branch. It also kicks
// off async loading of commit/diff stats and remote push status.
func (a App) openCreatePRDialog() tea.Cmd {
	repo := a.repo
	pctx := a.ctx
	return func() tea.Msg {
		headBranch, err := repo.Head()
		if err != nil {
			return pages.PRCreateErrorMsg{Err: fmt.Errorf("cannot determine current branch: %w", err)}
		}

		// Determine the default branch (target for the PR).
		baseBranch := "main" // fallback
		remoteURL, err := repo.RemoteURL("origin")
		if err == nil && remoteURL != "" {
			host := auth.HostFromRemoteURL(remoteURL)
			provider := auth.ProviderForHost(host)
			acct := auth.GetAccount(host)
			if acct != nil {
				ref, refErr := hosting.RepoRefFromRemoteURL(remoteURL)
				if refErr == nil {
					switch provider {
					case auth.ProviderGitHub:
						client := hosting.NewGitHubClient(acct.Token)
						if defaultBranch, dbErr := client.GetDefaultBranch(ref); dbErr == nil {
							baseBranch = defaultBranch
						}
					case auth.ProviderGitLab:
						client := hosting.NewGitLabClientWithHost(acct.Token, host)
						if defaultBranch, dbErr := client.GetDefaultBranch(ref); dbErr == nil {
							baseBranch = defaultBranch
						}
					}
				}
			}
		}

		// Gather remote branch names (strip "origin/" prefix).
		var branchNames []string
		remoteBranches, rbErr := repo.RemoteBranches()
		if rbErr == nil {
			for _, b := range remoteBranches {
				name := strings.TrimPrefix(b.Name, "origin/")
				if name == "HEAD" {
					continue
				}
				branchNames = append(branchNames, name)
			}
		}

		return showCreatePRDialogMsg{
			headBranch:     headBranch,
			baseBranch:     baseBranch,
			remoteBranches: branchNames,
			pctx:           pctx,
		}
	}
}

// showCreatePRDialogMsg carries all data needed to open the PR dialog and
// trigger async stat loading.
type showCreatePRDialogMsg struct {
	headBranch     string
	baseBranch     string
	remoteBranches []string
	pctx           *tuictx.ProgramContext
}

// loadPRStats loads commit count and diff stats between the base branch and HEAD.
func (a App) loadPRStats(baseBranch string) tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		commitCount, err := repo.RevListCount(baseBranch, "HEAD")
		if err != nil {
			commitCount = 0 // non-fatal: show zero if base branch doesn't exist yet
		}
		entries, err := repo.DiffStatBranch(baseBranch, "HEAD")
		if err != nil {
			entries = nil // non-fatal: show empty stats
		}

		filesChanged := len(entries)
		var additions, deletions int
		for _, e := range entries {
			additions += e.Added
			deletions += e.Removed
		}

		return dialog.CreatePRStatsMsg{
			CommitCount:  commitCount,
			FilesChanged: filesChanged,
			Additions:    additions,
			Deletions:    deletions,
		}
	}
}

// checkBranchPushed checks if the head branch exists on the remote.
func (a App) checkBranchPushed() tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		head, err := repo.Head()
		if err != nil {
			return dialog.CreatePRBranchPushedMsg{Pushed: false}
		}
		pushed := repo.HasRemoteBranch("origin", head)
		return dialog.CreatePRBranchPushedMsg{Pushed: pushed}
	}
}

// pushHeadBranch pushes the current HEAD branch to origin using auth.
func (a App) pushHeadBranch() tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		head, err := repo.Head()
		if err != nil {
			return dialog.CreatePRPushDoneMsg{Err: fmt.Errorf("cannot determine branch: %w", err)}
		}

		remoteURL, err := repo.RemoteURL("origin")
		if err != nil || remoteURL == "" {
			return dialog.CreatePRPushDoneMsg{Err: fmt.Errorf("no origin remote")}
		}

		host := auth.HostFromRemoteURL(remoteURL)
		acct := auth.GetAccount(host)
		if acct == nil {
			return dialog.CreatePRPushDoneMsg{Err: fmt.Errorf("not logged in")}
		}

		err = repo.PushSetUpstreamAuth("origin", head, acct.Username, acct.Token)
		if err != nil {
			return dialog.CreatePRPushDoneMsg{Err: err}
		}
		return dialog.CreatePRPushDoneMsg{}
	}
}

// createPullRequest calls the hosting API to create a pull request or merge request.
func (a App) createPullRequest(msg dialog.CreatePRSubmitMsg) tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		remoteURL, err := repo.RemoteURL("origin")
		if err != nil || remoteURL == "" {
			return pages.PRCreateErrorMsg{Err: fmt.Errorf("no origin remote configured")}
		}

		host := auth.HostFromRemoteURL(remoteURL)
		provider := auth.ProviderForHost(host)
		acct := auth.GetAccount(host)
		if acct == nil {
			return pages.PRCreateErrorMsg{Err: fmt.Errorf("not logged in — log in via Settings > Accounts")}
		}

		ref, err := hosting.RepoRefFromRemoteURL(remoteURL)
		if err != nil {
			return pages.PRCreateErrorMsg{Err: err}
		}

		prReq := hosting.CreatePRRequest{
			Title: msg.Title,
			Body:  msg.Body,
			Head:  msg.Head,
			Base:  msg.Base,
			Draft: msg.Draft,
		}

		var pr *hosting.PullRequest
		switch provider {
		case auth.ProviderGitHub:
			client := hosting.NewGitHubClient(acct.Token)
			pr, err = client.CreatePullRequest(ref, prReq)
		case auth.ProviderGitLab:
			client := hosting.NewGitLabClientWithHost(acct.Token, host)
			pr, err = client.CreatePullRequest(ref, prReq)
		default:
			return pages.PRCreateErrorMsg{Err: fmt.Errorf("PR creation not supported for %s", host)}
		}

		if err != nil {
			return pages.PRCreateErrorMsg{Err: err}
		}

		return pages.PRCreatedMsg{Number: pr.Number, URL: pr.URL}
	}
}
