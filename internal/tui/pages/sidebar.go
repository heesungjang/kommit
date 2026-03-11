package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nicholascross/opengit/internal/git"
	"github.com/nicholascross/opengit/internal/tui/keys"
	"github.com/nicholascross/opengit/internal/tui/styles"
	"github.com/nicholascross/opengit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// sidebarLoadedMsg carries all sidebar data after an async load.
type sidebarLoadedMsg struct {
	localBranches  []git.BranchInfo
	remoteBranches []git.BranchInfo
	tags           []git.TagInfo
	stashes        []git.StashEntry
	err            error
}

// sidebarOpDoneMsg is sent after a sidebar-initiated operation completes.
type sidebarOpDoneMsg struct {
	action string
	err    error
}

// SidebarViewStashMsg requests the main view to show a stash diff in the right panel.
type SidebarViewStashMsg struct {
	Index int
}

// ---------------------------------------------------------------------------
// Section types
// ---------------------------------------------------------------------------

type sidebarSection int

const (
	sectionLocal sidebarSection = iota
	sectionRemote
	sectionTags
	sectionStash
	sectionCount // sentinel — number of sections
)

// ---------------------------------------------------------------------------
// Sidebar model
// ---------------------------------------------------------------------------

// Sidebar is the left tree panel showing branches, remotes, tags, stash.
type Sidebar struct {
	repo *git.Repository

	// Data
	localBranches  []git.BranchInfo
	remoteBranches []git.BranchInfo
	tags           []git.TagInfo
	stashes        []git.StashEntry

	// Section collapsed state
	collapsed [sectionCount]bool

	// Navigation: flat cursor over visible items
	cursor int
	scroll int // viewport scroll offset

	// Pending operations for dialogs
	pendingRenameBranch string
	pendingDeleteBranch string
	pendingDeleteTag    string
	pendingDropStash    int
	pendingRebaseBranch string

	// State
	loading bool
	err     error
	filter  string // search filter for sidebar items (client-side)

	// Keys
	navKeys    keys.NavigationKeys
	branchKeys keys.BranchKeys
	stashKeys  keys.StashKeys

	// Dimensions
	width  int
	height int
}

// NewSidebar creates a new sidebar panel.
func NewSidebar(repo *git.Repository, width, height int) Sidebar {
	return Sidebar{
		repo:             repo,
		navKeys:          keys.NewNavigationKeys(),
		branchKeys:       keys.NewBranchKeys(),
		stashKeys:        keys.NewStashKeys(),
		width:            width,
		height:           height,
		loading:          true,
		pendingDropStash: -1,
	}
}

// Init loads sidebar data.
func (s Sidebar) Init() tea.Cmd {
	return s.loadData()
}

// ---------------------------------------------------------------------------
// Flat item list — maps cursor positions to tree items
// ---------------------------------------------------------------------------

type sidebarItemKind int

const (
	itemSectionHeader sidebarItemKind = iota
	itemLocalBranch
	itemRemoteBranch
	itemTag
	itemStash
)

type sidebarItem struct {
	kind    sidebarItemKind
	section sidebarSection
	index   int    // index into the corresponding data slice
	label   string // display text
}

// visibleItems builds a flat list of all currently visible items (expanding
// non-collapsed sections).
func (s Sidebar) visibleItems() []sidebarItem {
	var items []sidebarItem

	// LOCAL section
	items = append(items, sidebarItem{kind: itemSectionHeader, section: sectionLocal, label: fmt.Sprintf("LOCAL (%d)", len(s.localBranches))})
	if !s.collapsed[sectionLocal] {
		for i, b := range s.localBranches {
			items = append(items, sidebarItem{kind: itemLocalBranch, section: sectionLocal, index: i, label: b.Name})
		}
	}

	// REMOTE section
	items = append(items, sidebarItem{kind: itemSectionHeader, section: sectionRemote, label: fmt.Sprintf("REMOTE (%d)", len(s.remoteBranches))})
	if !s.collapsed[sectionRemote] {
		for i, b := range s.remoteBranches {
			items = append(items, sidebarItem{kind: itemRemoteBranch, section: sectionRemote, index: i, label: b.Name})
		}
	}

	// TAGS section
	items = append(items, sidebarItem{kind: itemSectionHeader, section: sectionTags, label: fmt.Sprintf("TAGS (%d)", len(s.tags))})
	if !s.collapsed[sectionTags] {
		for i, t := range s.tags {
			items = append(items, sidebarItem{kind: itemTag, section: sectionTags, index: i, label: t.Name})
		}
	}

	// STASH section
	items = append(items, sidebarItem{kind: itemSectionHeader, section: sectionStash, label: fmt.Sprintf("STASH (%d)", len(s.stashes))})
	if !s.collapsed[sectionStash] {
		for i, e := range s.stashes {
			msg := e.Message
			if msg == "" {
				msg = e.Ref
			}
			items = append(items, sidebarItem{kind: itemStash, section: sectionStash, index: i, label: msg})
		}
	}

	// Apply filter if set
	if s.filter != "" {
		var filtered []sidebarItem
		for _, item := range items {
			// Always show section headers
			if item.kind == itemSectionHeader {
				filtered = append(filtered, item)
				continue
			}
			if strings.Contains(strings.ToLower(item.label), s.filter) {
				filtered = append(filtered, item)
			}
		}
		return filtered
	}

	return items
}

