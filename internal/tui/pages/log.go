package pages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nicholascross/opengit/internal/git"
	"github.com/nicholascross/opengit/internal/tui/dialog"
	"github.com/nicholascross/opengit/internal/tui/keys"
	"github.com/nicholascross/opengit/internal/tui/styles"
	"github.com/nicholascross/opengit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

type logLoadedMsg struct {
	commits   []git.CommitInfo
	graphRows []git.GraphRow
	hasWIP    bool // true if a synthetic WIP entry was prepended
	err       error
}

type commitDetailMsg struct {
	commit git.CommitInfo
	diff   *git.DiffResult
	err    error
}

// wipDetailMsg carries the WIP (working changes) status data.
type wipDetailMsg struct {
	unstaged []git.FileStatus
	staged   []git.FileStatus
	err      error
}

// centerDiffMsg carries a loaded diff to display in the center panel.
type centerDiffMsg struct {
	path     string
	diff     string
	err      error
	isWIP    bool // true when this diff is from WIP context (staged/unstaged)
	isStaged bool // within WIP, whether this is a staged diff
}

// wipStageResultMsg is sent after a stage/unstage/discard operation completes.
type wipStageResultMsg struct {
	err error
}

// amendPrefillMsg carries the previous commit message for amend mode prefill.
type amendPrefillMsg struct {
	message string
}

// commitOpDoneMsg is sent after a revert or cherry-pick completes successfully.
type commitOpDoneMsg struct {
	op string // "revert" or "cherry-pick"
}

// editorDoneMsg is sent when an external editor process exits.
type editorDoneMsg struct {
	err error
}

// ---------------------------------------------------------------------------
// Stash diff display
// ---------------------------------------------------------------------------

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

// wipPanelFocus tracks which sub-panel is focused within the WIP staging area.
type wipPanelFocus int

const (
	wipFocusUnstaged wipPanelFocus = iota
	wipFocusStaged
	wipFocusCommit
)

// ---------------------------------------------------------------------------
// LogPage model
// ---------------------------------------------------------------------------

// LogPage is the main unified view: sidebar (branches/tags/stash) | commit
// graph | context-sensitive detail (WIP staging / commit detail / stash diff).
type LogPage struct {
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

	// WIP staging area — interactive when WIP row is selected
	wipUnstaged       []git.FileStatus
	wipStaged         []git.FileStatus
	wipUnstagedCursor int
	wipStagedCursor   int
	wipFocus          wipPanelFocus // which sub-panel is focused within the WIP detail

	// Inline commit message area (GitKraken-style, always visible in WIP)
	commitSummary textinput.Model // single-line commit title/summary
	commitDesc    textarea.Model  // multi-line commit description/body
	commitField   int             // 0 = summary focused, 1 = description focused
	commitAmend   bool            // true when in amend mode
	commitEditing bool            // true when actively typing in commit input (Enter to start, Esc to stop)

	// Pending discard in WIP context
	wipPendingDiscardPath      string
	wipPendingDiscardUntracked bool

	// Pending commit operation (revert/cherry-pick) — hash stored for confirm dialog
	pendingOpHash string

	// Stash diff display
	viewingStash     bool
	stashDiffIndex   int
	stashDiffContent string

	// Center diff mode — when a file is selected in the right panel,
	// the center panel shows that file's diff instead of the graph.
	centerDiffMode    bool
	centerDiffLines   []string // pre-split diff lines for center panel
	centerDiffPath    string   // file path shown in diff header
	centerDiffScroll  int      // vertical scroll offset in center diff
	centerDiffScrollX int      // horizontal scroll offset in center diff (characters)

	// Hunk navigation and staging
	centerDiffHunkStarts []int      // line indices where each hunk starts (@@)
	centerDiffHunks      []git.Hunk // parsed hunk data for staging operations
	currentHunkIdx       int        // currently selected hunk index
	centerDiffIsWIP      bool       // true if this diff is from WIP context
	centerDiffIsStaged   bool       // within WIP, whether staged or unstaged

	// Line-level selection (visual mode)
	diffVisualMode   bool // true when in visual/line-selection mode
	diffVisualCursor int  // current cursor position (line index in centerDiffLines)
	diffVisualAnchor int  // anchor point where selection started

	// Search/filter
	searching   bool            // true when search input is active
	searchInput textinput.Model // search text input
	searchQuery string          // active filter query (applied after Enter)
	searchPanel logFocus        // which panel the search is filtering

	// Focus
	focus logFocus

	// State
	loading bool
	err     error

	// Keys
	navKeys       keys.NavigationKeys
	statusKeys    keys.StatusKeys
	remoteKeys    keys.RemoteOpsKeys
	commitOpsKeys keys.CommitOpsKeys

	// Dimensions
	width  int
	height int
}

// sidebarWidth computes the width for the sidebar panel.
func (l LogPage) sidebarWidth() int {
	// ~15% of total width, minimum 18, maximum 26
	w := l.width * 15 / 100
	if w < 18 {
		w = 18
	}
	if w > 26 {
		w = 26
	}
	return w
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
	bw := styles.PanelBorderWidth
	sidebarW := l.sidebarWidth()
	remaining := l.width - sidebarW - 3*bw
	centerWidth := remaining * 70 / 100
	if centerWidth < 30 {
		centerWidth = 30
	}
	if centerWidth > remaining-20 {
		centerWidth = remaining - 20
	}
	if centerWidth < 10 {
		centerWidth = 10
	}
	innerWidth := centerWidth - styles.PanelPaddingWidth

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
// newCommitSummary creates a fresh single-line text input for the commit title.
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

func NewLogPage(repo *git.Repository, width, height int) LogPage {
	sbw := width * 15 / 100
	if sbw < 18 {
		sbw = 18
	}
	if sbw > 26 {
		sbw = 26
	}
	return LogPage{
		repo:          repo,
		sidebar:       NewSidebar(repo, sbw, height),
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
		default:
			keys.ActiveContext = keys.ContextBranches
		}
	case focusLogList:
		if l.centerDiffMode {
			keys.ActiveContext = keys.ContextDiff
		} else {
			keys.ActiveContext = keys.ContextLog
		}
	case focusLogDetail:
		if l.isWIPSelected() {
			keys.ActiveContext = keys.ContextStatus
		} else {
			keys.ActiveContext = keys.ContextLog
		}
	}
}

// Init loads the commit log and sidebar data.
func (l LogPage) Init() tea.Cmd {
	return tea.Batch(l.loadLog(), l.sidebar.Init())
}

