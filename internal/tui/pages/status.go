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

type statusLoadedMsg struct {
	status *git.StatusResult
	err    error
}

type diffLoadedMsg struct {
	path   string
	staged bool
	diff   string
	err    error
}

type stageFileMsg struct {
	path string
	err  error
}

type unstageFileMsg struct {
	path string
	err  error
}

type stageAllMsg struct {
	err error
}

// ---------------------------------------------------------------------------
// Focus targets
// ---------------------------------------------------------------------------

type statusFocus int

const (
	focusUnstaged statusFocus = iota
	focusStaged
	focusDiff
)

// ---------------------------------------------------------------------------
// StatusPage model
// ---------------------------------------------------------------------------

// StatusPage is the main page showing unstaged/staged file lists and a diff
// viewer in a two-pane layout.
type StatusPage struct {
	repo *git.Repository

	// Data
	unstaged []git.FileStatus
	staged   []git.FileStatus

	// Cursor positions
	unstagedCursor int
	stagedCursor   int

	// Current diff
	diffContent string
	diffPath    string
	diffScroll  int

	// Focus
	focus statusFocus

	// Pending discard — stores path while waiting for confirmation
	pendingDiscardPath      string
	pendingDiscardUntracked bool

	// Loading / error state
	loading bool
	err     error

	// Keybindings
	navKeys    keys.NavigationKeys
	statusKeys keys.StatusKeys

	// Dimensions
	width  int
	height int
}

// NewStatusPage creates a new status page.
func NewStatusPage(repo *git.Repository, width, height int) StatusPage {
	return StatusPage{
		repo:       repo,
		navKeys:    keys.NewNavigationKeys(),
		statusKeys: keys.NewStatusKeys(),
		width:      width,
		height:     height,
		loading:    true,
	}
}

// Init loads the initial git status.
func (s StatusPage) Init() tea.Cmd {
	return s.loadStatus()
}

// Update handles messages for the status page.
func (s StatusPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case statusLoadedMsg:
		s.loading = false
		if msg.err != nil {
			s.err = msg.err
			return s, nil
		}
		s.unstaged = msg.status.UnstagedFiles()
		s.staged = msg.status.StagedFiles()
		s.clampCursors()
		return s, s.loadSelectedDiff()

	case diffLoadedMsg:
		if msg.err != nil {
			s.diffContent = fmt.Sprintf("Error loading diff: %v", msg.err)
		} else {
			s.diffContent = msg.diff
			s.diffPath = msg.path
		}
		s.diffScroll = 0
		return s, nil

	case stageFileMsg:
		if msg.err != nil {
			s.err = msg.err
		}
		return s, s.loadStatus()

	case unstageFileMsg:
		if msg.err != nil {
			s.err = msg.err
		}
		return s, s.loadStatus()

	case stageAllMsg:
		if msg.err != nil {
			s.err = msg.err
		}
		return s, s.loadStatus()

	// Handle confirm dialog results (e.g. discard confirmation).
	case dialog.ConfirmResultMsg:
		if msg.ID == "discard" && msg.Confirmed && s.pendingDiscardPath != "" {
			path := s.pendingDiscardPath
			untracked := s.pendingDiscardUntracked
			s.pendingDiscardPath = ""
			s.pendingDiscardUntracked = false
			if untracked {
				return s, s.cleanFile(path)
			}
			return s, s.discardFile(path)
		}
		// Not confirmed or wrong ID — just clear pending state.
		s.pendingDiscardPath = ""
		s.pendingDiscardUntracked = false
		return s, nil

	// Allow parent to tell us to refresh.
	case RefreshStatusMsg:
		return s, s.loadStatus()

	case tea.KeyMsg:
		return s.handleKey(msg)
	}

	return s, nil
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (s StatusPage) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Panel switching
	if key.Matches(msg, key.NewBinding(key.WithKeys("tab"))) {
		switch s.focus {
		case focusUnstaged:
			if len(s.staged) > 0 {
				s.focus = focusStaged
			} else {
				s.focus = focusDiff
			}
		case focusStaged:
			s.focus = focusDiff
		case focusDiff:
			s.focus = focusUnstaged
		}
		return s, s.loadSelectedDiff()
	}

	// Navigation within the focused panel
	switch s.focus {
	case focusUnstaged:
		return s.handleUnstagedKeys(msg)
	case focusStaged:
		return s.handleStagedKeys(msg)
	case focusDiff:
		return s.handleDiffKeys(msg)
	}

	return s, nil
}

