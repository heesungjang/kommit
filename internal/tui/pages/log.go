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

type logLoadedMsg struct {
	commits []git.CommitInfo
	err     error
}

type commitDetailMsg struct {
	commit git.CommitInfo
	diff   string
	err    error
}

// ---------------------------------------------------------------------------
// Focus targets
// ---------------------------------------------------------------------------

type logFocus int

const (
	focusLogList logFocus = iota
	focusLogDetail
)

// ---------------------------------------------------------------------------
// LogPage model
// ---------------------------------------------------------------------------

// LogPage displays the commit history in a two-pane layout.
type LogPage struct {
	repo *git.Repository

	commits []git.CommitInfo
	cursor  int

	// Detail view
	detailCommit *git.CommitInfo
	detailDiff   string
	detailScroll int

	// Focus
	focus logFocus

	// State
	loading bool
	err     error

	// Keys
	navKeys keys.NavigationKeys

	// Dimensions
	width  int
	height int
}

// NewLogPage creates a new log page.
func NewLogPage(repo *git.Repository, width, height int) LogPage {
	return LogPage{
		repo:    repo,
		navKeys: keys.NewNavigationKeys(),
		width:   width,
		height:  height,
		loading: true,
	}
}

// Init loads the commit log.
func (l LogPage) Init() tea.Cmd {
	return l.loadLog()
}

// Update handles messages for the log page.
func (l LogPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height
		return l, nil

	case logLoadedMsg:
		l.loading = false
		if msg.err != nil {
			l.err = msg.err
			return l, nil
		}
		l.commits = msg.commits
		if len(l.commits) > 0 {
			return l, l.loadCommitDetail(l.commits[0])
		}
		return l, nil

	case commitDetailMsg:
		if msg.err != nil {
			l.detailDiff = fmt.Sprintf("Error: %v", msg.err)
		} else {
			l.detailCommit = &msg.commit
			l.detailDiff = msg.diff
		}
		l.detailScroll = 0
		return l, nil

	case tea.KeyMsg:
		return l.handleKey(msg)
	}

	return l, nil
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (l LogPage) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, key.NewBinding(key.WithKeys("tab"))) {
		if l.focus == focusLogList {
			l.focus = focusLogDetail
		} else {
			l.focus = focusLogList
		}
		return l, nil
	}

	switch l.focus {
	case focusLogList:
		return l.handleListKeys(msg)
	case focusLogDetail:
		return l.handleDetailKeys(msg)
	}
	return l, nil
}

func (l LogPage) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, l.navKeys.Down):
		if l.cursor < len(l.commits)-1 {
			l.cursor++
			return l, l.loadCommitDetail(l.commits[l.cursor])
		}
	case key.Matches(msg, l.navKeys.Up):
		if l.cursor > 0 {
			l.cursor--
			return l, l.loadCommitDetail(l.commits[l.cursor])
		}
	case key.Matches(msg, l.navKeys.PageDown):
		l.cursor += 10
		if l.cursor >= len(l.commits) {
			l.cursor = len(l.commits) - 1
		}
		if l.cursor >= 0 && l.cursor < len(l.commits) {
			return l, l.loadCommitDetail(l.commits[l.cursor])
		}
	case key.Matches(msg, l.navKeys.PageUp):
		l.cursor -= 10
		if l.cursor < 0 {
			l.cursor = 0
		}
		if len(l.commits) > 0 {
			return l, l.loadCommitDetail(l.commits[l.cursor])
		}
	}
	return l, nil
}

func (l LogPage) handleDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, l.navKeys.Down):
		l.detailScroll++
	case key.Matches(msg, l.navKeys.Up):
		if l.detailScroll > 0 {
			l.detailScroll--
		}
	case key.Matches(msg, l.navKeys.PageDown):
		l.detailScroll += 10
	case key.Matches(msg, l.navKeys.PageUp):
		l.detailScroll -= 10
		if l.detailScroll < 0 {
			l.detailScroll = 0
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
			Render("Loading log...")
	}
	if l.err != nil {
		return lipgloss.NewStyle().
			Width(l.width).Height(l.height).
			Padding(2, 4).
			Foreground(t.Red).Background(t.Base).
			Render(fmt.Sprintf("Error: %v", l.err))
	}

	bw := styles.PanelBorderWidth
	available := l.width - 2*bw
	leftWidth := available * 45 / 100
	if leftWidth < 38 {
		leftWidth = 38
	}
	if leftWidth > available-28 {
		leftWidth = available - 28
	}
	if leftWidth < 8 {
		leftWidth = 8
	}
	rightWidth := available - leftWidth

	leftPane := l.renderCommitList(leftWidth, l.height)
	rightPane := l.renderCommitDetail(rightWidth, l.height)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