// Update handles messages for the log page.
func (l LogPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height
		l.sidebar = l.sidebar.SetSize(l.sidebarWidth(), msg.Height)
		return l, nil

	case logLoadedMsg:
		l.loading = false
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
			l.centerDiffMode = false
			l.centerDiffLines = nil
			l.centerDiffPath = ""
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

	case wipDetailMsg:
		if msg.err != nil {
			l.wipUnstaged = nil
			l.wipStaged = nil
		} else {
			l.wipUnstaged = msg.unstaged
			l.wipStaged = msg.staged
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
		l.centerDiffScroll = 0
		l.centerDiffScrollX = 0
		l.centerDiffLines = nil
		l.centerDiffPath = ""
		l.centerDiffMode = false

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
		l.centerDiffPath = msg.path
		l.centerDiffLines = strings.Split(msg.diff, "\n")
		l.centerDiffScroll = 0
		l.centerDiffScrollX = 0
		l.centerDiffMode = true
		l.centerDiffIsWIP = msg.isWIP
		l.centerDiffIsStaged = msg.isStaged
		l.currentHunkIdx = 0

		// Parse hunk positions and build Hunk structs for staging
		l.centerDiffHunkStarts = nil
		l.centerDiffHunks = nil
		var currentHunk *git.Hunk
		for i, line := range l.centerDiffLines {
			if strings.HasPrefix(line, "@@") {
				// Save previous hunk
				if currentHunk != nil {
					l.centerDiffHunks = append(l.centerDiffHunks, *currentHunk)
				}
				l.centerDiffHunkStarts = append(l.centerDiffHunkStarts, i)
				oldStart, newStart := parseDiffHunkNums(line)
				currentHunk = &git.Hunk{
					Header:   line,
					StartOld: oldStart,
					StartNew: newStart,
				}
			} else if currentHunk != nil && !strings.HasPrefix(line, "diff --git") && !strings.HasPrefix(line, "--- ") && !strings.HasPrefix(line, "+++ ") && !strings.HasPrefix(line, "index ") {
				currentHunk.Lines = append(currentHunk.Lines, line)
			}
		}
		if currentHunk != nil {
			l.centerDiffHunks = append(l.centerDiffHunks, *currentHunk)
		}

		// Compute CountOld/CountNew for each hunk from its lines
		for i := range l.centerDiffHunks {
			h := &l.centerDiffHunks[i]
			for _, ln := range h.Lines {
				if len(ln) == 0 {
					h.CountOld++
					h.CountNew++
					continue
				}
				switch ln[0] {
				case '+':
					h.CountNew++
				case '-':
					h.CountOld++
				default:
					h.CountOld++
					h.CountNew++
				}
			}
		}

		return l, nil

	case wipStageResultMsg:
		// After stage/unstage/discard, reload WIP data.
		// If we're in diff mode, also reload the diff to reflect the change.
		if l.centerDiffMode && l.centerDiffIsWIP {
			return l, tea.Batch(l.loadWIPDetail(), l.loadCenterDiff())
		}
		return l, l.loadWIPDetail()

	case commitOpDoneMsg:
		// After revert or cherry-pick, show success and refresh
		label := msg.op
		if label == "cherry-pick" {
			label = "Cherry-pick"
		} else {
			label = "Revert"
		}
		return l, tea.Batch(
			func() tea.Msg { return RequestToastMsg{Message: label + " complete"} },
			func() tea.Msg { return RefreshStatusMsg{} },
		)

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
		l.centerDiffMode = false
		l.centerDiffScroll = 0
		l.centerDiffScrollX = 0
		l.centerDiffLines = nil
		l.centerDiffPath = ""
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
		// Route sidebar confirm results
		if strings.HasPrefix(msg.ID, "sidebar-") {
			var cmd tea.Cmd
			l.sidebar, cmd = l.sidebar.HandleDialogResult(dialogConfirmResultMsg{ID: msg.ID, Confirmed: msg.Confirmed})
			return l, cmd
		}
		return l, nil

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
	case SidebarViewStashMsg:
		// Switch right panel to stash diff view
		l.viewingStash = true
		l.stashDiffIndex = msg.Index
		l.stashDiffContent = ""
		return l, l.loadStashDiff(msg.Index)

	case stashDiffMsg:
		l.stashDiffIndex = msg.index
		l.centerDiffScroll = 0
		l.centerDiffScrollX = 0
		if msg.err != nil {
			l.stashDiffContent = "Error loading stash diff: " + msg.err.Error()
			l.centerDiffMode = false
			l.centerDiffLines = nil
			l.centerDiffPath = ""
		} else {
			l.stashDiffContent = msg.diff
			l.centerDiffLines = strings.Split(msg.diff, "\n")
			l.centerDiffPath = fmt.Sprintf("stash@{%d}", msg.index)
			l.centerDiffMode = true
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

	case RefreshStatusMsg:
		// Auto-refresh detected external changes — reload log and sidebar.
		return l, tea.Batch(l.loadLog(), l.sidebar.Refresh())

	case tea.MouseMsg:
		return l.handleMouse(msg)

	case tea.KeyMsg:
		m, cmd := l.handleKey(msg)
		if lp, ok := m.(LogPage); ok {
			lp.updateContext()
			return lp, cmd
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

	// When selecting a commit in the list, clear stash view mode
	if l.focus == focusLogList {
		l.viewingStash = false
	}

	// Global push/pull/fetch — available when center or right panel is focused.
	// When sidebar is focused, let it handle p/a/etc. contextually.
	if l.focus != focusSidebar {
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

	// Search — available from any panel (not in diff mode or visual mode)
	if key.Matches(msg, key.NewBinding(key.WithKeys("/"))) && !l.centerDiffMode {
		l.searching = true
		l.searchPanel = l.focus
		l.searchInput.SetValue("")
		l.searchInput.Focus()
		return l, l.searchInput.Focus()
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

func (l LogPage) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When center is showing a diff, j/k scroll the diff and Esc exits.
	if l.centerDiffMode {
		maxScroll := len(l.centerDiffLines) - 10
		if maxScroll < 0 {
			maxScroll = 0
		}

		// Visual (line-selection) mode — `v` to toggle
		if l.diffVisualMode {
			return l.handleDiffVisualKeys(msg)
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			l.centerDiffMode = false
			l.centerDiffLines = nil
			l.centerDiffPath = ""
			l.centerDiffScroll = 0
			l.centerDiffScrollX = 0
			l.diffVisualMode = false
			l.focus = focusLogDetail // return focus to right panel
			return l, nil
		case key.Matches(msg, l.navKeys.Down):
			if l.centerDiffScroll < maxScroll {
				l.centerDiffScroll++
			}
			return l, nil
		case key.Matches(msg, l.navKeys.Up):
			if l.centerDiffScroll > 0 {
				l.centerDiffScroll--
			}
			return l, nil
		case key.Matches(msg, l.navKeys.PageDown):
			l.centerDiffScroll += 10
			if l.centerDiffScroll > maxScroll {
				l.centerDiffScroll = maxScroll
			}
			return l, nil
		case key.Matches(msg, l.navKeys.PageUp):
			l.centerDiffScroll -= 10
			if l.centerDiffScroll < 0 {
				l.centerDiffScroll = 0
			}
			return l, nil
		case key.Matches(msg, l.navKeys.Home):
			l.centerDiffScroll = 0
			return l, nil
		case key.Matches(msg, l.navKeys.End):
			l.centerDiffScroll = maxScroll
			return l, nil
		case key.Matches(msg, l.navKeys.Left): // h — pan left
			l.centerDiffScrollX -= 4
			if l.centerDiffScrollX < 0 {
				l.centerDiffScrollX = 0
			}
			return l, nil
		case key.Matches(msg, l.navKeys.Right): // l — pan right
			l.centerDiffScrollX += 4
			return l, nil

		// Toggle visual (line-selection) mode
		case key.Matches(msg, key.NewBinding(key.WithKeys("v"))):
			if l.centerDiffIsWIP && len(l.centerDiffHunkStarts) > 0 {
				l.diffVisualMode = true
				l.diffVisualCursor = l.centerDiffScroll
				l.diffVisualAnchor = l.centerDiffScroll
			}
			return l, nil

		// Hunk navigation
		case key.Matches(msg, key.NewBinding(key.WithKeys("n", "]"))):
			if len(l.centerDiffHunkStarts) > 0 && l.currentHunkIdx < len(l.centerDiffHunkStarts)-1 {
				l.currentHunkIdx++
				l.centerDiffScroll = l.centerDiffHunkStarts[l.currentHunkIdx]
				if l.centerDiffScroll > maxScroll {
					l.centerDiffScroll = maxScroll
				}
			}
			return l, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("N", "["))):
			if len(l.centerDiffHunkStarts) > 0 && l.currentHunkIdx > 0 {
				l.currentHunkIdx--
				l.centerDiffScroll = l.centerDiffHunkStarts[l.currentHunkIdx]
				if l.centerDiffScroll > maxScroll {
					l.centerDiffScroll = maxScroll
				}
			}
			return l, nil

		// Hunk staging — only in WIP diff context
		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			if l.centerDiffIsWIP && !l.centerDiffIsStaged && l.currentHunkIdx < len(l.centerDiffHunks) {
				hunk := l.centerDiffHunks[l.currentHunkIdx]
				path := l.centerDiffPath
				return l, l.stageHunk(path, hunk)
			}
			return l, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("u"))):
			if l.centerDiffIsWIP && l.centerDiffIsStaged && l.currentHunkIdx < len(l.centerDiffHunks) {
				hunk := l.centerDiffHunks[l.currentHunkIdx]
				path := l.centerDiffPath
				return l, l.unstageHunk(path, hunk)
			}
			return l, nil
		}
		return l, nil
	}

	// Normal commit list navigation (graph mode).
	switch {
	case key.Matches(msg, l.navKeys.Down):
		if l.cursor < len(l.commits)-1 {
			l.cursor++
			return l, l.loadDetailForCursor()
		}
	case key.Matches(msg, l.navKeys.Up):
		if l.cursor > 0 {
			l.cursor--
			return l, l.loadDetailForCursor()
		}
	case key.Matches(msg, l.navKeys.PageDown):
		l.cursor += 10
		if l.cursor >= len(l.commits) {
			l.cursor = len(l.commits) - 1
		}
		if l.cursor < 0 {
			l.cursor = 0
		}
		return l, l.loadDetailForCursor()
	case key.Matches(msg, l.navKeys.PageUp):
		l.cursor -= 10
		if l.cursor < 0 {
			l.cursor = 0
		}
		return l, l.loadDetailForCursor()
	case key.Matches(msg, l.navKeys.Home):
		l.cursor = 0
		return l, l.loadDetailForCursor()
	case key.Matches(msg, l.navKeys.End):
		if len(l.commits) > 0 {
			l.cursor = len(l.commits) - 1
			return l, l.loadDetailForCursor()
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
		}
	}

	return l, nil
}

func (l LogPage) handleDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if l.isWIPSelected() {
		return l.handleWIPDetailKeys(msg)
	}
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		l.centerDiffMode = false
		l.centerDiffLines = nil
		l.centerDiffPath = ""
		l.centerDiffScroll = 0
		l.centerDiffScrollX = 0
		return l, nil
	case key.Matches(msg, l.navKeys.Select): // Enter — view diff for selected file
		if len(l.detailFiles) > 0 && l.detailFileCursor < len(l.detailFiles) {
			l.centerDiffScroll = 0
			l.centerDiffScrollX = 0
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
		if l.centerDiffMode {
			l.centerDiffScroll += 10
			max := len(l.centerDiffLines) - 10
			if max < 0 {
				max = 0
			}
			if l.centerDiffScroll > max {
				l.centerDiffScroll = max
			}
		}
	case key.Matches(msg, l.navKeys.PageUp):
		if l.centerDiffMode {
			l.centerDiffScroll -= 10
			if l.centerDiffScroll < 0 {
				l.centerDiffScroll = 0
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
		// Open selected file in editor (past commit context — file may not exist on disk)
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
	return l, nil
}

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
		l.centerDiffMode = false
		l.centerDiffLines = nil
		l.centerDiffPath = ""
		l.centerDiffScroll = 0
		l.centerDiffScrollX = 0
		return l, nil
	case key.Matches(msg, l.navKeys.Select): // Enter — view diff for selected file
		if len(l.wipUnstaged) > 0 && l.wipUnstagedCursor < len(l.wipUnstaged) {
			l.centerDiffScroll = 0
			l.centerDiffScrollX = 0
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
		if l.centerDiffMode {
			l.centerDiffScroll += 10
			max := len(l.centerDiffLines) - 10
			if max < 0 {
				max = 0
			}
			if l.centerDiffScroll > max {
				l.centerDiffScroll = max
			}
		}
		return l, nil
	case key.Matches(msg, l.navKeys.PageUp):
		if l.centerDiffMode {
			l.centerDiffScroll -= 10
			if l.centerDiffScroll < 0 {
				l.centerDiffScroll = 0
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
		l.centerDiffMode = false
		l.centerDiffLines = nil
		l.centerDiffPath = ""
		l.centerDiffScroll = 0
		l.centerDiffScrollX = 0
		return l, nil
	case key.Matches(msg, l.navKeys.Select): // Enter — view diff for selected file
		if len(l.wipStaged) > 0 && l.wipStagedCursor < len(l.wipStaged) {
			l.centerDiffScroll = 0
			l.centerDiffScrollX = 0
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
		if l.centerDiffMode {
			l.centerDiffScroll += 10
			max := len(l.centerDiffLines) - 10
			if max < 0 {
				max = 0
			}
			if l.centerDiffScroll > max {
				l.centerDiffScroll = max
			}
		}
		return l, nil
	case key.Matches(msg, l.navKeys.PageUp):
		if l.centerDiffMode {
			l.centerDiffScroll -= 10
			if l.centerDiffScroll < 0 {
				l.centerDiffScroll = 0
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

func (l LogPage) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	bw := styles.PanelBorderWidth

	// Compute panel boundaries (matching View layout).
	sidebarW := l.sidebarWidth()
	sidebarEnd := sidebarW + bw // sidebar outer width

	remaining := l.width - sidebarW - 3*bw // 3 panels × PanelBorderWidth each
	centerWidth := remaining * 70 / 100
	if centerWidth < 30 {
		centerWidth = 30
	}
	if centerWidth > remaining-20 {
		centerWidth = remaining - 20
	}
	if centerWidth < 10 {
		centerWidth = 10
	}
	centerEnd := sidebarEnd + centerWidth + bw

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
			if l.centerDiffMode {
				// Scroll center diff up
				l.centerDiffScroll -= 3
				if l.centerDiffScroll < 0 {
					l.centerDiffScroll = 0
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
			if l.centerDiffMode {
				// Scroll center diff down
				l.centerDiffScroll += 3
				max := len(l.centerDiffLines) - 10
				if max < 0 {
					max = 0
				}
				if l.centerDiffScroll > max {
					l.centerDiffScroll = max
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
			l.centerDiffScroll = 0
			l.centerDiffScrollX = 0
			return l, l.loadCenterDiff()
		}
	} else if relY >= stagedFileStart && relY < stagedFileEnd && len(l.wipStaged) > 0 {
		idx := relY - stagedFileStart
		if idx < len(l.wipStaged) {
			l.wipFocus = wipFocusStaged
			l.wipStagedCursor = idx
			l.centerDiffScroll = 0
			l.centerDiffScrollX = 0
			return l, l.loadCenterDiff()
		}
	}

	return l, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (l LogPage) View() string {
	t := theme.Active
	if l.loading {
		return lipgloss.NewStyle().
			Width(l.width).Height(l.height).
			Padding(2, 4).
			Foreground(t.Subtext0).Background(t.Base).
			Render("Loading...")
	}
	if l.err != nil {
		return lipgloss.NewStyle().
			Width(l.width).Height(l.height).
			Padding(2, 4).
			Foreground(t.Red).Background(t.Base).
			Render(fmt.Sprintf("Error: %v", l.err))
	}

	bw := styles.PanelBorderWidth

	// Three-column layout: sidebar | center (commit list) | right (detail)
	sidebarW := l.sidebarWidth()

	remaining := l.width - sidebarW - 3*bw // 3 panels × PanelBorderWidth each
	centerWidth := remaining * 70 / 100
	if centerWidth < 30 {
		centerWidth = 30
	}
	if centerWidth > remaining-20 {
		centerWidth = remaining - 20
	}
	if centerWidth < 10 {
		centerWidth = 10
	}
	rightWidth := remaining - centerWidth
	if rightWidth < 20 {
		rightWidth = 20
	}

	sidebarPane := l.sidebar.View(l.focus == focusSidebar)

	var centerPane string
	if l.centerDiffMode {
		centerPane = l.renderCenterDiff(centerWidth, l.height)
	} else {
		centerPane = l.renderCommitList(centerWidth, l.height)
	}

	var rightPane string
	if l.viewingStash {
		rightPane = l.renderStashDiff(rightWidth, l.height)
	} else {
		rightPane = l.renderCommitDetail(rightWidth, l.height)
	}

	layout := lipgloss.JoinHorizontal(lipgloss.Top, sidebarPane, centerPane, rightPane)

	// Overlay search bar at the bottom when active
	if l.searching {
		searchBar := l.renderSearchBar()
		// Replace the last line of the layout with the search bar
		lines := strings.Split(layout, "\n")
		if len(lines) > 0 {
			lines[len(lines)-1] = searchBar
		}
		layout = strings.Join(lines, "\n")
	} else if l.searchQuery != "" {
		// Show active filter indicator
		filterBar := l.renderFilterIndicator()
		lines := strings.Split(layout, "\n")
		if len(lines) > 0 {
			lines[len(lines)-1] = filterBar
		}
		layout = strings.Join(lines, "\n")
	}

	return layout
}

// renderSearchBar renders the search input bar.
func (l LogPage) renderSearchBar() string {
	t := theme.Active
	input := l.searchInput.View()
	bar := lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Surface0).
		Width(l.width).
		Render(input)
	return bar
}

// renderFilterIndicator shows a small bar indicating an active search filter.
func (l LogPage) renderFilterIndicator() string {
	t := theme.Active
	label := lipgloss.NewStyle().Foreground(t.Yellow).Bold(true).Render(" FILTER: ")
	query := lipgloss.NewStyle().Foreground(t.Text).Render(l.searchQuery)
	hint := lipgloss.NewStyle().Foreground(t.Overlay0).Render("  (/ to search, Esc in / to clear)")

	content := label + query + hint
	w := lipgloss.Width(content)
	pad := ""
	if w < l.width {
		pad = strings.Repeat(" ", l.width-w)
	}
	return lipgloss.NewStyle().Background(t.Surface0).Render(content + pad)
}

func (l LogPage) renderCommitList(width, height int) string {
	t := theme.Active
	innerWidth := width - styles.PanelPaddingWidth
	commitCount := len(l.commits)
	if l.hasWIP {
		commitCount-- // exclude synthetic WIP entry from count
	}
	titleStr := styles.PanelTitle(
		fmt.Sprintf("Commits (%d)", commitCount),
		"2", l.focus == focusLogList, innerWidth,
	)

	var lines []string
	if len(l.commits) == 0 {
		lines = append(lines, styles.DimStyle().Width(innerWidth).Render("  No commits yet. Make your first commit with 'c' on the Status page."))
	}

	// Viewport windowing: only render visible commits, following the cursor.
	ph := height - styles.PanelBorderHeight
	// Visible lines: panel height minus title (1) minus title gap (1) minus hints area (2: empty + hints)
	visibleCount := ph - 4
	if visibleCount < 1 {
		visibleCount = 1
	}

	// Compute viewport offset so the cursor is always visible.
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
	end := offset + visibleCount
	if end > len(l.commits) {
		end = len(l.commits)
	}

	// Compute max graph width across ALL commits (not just visible) for consistent column layout.
	graphWidth := 0
	for _, gr := range l.graphRows {
		w := len(gr.Cells)
		if w > graphWidth {
			graphWidth = w
		}
	}
	// Each graph cell takes 1 character, plus 1 space separator after graph
	graphColWidth := 0
	if graphWidth > 0 {
		graphColWidth = graphWidth + 1 // graph chars + 1 space
	}
	// Cap graph to ~30% of inner width, bounded [10, 40]
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

	hashWidth := 8
	dateWidth := 10
	authorWidth := 15
	subjectWidth := innerWidth - graphColWidth - hashWidth - dateWidth - authorWidth - 5
	if subjectWidth < 10 {
		subjectWidth = 10
	}

	for i := offset; i < end; i++ {
		c := l.commits[i]
		isWIP := l.hasWIP && i == 0
		selected := i == l.cursor && l.focus == focusLogList
		bg := t.Base
		if selected {
			bg = t.Surface1
		}
		bgS := lipgloss.NewStyle().Background(bg)

		// Render graph prefix with horizontal scroll viewport
		graphStr := ""
		if i < len(l.graphRows) && graphColWidth > 0 {
			gr := l.graphRows[i]
			displayCols := graphColWidth - 1 // -1 for trailing separator space
			scrollX := l.graphScrollX
			truncLeft := scrollX > 0
			truncRight := graphWidth > scrollX+displayCols

			// Determine the cell range to render
			startCol := scrollX
			endCol := scrollX + displayCols
			if truncLeft {
				startCol++ // reserve first column for ◂ indicator
			}
			if truncRight {
				endCol-- // reserve last column for ▸ indicator
			}

			var graphParts []string

			// Left scroll indicator
			if truncLeft {
				graphParts = append(graphParts, lipgloss.NewStyle().
					Foreground(t.Subtext0).Background(bg).Render("◂"))
			}

			// Render visible graph cells
			for j := startCol; j < endCol; j++ {
				if j < len(gr.Cells) {
					cell := gr.Cells[j]
					color := styles.GraphColor(cell.Column)
					graphParts = append(graphParts, lipgloss.NewStyle().
						Foreground(color).Background(bg).Render(cell.Char))
				} else {
					graphParts = append(graphParts, bgS.Render(" "))
				}
			}

			// Right scroll indicator
			if truncRight {
				graphParts = append(graphParts, lipgloss.NewStyle().
					Foreground(t.Subtext0).Background(bg).Render("▸"))
			}

			// Pad if total parts < displayCols (graph fits entirely)
			for len(graphParts) < displayCols {
				graphParts = append(graphParts, bgS.Render(" "))
			}

			graphStr = lipgloss.JoinHorizontal(lipgloss.Top, graphParts...)
		}

		sp := bgS.Render(" ")

		if isWIP {
			// WIP row — special rendering
			wipLabel := "Working Changes"
			fileCount := len(l.wipStaged) + len(l.wipUnstaged)
			if fileCount > 0 {
				wipLabel += fmt.Sprintf(" [%d files]", fileCount)
			}
			wipSubject := lipgloss.NewStyle().Foreground(t.Peach).Background(bg).Italic(true).
				Width(innerWidth - graphColWidth - 2).Render(wipLabel)

			var line string
			if graphStr != "" {
				line = lipgloss.JoinHorizontal(lipgloss.Top, sp, graphStr, sp, wipSubject)
			} else {
				line = lipgloss.JoinHorizontal(lipgloss.Top, sp, wipSubject)
			}

			lineStyle := lipgloss.NewStyle().Background(bg).Width(innerWidth)
			if selected {
				lineStyle = lineStyle.Bold(true)
			}
			line = lineStyle.Render(line)
			lines = append(lines, line)
			continue
		}

		hash := lipgloss.NewStyle().Foreground(t.Yellow).Background(bg).Width(hashWidth).Render(c.ShortHash)
		date := lipgloss.NewStyle().Foreground(t.Overlay0).Background(bg).Width(dateWidth).Render(c.Date.Format("2006-01-02"))
		author := lipgloss.NewStyle().Foreground(t.Teal).Background(bg).Width(authorWidth).Render(truncate(c.Author, authorWidth))

		// Render ref badges and compute remaining subject width
		badges := styles.RenderRefBadges(c.Refs, bg)
		badgeWidth := lipgloss.Width(badges)
		effectiveSubjectWidth := subjectWidth
		if badgeWidth > 0 {
			effectiveSubjectWidth = subjectWidth - badgeWidth - 1
			if effectiveSubjectWidth < 4 {
				// Badges too wide — skip them to avoid line overflow
				badges = ""
				badgeWidth = 0
				effectiveSubjectWidth = subjectWidth
			}
		}
		subject := lipgloss.NewStyle().Foreground(t.Text).Background(bg).Width(effectiveSubjectWidth).Render(truncate(c.Subject, effectiveSubjectWidth))

		var line string
		if graphStr != "" {
			if badgeWidth > 0 {
				line = lipgloss.JoinHorizontal(lipgloss.Top, sp, graphStr, sp, hash, sp, date, sp, author, sp, badges, sp, subject)
			} else {
				line = lipgloss.JoinHorizontal(lipgloss.Top, sp, graphStr, sp, hash, sp, date, sp, author, sp, subject)
			}
		} else {
			sp2 := bgS.Render("  ")
			if badgeWidth > 0 {
				line = lipgloss.JoinHorizontal(lipgloss.Top, sp2, hash, sp, date, sp, author, sp, badges, sp, subject)
			} else {
				line = lipgloss.JoinHorizontal(lipgloss.Top, sp2, hash, sp, date, sp, author, sp, subject)
			}
		}

		lineStyle := lipgloss.NewStyle().Background(bg).Width(innerWidth)
		if selected {
			lineStyle = lineStyle.Bold(true)
		}
		line = lineStyle.Render(line)

		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	// Pad content to exactly visibleCount lines so hints are pinned to the bottom
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > visibleCount {
		contentLines = contentLines[:visibleCount]
	}
	bgEmpty := lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render("")
	for len(contentLines) < visibleCount {
		contentLines = append(contentLines, bgEmpty)
	}
	content = strings.Join(contentLines, "\n")

	scrollInfo := ""
	if len(l.commits) > visibleCount {
		scrollInfo = fmt.Sprintf("  [%d/%d]", l.cursor+1, len(l.commits))
	}
	graphHint := ""
	if l.maxGraphWidth() > l.graphViewportCols() {
		graphHint = "  H/L:scroll graph"
	}
	hints := styles.KeyHintStyle().Background(t.Base).Width(innerWidth).Render(
		"j/k:navigate  g/G:top/bottom" + graphHint + scrollInfo,
	)
	titleGap := lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render("")
	emptyLine := lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render("")
	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, emptyLine, hints)
	// Clip to panel height so all panels stay the same outer height.
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.PanelStyle(l.focus == focusLogList).Width(width).Height(ph).Render(full)
}

// renderCenterDiff renders the center panel as a diff view (replacing the
// commit graph) when a file is selected in the right panel.
func (l LogPage) renderCenterDiff(width, height int) string {
	t := theme.Active
	iw := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight

	// Title shows the file path
	titleLabel := "Diff"
	if l.centerDiffPath != "" {
		titleLabel = "Diff: " + l.centerDiffPath
	}
	// Truncate label to fit within the title area (leave room for [2] tag)
	maxLabel := iw - 5
	if maxLabel > 0 && len(titleLabel) > maxLabel {
		titleLabel = titleLabel[:maxLabel] + "…"
	}
	focused := l.focus == focusLogList
	titleStr := styles.PanelTitle(titleLabel, "2", focused, iw)

	titleGap := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")

	if len(l.centerDiffLines) == 0 {
		content := styles.DimStyle().Width(iw).Render("  No diff content")
		full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content)
		if cl := strings.Split(full, "\n"); len(cl) > ph {
			full = strings.Join(cl[:ph], "\n")
		}
		return styles.PanelStyle(focused).Width(width).Height(ph).Render(full)
	}

	// Apply scroll offset
	startLine := l.centerDiffScroll
	if startLine > len(l.centerDiffLines) {
		startLine = len(l.centerDiffLines)
	}
	if startLine < 0 {
		startLine = 0
	}

	// Visible lines: ph minus title (1) minus title gap (1) minus hints (2: empty + hint line)
	contentHeight := ph - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	endLine := startLine + contentHeight
	if endLine > len(l.centerDiffLines) {
		endLine = len(l.centerDiffLines)
	}

	// --- Line number computation ---
	// We need to compute line numbers for ALL lines up to endLine,
	// starting from the beginning of the diff. Track old/new counters
	// through the entire content up to endLine.
	const gutterWidth = 11 // "NNNN NNNN │" = 4+1+4+1+1 = 11
	contentWidth := iw - gutterWidth
	if contentWidth < 10 {
		contentWidth = 10
	}

	// State for line number tracking
	oldNum := 0
	newNum := 0
	inHunk := false
	hunkIdx := -1 // which hunk the current line belongs to

	var sections []string
	for i := 0; i < endLine; i++ {
		line := ""
		if i < len(l.centerDiffLines) {
			line = l.centerDiffLines[i]
		}

		lineType := byte(' ')
		if len(line) > 0 {
			lineType = line[0]
		}

		// Detect line category
		isDiffHeader := strings.HasPrefix(line, "diff --git")
		isFileOld := strings.HasPrefix(line, "--- ")
		isFileNew := strings.HasPrefix(line, "+++ ")
		isIndex := strings.HasPrefix(line, "index ")
		isSimilarity := strings.HasPrefix(line, "similarity ") || strings.HasPrefix(line, "rename ") || strings.HasPrefix(line, "new file") || strings.HasPrefix(line, "deleted file") || strings.HasPrefix(line, "old mode") || strings.HasPrefix(line, "new mode")
		isHunkHeader := strings.HasPrefix(line, "@@")

		// Parse hunk header for line numbers
		if isHunkHeader {
			inHunk = true
			hunkIdx++
			oldNum, newNum = parseDiffHunkNums(line)
		}
		isCurrentHunk := hunkIdx >= 0 && hunkIdx == l.currentHunkIdx

		// Only render visible lines (i >= startLine)
		if i < startLine {
			// Still need to update line counters for non-visible lines
			if inHunk && !isHunkHeader && !isDiffHeader && !isFileOld && !isFileNew && !isIndex && !isSimilarity {
				switch lineType {
				case '+':
					newNum++
				case '-':
					oldNum++
				default: // context
					oldNum++
					newNum++
				}
			}
			continue
		}

		// Render the gutter + content for this visible line
		var gutterStr string
		var contentStr string

		scrollX := l.centerDiffScrollX

		if isDiffHeader || isFileOld || isFileNew {
			// File header lines: no line numbers, bold text
			gutterStr = styles.DiffGutterSepStyle(' ').Render(strings.Repeat(" ", gutterWidth))
			rendered := expandTabs(line, 4)
			rendered = horizontalSlice(rendered, scrollX, contentWidth)
			contentStr = styles.DiffFileHeaderStyle().Width(contentWidth).Render(rendered)
		} else if isIndex || isSimilarity {
			// Metadata lines: no line numbers, dimmed
			gutterStr = styles.DiffGutterSepStyle(' ').Render(strings.Repeat(" ", gutterWidth))
			rendered := expandTabs(line, 4)
			rendered = horizontalSlice(rendered, scrollX, contentWidth)
			contentStr = styles.DiffMetaStyle().Width(contentWidth).Render(rendered)
		} else if isHunkHeader {
			// Hunk header: no line numbers, special styling
			// Highlight the current hunk header with a distinct marker
			if isCurrentHunk {
				gutterStr = styles.DiffGutterSepStyle('@').Render("►" + strings.Repeat("─", gutterWidth-2) + "┤")
			} else {
				gutterStr = styles.DiffGutterSepStyle('@').Render(strings.Repeat("─", gutterWidth-1) + "┤")
			}
			rendered := expandTabs(line, 4)
			rendered = horizontalSlice(rendered, scrollX, contentWidth)
			if isCurrentHunk {
				contentStr = styles.DiffCurrentHunkStyle().Width(contentWidth).Render(rendered)
			} else {
				contentStr = styles.DiffLineStyle('@').Width(contentWidth).Render(rendered)
			}
		} else if inHunk {
			// Content lines within a hunk: show line numbers
			oldStr := "    "
			newStr := "    "
			switch lineType {
			case '+':
				newStr = fmt.Sprintf("%4d", newNum)
				newNum++
			case '-':
				oldStr = fmt.Sprintf("%4d", oldNum)
				oldNum++
			default: // context line
				oldStr = fmt.Sprintf("%4d", oldNum)
				newStr = fmt.Sprintf("%4d", newNum)
				oldNum++
				newNum++
			}

			numOldStyled := styles.DiffLineNumStyle(lineType).Render(oldStr)
			space := styles.DiffGutterSepStyle(lineType).Render(" ")
			numNewStyled := styles.DiffLineNumStyle(lineType).Render(newStr)
			sep := styles.DiffGutterSepStyle(lineType).Render(" │")
			gutterStr = lipgloss.JoinHorizontal(lipgloss.Top, numOldStyled, space, numNewStyled, sep)

			rendered := expandTabs(line, 4)
			rendered = horizontalSlice(rendered, scrollX, contentWidth)
			contentStr = styles.DiffLineStyle(lineType).Width(contentWidth).Render(rendered)
		} else {
			// Lines before first hunk (shouldn't happen often)
			gutterStr = styles.DiffGutterSepStyle(' ').Render(strings.Repeat(" ", gutterWidth))
			rendered := expandTabs(line, 4)
			rendered = horizontalSlice(rendered, scrollX, contentWidth)
			contentStr = styles.DiffLineStyle(lineType).Width(contentWidth).Render(rendered)
		}

		// Visual mode: highlight selected lines with a distinct marker
		if l.diffVisualMode {
			vLo, vHi := l.visualSelectionRange()
			if i >= vLo && i <= vHi {
				// Add selection indicator in gutter
				marker := lipgloss.NewStyle().Foreground(t.Blue).Background(t.Surface1).Bold(true).Render("▌")
				fullLine := lipgloss.JoinHorizontal(lipgloss.Top, marker, gutterStr, contentStr)
				sections = append(sections, fullLine)
				continue
			}
			// Show cursor position when outside selection
			if i == l.diffVisualCursor {
				marker := lipgloss.NewStyle().Foreground(t.Blue).Background(t.Base).Render("▸")
				fullLine := lipgloss.JoinHorizontal(lipgloss.Top, marker, gutterStr, contentStr)
				sections = append(sections, fullLine)
				continue
			}
		}

		fullLine := lipgloss.JoinHorizontal(lipgloss.Top, gutterStr, contentStr)
		sections = append(sections, fullLine)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Pad content to exactly contentHeight lines so hints are pinned to the bottom
	diffContentLines := strings.Split(content, "\n")
	if len(diffContentLines) > contentHeight {
		diffContentLines = diffContentLines[:contentHeight]
	}
	bgEmpty := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")
	for len(diffContentLines) < contentHeight {
		diffContentLines = append(diffContentLines, bgEmpty)
	}
	content = strings.Join(diffContentLines, "\n")

	// Scroll and hint line
	scrollInfo := ""
	if len(l.centerDiffLines) > contentHeight {
		scrollInfo = fmt.Sprintf("  [%d/%d lines]", startLine+1, len(l.centerDiffLines))
	}
	emptyLine := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")

	var hintParts string
	if l.diffVisualMode {
		vLo, vHi := l.visualSelectionRange()
		sel := vHi - vLo + 1
		hintParts = fmt.Sprintf("VISUAL  j/k:extend  %d lines", sel)
		if l.centerDiffIsStaged {
			hintParts += "  u:unstage lines"
		} else {
			hintParts += "  s:stage lines"
		}
		hintParts += "  Esc/v:cancel"
	} else {
		hintParts = "j/k:scroll  h/l:pan  Esc:back  g/G:top/bottom"
		if len(l.centerDiffHunkStarts) > 0 {
			hunkInfo := fmt.Sprintf("  n/N:hunk [%d/%d]", l.currentHunkIdx+1, len(l.centerDiffHunkStarts))
			hintParts += hunkInfo
		}
		if l.centerDiffIsWIP {
			if l.centerDiffIsStaged {
				hintParts += "  u:unstage hunk"
			} else {
				hintParts += "  s:stage hunk"
			}
			hintParts += "  v:select lines"
		}
		hintParts += scrollInfo
	}
	hints := styles.KeyHintStyle().Background(t.Base).Width(iw).Render(hintParts)

	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, emptyLine, hints)
	// Clip to panel height
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.PanelStyle(focused).Width(width).Height(ph).Render(full)
}

// parseDiffHunkNums extracts the old and new starting line numbers from a
// @@ -old,count +new,count @@ hunk header line.
func parseDiffHunkNums(line string) (oldStart, newStart int) {
	// Find the range info between @@ markers: "@@ -124,7 +148,8 @@ ..."
	idx := strings.Index(line, "@@")
	if idx < 0 {
		return 1, 1
	}
	rest := line[idx+2:]
	idx2 := strings.Index(rest, "@@")
	if idx2 <= 0 {
		return 1, 1
	}
	rangeInfo := strings.TrimSpace(rest[:idx2])
	parts := strings.Fields(rangeInfo)
	for _, part := range parts {
		if strings.HasPrefix(part, "-") {
			nums := strings.SplitN(part[1:], ",", 2)
			if len(nums) >= 1 {
				if n, err := strconv.Atoi(nums[0]); err == nil {
					oldStart = n
				}
			}
		} else if strings.HasPrefix(part, "+") {
			nums := strings.SplitN(part[1:], ",", 2)
			if len(nums) >= 1 {
				if n, err := strconv.Atoi(nums[0]); err == nil {
					newStart = n
				}
			}
		}
	}
	if oldStart == 0 {
		oldStart = 1
	}
	if newStart == 0 {
		newStart = 1
	}
	return
}

func (l LogPage) renderCommitDetail(width, height int) string {
	if l.isWIPSelected() {
		return l.renderWIPDetail(width, height)
	}

	focused := l.focus == focusLogDetail
	iw := width - styles.PanelPaddingWidth // inner width
	titleStr := styles.PanelTitle("Detail", "3", focused, iw)
	ph := height - styles.PanelBorderHeight

	t := theme.Active
	titleGap := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")

	if l.detailCommit == nil {
		content := styles.DimStyle().Width(iw).Render("Select a commit to view details")
		return styles.PanelStyle(focused).Width(width).Height(ph).Render(
			lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content),
		)
	}
	c := l.detailCommit

	bgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).Width(iw).Render(s)
	}

	// --- Section 1: Commit metadata header ---
	var sections []string
	sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).Bold(true).Render("commit "+c.Hash)))
	sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Teal).Background(t.Base).Render("Author: "+c.Author+" <"+c.AuthorEmail+">")))
	sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render("Date:   "+c.Date.Format("Mon Jan 2 15:04:05 2006 -0700"))))
	if len(c.Refs) > 0 {
		refsLabel := lipgloss.NewStyle().Foreground(t.Subtext0).Background(t.Base).Render("Refs:   ")
		refsBadges := styles.RenderRefBadges(c.Refs, t.Base)
		sections = append(sections, bgLine(refsLabel+refsBadges))
	}
	sections = append(sections, bgLine(""))
	sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Render("    "+c.Subject)))
	if c.Body != "" {
		sections = append(sections, bgLine(""))
		for _, line := range strings.Split(c.Body, "\n") {
			sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Render("    "+line)))
		}
	}
	sections = append(sections, bgLine(""))
	sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Surface2).Background(t.Base).Render(strings.Repeat("─", iw))))

	// --- Section 2: File list ---
	if len(l.detailFiles) == 0 {
		sections = append(sections, styles.DimStyle().Width(iw).Render("  No files changed"))
	} else {
		fileLabel := fmt.Sprintf("Files changed (%d)", len(l.detailFiles))
		sections = append(sections, lipgloss.NewStyle().Foreground(t.Subtext0).Background(t.Base).Bold(true).Width(iw).Render(fileLabel))
		for i, f := range l.detailFiles {
			icon := styles.FileListIcon(f.Status)
			color := styles.FileListColor(f.Status)
			path := f.NewPath
			if f.Status == "deleted" {
				path = f.OldPath
			}
			if f.Status == "renamed" && f.OldPath != f.NewPath {
				path = f.OldPath + " → " + f.NewPath
			}

			selected := i == l.detailFileCursor && focused
			bg := t.Base
			prefix := "  "
			if selected {
				bg = t.Surface1
				prefix = "▸ "
			}

			iconStr := lipgloss.NewStyle().Foreground(color).Background(bg).Render(icon)
			pathStr := lipgloss.NewStyle().Foreground(t.Text).Background(bg).Render(" " + path)
			lineContent := prefix + iconStr + pathStr
			sections = append(sections, lipgloss.NewStyle().Background(bg).Width(iw).Render(lineContent))
		}
	}
	// Build content from sections (without hints)
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Hint line (separate from content, pinned to bottom)
	hints := bgLine(styles.KeyHintStyle().Background(t.Base).Render(
		"j/k:files  Enter:view diff  Esc:close diff",
	))
	emptyLine := bgLine("")

	// contentBudget = ph - title(1) - titleGap(1) - emptyLine(1) - hints(1)
	contentBudget := ph - 4
	if contentBudget < 1 {
		contentBudget = 1
	}

	// Pad content to exactly contentBudget lines so hints are pinned to the bottom
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > contentBudget {
		contentLines = contentLines[:contentBudget]
	}
	for len(contentLines) < contentBudget {
		contentLines = append(contentLines, bgLine(""))
	}
	content = strings.Join(contentLines, "\n")

	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, emptyLine, hints)
	// Clip to panel height so all panels stay the same outer height.
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.PanelStyle(focused).Width(width).Height(ph).Render(full)
}

