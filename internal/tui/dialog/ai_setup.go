package dialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// AISetupResultMsg is sent when the user completes the AI setup flow.
// The app shell should save the API key and update the config.
type AISetupResultMsg struct {
	Provider string // "anthropic", "openai", "openai-compatible"
	APIKey   string // the entered API key
}

// AISetupCancelMsg is sent when the user cancels the AI setup.
type AISetupCancelMsg struct{}

// ---------------------------------------------------------------------------
// AI Setup dialog (multi-step: provider → API key)
// ---------------------------------------------------------------------------

type aiSetupStep int

const (
	aiSetupStepProvider aiSetupStep = iota
	aiSetupStepKey
)

// AISetup is a two-step dialog for first-time AI configuration.
// Step 1: Choose a provider from a list.
// Step 2: Enter the API key.
type AISetup struct {
	Base Base
	ctx  *tuictx.ProgramContext

	step     aiSetupStep
	provider string // selected provider (set after step 1)

	// Step 1: provider picker
	providers []aiProviderOption
	cursor    int

	// Step 2: API key input
	keyInput textinput.Model
}

type aiProviderOption struct {
	Label       string
	Value       string
	Description string
}

var defaultProviders = []aiProviderOption{
	{
		Label:       "GitHub Copilot",
		Value:       "copilot",
		Description: "Free with GitHub — login with browser",
	},
	{
		Label:       "Anthropic (Claude)",
		Value:       "anthropic",
		Description: "Requires ANTHROPIC_API_KEY",
	},
	{
		Label:       "OpenAI (GPT)",
		Value:       "openai",
		Description: "Requires OPENAI_API_KEY",
	},
	{
		Label:       "OpenAI-Compatible",
		Value:       "openai-compatible",
		Description: "Ollama, Groq, DeepSeek, LM Studio...",
	},
}

// NewAISetup creates a new AI setup dialog.
func NewAISetup(ctx *tuictx.ProgramContext) AISetup {
	ti := textinput.New()
	ti.Placeholder = "sk-..."
	ti.CharLimit = 256
	ti.Width = 40
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'

	return AISetup{
		Base:      NewBaseWithContext("AI Setup", "j/k: navigate  enter: select  esc: cancel", 52, 30, ctx),
		ctx:       ctx,
		step:      aiSetupStepProvider,
		providers: defaultProviders,
		keyInput:  ti,
	}
}

func (a AISetup) Init() tea.Cmd { return nil }

func (a AISetup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch a.step {
	case aiSetupStepProvider:
		return a.updateProviderStep(msg)
	case aiSetupStepKey:
		return a.updateKeyStep(msg)
	}
	return a, nil
}

func (a AISetup) View() string {
	switch a.step {
	case aiSetupStepProvider:
		return a.Base.Render(a.buildProviderLines())
	case aiSetupStepKey:
		return a.Base.Render(a.buildKeyLines())
	}
	return ""
}

// ---------------------------------------------------------------------------
// Step 1: Provider selection
// ---------------------------------------------------------------------------

func (a AISetup) updateProviderStep(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		a.provider = a.providers[a.cursor].Value

		// Copilot uses a browser-based OAuth flow, not an API key.
		if a.provider == "copilot" {
			provider := a.provider
			return a, func() tea.Msg {
				return AISetupResultMsg{Provider: provider, APIKey: "__copilot_oauth__"}
			}
		}

		// OpenAI-compatible endpoints may not need an API key (e.g. local Ollama).
		// Still ask for one, but allow empty submission.
		a.step = aiSetupStepKey
		a.keyInput.Focus()

		// Update hint and title for step 2.
		a.Base.Title = "API Key"
		a.Base.HintText = "enter: save  esc: back"
		if a.provider == "openai-compatible" {
			a.keyInput.Placeholder = "API key (optional for local models)"
		}
		return a, textinput.Blink

	case key.Matches(kmsg, key.NewBinding(key.WithKeys("esc", "q"))):
		return a, func() tea.Msg { return AISetupCancelMsg{} }
	}

	return a, nil
}

