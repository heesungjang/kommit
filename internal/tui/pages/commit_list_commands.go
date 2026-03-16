package pages

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/heesungjang/kommit/internal/git"
)

// ---------------------------------------------------------------------------
// Commands — loading
// ---------------------------------------------------------------------------

// defaultPageSize is the number of commits loaded per page.
const defaultPageSize = 200

func (l LogPage) pageSize() int {
	if l.logPageSize > 0 {
		return l.logPageSize
	}
	return defaultPageSize
}

func (l LogPage) loadLog() tea.Cmd {
	repo := l.repo
	ps := l.pageSize()
	return func() tea.Msg {
		commits, err := repo.Log(git.LogOptions{MaxCount: ps})
		if err != nil {
			return logLoadedMsg{commits: commits, err: err}
		}

		// Check for uncommitted changes — if dirty, prepend a synthetic WIP entry.
		status, statusErr := repo.Status()
		hasWIP := false
		if statusErr == nil && status != nil {
			hasWIP = len(status.StagedFiles()) > 0 || len(status.UnstagedFiles()) > 0
		}

		if hasWIP {
			wipEntry := git.CommitInfo{
				Hash:      "",
				ShortHash: "●",
				Subject:   "Working Changes",
			}
			commits = append([]git.CommitInfo{wipEntry}, commits...)
		}

		graphRows := git.ComputeGraph(commits)
		return logLoadedMsg{commits: commits, graphRows: graphRows, hasWIP: hasWIP, err: nil}
	}
}

func (l LogPage) loadLogFiltered(query string) tea.Cmd {
	repo := l.repo
	ps := l.pageSize()
	return func() tea.Msg {
		commits, err := repo.Log(git.LogOptions{MaxCount: ps, Grep: query})
		if err != nil {
			return logLoadedMsg{commits: commits, err: err}
		}

		// Check for uncommitted changes — if dirty, prepend a synthetic WIP entry.
		status, statusErr := repo.Status()
		hasWIP := false
		if statusErr == nil && status != nil {
			hasWIP = len(status.StagedFiles()) > 0 || len(status.UnstagedFiles()) > 0
		}

		if hasWIP {
			wipEntry := git.CommitInfo{
				Hash:      "",
				ShortHash: "●",
				Subject:   "Working Changes",
			}
			commits = append([]git.CommitInfo{wipEntry}, commits...)
		}

		graphRows := git.ComputeGraph(commits)
		return logLoadedMsg{commits: commits, graphRows: graphRows, hasWIP: hasWIP, err: nil}
	}
}

// loadLogMore loads the next page of commits and appends them.
func (l LogPage) loadLogMore() tea.Cmd {
	repo := l.repo
	ps := l.pageSize()
	// Skip = total real commits (exclude WIP entry).
	skip := len(l.commits)
	if l.hasWIP {
		skip-- // don't count the synthetic WIP entry
	}
	existing := make([]git.CommitInfo, len(l.commits))
	copy(existing, l.commits)
	hasWIP := l.hasWIP

	return func() tea.Msg {
		more, err := repo.Log(git.LogOptions{MaxCount: ps, Skip: skip})
		if err != nil {
			return logMoreLoadedMsg{err: err}
		}

		// Combine existing + new commits, preserving WIP entry if present.
		combined := existing
		combined = append(combined, more...)
		graphRows := git.ComputeGraph(combined)

		result := logMoreLoadedMsg{
			commits:   more,
			graphRows: graphRows,
			err:       nil,
		}
		// If we fetched fewer than a full page, there's nothing more to load.
		_ = hasWIP
		_ = ps
		return result
	}
}

func (l LogPage) loadCommitDetail(c git.CommitInfo) tea.Cmd {
	repo := l.repo
	compareBase := l.compareBase
	return func() tea.Msg {
		var diff *git.DiffResult
		var err error
		if compareBase != nil {
			// Compare mode: diff between base and selected commit
			diff, err = repo.DiffBranch(compareBase.Hash, c.Hash)
		} else {
			diff, err = repo.DiffCommit(c.Hash)
		}
		// Fetch the commit body separately (not included in bulk log format)
		body, _ := repo.CommitBody(c.Hash)
		c.Body = body
		return commitDetailMsg{commit: c, diff: diff, err: err}
	}
}

// loadCenterDiff determines the appropriate file based on context (WIP
// unstaged/staged or commit detail) and loads its diff to display in the
// center panel. It replaces the old loadWIPSelectedDiff().
func (l LogPage) loadCenterDiff() tea.Cmd {
	repo := l.repo

	// --- WIP context ---
	if l.isWIPSelected() {
		var path string
		var staged bool
		var untracked bool

		switch l.wipFocus {
		case wipFocusUnstaged:
			if len(l.wipUnstaged) > 0 && l.wipUnstagedCursor < len(l.wipUnstaged) {
				f := l.wipUnstaged[l.wipUnstagedCursor]
				path = f.Path
				staged = false
				untracked = f.IsUntracked()
			}
		case wipFocusStaged:
			if len(l.wipStaged) > 0 && l.wipStagedCursor < len(l.wipStaged) {
				path = l.wipStaged[l.wipStagedCursor].Path
				staged = true
			}
		}

		if path == "" {
			return nil
		}

		return func() tea.Msg {
			var diff string
			var err error
			if untracked {
				diff, err = repo.FileDiffUntracked(path)
			} else {
				diff, err = repo.FileDiff(path, staged)
			}
			return centerDiffMsg{path: path, diff: diff, err: err, isWIP: true, isStaged: staged}
		}
	}

	// --- Commit detail context ---
	if l.detailCommit != nil && len(l.detailFiles) > 0 && l.detailFileCursor < len(l.detailFiles) {
		f := l.detailFiles[l.detailFileCursor]
		commitHash := l.detailCommit.Hash
		filePath := f.NewPath
		if f.Status == "deleted" {
			filePath = f.OldPath
		}

		return func() tea.Msg {
			diff, err := repo.DiffCommitFile(commitHash, filePath)
			return centerDiffMsg{path: filePath, diff: diff, err: err}
		}
	}

	return nil
}

func (l LogPage) loadStashDiff(index int) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		diff, err := repo.StashShow(index)
		return stashDiffMsg{index: index, diff: diff, err: err}
	}
}
