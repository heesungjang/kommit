package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/heesungjang/kommit/internal/tui/dialog"
)

// ---------------------------------------------------------------------------
// Key handlers
// ---------------------------------------------------------------------------

func (l LogPage) handleWIPDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Tab within WIP panel cycles unstaged → staged → commit → sidebar
	if key.Matches(msg, key.NewBinding(key.WithKeys("tab"))) {
		switch l.wipFocus {
		case wipFocusUnstaged:
			if len(l.wipStaged) > 0 {
				l.wipFocus = wipFocusStaged
			} else {
				l.wipFocus = wipFocusCommit
				l.commitEditing = false
				l.commitSummary.Blur()
				l.commitDesc.Blur()
			}
		case wipFocusStaged:
			l.wipFocus = wipFocusCommit
			l.commitEditing = false
			l.commitSummary.Blur()
			l.commitDesc.Blur()
		case wipFocusCommit:
			l.commitEditing = false
			l.commitSummary.Blur()
			l.commitDesc.Blur()
			l.focus = focusSidebar
		default:
			l.focus = focusSidebar
		}
		return l, nil
	}
	if key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))) {
		switch l.wipFocus {
		case wipFocusCommit:
			l.commitEditing = false
			l.commitSummary.Blur()
			l.commitDesc.Blur()
			if len(l.wipStaged) > 0 {
				l.wipFocus = wipFocusStaged
			} else if len(l.wipUnstaged) > 0 {
				l.wipFocus = wipFocusUnstaged
			} else {
				l.focus = focusLogList
			}
		case wipFocusStaged:
			l.wipFocus = wipFocusUnstaged
		default:
			l.focus = focusLogList
		}
		return l, nil
	}

	// Undo — available when not editing commit text
	if !l.commitEditing && key.Matches(msg, l.statusKeys.Undo) {
		return l, l.doUndo()
	}

	// AI commit message — available from any WIP sub-focus when not editing
	if !l.commitEditing && key.Matches(msg, l.statusKeys.AICommit) {
		if l.aiGenerating {
			return l, nil // already generating
		}
		l.aiGenerating = true
		l.skeletonTick = 0
		l.spinner = l.spinner.SetLabel("Summarizing...").Start()
		return l, tea.Batch(
			l.spinner.Tick(),
			func() tea.Msg { return RequestAICommitMsg{} },
		)
	}

	// Stash save/pop — available from any WIP sub-focus when not editing
	if !l.commitEditing && key.Matches(msg, l.statusKeys.StashSave) {
		return l, func() tea.Msg { return RequestStashSaveMsg{} }
	}
	if !l.commitEditing && key.Matches(msg, l.statusKeys.StashPop) {
		return l, func() tea.Msg { return RequestStashPopMsg{} }
	}

	// Section jump shortcuts: u/s/c (available when not actively editing text)
	if !l.commitEditing {
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("u"))):
			l.commitSummary.Blur()
			l.commitDesc.Blur()
			l.commitEditing = false
			l.wipFocus = wipFocusUnstaged
			return l, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			l.commitSummary.Blur()
			l.commitDesc.Blur()
			l.commitEditing = false
			l.wipFocus = wipFocusStaged
			return l, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("c"))):
			l.wipFocus = wipFocusCommit
			l.commitAmend = false
			l.commitEditing = false
			l.commitSummary.Blur()
			l.commitDesc.Blur()
			return l, nil
		}
	}

	// Commit area selected but not editing — handle Enter/A/D to start editing or clear
	if l.wipFocus == wipFocusCommit && !l.commitEditing {
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			// Start editing the commit summary
			l.commitEditing = true
			l.commitField = 0
			l.commitSummary.Focus()
			l.commitDesc.Blur()
			return l, nil
		case key.Matches(msg, l.statusKeys.CommitAmend):
			// Start editing in amend mode
			l.commitEditing = true
			l.commitField = 0
			l.commitSummary.Focus()
			l.commitDesc.Blur()
			return l, l.loadAmendPrefill()
		case key.Matches(msg, key.NewBinding(key.WithKeys("D"))):
			// Clear both commit summary and description fields
			l.commitSummary.SetValue("")
			l.commitDesc.SetValue("")
			l.commitAmend = false
			return l, nil
		}
		return l, nil
	}

	switch l.wipFocus {
	case wipFocusUnstaged:
		return l.handleWIPUnstagedKeys(msg)
	case wipFocusStaged:
		return l.handleWIPStagedKeys(msg)
	case wipFocusCommit:
		return l.handleWIPCommitKeys(msg)
	}
	return l, nil
}

