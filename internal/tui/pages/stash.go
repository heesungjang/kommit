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

type stashListLoadedMsg struct {
	entries []git.StashEntry
	err     error
}

type stashDiffLoadedMsg struct {
	index int
	diff  string
	err   error
}

type stashOpDoneMsg struct {
	action string
	err    error
}

// ---------------------------------------------------------------------------
// Focus targets
// ---------------------------------------------------------------------------

type stashFocus int

const (
	focusStashList stashFocus = iota
	focusStashDiff
)

// ---------------------------------------------------------------------------
// StashPage model
// ---------------------------------------------------------------------------

// StashPage displays stash entries and their diffs.
type StashPage struct {
	repo *git.Repository

	entries []git.StashEntry
	cursor  int

	// Diff panel
	diffContent string
	diffScroll  int

	focus stashFocus

	loading bool
	err     error

	navKeys   keys.NavigationKeys
	stashKeys keys.StashKeys

	width  int
	height int
}

// NewStashPage creates a new stash page.
func NewStashPage(repo *git.Repository, width, height int) StashPage {
	return StashPage{
		repo:      repo,
		navKeys:   keys.NewNavigationKeys(),
		stashKeys: keys.NewStashKeys(),
		width:     width,
		height:    height,
		loading:   true,
	}
}

// Init loads the stash list.
func (s StashPage) Init() tea.Cmd {
	return s.loadStashList()
}

// Update handles messages for the stash page.
func (s StashPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case stashListLoadedMsg:
		s.loading = false
		if msg.err != nil {
			s.err = msg.err
			return s, nil
		}
		s.entries = msg.entries
		s.clampCursor()
		if len(s.entries) > 0 {
			return s, s.loadStashDiff(s.entries[s.cursor].Index)
		}
		s.diffContent = ""
		return s, nil

	case stashDiffLoadedMsg:
		if msg.err != nil {
			s.diffContent = fmt.Sprintf("Error: %v", msg.err)
		} else {
			s.diffContent = msg.diff
		}
		s.diffScroll = 0
		return s, nil

	case stashOpDoneMsg:
		if msg.err != nil {
			s.err = msg.err
		}
		return s, s.loadStashList()

	case tea.KeyMsg:
		return s.handleKey(msg)
	}

	return s, nil
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (s StashPage) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, key.NewBinding(key.WithKeys("tab"))) {
		if s.focus == focusStashList {
			s.focus = focusStashDiff
		} else {
			s.focus = focusStashList
		}
		return s, nil
	}

	switch s.focus {
	case focusStashList:
		return s.handleListKeys(msg)
	case focusStashDiff:
		return s.handleDiffKeys(msg)
	}
	return s, nil
}

func (s StashPage) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, s.navKeys.Down):
		if s.cursor < len(s.entries)-1 {
			s.cursor++
			return s, s.loadStashDiff(s.entries[s.cursor].Index)
		}
	case key.Matches(msg, s.navKeys.Up):
		if s.cursor > 0 {
			s.cursor--
			return s, s.loadStashDiff(s.entries[s.cursor].Index)
		}
	case key.Matches(msg, s.stashKeys.Save):
		return s, s.saveStash()
	case key.Matches(msg, s.stashKeys.Pop):
		if len(s.entries) > 0 {
			return s, s.popStash(s.entries[s.cursor].Index)
		}
	case key.Matches(msg, s.stashKeys.Apply):
		if len(s.entries) > 0 {
			return s, s.applyStash(s.entries[s.cursor].Index)
		}
	case key.Matches(msg, s.stashKeys.Drop):
		if len(s.entries) > 0 {
			return s, s.dropStash(s.entries[s.cursor].Index)
		}
	}
	return s, nil
}

