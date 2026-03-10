package dialog

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nicholascross/opengit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// CommitRequestMsg is sent when the user submits the commit message.
type CommitRequestMsg struct {
	Message string
	Amend   bool
}

// CommitCancelMsg is sent when the user cancels the commit dialog.
type CommitCancelMsg struct{}

// ---------------------------------------------------------------------------
// CommitMsg model
// ---------------------------------------------------------------------------

// CommitMsg is a dialog for entering a commit message with a textarea.
type CommitMsg struct {
	textarea    textarea.Model
	stagedCount int
	amend       bool
	width       int
	height      int
}

// NewCommitMsg creates a new commit message dialog.
func NewCommitMsg(stagedCount, width, height int) CommitMsg {
	ta := textarea.New()
	ta.Placeholder = "Enter commit message..."
	ta.Focus()
	ta.CharLimit = 0 // unlimited
	ta.SetWidth(50)
	ta.SetHeight(6)
	ta.ShowLineNumbers = false

	return CommitMsg{
		textarea:    ta,
		stagedCount: stagedCount,
		width:       width,
		height:      height,
	}
}

// NewCommitMsgAmend creates a commit dialog in amend mode, pre-filled with the
// previous commit message.
func NewCommitMsgAmend(stagedCount int, prevMessage string, width, height int) CommitMsg {
	ta := textarea.New()
	ta.Placeholder = "Enter commit message..."
	ta.Focus()
	ta.CharLimit = 0
	ta.SetWidth(50)
	ta.SetHeight(6)
	ta.ShowLineNumbers = false
	ta.SetValue(prevMessage)

	return CommitMsg{
		textarea:    ta,
		stagedCount: stagedCount,
		amend:       true,
		width:       width,
		height:      height,
	}
}

func (c CommitMsg) Init() tea.Cmd {
	return textarea.Blink
}

func (c CommitMsg) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			return c, func() tea.Msg { return CommitCancelMsg{} }

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+s"))):
			message := c.textarea.Value()
			if message == "" {
				return c, nil
			}
			amend := c.amend
			return c, func() tea.Msg {
				return CommitRequestMsg{Message: message, Amend: amend}
			}
		}
	}

	var cmd tea.Cmd
	c.textarea, cmd = c.textarea.Update(msg)
	return c, cmd
}

func (c CommitMsg) View() string {
	t := theme.Active

	dialogWidth := 60
	if dialogWidth+2 > c.width-2 {
		dialogWidth = c.width - 4
	}
	if dialogWidth < 20 {
		dialogWidth = 20
	}

	titleText := "Commit"
	if c.amend {
		titleText = "Amend Commit"
	}
	title := lipgloss.NewStyle().
		Foreground(t.Blue).
		Background(t.Surface0).
		Bold(true).
		Render(titleText)

	info := lipgloss.NewStyle().
		Foreground(t.Subtext0).
		Background(t.Surface0).
		Render(fmt.Sprintf("%d file(s) staged", c.stagedCount))

	headerSep := lipgloss.NewStyle().Background(t.Surface0).Render("  ")
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, headerSep, info)

	ta := c.textarea.View()

	hints := lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Surface0).
		Padding(1, 0, 0, 0).
		Render("ctrl+s: commit  esc: cancel")

	emptyLine := lipgloss.NewStyle().Background(t.Surface0).Render("")
	content := lipgloss.JoinVertical(lipgloss.Left, header, emptyLine, ta, hints)

	return lipgloss.NewStyle().
		Width(dialogWidth).
		Padding(1, 2).
		Background(t.Surface0).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Blue).
		BorderBackground(t.Surface0).
		Render(content)
}

var _ tea.Model = CommitMsg{}