// currentItem returns the item at the current cursor position, or nil.
func (s Sidebar) currentItem() *sidebarItem {
	items := s.visibleItems()
	if s.cursor >= 0 && s.cursor < len(items) {
		it := items[s.cursor]
		return &it
	}
	return nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (s Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case sidebarLoadedMsg:
		s.loading = false
		if msg.err != nil {
			s.err = msg.err
			return s, nil
		}
		s.err = nil
		s.localBranches = msg.localBranches
		s.remoteBranches = msg.remoteBranches
		s.tags = msg.tags
		s.stashes = msg.stashes
		// Clamp cursor
		items := s.visibleItems()
		if s.cursor >= len(items) {
			s.cursor = len(items) - 1
		}
		if s.cursor < 0 {
			s.cursor = 0
		}
		return s, nil

	case sidebarOpDoneMsg:
		if msg.err != nil {
			return s, func() tea.Msg {
				return RequestToastMsg{Message: msg.action + " failed: " + msg.err.Error(), IsError: true}
			}
		}
		// Show success toast and reload
		return s, tea.Batch(
			func() tea.Msg { return RequestToastMsg{Message: msg.action + " complete"} },
			s.loadData(),
			func() tea.Msg { return RefreshStatusMsg{} },
		)

	case tea.KeyMsg:
		return s.handleKey(msg)
	}

	return s, nil
}

// HandleDialogResult processes dialog results routed from the app shell.
func (s Sidebar) HandleDialogResult(msg tea.Msg) (Sidebar, tea.Cmd) {
	switch msg := msg.(type) {
	case dialogTextInputResultMsg:
		switch msg.ID {
		case "sidebar-new-branch":
			if msg.Value != "" {
				return s, s.createBranch(msg.Value)
			}
		case "sidebar-rename-branch":
			if msg.Value != "" && s.pendingRenameBranch != "" {
				old := s.pendingRenameBranch
				s.pendingRenameBranch = ""
				return s, s.renameBranch(old, msg.Value)
			}
		case "sidebar-stash-save":
			return s, s.stashSave(msg.Value)
		case "sidebar-new-tag":
			if msg.Value != "" {
				return s, s.createTag(msg.Value)
			}
		}

	case dialogTextInputCancelMsg:
		s.pendingRenameBranch = ""

	case dialogConfirmResultMsg:
		switch msg.ID {
		case "sidebar-delete-branch":
			if msg.Confirmed && s.pendingDeleteBranch != "" {
				name := s.pendingDeleteBranch
				s.pendingDeleteBranch = ""
				return s, s.deleteBranch(name)
			}
			s.pendingDeleteBranch = ""
		case "sidebar-stash-drop":
			if msg.Confirmed && s.pendingDropStash >= 0 {
				idx := s.pendingDropStash
				s.pendingDropStash = -1
				return s, s.stashDrop(idx)
			}
			s.pendingDropStash = -1
		case "sidebar-delete-tag":
			if msg.Confirmed && s.pendingDeleteTag != "" {
				name := s.pendingDeleteTag
				s.pendingDeleteTag = ""
				return s, s.deleteTag(name)
			}
			s.pendingDeleteTag = ""
		case "sidebar-rebase":
			if msg.Confirmed && s.pendingRebaseBranch != "" {
				name := s.pendingRebaseBranch
				s.pendingRebaseBranch = ""
				return s, s.rebaseBranch(name)
			}
			s.pendingRebaseBranch = ""
		}
	}

	return s, nil
}