func (a AISetup) buildProviderLines() []string {
	t := theme.Active
	w := a.Base.InnerWidth()

	lines := make([]string, 0, len(a.providers)*2+4)

	// Prompt text.
	prompt := lipgloss.NewStyle().
		Foreground(t.Subtext0).
		Background(t.Surface0).
		Width(w).
		Render("  Choose your AI provider:")
	lines = append(lines, prompt)

	// Blank line.
	blank := lipgloss.NewStyle().
		Background(t.Surface0).
		Width(w).
		Render("")
	lines = append(lines, blank)

	// Provider options.
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
		labelStyle := lipgloss.NewStyle().Foreground(fg).Background(bg)
		labelRendered := labelStyle.Render(label)

		// Pad to full width.
		labelW := lipgloss.Width(labelRendered)
		if labelW < w {
			pad := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", w-labelW))
			labelRendered += pad
		}
		lines = append(lines, labelRendered)

		// Description line (indented, dimmer).
		if p.Description != "" {
			desc := "    " + p.Description
			descStyle := lipgloss.NewStyle().
				Foreground(descFg).
				Background(bg).
				Width(w)
			lines = append(lines, descStyle.Render(desc))
		}
	}

	lines = append(lines, blank)
	return lines
}

// ---------------------------------------------------------------------------
// Step 2: API key entry
// ---------------------------------------------------------------------------

func (a AISetup) updateKeyStep(msg tea.Msg) (tea.Model, tea.Cmd) {
	if kmsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(kmsg, key.NewBinding(key.WithKeys("esc"))):
			// Go back to step 1.
			a.step = aiSetupStepProvider
			a.keyInput.SetValue("")
			a.keyInput.Blur()
			a.Base.Title = "AI Setup"
			a.Base.HintText = "j/k: navigate  enter: select  esc: cancel"
			return a, nil

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("enter"))):
			value := a.keyInput.Value()
			// For openai-compatible, allow empty key (local models).
			if value == "" && a.provider != "openai-compatible" {
				return a, nil
			}
			provider := a.provider
			return a, func() tea.Msg {
				return AISetupResultMsg{Provider: provider, APIKey: value}
			}
		}
	}

	var cmd tea.Cmd
	a.keyInput, cmd = a.keyInput.Update(msg)
	return a, cmd
}

func (a AISetup) buildKeyLines() []string {
	t := theme.Active
	w := a.Base.InnerWidth()

	var lines []string

	// Show which provider was selected.
	providerLabel := a.provider
	for _, p := range a.providers {
		if p.Value == a.provider {
			providerLabel = p.Label
			break
		}
	}
	providerLine := lipgloss.NewStyle().
		Foreground(t.Subtext0).
		Background(t.Surface0).
		Width(w).
		Render("  Provider: " + providerLabel)
	lines = append(lines, providerLine)

	// Blank line.
	blank := lipgloss.NewStyle().
		Background(t.Surface0).
		Width(w).
		Render("")
	lines = append(lines, blank)

	// Prompt.
	prompt := lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Surface0).
		Width(w).
		Render("  Enter your API key:")
	lines = append(lines, prompt)

	lines = append(lines, blank)

	// Text input.
	lines = append(lines, FlattenLines(a.keyInput.View())...)

	lines = append(lines, blank)

	// Help text.
	helpText := "  Key is saved to ~/.local/share/kommit/auth.json"
	help := lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Surface0).
		Width(w).
		Render(helpText)
	lines = append(lines, help)

	notInConfig := "  (never written to config files)"
	lines = append(lines, lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Surface0).
		Width(w).
		Render(notInConfig))

	lines = append(lines, blank)
	return lines
}

var _ tea.Model = AISetup{}
