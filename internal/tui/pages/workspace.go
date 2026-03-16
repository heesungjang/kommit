package pages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heesungjang/kommit/internal/config"
	"github.com/heesungjang/kommit/internal/git"
	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/keys"
	"github.com/heesungjang/kommit/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// repoStatusMsg carries an async status result for a single repo.
type repoStatusMsg struct {
	path   string
	status git.RepoQuickStatus
}

// ---------------------------------------------------------------------------
// Item model (flat list of visible items)
// ---------------------------------------------------------------------------

type wsItemKind int

const (
	wsItemWorkspaceHeader wsItemKind = iota
	wsItemRepoEntry
	wsItemSeparator
	wsItemRecentHeader
	wsItemRecentEntry
)

type wsItem struct {
	kind           wsItemKind
	workspaceIndex int    // index into Config.Workspaces (-1 for recent)
	repoIndex      int    // index into workspace.Repos or RecentRepos
	label          string // display text
	path           string // repo path (for repo/recent entries)
}

// ---------------------------------------------------------------------------
// Focus targets
// ---------------------------------------------------------------------------

type wsFocus int

const (
	wsFocusList   wsFocus = iota // left panel (workspace tree)
	wsFocusDetail                // right panel (repo status detail)
)

// ---------------------------------------------------------------------------
// WorkspacePage model
// ---------------------------------------------------------------------------

// WorkspacePage is the workspace overview: a collapsible tree of workspace
// groups and recent repos (left) with a status detail panel (right).
type WorkspacePage struct {
	ctx    *tuictx.ProgramContext
	width  int
	height int
	focus  wsFocus

	// Data
	workspaces  []config.WorkspaceEntry
	recentRepos []string

	// Collapse state: index -> collapsed. Missing = expanded.
	collapsed map[int]bool // workspace index -> collapsed
	// recentCollapsed tracks whether the "Recent" section is collapsed.
	recentCollapsed bool

	cursor int // flat cursor over visibleItems()
	scroll int // viewport scroll offset

	// Cached status for repos (path -> status)
	statusCache map[string]git.RepoQuickStatus

	// Keybindings
	navKeys       keys.NavigationKeys
	workspaceKeys keys.WorkspaceKeys
}

// NewWorkspacePage creates a new workspace page.
// Returns a pointer so that Sync() (pointer receiver) works through the
// tea.Model interface assertion used in app.go.
func NewWorkspacePage(ctx *tuictx.ProgramContext, width, height int) *WorkspacePage {
	ws := ctx.Config.Workspaces
	if ws == nil {
		ws = []config.WorkspaceEntry{}
	}
	recent := ctx.Config.RecentRepos
	if recent == nil {
		recent = []string{}
	}
	return &WorkspacePage{
		ctx:           ctx,
		width:         width,
		height:        height,
		focus:         wsFocusList,
		workspaces:    ws,
		recentRepos:   recent,
		collapsed:     make(map[int]bool),
		statusCache:   make(map[string]git.RepoQuickStatus),
		navKeys:       keys.NewNavigationKeys(),
		workspaceKeys: keys.NewWorkspaceKeys(),
	}
}

// Init loads initial status for the first visible repo.
func (w *WorkspacePage) Init() tea.Cmd {
	w.ctx.ActiveKeyContext = keys.ContextWorkspace
	items := w.visibleItems()
	if len(items) == 0 {
		return nil
	}
	// Preload status for all visible repos (up to first 20).
	var cmds []tea.Cmd
	count := 0
	for _, item := range items {
		if (item.kind == wsItemRepoEntry || item.kind == wsItemRecentEntry) && item.path != "" {
			path := item.path
			cmds = append(cmds, func() tea.Msg {
				return repoStatusMsg{path: path, status: git.QuickStatus(path)}
			})
			count++
			if count >= 20 {
				break
			}
		}
	}
	return tea.Batch(cmds...)
}

// Sync refreshes workspace data from the config.
func (w *WorkspacePage) Sync() {
	w.workspaces = w.ctx.Config.Workspaces
	if w.workspaces == nil {
		w.workspaces = []config.WorkspaceEntry{}
	}
	w.recentRepos = w.ctx.Config.RecentRepos
	if w.recentRepos == nil {
		w.recentRepos = []string{}
	}
}

