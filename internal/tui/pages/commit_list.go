package pages

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/dialog"
	"github.com/heesungjang/kommit/internal/tui/utils"
)

// ---------------------------------------------------------------------------
// Commit list — messages, key handlers, menu/compare handling
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// logLoadedMsg carries the result of loading the commit log.
type logLoadedMsg struct {
	commits   []git.CommitInfo
	graphRows []git.GraphRow
	hasWIP    bool // true if a synthetic WIP entry was prepended
	err       error
}

// logMoreLoadedMsg carries additional commits loaded via pagination.
type logMoreLoadedMsg struct {
	commits   []git.CommitInfo
	graphRows []git.GraphRow // graph for the FULL combined list
	err       error
}

// commitDetailMsg carries the result of loading a single commit's detail.
type commitDetailMsg struct {
	commit git.CommitInfo
	diff   *git.DiffResult
	err    error
}

// centerDiffMsg carries a loaded diff to display in the center panel.
type centerDiffMsg struct {
	path     string
	diff     string
	err      error
	isWIP    bool // true when this diff is from WIP context (staged/unstaged)
	isStaged bool // within WIP, whether this is a staged diff
}

// commitOpDoneMsg is sent after a revert or cherry-pick completes successfully.
type commitOpDoneMsg struct {
	op string // "revert" or "cherry-pick"
}

// undoTargetMsg carries the undo target hash back to the UI thread so it can
// be stored in pendingUndoHash before showing the confirm dialog.
type undoTargetMsg struct {
	hash      string
	shortHash string
	message   string
}

// redoTargetMsg carries the redo target hash back to the UI thread so it can
// be stored in pendingRedoHash before showing the confirm dialog.
type redoTargetMsg struct {
	hash      string
	shortHash string
	message   string
}

// safeResetMsg triggers a stash-bracketed hard reset: stash uncommitted
// changes, reset --hard, then pop stash. This preserves working directory
// changes while performing undo/redo operations.
type safeResetMsg struct {
	hash  string
	short string
	op    string // "Undo" or "Redo"
}

// ---------------------------------------------------------------------------
// Key handlers
// ---------------------------------------------------------------------------

// loadDetailForCursor loads either WIP or commit detail depending on cursor position.
func (l LogPage) loadDetailForCursor() tea.Cmd {
	if l.isWIPSelected() {
		return l.loadWIPDetail()
	}
	if l.cursor >= 0 && l.cursor < len(l.commits) {
		return l.loadCommitDetail(l.commits[l.cursor])
	}
	return nil
}

// loadDetailForCursorMaybeMore loads the detail for the current cursor AND
// triggers pagination if the cursor is within 20 items of the end.
func (l *LogPage) loadDetailForCursorMaybeMore() tea.Cmd {
	detailCmd := l.loadDetailForCursor()
	// Trigger pagination when within 20 items of the end.
	threshold := 20
	remaining := len(l.commits) - 1 - l.cursor
	if remaining <= threshold && l.canLoadMore && !l.loadingMore {
		l.loadingMore = true
		moreCmd := l.loadLogMore()
		return tea.Batch(detailCmd, moreCmd)
	}
	return detailCmd
}

