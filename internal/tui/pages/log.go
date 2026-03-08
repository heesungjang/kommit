package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
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

// wipDiffMsg carries the diff for a single WIP file (loaded on-demand).
type wipDiffMsg struct {
	path   string
	staged bool
	diff   string
	err    error
}

// wipStageResultMsg is sent after a stage/unstage/discard operation completes.
type wipStageResultMsg struct {
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

	commits   []git.CommitInfo
	cursor    int
	hasWIP    bool           // true when uncommitted changes exist; commits[0] is synthetic
	graphRows []git.GraphRow // parallel to commits; one GraphRow per commit

	// Detail view — structured file list (for commits)
	detailCommit     *git.CommitInfo
	detailFiles      []git.DiffFile
	detailFileCursor int
	detailScroll     int // scroll within the currently viewed file's diff

	// WIP staging area — interactive when WIP row is selected
	wipUnstaged       []git.FileStatus
	wipStaged         []git.FileStatus
	wipUnstagedCursor int
	wipStagedCursor   int
	wipFocus          wipPanelFocus // which sub-panel is focused within the WIP detail
	wipDiffContent    string        // diff for the currently selected WIP file
	wipDiffPath       string
	wipDiffScroll     int

	// Pending discard in WIP context
	wipPendingDiscardPath      string
	wipPendingDiscardUntracked bool

	// Stash diff display — shown in right panel when viewing a stash entry
	viewingStash     bool
	stashDiffIndex   int
	stashDiffContent string

	// Focus
	focus logFocus

	// State
	loading bool
	err     error

	// Keys
	navKeys    keys.NavigationKeys
	statusKeys keys.StatusKeys
	remoteKeys keys.RemoteOpsKeys

	// Dimensions
	width  int
	height int
}

// sidebarWidth computes the width for the sidebar panel.
func (l LogPage) sidebarWidth() int {
	// ~18% of total width, minimum 20, maximum 30
	w := l.width * 18 / 100
	if w < 20 {
		w = 20
	}
	if w > 30 {
		w = 30
	}
	return w
}

// NewLogPage creates a new log page (the main unified view).
func NewLogPage(repo *git.Repository, width, height int) LogPage {
	sbw := width * 18 / 100
	if sbw < 20 {
		sbw = 20
	}
	if sbw > 30 {
		sbw = 30
	}
	return LogPage{
		repo:       repo,
		sidebar:    NewSidebar(repo, sbw, height),
		navKeys:    keys.NewNavigationKeys(),
		statusKeys: keys.NewStatusKeys(),
		remoteKeys: keys.NewRemoteOpsKeys(),
		width:      width,
		height:     height,
		loading:    true,
	}
}

// isWIPSelected returns true when the cursor is on the synthetic WIP row.
func (l LogPage) isWIPSelected() bool {
	return l.hasWIP && l.cursor == 0
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
			l.wipDiffContent = ""
			l.wipDiffPath = ""
			l.focus = focusLogList
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
		l.wipDiffScroll = 0
		l.wipDiffContent = ""
		l.wipDiffPath = ""

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

		return l, tea.Batch(l.loadWIPSelectedDiff(), func() tea.Msg {
			return StatusDirtyMsg{Dirty: dirty}
		})

	case wipDiffMsg:
		l.wipDiffContent = msg.diff
		l.wipDiffPath = msg.path
		l.wipDiffScroll = 0
		return l, nil

	case wipStageResultMsg:
		// After stage/unstage/discard, reload WIP data
		return l, l.loadWIPDetail()

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
		l.detailScroll = 0
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
		if msg.err != nil {
			l.stashDiffContent = "Error loading stash diff: " + msg.err.Error()
		} else {
			l.stashDiffContent = msg.diff
		}
		l.stashDiffIndex = msg.index
		l.detailScroll = 0
		return l, nil

	case RefreshStatusMsg:
		// Auto-refresh detected external changes — reload log and sidebar.
		return l, tea.Batch(l.loadLog(), l.sidebar.Refresh())

	case tea.MouseMsg:
		return l.handleMouse(msg)

	case tea.KeyMsg:
		return l.handleKey(msg)
	}

	return l, nil
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (l LogPage) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
					if len(l.wipStaged) > 0 {
						l.wipFocus = wipFocusStaged
					} else {
						l.wipFocus = wipFocusUnstaged
					}
				}
			case focusLogList:
				l.focus = focusSidebar
			case focusLogDetail:
				l.focus = focusLogList
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
		case key.Matches(msg, l.remoteKeys.Pull):
			return l, func() tea.Msg { return RequestGitOpMsg{Op: "pull"} }
		case key.Matches(msg, l.remoteKeys.Fetch):
			return l, func() tea.Msg { return RequestGitOpMsg{Op: "fetch"} }
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
	}
	return l, nil
}