// visibleItems builds the flat list of items for the current collapse state.
func (w WorkspacePage) visibleItems() []wsItem {
	items := make([]wsItem, 0, len(w.workspaces)*4) // estimate: header + ~3 repos each

	for i, ws := range w.workspaces {
		items = append(items, wsItem{
			kind:           wsItemWorkspaceHeader,
			workspaceIndex: i,
			label:          ws.Name,
		})
		if !w.collapsed[i] {
			for j, repoPath := range ws.Repos {
				label := filepath.Base(repoPath)
				items = append(items, wsItem{
					kind:           wsItemRepoEntry,
					workspaceIndex: i,
					repoIndex:      j,
					label:          label,
					path:           repoPath,
				})
			}
		}
	}

	// Separator between workspace groups and recent repos
	if len(w.workspaces) > 0 && len(w.recentRepos) > 0 {
		items = append(items, wsItem{kind: wsItemSeparator, workspaceIndex: -1})
	}

	if len(w.recentRepos) > 0 {
		items = append(items, wsItem{
			kind:           wsItemRecentHeader,
			workspaceIndex: -1,
			label:          "Recent",
		})
		if !w.recentCollapsed {
			for j, repoPath := range w.recentRepos {
				label := shortenPath(repoPath)
				items = append(items, wsItem{
					kind:           wsItemRecentEntry,
					workspaceIndex: -1,
					repoIndex:      j,
					label:          label,
					path:           repoPath,
				})
			}
		}
	}

	return items
}

// currentItem returns the item at the cursor, or nil.
func (w WorkspacePage) currentItem() *wsItem {
	items := w.visibleItems()
	if w.cursor < 0 || w.cursor >= len(items) {
		return nil
	}
	return &items[w.cursor]
}

// Update processes messages.
func (w *WorkspacePage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
		return w, nil

	case repoStatusMsg:
		w.statusCache[msg.path] = msg.status
		return w, nil

	case WorkspaceBulkOpDoneMsg:
		// Refresh all statuses after a bulk operation.
		cmd := w.refreshAllStatuses()
		return w, cmd

	case tea.KeyMsg:
		return w.handleKey(msg)

	case tea.MouseMsg:
		return w, nil
	}

	return w, nil
}

// handleKey processes key input.
func (w *WorkspacePage) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := w.visibleItems()
	maxCursor := len(items) - 1
	if maxCursor < 0 {
		maxCursor = 0
	}

	switch {
	// Navigation
	case key.Matches(msg, w.navKeys.Up):
		if w.cursor > 0 {
			w.cursor--
			w.skipSeparator(items, -1)
			w.ensureVisible()
		}
		return w, w.fetchStatusForCurrent()

	case key.Matches(msg, w.navKeys.Down):
		if w.cursor < maxCursor {
			w.cursor++
			w.skipSeparator(items, +1)
			w.ensureVisible()
		}
		return w, w.fetchStatusForCurrent()

	case key.Matches(msg, w.navKeys.Home):
		w.cursor = 0
		w.skipSeparator(items, +1)
		w.scroll = 0
		return w, w.fetchStatusForCurrent()

	case key.Matches(msg, w.navKeys.End):
		w.cursor = maxCursor
		w.skipSeparator(items, -1)
		w.ensureVisible()
		return w, w.fetchStatusForCurrent()

	case key.Matches(msg, w.navKeys.PageUp):
		w.cursor -= w.listHeight()
		if w.cursor < 0 {
			w.cursor = 0
		}
		w.skipSeparator(items, -1)
		w.ensureVisible()
		return w, w.fetchStatusForCurrent()

	case key.Matches(msg, w.navKeys.PageDown):
		w.cursor += w.listHeight()
		if w.cursor > maxCursor {
			w.cursor = maxCursor
		}
		w.skipSeparator(items, +1)
		w.ensureVisible()
		return w, w.fetchStatusForCurrent()

	// Open repo
	case key.Matches(msg, w.workspaceKeys.Open):
		item := w.currentItem()
		if item == nil {
			return w, nil
		}
		switch item.kind {
		case wsItemRepoEntry, wsItemRecentEntry:
			return w, func() tea.Msg {
				return RequestOpenRepoMsg{Path: item.path}
			}
		case wsItemWorkspaceHeader:
			w.collapsed[item.workspaceIndex] = !w.collapsed[item.workspaceIndex]
			w.clampCursor()
			return w, nil
		case wsItemRecentHeader:
			w.recentCollapsed = !w.recentCollapsed
			w.clampCursor()
			return w, nil
		}

	// Toggle collapse
	case key.Matches(msg, w.workspaceKeys.ToggleCollapse):
		item := w.currentItem()
		if item == nil {
			return w, nil
		}
		switch item.kind {
		case wsItemWorkspaceHeader:
			w.collapsed[item.workspaceIndex] = !w.collapsed[item.workspaceIndex]
			w.clampCursor()
		case wsItemRecentHeader:
			w.recentCollapsed = !w.recentCollapsed
			w.clampCursor()
		}
		return w, nil

	// New workspace
	case key.Matches(msg, w.workspaceKeys.NewWorkspace):
		return w, func() tea.Msg {
			return RequestTextInputMsg{
				ID:          "workspace-new",
				Title:       "New Workspace",
				Placeholder: "Workspace name",
			}
		}

	// Delete workspace
	case key.Matches(msg, w.workspaceKeys.DeleteWorkspace):
		item := w.currentItem()
		if item == nil || item.kind != wsItemWorkspaceHeader {
			return w, nil
		}
		return w, func() tea.Msg {
			return RequestConfirmMsg{
				ID:      fmt.Sprintf("workspace-delete-%d", item.workspaceIndex),
				Title:   "Delete Workspace?",
				Message: fmt.Sprintf("Delete workspace %q? Repos will not be removed from disk.", item.label),
			}
		}

	// Rename workspace
	case key.Matches(msg, w.workspaceKeys.RenameWorkspace):
		item := w.currentItem()
		if item == nil || item.kind != wsItemWorkspaceHeader {
			return w, nil
		}
		return w, func() tea.Msg {
			return RequestTextInputMsg{
				ID:           fmt.Sprintf("workspace-rename-%d", item.workspaceIndex),
				Title:        "Rename Workspace",
				Placeholder:  "New name",
				InitialValue: item.label,
			}
		}

	// Add repo
	case key.Matches(msg, w.workspaceKeys.AddRepo):
		item := w.currentItem()
		wsIdx := -1
		if item != nil && item.kind == wsItemWorkspaceHeader {
			wsIdx = item.workspaceIndex
		} else if item != nil && item.kind == wsItemRepoEntry {
			wsIdx = item.workspaceIndex
		}
		if wsIdx < 0 && len(w.workspaces) > 0 {
			wsIdx = 0 // default to first workspace
		}
		if wsIdx < 0 {
			return w, func() tea.Msg {
				return RequestToastMsg{Message: "Create a workspace first", IsError: true}
			}
		}
		return w, func() tea.Msg {
			return RequestTextInputMsg{
				ID:          fmt.Sprintf("workspace-addrepo-%d", wsIdx),
				Title:       "Add Repository",
				Placeholder: "Absolute path to git repo",
			}
		}

	// Remove repo
	case key.Matches(msg, w.workspaceKeys.RemoveRepo):
		item := w.currentItem()
		if item == nil || item.kind != wsItemRepoEntry {
			return w, nil
		}
		return w, func() tea.Msg {
			return RequestConfirmMsg{
				ID:      fmt.Sprintf("workspace-removerepo-%d-%d", item.workspaceIndex, item.repoIndex),
				Title:   "Remove Repository?",
				Message: fmt.Sprintf("Remove %q from workspace? The repo will not be deleted from disk.", item.label),
			}
		}

	// Fetch all
	case key.Matches(msg, w.workspaceKeys.FetchAll):
		item := w.currentItem()
		wsIdx := w.resolveWorkspaceIndex(item)
		if wsIdx < 0 {
			return w, nil
		}
		return w, func() tea.Msg {
			return RequestWorkspaceFetchAllMsg{WorkspaceIndex: wsIdx}
		}

	// Pull all
	case key.Matches(msg, w.workspaceKeys.PullAll):
		item := w.currentItem()
		wsIdx := w.resolveWorkspaceIndex(item)
		if wsIdx < 0 {
			return w, nil
		}
		return w, func() tea.Msg {
			return RequestWorkspacePullAllMsg{WorkspaceIndex: wsIdx}
		}

	// Back to repo
	case key.Matches(msg, w.workspaceKeys.BackToRepo):
		return w, func() tea.Msg {
			return RequestOpenRepoMsg{Path: ""}
		}
	}

	return w, nil
}

