package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/anim"
	"github.com/heesungjang/kommit/internal/tui/dialog"
	"github.com/heesungjang/kommit/internal/tui/utils"
)

// Update handles messages for the log page.
func (l LogPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height
		l.sidebar = l.sidebar.SetSize(l.sidebarWidth(), msg.Height)
		return l, nil

	case anim.BorderAnimTickMsg:
		still := l.borderAnim.Tick()
		if still {
			return l, anim.ScheduleTick()
		}
		return l, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		l.spinner, cmd = l.spinner.Update(msg)
		if l.aiGenerating {
			l.skeletonTick++
		}
		return l, cmd

	case logLoadedMsg:
		l.loading = false
		l.loadingMore = false
		l.spinner = l.spinner.Stop()
		if msg.err != nil {
			l.err = msg.err
			return l, nil
		}
		l.err = nil // clear any previous error

		prevHadWIP := l.hasWIP
		l.commits = msg.commits
		l.graphRows = msg.graphRows
		l.graphState = msg.graphState
		l.hasWIP = msg.hasWIP
		l.graphScrollX = 0  // reset horizontal scroll on reload
		l.diffCache.Clear() // invalidate cached diffs on full reload
		l.updateGraphMaxWidth()

		// Determine if more commits are available for pagination.
		realCount := len(msg.commits)
		if msg.hasWIP {
			realCount-- // don't count synthetic WIP entry
		}
		l.canLoadMore = realCount >= l.pageSize()

		// Clamp cursor to valid range after reload.
		if l.cursor >= len(l.commits) {
			l.cursor = len(l.commits) - 1
		}
		if l.cursor < 0 {
			l.cursor = 0
		}

		// If WIP just disappeared (e.g. after commit), reset WIP state and
		// switch back to list focus so the user sees the new commit.
		if prevHadWIP && !l.hasWIP {
			l.wipUnstaged = nil
			l.wipStaged = nil
			l.diffViewer.Active = false
			l.diffViewer.Lines = nil
			l.diffViewer.Path = ""
			l.focus = focusLogList
			l.updateContext()
			// Reset inline commit area
			l.commitSummary = newCommitSummary(l.ctx.Theme)
			l.commitDesc = newCommitDesc(l.ctx.Theme)
			l.commitAmend = false
			l.commitField = 0
		}

		if l.hasWIP {
			// Load WIP detail for the first (synthetic) entry
			return l, l.loadWIPDetail()
		}
		if len(l.commits) > 0 && l.cursor < len(l.commits) {
			return l, l.loadCommitDetail(l.commits[l.cursor])
		}
		return l, nil

	case logMoreLoadedMsg:
		l.loadingMore = false
		if msg.err != nil {
			return l, nil // silently fail — user can retry
		}
		if len(msg.commits) == 0 {
			l.canLoadMore = false
			return l, nil
		}
		l.commits = append(l.commits, msg.commits...)
		l.graphRows = append(l.graphRows, msg.graphRows...)
		l.graphState = msg.graphState
		l.updateGraphMaxWidth()
		l.canLoadMore = len(msg.commits) >= l.pageSize()
		// Stop pagination once we hit the memory cap.
		if len(l.commits) >= maxCommitsInMemory {
			l.canLoadMore = false
		}
		return l, nil

	case wipDetailMsg:
		if msg.err != nil {
			l.wipUnstaged = nil
			l.wipStaged = nil
			l.wipUnstagedStats = nil
			l.wipStagedStats = nil
		} else {
			l.wipUnstaged = msg.unstaged
			l.wipStaged = msg.staged
			// Build stat lookup maps by path.
			l.wipUnstagedStats = make(map[string]git.DiffStatEntry, len(msg.unstagedStats))
			for _, e := range msg.unstagedStats {
				l.wipUnstagedStats[e.Path] = e
			}
			l.wipStagedStats = make(map[string]git.DiffStatEntry, len(msg.stagedStats))
			for _, e := range msg.stagedStats {
				l.wipStagedStats[e.Path] = e
			}
		}
		// Clamp cursors to valid range (preserve position after stage/unstage).
		if l.wipUnstagedCursor >= len(l.wipUnstaged) {
			l.wipUnstagedCursor = len(l.wipUnstaged) - 1
		}
		if l.wipUnstagedCursor < 0 {
			l.wipUnstagedCursor = 0
		}
		if l.wipStagedCursor >= len(l.wipStaged) {
			l.wipStagedCursor = len(l.wipStaged) - 1
		}
		if l.wipStagedCursor < 0 {
			l.wipStagedCursor = 0
		}
		// Clamp sub-viewport scroll offsets after cursor clamping.
		if l.wipUnstagedScroll > l.wipUnstagedCursor {
			l.wipUnstagedScroll = l.wipUnstagedCursor
		}
		if l.wipStagedScroll > l.wipStagedCursor {
			l.wipStagedScroll = l.wipStagedCursor
		}
		l.diffViewer.ScrollY = 0
		l.diffViewer.ScrollX = 0
		l.diffViewer.Lines = nil
		l.diffViewer.Path = ""
		l.diffViewer.Active = false

		// Auto-redirect focus when current panel becomes empty.
		switch l.wipFocus {
		case wipFocusUnstaged:
			if len(l.wipUnstaged) == 0 && len(l.wipStaged) > 0 {
				l.wipFocus = wipFocusStaged
			}
		case wipFocusStaged:
			if len(l.wipStaged) == 0 && len(l.wipUnstaged) > 0 {
				l.wipFocus = wipFocusUnstaged
			}
		}

		dirty := len(l.wipUnstaged) > 0 || len(l.wipStaged) > 0

		// If no more changes, the WIP row should disappear. Reload the log
		// to remove the synthetic entry and move cursor to the new commit.
		if !dirty {
			return l, tea.Batch(l.loadLog(), func() tea.Msg {
				return StatusDirtyMsg{Dirty: false}
			})
		}

		return l, func() tea.Msg {
			return StatusDirtyMsg{Dirty: dirty}
		}

	case centerDiffMsg:
		syntaxTheme := ""
		if l.ctx != nil && l.ctx.Config != nil {
			syntaxTheme = l.ctx.Config.Appearance.SyntaxTheme
		}
		l.diffViewer.SetContent(msg.path, msg.diff, msg.isWIP, msg.isStaged, syntaxTheme)
		return l, nil

	case wipStageResultMsg:
		// After stage/unstage/discard, reload WIP data.
		// If we're in diff mode, also reload the diff to reflect the change.
		if l.diffViewer.Active && l.diffViewer.IsWIP {
			return l, tea.Batch(l.loadWIPDetail(), l.loadCenterDiff())
		}
		return l, l.loadWIPDetail()

	case commitOpDoneMsg:
		// After any commit operation, show success and refresh
		label := msg.op
		return l, tea.Batch(
			func() tea.Msg { return RequestToastMsg{Message: label + " complete"} },
			func() tea.Msg { return RefreshStatusMsg{} },
		)

	case undoTargetMsg:
		// Store the target hash and show confirm dialog
		l.pendingUndoHash = msg.hash
		short := msg.shortHash
		message := msg.message
		return l, func() tea.Msg {
			return RequestConfirmMsg{
				ID:      "undo-action",
				Title:   "Undo?",
				Message: "Undo: " + message + "?\n\nHEAD will move to " + short + ". Uncommitted changes are preserved.",
			}
		}

	case redoTargetMsg:
		// Store the target hash and show confirm dialog
		l.pendingRedoHash = msg.hash
		short := msg.shortHash
		message := msg.message
		return l, func() tea.Msg {
			return RequestConfirmMsg{
				ID:      "redo-action",
				Title:   "Redo?",
				Message: "Redo: " + message + "?\n\nHEAD will move to " + short + ". Uncommitted changes are preserved.",
			}
		}

	case safeResetMsg:
		// Stash-bracketed hard reset: preserve uncommitted changes
		repo := l.repo
		op := msg.op
		hash := msg.hash
		short := msg.short
		return l, func() tea.Msg {
			// 1. Check if working tree is dirty
			dirty, _ := repo.IsDirty()
			stashed := false

			// 2. Stash uncommitted changes if dirty
			if dirty {
				err := repo.StashSave(op + " auto-stash")
				if err == nil {
					stashed = true
				}
				// If stash fails, proceed anyway — the user confirmed
			}

			// 3. Hard reset to the target
			err := repo.ResetHard(hash)
			if err != nil {
				// Try to pop stash back if reset failed
				if stashed {
					if popErr := repo.StashPop(0); popErr != nil {
						return RequestToastMsg{
							Message: op + " failed: " + err.Error() + " (stash restore also failed — run 'git stash pop' manually)",
							IsError: true,
						}
					}
				}
				return RequestToastMsg{Message: op + " failed: " + err.Error(), IsError: true}
			}

			// 4. Pop stash to restore working directory changes
			if stashed {
				popErr := repo.StashPop(0)
				if popErr != nil {
					// Stash pop conflict — notify user but the undo/redo itself succeeded
					return commitOpDoneMsg{op: op + " to " + short + " (stash conflict — resolve manually)"}
				}
			}

			return commitOpDoneMsg{op: op + " to " + short}
		}

	case editorDoneMsg:
		// Editor exited — refresh to pick up any changes
		return l, func() tea.Msg { return RefreshStatusMsg{} }

	case commitDetailMsg:
		if msg.err != nil {
			l.detailCommit = &msg.commit
			l.detailFiles = nil
		} else {
			l.detailCommit = &msg.commit
			if msg.diff != nil {
				l.detailFiles = msg.diff.Files
			} else {
				l.detailFiles = nil
			}
			// Populate the diff cache for fast j/k navigation.
			if msg.commit.Hash != "" && l.compareBase == nil {
				l.diffCache.Put(msg.commit.Hash, msg.diff, msg.commit.Body)
			}
		}
		l.detailFileCursor = 0
		l.diffViewer.Active = false
		l.diffViewer.ScrollY = 0
		l.diffViewer.ScrollX = 0
		l.diffViewer.Lines = nil
		l.diffViewer.Path = ""
		return l, nil

	// Handle confirm dialog results — route to WIP panel or sidebar.
	case dialog.ConfirmResultMsg:
		// WIP discard
		if msg.ID == "wip-discard" && msg.Confirmed && l.wipPendingDiscardPath != "" {
			path := l.wipPendingDiscardPath
			untracked := l.wipPendingDiscardUntracked
			l.wipPendingDiscardPath = ""
			l.wipPendingDiscardUntracked = false
			if untracked {
				return l, l.wipCleanFile(path)
			}
			return l, l.wipDiscardFile(path)
		}
		l.wipPendingDiscardPath = ""
		l.wipPendingDiscardUntracked = false
		// Revert commit
		if msg.ID == "revert-commit" && msg.Confirmed && l.pendingOpHash != "" {
			hash := l.pendingOpHash
			l.pendingOpHash = ""
			return l, l.doRevertCommit(hash)
		}
		// Cherry-pick commit
		if msg.ID == "cherry-pick-commit" && msg.Confirmed && l.pendingOpHash != "" {
			hash := l.pendingOpHash
			l.pendingOpHash = ""
			return l, l.doCherryPick(hash)
		}
		if msg.ID == "revert-commit" || msg.ID == "cherry-pick-commit" {
			l.pendingOpHash = ""
		}
		// Rebase action (squash/fixup/drop)
		if strings.HasPrefix(msg.ID, "rebase-") && msg.Confirmed && l.pendingOpHash != "" {
			hash := l.pendingOpHash
			action := l.pendingOpAction
			l.pendingOpHash = ""
			l.pendingOpAction = ""
			return l, l.doRebaseAction(hash, action)
		}
		if strings.HasPrefix(msg.ID, "rebase-") {
			l.pendingOpHash = ""
			l.pendingOpAction = ""
		}
		// Hard reset confirm
		if msg.ID == "reset-hard-confirm" && msg.Confirmed && l.pendingOpHash != "" {
			hash := l.pendingOpHash
			short := hash
			if len(short) > 7 {
				short = short[:7]
			}
			l.pendingOpHash = ""
			l.pendingOpAction = ""
			return l, l.doResetOp(hash, "hard", short)
		}
		if msg.ID == "reset-hard-confirm" {
			l.pendingOpHash = ""
			l.pendingOpAction = ""
		}
		// Nuke working tree
		if msg.ID == "nuke-working-tree" && msg.Confirmed {
			return l, l.doNukeWorkingTree()
		}
		// Undo confirm
		if msg.ID == "undo-action" && msg.Confirmed {
			cmd := l.doUndoConfirmed()
			l.pendingUndoHash = ""
			return l, cmd
		}
		if msg.ID == "undo-action" {
			l.pendingUndoHash = ""
		}
		// Redo confirm
		if msg.ID == "redo-action" && msg.Confirmed {
			cmd := l.doRedoConfirmed()
			l.pendingRedoHash = ""
			return l, cmd
		}
		if msg.ID == "redo-action" {
			l.pendingRedoHash = ""
		}
		// Bisect reset confirm
		if msg.ID == "bisect-reset" && msg.Confirmed {
			return l, l.doBisectReset()
		}
		// Route sidebar confirm results
		if strings.HasPrefix(msg.ID, "sidebar-") {
			var cmd tea.Cmd
			l.sidebar, cmd = l.sidebar.HandleDialogResult(dialogConfirmResultMsg{ID: msg.ID, Confirmed: msg.Confirmed})
			return l, cmd
		}
		return l, nil

	// Handle menu dialog results
	case dialog.MenuResultMsg:
		return l.handleMenuResult(msg)

	// Route text input results to sidebar
	case dialog.TextInputResultMsg:
		if strings.HasPrefix(msg.ID, "sidebar-") {
			var cmd tea.Cmd
			l.sidebar, cmd = l.sidebar.HandleDialogResult(dialogTextInputResultMsg{ID: msg.ID, Value: msg.Value})
			return l, cmd
		}
		return l, nil
	case dialog.TextInputCancelMsg:
		if strings.HasPrefix(msg.ID, "sidebar-") {
			var cmd tea.Cmd
			l.sidebar, cmd = l.sidebar.HandleDialogResult(dialogTextInputCancelMsg{ID: msg.ID})
			return l, cmd
		}
		return l, nil

	// Sidebar internal messages
	case sidebarLoadedMsg:
		l.sidebar, _ = l.sidebar.Update(msg)
		return l, nil
	case sidebarOpDoneMsg:
		var cmd tea.Cmd
		l.sidebar, cmd = l.sidebar.Update(msg)
		return l, cmd
	case SidebarOpenPRInBrowserMsg:
		prURL := msg.URL
		return l, func() tea.Msg {
			if err := utils.OpenBrowser(prURL); err != nil {
				return RequestToastMsg{Message: "Failed to open browser: " + err.Error(), IsError: true}
			}
			return RequestToastMsg{Message: "Opened PR in browser"}
		}

	case SidebarPRsLoadedMsg:
		l.sidebar, _ = l.sidebar.Update(msg)
		return l, nil

	case SidebarViewPRMsg:
		// Switch right panel to PR detail view
		l.viewingStash = false
		l.viewingPR = true
		l.viewedPR = &msg.PR
		l.diffViewer.Active = false
		l.diffViewer.Lines = nil
		l.diffViewer.Path = ""
		return l, nil

	case SidebarViewStashMsg:
		// Switch right panel to stash diff view
		l.viewingPR = false
		l.viewingStash = true
		l.stashDiffIndex = msg.Index
		l.stashDiffContent = ""
		return l, l.loadStashDiff(msg.Index)

	case stashDiffMsg:
		l.stashDiffIndex = msg.index
		l.diffViewer.ScrollY = 0
		l.diffViewer.ScrollX = 0
		if msg.err != nil {
			l.stashDiffContent = "Error loading stash diff: " + msg.err.Error()
			l.diffViewer.Active = false
			l.diffViewer.Lines = nil
			l.diffViewer.Path = ""
		} else {
			l.stashDiffContent = msg.diff
			syntaxTheme := ""
			if l.ctx != nil && l.ctx.Config != nil {
				syntaxTheme = l.ctx.Config.Appearance.SyntaxTheme
			}
			l.diffViewer.SetContent(fmt.Sprintf("stash@{%d}", msg.index), msg.diff, false, false, syntaxTheme)
		}
		return l, nil

	case amendPrefillMsg:
		l.commitAmend = true
		// Split message into summary (first line) and description (rest)
		summary, desc := splitCommitMessage(msg.message)
		l.commitSummary.SetValue(summary)
		l.commitDesc.SetValue(desc)
		l.wipFocus = wipFocusCommit
		l.commitField = 0
		l.commitEditing = true
		l.commitSummary.Focus()
		l.commitDesc.Blur()
		return l, nil

	case AICommitResultMsg:
		l.aiGenerating = false
		if !l.loading {
			l.spinner = l.spinner.SetLabel("Loading...").Stop()
		}
		// Populate the commit fields with the AI-generated message.
		l.commitSummary.SetValue(msg.Summary)
		l.commitDesc.SetValue(msg.Description)
		// Move focus to the commit area so the user can review/edit.
		l.wipFocus = wipFocusCommit
		l.commitField = 0
		l.commitEditing = true
		l.commitSummary.Focus()
		l.commitDesc.Blur()
		return l, nil

	case AICommitErrorMsg:
		l.aiGenerating = false
		if !l.loading {
			l.spinner = l.spinner.SetLabel("Loading...").Stop()
		}
		return l, func() tea.Msg {
			return RequestToastMsg{Message: "AI: " + msg.Err.Error(), IsError: true}
		}

	case RestoreCommitMsg:
		// Restore commit fields after a canceled commit confirmation.
		l.commitSummary.SetValue(msg.Summary)
		l.commitDesc.SetValue(msg.Description)
		l.wipFocus = wipFocusCommit
		l.commitField = 0
		l.commitEditing = true
		l.commitSummary.Focus()
		l.commitDesc.Blur()
		return l, nil

	case DiffFullscreenToggleMsg:
		l.diffFullscreen = !l.diffFullscreen
		return l, nil

	case RefreshStatusMsg:
		// Auto-refresh detected external changes — reload log and sidebar.
		return l, tea.Batch(l.loadLog(), l.sidebar.Refresh())

	case SettingsUpdatedMsg:
		// Sync local state with settings changes from the settings dialog.
		if msg.Key == "appearance.diffMode" {
			l.diffViewer.SideBySide = msg.Value == "side-by-side"
		}
		return l, nil

	case tea.MouseMsg:
		return l.handleMouse(msg)

	case tea.KeyMsg:
		m, cmd := l.handleKey(msg)
		if lp, ok := m.(LogPage); ok {
			lp.updateContext()
			animCmd := lp.updateBorderTargets()
			return lp, tea.Batch(cmd, animCmd)
		}
		return m, cmd
	}

	// When the commit area is focused, forward non-key messages (blink, etc.)
	// Forward non-key messages to the search input when active (cursor blink).
	if l.searching {
		var cmd tea.Cmd
		l.searchInput, cmd = l.searchInput.Update(msg)
		return l, cmd
	}

	// to the active input field.
	if l.isWIPSelected() && l.focus == focusLogDetail && l.wipFocus == wipFocusCommit {
		var cmd tea.Cmd
		if l.commitField == 0 {
			l.commitSummary, cmd = l.commitSummary.Update(msg)
		} else {
			l.commitDesc, cmd = l.commitDesc.Update(msg)
		}
		return l, cmd
	}

	return l, nil
}