func (s StatusPage) handleUnstagedKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, s.navKeys.Down):
		if s.unstagedCursor < len(s.unstaged)-1 {
			s.unstagedCursor++
			return s, s.loadSelectedDiff()
		}
	case key.Matches(msg, s.navKeys.Up):
		if s.unstagedCursor > 0 {
			s.unstagedCursor--
			return s, s.loadSelectedDiff()
		}
	case key.Matches(msg, s.statusKeys.Stage), key.Matches(msg, key.NewBinding(key.WithKeys(" "))):
		if len(s.unstaged) > 0 {
			return s, s.stageFile(s.unstaged[s.unstagedCursor].Path)
		}
	case key.Matches(msg, s.statusKeys.StageAll):
		return s, s.stageAllFiles()
	case key.Matches(msg, s.statusKeys.Commit):
		return s, s.requestCommitDialog()
	case key.Matches(msg, s.statusKeys.Refresh):
		return s, s.loadStatus()
	case key.Matches(msg, s.statusKeys.Discard):
		if len(s.unstaged) > 0 {
			f := s.unstaged[s.unstagedCursor]
			s.pendingDiscardPath = f.Path
			s.pendingDiscardUntracked = f.IsUntracked()

			action := "Discard changes"
			if f.IsUntracked() {
				action = "Delete untracked file"
			}
			confirmMsg := fmt.Sprintf("%s to '%s'?\n\nThis action cannot be undone.", action, f.Path)
			return s, func() tea.Msg {
				return RequestConfirmMsg{
					ID:      "discard",
					Title:   "Confirm " + action,
					Message: confirmMsg,
				}
			}
		}
	}
	return s, nil
}

func (s StatusPage) handleStagedKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, s.navKeys.Down):
		if s.stagedCursor < len(s.staged)-1 {
			s.stagedCursor++
			return s, s.loadSelectedDiff()
		}
	case key.Matches(msg, s.navKeys.Up):
		if s.stagedCursor > 0 {
			s.stagedCursor--
			return s, s.loadSelectedDiff()
		}
	case key.Matches(msg, s.statusKeys.Unstage), key.Matches(msg, key.NewBinding(key.WithKeys(" "))):
		if len(s.staged) > 0 {
			return s, s.unstageFile(s.staged[s.stagedCursor].Path)
		}
	case key.Matches(msg, s.statusKeys.Commit):
		return s, s.requestCommitDialog()
	case key.Matches(msg, s.statusKeys.Refresh):
		return s, s.loadStatus()
	}
	return s, nil
}

