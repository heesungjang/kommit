package dialog

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heesungjang/kommit/internal/auth"
	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/msgs"
	"github.com/heesungjang/kommit/internal/tui/utils"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// AccountLoginResultMsg is sent when login completes successfully.
type AccountLoginResultMsg struct {
	Account auth.Account
}

// AccountLoginCancelMsg is sent when the user cancels.
type AccountLoginCancelMsg struct{}

// DialogToastMsg is emitted by dialogs to request a toast notification.
type DialogToastMsg = msgs.ToastMsg

// accountDeviceCodeMsg carries the device code response.
type accountDeviceCodeMsg struct {
	dc  *auth.DeviceCodeResponse
	err error
}

// accountTokenMsg carries the OAuth token result.
type accountTokenMsg struct {
	token string
	err   error
}

// accountProfileMsg carries the fetched profile + saved account.
type accountProfileMsg struct {
	account auth.Account
	err     error
}

// ---------------------------------------------------------------------------
// AccountLogin dialog
// ---------------------------------------------------------------------------

type accountLoginStep int

const (
	accountStepProvider accountLoginStep = iota
	accountStepDeviceCode
	accountStepToken
	accountStepFetchingProfile
)

// AccountLogin is a multi-step dialog for logging into a git hosting provider.
//
// GitHub uses OAuth device flow by default (browser-based login). Users can
// press 't' during the device code step to switch to PAT input instead.
// All other providers use PAT/app password input.
type AccountLogin struct {
	Base Base
	ctx  *tuictx.ProgramContext

	step     accountLoginStep
	provider auth.HostProvider

	// Step 1: provider picker
	providers []accountProviderOption
	cursor    int

	// Step 2 (device flow): device code display
	loading  bool
	waiting  bool
	dc       *auth.DeviceCodeResponse
	cancelFn context.CancelFunc

	// Step 2 (PAT): token input
	tokenInput textinput.Model

	// Error/success
	errMsg string
}

type accountProviderOption struct {
	Label       string
	Value       auth.HostProvider
	Description string
	UseDevice   bool // true = OAuth device flow, false = PAT input
}

var defaultAccountProviders = []accountProviderOption{
	{
		Label:       "GitHub",
		Value:       auth.ProviderGitHub,
		Description: "Login with browser (OAuth)",
		UseDevice:   true,
	},
	{
		Label:       "GitLab",
		Value:       auth.ProviderGitLab,
		Description: "Personal Access Token (read_user, api)",
		UseDevice:   false,
	},
	{
		Label:       "Azure DevOps",
		Value:       auth.ProviderAzureDevOps,
		Description: "Personal Access Token (Code: Read & Write)",
		UseDevice:   false,
	},
	{
		Label:       "Bitbucket",
		Value:       auth.ProviderBitbucket,
		Description: "App Password (Repositories: Read & Write)",
		UseDevice:   false,
	},
}

func newTokenInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "paste token here..."
	ti.CharLimit = 512
	ti.Width = 40
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	return ti
}

// NewAccountLogin creates a new account login dialog.
func NewAccountLogin(ctx *tuictx.ProgramContext) AccountLogin {
	return AccountLogin{
		Base:       NewBaseWithContext("Account Login", "j/k: navigate  enter: select  esc: cancel", 54, 30, ctx),
		ctx:        ctx,
		step:       accountStepProvider,
		providers:  defaultAccountProviders,
		tokenInput: newTokenInput(),
	}
}

// NewAccountLoginForProvider creates a login dialog pre-set to a specific
// provider, skipping the provider picker step.
func NewAccountLoginForProvider(provider auth.HostProvider, ctx *tuictx.ProgramContext) AccountLogin {
	d := NewAccountLogin(ctx)
	d.provider = provider

	for _, p := range d.providers {
		if p.Value == provider {
			if p.UseDevice {
				d.step = accountStepDeviceCode
				d.loading = true
				d.Base.Title = p.Label + " Login"
				d.Base.HintText = "t: use token instead  esc: cancel"
			} else {
				d.step = accountStepToken
				d.tokenInput.Focus()
				d.Base.Title = providerDisplayName(provider) + " Login"
				d.Base.HintText = "enter: save  esc: cancel"
			}
			break
		}
	}
	return d
}

