package pages

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/heesungjang/kommit/internal/git"
)

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (l LogPage) loadWIPDetail() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		status, err := repo.Status()
		if err != nil {
			return wipDetailMsg{err: err}
		}
		// Fetch diff stats for unstaged and staged files (non-fatal if they fail).
		unstagedStats, _ := repo.DiffStat()
		stagedStats, _ := repo.DiffStatStaged()

		// Fill in stats for untracked files (git diff --numstat skips them).
		unstaged := status.UnstagedFiles()
		statMap := make(map[string]bool, len(unstagedStats))
		for _, e := range unstagedStats {
			statMap[e.Path] = true
		}
		for _, f := range unstaged {
			if f.IsUntracked() && !statMap[f.Path] {
				if lines, err := repo.CountFileLines(f.Path); err == nil && lines > 0 {
					unstagedStats = append(unstagedStats, git.DiffStatEntry{
						Path:  f.Path,
						Added: lines,
					})
				}
			}
		}

		return wipDetailMsg{
			unstaged:      unstaged,
			staged:        status.StagedFiles(),
			unstagedStats: unstagedStats,
			stagedStats:   stagedStats,
		}
	}
}

func (l LogPage) wipStageFile(path string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.StageFile(path)
		return wipStageResultMsg{err: err}
	}
}

func (l LogPage) wipUnstageFile(path string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.UnstageFile(path)
		return wipStageResultMsg{err: err}
	}
}

func (l LogPage) wipStageAll() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.StageAll()
		return wipStageResultMsg{err: err}
	}
}

func (l LogPage) wipDiscardFile(path string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.DiscardFile(path)
		return wipStageResultMsg{err: err}
	}
}

func (l LogPage) wipCleanFile(path string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.CleanFile(path)
		return wipStageResultMsg{err: err}
	}
}

// loadAmendPrefill fetches the last commit message and sends it back as an
// amendPrefillMsg so the inline commit textarea can be pre-filled.
func (l LogPage) loadAmendPrefill() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		info, err := repo.LastCommit()
		if err != nil || info == nil {
			return amendPrefillMsg{message: ""}
		}
		msg := info.Subject
		if info.Body != "" {
			msg = info.Subject + "\n\n" + info.Body
		}
		return amendPrefillMsg{message: msg}
	}
}