func (l LogPage) handleWIPUnstagedKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		l.diffViewer.Active = false
		l.diffViewer.Lines = nil
		l.diffViewer.Path = ""
		l.diffViewer.ScrollY = 0
		l.diffViewer.ScrollX = 0
		return l, nil
	case key.Matches(msg, l.navKeys.Select): // Enter — view diff for selected file
		if len(l.wipUnstaged) > 0 && l.wipUnstagedCursor < len(l.wipUnstaged) {
			l.diffViewer.ScrollY = 0
			l.diffViewer.ScrollX = 0
			return l, l.loadCenterDiff()
		}
	case key.Matches(msg, l.navKeys.Down):
		if l.wipUnstagedCursor < len(l.wipUnstaged)-1 {
			l.wipUnstagedCursor++
		} else if len(l.wipStaged) > 0 {
			// Auto-advance to staged section
			l.wipFocus = wipFocusStaged
			l.wipStagedCursor = 0
		}
	case key.Matches(msg, l.navKeys.Up):
		if l.wipUnstagedCursor > 0 {
			l.wipUnstagedCursor--
		}
	case key.Matches(msg, l.navKeys.Home):
		l.wipUnstagedCursor = 0
	case key.Matches(msg, l.navKeys.End):
		if len(l.wipUnstaged) > 0 {
			l.wipUnstagedCursor = len(l.wipUnstaged) - 1
		}
	case key.Matches(msg, l.navKeys.PageDown):
		// Always scroll the file list when file list has focus.
		jump := l.wipFilePageSize() / 2
		if jump < 1 {
			jump = 1
		}
		l.wipUnstagedCursor += jump
		if l.wipUnstagedCursor >= len(l.wipUnstaged) {
			l.wipUnstagedCursor = len(l.wipUnstaged) - 1
		}
		if l.wipUnstagedCursor < 0 {
			l.wipUnstagedCursor = 0
		}
		return l, nil
	case key.Matches(msg, l.navKeys.PageUp):
		jump := l.wipFilePageSize() / 2
		if jump < 1 {
			jump = 1
		}
		l.wipUnstagedCursor -= jump
		if l.wipUnstagedCursor < 0 {
			l.wipUnstagedCursor = 0
		}
		return l, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys(" "))):
		if len(l.wipUnstaged) > 0 && l.wipUnstagedCursor < len(l.wipUnstaged) {
			return l, l.wipStageFile(l.wipUnstaged[l.wipUnstagedCursor].Path)
		}
	case key.Matches(msg, l.statusKeys.StageAll):
		return l, l.wipStageAll()
	case key.Matches(msg, l.statusKeys.CommitAmend):
		// Activate inline commit area in amend mode (directly start editing)
		l.wipFocus = wipFocusCommit
		l.commitEditing = true
		l.commitSummary.Focus()
		l.commitDesc.Blur()
		l.commitField = 0
		return l, l.loadAmendPrefill()
	case key.Matches(msg, l.statusKeys.Discard):
		if len(l.wipUnstaged) > 0 && l.wipUnstagedCursor < len(l.wipUnstaged) {
			f := l.wipUnstaged[l.wipUnstagedCursor]
			l.wipPendingDiscardPath = f.Path
			l.wipPendingDiscardUntracked = f.IsUntracked()
			action := "Discard changes"
			if f.IsUntracked() {
				action = "Delete untracked file"
			}
			confirmMsg := fmt.Sprintf("%s to '%s'?\n\nThis action cannot be undone.", action, f.Path)
			return l, func() tea.Msg {
				return RequestConfirmMsg{
					ID:      "wip-discard",
					Title:   "Confirm " + action,
					Message: confirmMsg,
				}
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
		if len(l.wipUnstaged) > 0 && l.wipUnstagedCursor < len(l.wipUnstaged) {
			return l, l.openInEditor(l.wipUnstaged[l.wipUnstagedCursor].Path)
		}
	}
	return l, nil
}

