package dialog

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuictx "github.com/heesungjang/kommit/internal/tui/context"
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
	Base        Base
	textarea    textarea.Model
	stagedCount int
	amend       bool
}

// NewCommitMsg creates a new commit message dialog using a shared ProgramContext.
func NewCommitMsg(stagedCount int, ctx *tuictx.ProgramContext) CommitMsg {
	ta := textarea.New()
	ta.Placeholder = "Enter commit message..."
	ta.Focus()
	ta.CharLimit = 0 // unlimited
	ta.SetWidth(50)
	ta.SetHeight(6)
	ta.ShowLineNumbers = false

	return CommitMsg{
		Base:        NewBaseWithContext("Commit", "ctrl+s: commit  esc: cancel", 60, 20, ctx),
		textarea:    ta,
		stagedCount: stagedCount,
	}
}

// NewCommitMsgAmend creates a commit dialog in amend mode, pre-filled with the
// previous commit message.
func NewCommitMsgAmend(stagedCount int, prevMessage string, ctx *tuictx.ProgramContext) CommitMsg {
	ta := textarea.New()
	ta.Placeholder = "Enter commit message..."
	ta.Focus()
	ta.CharLimit = 0
	ta.SetWidth(50)
	ta.SetHeight(6)
	ta.ShowLineNumbers = false
	ta.SetValue(prevMessage)

	return CommitMsg{
		Base:        NewBaseWithContext("Amend Commit", "ctrl+s: commit  esc: cancel", 60, 20, ctx),
		textarea:    ta,
		stagedCount: stagedCount,
		amend:       true,
	}
}

func (c CommitMsg) Init() tea.Cmd {
	return textarea.Blink
}

func (c CommitMsg) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if kmsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(kmsg, key.NewBinding(key.WithKeys("esc"))):
			return c, func() tea.Msg { return CommitCancelMsg{} }

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("ctrl+s"))):
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
	return c.Base.Render(c.buildContentLines())
}

// buildContentLines produces the scrollable content lines for the commit
// dialog. Every entry is a single terminal line.
func (c CommitMsg) buildContentLines() []string {
	t := c.Base.ctx.Theme
	emptyLine := lipgloss.NewStyle().Background(t.Surface0).Render("")

	info := lipgloss.NewStyle().
		Foreground(t.Subtext0).
		Background(t.Surface0).
		Render(fmt.Sprintf("%d file(s) staged", c.stagedCount))

	lines := make([]string, 0, 8)
	lines = append(lines, info)
	lines = append(lines, emptyLine)
	lines = append(lines, FlattenLines(c.textarea.View())...)
	return lines
}

var _ tea.Model = CommitMsg{}