// resolveWorkspaceIndex determines the workspace index from the current cursor.
func (w WorkspacePage) resolveWorkspaceIndex(item *wsItem) int {
	if item == nil {
		return -1
	}
	if item.kind == wsItemWorkspaceHeader || item.kind == wsItemRepoEntry {
		return item.workspaceIndex
	}
	return -1
}

// clampCursor ensures the cursor is within bounds after items change.
func (w *WorkspacePage) clampCursor() {
	items := w.visibleItems()
	if w.cursor >= len(items) {
		w.cursor = len(items) - 1
	}
	if w.cursor < 0 {
		w.cursor = 0
	}
	w.skipSeparator(items, +1)
}

// skipSeparator moves the cursor past separator items in the given direction.
func (w *WorkspacePage) skipSeparator(items []wsItem, dir int) {
	for w.cursor >= 0 && w.cursor < len(items) && items[w.cursor].kind == wsItemSeparator {
		w.cursor += dir
	}
	if w.cursor < 0 {
		w.cursor = 0
	}
	if w.cursor >= len(items) {
		w.cursor = len(items) - 1
	}
}

// listHeight returns the visible height of the list panel.
func (w WorkspacePage) listHeight() int {
	// -2 for title+gap, -1 for scroll counter
	h := w.height - styles.PanelBorderHeight - 3
	if h < 1 {
		h = 1
	}
	return h
}

// ensureVisible adjusts the scroll so the cursor is visible.
func (w *WorkspacePage) ensureVisible() {
	h := w.listHeight()
	if w.cursor < w.scroll {
		w.scroll = w.cursor
	}
	if w.cursor >= w.scroll+h {
		w.scroll = w.cursor - h + 1
	}
}

