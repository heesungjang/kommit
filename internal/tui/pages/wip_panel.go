package pages

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/styles"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// WIP Panel — types, constructors, viewport helpers
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
func newCommitSummary(t theme.Theme) textinput.Model {
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
func newCommitDesc(t theme.Theme) textarea.Model {
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
// Viewport height helpers
// ---------------------------------------------------------------------------

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