// dialogTextInputResultMsg / dialogTextInputCancelMsg / dialogConfirmResultMsg
// are thin wrappers that the log page routes to the sidebar after the app
// shell closes a dialog. They mirror the dialog package's types but live here
// so the sidebar can match on them without importing the dialog package.
type dialogTextInputResultMsg struct {
	ID    string
	Value string
}
type dialogTextInputCancelMsg struct {
	ID string
}
type dialogConfirmResultMsg struct {
	ID        string
	Confirmed bool
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (s Sidebar) handleKey(msg tea.KeyMsg) (Sidebar, tea.Cmd) {
	items := s.visibleItems()
	maxCursor := len(items) - 1
	if maxCursor < 0 {
		maxCursor = 0
	}

	switch {
	case key.Matches(msg, s.navKeys.Down):
		if s.cursor < maxCursor {
			s.cursor++
		}
	case key.Matches(msg, s.navKeys.Up):
		if s.cursor > 0 {
			s.cursor--
		}
	case key.Matches(msg, s.navKeys.Home):
		s.cursor = 0
	case key.Matches(msg, s.navKeys.End):
		s.cursor = maxCursor
	case key.Matches(msg, s.navKeys.PageDown):
		s.cursor += 10
		if s.cursor > maxCursor {
			s.cursor = maxCursor
		}
	case key.Matches(msg, s.navKeys.PageUp):
		s.cursor -= 10
		if s.cursor < 0 {
			s.cursor = 0
		}

	// Enter or Space: toggle section collapse, or activate item
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
		return s.handleActivate()

	// Branch / Tag actions — routed by current section
	case key.Matches(msg, s.branchKeys.New):
		cur := s.currentItem()
		if cur != nil && cur.section == sectionTags {
			return s, func() tea.Msg {
				return RequestTextInputMsg{ID: "sidebar-new-tag", Title: "New Tag (at HEAD)", Placeholder: "tag-name"}
			}
		}
		return s, func() tea.Msg {
			return RequestTextInputMsg{ID: "sidebar-new-branch", Title: "New Branch", Placeholder: "branch-name"}
		}
	case key.Matches(msg, s.branchKeys.Rename):
		return s.handleRename()
	case key.Matches(msg, s.branchKeys.Delete):
		cur := s.currentItem()
		if cur != nil && cur.section == sectionTags {
			return s.handleDeleteTag()
		}
		return s.handleDelete()
	case key.Matches(msg, s.branchKeys.Merge):
		return s.handleMerge()
	case key.Matches(msg, s.branchKeys.Rebase):
		return s.handleRebase()

	// Stash actions
	case key.Matches(msg, s.stashKeys.Save):
		return s, func() tea.Msg {
			return RequestTextInputMsg{ID: "sidebar-stash-save", Title: "Stash Save", Placeholder: "optional message"}
		}
	case key.Matches(msg, s.stashKeys.Pop):
		return s.handleStashPop()
	case key.Matches(msg, s.stashKeys.Apply):
		return s.handleStashApply()
	case key.Matches(msg, s.stashKeys.Drop):
		return s.handleStashDrop()
	}

	return s, nil
}

func (s Sidebar) handleActivate() (Sidebar, tea.Cmd) {
	item := s.currentItem()
	if item == nil {
		return s, nil
	}

	// Section header: toggle collapsed
	if item.kind == itemSectionHeader {
		s.collapsed[item.section] = !s.collapsed[item.section]
		// Clamp cursor after collapse
		items := s.visibleItems()
		if s.cursor >= len(items) {
			s.cursor = len(items) - 1
		}
		return s, nil
	}

	// Branch item: checkout
	if item.kind == itemLocalBranch && item.index < len(s.localBranches) {
		b := s.localBranches[item.index]
		if b.IsCurrent {
			return s, nil // already on this branch
		}
		return s, s.checkout(b.Name)
	}

	// Remote branch: checkout (creates local tracking branch)
	if item.kind == itemRemoteBranch && item.index < len(s.remoteBranches) {
		b := s.remoteBranches[item.index]
		return s, s.checkout(b.Name)
	}

	// Tag: checkout (detached HEAD)
	if item.kind == itemTag && item.index < len(s.tags) {
		t := s.tags[item.index]
		return s, s.checkout(t.Name)
	}

	// Stash: view diff in right panel
	if item.kind == itemStash && item.index < len(s.stashes) {
		return s, func() tea.Msg {
			return SidebarViewStashMsg{Index: s.stashes[item.index].Index}
		}
	}

	return s, nil
}

func (s Sidebar) handleRename() (Sidebar, tea.Cmd) {
	item := s.currentItem()
	if item == nil || item.kind != itemLocalBranch || item.index >= len(s.localBranches) {
		return s, nil
	}
	b := s.localBranches[item.index]
	s.pendingRenameBranch = b.Name
	return s, func() tea.Msg {
		return RequestTextInputMsg{
			ID:           "sidebar-rename-branch",
			Title:        "Rename Branch",
			Placeholder:  "new-name",
			InitialValue: b.Name,
		}
	}
}

func (s Sidebar) handleDelete() (Sidebar, tea.Cmd) {
	item := s.currentItem()
	if item == nil || item.kind != itemLocalBranch || item.index >= len(s.localBranches) {
		return s, nil
	}
	b := s.localBranches[item.index]
	if b.IsCurrent {
		return s, func() tea.Msg {
			return RequestToastMsg{Message: "Cannot delete the current branch", IsError: true}
		}
	}
	s.pendingDeleteBranch = b.Name
	return s, func() tea.Msg {
		return RequestConfirmMsg{
			ID:      "sidebar-delete-branch",
			Title:   "Delete Branch",
			Message: fmt.Sprintf("Delete branch '%s'?\n\nThis cannot be undone.", b.Name),
		}
	}
}

func (s Sidebar) handleDeleteTag() (Sidebar, tea.Cmd) {
	item := s.currentItem()
	if item == nil || item.kind != itemTag || item.index >= len(s.tags) {
		return s, nil
	}
	t := s.tags[item.index]
	s.pendingDeleteTag = t.Name
	return s, func() tea.Msg {
		return RequestConfirmMsg{
			ID:      "sidebar-delete-tag",
			Title:   "Delete Tag",
			Message: fmt.Sprintf("Delete tag '%s'?\n\nThis cannot be undone.", t.Name),
		}
	}
}

func (s Sidebar) handleMerge() (Sidebar, tea.Cmd) {
	item := s.currentItem()
	if item == nil {
		return s, nil
	}
	var name string
	switch item.kind {
	case itemLocalBranch:
		if item.index < len(s.localBranches) {
			b := s.localBranches[item.index]
			if b.IsCurrent {
				return s, nil
			}
			name = b.Name
		}
	case itemRemoteBranch:
		if item.index < len(s.remoteBranches) {
			name = s.remoteBranches[item.index].Name
		}
	default:
		return s, nil
	}
	if name == "" {
		return s, nil
	}
	return s, s.mergeBranch(name)
}

func (s Sidebar) handleStashPop() (Sidebar, tea.Cmd) {
	item := s.currentItem()
	if item == nil || item.kind != itemStash || item.index >= len(s.stashes) {
		return s, nil
	}
	return s, s.stashPop(s.stashes[item.index].Index)
}

func (s Sidebar) handleStashApply() (Sidebar, tea.Cmd) {
	item := s.currentItem()
	if item == nil || item.kind != itemStash || item.index >= len(s.stashes) {
		return s, nil
	}
	return s, s.stashApply(s.stashes[item.index].Index)
}

func (s Sidebar) handleStashDrop() (Sidebar, tea.Cmd) {
	item := s.currentItem()
	if item == nil || item.kind != itemStash || item.index >= len(s.stashes) {
		return s, nil
	}
	e := s.stashes[item.index]
	s.pendingDropStash = e.Index
	return s, func() tea.Msg {
		return RequestConfirmMsg{
			ID:      "sidebar-stash-drop",
			Title:   "Drop Stash",
			Message: fmt.Sprintf("Drop stash entry '%s'?\n\nThis cannot be undone.", e.Ref),
		}
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (s Sidebar) View(focused bool, borderColor lipgloss.Color) string {
	t := theme.Active
	iw := s.width - styles.PanelPaddingWidth
	ph := s.height - styles.PanelBorderHeight
	if iw < 4 {
		iw = 4
	}
	if ph < 1 {
		ph = 1
	}

	items := s.visibleItems()

	if s.loading {
		content := styles.DimStyle().Width(iw).Render("Loading...")
		return styles.ClipPanel(styles.PanelStyleColor(borderColor).Width(s.width).Height(ph).Render(content), s.height)
	}

	// Build hints with scroll position indicator
	hintKeys := "enter:act  n:new  D:del"
	scrollInfo := ""
	if len(items) > 0 {
		scrollInfo = fmt.Sprintf(" %d/%d", s.cursor+1, len(items))
	}
	hintRendered := lipgloss.NewStyle().Background(t.Base).Width(iw).Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			styles.KeyHintStyle().Render(hintKeys),
			lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render(scrollInfo),
		),
	)
	hintHeight := strings.Count(hintRendered, "\n") + 1

	// Viewport windowing
	visibleCount := ph - 2 - hintHeight // title(1) + titleGap(1) + hintHeight
	if visibleCount < 1 {
		visibleCount = 1
	}

	offset := s.scroll
	if s.cursor >= offset+visibleCount {
		offset = s.cursor - visibleCount + 1
	}
	if s.cursor < offset {
		offset = s.cursor
	}
	if offset > len(items)-visibleCount {
		offset = len(items) - visibleCount
	}
	if offset < 0 {
		offset = 0
	}
	s.scroll = offset

	end := offset + visibleCount
	if end > len(items) {
		end = len(items)
	}

	var lines []string
	for i := offset; i < end; i++ {
		item := items[i]
		selected := i == s.cursor && focused
		bg := t.Base
		if selected {
			bg = t.Surface1
		}

		var line string
		switch item.kind {
		case itemSectionHeader:
			chevron := "▾"
			if s.collapsed[item.section] {
				chevron = "▸"
			}
			line = lipgloss.NewStyle().
				Foreground(t.Subtext0).Background(bg).Bold(true).
				Width(iw).
				Render(chevron + " " + item.label)

		case itemLocalBranch:
			prefix := "  "
			fg := t.Text
			if item.index < len(s.localBranches) && s.localBranches[item.index].IsCurrent {
				prefix = "✓ "
				fg = t.BranchCurrent()
			}
			if selected {
				prefix = "▸ "
			}
			text := prefix + truncate(item.label, iw-2)
			line = lipgloss.NewStyle().
				Foreground(fg).Background(bg).Width(iw).
				Render(text)

		case itemRemoteBranch:
			prefix := "  "
			if selected {
				prefix = "▸ "
			}
			text := prefix + truncate(item.label, iw-2)
			line = lipgloss.NewStyle().
				Foreground(t.BranchRemote()).Background(bg).Width(iw).
				Render(text)

		case itemTag:
			prefix := "  "
			if selected {
				prefix = "▸ "
			}
			text := prefix + truncate(item.label, iw-2)
			line = lipgloss.NewStyle().
				Foreground(t.TagColor()).Background(bg).Width(iw).
				Render(text)

		case itemStash:
			prefix := "  "
			if selected {
				prefix = "▸ "
			}
			text := prefix + truncate(item.label, iw-2)
			line = lipgloss.NewStyle().
				Foreground(t.Peach).Background(bg).Width(iw).
				Render(text)
		}

		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	// Title with panel shortcut indicator
	titleStr := styles.PanelTitle("Sidebar", "1", focused, iw)

	titleGap := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")

	// Pad content to exactly visibleCount lines so hints are pinned to the bottom
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > visibleCount {
		contentLines = contentLines[:visibleCount]
	}
	bgEmpty := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")
	for len(contentLines) < visibleCount {
		contentLines = append(contentLines, bgEmpty)
	}
	content = strings.Join(contentLines, "\n")

	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, hintRendered)
	// Clip to panel height so sidebar stays the same outer height as other panels.
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.ClipPanel(styles.PanelStyleColor(borderColor).Width(s.width).Height(ph).Render(full), s.height)
}

// Dimensions returns the current width/height.
func (s Sidebar) Dimensions() (int, int) {
	return s.width, s.height
}

// SetSize updates the sidebar dimensions.
func (s Sidebar) SetSize(w, h int) Sidebar {
	s.width = w
	s.height = h
	return s
}

// ---------------------------------------------------------------------------
// Commands (async git operations)
// ---------------------------------------------------------------------------

func (s Sidebar) loadData() tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		var msg sidebarLoadedMsg

		local, err := repo.Branches()
		if err != nil {
			msg.err = err
			return msg
		}
		msg.localBranches = local

		remote, err := repo.RemoteBranches()
		if err != nil {
			// Non-fatal — just empty remote list
			msg.remoteBranches = nil
		} else {
			msg.remoteBranches = remote
		}

		tags, err := repo.Tags()
		if err != nil {
			msg.tags = nil
		} else {
			msg.tags = tags
		}

		stashes, err := repo.StashList()
		if err != nil {
			msg.stashes = nil
		} else {
			msg.stashes = stashes
		}

		return msg
	}
}

