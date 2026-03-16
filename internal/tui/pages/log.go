package pages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/hosting"
	"github.com/heesungjang/kommit/internal/tui/anim"
	"github.com/heesungjang/kommit/internal/tui/components"
	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/dialog"
	"github.com/heesungjang/kommit/internal/tui/keys"
	"github.com/heesungjang/kommit/internal/tui/styles"
	"github.com/heesungjang/kommit/internal/tui/theme"
	"github.com/heesungjang/kommit/internal/tui/utils"
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

	commits      []git.CommitInfo
	cursor       int
	hasWIP       bool           // true when uncommitted changes exist; commits[0] is synthetic
	graphRows    []git.GraphRow // parallel to commits; one GraphRow per commit
	graphScrollX int            // horizontal scroll offset for graph column

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
func newSearchInput() textinput.Model {
	t := theme.Active
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
		commitSummary: newCommitSummary(),
		commitDesc:    newCommitDesc(),
		navKeys:       keys.NewNavigationKeys(),
		statusKeys:    keys.NewStatusKeys(),
		remoteKeys:    keys.NewRemoteOpsKeys(),
		commitOpsKeys: keys.NewCommitOpsKeys(),
		searchInput:   newSearchInput(),
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

// updateContext sets keys.ActiveContext based on current panel focus so the
// help dialog shows the correct bindings.
func (l *LogPage) updateContext() {
	switch l.focus {
	case focusSidebar:
		switch l.sidebar.CurrentSectionName() {
		case "stash":
			keys.ActiveContext = keys.ContextStash
		case "remote":
			keys.ActiveContext = keys.ContextRemotes
		case "pr":
			keys.ActiveContext = keys.ContextPR
		default:
			keys.ActiveContext = keys.ContextBranches
		}
	case focusLogList:
		if l.diffViewer.Active {
			keys.ActiveContext = keys.ContextDiff
		} else {
			keys.ActiveContext = keys.ContextLog
		}
	case focusLogDetail:
		if l.isWIPSelected() {
			keys.ActiveContext = keys.ContextStatus
		} else {
			keys.ActiveContext = keys.ContextDetail
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
		l.hasWIP = msg.hasWIP
		l.graphScrollX = 0 // reset horizontal scroll on reload

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
			l.commitSummary = newCommitSummary()
			l.commitDesc = newCommitDesc()
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
		l.graphRows = msg.graphRows
		l.canLoadMore = len(msg.commits) >= l.pageSize()
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
					_ = repo.StashPop(0)
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

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (l LogPage) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When search input is active, route keys to it.
	if l.searching {
		return l.handleSearchKeys(msg)
	}

	// When commit message editor is active, route ALL keys directly to it.
	// This prevents q/p/1/2/3/etc. from triggering global actions while typing.
	if l.IsEditing() {
		return l.handleWIPCommitKeys(msg)
	}

	isTab := key.Matches(msg, key.NewBinding(key.WithKeys("tab")))
	isShiftTab := key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab")))

	// Exit fullscreen diff mode when switching panels.
	if l.diffFullscreen && (isTab || isShiftTab) {
		l.diffFullscreen = false
		return l, nil
	}

	if isTab || isShiftTab {
		// When WIP is selected and we're in the detail panel, let the WIP
		// sub-focus system handle tab internally (unstaged → staged → then next panel).
		if l.focus == focusLogDetail && l.isWIPSelected() {
			return l.handleWIPDetailKeys(msg)
		}

		// Three-panel cycle: sidebar → center → right → sidebar
		if isTab {
			switch l.focus {
			case focusSidebar:
				l.focus = focusLogList
			case focusLogList:
				l.focus = focusLogDetail
				// When entering WIP detail, set initial sub-focus.
				if l.isWIPSelected() {
					if len(l.wipUnstaged) == 0 && len(l.wipStaged) > 0 {
						l.wipFocus = wipFocusStaged
					} else {
						l.wipFocus = wipFocusUnstaged
					}
				}
			case focusLogDetail:
				l.focus = focusSidebar
			}
		} else { // shift+tab
			switch l.focus {
			case focusSidebar:
				l.focus = focusLogDetail
				if l.isWIPSelected() {
					// Land on the commit area (last sub-panel in cycle)
					// Selected but not editing — user can press Enter to start typing
					l.wipFocus = wipFocusCommit
					l.commitSummary.Blur()
					l.commitDesc.Blur()
					l.commitEditing = false
				}
			case focusLogList:
				l.focus = focusSidebar
			case focusLogDetail:
				l.focus = focusLogList
			}
		}
		return l, nil
	}

	// Direct panel focus keys: 1 = sidebar, 2 = center, 3 = right
	// Exit fullscreen diff mode when using direct panel focus keys.
	if l.diffFullscreen {
		if key.Matches(msg, key.NewBinding(key.WithKeys("1", "3"))) {
			l.diffFullscreen = false
		}
	}
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("1"))):
		l.focus = focusSidebar
		return l, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("2"))):
		l.focus = focusLogList
		return l, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("3"))):
		l.focus = focusLogDetail
		if l.isWIPSelected() {
			if len(l.wipUnstaged) == 0 && len(l.wipStaged) > 0 {
				l.wipFocus = wipFocusStaged
			} else {
				l.wipFocus = wipFocusUnstaged
			}
		}
		return l, nil
	}

	// When selecting a commit in the list, clear stash/PR view mode
	if l.focus == focusLogList {
		l.viewingStash = false
		l.viewingPR = false
		l.viewedPR = nil
	}

	// Global push/pull/fetch — available when center or right panel is focused
	// AND when we're NOT on a non-WIP commit in the center panel (to avoid
	// conflicting with commit ops keys like f=fixup, p=push, etc.).
	// When sidebar is focused, let it handle p/a/etc. contextually.
	if l.focus != focusSidebar && (l.focus != focusLogList || l.isWIPSelected() || len(l.commits) == 0) {
		switch {
		case key.Matches(msg, l.remoteKeys.Push):
			return l, func() tea.Msg { return RequestGitOpMsg{Op: "push"} }
		case key.Matches(msg, l.remoteKeys.ForcePush):
			return l, func() tea.Msg { return RequestGitOpMsg{Op: "push", Force: true} }
		case key.Matches(msg, l.remoteKeys.Pull):
			return l, func() tea.Msg { return RequestGitOpMsg{Op: "pull"} }
		case key.Matches(msg, l.remoteKeys.Fetch):
			return l, func() tea.Msg { return RequestGitOpMsg{Op: "fetch"} }
		}
	}

	// Search — available from sidebar and commit list (not detail panel, not in diff mode)
	if key.Matches(msg, key.NewBinding(key.WithKeys("/"))) && !l.diffViewer.Active {
		if l.focus == focusSidebar || l.focus == focusLogList {
			l.searching = true
			l.searchPanel = l.focus
			// Set context-aware placeholder, width, and pre-fill with existing filter
			pw := l.panelLayout()
			switch l.focus {
			case focusSidebar:
				l.searchInput.Placeholder = "filter branches, tags..."
				l.searchInput.SetValue(l.sidebar.Filter())
				l.searchInput.Width = pw.sidebar - styles.PanelPaddingWidth - 2 // -2 for prompt
			case focusLogList:
				l.searchInput.Placeholder = "search commits..."
				l.searchInput.SetValue(l.commitFilterQuery)
				l.searchInput.Width = pw.center - styles.PanelPaddingWidth - 2
			}
			l.searchInput.CursorEnd()
			l.searchInput.Focus()
			return l, l.searchInput.Focus()
		}
	}

	switch l.focus {
	case focusSidebar:
		return l.handleSidebarKeys(msg)
	case focusLogList:
		return l.handleListKeys(msg)
	case focusLogDetail:
		return l.handleDetailKeys(msg)
	}
	return l, nil
}