// fetchStatusForCurrent returns a cmd to fetch the status for the current item.
func (w WorkspacePage) fetchStatusForCurrent() tea.Cmd {
	item := w.currentItem()
	if item == nil {
		return nil
	}
	if item.kind != wsItemRepoEntry && item.kind != wsItemRecentEntry {
		return nil
	}
	path := item.path
	if _, ok := w.statusCache[path]; ok {
		return nil // already cached
	}
	return func() tea.Msg {
		return repoStatusMsg{path: path, status: git.QuickStatus(path)}
	}
}

// refreshAllStatuses refreshes the status cache for all visible repos.
func (w *WorkspacePage) refreshAllStatuses() tea.Cmd {
	w.statusCache = make(map[string]git.RepoQuickStatus)
	items := w.visibleItems()
	var cmds []tea.Cmd
	for _, item := range items {
		if item.kind == wsItemRepoEntry || item.kind == wsItemRecentEntry {
			path := item.path
			cmds = append(cmds, func() tea.Msg {
				return repoStatusMsg{path: path, status: git.QuickStatus(path)}
			})
		}
	}
	return tea.Batch(cmds...)
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View renders the workspace page.
func (w *WorkspacePage) View() string {
	t := w.ctx.Theme
	items := w.visibleItems()

	// Layout: left list panel (40%) | right detail panel (60%)
	leftWidth := w.width * 40 / 100
	rightWidth := w.width - leftWidth
	if leftWidth < 30 {
		leftWidth = 30
		rightWidth = w.width - leftWidth
	}

	// Content widths (subtract borders + padding)
	leftContentW := leftWidth - styles.PanelBorderWidth - styles.PanelPaddingWidth
	rightContentW := rightWidth - styles.PanelBorderWidth - styles.PanelPaddingWidth
	if leftContentW < 10 {
		leftContentW = 10
	}
	if rightContentW < 10 {
		rightContentW = 10
	}
	contentHeight := w.height - styles.PanelBorderHeight

	// -- Left panel: workspace tree --
	// Use zero-padding panel so that selection highlight extends edge-to-edge.
	// Padding is baked into each rendered line (see renderItem/renderList).
	leftLines := w.renderList(items, leftContentW, contentHeight)
	leftTitle := styles.PanelTitle("Workspaces", "W", w.focus == wsFocusList, leftContentW)
	titleGap := lipgloss.NewStyle().Background(t.Base).Width(leftContentW).Render("")
	leftBody := leftTitle + "\n" + titleGap + "\n" + strings.Join(leftLines, "\n")
	leftPanelStyle := styles.PanelStyle(w.focus == wsFocusList).Padding(0, 0)
	leftPanel := leftPanelStyle.
		Width(leftContentW + styles.PanelPaddingWidth).
		Height(contentHeight).
		Render(leftBody)
	leftPanel = styles.ClipPanel(leftPanel, w.height)

	// -- Right panel: repo detail --
	rightBody := w.renderDetail(rightContentW)
	rightTitle := styles.PanelTitle("Detail", "D", w.focus == wsFocusDetail, rightContentW)
	rightGap := lipgloss.NewStyle().Background(t.Base).Width(rightContentW).Render("")
	rightContent := rightTitle + "\n" + rightGap + "\n" + rightBody
	rightPanel := styles.PanelStyle(w.focus == wsFocusDetail).
		Width(rightContentW).
		Height(contentHeight).
		Render(rightContent)
	rightPanel = styles.ClipPanel(rightPanel, w.height)

	layout := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Fill background
	return lipgloss.NewStyle().
		Width(w.width).
		Height(w.height).
		Background(t.Base).
		Render(layout)
}

// ---------------------------------------------------------------------------
// Left panel rendering
// ---------------------------------------------------------------------------

// renderList renders the left panel list items with a scroll counter at bottom.
func (w WorkspacePage) renderList(items []wsItem, width, height int) []string {
	t := w.ctx.Theme
	// Height budget: total content height minus title(1) + gap(1) + scroll counter(1) = 3
	visibleHeight := height - 3
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	if len(items) == 0 {
		return w.renderEmptyState(width, visibleHeight)
	}

	lines := make([]string, 0, visibleHeight)
	for i := w.scroll; i < len(items) && len(lines) < visibleHeight; i++ {
		item := items[i]
		selected := i == w.cursor

		line := w.renderItem(item, selected, width)
		lines = append(lines, line)
	}

	// Pad remaining lines with manual left/right padding
	emptyPad := lipgloss.NewStyle().Background(t.Base).Render(" ")
	innerEmpty := lipgloss.NewStyle().Background(t.Base).Width(width - 2).Render("")
	emptyLine := emptyPad + innerEmpty + emptyPad
	for len(lines) < visibleHeight {
		lines = append(lines, emptyLine)
	}

	// Scroll counter pinned at bottom
	scrollInfo := ""
	if len(items) > 0 {
		scrollInfo = fmt.Sprintf("%d/%d", w.cursor+1, len(items))
	}
	scrollLine := lipgloss.NewStyle().Background(t.Base).Width(width).Render(
		lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render(scrollInfo),
	)
	lines = append(lines, scrollLine)

	return lines
}

// renderEmptyState renders a bordered empty state with key hints.
func (w WorkspacePage) renderEmptyState(width, visibleHeight int) []string {
	t := w.ctx.Theme

	titleLine := lipgloss.NewStyle().
		Foreground(t.Subtext0).Background(t.Base).Bold(true).
		Render("No workspaces yet")

	hint1 := w.renderKeyHints([]keyHint{
		{key: "n", desc: "create workspace"},
	}, t.Base)
	hint2 := w.renderKeyHints([]keyHint{
		{key: "a", desc: "add repository"},
	}, t.Base)

	boxContent := titleLine + "\n\n" + hint1 + "\n" + hint2

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Surface2).
		BorderBackground(t.Base).
		Background(t.Base).
		Padding(1, 3).
		Render(boxContent)

	// Center the box
	boxWidth := lipgloss.Width(box)
	leftPad := 0
	if boxWidth < width {
		leftPad = (width - boxWidth) / 2
	}
	padStr := ""
	if leftPad > 0 {
		padStr = lipgloss.NewStyle().Background(t.Base).Render(strings.Repeat(" ", leftPad))
	}

	boxLines := strings.Split(box, "\n")
	result := make([]string, 0, visibleHeight)

	// Vertical centering: add empty lines above the box
	topPad := 0
	if len(boxLines) < visibleHeight {
		topPad = (visibleHeight - len(boxLines)) / 3 // bias toward upper-third
	}
	bgEmpty := lipgloss.NewStyle().Background(t.Base).Width(width).Render("")
	for i := 0; i < topPad && len(result) < visibleHeight; i++ {
		result = append(result, bgEmpty)
	}
	for _, bl := range boxLines {
		if len(result) >= visibleHeight {
			break
		}
		result = append(result, padStr+bl)
	}
	for len(result) < visibleHeight {
		result = append(result, bgEmpty)
	}

	return result
}