func (a AccountLogin) Init() tea.Cmd {
	if a.step == accountStepDeviceCode && a.loading {
		return a.requestDeviceCode()
	}
	if a.step == accountStepToken {
		return textinput.Blink
	}
	return nil
}

func (a AccountLogin) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch a.step {
	case accountStepProvider:
		return a.updateProviderStep(msg)
	case accountStepDeviceCode:
		return a.updateDeviceCodeStep(msg)
	case accountStepToken:
		return a.updateTokenStep(msg)
	case accountStepFetchingProfile:
		return a.updateFetchingStep(msg)
	}
	return a, nil
}

func (a AccountLogin) View() string {
	switch a.step {
	case accountStepProvider:
		return a.Base.Render(a.buildProviderLines())
	case accountStepDeviceCode:
		return a.Base.Render(a.buildDeviceCodeLines())
	case accountStepToken:
		return a.Base.Render(a.buildTokenLines())
	case accountStepFetchingProfile:
		return a.Base.Render(a.buildFetchingLines())
	}
	return ""
}

// ---------------------------------------------------------------------------
// Step 1: Provider selection
// ---------------------------------------------------------------------------

func (a AccountLogin) updateProviderStep(msg tea.Msg) (tea.Model, tea.Cmd) {
	kmsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}

	switch {
	case key.Matches(kmsg, key.NewBinding(key.WithKeys("up", "k"))):
		if a.cursor > 0 {
			a.cursor--
		}
		return a, nil

	case key.Matches(kmsg, key.NewBinding(key.WithKeys("down", "j"))):
		if a.cursor < len(a.providers)-1 {
			a.cursor++
		}
		return a, nil

	case key.Matches(kmsg, key.NewBinding(key.WithKeys("enter"))):
		selected := a.providers[a.cursor]
		a.provider = selected.Value

		if selected.UseDevice {
			a.step = accountStepDeviceCode
			a.loading = true
			a.Base.Title = selected.Label + " Login"
			a.Base.HintText = "t: use token instead  esc: cancel"
			return a, a.requestDeviceCode()
		}

		// PAT input.
		a.step = accountStepToken
		a.tokenInput.Focus()
		a.Base.Title = selected.Label + " Login"
		a.Base.HintText = "enter: save  esc: back"
		return a, textinput.Blink

	case key.Matches(kmsg, key.NewBinding(key.WithKeys("esc", "q"))):
		return a, func() tea.Msg { return AccountLoginCancelMsg{} }
	}

	return a, nil
}

func (a AccountLogin) buildProviderLines() []string {
	t := a.ctx.Theme
	w := a.Base.InnerWidth()

	lines := make([]string, 0, len(a.providers)*2+4)

	prompt := lipgloss.NewStyle().
		Foreground(t.Subtext0).Background(t.Surface0).Width(w).
		Render("  Choose a hosting provider:")
	lines = append(lines, prompt)

	blank := lipgloss.NewStyle().Background(t.Surface0).Width(w).Render("")
	lines = append(lines, blank)

	for i, p := range a.providers {
		selected := i == a.cursor
		bg := t.Surface0
		fg := t.Text
		descFg := t.Overlay0
		prefix := "  "
		if selected {
			bg = t.Blue
			fg = t.Base
			descFg = t.Base
			prefix = "> "
		}

		label := prefix + p.Label
		labelRendered := lipgloss.NewStyle().Foreground(fg).Background(bg).Render(label)
		labelW := lipgloss.Width(labelRendered)
		if labelW < w {
			pad := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", w-labelW))
			labelRendered += pad
		}
		lines = append(lines, labelRendered)

		if p.Description != "" {
			desc := "    " + p.Description
			lines = append(lines, lipgloss.NewStyle().
				Foreground(descFg).Background(bg).Width(w).
				Render(desc))
		}
	}

	lines = append(lines, blank)
	return lines
}

// ---------------------------------------------------------------------------
// Step 2a: Device code flow (GitHub)
// ---------------------------------------------------------------------------

func (a AccountLogin) requestDeviceCode() tea.Cmd {
	provider := a.provider
	return func() tea.Msg {
		var cfg auth.DeviceFlowConfig
		switch provider {
		case auth.ProviderGitHub:
			cfg = auth.GitHubDeviceLogin()
		default:
			return accountDeviceCodeMsg{err: fmt.Errorf("device flow not supported for %s", provider)}
		}
		dc, err := cfg.RequestDeviceCode(context.Background())
		return accountDeviceCodeMsg{dc: dc, err: err}
	}
}