func (l LogPage) handleSidebarKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	l.sidebar, cmd = l.sidebar.Update(msg)
	return l, cmd
}

// loadDetailForCursor, handleListKeys, handleDetailKeys — now in commit_list.go
// handleWIPDetailKeys, handleWIPUnstagedKeys, handleWIPStagedKeys,
// handleWIPCommitKeys, submitCommit — now in wip_panel.go

// ---------------------------------------------------------------------------
// Mouse handling
// ---------------------------------------------------------------------------

func (l LogPage) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	bw := styles.PanelBorderWidth
	pw := l.panelLayout()

	// Compute panel boundaries (matching View layout).
	sidebarEnd := pw.sidebar + bw // sidebar outer width
	centerEnd := sidebarEnd + pw.center + bw

	// Determine which zone the mouse is in.
	type zone int
	const (
		zoneSidebar zone = iota
		zoneCenter
		zoneRight
	)
	var z zone
	if msg.X < sidebarEnd {
		z = zoneSidebar
	} else if msg.X < centerEnd {
		z = zoneCenter
	} else {
		z = zoneRight
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		switch z {
		case zoneSidebar:
			l.focus = focusSidebar
			// Let sidebar handle scroll via key up equivalent
		case zoneCenter:
			if l.diffViewer.Active {
				// Scroll center diff up
				l.diffViewer.ScrollY -= 3
				if l.diffViewer.ScrollY < 0 {
					l.diffViewer.ScrollY = 0
				}
			} else if l.cursor > 0 {
				l.cursor--
				l.focus = focusLogList
				return l, l.loadDetailForCursor()
			}
		case zoneRight:
			// Right panel has no scrollable diff; no-op for now
			l.focus = focusLogDetail
		}
		return l, nil

	case tea.MouseButtonWheelDown:
		switch z {
		case zoneSidebar:
			l.focus = focusSidebar
		case zoneCenter:
			if l.diffViewer.Active {
				// Scroll center diff down
				l.diffViewer.ScrollY += 3
				maxScroll := len(l.diffViewer.Lines) - 10
				if maxScroll < 0 {
					maxScroll = 0
				}
				if l.diffViewer.ScrollY > maxScroll {
					l.diffViewer.ScrollY = maxScroll
				}
			} else if l.cursor < len(l.commits)-1 {
				l.cursor++
				l.focus = focusLogList
				return l, l.loadDetailForCursor()
			}
		case zoneRight:
			// Right panel has no scrollable diff; no-op for now
			l.focus = focusLogDetail
		}
		return l, nil

	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return l, nil
		}
		switch z {
		case zoneSidebar:
			l.focus = focusSidebar
		case zoneCenter:
			l.focus = focusLogList
			l.viewingStash = false
			l.viewingPR = false
			l.viewedPR = nil
			itemY := msg.Y - 2
			if itemY >= 0 {
				ph := l.height - styles.PanelBorderHeight
				visibleCount := ph - 3
				if visibleCount < 1 {
					visibleCount = 1
				}
				offset := 0
				if l.cursor >= visibleCount {
					offset = l.cursor - visibleCount + 1
				}
				if offset > len(l.commits)-visibleCount {
					offset = len(l.commits) - visibleCount
				}
				if offset < 0 {
					offset = 0
				}
				clickedIdx := offset + itemY
				if clickedIdx >= 0 && clickedIdx < len(l.commits) {
					l.cursor = clickedIdx
					return l, l.loadDetailForCursor()
				}
			}
		case zoneRight:
			l.focus = focusLogDetail
			if l.isWIPSelected() && !l.viewingStash {
				return l.handleWIPMouseClick(msg, centerEnd)
			}
		}
		return l, nil
	}

	return l, nil
}

