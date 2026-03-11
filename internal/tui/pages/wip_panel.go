package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/anim"
	"github.com/heesungjang/kommit/internal/tui/dialog"
	"github.com/heesungjang/kommit/internal/tui/styles"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// WIP Panel — types, constructors, key handlers, render, commands
// ---------------------------------------------------------------------------

// wipPanelFocus tracks which sub-panel is focused within the WIP staging area.
type wipPanelFocus int

const (
	wipFocusUnstaged wipPanelFocus = iota
	wipFocusStaged
	wipFocusCommit
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// wipDetailMsg carries the WIP (working changes) status data.
type wipDetailMsg struct {
	unstaged      []git.FileStatus
	staged        []git.FileStatus
	unstagedStats []git.DiffStatEntry
	stagedStats   []git.DiffStatEntry
	err           error
}

// wipStageResultMsg is sent after a stage/unstage/discard operation completes.
type wipStageResultMsg struct {
	err error
}

// amendPrefillMsg carries the previous commit message for amend mode prefill.
type amendPrefillMsg struct {
	message string
}

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// newCommitSummary creates a fresh single-line text input for the commit title.
func newCommitSummary() textinput.Model {
	t := theme.Active
	ti := textinput.New()
	ti.Placeholder = "Summary (required)"
	ti.CharLimit = 72
	ti.Prompt = ""
	ti.TextStyle = lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface0)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Surface0)
	ti.Cursor.Style = lipgloss.NewStyle().Background(t.Surface0)
	return ti
}

// newCommitDesc creates a fresh multi-line textarea for the commit description.
func newCommitDesc() textarea.Model {
	t := theme.Active
	ta := textarea.New()
	ta.Placeholder = "Description (optional)"
	ta.CharLimit = 0 // unlimited
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.FocusedStyle.Base = lipgloss.NewStyle().Background(t.Surface0)
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface0)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Surface0)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(t.Surface0)
	ta.BlurredStyle.Base = lipgloss.NewStyle().Background(t.Surface0)
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface0)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Surface0)
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle().Background(t.Surface0)
	// Re-establish style pointer after modifying BlurredStyle/FocusedStyle.
	// textarea.New() sets m.style to a stale pointer; Blur() re-points it
	// to &m.BlurredStyle so our style overrides actually take effect.
	ta.Blur()
	return ta
}

