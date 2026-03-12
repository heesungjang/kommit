package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// CreatePRSubmitMsg is sent when the user submits the create PR form.
type CreatePRSubmitMsg struct {
	Title string
	Body  string
	Head  string // source branch
	Base  string // target branch
	Draft bool
}

// CreatePRCancelMsg is sent when the user cancels PR creation.
type CreatePRCancelMsg struct{}

// CreatePRRequestAIMsg is sent to request AI generation of title and body.
type CreatePRRequestAIMsg struct {
	BaseBranch string
}

// CreatePRAIResultMsg carries the AI-generated title and body back.
type CreatePRAIResultMsg struct {
	Title string
	Body  string
}

// CreatePRAIErrorMsg is sent when AI generation fails.
type CreatePRAIErrorMsg struct {
	Err error
}

// ---------------------------------------------------------------------------
// CreatePR dialog
// ---------------------------------------------------------------------------

type createPRField int

const (
	prFieldTitle createPRField = iota
	prFieldBody
)

// CreatePR is a dialog for creating a new pull request.
type CreatePR struct {
	Base Base
	ctx  *tuictx.ProgramContext

	// Branch info
	headBranch    string // current branch (source)
	baseBranch    string // target branch (e.g. main)
	defaultBranch string // default branch from API

	// Fields
	titleInput textinput.Model
	bodyInput  textarea.Model
	field      createPRField
	editing    bool // true when actively typing in textarea

	// Options
	draft bool

	// AI generation state
	aiGenerating bool
	errMsg       string
}

// NewCreatePR creates a new Create PR dialog.
func NewCreatePR(headBranch, baseBranch string, ctx *tuictx.ProgramContext) CreatePR {
	t := theme.Active

	ti := textinput.New()
	ti.Placeholder = "PR title"
	ti.Focus()
	ti.CharLimit = 120
	ti.Width = 55
	ti.PromptStyle = lipgloss.NewStyle().Foreground(t.Blue).Background(t.Surface0)
	ti.TextStyle = lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface0)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Surface0)

	ta := textarea.New()
	ta.Placeholder = "PR description (optional, supports markdown)"
	ta.CharLimit = 4096
	ta.SetWidth(55)
	ta.SetHeight(6)
	ta.ShowLineNumbers = false

	// Make textarea fully opaque with Surface0 background.
	// Base style propagates to all sub-styles via inheritance.
	baseBg := lipgloss.NewStyle().Background(t.Surface0)
	ta.FocusedStyle.Base = baseBg
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(t.Text)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(t.Overlay0)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(t.Surface1)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(t.Blue)
	ta.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(t.Surface2)

	ta.BlurredStyle.Base = baseBg
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(t.Subtext0)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(t.Overlay0)
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(t.Surface2)
	ta.BlurredStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(t.Surface2)

	ta.Blur()

	hint := "tab:switch field  ctrl+g:AI generate  ctrl+d:toggle draft  enter:create  esc:cancel"

	return CreatePR{
		Base:          NewBaseWithContext("Create Pull Request", hint, 65, 40, ctx),
		ctx:           ctx,
		headBranch:    headBranch,
		baseBranch:    baseBranch,
		defaultBranch: baseBranch,
		titleInput:    ti,
		bodyInput:     ta,
		field:         prFieldTitle,
		editing:       false,
	}
}

func (d CreatePR) Init() tea.Cmd {
	return textinput.Blink
}

func (d CreatePR) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CreatePRAIResultMsg:
		d.aiGenerating = false
		d.errMsg = ""
		if msg.Title != "" {
			d.titleInput.SetValue(msg.Title)
		}
		if msg.Body != "" {
			d.bodyInput.SetValue(msg.Body)
		}
		return d, nil

	case CreatePRAIErrorMsg:
		d.aiGenerating = false
		d.errMsg = "AI error: " + msg.Err.Error()
		return d, nil

	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	return d, nil
}