func (l LogPage) renderWIPDetail(width, height int) string {
	focused := l.focus == focusLogDetail
	iw := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight
	t := theme.Active

	bgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).Width(iw).Render(s)
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

			text := prefix + icon + " " + f.Path
			if len(text) > iw {
				text = text[:iw]
			}
			style := lipgloss.NewStyle().Foreground(color).Background(bg).Width(iw)
			if selected {
				style = style.Bold(true)
			}
			fileSections = append(fileSections, style.Render(text))
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

			text := prefix + icon + " " + f.Path
			if len(text) > iw {
				text = text[:iw]
			}
			style := lipgloss.NewStyle().Foreground(color).Background(bg).Width(iw)
			if selected {
				style = style.Bold(true)
			}
			fileSections = append(fileSections, style.Render(text))
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

	summaryBorder := t.Surface2
	if commitFocused && l.commitEditing && l.commitField == 0 {
		summaryBorder = t.Blue
	}
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

	descBorder := t.Surface2
	if commitFocused && l.commitEditing && l.commitField == 1 {
		descBorder = t.Blue
	}
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
	containerBorder := t.Surface2
	if commitFocused {
		containerBorder = t.Blue
	}
	innerContent := lipgloss.JoinVertical(lipgloss.Left, commitInner...)
	commitContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(containerBorder).
		BorderBackground(t.Base).
		Background(t.Base).
		Width(iw - 2). // outer border width
		Render(innerContent)

	// ---------------------------------------------------------------
	// Fixed commit box height: header(1) + margin(1) + summary(1+2border=3) + desc(3+2border=5) = 10 inner + 2 outer border = 12
	// ---------------------------------------------------------------
	commitBoxHeight := 12

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
		hintText = "Enter:diff  space:stage/unstage  a:all  A:amend  d:discard"
	}
	hintsLine := bgLine(styles.KeyHintStyle().Background(t.Base).Render(hintText))

	// ---------------------------------------------------------------
	// Compute file area height and pad/clip to fill the gap
	// fileAreaHeight = ph - title(1) - titleGap(1) - commitBoxHeight - hints(1)
	// ---------------------------------------------------------------
	fileAreaHeight := ph - 2 - commitBoxHeight - 1
	if fileAreaHeight < 1 {
		fileAreaHeight = 1
	}

	fileContent := lipgloss.JoinVertical(lipgloss.Left, fileSections...)
	fileLines := strings.Split(fileContent, "\n")

	// Clip if file list is too tall
	if len(fileLines) > fileAreaHeight {
		fileLines = fileLines[:fileAreaHeight]
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
	return styles.PanelStyle(focused).Width(width).Height(ph).Render(full)
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (l LogPage) loadLog() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		commits, err := repo.Log(git.LogOptions{MaxCount: 200})
		if err != nil {
			return logLoadedMsg{commits: commits, err: err}
		}

		// Check for uncommitted changes — if dirty, prepend a synthetic WIP entry.
		status, statusErr := repo.Status()
		hasWIP := false
		if statusErr == nil && status != nil {
			hasWIP = len(status.StagedFiles()) > 0 || len(status.UnstagedFiles()) > 0
		}

		if hasWIP {
			wipEntry := git.CommitInfo{
				Hash:      "",
				ShortHash: "●",
				Subject:   "Working Changes",
			}
			commits = append([]git.CommitInfo{wipEntry}, commits...)
		}

		graphRows := git.ComputeGraph(commits)
		return logLoadedMsg{commits: commits, graphRows: graphRows, hasWIP: hasWIP, err: nil}
	}
}

