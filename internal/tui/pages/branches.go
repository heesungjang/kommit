package pages

import (
	"fmt"

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

type branchesLoadedMsg struct {
	branches []git.BranchInfo
	err      error
}

type branchOpDoneMsg struct {
	action string
	err    error
}

// ---------------------------------------------------------------------------
// BranchesPage model
// ---------------------------------------------------------------------------

// BranchesPage displays and manages local branches.
type BranchesPage struct {
	repo *git.Repository

	branches []git.BranchInfo
	cursor   int

	loading bool
	err     error

	navKeys    keys.NavigationKeys
	branchKeys keys.BranchKeys

	width  int
	height int
}

// NewBranchesPage creates a new branches page.
func NewBranchesPage(repo *git.Repository, width, height int) BranchesPage {
	return BranchesPage{
		repo:       repo,
		navKeys:    keys.NewNavigationKeys(),
		branchKeys: keys.NewBranchKeys(),
		width:      width,
		height:     height,
		loading:    true,
	}
}

// Init loads the branch list.
func (b BranchesPage) Init() tea.Cmd {
	return b.loadBranches()
}

// Update handles messages for the branches page.
func (b BranchesPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.width = msg.Width
		b.height = msg.Height
		return b, nil

	case branchesLoadedMsg:
		b.loading = false
		if msg.err != nil {
			b.err = msg.err
			return b, nil
		}
		b.branches = msg.branches
		b.clampCursor()
		return b, nil

	case branchOpDoneMsg:
		if msg.err != nil {
			b.err = msg.err
		}
		return b, b.loadBranches()

	case tea.KeyMsg:
		return b.handleKey(msg)
	}

	return b, nil
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (b BranchesPage) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, b.navKeys.Down):
		if b.cursor < len(b.branches)-1 {
			b.cursor++
		}
		return b, nil

	case key.Matches(msg, b.navKeys.Up):
		if b.cursor > 0 {
			b.cursor--
		}
		return b, nil

	case key.Matches(msg, b.branchKeys.Checkout):
		if len(b.branches) > 0 {
			return b, b.checkoutBranch(b.branches[b.cursor].Name)
		}

	case key.Matches(msg, b.branchKeys.New):
		// In a full implementation this would open an input dialog.
		// For now, this is a placeholder that emits a message.
		return b, nil

	case key.Matches(msg, b.branchKeys.Delete):
		if len(b.branches) > 0 && !b.branches[b.cursor].IsCurrent {
			return b, b.deleteBranch(b.branches[b.cursor].Name)
		}

	case key.Matches(msg, b.branchKeys.Merge):
		if len(b.branches) > 0 && !b.branches[b.cursor].IsCurrent {
			return b, b.mergeBranch(b.branches[b.cursor].Name)
		}
	}

	return b, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (b BranchesPage) View() string {
	if b.loading {
		return lipgloss.NewStyle().
			Width(b.width).Height(b.height).
			Padding(2, 4).
			Foreground(theme.Active.Subtext0).Background(theme.Active.Base).
			Render("Loading branches...")
	}
	if b.err != nil {
		return lipgloss.NewStyle().
			Width(b.width).Height(b.height).
			Padding(2, 4).
			Foreground(theme.Active.Red).Background(theme.Active.Base).
			Render(fmt.Sprintf("Error: %v", b.err))
	}

	t := theme.Active
	pw := b.width - styles.PanelBorderWidth     // Width() param for PanelStyle
	ph := b.height - styles.PanelBorderHeight   // Height() param for PanelStyle
	innerWidth := pw - styles.PanelPaddingWidth // content inside padding
	titleStr := styles.TitleStyle(true).Render(
		fmt.Sprintf("Branches (%d)", len(b.branches)),
	)

	var lines []string
	if len(b.branches) == 0 {
		lines = append(lines, styles.DimStyle().Width(innerWidth).Render("  No branches"))
	}

	for i, br := range b.branches {
		prefix := "  "
		nameColor := t.BranchLocal()
		if br.IsCurrent {
			prefix = "* "
			nameColor = t.BranchCurrent()
		}

		name := lipgloss.NewStyle().Foreground(nameColor).Background(t.Base).Bold(br.IsCurrent).Render(br.Name)
		hash := lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).Render(br.Hash)
		subject := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render(truncate(br.Subject, innerWidth-40))

		upstream := ""
		if br.Upstream != "" {
			upstream = lipgloss.NewStyle().Foreground(t.Mauve).Background(t.Base).Render(" -> " + br.Upstream)
		}

		line := fmt.Sprintf("%s%s %s %s%s", prefix, name, hash, subject, upstream)
		line = lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render(line)

		if i == b.cursor {
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
		"enter:checkout  n:new  d:delete  m:merge  r:rename",
	)
	emptyLine := lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render("")

	return styles.PanelStyle(true).Width(pw).Height(ph).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, content, emptyLine, hints),
	)
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (b BranchesPage) loadBranches() tea.Cmd {
	repo := b.repo
	return func() tea.Msg {
		branches, err := repo.Branches()
		return branchesLoadedMsg{branches: branches, err: err}
	}
}

func (b BranchesPage) checkoutBranch(name string) tea.Cmd {
	repo := b.repo
	return func() tea.Msg {
		err := repo.Checkout(name)
		return branchOpDoneMsg{action: "checkout", err: err}
	}
}

func (b BranchesPage) deleteBranch(name string) tea.Cmd {
	repo := b.repo
	return func() tea.Msg {
		err := repo.DeleteBranch(name, false)
		return branchOpDoneMsg{action: "delete", err: err}
	}
}

func (b BranchesPage) mergeBranch(name string) tea.Cmd {
	repo := b.repo
	return func() tea.Msg {
		err := repo.Merge(name)
		return branchOpDoneMsg{action: "merge", err: err}
	}
}

func (b *BranchesPage) clampCursor() {
	if b.cursor >= len(b.branches) {
		b.cursor = len(b.branches) - 1
	}
	if b.cursor < 0 {
		b.cursor = 0
	}
}

var _ tea.Model = BranchesPage{}