func (l LogPage) renderCommitList(width, height int) string {
	t := theme.Active
	titleStr := styles.TitleStyle(l.focus == focusLogList).Render(
		fmt.Sprintf("Commits (%d)", len(l.commits)),
	)

	var lines []string
	if len(l.commits) == 0 {
		lines = append(lines, styles.DimStyle().Render("  No commits"))
	}

	innerWidth := width - styles.PanelPaddingWidth
	hashWidth := 8
	dateWidth := 11
	authorWidth := 15
	subjectWidth := innerWidth - hashWidth - dateWidth - authorWidth - 6
	if subjectWidth < 10 {
		subjectWidth = 10
	}

	for i, c := range l.commits {
		hash := lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).Width(hashWidth).Render(c.ShortHash)
		date := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Width(dateWidth).Render(c.Date.Format("2006-01-02"))
		author := lipgloss.NewStyle().Foreground(t.Teal).Background(t.Base).Width(authorWidth).Render(truncate(c.Author, authorWidth))
		subject := lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Width(subjectWidth).Render(truncate(c.Subject, subjectWidth))

		line := lipgloss.JoinHorizontal(lipgloss.Top, "  ", hash, " ", date, " ", author, " ", subject)
		line = lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render(line)

		if i == l.cursor && l.focus == focusLogList {
			line = lipgloss.NewStyle().
				Background(t.Surface1).
				Bold(true).
				Width(innerWidth).
				Render(line)
		}

		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	ph := height - styles.PanelBorderHeight
	return styles.PanelStyle(l.focus == focusLogList).Width(width).Height(ph).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
	)
}

func (l LogPage) renderCommitDetail(width, height int) string {
	focused := l.focus == focusLogDetail
	titleStr := styles.TitleStyle(focused).Render("Detail")
	ph := height - styles.PanelBorderHeight

	if l.detailCommit == nil {
		content := styles.DimStyle().Render("Select a commit to view details")
		return styles.PanelStyle(focused).Width(width).Height(ph).Render(
			lipgloss.JoinVertical(lipgloss.Left, titleStr, content),
		)
	}

	t := theme.Active
	c := l.detailCommit
	detailInnerWidth := width - styles.PanelPaddingWidth

	bgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).Width(detailInnerWidth).Render(s)
	}

	// Header
	var header []string
	header = append(header, bgLine(lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).Bold(true).Render("commit "+c.Hash)))
	header = append(header, bgLine(lipgloss.NewStyle().Foreground(t.Teal).Background(t.Base).Render("Author: "+c.Author+" <"+c.AuthorEmail+">")))
	header = append(header, bgLine(lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render("Date:   "+c.Date.Format("Mon Jan 2 15:04:05 2006 -0700"))))
	if len(c.Refs) > 0 {
		header = append(header, bgLine(lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).Render("Refs:   "+strings.Join(c.Refs, ", "))))
	}
	header = append(header, bgLine(""))
	header = append(header, bgLine(lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Render("    "+c.Subject)))
	if c.Body != "" {
		header = append(header, bgLine(""))
		for _, line := range strings.Split(c.Body, "\n") {
			header = append(header, bgLine(lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Render("    "+line)))
		}
	}
	header = append(header, bgLine(""))
	header = append(header, bgLine(lipgloss.NewStyle().Foreground(t.Surface2).Background(t.Base).Render(strings.Repeat("-", detailInnerWidth))))

	// Diff stat
	allLines := header
	if l.detailDiff != "" {
		allLines = append(allLines, strings.Split(l.detailDiff, "\n")...)
	}

	// Scroll — visible area accounts for border, title
	contentHeight := ph - 1 // title takes ~1 row
	if contentHeight < 1 {
		contentHeight = 1
	}
	maxScroll := len(allLines) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := l.detailScroll
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
		rendered = append(rendered, styles.DiffLineStyle(lineType).Width(detailInnerWidth).Render(line))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rendered...)
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
		return logLoadedMsg{commits: commits, err: err}
	}
}

func (l LogPage) loadCommitDetail(c git.CommitInfo) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		diff, err := repo.DiffCommit(c.Hash)
		diffStr := ""
		if err == nil && diff != nil {
			var sb strings.Builder
			for _, f := range diff.Files {
				sb.WriteString(fmt.Sprintf("--- %s\n+++ %s\n", f.OldPath, f.NewPath))
				for _, h := range f.Hunks {
					sb.WriteString(h.Header + "\n")
					for _, line := range h.Lines {
						sb.WriteString(line + "\n")
					}
				}
			}
			diffStr = sb.String()
		}
		return commitDetailMsg{commit: c, diff: diffStr, err: err}
	}
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

var _ tea.Model = LogPage{}