func (l LogPage) loadLogFiltered(query string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		commits, err := repo.Log(git.LogOptions{MaxCount: 200, Grep: query})
		if err != nil {
			return logLoadedMsg{commits: commits, err: err}
		}

		// Check for uncommitted changes — if dirty, prepend a synthetic WIP entry.
		status, statusErr := repo.Status()
		hasWIP := false
		if statusErr == nil && status != nil {
			hasWIP = len(status.StagedFiles()) > 0 || len(status.UnstagedFiles()) > 0
		}

		if hasWIP {
			wipEntry := git.CommitInfo{
				Hash:      "",
				ShortHash: "●",
				Subject:   "Working Changes",
			}
			commits = append([]git.CommitInfo{wipEntry}, commits...)
		}

		graphRows := git.ComputeGraph(commits)
		return logLoadedMsg{commits: commits, graphRows: graphRows, hasWIP: hasWIP, err: nil}
	}
}

func (l LogPage) loadWIPDetail() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		status, err := repo.Status()
		if err != nil {
			return wipDetailMsg{err: err}
		}
		return wipDetailMsg{
			unstaged: status.UnstagedFiles(),
			staged:   status.StagedFiles(),
		}
	}
}

// loadCenterDiff determines the appropriate file based on context (WIP
// unstaged/staged or commit detail) and loads its diff to display in the
// center panel. It replaces the old loadWIPSelectedDiff().
func (l LogPage) loadCenterDiff() tea.Cmd {
	repo := l.repo

	// --- WIP context ---
	if l.isWIPSelected() {
		var path string
		var staged bool
		var untracked bool

		switch l.wipFocus {
		case wipFocusUnstaged:
			if len(l.wipUnstaged) > 0 && l.wipUnstagedCursor < len(l.wipUnstaged) {
				f := l.wipUnstaged[l.wipUnstagedCursor]
				path = f.Path
				staged = false
				untracked = f.IsUntracked()
			}
		case wipFocusStaged:
			if len(l.wipStaged) > 0 && l.wipStagedCursor < len(l.wipStaged) {
				path = l.wipStaged[l.wipStagedCursor].Path
				staged = true
			}
		}

		if path == "" {
			return nil
		}

		return func() tea.Msg {
			var diff string
			var err error
			if untracked {
				diff, err = repo.FileDiffUntracked(path)
			} else {
				diff, err = repo.FileDiff(path, staged)
			}
			return centerDiffMsg{path: path, diff: diff, err: err, isWIP: true, isStaged: staged}
		}
	}

	// --- Commit detail context ---
	if l.detailCommit != nil && len(l.detailFiles) > 0 && l.detailFileCursor < len(l.detailFiles) {
		f := l.detailFiles[l.detailFileCursor]
		commitHash := l.detailCommit.Hash
		filePath := f.NewPath
		if f.Status == "deleted" {
			filePath = f.OldPath
		}

		return func() tea.Msg {
			diff, err := repo.DiffCommitFile(commitHash, filePath)
			return centerDiffMsg{path: filePath, diff: diff, err: err}
		}
	}

	return nil
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

// copyToClipboard copies text to the system clipboard and shows a toast.
func (l LogPage) copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("pbcopy")
		case "linux":
			// Try xclip first, then xsel
			if _, err := exec.LookPath("xclip"); err == nil {
				cmd = exec.Command("xclip", "-selection", "clipboard")
			} else {
				cmd = exec.Command("xsel", "--clipboard", "--input")
			}
		case "windows":
			cmd = exec.Command("clip")
		default:
			return RequestToastMsg{Message: "Clipboard not supported on this OS", IsError: true}
		}
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return RequestToastMsg{Message: "Copy failed: " + err.Error(), IsError: true}
		}
		short := text
		if len(short) > 7 {
			short = short[:7]
		}
		return RequestToastMsg{Message: "Copied " + short + " to clipboard"}
	}
}

