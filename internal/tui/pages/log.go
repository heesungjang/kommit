package pages

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/hosting"
	"github.com/heesungjang/kommit/internal/tui/anim"
	"github.com/heesungjang/kommit/internal/tui/components"
	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/keys"
	"github.com/heesungjang/kommit/internal/tui/styles"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// logLoadedMsg, commitDetailMsg, centerDiffMsg, commitOpDoneMsg, undoTargetMsg, redoTargetMsg — now in commit_list.go

// editorDoneMsg is sent when an external editor process exits.
type editorDoneMsg struct {
	err error
}

// stashDiffMsg carries a loaded stash diff to show in the right panel.
type stashDiffMsg struct {
	index int
	diff  string
	err   error
}

// ---------------------------------------------------------------------------
// Focus targets
// ---------------------------------------------------------------------------

type logFocus int

const (
	focusLogList logFocus = iota // default: center panel (commit list)
	focusLogDetail
	focusSidebar
)

// ---------------------------------------------------------------------------
// LogPage model
// ---------------------------------------------------------------------------

// LogPage is the main unified view: sidebar (branches/tags/stash) | commit
// graph | context-sensitive detail (WIP staging / commit detail / stash diff).
type LogPage struct {
	ctx  *tuictx.ProgramContext // shared context (dimensions, theme, config, repo)
	repo *git.Repository

	// Left sidebar (branches, remotes, tags, stash)
	sidebar Sidebar

	commits        []git.CommitInfo
	cursor         int
	hasWIP         bool            // true when uncommitted changes exist; commits[0] is synthetic
	graphRows      []git.GraphRow  // parallel to commits; one GraphRow per commit
	graphScrollX   int             // horizontal scroll offset for graph column
	cachedGraphMax int             // cached max graph cell count (updated on data load, not per frame)
	graphState     *git.GraphState // lane state for incremental graph computation on pagination

	// Detail view — structured file list (for commits)
	detailCommit     *git.CommitInfo
	detailFiles      []git.DiffFile
	detailFileCursor int
	detailTab        int // 0=files, 1=message, 2=stats
	detailTabScroll  int // scroll offset for message/stats tab

	// WIP staging area — interactive when WIP row is selected
	wipUnstaged       []git.FileStatus
	wipStaged         []git.FileStatus
	wipUnstagedCursor int
	wipStagedCursor   int
	wipFocus          wipPanelFocus // which sub-panel is focused within the WIP detail
	wipUnstagedStats  map[string]git.DiffStatEntry
	wipStagedStats    map[string]git.DiffStatEntry
	wipUnstagedScroll int // viewport scroll offset for unstaged file list
	wipStagedScroll   int // viewport scroll offset for staged file list

	// Inline commit message area (GitKraken-style, always visible in WIP)
	commitSummary textinput.Model // single-line commit title/summary
	commitDesc    textarea.Model  // multi-line commit description/body
	commitField   int             // 0 = summary focused, 1 = description focused
	commitAmend   bool            // true when in amend mode
	commitEditing bool            // true when actively typing in commit input (Enter to start, Esc to stop)
	aiGenerating  bool            // true when AI commit message is being generated
	skeletonTick  int             // counter for skeleton pulse animation during AI generation

	// Pending discard in WIP context
	wipPendingDiscardPath      string
	wipPendingDiscardUntracked bool

	// Pending commit operation (revert/cherry-pick/reset/rebase) — hash stored for confirm dialog
	pendingOpHash   string
	pendingOpAction string // "revert", "cherry-pick", "squash", "fixup", "drop", "reset-soft", "reset-mixed", "reset-hard"

	// Compare mode — diff two commits
	compareBase *git.CommitInfo // non-nil when comparing

	// Undo — reflog-based
	pendingUndoHash string // captured at confirm time to avoid TOCTOU race
	pendingRedoHash string // captured at confirm time to avoid TOCTOU race

	// Diff cache — avoids re-running git diff on j/k navigation
	diffCache diffLRU

	// Stash diff display
	viewingStash     bool
	stashDiffIndex   int
	stashDiffContent string

	// Pull Request detail display
	viewingPR bool
	viewedPR  *hosting.PullRequest

	// Center diff viewer — shows file diffs, hunk navigation, visual mode
	diffViewer     DiffViewer
	diffFullscreen bool // true when diff uses full terminal width (sidebar + right hidden)

	// Search/filter — inline per-panel search
	searching   bool            // true when search input is active
	searchInput textinput.Model // search text input
	searchPanel logFocus        // which panel the search is filtering

	// Per-panel persisted filter queries (sidebar uses sidebar.filter directly)
	commitFilterQuery string // active git-grep filter for commit list

	// Focus
	focus logFocus

	// State
	loading     bool
	loadingMore bool // true when loading additional pages
	err         error
	spinner     components.Spinner

	// Pagination
	logPageSize int  // commits per page (default 200)
	canLoadMore bool // true if last load returned a full page

	// Keys
	navKeys       keys.NavigationKeys
	statusKeys    keys.StatusKeys
	remoteKeys    keys.RemoteOpsKeys
	commitOpsKeys keys.CommitOpsKeys

	// Dimensions
	width  int
	height int

	// Border animation
	borderAnim anim.BorderAnim
}