func (s StatusPage) handleDiffKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, s.navKeys.Down):
		s.diffScroll++
		return s, nil
	case key.Matches(msg, s.navKeys.Up):
		if s.diffScroll > 0 {
			s.diffScroll--
		}
		return s, nil
	case key.Matches(msg, s.navKeys.PageDown):
		s.diffScroll += 10
		return s, nil
	case key.Matches(msg, s.navKeys.PageUp):
		s.diffScroll -= 10
		if s.diffScroll < 0 {
			s.diffScroll = 0
		}
		return s, nil
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (s StatusPage) View() string {
	t := theme.Active

	if s.loading {
		return lipgloss.NewStyle().
			Width(s.width).
			Height(s.height).
			Padding(2, 4).
			Foreground(t.Subtext0).
			Background(t.Base).
			Render("Loading status...")
	}

	if s.err != nil {
		return lipgloss.NewStyle().
			Width(s.width).
			Height(s.height).
			Padding(2, 4).
			Foreground(t.Red).
			Background(t.Base).
			Render(fmt.Sprintf("Error: %v", s.err))
	}

	// Each bordered panel adds PanelBorderWidth to its outer width.
	// Two panels: leftOuter + rightOuter = s.width.
	// leftOuter = leftWidth + border, so leftWidth + rightWidth = s.width - 2*border.
	bw := styles.PanelBorderWidth
	available := s.width - 2*bw
	leftWidth := available * 35 / 100
	if leftWidth < 28 {
		leftWidth = 28
	}
	if leftWidth > available-28 {
		leftWidth = available - 28
	}
	if leftWidth < 8 {
		leftWidth = 8
	}
	rightWidth := available - leftWidth
	leftOuter := leftWidth + bw // outer width of left panel

	// Left pane: unstaged + staged file lists stacked
	unstagedView := s.renderFileList("Unstaged Changes", s.unstaged, s.unstagedCursor, s.focus == focusUnstaged, leftWidth)
	stagedView := s.renderFileList("Staged Changes", s.staged, s.stagedCursor, s.focus == focusStaged, leftWidth)

	unstagedHeight := s.height / 2
	stagedHeight := s.height - unstagedHeight

	unstagedView = lipgloss.NewStyle().
		Width(leftOuter).
		Height(unstagedHeight).MaxHeight(unstagedHeight).
		Background(t.Base).
		Render(unstagedView)
	stagedView = lipgloss.NewStyle().
		Width(leftOuter).
		Height(stagedHeight).MaxHeight(stagedHeight).
		Background(t.Base).
		Render(stagedView)

	leftPane := lipgloss.JoinVertical(lipgloss.Left, unstagedView, stagedView)

	// Right pane: diff viewer
	rightPane := s.renderDiff(rightWidth, s.height, s.focus == focusDiff)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

// renderFileList renders a titled list of files.
func (s StatusPage) renderFileList(title string, files []git.FileStatus, cursor int, focused bool, width int) string {
	t := theme.Active

	innerWidth := width - styles.PanelPaddingWidth // content inside padding
	titleStr := styles.TitleStyle(focused).MaxWidth(innerWidth).Render(title + fmt.Sprintf(" (%d)", len(files)))
	var lines []string
	if len(files) == 0 {
		lines = append(lines, styles.DimStyle().Width(innerWidth).Render("  (empty)"))
	}
	for i, f := range files {
		icon := f.StatusIcon()
		code := f.UnstagedCode
		if f.IsStaged() {
			code = f.StagedCode
		}
		color := styles.FileStatusColor(code)

		prefix := "  "
		style := lipgloss.NewStyle().Foreground(color).Background(t.Base)
		if i == cursor && focused {
			prefix = "> "
			style = style.Bold(true).Background(t.Surface1)
		}
		text := prefix + icon + " " + f.Path
		if len(text) > innerWidth {
			text = text[:innerWidth]
		}
		line := style.Width(innerWidth).Render(text)
		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	panel := styles.PanelStyle(focused).Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
	)
	return panel
}

// renderDiff renders the diff content in a scrollable panel.
// width is the lipgloss Width() param (includes padding, excludes border).
func (s StatusPage) renderDiff(width, height int, focused bool) string {
	innerWidth := width - styles.PanelPaddingWidth
	title := "Diff"
	if s.diffPath != "" {
		title = "Diff: " + s.diffPath
	}
	titleStr := styles.TitleStyle(focused).MaxWidth(innerWidth).Render(title)

	if s.diffContent == "" {
		content := styles.DimStyle().Render("Select a file to view its diff")
		return styles.PanelStyle(focused).Width(width).Height(height).Render(
			lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
		)
	}

	// Visible line count accounts for border+padding overhead in height.
	// Height() includes padding but excludes border, same as Width().
	// PanelStyle has Padding(0,1) = no vertical padding, so all height is content.
	// But border takes 2 rows. Title takes ~1 row.
	contentHeight := height - styles.PanelBorderHeight - 1
	if contentHeight < 1 {
		contentHeight = 1
	}

	allLines := strings.Split(s.diffContent, "\n")
	maxScroll := len(allLines) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if s.diffScroll > maxScroll {
		s.diffScroll = maxScroll
	}

	end := s.diffScroll + contentHeight
	if end > len(allLines) {
		end = len(allLines)
	}
	visibleLines := allLines[s.diffScroll:end]

	var rendered []string
	for _, line := range visibleLines {
		lineType := byte(' ')
		if len(line) > 0 {
			lineType = line[0]
		}
		// Expand tabs and truncate to display width
		line = expandTabs(line, 4)
		line = truncateToWidth(line, innerWidth)
		rendered = append(rendered, styles.DiffLineStyle(lineType).Width(innerWidth).Render(line))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rendered...)
	return styles.PanelStyle(focused).Width(width).Height(height - styles.PanelBorderHeight).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
	)
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (s StatusPage) loadStatus() tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		status, err := repo.Status()
		return statusLoadedMsg{status: status, err: err}
	}
}