// IsSearching returns true when the search input is active.
func (l LogPage) IsSearching() bool {
	return l.searching
}

// handleSearchKeys handles keyboard input while the search bar is active.
func (l LogPage) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		// Cancel search — clear query and close
		l.searching = false
		l.searchInput.Blur()
		// If there was a previous query, clear the filter
		if l.searchQuery != "" {
			l.searchQuery = ""
			return l, l.loadLog() // reload unfiltered
		}
		return l, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		// Apply search
		l.searching = false
		l.searchInput.Blur()
		query := l.searchInput.Value()
		l.searchQuery = query
		if query == "" {
			return l, l.loadLog() // empty query — reload unfiltered
		}
		// Apply filter depending on which panel initiated search
		switch l.searchPanel {
		case focusLogList:
			// Server-side grep through commit messages
			return l, l.loadLogFiltered(query)
		case focusSidebar:
			// Client-side filter — filter sidebar items
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

// handleDiffVisualKeys handles keys when in visual (line-selection) mode.
func (l LogPage) handleDiffVisualKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxLine := len(l.centerDiffLines) - 1
	if maxLine < 0 {
		maxLine = 0
	}

	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "v"))):
		// Exit visual mode
		l.diffVisualMode = false
		return l, nil

	case key.Matches(msg, l.navKeys.Down):
		if l.diffVisualCursor < maxLine {
			l.diffVisualCursor++
			// Auto-scroll to keep cursor visible
			contentHeight := l.height - styles.PanelBorderHeight - 4 // approximate visible area
			if contentHeight < 1 {
				contentHeight = 1
			}
			if l.diffVisualCursor >= l.centerDiffScroll+contentHeight {
				l.centerDiffScroll = l.diffVisualCursor - contentHeight + 1
			}
		}
		return l, nil

	case key.Matches(msg, l.navKeys.Up):
		if l.diffVisualCursor > 0 {
			l.diffVisualCursor--
			if l.diffVisualCursor < l.centerDiffScroll {
				l.centerDiffScroll = l.diffVisualCursor
			}
		}
		return l, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
		// Stage selected lines
		if l.centerDiffIsWIP && !l.centerDiffIsStaged {
			return l, l.stageSelectedLines()
		}
		return l, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("u"))):
		// Unstage selected lines
		if l.centerDiffIsWIP && l.centerDiffIsStaged {
			return l, l.unstageSelectedLines()
		}
		return l, nil
	}
	return l, nil
}