// splitCommitMessage splits a full commit message into summary (first line)
// and description (everything after the first blank line separator).
func splitCommitMessage(message string) (summary, description string) {
	parts := strings.SplitN(message, "\n", 2)
	summary = parts[0]
	if len(parts) > 1 {
		description = strings.TrimSpace(parts[1])
	}
	return summary, description
}

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
		return l, func() tea.Msg { return RequestAICommitMsg{} }
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

	// Commit area selected but not editing — handle Enter/A to start editing
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
			// Page-jump in file list
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
		}
		return l, nil
	case key.Matches(msg, l.navKeys.PageUp):
		if l.diffViewer.Active {
			l.diffViewer.ScrollY -= 10
			if l.diffViewer.ScrollY < 0 {
				l.diffViewer.ScrollY = 0
			}
		} else {
			jump := l.wipFilePageSize() / 2
			if jump < 1 {
				jump = 1
			}
			l.wipUnstagedCursor -= jump
			if l.wipUnstagedCursor < 0 {
				l.wipUnstagedCursor = 0
			}
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
		}
		return l, nil
	case key.Matches(msg, l.navKeys.PageUp):
		if l.diffViewer.Active {
			l.diffViewer.ScrollY -= 10
			if l.diffViewer.ScrollY < 0 {
				l.diffViewer.ScrollY = 0
			}
		} else {
			jump := l.wipFilePageSize() / 2
			if jump < 1 {
				jump = 1
			}
			l.wipStagedCursor -= jump
			if l.wipStagedCursor < 0 {
				l.wipStagedCursor = 0
			}
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
// unstaged sub-viewport. Used by mouse click handler to map Y coordinates.
func (l LogPage) wipUnstagedViewHeight() int {
	unstagedContentH, _ := l.wipViewportContentHeights()
	return unstagedContentH
}

// wipStagedViewHeight returns the number of file lines visible in the
// staged sub-viewport. Used by mouse click handler to map Y coordinates.
func (l LogPage) wipStagedViewHeight() int {
	_, stagedContentH := l.wipViewportContentHeights()
	return stagedContentH
}

// wipViewportContentHeights computes the content-only heights for the unstaged
// and staged sub-viewports. This duplicates the height allocation logic from
// renderWIPDetail to keep mouse mapping consistent with rendering.
func (l LogPage) wipViewportContentHeights() (unstagedContentH, stagedContentH int) {
	ph := l.height - styles.PanelBorderHeight
	// Estimate commit box height (~11 lines) and hint height (~1 line)
	commitBoxHeight := 11
	hintHeight := 1
	separatorHeight := 1
	fileAreaHeight := ph - 2 - separatorHeight - commitBoxHeight - hintHeight
	if fileAreaHeight < 2 {
		fileAreaHeight = 2
	}

	unstagedChrome := 2
	stagedChrome := 2
	unstagedDataLines := len(l.wipUnstaged)
	if unstagedDataLines == 0 {
		unstagedDataLines = 1
	}
	stagedDataLines := len(l.wipStaged)
	if stagedDataLines == 0 {
		stagedDataLines = 1
	}

	minStagedHeight := stagedChrome + 1
	if minStagedHeight > fileAreaHeight-unstagedChrome-1 {
		minStagedHeight = fileAreaHeight - unstagedChrome - 1
		if minStagedHeight < 1 {
			minStagedHeight = 1
		}
	}

	wantUnstaged := unstagedChrome + unstagedDataLines
	wantStaged := stagedChrome + stagedDataLines
	totalWant := wantUnstaged + wantStaged

	var unstagedViewH, stagedViewH int
	if totalWant <= fileAreaHeight {
		unstagedViewH = wantUnstaged
		stagedViewH = fileAreaHeight - unstagedViewH
	} else {
		stagedViewH = minStagedHeight
		remaining := fileAreaHeight - stagedViewH
		if remaining < unstagedChrome+1 {
			remaining = unstagedChrome + 1
			stagedViewH = fileAreaHeight - remaining
		}
		if wantStaged < stagedViewH {
			stagedViewH = wantStaged
			remaining = fileAreaHeight - stagedViewH
		}
		unstagedViewH = remaining
	}

	if unstagedViewH < 1 {
		unstagedViewH = 1
	}
	if stagedViewH < 1 {
		stagedViewH = 1
	}

	unstagedContentH = unstagedViewH - unstagedChrome
	if unstagedContentH < 0 {
		unstagedContentH = 0
	}
	stagedContentH = stagedViewH - stagedChrome
	if stagedContentH < 0 {
		stagedContentH = 0
	}
	return
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func (l LogPage) renderWIPDetail(width, height int) string {
	focused := l.focus == focusLogDetail
	iw := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight
	t := theme.Active

	bgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).MaxWidth(iw).Width(iw).Render(s)
	}

	// ---------------------------------------------------------------
	// Build file list sections (top part — will be clipped/padded)
	// ---------------------------------------------------------------
	// sectionTitle renders a section header with a right-aligned shortcut key tag.
	sectionTitle := func(label, shortcut string, active bool) string {
		fg := t.Subtext0
		if active {
			fg = t.Blue
		}
		keyTag := "[" + shortcut + "]"
		keyStyle := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base)
		titleStyle := lipgloss.NewStyle().Foreground(fg).Background(t.Base).Bold(true)
		labelWidth := iw - len(keyTag) - 1
		if labelWidth < 4 {
			return titleStyle.Width(iw).Render(label)
		}
		if len(label) > labelWidth {
			if labelWidth > 1 {
				label = label[:labelWidth-1] + "…"
			} else {
				label = label[:labelWidth]
			}
		}
		leftPart := titleStyle.Width(labelWidth).Render(label)
		rightPart := keyStyle.Render(keyTag)
		gap := lipgloss.NewStyle().Background(t.Base).Render(" ")
		return lipgloss.JoinHorizontal(lipgloss.Top, leftPart, gap, rightPart)
	}

	// --- Helper: render a file line ---
	renderFileLine := func(f git.FileStatus, idx int, cursorIdx int, isFocused bool, staged bool) string {
		icon := f.StatusIcon()
		code := f.UnstagedCode
		statsMap := l.wipUnstagedStats
		if staged {
			code = f.StagedCode
			statsMap = l.wipStagedStats
		}
		color := styles.FileStatusColor(code)

		selected := idx == cursorIdx && isFocused
		bg := t.Base
		prefix := "  "
		if selected {
			bg = t.Surface1
			prefix = "▸ "
		}

		iconStr := lipgloss.NewStyle().Foreground(color).Render(icon)
		pathStr := lipgloss.NewStyle().Foreground(t.Text).Render(" " + f.Path)
		if selected {
			iconStr = lipgloss.NewStyle().Foreground(color).Bold(true).Render(icon)
			pathStr = lipgloss.NewStyle().Foreground(t.Text).Bold(true).Render(" " + f.Path)
		}

		var statStr string
		if st, ok := statsMap[f.Path]; ok && (st.Added > 0 || st.Removed > 0) {
			var statParts []string
			if st.Added > 0 {
				statParts = append(statParts, lipgloss.NewStyle().Foreground(t.Green).Render(fmt.Sprintf("+%d", st.Added)))
			}
			if st.Removed > 0 {
				statParts = append(statParts, lipgloss.NewStyle().Foreground(t.Red).Render(fmt.Sprintf("-%d", st.Removed)))
			}
			statStr = " " + strings.Join(statParts, " ")
		}

		lineContent := lipgloss.NewStyle().MaxWidth(iw).Render(prefix + iconStr + pathStr + statStr)
		return fillBg(lineContent, bg, iw)
	}

	// --- Build unstaged file lines ---
	unstagedFocused := focused && l.wipFocus == wipFocusUnstaged
	var unstagedFileLines []string
	if len(l.wipUnstaged) == 0 {
		unstagedFileLines = append(unstagedFileLines, styles.DimStyle().Width(iw).Render("  Working tree clean"))
	} else {
		for i, f := range l.wipUnstaged {
			unstagedFileLines = append(unstagedFileLines, renderFileLine(f, i, l.wipUnstagedCursor, unstagedFocused, false))
		}
	}

	// --- Build staged file lines ---
	stagedFocused := focused && l.wipFocus == wipFocusStaged
	var stagedFileLines []string
	if len(l.wipStaged) == 0 {
		stagedFileLines = append(stagedFileLines, styles.DimStyle().Width(iw).Render("  No files staged"))
	} else {
		for i, f := range l.wipStaged {
			stagedFileLines = append(stagedFileLines, renderFileLine(f, i, l.wipStagedCursor, stagedFocused, true))
		}
	}

	// ---------------------------------------------------------------
	// Build commit area (bottom part — pinned to panel bottom)
	// Wrapped in a single border container that highlights when focused.
	// ---------------------------------------------------------------
	commitFocused := focused && l.wipFocus == wipFocusCommit

	// Inner width for content inside the outer commit container border
	ciw := iw - 2 // -2 for outer container border
	if ciw < 10 {
		ciw = 10
	}
	cBgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).Width(ciw).Render(s)
	}

	var commitInner []string

	// Header: ─○ Commit (or ─○ Amend Commit) with [c] shortcut
	commitHeaderLabel := "─○ Commit"
	if l.aiGenerating {
		commitHeaderLabel = "─○ AI generating..."
	} else if l.commitAmend {
		commitHeaderLabel = "─○ Amend Commit"
	}
	{
		fg := t.Subtext0
		if commitFocused {
			fg = t.Blue
		}
		keyTag := "[c]"
		keyStyle := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base)
		titleStyle := lipgloss.NewStyle().Foreground(fg).Background(t.Base).Bold(true)
		labelWidth := ciw - len(keyTag) - 1
		if labelWidth < 4 {
			labelWidth = 4
		}
		label := commitHeaderLabel
		if lipgloss.Width(label) > labelWidth {
			label = label[:labelWidth-1] + "…"
		}
		gap := ciw - lipgloss.Width(label) - lipgloss.Width(keyTag)
		if gap < 1 {
			gap = 1
		}
		gapStyle := lipgloss.NewStyle().Background(t.Base)
		commitInner = append(commitInner, lipgloss.NewStyle().Background(t.Base).Width(ciw).Render(
			titleStyle.Render(label)+gapStyle.Render(strings.Repeat(" ", gap))+keyStyle.Render(keyTag),
		))
	}
	commitInner = append(commitInner, cBgLine("")) // margin bottom

	// Summary input — single-line
	inputWidth := ciw - 2 // -2 for individual input border
	if inputWidth < 6 {
		inputWidth = 6
	}
	l.commitSummary.Width = inputWidth

	summaryBorder := l.borderAnim.Color(anim.BorderCommitSummary, t.Surface2, t.Blue)
	summaryView := fillBg(l.commitSummary.View(), t.Surface0, inputWidth)
	summaryBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(summaryBorder).
		BorderBackground(t.Surface0).
		Background(t.Surface0).
		Width(ciw - 2). // -2 for input border
		Render(summaryView)
	commitInner = append(commitInner, cBgLine(summaryBox))

	// Description textarea — multi-line
	l.commitDesc.SetWidth(inputWidth)

	descBorder := l.borderAnim.Color(anim.BorderCommitDesc, t.Surface2, t.Blue)
	descView := fillBg(l.commitDesc.View(), t.Surface0, inputWidth)
	descBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(descBorder).
		BorderBackground(t.Surface0).
		Background(t.Surface0).
		Width(ciw - 2). // -2 for input border
		Render(descView)
	commitInner = append(commitInner, cBgLine(descBox))

	// Wrap commit area in a single outer container border
	containerBorder := l.borderAnim.Color(anim.BorderCommitOuter, t.Surface2, t.Blue)
	innerContent := lipgloss.JoinVertical(lipgloss.Left, commitInner...)
	commitContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(containerBorder).
		BorderBackground(t.Base).
		Background(t.Base).
		Width(iw - 2). // outer border width
		Render(innerContent)

	// ---------------------------------------------------------------
	// Measure actual commit box height (instead of hardcoding).
	// The rendered commitContent may vary depending on widget output.
	// ---------------------------------------------------------------
	commitBoxHeight := strings.Count(commitContent, "\n") + 1

	// ---------------------------------------------------------------
	// Title
	// ---------------------------------------------------------------
	titleStr := styles.PanelTitle(
		"Working Changes",
		"3", focused, iw,
	)
	titleGap := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")

	// ---------------------------------------------------------------
	// Build the context-dependent hint line (placed OUTSIDE the commit box)
	// ---------------------------------------------------------------
	var hintText string
	if commitFocused && l.commitEditing {
		hintText = "Enter:commit  Tab:desc  Esc:stop"
		if l.commitField == 1 {
			hintText = "ctrl+s:commit  Tab:summary  Esc:stop"
		}
	} else if commitFocused {
		hintText = "Enter:edit  A:amend  ctrl+g:AI"
	} else {
		// ctrl+g works from any WIP sub-focus when not editing
		hintText = "ctrl+g:AI commit"
	}
	hintsLine := lipgloss.NewStyle().Background(t.Base).Width(iw).PaddingLeft(1).Render(
		styles.KeyHintStyle().Render(hintText),
	)
	hintHeight := strings.Count(hintsLine, "\n") + 1

	// ---------------------------------------------------------------
	// Split file area into two independent sub-viewports so the staged
	// section header is ALWAYS visible, even with many unstaged files.
	// Layout: title(1) + gap(1) + [unstaged viewport] + separator(1)
	//         + [staged viewport] + commitBox + hints
	// ---------------------------------------------------------------
	separatorHeight := 1
	fileAreaHeight := ph - 2 - separatorHeight - commitBoxHeight - hintHeight
	if fileAreaHeight < 2 {
		fileAreaHeight = 2
	}

	// Each sub-viewport gets: header(1) + margin(1) + file lines.
	// Chrome per section = 2 (header + margin).
	unstagedChrome := 2
	stagedChrome := 2

	// Allocate heights proportionally — staged gets at least enough to show
	// header + margin + 1 visible file line (min 3), unstaged gets the rest.
	unstagedDataLines := len(unstagedFileLines) // actual file rows (or 1 placeholder)
	stagedDataLines := len(stagedFileLines)

	// Minimum staged viewport: chrome + 1 file line
	minStagedHeight := stagedChrome + 1
	if minStagedHeight > fileAreaHeight-unstagedChrome-1 {
		minStagedHeight = fileAreaHeight - unstagedChrome - 1
		if minStagedHeight < 1 {
			minStagedHeight = 1
		}
	}

	// Desired heights (if there were unlimited space)
	wantUnstaged := unstagedChrome + unstagedDataLines
	wantStaged := stagedChrome + stagedDataLines
	totalWant := wantUnstaged + wantStaged

	var unstagedViewH, stagedViewH int
	if totalWant <= fileAreaHeight {
		// Everything fits — no scrolling needed
		unstagedViewH = wantUnstaged
		stagedViewH = fileAreaHeight - unstagedViewH
	} else {
		// Not everything fits — allocate proportionally with staged minimum
		stagedViewH = minStagedHeight
		remaining := fileAreaHeight - stagedViewH
		if remaining < unstagedChrome+1 {
			remaining = unstagedChrome + 1
			stagedViewH = fileAreaHeight - remaining
		}
		// If staged has few items, give more to unstaged
		if wantStaged < stagedViewH {
			stagedViewH = wantStaged
			remaining = fileAreaHeight - stagedViewH
		}
		unstagedViewH = remaining
	}

	// Ensure minimums
	if unstagedViewH < 1 {
		unstagedViewH = 1
	}
	if stagedViewH < 1 {
		stagedViewH = 1
	}

	// --- Render unstaged sub-viewport ---
	unstagedHeader := sectionTitle(fmt.Sprintf("▾ Unstaged Files (%d)", len(l.wipUnstaged)), "u", unstagedFocused)
	unstagedMargin := bgLine("")
	unstagedContentH := unstagedViewH - unstagedChrome
	if unstagedContentH < 0 {
		unstagedContentH = 0
	}

	// Scroll unstaged viewport to keep cursor visible
	if len(unstagedFileLines) > unstagedContentH && unstagedContentH > 0 {
		if l.wipUnstagedScroll > l.wipUnstagedCursor {
			l.wipUnstagedScroll = l.wipUnstagedCursor
		}
		if l.wipUnstagedCursor >= l.wipUnstagedScroll+unstagedContentH {
			l.wipUnstagedScroll = l.wipUnstagedCursor - unstagedContentH + 1
		}
		maxScroll := len(unstagedFileLines) - unstagedContentH
		if l.wipUnstagedScroll > maxScroll {
			l.wipUnstagedScroll = maxScroll
		}
		if l.wipUnstagedScroll < 0 {
			l.wipUnstagedScroll = 0
		}
	} else {
		l.wipUnstagedScroll = 0
	}

	// Clip unstaged file lines to viewport
	visibleUnstaged := unstagedFileLines
	if len(visibleUnstaged) > unstagedContentH && unstagedContentH > 0 {
		end := l.wipUnstagedScroll + unstagedContentH
		if end > len(visibleUnstaged) {
			end = len(visibleUnstaged)
		}
		visibleUnstaged = visibleUnstaged[l.wipUnstagedScroll:end]
	}
	// Pad to fill the viewport
	for len(visibleUnstaged) < unstagedContentH {
		visibleUnstaged = append(visibleUnstaged, bgLine(""))
	}

	// Build scroll indicator for unstaged
	unstagedScrollHint := ""
	if len(unstagedFileLines) > unstagedContentH && unstagedContentH > 0 {
		if l.wipUnstagedScroll > 0 {
			unstagedScrollHint += "↑"
		}
		if l.wipUnstagedScroll+unstagedContentH < len(unstagedFileLines) {
			if unstagedScrollHint != "" {
				unstagedScrollHint += " "
			}
			unstagedScrollHint += "↓"
		}
	}
	if unstagedScrollHint != "" {
		scrollTag := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render(" " + unstagedScrollHint)
		unstagedHeader = sectionTitle(fmt.Sprintf("▾ Unstaged (%d) %s", len(l.wipUnstaged), scrollTag), "u", unstagedFocused)
	}

	var unstagedSection []string
	unstagedSection = append(unstagedSection, unstagedHeader)
	unstagedSection = append(unstagedSection, unstagedMargin)
	unstagedSection = append(unstagedSection, visibleUnstaged...)

	// --- Separator ---
	separator := bgLine(lipgloss.NewStyle().Foreground(t.Surface2).Background(t.Base).Render(strings.Repeat("─", iw)))

	// --- Render staged sub-viewport ---
	stagedHeader := sectionTitle(fmt.Sprintf("▾ Staged Files (%d)", len(l.wipStaged)), "s", stagedFocused)
	stagedMargin := bgLine("")
	stagedContentH := stagedViewH - stagedChrome
	if stagedContentH < 0 {
		stagedContentH = 0
	}

	// Scroll staged viewport to keep cursor visible
	if len(stagedFileLines) > stagedContentH && stagedContentH > 0 {
		if l.wipStagedScroll > l.wipStagedCursor {
			l.wipStagedScroll = l.wipStagedCursor
		}
		if l.wipStagedCursor >= l.wipStagedScroll+stagedContentH {
			l.wipStagedScroll = l.wipStagedCursor - stagedContentH + 1
		}
		maxScroll := len(stagedFileLines) - stagedContentH
		if l.wipStagedScroll > maxScroll {
			l.wipStagedScroll = maxScroll
		}
		if l.wipStagedScroll < 0 {
			l.wipStagedScroll = 0
		}
	} else {
		l.wipStagedScroll = 0
	}

	// Clip staged file lines to viewport
	visibleStaged := stagedFileLines
	if len(visibleStaged) > stagedContentH && stagedContentH > 0 {
		end := l.wipStagedScroll + stagedContentH
		if end > len(visibleStaged) {
			end = len(visibleStaged)
		}
		visibleStaged = visibleStaged[l.wipStagedScroll:end]
	}
	// Pad to fill the viewport
	for len(visibleStaged) < stagedContentH {
		visibleStaged = append(visibleStaged, bgLine(""))
	}

	// Build scroll indicator for staged
	stagedScrollHint := ""
	if len(stagedFileLines) > stagedContentH && stagedContentH > 0 {
		if l.wipStagedScroll > 0 {
			stagedScrollHint += "↑"
		}
		if l.wipStagedScroll+stagedContentH < len(stagedFileLines) {
			if stagedScrollHint != "" {
				stagedScrollHint += " "
			}
			stagedScrollHint += "↓"
		}
	}
	if stagedScrollHint != "" {
		scrollTag := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render(" " + stagedScrollHint)
		stagedHeader = sectionTitle(fmt.Sprintf("▾ Staged (%d) %s", len(l.wipStaged), scrollTag), "s", stagedFocused)
	}

	var stagedSection []string
	stagedSection = append(stagedSection, stagedHeader)
	stagedSection = append(stagedSection, stagedMargin)
	stagedSection = append(stagedSection, visibleStaged...)

	// ---------------------------------------------------------------
	// Assemble the file area from the two sub-viewports
	// ---------------------------------------------------------------
	var fileAreaParts []string
	fileAreaParts = append(fileAreaParts, unstagedSection...)
	fileAreaParts = append(fileAreaParts, separator)
	fileAreaParts = append(fileAreaParts, stagedSection...)
	fileContent := strings.Join(fileAreaParts, "\n")

	// ---------------------------------------------------------------
	// Assemble: title + gap + file area + commit area + hints
	// ---------------------------------------------------------------
	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, fileContent, commitContent, hintsLine)

	// Safety clip to panel height
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.ClipPanel(styles.PanelStyleColor(l.borderAnim.Color(anim.BorderRight, t.Surface1, t.Blue)).Width(width).Height(ph).Render(full), height)
}

// wipFilePageSize returns an approximate number of file lines visible in one
// sub-viewport, used for ctrl+d / ctrl+u page-jump calculations.
func (l LogPage) wipFilePageSize() int {
	ph := l.height - styles.PanelBorderHeight
	// Rough: title(1) + gap(1) + separator(1) + 2*chrome(2*2=4) + commitBox(~11) + hint(1) = ~19 chrome
	// Each section gets about half the remaining space
	fileArea := (ph - 19) / 2
	if fileArea < 4 {
		fileArea = 4
	}
	return fileArea
}

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