// switchToTokenInput transitions from device flow to PAT input.
func (a AccountLogin) switchToTokenInput() (AccountLogin, tea.Cmd) {
	if a.cancelFn != nil {
		a.cancelFn()
		a.cancelFn = nil
	}
	a.step = accountStepToken
	a.loading = false
	a.waiting = false
	a.dc = nil
	a.errMsg = ""
	a.tokenInput = newTokenInput()
	a.tokenInput.Focus()
	a.Base.Title = providerDisplayName(a.provider) + " Login"
	a.Base.HintText = "enter: save  esc: back"
	return a, textinput.Blink
}

func (a AccountLogin) updateDeviceCodeStep(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case accountDeviceCodeMsg:
		if msg.err != nil {
			a.loading = false
			a.errMsg = msg.err.Error()
			return a, nil
		}
		a.loading = false
		a.waiting = true
		a.dc = msg.dc

		pollCtx, cancel := context.WithCancel(context.Background())
		a.cancelFn = cancel
		dc := msg.dc
		provider := a.provider

		pollCmd := func() tea.Msg {
			var cfg auth.DeviceFlowConfig
			switch provider {
			case auth.ProviderGitHub:
				cfg = auth.GitHubDeviceLogin()
			default:
				return accountTokenMsg{err: fmt.Errorf("unsupported provider: %s", provider)}
			}
			tok, err := cfg.PollForToken(pollCtx, dc)
			if err != nil {
				return accountTokenMsg{err: err}
			}
			return accountTokenMsg{token: tok.AccessToken}
		}

		return a, pollCmd

	case accountTokenMsg:
		a.waiting = false
		if msg.err != nil {
			a.errMsg = msg.err.Error()
			return a, nil
		}
		// Token received — fetch profile.
		a.step = accountStepFetchingProfile
		token := msg.token
		provider := a.provider
		return a, func() tea.Msg {
			return fetchAndSaveProfile(provider, token)
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("c"))):
			// Copy device code to clipboard.
			if a.dc != nil {
				code := a.dc.UserCode
				return a, func() tea.Msg {
					if err := utils.CopyToClipboard(code); err != nil {
						return DialogToastMsg{Message: "Failed to copy: " + err.Error(), IsError: true}
					}
					return DialogToastMsg{Message: "Copied " + code + " to clipboard"}
				}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("o"))):
			// Open verification URL in browser.
			if a.dc != nil {
				url := a.dc.VerificationURI
				return a, func() tea.Msg {
					if err := utils.OpenBrowser(url); err != nil {
						return DialogToastMsg{Message: "Failed to open browser: " + err.Error(), IsError: true}
					}
					return DialogToastMsg{Message: "Opened browser"}
				}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("t"))):
			// Switch to PAT input as fallback.
			updated, cmd := a.switchToTokenInput()
			return updated, cmd
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q"))):
			if a.cancelFn != nil {
				a.cancelFn()
			}
			return a, func() tea.Msg { return AccountLoginCancelMsg{} }
		}
	}
	return a, nil
}