// panelLayout computes the three-panel widths (sidebar, center, right)
// respecting user configuration for sidebar width and center percentage.
type panelWidths struct {
	sidebar int
	center  int
	right   int
}

func (l LogPage) panelLayout() panelWidths {
	bw := styles.PanelBorderWidth

	// --- Sidebar width ---
	var sbw int
	if l.ctx != nil && l.ctx.Config != nil && l.ctx.Config.Appearance.SidebarWidth > 0 {
		// Fixed width from config
		sbw = l.ctx.Config.Appearance.SidebarWidth
	} else {
		// Percentage-based (default 15%, configurable via SidebarMaxPct)
		pct := 15
		if l.ctx != nil && l.ctx.Config != nil && l.ctx.Config.Appearance.SidebarMaxPct > 0 {
			pct = l.ctx.Config.Appearance.SidebarMaxPct
		}
		sbw = l.width * pct / 100
	}
	if sbw < 18 {
		sbw = 18
	}
	if sbw > 40 {
		sbw = 40
	}

	// --- Center / right split ---
	remaining := l.width - sbw - 3*bw
	centerPct := 70
	if l.ctx != nil && l.ctx.Config != nil && l.ctx.Config.Appearance.CenterPct > 0 {
		centerPct = l.ctx.Config.Appearance.CenterPct
		if centerPct < 30 {
			centerPct = 30
		}
		if centerPct > 90 {
			centerPct = 90
		}
	}
	cw := remaining * centerPct / 100
	if cw < 30 {
		cw = 30
	}
	if cw > remaining-20 {
		cw = remaining - 20
	}
	if cw < 10 {
		cw = 10
	}
	rw := remaining - cw
	if rw < 20 {
		rw = 20
	}

	return panelWidths{sidebar: sbw, center: cw, right: rw}
}

// sidebarWidth computes the width for the sidebar panel.
func (l LogPage) sidebarWidth() int {
	return l.panelLayout().sidebar
}

// maxGraphWidth returns the maximum number of graph cells across all commits.
func (l LogPage) maxGraphWidth() int {
	w := 0
	for _, gr := range l.graphRows {
		if len(gr.Cells) > w {
			w = len(gr.Cells)
		}
	}
	return w
}

// graphViewportCols returns how many graph cell columns fit in the current
// layout (i.e. graphColWidth - 1 for the trailing separator space).
func (l LogPage) graphViewportCols() int {
	pw := l.panelLayout()
	innerWidth := pw.center - styles.PanelPaddingWidth

	graphWidth := l.maxGraphWidth()
	graphColWidth := 0
	if graphWidth > 0 {
		graphColWidth = graphWidth + 1
	}
	maxGraph := innerWidth * 30 / 100
	if maxGraph > 40 {
		maxGraph = 40
	}
	if maxGraph < 10 {
		maxGraph = 10
	}
	if graphColWidth > maxGraph {
		graphColWidth = maxGraph
	}
	if graphColWidth <= 1 {
		return 0
	}
	return graphColWidth - 1
}

// NewLogPage creates a new log page (the main unified view).
func newSearchInput(t theme.Theme) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "search..."
	ti.Prompt = "/"
	ti.PromptStyle = lipgloss.NewStyle().Foreground(t.Yellow).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(t.Text)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(t.Overlay0)
	ti.CharLimit = 256
	return ti
}

func NewLogPage(ctx *tuictx.ProgramContext, width, height int) LogPage {
	// Create a temporary LogPage to use panelLayout() for the initial sidebar width.
	tmp := LogPage{ctx: ctx, width: width, height: height}
	pw := tmp.panelLayout()

	// Initialize diff viewer mode from config.
	var dv DiffViewer
	if ctx.Config != nil && ctx.Config.Appearance.DiffMode == "side-by-side" {
		dv.SideBySide = true
	}

	return LogPage{
		ctx:           ctx,
		repo:          ctx.Repo,
		sidebar:       NewSidebar(ctx.Repo, pw.sidebar, height),
		diffViewer:    dv,
		commitSummary: newCommitSummary(ctx.Theme),
		commitDesc:    newCommitDesc(ctx.Theme),
		navKeys:       keys.NewNavigationKeys(),
		statusKeys:    keys.NewStatusKeys(),
		remoteKeys:    keys.NewRemoteOpsKeys(),
		commitOpsKeys: keys.NewCommitOpsKeys(),
		searchInput:   newSearchInput(ctx.Theme),
		width:         width,
		height:        height,
		loading:       true,
		spinner:       components.NewSpinner().Start(),
	}
}