func (l LogPage) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When center is showing a diff, j/k scroll the diff and Esc exits.
	if l.diffViewer.Active {
		// In fullscreen mode, Esc exits fullscreen first (keeps the diff open).
		if l.diffFullscreen && key.Matches(msg, key.NewBinding(key.WithKeys("esc"))) {
			l.diffFullscreen = false
			return l, nil
		}

		// Delegate all diff-mode keys to the DiffViewer.
		handled, cmd := l.diffViewer.HandleKeys(msg, l.navKeys, l.repo, l.height)
		if handled {
			// HandleKeys.Reset() means Esc was pressed — return focus to detail.
			if !l.diffViewer.Active {
				l.diffFullscreen = false
				l.focus = focusLogDetail
			}
			return l, cmd
		}
		return l, nil
	}

	// Normal commit list navigation (graph mode).
	switch {
	case key.Matches(msg, l.navKeys.Down):
		if l.cursor < len(l.commits)-1 {
			l.cursor++
			cmd := l.loadDetailForCursorMaybeMore()
			return l, cmd
		}
	case key.Matches(msg, l.navKeys.Up):
		if l.cursor > 0 {
			l.cursor--
			cmd := l.loadDetailForCursor()
			return l, cmd
		}
	case key.Matches(msg, l.navKeys.PageDown):
		l.cursor += 10
		if l.cursor >= len(l.commits) {
			l.cursor = len(l.commits) - 1
		}
		if l.cursor < 0 {
			l.cursor = 0
		}
		cmd := l.loadDetailForCursorMaybeMore()
		return l, cmd
	case key.Matches(msg, l.navKeys.PageUp):
		l.cursor -= 10
		if l.cursor < 0 {
			l.cursor = 0
		}
		cmd := l.loadDetailForCursor()
		return l, cmd
	case key.Matches(msg, l.navKeys.Home):
		l.cursor = 0
		cmd := l.loadDetailForCursor()
		return l, cmd
	case key.Matches(msg, l.navKeys.End):
		if len(l.commits) > 0 {
			l.cursor = len(l.commits) - 1
			cmd := l.loadDetailForCursorMaybeMore()
			return l, cmd
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		return l, l.loadLog()
	case key.Matches(msg, key.NewBinding(key.WithKeys("H"))):
		// Scroll graph left
		if l.graphScrollX > 0 {
			l.graphScrollX--
		}
		return l, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("L"))):
		// Scroll graph right
		maxScroll := l.maxGraphWidth() - l.graphViewportCols()
		if maxScroll < 0 {
			maxScroll = 0
		}
		if l.graphScrollX < maxScroll {
			l.graphScrollX++
		}
		return l, nil
	}

	// Undo/Redo — works regardless of selection
	switch {
	case key.Matches(msg, l.commitOpsKeys.Undo):
		return l, l.doUndo()
	case key.Matches(msg, l.commitOpsKeys.Redo):
		return l, l.doRedo()
	}

	// Bisect menu — works regardless of selection
	if key.Matches(msg, l.commitOpsKeys.BisectMenu) {
		return l, l.showBisectMenu()
	}

	// Compare mode toggle
	if key.Matches(msg, l.commitOpsKeys.CompareRef) {
		return l.handleCompareToggle()
	}

	// Commit operations — only on past (non-WIP) commits
	if !l.isWIPSelected() && l.cursor >= 0 && l.cursor < len(l.commits) {
		commit := l.commits[l.cursor]
		short := commit.ShortHash
		if short == "" {
			short = commit.Hash
			if len(short) > 7 {
				short = short[:7]
			}
		}
		switch {
		case key.Matches(msg, l.commitOpsKeys.Revert):
			l.pendingOpHash = commit.Hash
			return l, func() tea.Msg {
				return RequestConfirmMsg{
					ID:      "revert-commit",
					Title:   "Revert Commit?",
					Message: "Revert " + short + " " + commit.Subject + "?",
				}
			}
		case key.Matches(msg, l.commitOpsKeys.CherryPick):
			l.pendingOpHash = commit.Hash
			return l, func() tea.Msg {
				return RequestConfirmMsg{
					ID:      "cherry-pick-commit",
					Title:   "Cherry-Pick?",
					Message: "Cherry-pick " + short + " onto current branch?",
				}
			}
		case key.Matches(msg, l.commitOpsKeys.CopyHash):
			return l, l.copyToClipboard(commit.Hash)
		case key.Matches(msg, l.commitOpsKeys.ResetMenu):
			return l, l.showResetMenu(commit, short)
		case key.Matches(msg, l.commitOpsKeys.Squash):
			l.pendingOpHash = commit.Hash
			l.pendingOpAction = "squash"
			return l, l.showRebaseConfirm(commit, short, "squash")
		case key.Matches(msg, l.commitOpsKeys.Fixup):
			l.pendingOpHash = commit.Hash
			l.pendingOpAction = "fixup"
			return l, l.showRebaseConfirm(commit, short, "fixup")
		case key.Matches(msg, l.commitOpsKeys.Drop):
			l.pendingOpHash = commit.Hash
			l.pendingOpAction = "drop"
			return l, l.showRebaseConfirm(commit, short, "drop")
		}
	}

	return l, nil
}