func (s StashPage) handleDiffKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, s.navKeys.Down):
		s.diffScroll++
	case key.Matches(msg, s.navKeys.Up):
		if s.diffScroll > 0 {
			s.diffScroll--
		}
	case key.Matches(msg, s.navKeys.PageDown):
		s.diffScroll += 10
	case key.Matches(msg, s.navKeys.PageUp):
		s.diffScroll -= 10
		if s.diffScroll < 0 {
			s.diffScroll = 0
		}
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (s StashPage) View() string {
	if s.loading {
		return lipgloss.NewStyle().
			Width(s.width).Height(s.height).
			Padding(2, 4).
			Foreground(theme.Active.Subtext0).Background(theme.Active.Base).
			Render("Loading stash...")
	}
	if s.err != nil {
		return lipgloss.NewStyle().
			Width(s.width).Height(s.height).
			Padding(2, 4).
			Foreground(theme.Active.Red).Background(theme.Active.Base).
			Render(fmt.Sprintf("Error: %v", s.err))
	}

	bw := styles.PanelBorderWidth
	available := s.width - 2*bw
	leftWidth := available * 40 / 100
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

	leftPane := s.renderStashList(leftWidth, s.height)
	rightPane := s.renderStashDiff(rightWidth, s.height)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

func (s StashPage) renderStashList(width, height int) string {
	t := theme.Active
	focused := s.focus == focusStashList
	innerWidth := width - styles.PanelPaddingWidth
	titleStr := styles.TitleStyle(focused).Render(
		fmt.Sprintf("Stash (%d)", len(s.entries)),
	)

	var lines []string
	if len(s.entries) == 0 {
		lines = append(lines, styles.DimStyle().Width(innerWidth).Render("  No stash entries"))
	}

	for i, entry := range s.entries {
		ref := lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).Render(entry.Ref)
		msg := lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Render(truncate(entry.Message, width-20))

		line := fmt.Sprintf("  %s %s", ref, msg)
		line = lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render(line)

		if i == s.cursor && focused {
			line = lipgloss.NewStyle().
				Background(t.Surface1).
				Bold(true).
				Width(innerWidth).
				Render(line)
		}

		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	hints := styles.KeyHintStyle().Background(t.Base).Width(innerWidth).Render(
		"s:save  p:pop  a:apply  D:drop",
	)
	emptyLine := lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render("")

	ph := height - styles.PanelBorderHeight
	return styles.PanelStyle(focused).Width(width).Height(ph).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, content, emptyLine, hints),
	)
}

func (s StashPage) renderStashDiff(width, height int) string {
	focused := s.focus == focusStashDiff
	innerWidth := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight
	titleStr := styles.TitleStyle(focused).Render("Stash Diff")

	if s.diffContent == "" {
		content := styles.DimStyle().Width(innerWidth).Render("Select a stash entry to view its diff")
		return styles.PanelStyle(focused).Width(width).Height(ph).Render(
			lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
		)
	}

	contentHeight := ph - 1
	if contentHeight < 1 {
		contentHeight = 1
	}
	allLines := strings.Split(s.diffContent, "\n")
	maxScroll := len(allLines) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := s.diffScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + contentHeight
	if end > len(allLines) {
		end = len(allLines)
	}
	visibleLines := allLines[scroll:end]

	var rendered []string
	for _, line := range visibleLines {
		lineType := byte(' ')
		if len(line) > 0 {
			lineType = line[0]
		}
		rendered = append(rendered, styles.DiffLineStyle(lineType).Width(innerWidth).Render(line))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rendered...)
	return styles.PanelStyle(focused).Width(width).Height(ph).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
	)
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (s StashPage) loadStashList() tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		entries, err := repo.StashList()
		return stashListLoadedMsg{entries: entries, err: err}
	}
}

func (s StashPage) loadStashDiff(index int) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		diff, err := repo.StashShow(index)
		return stashDiffLoadedMsg{index: index, diff: diff, err: err}
	}
}

func (s StashPage) saveStash() tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.StashSave("")
		return stashOpDoneMsg{action: "save", err: err}
	}
}

func (s StashPage) popStash(index int) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.StashPop(index)
		return stashOpDoneMsg{action: "pop", err: err}
	}
}

func (s StashPage) applyStash(index int) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.StashApply(index)
		return stashOpDoneMsg{action: "apply", err: err}
	}
}

func (s StashPage) dropStash(index int) tea.Cmd {
	repo := s.repo
	return func() tea.Msg {
		err := repo.StashDrop(index)
		return stashOpDoneMsg{action: "drop", err: err}
	}
}

func (s *StashPage) clampCursor() {
	if s.cursor >= len(s.entries) {
		s.cursor = len(s.entries) - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
}

var _ tea.Model = StashPage{}