func (l LogPage) handleDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if l.isWIPSelected() {
		return l.handleWIPDetailKeys(msg)
	}
	switch {
	case key.Matches(msg, l.navKeys.Down):
		if l.detailFileCursor < len(l.detailFiles)-1 {
			l.detailFileCursor++
			l.detailScroll = 0
		}
	case key.Matches(msg, l.navKeys.Up):
		if l.detailFileCursor > 0 {
			l.detailFileCursor--
			l.detailScroll = 0
		}
	case key.Matches(msg, l.navKeys.Home):
		l.detailFileCursor = 0
		l.detailScroll = 0
	case key.Matches(msg, l.navKeys.End):
		if len(l.detailFiles) > 0 {
			l.detailFileCursor = len(l.detailFiles) - 1
			l.detailScroll = 0
		}
	case key.Matches(msg, l.navKeys.PageDown):
		maxScroll := l.detailMaxScroll()
		l.detailScroll += 10
		if l.detailScroll > maxScroll {
			l.detailScroll = maxScroll
		}
	case key.Matches(msg, l.navKeys.PageUp):
		l.detailScroll -= 10
		if l.detailScroll < 0 {
			l.detailScroll = 0
		}
	}
	return l, nil
}

func (l LogPage) handleWIPDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Tab within WIP panel cycles unstaged → staged → sidebar
	if key.Matches(msg, key.NewBinding(key.WithKeys("tab"))) {
		if l.wipFocus == wipFocusUnstaged && len(l.wipStaged) > 0 {
			l.wipFocus = wipFocusStaged
		} else {
			// Cycle to sidebar
			l.focus = focusSidebar
		}
		return l, nil
	}
	if key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))) {
		if l.wipFocus == wipFocusStaged {
			l.wipFocus = wipFocusUnstaged
		} else {
			l.focus = focusLogList
		}
		return l, nil
	}

	switch l.wipFocus {
	case wipFocusUnstaged:
		return l.handleWIPUnstagedKeys(msg)
	case wipFocusStaged:
		return l.handleWIPStagedKeys(msg)
	}
	return l, nil
}

