package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nicholascross/opengit/internal/git"
	"github.com/nicholascross/opengit/internal/tui/anim"
	"github.com/nicholascross/opengit/internal/tui/dialog"
	"github.com/nicholascross/opengit/internal/tui/styles"
	"github.com/nicholascross/opengit/internal/tui/theme"
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
			max := len(l.diffViewer.Lines) - 10
			if max < 0 {
				max = 0
			}
			if l.diffViewer.ScrollY > max {
				l.diffViewer.ScrollY = max
			}
		}
		return l, nil
	case key.Matches(msg, l.navKeys.PageUp):
		if l.diffViewer.Active {
			l.diffViewer.ScrollY -= 10
			if l.diffViewer.ScrollY < 0 {
				l.diffViewer.ScrollY = 0
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
			max := len(l.diffViewer.Lines) - 10
			if max < 0 {
				max = 0
			}
			if l.diffViewer.ScrollY > max {
				l.diffViewer.ScrollY = max
			}
		}
		return l, nil
	case key.Matches(msg, l.navKeys.PageUp):
		if l.diffViewer.Active {
			l.diffViewer.ScrollY -= 10
			if l.diffViewer.ScrollY < 0 {
				l.diffViewer.ScrollY = 0
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
	// Reset fields
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
// and updates focus/cursor accordingly.
func (l LogPage) handleWIPMouseClick(msg tea.MouseMsg, leftOuter int) (tea.Model, tea.Cmd) {
	// Y coordinate relative to right panel top: subtract border (1) + title (1) = 2
	relY := msg.Y - 2
	if relY < 0 {
		return l, nil
	}

	// Layout in renderWIPDetail:
	// row 0: "Unstaged Files (N)" header
	// rows 1..U: unstaged files (or 1 row "Working tree clean")
	// row U+1: separator ─
	// row U+2: "Staged Files (N)" header
	// rows U+3..U+2+S: staged files (or 1 row "No files staged")
	// rest: separator, commit hint, diff...

	unstagedRows := len(l.wipUnstaged)
	if unstagedRows == 0 {
		unstagedRows = 1 // "Working tree clean" placeholder
	}
	stagedRows := len(l.wipStaged)
	if stagedRows == 0 {
		stagedRows = 1 // "No files staged" placeholder
	}

	// Unstaged header at row 0
	// Unstaged files: rows 1 .. unstagedRows
	// Separator: row unstagedRows + 1
	// Staged header: row unstagedRows + 2
	// Staged files: rows unstagedRows + 3 .. unstagedRows + 2 + stagedRows

	unstagedFileStart := 1
	unstagedFileEnd := unstagedFileStart + unstagedRows // exclusive
	stagedHeaderRow := unstagedFileEnd + 1              // after separator
	stagedFileStart := stagedHeaderRow + 1
	stagedFileEnd := stagedFileStart + stagedRows // exclusive

	if relY >= unstagedFileStart && relY < unstagedFileEnd && len(l.wipUnstaged) > 0 {
		idx := relY - unstagedFileStart
		if idx < len(l.wipUnstaged) {
			l.wipFocus = wipFocusUnstaged
			l.wipUnstagedCursor = idx
			l.diffViewer.ScrollY = 0
			l.diffViewer.ScrollX = 0
			return l, l.loadCenterDiff()
		}
	} else if relY >= stagedFileStart && relY < stagedFileEnd && len(l.wipStaged) > 0 {
		idx := relY - stagedFileStart
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

	var fileSections []string

	// --- Unstaged Files ---
	unstagedFocused := focused && l.wipFocus == wipFocusUnstaged
	fileSections = append(fileSections, sectionTitle(fmt.Sprintf("▾ Unstaged Files (%d)", len(l.wipUnstaged)), "u", unstagedFocused))
	fileSections = append(fileSections, bgLine("")) // margin bottom

	if len(l.wipUnstaged) == 0 {
		fileSections = append(fileSections, styles.DimStyle().Width(iw).Render("  Working tree clean"))
	} else {
		for i, f := range l.wipUnstaged {
			icon := f.StatusIcon()
			code := f.UnstagedCode
			color := styles.FileStatusColor(code)

			selected := i == l.wipUnstagedCursor && unstagedFocused
			bg := t.Base
			prefix := "  "
			if selected {
				bg = t.Surface1
				prefix = "▸ "
			}

			// Build line with ANSI-safe segments (no .Background on segments)
			iconStr := lipgloss.NewStyle().Foreground(color).Render(icon)
			pathStr := lipgloss.NewStyle().Foreground(t.Text).Render(" " + f.Path)
			if selected {
				iconStr = lipgloss.NewStyle().Foreground(color).Bold(true).Render(icon)
				pathStr = lipgloss.NewStyle().Foreground(t.Text).Bold(true).Render(" " + f.Path)
			}

			// Diff stats: +N -M
			var statStr string
			if st, ok := l.wipUnstagedStats[f.Path]; ok && (st.Added > 0 || st.Removed > 0) {
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
			fileSections = append(fileSections, fillBg(lineContent, bg, iw))
		}
	}

	fileSections = append(fileSections, bgLine(lipgloss.NewStyle().Foreground(t.Surface2).Background(t.Base).Render(strings.Repeat("─", iw))))

	// --- Staged Files ---
	stagedFocused := focused && l.wipFocus == wipFocusStaged
	fileSections = append(fileSections, sectionTitle(fmt.Sprintf("▾ Staged Files (%d)", len(l.wipStaged)), "s", stagedFocused))
	fileSections = append(fileSections, bgLine("")) // margin bottom

	if len(l.wipStaged) == 0 {
		fileSections = append(fileSections, styles.DimStyle().Width(iw).Render("  No files staged"))
	} else {
		for i, f := range l.wipStaged {
			icon := f.StatusIcon()
			code := f.StagedCode
			color := styles.FileStatusColor(code)

			selected := i == l.wipStagedCursor && stagedFocused
			bg := t.Base
			prefix := "  "
			if selected {
				bg = t.Surface1
				prefix = "▸ "
			}

			// Build line with ANSI-safe segments (no .Background on segments)
			iconStr := lipgloss.NewStyle().Foreground(color).Render(icon)
			pathStr := lipgloss.NewStyle().Foreground(t.Text).Render(" " + f.Path)
			if selected {
				iconStr = lipgloss.NewStyle().Foreground(color).Bold(true).Render(icon)
				pathStr = lipgloss.NewStyle().Foreground(t.Text).Bold(true).Render(" " + f.Path)
			}

			// Diff stats: +N -M
			var statStr string
			if st, ok := l.wipStagedStats[f.Path]; ok && (st.Added > 0 || st.Removed > 0) {
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
			fileSections = append(fileSections, fillBg(lineContent, bg, iw))
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
	if l.commitAmend {
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
		hintText = "Enter:edit  A:amend  u:unstaged  s:staged"
	} else {
		hintText = "Enter:diff  spc:stage  a:all  A:amend  d:del"
	}
	hintsLine := lipgloss.NewStyle().Background(t.Base).Width(iw).Render(
		styles.KeyHintStyle().Render(hintText),
	)
	hintHeight := strings.Count(hintsLine, "\n") + 1

	// ---------------------------------------------------------------
	// Compute file area height and pad/clip to fill the gap
	// fileAreaHeight = ph - title(1) - titleGap(1) - commitBoxHeight - hintHeight
	// ---------------------------------------------------------------
	fileAreaHeight := ph - 2 - commitBoxHeight - hintHeight
	if fileAreaHeight < 1 {
		fileAreaHeight = 1
	}

	fileContent := lipgloss.JoinVertical(lipgloss.Left, fileSections...)
	fileLines := strings.Split(fileContent, "\n")

	// Viewport windowing: compute cursor's line position and scroll to keep it visible
	if len(fileLines) > fileAreaHeight {
		// Compute the cursor's line index within fileSections.
		// Layout: unstaged header(1) + margin(1) + unstaged files/empty(N) + separator(1)
		//         + staged header(1) + margin(1) + staged files/empty(M)
		unstagedFileCount := len(l.wipUnstaged)
		if unstagedFileCount == 0 {
			unstagedFileCount = 1 // "Working tree clean" placeholder
		}
		stagedFileCount := len(l.wipStaged)
		if stagedFileCount == 0 {
			stagedFileCount = 1 // "No files staged" placeholder
		}

		cursorLine := 0
		switch l.wipFocus {
		case wipFocusUnstaged:
			cursorLine = 2 + l.wipUnstagedCursor // header(1) + margin(1) + cursor offset
		case wipFocusStaged:
			cursorLine = 2 + unstagedFileCount + 1 + 2 + l.wipStagedCursor // unstaged section + separator(1) + staged header(1) + margin(1)
		case wipFocusCommit:
			// Commit box is below the file area — scroll to show end of files
			cursorLine = 0
		}

		// Compute scroll offset
		scrollOffset := 0
		if cursorLine >= fileAreaHeight {
			scrollOffset = cursorLine - fileAreaHeight + 1
		}
		if scrollOffset > len(fileLines)-fileAreaHeight {
			scrollOffset = len(fileLines) - fileAreaHeight
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}
		fileLines = fileLines[scrollOffset : scrollOffset+fileAreaHeight]
	}

	// Pad with empty bg lines if file list is shorter — this pushes commit area to the bottom
	for len(fileLines) < fileAreaHeight {
		fileLines = append(fileLines, lipgloss.NewStyle().Background(t.Base).Width(iw).Render(""))
	}
	fileContent = strings.Join(fileLines, "\n")

	// ---------------------------------------------------------------
	// Assemble: title + file area (padded) + commit area (fixed height) + hints (pinned bottom)
	// ---------------------------------------------------------------
	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, fileContent, commitContent, hintsLine)

	// Safety clip to panel height
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.ClipPanel(styles.PanelStyleColor(l.borderAnim.Color(anim.BorderRight, t.Surface1, t.Blue)).Width(width).Height(ph).Render(full), height)
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