func (s Sidebar) checkout(ref string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.Checkout(ref)
		return sidebarOpDoneMsg{action: "Checkout " + ref, err: err}
	}
}

func (s Sidebar) createBranch(name string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.CreateBranch(name)
		return sidebarOpDoneMsg{action: "Create branch " + name, err: err}
	}
}

func (s Sidebar) renameBranch(old, new_ string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.RenameBranch(old, new_)
		return sidebarOpDoneMsg{action: "Rename " + old + " → " + new_, err: err}
	}
}

func (s Sidebar) deleteBranch(name string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.DeleteBranch(name, false)
		return sidebarOpDoneMsg{action: "Delete branch " + name, err: err}
	}
}

func (s Sidebar) mergeBranch(name string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.Merge(name)
		return sidebarOpDoneMsg{action: "Merge " + name, err: err}
	}
}

func (s Sidebar) handleRebase() (Sidebar, tea.Cmd) {
	item := s.currentItem()
	if item == nil {
		return s, nil
	}
	var name string
	switch item.kind {
	case itemLocalBranch:
		if item.index < len(s.localBranches) {
			b := s.localBranches[item.index]
			if b.IsCurrent {
				return s, nil
			}
			name = b.Name
		}
	case itemRemoteBranch:
		if item.index < len(s.remoteBranches) {
			name = s.remoteBranches[item.index].Name
		}
	default:
		return s, nil
	}
	if name == "" {
		return s, nil
	}
	s.pendingRebaseBranch = name
	branchName := name
	return s, func() tea.Msg {
		return RequestConfirmMsg{
			ID:      "sidebar-rebase",
			Title:   "Rebase?",
			Message: "Rebase current branch onto " + branchName + "?\n\nThis rewrites commit history.",
		}
	}
}