// isWIPSelected returns true when the cursor is on the synthetic WIP row.
func (l LogPage) isWIPSelected() bool {
	return l.hasWIP && l.cursor == 0
}

// IsEditing returns true when actively typing in an input field
// (commit message or search). Used by the app shell to suppress global
// shortcuts like q=quit.
func (l LogPage) IsEditing() bool {
	return l.commitEditing || l.searching
}

// updateContext sets ctx.ActiveKeyContext based on current panel focus so the
// help dialog shows the correct bindings.
func (l *LogPage) updateContext() {
	switch l.focus {
	case focusSidebar:
		switch l.sidebar.CurrentSectionName() {
		case "stash":
			l.ctx.ActiveKeyContext = keys.ContextStash
		case "remote":
			l.ctx.ActiveKeyContext = keys.ContextRemotes
		case "pr":
			l.ctx.ActiveKeyContext = keys.ContextPR
		default:
			l.ctx.ActiveKeyContext = keys.ContextBranches
		}
	case focusLogList:
		if l.diffViewer.Active {
			l.ctx.ActiveKeyContext = keys.ContextDiff
		} else {
			l.ctx.ActiveKeyContext = keys.ContextLog
		}
	case focusLogDetail:
		if l.isWIPSelected() {
			l.ctx.ActiveKeyContext = keys.ContextStatus
		} else {
			l.ctx.ActiveKeyContext = keys.ContextDetail
		}
	}
}

// updateBorderTargets sets animation targets for all animated borders based
// on the current focus state. Called after every key event alongside
// updateContext(). Returns a tea.Cmd to schedule the first animation tick
// if any border needs to transition.
func (l *LogPage) updateBorderTargets() tea.Cmd {
	l.borderAnim.SetFocus(anim.BorderSidebar, l.focus == focusSidebar)
	l.borderAnim.SetFocus(anim.BorderCenter, l.focus == focusLogList)
	l.borderAnim.SetFocus(anim.BorderRight, l.focus == focusLogDetail)

	// Commit box borders only apply when WIP is selected
	commitFocused := l.focus == focusLogDetail && l.isWIPSelected() && l.wipFocus == wipFocusCommit
	l.borderAnim.SetFocus(anim.BorderCommitOuter, commitFocused)
	l.borderAnim.SetFocus(anim.BorderCommitSummary, commitFocused && l.commitEditing && l.commitField == 0)
	l.borderAnim.SetFocus(anim.BorderCommitDesc, commitFocused && l.commitEditing && l.commitField == 1)

	if l.borderAnim.Active() {
		return anim.ScheduleTick()
	}
	return nil
}

// Init loads the commit log and sidebar data.
func (l LogPage) Init() tea.Cmd {
	return tea.Batch(l.loadLog(), l.sidebar.Init(), l.spinner.Tick())
}

// Update — now in log_update.go
// handleKey, handleSidebarKeys, IsSearching, handleSearchKeys — now in log_keys.go
// handleMouse — now in log_mouse.go
// loadDetailForCursor, handleListKeys, handleDetailKeys — now in commit_list.go
// handleWIPDetailKeys, handleWIPUnstagedKeys, handleWIPStagedKeys,
// handleWIPCommitKeys, submitCommit — now in wip_panel.go
// handleWIPMouseClick — now in wip_panel.go

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (l LogPage) View() string {
	t := l.ctx.Theme
	if l.loading {
		return lipgloss.NewStyle().
			Width(l.width).Height(l.height).
			Padding(2, 4).
			Background(t.Base).
			Render(l.spinner.View())
	}
	if l.err != nil {
		return lipgloss.NewStyle().
			Width(l.width).Height(l.height).
			Padding(2, 4).
			Foreground(t.Red).Background(t.Base).
			Render(fmt.Sprintf("Error: %v", l.err))
	}

	// Fullscreen diff mode — only the diff viewer, edge-to-edge.
	if l.diffFullscreen && l.diffViewer.Active {
		return l.diffViewer.Render(l.width-styles.PanelBorderWidth, l.height, true, l.borderAnim)
	}

	// Three-column layout: sidebar | center (commit list) | right (detail)
	pw := l.panelLayout()

	sidebarSearching := l.searching && l.searchPanel == focusSidebar
	sidebarSearchView := ""
	if sidebarSearching {
		sidebarSearchView = l.searchInput.View()
	}
	sidebarPane := l.sidebar.View(l.focus == focusSidebar, l.borderAnim.Color(anim.BorderSidebar, t.Surface1, t.Blue), sidebarSearching, sidebarSearchView)

	var centerPane string
	if l.diffViewer.Active {
		centerPane = l.diffViewer.Render(pw.center, l.height, l.focus == focusLogList, l.borderAnim)
	} else {
		centerPane = l.renderCommitList(pw.center, l.height)
	}

	var rightPane string
	if l.viewingPR && l.viewedPR != nil {
		rightPane = l.renderPRDetail(pw.right, l.height)
	} else if l.viewingStash {
		rightPane = l.renderStashDiff(pw.right, l.height)
	} else {
		rightPane = l.renderCommitDetail(pw.right, l.height)
	}

	layout := lipgloss.JoinHorizontal(lipgloss.Top, sidebarPane, centerPane, rightPane)

	return layout
}