// renderItem renders a single list item as a single line.
func (w WorkspacePage) renderItem(item wsItem, selected bool, width int) string {
	t := w.ctx.Theme

	bg := t.Base
	if selected {
		bg = t.Surface1
	}

	// pad is a 1-char left/right margin rendered with the row's bg so
	// that the selection highlight extends edge-to-edge within the panel.
	pad := lipgloss.NewStyle().Background(bg).Render(" ")

	// padLine ensures the inner content fills exactly (width-2) visible
	// characters, then wraps it with the left/right pad.
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	padLine := func(line string) string {
		vis := lipgloss.Width(line)
		if vis < innerWidth {
			line += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", innerWidth-vis))
		}
		return pad + line + pad
	}

	switch item.kind {
	case wsItemWorkspaceHeader:
		chevron := "\u25be" // ▾
		if w.collapsed[item.workspaceIndex] {
			chevron = "\u25b8" // ▸
		}
		count := len(w.workspaces[item.workspaceIndex].Repos)
		label := fmt.Sprintf("%s %s (%d)", chevron, item.label, count)

		headerStyle := lipgloss.NewStyle().
			Foreground(t.Subtext0).
			Background(bg).
			Bold(true)

		return padLine(headerStyle.Render(label))

	case wsItemRecentHeader:
		chevron := "\u25be" // ▾
		if w.recentCollapsed {
			chevron = "\u25b8" // ▸
		}
		headerStyle := lipgloss.NewStyle().
			Foreground(t.Subtext0).
			Background(bg).
			Bold(true)

		return padLine(headerStyle.Render(fmt.Sprintf("%s %s", chevron, item.label)))

	case wsItemSeparator:
		sep := lipgloss.NewStyle().
			Foreground(t.Surface2).
			Background(bg).
			Render(strings.Repeat("\u2500", innerWidth))
		return padLine(sep)

	case wsItemRepoEntry, wsItemRecentEntry:
		// Cursor prefix: ▸ when selected, spaces when not
		prefix := "  "
		if selected {
			prefix = "\u25b8 "
		}
		prefixStr := lipgloss.NewStyle().Foreground(t.Text).Background(bg).Render(prefix)

		// Status dot
		indicator := w.statusIndicator(item.path, bg)
		space := lipgloss.NewStyle().Background(bg).Render(" ")

		// Repo name
		nameStr := lipgloss.NewStyle().Foreground(t.Text).Background(bg).Render(item.label)

		// Branch + ahead/behind from cache
		var extras string
		if qs, ok := w.statusCache[item.path]; ok && qs.Error == "" {
			branchStr := lipgloss.NewStyle().Foreground(t.Green).Background(bg).Italic(true).
				Render(qs.Branch)
			extras = space + space + branchStr
			if qs.Ahead > 0 {
				extras += lipgloss.NewStyle().Foreground(t.Green).Background(bg).
					Render(fmt.Sprintf(" \u2191%d", qs.Ahead))
			}
			if qs.Behind > 0 {
				extras += lipgloss.NewStyle().Foreground(t.Red).Background(bg).
					Render(fmt.Sprintf(" \u2193%d", qs.Behind))
			}
		}

		line := prefixStr + indicator + space + nameStr + extras
		return padLine(line)
	}

	text := lipgloss.NewStyle().Foreground(t.Text).Background(bg).Render(item.label)
	return padLine(text)
}