// handleWIPMouseClick — now in wip_panel.go

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (l LogPage) View() string {
	t := theme.Active
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

// Remaining: search, editor, helpers

// IsSearching returns true when the search input is active.
func (l LogPage) IsSearching() bool {
	return l.searching
}

// handleSearchKeys handles keyboard input while the search bar is active.
func (l LogPage) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		// Cancel search — close input and clear filter for this panel
		l.searching = false
		l.searchInput.Blur()
		switch l.searchPanel {
		case focusLogList:
			if l.commitFilterQuery != "" {
				l.commitFilterQuery = ""
				return l, l.loadLog() // reload unfiltered
			}
		case focusSidebar:
			if l.sidebar.Filter() != "" {
				l.sidebar = l.sidebar.ClearFilter()
			}
		}
		return l, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		// Apply search
		l.searching = false
		l.searchInput.Blur()
		query := l.searchInput.Value()
		if query == "" {
			// Empty query — clear filter for this panel
			switch l.searchPanel {
			case focusLogList:
				l.commitFilterQuery = ""
				return l, l.loadLog()
			case focusSidebar:
				l.sidebar = l.sidebar.ClearFilter()
			}
			return l, nil
		}
		// Apply filter depending on which panel initiated search
		switch l.searchPanel {
		case focusLogList:
			l.commitFilterQuery = query
			return l, l.loadLogFiltered(query)
		case focusSidebar:
			l.sidebar = l.sidebar.SetFilter(query)
			return l, nil
		}
		return l, nil
	}

	// Forward all other keys to the text input
	var cmd tea.Cmd
	l.searchInput, cmd = l.searchInput.Update(msg)
	return l, cmd
}

// handleDiffVisualKeys, visualSelectionRange, stageSelectedLines,
// unstageSelectedLines, stageHunk, unstageHunk — now in diffviewer.go

// openInEditor returns a tea.Cmd that suspends the TUI and opens the given
// file in the user's preferred editor.
func (l LogPage) openInEditor(path string) tea.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vim"
	}

	// Build absolute path relative to the repo root.
	abs := filepath.Join(l.repo.Path(), path)
	c := exec.Command(editor, abs)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}

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

// splitCommitMessage, newCommitSummary, newCommitDesc — now in wip_panel.go

var _ tea.Model = LogPage{}