// detailTabCount is the number of tabs in the commit detail panel.
const detailTabCount = 3

// detailTabNames are the tab labels for the commit detail panel.
var detailTabNames = []string{"Files", "Message", "Stats"}

func (l LogPage) handleDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// PR detail view — limited keybindings
	if l.viewingPR && l.viewedPR != nil {
		return l.handlePRDetailKeys(msg)
	}

	if l.isWIPSelected() {
		return l.handleWIPDetailKeys(msg)
	}

	// Tab switching: left/right arrows cycle through detail tabs
	switch {
	case key.Matches(msg, l.navKeys.Right):
		l.detailTab = (l.detailTab + 1) % detailTabCount
		l.detailTabScroll = 0
		return l, nil
	case key.Matches(msg, l.navKeys.Left):
		l.detailTab = (l.detailTab - 1 + detailTabCount) % detailTabCount
		l.detailTabScroll = 0
		return l, nil
	}

	if key.Matches(msg, key.NewBinding(key.WithKeys("esc"))) {
		l.diffViewer.Active = false
		l.diffViewer.Lines = nil
		l.diffViewer.Path = ""
		l.diffViewer.ScrollY = 0
		l.diffViewer.ScrollX = 0
		return l, nil
	}

	// Compare toggle also works from the detail panel
	if key.Matches(msg, l.commitOpsKeys.CompareRef) {
		return l.handleCompareToggle()
	}

	// Tab-specific keys
	switch l.detailTab {
	case 0: // Files tab
		switch {
		case key.Matches(msg, l.navKeys.Select): // Enter — view diff for selected file
			if len(l.detailFiles) > 0 && l.detailFileCursor < len(l.detailFiles) {
				l.diffViewer.ScrollY = 0
				l.diffViewer.ScrollX = 0
				return l, l.loadCenterDiff()
			}
		case key.Matches(msg, l.navKeys.Down):
			if l.detailFileCursor < len(l.detailFiles)-1 {
				l.detailFileCursor++
			}
		case key.Matches(msg, l.navKeys.Up):
			if l.detailFileCursor > 0 {
				l.detailFileCursor--
			}
		case key.Matches(msg, l.navKeys.Home):
			l.detailFileCursor = 0
		case key.Matches(msg, l.navKeys.End):
			if len(l.detailFiles) > 0 {
				l.detailFileCursor = len(l.detailFiles) - 1
			}
		case key.Matches(msg, l.navKeys.PageDown):
			if l.diffViewer.Active {
				l.diffViewer.ScrollY += 10
				maxScroll := len(l.diffViewer.Lines) - 10
				if maxScroll < 0 {
					maxScroll = 0
				}
				if l.diffViewer.ScrollY > maxScroll {
					l.diffViewer.ScrollY = maxScroll
				}
			} else {
				l.detailFileCursor += 10
				if l.detailFileCursor >= len(l.detailFiles) {
					l.detailFileCursor = len(l.detailFiles) - 1
				}
				if l.detailFileCursor < 0 {
					l.detailFileCursor = 0
				}
			}
		case key.Matches(msg, l.navKeys.PageUp):
			if l.diffViewer.Active {
				l.diffViewer.ScrollY -= 10
				if l.diffViewer.ScrollY < 0 {
					l.diffViewer.ScrollY = 0
				}
			} else {
				l.detailFileCursor -= 10
				if l.detailFileCursor < 0 {
					l.detailFileCursor = 0
				}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
			// Open selected file in editor
			if len(l.detailFiles) > 0 && l.detailFileCursor < len(l.detailFiles) {
				path := l.detailFiles[l.detailFileCursor].NewPath
				if path == "" {
					path = l.detailFiles[l.detailFileCursor].OldPath
				}
				if path != "" {
					return l, l.openInEditor(path)
				}
			}
		}
	case 1, 2: // Message or Stats tab — j/k scroll
		switch {
		case key.Matches(msg, l.navKeys.Down):
			l.detailTabScroll++
		case key.Matches(msg, l.navKeys.Up):
			if l.detailTabScroll > 0 {
				l.detailTabScroll--
			}
		case key.Matches(msg, l.navKeys.PageDown):
			l.detailTabScroll += 10
		case key.Matches(msg, l.navKeys.PageUp):
			l.detailTabScroll -= 10
			if l.detailTabScroll < 0 {
				l.detailTabScroll = 0
			}
		case key.Matches(msg, l.navKeys.Home):
			l.detailTabScroll = 0
		}
	}
	return l, nil
}