// statusIndicator returns a colored dot for the repo's status with the given bg.
func (w WorkspacePage) statusIndicator(path string, bg lipgloss.Color) string {
	t := w.ctx.Theme
	qs, ok := w.statusCache[path]
	if !ok {
		return lipgloss.NewStyle().Foreground(t.Overlay0).Background(bg).Render("\u25cb") // ○ loading
	}
	if qs.Error != "" {
		return lipgloss.NewStyle().Foreground(t.Red).Background(bg).Render("\u25cf") // ● error
	}
	if qs.Conflicts > 0 {
		return lipgloss.NewStyle().Foreground(t.Red).Background(bg).Render("\u25cf") // ● conflicts
	}
	if qs.IsClean {
		return lipgloss.NewStyle().Foreground(t.Green).Background(bg).Render("\u25cf") // ● clean
	}
	return lipgloss.NewStyle().Foreground(t.Yellow).Background(bg).Render("\u25cf") // ● dirty
}

// ---------------------------------------------------------------------------
// Right panel rendering
// ---------------------------------------------------------------------------

// renderDetail renders the right panel with detailed status for the selected item.
func (w WorkspacePage) renderDetail(width int) string {
	item := w.currentItem()

	if item == nil {
		return w.renderCenteredMessage("No item selected", width)
	}

	switch item.kind {
	case wsItemWorkspaceHeader:
		return w.renderWorkspaceDetail(item, width)
	case wsItemRepoEntry, wsItemRecentEntry:
		return w.renderRepoDetail(item, width)
	case wsItemRecentHeader:
		return w.renderCenteredMessage("Recent repositories", width)
	case wsItemSeparator:
		return w.renderCenteredMessage("", width)
	}

	return ""
}

// renderWorkspaceDetail shows an overview of a workspace with mini repo table.
func (w WorkspacePage) renderWorkspaceDetail(item *wsItem, width int) string {
	t := w.ctx.Theme
	ws := w.workspaces[item.workspaceIndex]

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Blue).
		Background(t.Base).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Base)

	sepStyle := lipgloss.NewStyle().
		Foreground(t.Surface2).
		Background(t.Base)

	var sb strings.Builder

	// Header
	sb.WriteString(titleStyle.Render(ws.Name))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render(fmt.Sprintf("%d repositories", len(ws.Repos))))
	sb.WriteString("\n\n")

	// Separator
	sepW := width - 2
	if sepW < 4 {
		sepW = 4
	}
	sep := sepStyle.Render(strings.Repeat("\u2500", sepW))
	sb.WriteString(sep)
	sb.WriteString("\n\n")

	// Summary health — inline
	clean, dirty, errored, loading := 0, 0, 0, 0
	for _, repoPath := range ws.Repos {
		qs, ok := w.statusCache[repoPath]
		if !ok {
			loading++
			continue
		}
		if qs.Error != "" {
			errored++
		} else if qs.IsClean {
			clean++
		} else {
			dirty++
		}
	}

	sectionLabel := lipgloss.NewStyle().
		Foreground(t.Subtext0).Background(t.Base).Bold(true)

	sb.WriteString(sectionLabel.Render("Summary"))
	sb.WriteString("\n")

	var summaryParts []string
	statusBg := t.Base
	if clean > 0 {
		summaryParts = append(summaryParts,
			lipgloss.NewStyle().Foreground(t.Green).Background(statusBg).
				Render(fmt.Sprintf("\u25cf %d clean", clean)))
	}
	if dirty > 0 {
		summaryParts = append(summaryParts,
			lipgloss.NewStyle().Foreground(t.Yellow).Background(statusBg).
				Render(fmt.Sprintf("\u25cf %d dirty", dirty)))
	}
	if errored > 0 {
		summaryParts = append(summaryParts,
			lipgloss.NewStyle().Foreground(t.Red).Background(statusBg).
				Render(fmt.Sprintf("\u25cf %d errors", errored)))
	}
	if loading > 0 {
		summaryParts = append(summaryParts,
			lipgloss.NewStyle().Foreground(t.Overlay0).Background(statusBg).
				Render(fmt.Sprintf("\u25cb %d loading", loading)))
	}
	if len(summaryParts) > 0 {
		spacer := lipgloss.NewStyle().Background(statusBg).Render("  ")
		sb.WriteString("  ")
		sb.WriteString(strings.Join(summaryParts, spacer))
	} else {
		sb.WriteString(dimStyle.Render("  No repos"))
	}
	sb.WriteString("\n\n")

	// Separator
	sb.WriteString(sep)
	sb.WriteString("\n\n")

	// Mini repo table
	if len(ws.Repos) > 0 {
		sb.WriteString(sectionLabel.Render("Repositories"))
		sb.WriteString("\n")

		for _, repoPath := range ws.Repos {
			name := filepath.Base(repoPath)
			qs, ok := w.statusCache[repoPath]

			// Status dot
			dot := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render("\u25cb")
			branch := ""
			statusHint := ""
			if ok {
				if qs.Error != "" {
					dot = lipgloss.NewStyle().Foreground(t.Red).Background(t.Base).Render("\u25cf")
					statusHint = lipgloss.NewStyle().Foreground(t.Red).Background(t.Base).Render("err")
				} else if qs.IsClean {
					dot = lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).Render("\u25cf")
					statusHint = lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).Render("\u2713")
				} else {
					dot = lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).Render("\u25cf")
					changes := qs.Modified + qs.Staged + qs.Untracked
					statusHint = lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).
						Render(fmt.Sprintf("M%d", changes))
				}
				branch = lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).
					Render(qs.Branch)
			}

			nameStr := lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Render(name)
			// Truncate name to reasonable width
			maxName := width/3 - 4
			if maxName < 8 {
				maxName = 8
			}
			if len(name) > maxName {
				name = name[:maxName-1] + "\u2026"
				nameStr = lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Render(name)
			}

			sp := lipgloss.NewStyle().Background(t.Base).Render(" ")
			gap := lipgloss.NewStyle().Background(t.Base).Render("  ")

			line := "  " + dot + sp + nameStr
			if branch != "" {
				line += gap + branch
			}
			if statusHint != "" {
				line += gap + statusHint
			}
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Separator
	sb.WriteString(sep)
	sb.WriteString("\n\n")

	// Key hints
	sb.WriteString(w.renderKeyHints([]keyHint{
		{key: "enter", desc: "expand"},
		{key: "n", desc: "new"},
		{key: "R", desc: "rename"},
		{key: "D", desc: "delete"},
	}, t.Base))
	sb.WriteString("\n")
	sb.WriteString(w.renderKeyHints([]keyHint{
		{key: "f", desc: "fetch all"},
		{key: "P", desc: "pull all"},
	}, t.Base))

	return sb.String()
}