func (l LogPage) handleWIPUnstagedKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, l.navKeys.Down):
		if l.wipUnstagedCursor < len(l.wipUnstaged)-1 {
			l.wipUnstagedCursor++
			return l, l.loadWIPSelectedDiff()
		}
	case key.Matches(msg, l.navKeys.Up):
		if l.wipUnstagedCursor > 0 {
			l.wipUnstagedCursor--
			return l, l.loadWIPSelectedDiff()
		}
	case key.Matches(msg, l.navKeys.Home):
		l.wipUnstagedCursor = 0
		return l, l.loadWIPSelectedDiff()
	case key.Matches(msg, l.navKeys.End):
		if len(l.wipUnstaged) > 0 {
			l.wipUnstagedCursor = len(l.wipUnstaged) - 1
			return l, l.loadWIPSelectedDiff()
		}
	case key.Matches(msg, l.navKeys.PageDown):
		l.wipDiffScroll += 10
		max := l.wipMaxScroll()
		if l.wipDiffScroll > max {
			l.wipDiffScroll = max
		}
		return l, nil
	case key.Matches(msg, l.navKeys.PageUp):
		l.wipDiffScroll -= 10
		if l.wipDiffScroll < 0 {
			l.wipDiffScroll = 0
		}
		return l, nil
	case key.Matches(msg, l.statusKeys.Stage), key.Matches(msg, key.NewBinding(key.WithKeys(" "))):
		// Stage the selected file
		if len(l.wipUnstaged) > 0 && l.wipUnstagedCursor < len(l.wipUnstaged) {
			return l, l.wipStageFile(l.wipUnstaged[l.wipUnstagedCursor].Path)
		}
	case key.Matches(msg, l.statusKeys.StageAll):
		return l, l.wipStageAll()
	case key.Matches(msg, l.statusKeys.Commit):
		return l, l.wipRequestCommit()
	case key.Matches(msg, l.statusKeys.CommitAmend):
		return l, l.wipRequestAmend()
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
	}
	return l, nil
}

func (l LogPage) handleWIPStagedKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, l.navKeys.Down):
		if l.wipStagedCursor < len(l.wipStaged)-1 {
			l.wipStagedCursor++
			return l, l.loadWIPSelectedDiff()
		}
	case key.Matches(msg, l.navKeys.Up):
		if l.wipStagedCursor > 0 {
			l.wipStagedCursor--
			return l, l.loadWIPSelectedDiff()
		}
	case key.Matches(msg, l.navKeys.Home):
		l.wipStagedCursor = 0
		return l, l.loadWIPSelectedDiff()
	case key.Matches(msg, l.navKeys.End):
		if len(l.wipStaged) > 0 {
			l.wipStagedCursor = len(l.wipStaged) - 1
			return l, l.loadWIPSelectedDiff()
		}
	case key.Matches(msg, l.navKeys.PageDown):
		l.wipDiffScroll += 10
		max := l.wipMaxScroll()
		if l.wipDiffScroll > max {
			l.wipDiffScroll = max
		}
		return l, nil
	case key.Matches(msg, l.navKeys.PageUp):
		l.wipDiffScroll -= 10
		if l.wipDiffScroll < 0 {
			l.wipDiffScroll = 0
		}
		return l, nil
	case key.Matches(msg, l.statusKeys.Unstage), key.Matches(msg, key.NewBinding(key.WithKeys(" "))):
		// Unstage the selected file
		if len(l.wipStaged) > 0 && l.wipStagedCursor < len(l.wipStaged) {
			return l, l.wipUnstageFile(l.wipStaged[l.wipStagedCursor].Path)
		}
	case key.Matches(msg, l.statusKeys.Commit):
		return l, l.wipRequestCommit()
	case key.Matches(msg, l.statusKeys.CommitAmend):
		return l, l.wipRequestAmend()
	}
	return l, nil
}

// wipMaxScroll returns the max scroll for the WIP diff content.
func (l LogPage) wipMaxScroll() int {
	if l.wipDiffContent == "" {
		return 0
	}
	lines := strings.Count(l.wipDiffContent, "\n") + 1
	visible := l.height - styles.PanelBorderHeight - 10
	if visible < 1 {
		visible = 1
	}
	max := lines - visible
	if max < 0 {
		return 0
	}
	return max
}

// detailDiffLineCount returns the number of diff lines for the currently
// selected file in the detail panel.
func (l LogPage) detailDiffLineCount() int {
	if len(l.detailFiles) == 0 || l.detailFileCursor >= len(l.detailFiles) {
		return 0
	}
	f := l.detailFiles[l.detailFileCursor]
	count := 0
	for _, h := range f.Hunks {
		count++ // hunk header
		count += len(h.Lines)
	}
	return count
}