// handlePRDetailKeys handles keyboard input when the PR detail view is active.
func (l LogPage) handlePRDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("o"))):
		// Open PR in browser
		if l.viewedPR != nil && l.viewedPR.URL != "" {
			prURL := l.viewedPR.URL
			return l, func() tea.Msg {
				if err := utils.OpenBrowser(prURL); err != nil {
					return RequestToastMsg{Message: "Failed to open browser: " + err.Error(), IsError: true}
				}
				return RequestToastMsg{Message: "Opened PR in browser"}
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		// Exit PR view
		l.viewingPR = false
		l.viewedPR = nil
		return l, nil
	}
	return l, nil
}

// ---------------------------------------------------------------------------
// Menu result handlers
// ---------------------------------------------------------------------------

func (l LogPage) handleMenuResult(msg dialog.MenuResultMsg) (tea.Model, tea.Cmd) {
	switch msg.ID {
	case "reset-menu":
		return l.handleResetMenuResult(msg.Index)
	case "bisect-menu":
		return l.handleBisectMenuResult(msg.Index)
	}
	return l, nil
}

func (l LogPage) handleResetMenuResult(idx int) (tea.Model, tea.Cmd) {
	if !l.isWIPSelected() && l.cursor >= 0 && l.cursor < len(l.commits) {
		commit := l.commits[l.cursor]
		hash := commit.Hash
		short := commit.ShortHash
		if short == "" && len(hash) > 7 {
			short = hash[:7]
		}
		switch idx {
		case 0: // Soft reset
			l.pendingOpHash = hash
			l.pendingOpAction = "reset-soft"
			return l, l.doResetOp(hash, "soft", short)
		case 1: // Mixed reset
			l.pendingOpHash = hash
			l.pendingOpAction = "reset-mixed"
			return l, l.doResetOp(hash, "mixed", short)
		case 2: // Hard reset
			l.pendingOpHash = hash
			l.pendingOpAction = "reset-hard"
			return l, func() tea.Msg {
				return RequestConfirmMsg{
					ID:      "reset-hard-confirm",
					Title:   "Hard Reset?",
					Message: "Hard reset to " + short + "?\n\nThis will discard all uncommitted changes.",
				}
			}
		case 3: // Nuke
			return l, func() tea.Msg {
				return RequestConfirmMsg{
					ID:      "nuke-working-tree",
					Title:   "Nuke Working Tree?",
					Message: "Reset --hard HEAD and remove ALL untracked files?\n\nThis cannot be undone.",
				}
			}
		}
	}
	return l, nil
}

// ---------------------------------------------------------------------------
// Compare two commits
// ---------------------------------------------------------------------------

func (l LogPage) handleCompareToggle() (tea.Model, tea.Cmd) {
	if l.compareBase != nil {
		// Exit compare mode
		l.compareBase = nil
		return l, tea.Batch(
			func() tea.Msg { return RequestToastMsg{Message: "Compare mode off"} },
			func() tea.Msg { return CompareStateMsg{Active: false} },
		)
	}
	// Enter compare mode — mark current commit as base
	if !l.isWIPSelected() && l.cursor >= 0 && l.cursor < len(l.commits) {
		c := l.commits[l.cursor]
		l.compareBase = &c
		short := c.ShortHash
		if short == "" && len(c.Hash) > 7 {
			short = c.Hash[:7]
		}
		s := short
		return l, tea.Batch(
			func() tea.Msg {
				return RequestToastMsg{Message: "Comparing from " + s + " — select another commit"}
			},
			func() tea.Msg { return CompareStateMsg{Active: true, Hash: s} },
		)
	}
	return l, nil
}