// renderRepoDetail shows detailed status for a single repo in a structured card.
func (w WorkspacePage) renderRepoDetail(item *wsItem, width int) string {
	t := w.ctx.Theme

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Base).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Base)

	sepStyle := lipgloss.NewStyle().
		Foreground(t.Surface2).
		Background(t.Base)

	sepW := width - 2
	if sepW < 4 {
		sepW = 4
	}
	sep := sepStyle.Render(strings.Repeat("\u2500", sepW))

	qs, ok := w.statusCache[item.path]

	var sb strings.Builder

	// Header: repo name + path
	sb.WriteString(titleStyle.Render(item.label))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render(shortenPath(item.path)))
	sb.WriteString("\n\n")

	if !ok {
		sb.WriteString(dimStyle.Render("Loading status..."))
		return sb.String()
	}

	if qs.Error != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Red).Background(t.Base).Render("Error: " + qs.Error))
		return sb.String()
	}

	// Separator
	sb.WriteString(sep)
	sb.WriteString("\n\n")

	// Branch section with ⑂ icon
	branchIcon := lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).Render("\u2442")
	branchName := lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).Bold(true).
		Render(" " + qs.Branch)
	branchLine := branchIcon + branchName

	// Ahead/behind on same line
	if qs.Ahead > 0 || qs.Behind > 0 {
		gap := lipgloss.NewStyle().Background(t.Base).Render("  ")
		branchLine += gap

		if qs.Ahead > 0 {
			branchLine += lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).
				Render(fmt.Sprintf("\u2191%d", qs.Ahead))
		}
		if qs.Ahead > 0 && qs.Behind > 0 {
			branchLine += lipgloss.NewStyle().Background(t.Base).Render(" ")
		}
		if qs.Behind > 0 {
			branchLine += lipgloss.NewStyle().Foreground(t.Red).Background(t.Base).
				Render(fmt.Sprintf("\u2193%d", qs.Behind))
		}
	}
	sb.WriteString(branchLine)
	sb.WriteString("\n\n")

	// Separator
	sb.WriteString(sep)
	sb.WriteString("\n\n")

	// Status section
	if qs.IsClean {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).
			Render("\u2713 Working tree clean"))
		sb.WriteString("\n")
	} else {
		sectionLabel := lipgloss.NewStyle().
			Foreground(t.Subtext0).Background(t.Base).Bold(true)
		sb.WriteString(sectionLabel.Render("Changes"))
		sb.WriteString("\n")

		// Stat bar
		total := qs.Staged + qs.Modified + qs.Untracked + qs.Conflicts
		if total > 0 {
			sb.WriteString("  ")
			sb.WriteString(w.renderStatBar(qs, width-6))
			sb.WriteString("\n")
		}

		sb.WriteString("\n")

		if qs.Staged > 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).
				Render(fmt.Sprintf("  \u25cf %d staged", qs.Staged)))
			sb.WriteString("\n")
		}
		if qs.Modified > 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).
				Render(fmt.Sprintf("  \u25cf %d modified", qs.Modified)))
			sb.WriteString("\n")
		}
		if qs.Untracked > 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).
				Render(fmt.Sprintf("  \u25cb %d untracked", qs.Untracked)))
			sb.WriteString("\n")
		}
		if qs.Conflicts > 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(t.Red).Background(t.Base).Bold(true).
				Render(fmt.Sprintf("  ! %d conflicts", qs.Conflicts)))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")

	// Separator
	sb.WriteString(sep)
	sb.WriteString("\n\n")

	// Key hints
	hints := []keyHint{
		{key: "enter", desc: "open"},
	}
	if item.kind == wsItemRepoEntry {
		hints = append(hints, keyHint{key: "x", desc: "remove"})
	}
	hints = append(hints, keyHint{key: "f", desc: "fetch"})
	sb.WriteString(w.renderKeyHints(hints, t.Base))

	return sb.String()
}