func (l LogPage) handleWIPStagedKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		l.diffViewer.Active = false
		l.diffViewer.Lines = nil
		l.diffViewer.Path = ""
		l.diffViewer.ScrollY = 0
		l.diffViewer.ScrollX = 0
		return l, nil
	case key.Matches(msg, l.navKeys.Select): // Enter — view diff for selected file
		if len(l.wipStaged) > 0 && l.wipStagedCursor < len(l.wipStaged) {
			l.diffViewer.ScrollY = 0
			l.diffViewer.ScrollX = 0
			return l, l.loadCenterDiff()
		}
	case key.Matches(msg, l.navKeys.Down):
		if l.wipStagedCursor < len(l.wipStaged)-1 {
			l.wipStagedCursor++
		}
	case key.Matches(msg, l.navKeys.Up):
		if l.wipStagedCursor > 0 {
			l.wipStagedCursor--
		} else if len(l.wipUnstaged) > 0 {
			// Auto-advance back to unstaged section
			l.wipFocus = wipFocusUnstaged
			l.wipUnstagedCursor = len(l.wipUnstaged) - 1
		}
	case key.Matches(msg, l.navKeys.Home):
		l.wipStagedCursor = 0
	case key.Matches(msg, l.navKeys.End):
		if len(l.wipStaged) > 0 {
			l.wipStagedCursor = len(l.wipStaged) - 1
		}
	case key.Matches(msg, l.navKeys.PageDown):
		// Always scroll the file list when file list has focus.
		jump := l.wipFilePageSize() / 2
		if jump < 1 {
			jump = 1
		}
		l.wipStagedCursor += jump
		if l.wipStagedCursor >= len(l.wipStaged) {
			l.wipStagedCursor = len(l.wipStaged) - 1
		}
		if l.wipStagedCursor < 0 {
			l.wipStagedCursor = 0
		}
		return l, nil
	case key.Matches(msg, l.navKeys.PageUp):
		jump := l.wipFilePageSize() / 2
		if jump < 1 {
			jump = 1
		}
		l.wipStagedCursor -= jump
		if l.wipStagedCursor < 0 {
			l.wipStagedCursor = 0
		}
		return l, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys(" "))):
		// Unstage the selected file
		if len(l.wipStaged) > 0 && l.wipStagedCursor < len(l.wipStaged) {
			return l, l.wipUnstageFile(l.wipStaged[l.wipStagedCursor].Path)
		}
	case key.Matches(msg, l.statusKeys.CommitAmend):
		// Activate inline commit area in amend mode (directly start editing)
		l.wipFocus = wipFocusCommit
		l.commitEditing = true
		l.commitSummary.Focus()
		l.commitDesc.Blur()
		l.commitField = 0
		return l, l.loadAmendPrefill()
	case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
		if len(l.wipStaged) > 0 && l.wipStagedCursor < len(l.wipStaged) {
			return l, l.openInEditor(l.wipStaged[l.wipStagedCursor].Path)
		}
	}
	return l, nil
}

func (l LogPage) handleWIPCommitKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		// Stop editing but stay on the commit area (selected, not editing)
		l.commitEditing = false
		l.commitSummary.Blur()
		l.commitDesc.Blur()
		return l, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+s"))):
		// Submit the commit
		return l.submitCommit()

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		// Enter in summary field submits; in description field inserts newline
		if l.commitField == 0 {
			return l.submitCommit()
		}
		// Fall through to textarea Update below for newline insertion

	case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
		// Toggle between summary and description
		if l.commitField == 0 {
			l.commitField = 1
			l.commitSummary.Blur()
			l.commitDesc.Focus()
		} else {
			l.commitField = 0
			l.commitDesc.Blur()
			l.commitSummary.Focus()
		}
		return l, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))):
		// Reverse toggle
		if l.commitField == 1 {
			l.commitField = 0
			l.commitDesc.Blur()
			l.commitSummary.Focus()
		} else {
			// Exit editing and go backwards to staged/unstaged
			l.commitEditing = false
			l.commitSummary.Blur()
			l.commitDesc.Blur()
			if len(l.wipStaged) > 0 {
				l.wipFocus = wipFocusStaged
			} else if len(l.wipUnstaged) > 0 {
				l.wipFocus = wipFocusUnstaged
			}
		}
		return l, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down"))):
		// In summary field, move focus to description
		if l.commitField == 0 {
			l.commitField = 1
			l.commitSummary.Blur()
			l.commitDesc.Focus()
			return l, nil
		}
		// In description, let textarea handle cursor movement (fall through)

	case key.Matches(msg, key.NewBinding(key.WithKeys("up"))):
		// In description field on the first line, move focus back to summary
		if l.commitField == 1 && l.commitDesc.Line() == 0 {
			l.commitField = 0
			l.commitDesc.Blur()
			l.commitSummary.Focus()
			return l, nil
		}
		// Otherwise let textarea handle cursor movement (fall through)
		if l.commitField == 0 {
			return l, nil // No-op in summary
		}
	}

	// Forward all other keys to the active input field for typing
	var cmd tea.Cmd
	if l.commitField == 0 {
		l.commitSummary, cmd = l.commitSummary.Update(msg)
	} else {
		l.commitDesc, cmd = l.commitDesc.Update(msg)
	}
	return l, cmd
}