// visualSelectionRange returns the ordered (lo, hi) range of the visual selection.
func (l LogPage) visualSelectionRange() (int, int) {
	lo, hi := l.diffVisualAnchor, l.diffVisualCursor
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi
}

// stageSelectedLines stages only the selected lines from the visual selection.
func (l LogPage) stageSelectedLines() tea.Cmd {
	lo, hi := l.visualSelectionRange()
	path := l.centerDiffPath
	repo := l.repo

	// Find which hunk(s) overlap the selection and build partial patches
	type hunkSelection struct {
		hunk    git.Hunk
		indices map[int]bool // indices within hunk.Lines
	}
	var selections []hunkSelection

	for hIdx, hunk := range l.centerDiffHunks {
		if hIdx >= len(l.centerDiffHunkStarts) {
			break
		}
		hunkStart := l.centerDiffHunkStarts[hIdx] + 1 // +1 to skip @@ header line
		hunkEnd := hunkStart + len(hunk.Lines) - 1

		// Check if selection overlaps this hunk's lines
		if hi < hunkStart || lo > hunkEnd {
			continue
		}

		selected := make(map[int]bool)
		for lineIdx := range hunk.Lines {
			diffLineIdx := hunkStart + lineIdx
			if diffLineIdx >= lo && diffLineIdx <= hi {
				selected[lineIdx] = true
			}
		}
		if len(selected) > 0 {
			selections = append(selections, hunkSelection{hunk: hunk, indices: selected})
		}
	}

	if len(selections) == 0 {
		return nil
	}

	l.diffVisualMode = false
	return func() tea.Msg {
		for _, sel := range selections {
			err := repo.StageLines(path, sel.hunk, sel.indices)
			if err != nil {
				return RequestToastMsg{Message: "Stage lines failed: " + err.Error(), IsError: true}
			}
		}
		return wipStageResultMsg{err: nil}
	}
}

