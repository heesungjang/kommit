package pages

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/utils"
)

// ---------------------------------------------------------------------------
// Commit operations
// ---------------------------------------------------------------------------

// doRevertCommit runs git revert in the background and refreshes on completion.
func (l LogPage) doRevertCommit(hash string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.RevertCommit(hash)
		if err != nil {
			return RequestToastMsg{Message: "Revert failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "revert"}
	}
}

// doCherryPick runs git cherry-pick in the background and refreshes on completion.
func (l LogPage) doCherryPick(hash string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.CherryPick(hash)
		if err != nil {
			return RequestToastMsg{Message: "Cherry-pick failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "cherry-pick"}
	}
}

// ---------------------------------------------------------------------------
// Reset menu
// ---------------------------------------------------------------------------

func (l LogPage) showResetMenu(_ git.CommitInfo, short string) tea.Cmd {
	return func() tea.Msg {
		return RequestMenuMsg{
			ID:    "reset-menu",
			Title: "Reset to " + short,
			Options: []MenuOption{
				{Label: "Soft reset", Description: "Keep all changes staged", Key: "s"},
				{Label: "Mixed reset", Description: "Keep changes as unstaged", Key: "m"},
				{Label: "Hard reset", Description: "Discard all changes", Key: "h"},
				{Label: "Nuke working tree", Description: "Hard reset HEAD + clean untracked", Key: "n"},
			},
		}
	}
}

func (l LogPage) doResetOp(hash, mode, short string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		var err error
		switch mode {
		case "soft":
			err = repo.ResetSoft(hash)
		case "mixed":
			err = repo.ResetMixed(hash)
		case "hard":
			err = repo.ResetHard(hash)
		}
		if err != nil {
			return RequestToastMsg{Message: "Reset failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "Reset (" + mode + ") to " + short}
	}
}

func (l LogPage) doNukeWorkingTree() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.NukeWorkingTree()
		if err != nil {
			return RequestToastMsg{Message: "Nuke failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "Nuke working tree"}
	}
}

// ---------------------------------------------------------------------------
// Interactive rebase (one-shot actions)
// ---------------------------------------------------------------------------

func (l LogPage) showRebaseConfirm(commit git.CommitInfo, short, action string) tea.Cmd {
	subject := commit.Subject
	actionTitle := strings.ToUpper(action[:1]) + action[1:]
	return func() tea.Msg {
		return RequestConfirmMsg{
			ID:      "rebase-" + action,
			Title:   actionTitle + " Commit?",
			Message: actionTitle + " " + short + " " + subject + "?\n\nThis uses interactive rebase.",
		}
	}
}

func (l LogPage) doRebaseAction(hash, action string) tea.Cmd {
	repo := l.repo
	actionTitle := strings.ToUpper(action[:1]) + action[1:]
	return func() tea.Msg {
		err := repo.RebaseInteractiveAction(hash, action)
		if err != nil {
			return RequestToastMsg{Message: actionTitle + " failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: actionTitle}
	}
}

// ---------------------------------------------------------------------------
// Bisect
// ---------------------------------------------------------------------------

func (l LogPage) showBisectMenu() tea.Cmd {
	bisecting := l.repo.IsBisecting()
	var opts []MenuOption
	if !bisecting {
		opts = []MenuOption{
			{Label: "Start bisect", Description: "Begin a bisect session", Key: "s"},
		}
	} else {
		opts = []MenuOption{
			{Label: "Mark as bad", Description: "Current commit introduces the bug", Key: "b"},
			{Label: "Mark as good", Description: "Current commit is before the bug", Key: "g"},
			{Label: "Skip", Description: "Skip this commit (untestable)", Key: "k"},
			{Label: "Reset bisect", Description: "End bisect session", Key: "r"},
		}
	}
	return func() tea.Msg {
		return RequestMenuMsg{
			ID:      "bisect-menu",
			Title:   "Bisect",
			Options: opts,
		}
	}
}

func (l LogPage) handleBisectMenuResult(idx int) (tea.Model, tea.Cmd) {
	bisecting := l.repo.IsBisecting()
	if !bisecting {
		// Only option: start
		if idx == 0 {
			return l, l.doBisectStart()
		}
		return l, nil
	}
	switch idx {
	case 0: // bad
		return l, l.doBisectMark("bad")
	case 1: // good
		return l, l.doBisectMark("good")
	case 2: // skip
		return l, l.doBisectSkip()
	case 3: // reset
		return l, func() tea.Msg {
			return RequestConfirmMsg{
				ID:      "bisect-reset",
				Title:   "End Bisect?",
				Message: "Reset the bisect session and return to the original branch?",
			}
		}
	}
	return l, nil
}

func (l LogPage) doBisectStart() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.BisectStart()
		if err != nil {
			return RequestToastMsg{Message: "Bisect start failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "Bisect started"}
	}
}

func (l LogPage) doBisectMark(markType string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		var out string
		var err error
		switch markType {
		case "bad":
			out, err = repo.BisectBad("")
		case "good":
			out, err = repo.BisectGood("")
		}
		if err != nil {
			return RequestToastMsg{Message: "Bisect " + markType + " failed: " + err.Error(), IsError: true}
		}
		// Check if bisect found the culprit
		msg := "Marked as " + markType
		if strings.Contains(out, "is the first bad commit") {
			// Extract first line (the culprit hash + subject)
			culprit := out
			if idx := strings.Index(out, " is the first bad commit"); idx > 0 {
				culprit = out[:idx]
				if len(culprit) > 12 {
					culprit = culprit[:12]
				}
			}
			msg = "Bisect complete! Culprit: " + culprit
		}
		return commitOpDoneMsg{op: msg}
	}
}

func (l LogPage) doBisectSkip() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		_, err := repo.BisectSkip()
		if err != nil {
			return RequestToastMsg{Message: "Bisect skip failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "Bisect: skipped commit"}
	}
}

func (l LogPage) doBisectReset() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.BisectReset()
		if err != nil {
			return RequestToastMsg{Message: "Bisect reset failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "Bisect ended"}
	}
}

// ---------------------------------------------------------------------------
// Undo / Redo via reflog
// ---------------------------------------------------------------------------

func (l LogPage) doUndo() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		entries, err := repo.Reflog(20)
		if err != nil || len(entries) < 2 {
			return RequestToastMsg{Message: "Nothing to undo", IsError: true}
		}
		// entries[0] is the current state, entries[1] is the previous state
		target := entries[1]
		return undoTargetMsg{
			hash:      target.Hash,
			shortHash: target.ShortHash,
			message:   target.Message,
		}
	}
}

func (l LogPage) doUndoConfirmed() tea.Cmd {
	hash := l.pendingUndoHash
	if hash == "" {
		return func() tea.Msg {
			return RequestToastMsg{Message: "Nothing to undo", IsError: true}
		}
	}
	short := hash
	if len(short) > 7 {
		short = short[:7]
	}
	return func() tea.Msg {
		return safeResetMsg{hash: hash, short: short, op: "Undo"}
	}
}

func (l LogPage) doRedo() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		// After an undo (reset --hard), the reflog looks like:
		// entries[0] = current state (after undo)
		// entries[1] = the reset operation itself
		// entries[2] = state before undo (what we want to redo to)
		entries, err := repo.Reflog(30)
		if err != nil || len(entries) < 3 {
			return RequestToastMsg{Message: "Nothing to redo", IsError: true}
		}
		if !strings.HasPrefix(entries[1].Action, "reset") {
			return RequestToastMsg{Message: "Nothing to redo", IsError: true}
		}
		target := entries[2]
		return redoTargetMsg{
			hash:      target.Hash,
			shortHash: target.ShortHash,
			message:   target.Message,
		}
	}
}