// detailMaxScroll returns the maximum scroll offset for the diff portion
// of the detail panel (the selected file's diff).
func (l LogPage) detailMaxScroll() int {
	diffLines := l.detailDiffLineCount()
	if diffLines == 0 {
		return 0
	}
	// Visible area for diff: total panel height minus border, title, header area, file list, separator
	headerLines := l.detailHeaderLineCount()
	fileListLines := len(l.detailFiles) + 1     // files + separator
	overhead := 1 + headerLines + fileListLines // title + header + file list
	visible := l.height - styles.PanelBorderHeight - overhead
	if visible < 1 {
		visible = 1
	}
	max := diffLines - visible
	if max < 0 {
		return 0
	}
	return max
}

// detailHeaderLineCount returns the number of lines in the commit header.
func (l LogPage) detailHeaderLineCount() int {
	if l.detailCommit == nil {
		return 0
	}
	c := l.detailCommit
	lines := 4 // hash, author, date, empty
	if len(c.Refs) > 0 {
		lines++
	}
	lines++ // subject
	if c.Body != "" {
		lines += 1 + strings.Count(c.Body, "\n") + 1 // empty + body lines
	}
	lines += 2 // empty + separator
	return lines
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
	centerWidth := remaining * 50 / 100
	if centerWidth < 30 {
		centerWidth = 30
	}
	if centerWidth > remaining-24 {
		centerWidth = remaining - 24
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
			if l.cursor > 0 {
				l.cursor--
				l.focus = focusLogList
				return l, l.loadDetailForCursor()
			}
		case zoneRight:
			if l.isWIPSelected() && !l.viewingStash {
				if l.wipDiffScroll > 0 {
					l.wipDiffScroll -= 3
					if l.wipDiffScroll < 0 {
						l.wipDiffScroll = 0
					}
				}
			} else {
				if l.detailScroll > 0 {
					l.detailScroll -= 3
					if l.detailScroll < 0 {
						l.detailScroll = 0
					}
				}
			}
			l.focus = focusLogDetail
		}
		return l, nil

	case tea.MouseButtonWheelDown:
		switch z {
		case zoneSidebar:
			l.focus = focusSidebar
		case zoneCenter:
			if l.cursor < len(l.commits)-1 {
				l.cursor++
				l.focus = focusLogList
				return l, l.loadDetailForCursor()
			}
		case zoneRight:
			if l.isWIPSelected() && !l.viewingStash {
				maxScroll := l.wipMaxScroll()
				l.wipDiffScroll += 3
				if l.wipDiffScroll > maxScroll {
					l.wipDiffScroll = maxScroll
				}
			} else {
				maxScroll := l.detailMaxScroll()
				l.detailScroll += 3
				if l.detailScroll > maxScroll {
					l.detailScroll = maxScroll
				}
			}
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
			return l, l.loadWIPSelectedDiff()
		}
	} else if relY >= stagedFileStart && relY < stagedFileEnd && len(l.wipStaged) > 0 {
		idx := relY - stagedFileStart
		if idx < len(l.wipStaged) {
			l.wipFocus = wipFocusStaged
			l.wipStagedCursor = idx
			return l, l.loadWIPSelectedDiff()
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
	centerWidth := remaining * 50 / 100
	if centerWidth < 30 {
		centerWidth = 30
	}
	if centerWidth > remaining-24 {
		centerWidth = remaining - 24
	}
	if centerWidth < 10 {
		centerWidth = 10
	}
	rightWidth := remaining - centerWidth
	if rightWidth < 20 {
		rightWidth = 20
	}

	sidebarPane := l.sidebar.View(l.focus == focusSidebar)
	centerPane := l.renderCommitList(centerWidth, l.height)

	var rightPane string
	if l.viewingStash {
		rightPane = l.renderStashDiff(rightWidth, l.height)
	} else {
		rightPane = l.renderCommitDetail(rightWidth, l.height)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarPane, centerPane, rightPane)
}

func (l LogPage) renderCommitList(width, height int) string {
	t := theme.Active
	innerWidth := width - styles.PanelPaddingWidth
	commitCount := len(l.commits)
	if l.hasWIP {
		commitCount-- // exclude synthetic WIP entry from count
	}
	titleStr := styles.TitleStyle(l.focus == focusLogList).Width(innerWidth).Render(
		fmt.Sprintf("Commits (%d)", commitCount),
	)

	var lines []string
	if len(l.commits) == 0 {
		lines = append(lines, styles.DimStyle().Width(innerWidth).Render("  No commits yet. Make your first commit with 'c' on the Status page."))
	}

	// Viewport windowing: only render visible commits, following the cursor.
	ph := height - styles.PanelBorderHeight
	// Visible lines: panel height minus title (1) minus hints area (2: empty + hints)
	visibleCount := ph - 3
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
	// Cap graph width to keep commit info readable
	if graphColWidth > 20 {
		graphColWidth = 20
	}

	hashWidth := 8
	dateWidth := 11
	authorWidth := 15
	subjectWidth := innerWidth - graphColWidth - hashWidth - dateWidth - authorWidth - 6
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

		// Render graph prefix
		graphStr := ""
		if i < len(l.graphRows) && graphColWidth > 0 {
			gr := l.graphRows[i]
			var graphParts []string
			for j := 0; j < graphWidth && j < len(gr.Cells); j++ {
				cell := gr.Cells[j]
				color := styles.GraphColor(cell.Column)
				graphParts = append(graphParts, lipgloss.NewStyle().Foreground(color).Background(bg).Render(cell.Char))
			}
			// Pad remaining columns with spaces
			for j := len(gr.Cells); j < graphWidth; j++ {
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
			if effectiveSubjectWidth < 8 {
				effectiveSubjectWidth = 8
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
	scrollInfo := ""
	if len(l.commits) > visibleCount {
		scrollInfo = fmt.Sprintf("  [%d/%d]", l.cursor+1, len(l.commits))
	}
	hints := styles.KeyHintStyle().Background(t.Base).Width(innerWidth).Render(
		"j/k:navigate  tab:switch panel  g/G:top/bottom" + scrollInfo,
	)
	emptyLine := lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render("")
	return styles.PanelStyle(l.focus == focusLogList).Width(width).Height(ph).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, content, emptyLine, hints),
	)
}

func (l LogPage) renderCommitDetail(width, height int) string {
	if l.isWIPSelected() {
		return l.renderWIPDetail(width, height)
	}

	focused := l.focus == focusLogDetail
	iw := width - styles.PanelPaddingWidth // inner width
	titleStr := styles.TitleStyle(focused).Width(iw).Render("Detail")
	ph := height - styles.PanelBorderHeight

	if l.detailCommit == nil {
		content := styles.DimStyle().Width(iw).Render("Select a commit to view details")
		return styles.PanelStyle(focused).Width(width).Height(ph).Render(
			lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
		)
	}

	t := theme.Active
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
	sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Surface2).Background(t.Base).Render(strings.Repeat("─", iw))))

	// --- Section 3: Diff for selected file ---
	if len(l.detailFiles) > 0 && l.detailFileCursor < len(l.detailFiles) {
		f := l.detailFiles[l.detailFileCursor]
		// File header
		diffHeader := fmt.Sprintf("--- %s", f.OldPath)
		if f.NewPath != f.OldPath {
			diffHeader += fmt.Sprintf("\n+++ %s", f.NewPath)
		} else {
			diffHeader += fmt.Sprintf("\n+++ %s", f.NewPath)
		}
		for _, hdrLine := range strings.Split(diffHeader, "\n") {
			sections = append(sections, lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Bold(true).Width(iw).Render(hdrLine))
		}

		if f.Binary {
			sections = append(sections, styles.DimStyle().Width(iw).Render("  Binary file"))
		} else {
			// Render hunks with scroll offset
			var diffLines []string
			for _, h := range f.Hunks {
				diffLines = append(diffLines, h.Header)
				diffLines = append(diffLines, h.Lines...)
			}

			// Apply scroll offset to diff lines
			startLine := l.detailScroll
			if startLine > len(diffLines) {
				startLine = len(diffLines)
			}
			if startLine < 0 {
				startLine = 0
			}

			// Compute how many diff lines we can show
			fixedLines := len(sections) + 1 // +1 for title
			contentHeight := ph - 1         // minus title
			availForDiff := contentHeight - fixedLines
			if availForDiff < 1 {
				availForDiff = 1
			}

			endLine := startLine + availForDiff
			if endLine > len(diffLines) {
				endLine = len(diffLines)
			}

			for _, line := range diffLines[startLine:endLine] {
				lineType := byte(' ')
				if len(line) > 0 {
					lineType = line[0]
				}
				rendered := expandTabs(line, 4)
				rendered = truncateToWidth(rendered, iw)
				sections = append(sections, styles.DiffLineStyle(lineType).Width(iw).Render(rendered))
			}
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return styles.PanelStyle(focused).Width(width).Height(ph).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
	)
}