func (d CreatePR) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys (always available)
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		if d.editing {
			// Exit textarea editing mode
			d.editing = false
			d.bodyInput.Blur()
			return d, nil
		}
		return d, func() tea.Msg { return CreatePRCancelMsg{} }

	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+g"))):
		if d.aiGenerating {
			return d, nil
		}
		d.aiGenerating = true
		d.errMsg = ""
		base := d.baseBranch
		return d, func() tea.Msg {
			return CreatePRRequestAIMsg{BaseBranch: base}
		}

	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+d"))):
		d.draft = !d.draft
		return d, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
		if d.field == prFieldTitle {
			d.field = prFieldBody
			d.titleInput.Blur()
			d.bodyInput.Focus()
			d.editing = true
			return d, nil
		}
		d.field = prFieldTitle
		d.editing = false
		d.bodyInput.Blur()
		d.titleInput.Focus()
		return d, textinput.Blink

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		// In title field: submit. In body field: insert newline.
		if d.field == prFieldTitle {
			title := strings.TrimSpace(d.titleInput.Value())
			if title == "" {
				d.errMsg = "Title is required"
				return d, nil
			}
			return d, func() tea.Msg {
				return CreatePRSubmitMsg{
					Title: title,
					Body:  strings.TrimSpace(d.bodyInput.Value()),
					Head:  d.headBranch,
					Base:  d.baseBranch,
					Draft: d.draft,
				}
			}
		}
	}

	// Route to focused field
	var cmd tea.Cmd
	switch d.field {
	case prFieldTitle:
		d.titleInput, cmd = d.titleInput.Update(msg)
	case prFieldBody:
		d.bodyInput, cmd = d.bodyInput.Update(msg)
	}
	return d, cmd
}

func (d CreatePR) View() string {
	return d.Base.Render(d.buildContentLines())
}

func (d CreatePR) buildContentLines() []string {
	th := theme.Active
	var lines []string

	labelStyle := lipgloss.NewStyle().Foreground(th.Subtext0).Background(th.Surface0)
	valStyle := lipgloss.NewStyle().Foreground(th.Text).Background(th.Surface0)
	branchStyle := lipgloss.NewStyle().Foreground(th.Sapphire).Background(th.Surface0).Bold(true)
	arrowStyle := lipgloss.NewStyle().Foreground(th.Overlay0).Background(th.Surface0)

	// Branch info
	branchLine := branchStyle.Render(d.headBranch) +
		arrowStyle.Render(" → ") +
		lipgloss.NewStyle().Foreground(th.Green).Background(th.Surface0).Bold(true).Render(d.baseBranch)
	lines = append(lines, branchLine)
	lines = append(lines, "")

	// Draft badge
	if d.draft {
		draftBadge := lipgloss.NewStyle().
			Foreground(th.Base).Background(th.Overlay0).
			Bold(true).Padding(0, 1).
			Render("DRAFT")
		lines = append(lines, draftBadge)
		lines = append(lines, "")
	}

	// Title field
	titleLabel := "Title"
	titleLabelFg := th.Subtext0
	if d.field == prFieldTitle {
		titleLabel = "▸ Title"
		titleLabelFg = th.Blue
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(titleLabelFg).Background(th.Surface0).Render(titleLabel))
	lines = append(lines, FlattenLines(d.titleInput.View())...)
	lines = append(lines, "")

	// Body field
	bodyLabel := "Description"
	bodyLabelFg := th.Subtext0
	if d.field == prFieldBody {
		bodyLabel = "▸ Description"
		bodyLabelFg = th.Blue
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(bodyLabelFg).Background(th.Surface0).Render(bodyLabel))
	lines = append(lines, FlattenLines(d.bodyInput.View())...)

	// AI generating indicator
	if d.aiGenerating {
		lines = append(lines, "")
		lines = append(lines,
			lipgloss.NewStyle().Foreground(th.Blue).Background(th.Surface0).
				Render("⟳ Generating AI description..."),
		)
	}

	// Error message
	if d.errMsg != "" {
		lines = append(lines, "")
		lines = append(lines,
			lipgloss.NewStyle().Foreground(th.Red).Background(th.Surface0).
				Render(d.errMsg),
		)
	}

	// Status line
	lines = append(lines, "")
	var statusParts []string
	statusParts = append(statusParts, valStyle.Render(fmt.Sprintf("%d chars", len(d.titleInput.Value()))))
	if d.draft {
		statusParts = append(statusParts, labelStyle.Render("draft"))
	}
	lines = append(lines, strings.Join(statusParts, labelStyle.Render(" · ")))

	return lines
}