func (a AccountLogin) buildDeviceCodeLines() []string {
	t := a.ctx.Theme
	w := a.Base.InnerWidth()
	blank := lipgloss.NewStyle().Background(t.Surface0).Width(w).Render("")

	if a.loading {
		msg := lipgloss.NewStyle().
			Foreground(t.Subtext0).Background(t.Surface0).Width(w).
			Render("  Requesting device code...")
		return []string{blank, msg, blank}
	}

	if a.errMsg != "" {
		lines := make([]string, 0, 6)
		lines = append(lines, blank)
		errLine := lipgloss.NewStyle().
			Foreground(t.Red).Background(t.Surface0).Width(w).
			Render("  Error: " + a.errMsg)
		lines = append(lines, errLine)
		lines = append(lines, blank)
		hint := lipgloss.NewStyle().
			Foreground(t.Overlay0).Background(t.Surface0).Width(w).
			Render("  Press t to use a token instead, esc to close")
		lines = append(lines, hint)
		lines = append(lines, blank)
		return lines
	}

	if a.waiting && a.dc != nil {
		lines := make([]string, 0, 12)
		lines = append(lines, blank)

		step1 := lipgloss.NewStyle().
			Foreground(t.Text).Background(t.Surface0).Width(w).
			Render("  1. Open this URL in your browser:")
		lines = append(lines, step1)
		lines = append(lines, blank)

		urlLine := lipgloss.NewStyle().
			Foreground(t.Blue).Background(t.Surface0).Width(w).Bold(true).
			Render("     " + a.dc.VerificationURI)
		lines = append(lines, urlLine)
		lines = append(lines, blank)

		step2 := lipgloss.NewStyle().
			Foreground(t.Text).Background(t.Surface0).Width(w).
			Render("  2. Enter this code:")
		lines = append(lines, step2)
		lines = append(lines, blank)

		codeText := lipgloss.NewStyle().
			Foreground(t.Green).Background(t.Surface0).Bold(true).
			Render("     " + a.dc.UserCode)
		copyHint := lipgloss.NewStyle().
			Foreground(t.Overlay0).Background(t.Surface0).
			Render("  [c] copy  [o] open")
		codeLine := lipgloss.NewStyle().
			Background(t.Surface0).Width(w).
			Render(codeText + copyHint)
		lines = append(lines, codeLine)
		lines = append(lines, blank)

		waiting := lipgloss.NewStyle().
			Foreground(t.Overlay0).Background(t.Surface0).Width(w).
			Render("  Waiting for authorization...")
		lines = append(lines, waiting)
		lines = append(lines, blank)

		return lines
	}

	done := lipgloss.NewStyle().
		Foreground(t.Green).Background(t.Surface0).Width(w).
		Render("  Authenticated!")
	return []string{blank, done, blank}
}

// ---------------------------------------------------------------------------
// Step 2b: PAT / App password input
// ---------------------------------------------------------------------------

func (a AccountLogin) updateTokenStep(msg tea.Msg) (tea.Model, tea.Cmd) {
	if kmsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(kmsg, key.NewBinding(key.WithKeys("esc"))):
			// Back to provider picker.
			a.step = accountStepProvider
			a.tokenInput.SetValue("")
			a.tokenInput.Blur()
			a.errMsg = ""
			a.Base.Title = "Account Login"
			a.Base.HintText = "j/k: navigate  enter: select  esc: cancel"
			return a, nil

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("enter"))):
			value := strings.TrimSpace(a.tokenInput.Value())
			if value == "" {
				return a, nil
			}
			a.step = accountStepFetchingProfile
			provider := a.provider
			return a, func() tea.Msg {
				return fetchAndSaveProfile(provider, value)
			}
		}
	}

	var cmd tea.Cmd
	a.tokenInput, cmd = a.tokenInput.Update(msg)
	return a, cmd
}

func (a AccountLogin) buildTokenLines() []string {
	t := a.ctx.Theme
	w := a.Base.InnerWidth()
	blank := lipgloss.NewStyle().Background(t.Surface0).Width(w).Render("")

	lines := make([]string, 0, 12)

	providerLine := lipgloss.NewStyle().
		Foreground(t.Subtext0).Background(t.Surface0).Width(w).
		Render("  Provider: " + providerDisplayName(a.provider))
	lines = append(lines, providerLine)
	lines = append(lines, blank)

	var prompt string
	switch a.provider {
	case auth.ProviderBitbucket:
		prompt = "  Enter your App Password:"
	default:
		prompt = "  Enter your Personal Access Token:"
	}
	lines = append(lines, lipgloss.NewStyle().
		Foreground(t.Text).Background(t.Surface0).Width(w).
		Render(prompt))
	lines = append(lines, blank)

	lines = append(lines, FlattenLines(a.tokenInput.View())...)
	lines = append(lines, blank)

	// Scope hints per provider.
	var scopeHints []string
	switch a.provider {
	case auth.ProviderGitHub:
		scopeHints = []string{
			"  Create at: github.com/settings/tokens",
			"  Required scopes: repo, read:user, workflow, read:org, gist",
		}
	case auth.ProviderGitLab:
		scopeHints = []string{
			"  Required scopes: read_user, api",
		}
	case auth.ProviderAzureDevOps:
		scopeHints = []string{
			"  Required scope: Code (Read & Write)",
		}
	case auth.ProviderBitbucket:
		scopeHints = []string{
			"  Required: Repositories (Read & Write)",
		}
	}
	for _, hint := range scopeHints {
		lines = append(lines, lipgloss.NewStyle().
			Foreground(t.Overlay0).Background(t.Surface0).Width(w).
			Render(hint))
	}

	lines = append(lines, lipgloss.NewStyle().
		Foreground(t.Overlay0).Background(t.Surface0).Width(w).
		Render("  Token saved to ~/.local/share/kommit/auth.json"))
	lines = append(lines, blank)

	return lines
}