func (l LogPage) renderWIPDetail(width, height int) string {
	focused := l.focus == focusLogDetail
	iw := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight
	t := theme.Active

	bgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).Width(iw).Render(s)
	}

	var sections []string

	// Count total height for file lists vs diff split
	totalFiles := len(l.wipUnstaged) + len(l.wipStaged)
	// File lists section takes: 2 headers + file count + 2 separators + commit hint area (3 lines)
	fileListHeight := totalFiles + 7
	if len(l.wipUnstaged) == 0 {
		fileListHeight++ // "working tree clean" line
	}
	if len(l.wipStaged) == 0 {
		fileListHeight++ // "no files staged" line
	}

	// --- Section 1: Unstaged Files ---
	unstagedTitle := fmt.Sprintf("▾ Unstaged Files (%d)", len(l.wipUnstaged))
	unstagedFocused := focused && l.wipFocus == wipFocusUnstaged
	titleColor := t.Subtext0
	if unstagedFocused {
		titleColor = t.Blue
	}
	sections = append(sections, lipgloss.NewStyle().Foreground(titleColor).Background(t.Base).Bold(true).Width(iw).Render(unstagedTitle))

	if len(l.wipUnstaged) == 0 {
		sections = append(sections, styles.DimStyle().Width(iw).Render("  Working tree clean"))
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
			sections = append(sections, style.Render(text))
		}
	}

	sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Surface2).Background(t.Base).Render(strings.Repeat("─", iw))))

	// --- Section 2: Staged Files ---
	stagedTitle := fmt.Sprintf("▾ Staged Files (%d)", len(l.wipStaged))
	stagedFocused := focused && l.wipFocus == wipFocusStaged
	titleColor = t.Subtext0
	if stagedFocused {
		titleColor = t.Blue
	}
	sections = append(sections, lipgloss.NewStyle().Foreground(titleColor).Background(t.Base).Bold(true).Width(iw).Render(stagedTitle))

	if len(l.wipStaged) == 0 {
		sections = append(sections, styles.DimStyle().Width(iw).Render("  No files staged"))
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
			sections = append(sections, style.Render(text))
		}
	}

	sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Surface2).Background(t.Base).Render(strings.Repeat("─", iw))))

	// --- Section 3: Commit hint area ---
	commitIcon := lipgloss.NewStyle().Foreground(t.Peach).Background(t.Base).Render("─○")
	commitLabel := lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Bold(true).Render(" Commit")
	sections = append(sections, bgLine(commitIcon+commitLabel))
	sections = append(sections, bgLine(styles.KeyHintStyle().Background(t.Base).Render(
		"space:stage/unstage  a:stage all  c:commit  A:amend  d:discard",
	)))

	sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Surface2).Background(t.Base).Render(strings.Repeat("─", iw))))

	// --- Section 4: Diff for selected file ---
	if l.wipDiffContent != "" {
		titleLine := "Diff"
		if l.wipDiffPath != "" {
			titleLine = "Diff: " + l.wipDiffPath
		}
		sections = append(sections, lipgloss.NewStyle().Foreground(t.Subtext0).Background(t.Base).Bold(true).Width(iw).Render(titleLine))

		allLines := strings.Split(l.wipDiffContent, "\n")
		startLine := l.wipDiffScroll
		if startLine > len(allLines) {
			startLine = len(allLines)
		}
		if startLine < 0 {
			startLine = 0
		}

		fixedLines := len(sections) + 1 // +1 for panel title
		contentHeight := ph - 1
		availForDiff := contentHeight - fixedLines
		if availForDiff < 1 {
			availForDiff = 1
		}

		endLine := startLine + availForDiff
		if endLine > len(allLines) {
			endLine = len(allLines)
		}

		for _, line := range allLines[startLine:endLine] {
			lineType := byte(' ')
			if len(line) > 0 {
				lineType = line[0]
			}
			rendered := expandTabs(line, 4)
			rendered = truncateToWidth(rendered, iw)
			sections = append(sections, styles.DiffLineStyle(lineType).Width(iw).Render(rendered))
		}
	}

	titleStr := styles.TitleStyle(focused).Width(iw).Render(
		fmt.Sprintf("Working Changes (%d staged, %d unstaged)", len(l.wipStaged), len(l.wipUnstaged)),
	)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return styles.PanelStyle(focused).Width(width).Height(ph).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
	)
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