// submitCommit validates and submits the commit message.
func (l LogPage) submitCommit() (tea.Model, tea.Cmd) {
	summary := l.commitSummary.Value()
	if summary == "" {
		return l, nil
	}
	desc := l.commitDesc.Value()
	message := summary
	if strings.TrimSpace(desc) != "" {
		message = summary + "\n\n" + strings.TrimSpace(desc)
	}
	amend := l.commitAmend
	// Reset fields and exit editing mode.
	l.commitEditing = false
	l.commitSummary.SetValue("")
	l.commitDesc.SetValue("")
	l.commitSummary.Blur()
	l.commitDesc.Blur()
	l.commitAmend = false
	l.commitField = 0
	if len(l.wipUnstaged) > 0 {
		l.wipFocus = wipFocusUnstaged
	} else if len(l.wipStaged) > 0 {
		l.wipFocus = wipFocusStaged
	}
	return l, func() tea.Msg {
		return dialog.CommitRequestMsg{Message: message, Amend: amend}
	}
}

// ---------------------------------------------------------------------------
// Mouse handling
// ---------------------------------------------------------------------------

// handleWIPMouseClick processes a left-click in the WIP detail (right) panel.
// It maps the Y coordinate to either the unstaged or staged file list section
// and updates focus/cursor accordingly. Accounts for sub-viewport scrolling.
func (l LogPage) handleWIPMouseClick(msg tea.MouseMsg, _ int) (tea.Model, tea.Cmd) {
	// Y coordinate relative to right panel content: subtract border(1) + title(1) + gap(1) = 3
	relY := msg.Y - 3
	if relY < 0 {
		return l, nil
	}

	// Layout with split sub-viewports:
	// row 0: unstaged header
	// row 1: margin
	// rows 2..2+visUnstaged-1: visible unstaged file lines
	// row 2+visUnstaged: separator ─
	// row 3+visUnstaged: staged header
	// row 4+visUnstaged: margin
	// rows 5+visUnstaged..5+visUnstaged+visStaged-1: visible staged file lines

	// Compute visible unstaged count (same as render — approximate using viewport height)
	unstagedContentH := l.wipUnstagedViewHeight()
	visUnstaged := len(l.wipUnstaged)
	if visUnstaged == 0 {
		visUnstaged = 1 // placeholder
	}
	if visUnstaged > unstagedContentH && unstagedContentH > 0 {
		visUnstaged = unstagedContentH
	}

	unstagedFileStart := 2 // after header + margin
	unstagedFileEnd := unstagedFileStart + visUnstaged
	separatorRow := unstagedFileEnd
	stagedFileStart := separatorRow + 3 // separator + header + margin
	stagedContentH := l.wipStagedViewHeight()
	visStaged := len(l.wipStaged)
	if visStaged == 0 {
		visStaged = 1 // placeholder
	}
	if visStaged > stagedContentH && stagedContentH > 0 {
		visStaged = stagedContentH
	}
	stagedFileEnd := stagedFileStart + visStaged

	if relY >= unstagedFileStart && relY < unstagedFileEnd && len(l.wipUnstaged) > 0 {
		visIdx := relY - unstagedFileStart
		idx := l.wipUnstagedScroll + visIdx
		if idx < len(l.wipUnstaged) {
			l.wipFocus = wipFocusUnstaged
			l.wipUnstagedCursor = idx
			l.diffViewer.ScrollY = 0
			l.diffViewer.ScrollX = 0
			return l, l.loadCenterDiff()
		}
	} else if relY >= stagedFileStart && relY < stagedFileEnd && len(l.wipStaged) > 0 {
		visIdx := relY - stagedFileStart
		idx := l.wipStagedScroll + visIdx
		if idx < len(l.wipStaged) {
			l.wipFocus = wipFocusStaged
			l.wipStagedCursor = idx
			l.diffViewer.ScrollY = 0
			l.diffViewer.ScrollX = 0
			return l, l.loadCenterDiff()
		}
	}

	return l, nil
}

// wipUnstagedViewHeight returns the number of file lines visible in the