// ---------------------------------------------------------------------------
// Step 3: Fetching profile + saving
// ---------------------------------------------------------------------------

func (a AccountLogin) updateFetchingStep(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case accountProfileMsg:
		if msg.err != nil {
			a.errMsg = msg.err.Error()
			// Go back to token step to let user retry.
			a.step = accountStepToken
			a.tokenInput = newTokenInput()
			a.tokenInput.Focus()
			a.Base.HintText = "enter: save  esc: back"
			return a, textinput.Blink
		}
		acct := msg.account
		return a, func() tea.Msg {
			return AccountLoginResultMsg{Account: acct}
		}

	case tea.KeyMsg:
		if key.Matches(msg, key.NewBinding(key.WithKeys("esc"))) {
			return a, func() tea.Msg { return AccountLoginCancelMsg{} }
		}
	}
	return a, nil
}

func (a AccountLogin) buildFetchingLines() []string {
	t := a.ctx.Theme
	w := a.Base.InnerWidth()
	blank := lipgloss.NewStyle().Background(t.Surface0).Width(w).Render("")

	if a.errMsg != "" {
		errLine := lipgloss.NewStyle().
			Foreground(t.Red).Background(t.Surface0).Width(w).
			Render("  Error: " + a.errMsg)
		return []string{blank, errLine, blank}
	}

	msg := lipgloss.NewStyle().
		Foreground(t.Subtext0).Background(t.Surface0).Width(w).
		Render("  Fetching profile...")
	return []string{blank, msg, blank}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// fetchAndSaveProfile fetches the user profile for a provider and saves
// the account. Returns an accountProfileMsg.
func fetchAndSaveProfile(provider auth.HostProvider, token string) accountProfileMsg {
	switch provider {
	case auth.ProviderGitHub:
		acct, err := auth.LoginGitHub(context.Background(), token)
		if err != nil {
			return accountProfileMsg{err: err}
		}
		return accountProfileMsg{account: *acct}

	case auth.ProviderGitLab:
		acct := auth.Account{
			Token:    token,
			Provider: auth.ProviderGitLab,
			GitUser:  auth.GitUserForProvider(auth.ProviderGitLab),
			Host:     "gitlab.com",
		}
		if err := auth.SaveAccount("gitlab.com", acct); err != nil {
			return accountProfileMsg{err: err}
		}
		return accountProfileMsg{account: acct}

	case auth.ProviderAzureDevOps:
		acct := auth.Account{
			Token:    token,
			Provider: auth.ProviderAzureDevOps,
			GitUser:  auth.GitUserForProvider(auth.ProviderAzureDevOps),
			Host:     "dev.azure.com",
		}
		if err := auth.SaveAccount("dev.azure.com", acct); err != nil {
			return accountProfileMsg{err: err}
		}
		return accountProfileMsg{account: acct}

	case auth.ProviderBitbucket:
		acct := auth.Account{
			Token:    token,
			Provider: auth.ProviderBitbucket,
			GitUser:  auth.GitUserForProvider(auth.ProviderBitbucket),
			Host:     "bitbucket.org",
		}
		if err := auth.SaveAccount("bitbucket.org", acct); err != nil {
			return accountProfileMsg{err: err}
		}
		return accountProfileMsg{account: acct}

	default:
		return accountProfileMsg{err: fmt.Errorf("unsupported provider: %s", provider)}
	}
}

func providerDisplayName(p auth.HostProvider) string {
	switch p {
	case auth.ProviderGitHub:
		return "GitHub"
	case auth.ProviderGitLab:
		return "GitLab"
	case auth.ProviderAzureDevOps:
		return "Azure DevOps"
	case auth.ProviderBitbucket:
		return "Bitbucket"
	default:
		return string(p)
	}
}

var _ tea.Model = AccountLogin{}