// unstageSelectedLines unstages only the selected lines from the visual selection.
func (l LogPage) unstageSelectedLines() tea.Cmd {
	lo, hi := l.visualSelectionRange()
	path := l.centerDiffPath
	repo := l.repo

	type hunkSelection struct {
		hunk    git.Hunk
		indices map[int]bool
	}
	var selections []hunkSelection

	for hIdx, hunk := range l.centerDiffHunks {
		if hIdx >= len(l.centerDiffHunkStarts) {
			break
		}
		hunkStart := l.centerDiffHunkStarts[hIdx] + 1
		hunkEnd := hunkStart + len(hunk.Lines) - 1

		if hi < hunkStart || lo > hunkEnd {
			continue
		}

		selected := make(map[int]bool)
		for lineIdx := range hunk.Lines {
			diffLineIdx := hunkStart + lineIdx
			if diffLineIdx >= lo && diffLineIdx <= hi {
				selected[lineIdx] = true
			}
		}
		if len(selected) > 0 {
			selections = append(selections, hunkSelection{hunk: hunk, indices: selected})
		}
	}

	if len(selections) == 0 {
		return nil
	}

	l.diffVisualMode = false
	return func() tea.Msg {
		for _, sel := range selections {
			err := repo.UnstageLines(path, sel.hunk, sel.indices)
			if err != nil {
				return RequestToastMsg{Message: "Unstage lines failed: " + err.Error(), IsError: true}
			}
		}
		return wipStageResultMsg{err: nil}
	}
}