// renderCommitList, renderCommitDetail, renderStashDiff — now in commit_list.go
// renderCenterDiff, parseDiffHunkNums — now in diffviewer.go

// The following large block of code has been moved to commit_list.go:
// loadLog, loadLogFiltered, loadCenterDiff, loadCommitDetail, loadStashDiff,
// doRevertCommit, doCherryPick, showResetMenu, handleMenuResult,
// handleResetMenuResult, doResetOp, doNukeWorkingTree, showRebaseConfirm,
// doRebaseAction, handleCompareToggle, showBisectMenu, handleBisectMenuResult,
// doBisectStart, doBisectMark, doBisectSkip, doBisectReset,
// doUndo, doUndoConfirmed, doRedo, copyToClipboard

// IsSearching, handleSearchKeys — now in log_keys.go
// handleDiffVisualKeys, visualSelectionRange, stageSelectedLines,
// unstageSelectedLines, stageHunk, unstageHunk — now in diffviewer.go
// openInEditor — now in log_editor.go
// loadAmendPrefill — now in wip_panel.go

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// stripHunkContext, expandTabs, truncateToWidth, horizontalSlice — now in diffviewer.go

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// fillBg forces a background color on every line of a rendered string.
// Bubbles components emit ANSI reset sequences (\x1b[0m) and their own
// background codes (\x1b[40m etc.) that kill any outer background. This
// function strips all ANSI background sequences from the input, then
// re-inserts our desired background after every reset so the color
// persists across the entire line.
func fillBg(s string, bg lipgloss.Color, width int) string {
	hex := string(bg)
	if hex != "" && hex[0] == '#' {
		hex = hex[1:]
	}
	r, g, b := hexToRGB(hex)
	bgSeq := fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
	reset := "\x1b[0m"

	// Strip any existing ANSI background sequences from the source.
	// This covers: \x1b[4Xm (basic 8), \x1b[10Xm (bright 8),
	// \x1b[48;5;Nm (256-color), and \x1b[48;2;R;G;Bm (24-bit).
	stripped := ansiBgRe.ReplaceAllString(s, "")

	lines := strings.Split(stripped, "\n")
	for i, line := range lines {
		// After every ANSI reset, re-insert our background.
		patched := strings.ReplaceAll(line, reset, reset+bgSeq)
		// Pad to full width.
		w := lipgloss.Width(line)
		pad := 0
		if w < width {
			pad = width - w
		}
		lines[i] = bgSeq + patched + strings.Repeat(" ", pad) + reset
	}
	return strings.Join(lines, "\n")
}

// ansiBgRe matches ANSI escape sequences that set a background color.
var ansiBgRe = regexp.MustCompile(
	`\x1b\[` + // CSI
		`(?:` +
		`4[0-7]` + // basic 8 bg colors: 40-47
		`|49` + // default bg
		`|10[0-7]` + // bright bg colors: 100-107
		`|48;5;\d+` + // 256-color bg
		`|48;2;\d+;\d+;\d+` + // 24-bit bg
		`)m`,
)

// hexToRGB converts a 6-character hex string to r, g, b values.
func hexToRGB(hex string) (r, g, b uint8) {
	if len(hex) != 6 {
		return 0, 0, 0
	}
	val, _ := strconv.ParseUint(hex, 16, 32)
	r = uint8((val >> 16) & 0xFF)
	g = uint8((val >> 8) & 0xFF)
	b = uint8(val & 0xFF)
	return r, g, b
}

// updateGraphMaxWidth recomputes the cached max graph cell count from graphRows.
// Called whenever graphRows is replaced (initial load, pagination, refresh)
// so the render loop can read cachedGraphMax without scanning all rows per frame.
func (l *LogPage) updateGraphMaxWidth() {
	m := 0
	for _, gr := range l.graphRows {
		if w := len(gr.Cells); w > m {
			m = w
		}
	}
	l.cachedGraphMax = m
}

// splitCommitMessage, newCommitSummary, newCommitDesc — now in wip_panel.go

var _ tea.Model = LogPage{}