func (l LogPage) doRedoConfirmed() tea.Cmd {
	hash := l.pendingRedoHash
	if hash == "" {
		return func() tea.Msg {
			return RequestToastMsg{Message: "Nothing to redo", IsError: true}
		}
	}
	short := hash
	if len(short) > 7 {
		short = short[:7]
	}
	return func() tea.Msg {
		return safeResetMsg{hash: hash, short: short, op: "Redo"}
	}
}

// ---------------------------------------------------------------------------
// AI Explain
// ---------------------------------------------------------------------------

// requestAIExplain sends a request to explain the given commit's diff using AI.
func (l LogPage) requestAIExplain(commit git.CommitInfo) tea.Cmd {
	repo := l.repo
	hash := commit.Hash
	subject := commit.Subject
	return func() tea.Msg {
		diff, err := repo.DiffCommitRaw(hash)
		if err != nil {
			return RequestToastMsg{Message: "Failed to get diff: " + err.Error(), IsError: true}
		}
		if strings.TrimSpace(diff) == "" {
			return RequestToastMsg{Message: "No changes to explain", IsError: true}
		}
		return RequestAIExplainMsg{
			Diff:    diff,
			Subject: subject,
		}
	}
}

// ---------------------------------------------------------------------------
// Clipboard
// ---------------------------------------------------------------------------

// copyToClipboard copies text to the system clipboard and shows a toast.
func (l LogPage) copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		if err := utils.CopyToClipboard(text); err != nil {
			return RequestToastMsg{Message: "Copy failed: " + err.Error(), IsError: true}
		}
		short := text
		if len(short) > 7 {
			short = short[:7]
		}
		return RequestToastMsg{Message: "Copied " + short + " to clipboard"}
	}
}
