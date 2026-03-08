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

type remotesLoadedMsg struct {
	remotes []git.RemoteInfo
	err     error
}

type remoteOpDoneMsg struct {
	action string
	err    error
}

// ---------------------------------------------------------------------------
// RemotesPage model
// ---------------------------------------------------------------------------

// RemotesPage displays remotes and allows push/pull/fetch operations.
type RemotesPage struct {
	repo *git.Repository

	remotes []git.RemoteInfo
	cursor  int

	loading bool
	err     error

	navKeys keys.NavigationKeys

	width  int
	height int
}

// NewRemotesPage creates a new remotes page.
func NewRemotesPage(repo *git.Repository, width, height int) RemotesPage {
	return RemotesPage{
		repo:    repo,
		navKeys: keys.NewNavigationKeys(),
		width:   width,
		height:  height,
		loading: true,
	}
}

// Init loads the remote list.
func (r RemotesPage) Init() tea.Cmd {
	return r.loadRemotes()
}

// Update handles messages for the remotes page.
func (r RemotesPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height
		return r, nil

	case remotesLoadedMsg:
		r.loading = false
		if msg.err != nil {
			r.err = msg.err
			return r, nil
		}
		r.remotes = msg.remotes
		r.clampCursor()
		return r, nil

	case remoteOpDoneMsg:
		if msg.err != nil {
			r.err = msg.err
		}
		return r, r.loadRemotes()

	case tea.KeyMsg:
		return r.handleKey(msg)
	}

	return r, nil
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (r RemotesPage) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, r.navKeys.Down):
		if r.cursor < len(r.remotes)-1 {
			r.cursor++
		}
		return r, nil

	case key.Matches(msg, r.navKeys.Up):
		if r.cursor > 0 {
			r.cursor--
		}
		return r, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("p"))):
		// Push to selected remote
		if len(r.remotes) > 0 {
			return r, r.pushToRemote(r.remotes[r.cursor].Name)
		}

	case key.Matches(msg, key.NewBinding(key.WithKeys("P"))):
		// Pull from selected remote
		if len(r.remotes) > 0 {
			return r, r.pullFromRemote(r.remotes[r.cursor].Name)
		}

	case key.Matches(msg, key.NewBinding(key.WithKeys("f"))):
		// Fetch from selected remote
		if len(r.remotes) > 0 {
			return r, r.fetchRemote(r.remotes[r.cursor].Name)
		}

	case key.Matches(msg, key.NewBinding(key.WithKeys("F"))):
		// Fetch all
		return r, r.fetchAll()
	}

	return r, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (r RemotesPage) View() string {
	if r.loading {
		return lipgloss.NewStyle().
			Width(r.width).Height(r.height).
			Padding(2, 4).
			Foreground(theme.Active.Subtext0).Background(theme.Active.Base).
			Render("Loading remotes...")
	}
	if r.err != nil {
		return lipgloss.NewStyle().
			Width(r.width).Height(r.height).
			Padding(2, 4).
			Foreground(theme.Active.Red).Background(theme.Active.Base).
			Render(fmt.Sprintf("Error: %v", r.err))
	}

	t := theme.Active
	pw := r.width - styles.PanelBorderWidth
	ph := r.height - styles.PanelBorderHeight
	innerWidth := pw - styles.PanelPaddingWidth
	titleStr := styles.TitleStyle(true).Render(
		fmt.Sprintf("Remotes (%d)", len(r.remotes)),
	)

	var lines []string
	if len(r.remotes) == 0 {
		lines = append(lines, styles.DimStyle().Width(innerWidth).Render("  No remotes configured"))
	}

	for i, remote := range r.remotes {
		name := lipgloss.NewStyle().
			Foreground(t.Green).
			Background(t.Base).
			Bold(true).
			Width(15).
			Render(remote.Name)
		fetchURL := lipgloss.NewStyle().
			Foreground(t.Text).
			Background(t.Base).
			Render(remote.FetchURL)

		line := fmt.Sprintf("  %s %s", name, fetchURL)
		line = lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render(line)

		if remote.PushURL != "" && remote.PushURL != remote.FetchURL {
			pushLine := lipgloss.NewStyle().
				Foreground(t.Overlay0).
				Background(t.Base).
				Width(innerWidth).
				Render(fmt.Sprintf("  %s %s (push)", lipgloss.NewStyle().Width(15).Render(""), remote.PushURL))
			line = line + "\n" + pushLine
		}

		if i == r.cursor {
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
		"p:push  P:pull  f:fetch  F:fetch all",
	)
	emptyLine := lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render("")

	return styles.PanelStyle(true).Width(pw).Height(ph).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, content, emptyLine, hints),
	)
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (r RemotesPage) loadRemotes() tea.Cmd {
	repo := r.repo
	return func() tea.Msg {
		remotes, err := repo.RemoteList()
		return remotesLoadedMsg{remotes: remotes, err: err}
	}
}

func (r RemotesPage) pushToRemote(name string) tea.Cmd {
	repo := r.repo
	return func() tea.Msg {
		branch, _ := repo.Head()
		err := repo.Push(name, branch)
		return remoteOpDoneMsg{action: "push", err: err}
	}
}

func (r RemotesPage) pullFromRemote(name string) tea.Cmd {
	repo := r.repo
	return func() tea.Msg {
		branch, _ := repo.Head()
		err := repo.Pull(name, branch)
		return remoteOpDoneMsg{action: "pull", err: err}
	}
}

func (r RemotesPage) fetchRemote(name string) tea.Cmd {
	repo := r.repo
	return func() tea.Msg {
		err := repo.FetchRemote(name)
		return remoteOpDoneMsg{action: "fetch", err: err}
	}
}

func (r RemotesPage) fetchAll() tea.Cmd {
	repo := r.repo
	return func() tea.Msg {
		err := repo.Fetch()
		return remoteOpDoneMsg{action: "fetch all", err: err}
	}
}

func (r *RemotesPage) clampCursor() {
	if r.cursor >= len(r.remotes) {
		r.cursor = len(r.remotes) - 1
	}
	if r.cursor < 0 {
		r.cursor = 0
	}
}

var _ tea.Model = RemotesPage{}
