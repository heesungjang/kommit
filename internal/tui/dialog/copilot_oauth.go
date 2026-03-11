package dialog

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heesungjang/kommit/internal/ai"
	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// CopilotOAuthResultMsg is sent when the Copilot OAuth flow completes.
type CopilotOAuthResultMsg struct {
	GitHubToken  string // the GitHub OAuth token (for refresh)
	CopilotToken string // the Copilot bearer token (for API calls)
}

// CopilotOAuthErrorMsg is sent when the Copilot OAuth flow fails.
type CopilotOAuthErrorMsg struct {
	Err error
}

// CopilotOAuthCancelMsg is sent when the user cancels.
type CopilotOAuthCancelMsg struct{}

// copilotDeviceCodeMsg carries the device code response from GitHub.
type copilotDeviceCodeMsg struct {
	dc  *ai.DeviceCodeResponse
	err error
}

// copilotTokenMsg carries the result of polling for the GitHub token.
type copilotTokenMsg struct {
	githubToken  string
	copilotToken string
	err          error
}

// ---------------------------------------------------------------------------
// CopilotOAuth dialog
// ---------------------------------------------------------------------------

// CopilotOAuth shows the GitHub device code flow UI.
type CopilotOAuth struct {
	Base Base

	// State
	loading  bool                   // requesting device code
	waiting  bool                   // waiting for user to authorize
	dc       *ai.DeviceCodeResponse // device code response
	errMsg   string                 // error message if any
	cancelFn context.CancelFunc     // cancel polling
}

// NewCopilotOAuth creates a new Copilot OAuth dialog.
func NewCopilotOAuth(ctx *tuictx.ProgramContext) CopilotOAuth {
	return CopilotOAuth{
		Base:    NewBaseWithContext("GitHub Copilot Login", "esc: cancel", 52, 30, ctx),
		loading: true,
	}
}

func (c CopilotOAuth) Init() tea.Cmd {
	// Request the device code from GitHub.
	return func() tea.Msg {
		dc, err := ai.RequestDeviceCode(context.Background())
		return copilotDeviceCodeMsg{dc: dc, err: err}
	}
}

func (c CopilotOAuth) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case copilotDeviceCodeMsg:
		if msg.err != nil {
			c.loading = false
			c.errMsg = msg.err.Error()
			return c, nil
		}
		c.loading = false
		c.waiting = true
		c.dc = msg.dc

		// Start polling for the token in the background.
		pollCtx, cancel := context.WithCancel(context.Background())
		c.cancelFn = cancel
		dc := msg.dc
		return c, func() tea.Msg {
			githubToken, err := ai.PollForGitHubToken(pollCtx, dc)
			if err != nil {
				return copilotTokenMsg{err: err}
			}
			// Exchange for Copilot bearer token.
			copilotToken, err := ai.ExchangeForCopilotToken(pollCtx, githubToken)
			if err != nil {
				return copilotTokenMsg{err: fmt.Errorf("copilot token exchange: %w", err)}
			}
			return copilotTokenMsg{githubToken: githubToken, copilotToken: copilotToken.Token}
		}

	case copilotTokenMsg:
		c.waiting = false
		if msg.err != nil {
			c.errMsg = msg.err.Error()
			return c, nil
		}
		ghToken := msg.githubToken
		cpToken := msg.copilotToken
		return c, func() tea.Msg {
			return CopilotOAuthResultMsg{
				GitHubToken:  ghToken,
				CopilotToken: cpToken,
			}
		}

	case tea.KeyMsg:
		if key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q"))) {
			if c.cancelFn != nil {
				c.cancelFn()
			}
			return c, func() tea.Msg { return CopilotOAuthCancelMsg{} }
		}
	}
	return c, nil
}

func (c CopilotOAuth) View() string {
	return c.Base.Render(c.buildContentLines())
}

func (c CopilotOAuth) buildContentLines() []string {
	t := theme.Active
	w := c.Base.InnerWidth()

	blank := lipgloss.NewStyle().Background(t.Surface0).Width(w).Render("")

	if c.loading {
		msg := lipgloss.NewStyle().
			Foreground(t.Subtext0).Background(t.Surface0).Width(w).
			Render("  Requesting device code from GitHub...")
		return []string{blank, msg, blank}
	}

	if c.errMsg != "" {
		errLine := lipgloss.NewStyle().
			Foreground(t.Red).Background(t.Surface0).Width(w).
			Render("  Error: " + c.errMsg)
		hint := lipgloss.NewStyle().
			Foreground(t.Overlay0).Background(t.Surface0).Width(w).
			Render("  Press esc to close")
		return []string{blank, errLine, blank, hint, blank}
	}

	if c.waiting && c.dc != nil {
		lines := make([]string, 0, 12)
		lines = append(lines, blank)

		step1 := lipgloss.NewStyle().
			Foreground(t.Text).Background(t.Surface0).Width(w).
			Render("  1. Open this URL in your browser:")
		lines = append(lines, step1)
		lines = append(lines, blank)

		url := lipgloss.NewStyle().
			Foreground(t.Blue).Background(t.Surface0).Width(w).Bold(true).
			Render("     " + c.dc.VerificationURI)
		lines = append(lines, url)
		lines = append(lines, blank)

		step2 := lipgloss.NewStyle().
			Foreground(t.Text).Background(t.Surface0).Width(w).
			Render("  2. Enter this code:")
		lines = append(lines, step2)
		lines = append(lines, blank)

		code := lipgloss.NewStyle().
			Foreground(t.Green).Background(t.Surface0).Width(w).Bold(true).
			Render("     " + c.dc.UserCode)
		lines = append(lines, code)
		lines = append(lines, blank)

		waiting := lipgloss.NewStyle().
			Foreground(t.Overlay0).Background(t.Surface0).Width(w).
			Render("  Waiting for authorization...")
		lines = append(lines, waiting)
		lines = append(lines, blank)

		return lines
	}

	// Success state (brief, dialog closes quickly).
	done := lipgloss.NewStyle().
		Foreground(t.Green).Background(t.Surface0).Width(w).
		Render("  Authenticated with GitHub Copilot!")
	return []string{blank, done, blank}
}

var _ tea.Model = CopilotOAuth{}