func (s Sidebar) rebaseBranch(name string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.Rebase(name)
		return sidebarOpDoneMsg{action: "Rebase onto " + name, err: err}
	}
}

func (s Sidebar) stashSave(message string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.StashSave(message)
		return sidebarOpDoneMsg{action: "Stash save", err: err}
	}
}

func (s Sidebar) stashPop(index int) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.StashPop(index)
		return sidebarOpDoneMsg{action: "Stash pop", err: err}
	}
}

func (s Sidebar) stashApply(index int) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.StashApply(index)
		return sidebarOpDoneMsg{action: "Stash apply", err: err}
	}
}

func (s Sidebar) stashDrop(index int) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.StashDrop(index)
		return sidebarOpDoneMsg{action: fmt.Sprintf("Drop stash@{%d}", index), err: err}
	}
}

func (s Sidebar) createTag(name string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.CreateTag(name, "") // tag at HEAD
		return sidebarOpDoneMsg{action: "Tag created: " + name, err: err}
	}
}

func (s Sidebar) deleteTag(name string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.DeleteTag(name)
		return sidebarOpDoneMsg{action: "Tag deleted: " + name, err: err}
	}
}

// Refresh reloads sidebar data.
func (s Sidebar) Refresh() tea.Cmd {
	return s.loadData()
}

// HasStashes returns true if there are stash entries.
func (s Sidebar) HasStashes() bool {
	return len(s.stashes) > 0
}

// StashCount returns the number of stash entries.
func (s Sidebar) StashCount() int {
	return len(s.stashes)
}

// SetFilter sets a client-side filter string for sidebar items.
func (s Sidebar) SetFilter(query string) Sidebar {
	s.filter = strings.ToLower(query)
	s.cursor = 0 // reset cursor when filter changes
	return s
}

// ClearFilter removes the client-side filter.
func (s Sidebar) ClearFilter() Sidebar {
	s.filter = ""
	return s
}

// CurrentSectionName returns the name of the sidebar section under the cursor.
// Possible values: "local", "remote", "tags", "stash", or "" if nothing selected.
func (s Sidebar) CurrentSectionName() string {
	it := s.currentItem()
	if it == nil {
		return ""
	}
	switch it.section {
	case sectionLocal:
		return "local"
	case sectionRemote:
		return "remote"
	case sectionTags:
		return "tags"
	case sectionStash:
		return "stash"
	default:
		return ""
	}
}