// stageHunk stages a single diff hunk and refreshes the diff view.
func (l LogPage) stageHunk(path string, hunk git.Hunk) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.StageHunk(path, hunk)
		if err != nil {
			return RequestToastMsg{Message: "Stage hunk failed: " + err.Error(), IsError: true}
		}
		return wipStageResultMsg{err: nil}
	}
}

// unstageHunk unstages a single diff hunk and refreshes the diff view.
func (l LogPage) unstageHunk(path string, hunk git.Hunk) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.UnstageHunk(path, hunk)
		if err != nil {
			return RequestToastMsg{Message: "Unstage hunk failed: " + err.Error(), IsError: true}
		}
		return wipStageResultMsg{err: nil}
	}
}

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

func (l LogPage) loadCommitDetail(c git.CommitInfo) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		diff, err := repo.DiffCommit(c.Hash)
		return commitDetailMsg{commit: c, diff: diff, err: err}
	}
}

func (l LogPage) loadStashDiff(index int) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		diff, err := repo.StashShow(index)
		return stashDiffMsg{index: index, diff: diff, err: err}
	}
}

func (l LogPage) renderStashDiff(width, height int) string {
	focused := l.focus == focusLogDetail
	iw := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight
	t := theme.Active

	titleStr := styles.PanelTitle(
		fmt.Sprintf("Stash Info (stash@{%d})", l.stashDiffIndex),
		"3", focused, iw,
	)

	var sections []string

	bgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).Width(iw).Render(s)
	}

	if l.stashDiffContent == "" {
		sections = append(sections, styles.DimStyle().Width(iw).Render("Loading stash info..."))
	} else {
		sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).Bold(true).Render(
			fmt.Sprintf("stash@{%d}", l.stashDiffIndex),
		)))
		sections = append(sections, bgLine(""))
		sections = append(sections, bgLine(styles.DimStyle().Render("Diff is shown in the center panel.")))
	}

	titleGap := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Hint line (separate from content, pinned to bottom)
	hints := bgLine(styles.KeyHintStyle().Background(t.Base).Render(
		"Esc:back to graph  j/k:scroll diff",
	))
	emptyLine := bgLine("")

	// contentBudget = ph - title(1) - titleGap(1) - emptyLine(1) - hints(1)
	contentBudget := ph - 4
	if contentBudget < 1 {
		contentBudget = 1
	}

	// Pad content to exactly contentBudget lines so hints are pinned to the bottom
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > contentBudget {
		contentLines = contentLines[:contentBudget]
	}
	for len(contentLines) < contentBudget {
		contentLines = append(contentLines, bgLine(""))
	}
	content = strings.Join(contentLines, "\n")

	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, emptyLine, hints)
	// Clip to panel height so all panels stay the same outer height.
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.PanelStyle(focused).Width(width).Height(ph).Render(full)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// expandTabs replaces tab characters with spaces.
func expandTabs(s string, tabWidth int) string {
	var result strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			for i := 0; i < spaces; i++ {
				result.WriteByte(' ')
			}
			col += spaces
		} else {
			result.WriteRune(r)
			col++
		}
	}
	return result.String()
}

// truncateToWidth truncates a string to fit within maxWidth visible columns.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	w := 0
	for i, r := range s {
		w++
		if w > maxWidth {
			return s[:i]
		}
		_ = r
	}
	return s
}

// horizontalSlice drops the first offset visible characters from s, then
// truncates the result to width characters. This is used for horizontal
// scrolling of diff content while keeping the line-number gutter fixed.
func horizontalSlice(s string, offset, width int) string {
	if offset <= 0 {
		return truncateToWidth(s, width)
	}
	// Skip the first 'offset' characters
	skipped := 0
	start := len(s) // default: entire string consumed
	for i, r := range s {
		if skipped >= offset {
			start = i
			break
		}
		skipped++
		_ = r
	}
	if start >= len(s) {
		return ""
	}
	return truncateToWidth(s[start:], width)
}

// fillBg forces a background color on every line of a rendered string.
// Bubbles components emit ANSI reset sequences (\x1b[0m) and their own
// background codes (\x1b[40m etc.) that kill any outer background. This
// function strips all ANSI background sequences from the input, then
// re-inserts our desired background after every reset so the color
// persists across the entire line.
func fillBg(s string, bg lipgloss.Color, width int) string {
	hex := string(bg)
	if len(hex) > 0 && hex[0] == '#' {
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

var _ tea.Model = LogPage{}