func (s StatusPage) loadSelectedDiff() tea.Cmd {
	repo := s.repo
	var path string
	var staged bool
	var untracked bool

	switch s.focus {
	case focusUnstaged:
		if len(s.unstaged) > 0 && s.unstagedCursor < len(s.unstaged) {
			f := s.unstaged[s.unstagedCursor]
			path = f.Path
			staged = false
			untracked = f.IsUntracked()
		}
	case focusStaged:
		if len(s.staged) > 0 && s.stagedCursor < len(s.staged) {
			path = s.staged[s.stagedCursor].Path
			staged = true
		}
	default:
		return nil
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
		return diffLoadedMsg{path: path, staged: staged, diff: diff, err: err}
	}
}

func (s StatusPage) stageFile(path string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.StageFile(path)
		return stageFileMsg{path: path, err: err}
	}
}

func (s StatusPage) unstageFile(path string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.UnstageFile(path)
		return unstageFileMsg{path: path, err: err}
	}
}

func (s StatusPage) stageAllFiles() tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.StageAll()
		return stageAllMsg{err: err}
	}
}

func (s StatusPage) discardFile(path string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.DiscardFile(path)
		return stageFileMsg{path: path, err: err} // reuse msg to trigger reload
	}
}

func (s StatusPage) cleanFile(path string) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.CleanFile(path)
		return stageFileMsg{path: path, err: err} // reuse msg to trigger reload
	}
}

func (s StatusPage) requestCommitDialog() tea.Cmd {
	stagedCount := len(s.staged)
	return func() tea.Msg {
		return RequestCommitDialogMsg{StagedCount: stagedCount}
	}
}

// RequestCommitDialogMsg is emitted so the App shell can open the commit dialog.
type RequestCommitDialogMsg struct {
	StagedCount int
}

// RequestConfirmMsg is emitted so the App shell can open a confirm dialog.
type RequestConfirmMsg struct {
	ID      string
	Title   string
	Message string
}

// RefreshStatusMsg is sent by the app shell to tell the status page to reload.
type RefreshStatusMsg struct{}

// clampCursors ensures cursors are within valid bounds.
func (s *StatusPage) clampCursors() {
	if s.unstagedCursor >= len(s.unstaged) {
		s.unstagedCursor = len(s.unstaged) - 1
	}
	if s.unstagedCursor < 0 {
		s.unstagedCursor = 0
	}
	if s.stagedCursor >= len(s.staged) {
		s.stagedCursor = len(s.staged) - 1
	}
	if s.stagedCursor < 0 {
		s.stagedCursor = 0
	}
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

// Ensure interface compliance.
var _ tea.Model = StatusPage{}