// renderStatBar renders a visual bar showing proportions of staged/modified/untracked changes.
func (w WorkspacePage) renderStatBar(qs git.RepoQuickStatus, barWidth int) string {
	t := w.ctx.Theme

	total := qs.Staged + qs.Modified + qs.Untracked + qs.Conflicts
	if total == 0 || barWidth < 4 {
		return ""
	}

	if barWidth > 30 {
		barWidth = 30
	}

	// Calculate proportions
	stagedW := qs.Staged * barWidth / total
	conflictW := qs.Conflicts * barWidth / total
	modifiedW := qs.Modified * barWidth / total
	untrackedW := barWidth - stagedW - conflictW - modifiedW
	if untrackedW < 0 {
		untrackedW = 0
	}

	// Ensure at least 1 cell for each non-zero category
	if qs.Staged > 0 && stagedW == 0 {
		stagedW = 1
	}
	if qs.Conflicts > 0 && conflictW == 0 {
		conflictW = 1
	}
	if qs.Modified > 0 && modifiedW == 0 {
		modifiedW = 1
	}
	if qs.Untracked > 0 && untrackedW == 0 {
		untrackedW = 1
	}
	// Clamp total to barWidth
	for stagedW+conflictW+modifiedW+untrackedW > barWidth {
		if untrackedW > 1 {
			untrackedW--
		} else if modifiedW > 1 {
			modifiedW--
		} else if stagedW > 1 {
			stagedW--
		} else {
			break
		}
	}

	var bar string
	if stagedW > 0 {
		bar += lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).
			Render(strings.Repeat("\u2588", stagedW))
	}
	if conflictW > 0 {
		bar += lipgloss.NewStyle().Foreground(t.Red).Background(t.Base).
			Render(strings.Repeat("\u2588", conflictW))
	}
	if modifiedW > 0 {
		bar += lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).
			Render(strings.Repeat("\u2588", modifiedW))
	}
	if untrackedW > 0 {
		bar += lipgloss.NewStyle().Foreground(t.Surface2).Background(t.Base).
			Render(strings.Repeat("\u2591", untrackedW))
	}

	// Percentage label
	pct := ""
	if total > 0 {
		cleanPct := 0
		if qs.Staged > 0 {
			cleanPct = qs.Staged * 100 / total
		}
		pct = lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).
			Render(fmt.Sprintf("  %d%%", cleanPct) + " staged")
	}

	return bar + pct
}

// renderCenteredMessage renders a centered dim message.
func (w WorkspacePage) renderCenteredMessage(msg string, width int) string {
	t := w.ctx.Theme
	if msg == "" {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Base).
		Width(width).
		Render(msg)
}

// ---------------------------------------------------------------------------
// Key hint rendering
// ---------------------------------------------------------------------------

// keyHint describes a single key hint: key + description.
type keyHint struct {
	key  string
	desc string
}

// renderKeyHints renders styled key hints: key in Mauve+Bold, desc in Subtext0.
func (w WorkspacePage) renderKeyHints(hints []keyHint, bg lipgloss.Color) string {
	t := w.ctx.Theme
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		k := lipgloss.NewStyle().Foreground(t.Mauve).Background(bg).Bold(true).Render(h.key)
		d := lipgloss.NewStyle().Foreground(t.Subtext0).Background(bg).Render(":" + h.desc)
		parts = append(parts, k+d)
	}
	spacer := lipgloss.NewStyle().Background(bg).Render("  ")
	return strings.Join(parts, spacer)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// shortenPath replaces the home directory with ~ for display.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