func (l LogPage) loadWIPSelectedDiff() tea.Cmd {
	repo := l.repo
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
		return wipDiffMsg{path: path, staged: staged, diff: diff, err: err}
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

func (l LogPage) wipRequestCommit() tea.Cmd {
	stagedCount := len(l.wipStaged)
	return func() tea.Msg {
		return RequestCommitDialogMsg{StagedCount: stagedCount}
	}
}

func (l LogPage) wipRequestAmend() tea.Cmd {
	stagedCount := len(l.wipStaged)
	return func() tea.Msg {
		return RequestAmendDialogMsg{StagedCount: stagedCount}
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

	titleStr := styles.TitleStyle(focused).Width(iw).Render(
		fmt.Sprintf("Stash Diff (stash@{%d})", l.stashDiffIndex),
	)

	if l.stashDiffContent == "" {
		content := styles.DimStyle().Width(iw).Render("Loading stash diff...")
		return styles.PanelStyle(focused).Width(width).Height(ph).Render(
			lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
		)
	}

	allLines := strings.Split(l.stashDiffContent, "\n")
	startLine := l.detailScroll
	if startLine > len(allLines) {
		startLine = len(allLines)
	}
	if startLine < 0 {
		startLine = 0
	}

	contentHeight := ph - 1 // minus title
	if contentHeight < 1 {
		contentHeight = 1
	}
	endLine := startLine + contentHeight
	if endLine > len(allLines) {
		endLine = len(allLines)
	}

	var sections []string
	for _, line := range allLines[startLine:endLine] {
		lineType := byte(' ')
		if len(line) > 0 {
			lineType = line[0]
		}
		rendered := expandTabs(line, 4)
		rendered = truncateToWidth(rendered, iw)
		sections = append(sections, styles.DiffLineStyle(lineType).Width(iw).Render(rendered))
	}

	// Scroll hint
	if len(allLines) > contentHeight {
		hint := fmt.Sprintf("[%d/%d lines]", startLine+1, len(allLines))
		sections = append(sections, styles.KeyHintStyle().Background(t.Base).Width(iw).Render(hint))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return styles.PanelStyle(focused).Width(width).Height(ph).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
	)
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

var _ tea.Model = LogPage{}
