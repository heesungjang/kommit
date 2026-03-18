package dialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heesungjang/kommit/internal/ai"
	"github.com/heesungjang/kommit/internal/auth"
	"github.com/heesungjang/kommit/internal/config"
	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// SettingsCloseMsg is sent when the user closes the settings dialog.
type SettingsCloseMsg struct{}

// SettingsChangeMsg is sent when a setting value has been changed.
// The app shell uses this to apply the change (e.g. theme hot-swap) and
// persist to config (unless Preview is true).
type SettingsChangeMsg struct {
	Key     string // setting key, e.g. "theme", "appearance.diffMode"
	Value   string // new value
	Preview bool   // if true, this is a live preview — do not persist to disk
}

// RequestAccountLoginMsg is sent when the user requests to log into
// a hosting provider from the settings dialog.
type RequestAccountLoginMsg struct {
	Provider auth.HostProvider
}

// RequestAccountLogoutMsg is sent when the user requests to log out of
// a hosting provider from the settings dialog.
type RequestAccountLogoutMsg struct {
	Host string
}

// ---------------------------------------------------------------------------
// Setting definition
// ---------------------------------------------------------------------------

// settingKind describes how a setting is edited.
type settingKind int

const (
	settingList   settingKind = iota // opens a list picker
	settingToggle                    // boolean toggle (enter flips it)
	settingAction                    // action button (enter triggers a message)
)

// settingDef describes a single setting row in the dialog.
type settingDef struct {
	Key            string              // config key, e.g. "theme"
	Label          string              // display label
	Section        string              // section header, e.g. "APPEARANCE"
	Kind           settingKind         // how to edit
	Options        []string            // for settingList: option values
	OptionLabel    func(string) string // optional: map value -> display label
	ActionProvider auth.HostProvider   // for settingAction: which hosting provider
}

// ---------------------------------------------------------------------------
// Settings model
// ---------------------------------------------------------------------------

// Settings is the main settings dialog. It shows a sectioned list of settings.
// Pressing enter on a list-type setting opens a sub-picker; pressing enter on
// a toggle flips the boolean value.
type Settings struct {
	Base    Base
	cfg     *config.Config
	ctx     *tuictx.ProgramContext
	defs    []settingDef
	cursor  int
	changed map[string]string // accumulated changes (key -> new value)

	// Sub-dialog state: when a list picker is open, we route input to it.
	subPicker    *ListPicker
	subPickerKey string // the setting key the sub-picker is editing
	preOpenValue string // value before the sub-picker opened (for cancel restore)

	// Accounts: cached account data for display.
	accounts map[string]auth.Account
}

// NewSettings creates a new settings dialog from the current config.
func NewSettings(cfg *config.Config, ctx *tuictx.ProgramContext) Settings {
	// Build the available theme names.
	themeNames := themeOptionNames()

	// Load accounts for the Accounts section.
	accounts, _ := auth.LoadAccounts()

	defs := []settingDef{
		{
			Key:     "theme",
			Label:   "Theme",
			Section: "APPEARANCE",
			Kind:    settingList,
			Options: themeNames,
		},
		{
			Key:     "appearance.diffMode",
			Label:   "Diff Mode",
			Section: "APPEARANCE",
			Kind:    settingList,
			Options: []string{"inline", "side-by-side"},
		},
		{
			Key:     "appearance.showGraph",
			Label:   "Show Graph",
			Section: "APPEARANCE",
			Kind:    settingToggle,
		},
		{
			Key:     "appearance.compactLog",
			Label:   "Compact Log",
			Section: "APPEARANCE",
			Kind:    settingToggle,
		},
		{
			Key:     "appearance.nerdFonts",
			Label:   "Nerd Font Icons",
			Section: "APPEARANCE",
			Kind:    settingToggle,
		},
		{
			Key:     "ai.provider",
			Label:   "AI Provider",
			Section: "AI",
			Kind:    settingList,
			Options: []string{"copilot", "anthropic", "openai", "openai-compatible"},
			OptionLabel: func(v string) string {
				switch v {
				case "copilot":
					return "GitHub Copilot"
				case "anthropic":
					return "Anthropic (Claude)"
				case "openai":
					return "OpenAI (GPT)"
				case "openai-compatible":
					return "OpenAI-Compatible"
				default:
					return v
				}
			},
		},
		{
			Key:     "ai.model",
			Label:   "AI Model",
			Section: "AI",
			Kind:    settingList,
			Options: aiModelOptions(cfg.AI.Provider),
			OptionLabel: func(v string) string {
				if v == "" {
					return "(default)"
				}
				return v
			},
		},
		// Accounts section — one row per hosting provider.
		{
			Key:            "account.github",
			Label:          "GitHub",
			Section:        "ACCOUNTS",
			Kind:           settingAction,
			ActionProvider: auth.ProviderGitHub,
		},
		{
			Key:            "account.gitlab",
			Label:          "GitLab",
			Section:        "ACCOUNTS",
			Kind:           settingAction,
			ActionProvider: auth.ProviderGitLab,
		},
		{
			Key:            "account.azure",
			Label:          "Azure DevOps",
			Section:        "ACCOUNTS",
			Kind:           settingAction,
			ActionProvider: auth.ProviderAzureDevOps,
		},
		{
			Key:            "account.bitbucket",
			Label:          "Bitbucket",
			Section:        "ACCOUNTS",
			Kind:           settingAction,
			ActionProvider: auth.ProviderBitbucket,
		},
	}

	return Settings{
		Base:     NewBaseWithContext("Settings", ",: settings  enter: change  esc: close", 48, 30, ctx),
		cfg:      cfg,
		ctx:      ctx,
		defs:     defs,
		cursor:   0,
		changed:  make(map[string]string),
		accounts: accounts,
	}
}

func (s Settings) Init() tea.Cmd { return nil }

func (s Settings) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If a sub-picker is open, route messages to it.
	if s.subPicker != nil {
		return s.updateSubPicker(msg)
	}

	if kmsg, ok := msg.(tea.KeyMsg); ok {
		totalLines := len(s.buildContentLines())
		if s.Base.HandleScrollKeys(kmsg, totalLines) {
			return s, nil
		}

		switch {
		case key.Matches(kmsg, key.NewBinding(key.WithKeys("up", "k"))):
			if s.cursor > 0 {
				s.cursor--
			}
			s.ensureCursorVisible()
			return s, nil

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("down", "j"))):
			if s.cursor < len(s.defs)-1 {
				s.cursor++
			}
			s.ensureCursorVisible()
			return s, nil

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("enter"))):
			return s.activateSetting()

		case key.Matches(kmsg, key.NewBinding(key.WithKeys("esc", "q", ","))):
			return s, func() tea.Msg { return SettingsCloseMsg{} }
		}
	}
	return s, nil
}

func (s Settings) View() string {
	// If a sub-picker is open, render it instead.
	if s.subPicker != nil {
		return s.subPicker.View()
	}
	return s.Base.Render(s.buildContentLines())
}

// ---------------------------------------------------------------------------
// Setting value accessors
// ---------------------------------------------------------------------------

// currentValue returns the display value for a setting, reflecting any
// accumulated changes.
func (s Settings) currentValue(def settingDef) string {
	// Account action rows have dynamic values.
	if def.Kind == settingAction {
		return s.accountDisplayValue(def)
	}
	// Check if there's an accumulated change.
	if v, ok := s.changed[def.Key]; ok {
		return s.formatValue(def, v)
	}
	// Read from config.
	return s.formatValue(def, s.rawValue(def))
}

// accountDisplayValue returns the display string for an account row.
func (s Settings) accountDisplayValue(def settingDef) string {
	host := hostForProvider(def.ActionProvider)
	acct, ok := s.accounts[host]
	if !ok || acct.Token == "" {
		return "Not connected"
	}
	if acct.Username != "" {
		return "@" + acct.Username
	}
	return "Connected"
}

// hostForProvider returns the default host for a hosting provider.
func hostForProvider(p auth.HostProvider) string {
	switch p {
	case auth.ProviderGitHub:
		return "github.com"
	case auth.ProviderGitLab:
		return "gitlab.com"
	case auth.ProviderAzureDevOps:
		return "dev.azure.com"
	case auth.ProviderBitbucket:
		return "bitbucket.org"
	default:
		return ""
	}
}

// rawValue reads the raw string value from config for a setting.
func (s Settings) rawValue(def settingDef) string {
	switch def.Key {
	case "theme":
		return s.cfg.Theme
	case "appearance.diffMode":
		return s.cfg.Appearance.DiffMode
	case "appearance.showGraph":
		if s.cfg.Appearance.ShowGraph {
			return "true"
		}
		return "false"
	case "appearance.compactLog":
		if s.cfg.Appearance.CompactLog {
			return "true"
		}
		return "false"
	case "appearance.nerdFonts":
		if s.cfg.Appearance.NerdFonts {
			return "true"
		}
		return "false"
	case "ai.provider":
		return s.cfg.AI.Provider
	case "ai.model":
		return s.cfg.AI.Model
	}
	return ""
}

// effectiveRawValue returns the raw value considering accumulated changes.
func (s Settings) effectiveRawValue(def settingDef) string {
	if v, ok := s.changed[def.Key]; ok {
		return v
	}
	return s.rawValue(def)
}

// formatValue formats a raw value for display.
func (s Settings) formatValue(def settingDef, raw string) string {
	if def.Kind == settingToggle {
		if raw == "true" {
			return "on"
		}
		return "off"
	}
	if def.OptionLabel != nil {
		return def.OptionLabel(raw)
	}
	if raw == "" {
		return "(not set)"
	}
	return raw
}

// ---------------------------------------------------------------------------
// Content rendering
// ---------------------------------------------------------------------------

func (s Settings) buildContentLines() []string {
	t := s.ctx.Theme
	w := s.Base.InnerWidth()

	lines := make([]string, 0, len(s.defs)*2)
	lastSection := ""
	for i, def := range s.defs {
		// Section header.
		if def.Section != lastSection {
			if lastSection != "" {
				// Blank line between sections.
				blank := lipgloss.NewStyle().
					Background(t.Surface0).
					Width(w).
					Render("")
				lines = append(lines, blank)
			}
			sectionStyle := lipgloss.NewStyle().
				Foreground(t.Overlay0).
				Background(t.Surface0).
				Bold(true).
				Width(w)
			lines = append(lines, sectionStyle.Render("  "+def.Section))
			lastSection = def.Section
		}

		selected := i == s.cursor
		bg := t.Surface0
		fg := t.Text
		valueFg := t.Subtext0
		prefix := "  "
		if selected {
			bg = t.Blue
			fg = t.Base
			valueFg = t.Base
			prefix = "> "
		}

		// Format: "  Label          value"
		label := prefix + def.Label
		value := s.currentValue(def)

		// For account action rows, show a colored status + action hint.
		if def.Kind == settingAction {
			lines = append(lines, s.buildAccountLine(def, label, value, w, selected, bg, fg))
			continue
		}

		// Truncate value if too long.
		maxValueW := w/2 - 2
		if maxValueW < 8 {
			maxValueW = 8
		}
		if len(value) > maxValueW {
			value = value[:maxValueW-1] + "…"
		}

		// Build the line with label left-aligned and value right-aligned.
		labelStyle := lipgloss.NewStyle().Foreground(fg).Background(bg)
		valueStyle := lipgloss.NewStyle().Foreground(valueFg).Background(bg)

		labelRendered := labelStyle.Render(label)
		valueRendered := valueStyle.Render(value)

		labelW := lipgloss.Width(labelRendered)
		valueW := lipgloss.Width(valueRendered)
		gap := w - labelW - valueW - 1
		if gap < 1 {
			gap = 1
		}
		gapStr := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", gap))
		trailing := lipgloss.NewStyle().Background(bg).Render(" ")

		line := labelRendered + gapStr + valueRendered + trailing

		// Ensure exact width.
		lineW := lipgloss.Width(line)
		if lineW < w {
			line += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", w-lineW))
		}

		lines = append(lines, line)
	}
	return lines
}

// ensureCursorVisible adjusts scroll so the cursor row is visible.
func (s *Settings) ensureCursorVisible() {
	// Compute the line index for the current cursor.
	// We need to count section headers and blank separators.
	lineIdx := 0
	lastSection := ""
	for i, def := range s.defs {
		if def.Section != lastSection {
			if lastSection != "" {
				lineIdx++ // blank line
			}
			lineIdx++ // section header
			lastSection = def.Section
		}
		if i == s.cursor {
			break
		}
		lineIdx++
	}

	totalLines := len(s.buildContentLines())
	s.Base.EnsureVisible(lineIdx, totalLines)
}

// ---------------------------------------------------------------------------
// Setting activation
// ---------------------------------------------------------------------------

// activateSetting handles pressing enter on the current setting.
func (s Settings) activateSetting() (tea.Model, tea.Cmd) {
	if s.cursor < 0 || s.cursor >= len(s.defs) {
		return s, nil
	}
	def := s.defs[s.cursor]

	switch def.Kind {
	case settingToggle:
		// Flip the boolean immediately.
		raw := s.effectiveRawValue(def)
		newVal := "true"
		if raw == "true" {
			newVal = "false"
		}
		s.changed[def.Key] = newVal
		settingKey := def.Key
		return s, func() tea.Msg {
			return SettingsChangeMsg{Key: settingKey, Value: newVal}
		}

	case settingAction:
		// Account action: Login or Logout.
		host := hostForProvider(def.ActionProvider)
		acct, connected := s.accounts[host]
		if connected && acct.Token != "" {
			// Logout: remove the account.
			hostStr := host
			return s, func() tea.Msg {
				return RequestAccountLogoutMsg{Host: hostStr}
			}
		}
		// Login: open the account login dialog.
		_ = acct
		provider := def.ActionProvider
		return s, func() tea.Msg {
			return RequestAccountLoginMsg{Provider: provider}
		}

	case settingList:
		// Open a sub-picker. Save the current value so we can restore on cancel.
		current := s.effectiveRawValue(def)
		s.preOpenValue = current

		// For ai.model, refresh options based on the current provider.
		options := def.Options
		if def.Key == "ai.model" {
			provider := s.effectiveRawValue(settingDef{Key: "ai.provider"})
			options = aiModelOptions(provider)
		}

		opts := make([]ListPickerOption, len(options))
		for i, v := range options {
			label := v
			if def.OptionLabel != nil {
				label = def.OptionLabel(v)
			}
			if label == "" {
				label = v
			}
			opts[i] = ListPickerOption{
				Label: label,
				Value: v,
			}
		}
		picker := NewListPicker(def.Key, def.Label, opts, current, s.ctx)
		s.subPicker = &picker
		s.subPickerKey = def.Key
		return s, nil
	}

	return s, nil
}

// updateSubPicker routes messages to the active sub-picker and handles results.
func (s Settings) updateSubPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle sub-picker result messages.
	switch msg := msg.(type) {
	case ListPickerResultMsg:
		s.changed[s.subPickerKey] = msg.Value
		settingKey := s.subPickerKey
		value := msg.Value

		// When the AI provider changes, auto-reset the model to the new
		// provider's default so the displayed value stays in sync.
		var modelResetCmd tea.Cmd
		if settingKey == "ai.provider" {
			newDefault := ai.DefaultModel(value)
			s.changed["ai.model"] = newDefault
			modelResetCmd = func() tea.Msg {
				return SettingsChangeMsg{Key: "ai.model", Value: newDefault}
			}
		}

		s.subPicker = nil
		s.subPickerKey = ""
		return s, tea.Batch(func() tea.Msg {
			return SettingsChangeMsg{Key: settingKey, Value: value}
		}, modelResetCmd)

	case ListPickerCancelMsg:
		s.subPicker = nil
		settingKey := s.subPickerKey
		originalVal := s.preOpenValue
		s.subPickerKey = ""
		s.preOpenValue = ""
		// Restore the original value on cancel (undo any preview changes).
		if originalVal != "" {
			return s, func() tea.Msg {
				return SettingsChangeMsg{Key: settingKey, Value: originalVal}
			}
		}
		return s, nil

	case ListPickerPreviewMsg:
		// Live preview for theme changes — marked as preview so the app
		// shell does not persist to disk.
		if s.subPickerKey == "theme" {
			value := msg.Value
			return s, func() tea.Msg {
				return SettingsChangeMsg{Key: "theme", Value: value, Preview: true}
			}
		}
		// For other settings, just update the sub-picker.
		return s, nil
	}

	// Route all other messages to the sub-picker.
	if s.subPicker != nil {
		updated, cmd := s.subPicker.Update(msg)
		if lp, ok := updated.(ListPicker); ok {
			s.subPicker = &lp
		}
		return s, cmd
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// Account line rendering
// ---------------------------------------------------------------------------

// buildAccountLine renders a single account row with status and action hint.
// Format: "  GitHub     @username  [Logout]"  or  "  GitHub     Not connected  [Login]"
func (s Settings) buildAccountLine(def settingDef, label, value string, w int, selected bool, bg, fg lipgloss.Color) string {
	t := s.ctx.Theme

	labelStyle := lipgloss.NewStyle().Foreground(fg).Background(bg)
	labelRendered := labelStyle.Render(label)

	// Determine connected state and action label.
	host := hostForProvider(def.ActionProvider)
	_, connected := s.accounts[host]
	if connected {
		acct := s.accounts[host]
		connected = acct.Token != ""
	}

	var statusFg lipgloss.Color
	actionLabel := "[Login]"
	if connected {
		actionLabel = "[Logout]"
		if selected {
			statusFg = t.Base
		} else {
			statusFg = t.Green
		}
	} else {
		if selected {
			statusFg = t.Base
		} else {
			statusFg = t.Overlay0
		}
	}

	statusRendered := lipgloss.NewStyle().Foreground(statusFg).Background(bg).Render(value)

	var actionFg lipgloss.Color
	if selected {
		actionFg = t.Base
	} else if connected {
		actionFg = t.Red
	} else {
		actionFg = t.Blue
	}
	actionRendered := lipgloss.NewStyle().Foreground(actionFg).Background(bg).Render(actionLabel)

	// Layout: label + gap + status + " " + action + trailing
	labelW := lipgloss.Width(labelRendered)
	statusW := lipgloss.Width(statusRendered)
	actionW := lipgloss.Width(actionRendered)

	gap := w - labelW - statusW - actionW - 3 // 2 spaces + 1 trailing
	if gap < 1 {
		gap = 1
	}
	gapStr := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", gap))
	spacer := lipgloss.NewStyle().Background(bg).Render("  ")
	trailing := lipgloss.NewStyle().Background(bg).Render(" ")

	line := labelRendered + gapStr + statusRendered + spacer + actionRendered + trailing

	lineW := lipgloss.Width(line)
	if lineW < w {
		line += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", w-lineW))
	}
	return line
}

// RefreshAccounts reloads accounts from disk. Called after login/logout.
func (s *Settings) RefreshAccounts() {
	accounts, _ := auth.LoadAccounts()
	s.accounts = accounts
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// aiModelOptions returns model choices based on the selected provider.
func aiModelOptions(provider string) []string {
	switch provider {
	case "anthropic":
		return []string{
			"claude-sonnet-4-6",
			"claude-opus-4-6",
			"claude-haiku-4-5",
			"claude-sonnet-4-5",
			"claude-opus-4-5",
			"claude-opus-4-1",
			"claude-sonnet-4-0",
			"claude-opus-4-0",
		}
	case "openai":
		return []string{
			"gpt-4o-mini",
			"gpt-4o",
			"gpt-5-nano",
			"gpt-5-mini",
			"gpt-5",
			"gpt-5.4",
			"gpt-4.1-nano",
			"gpt-4.1-mini",
			"gpt-4.1",
			"o4-mini",
			"o3-mini",
			"o3",
		}
	case "copilot":
		return []string{
			"gpt-4o",
			"gpt-4o-mini",
			"gpt-4.1",
			"claude-sonnet-4-6",
			"claude-3.5-sonnet",
			"o4-mini",
			"o3-mini",
		}
	default:
		// For openai-compatible, user typically sets model in config.
		return []string{""}
	}
}

// themeOptionNames returns the available theme names (sorted).
func themeOptionNames() []string {
	names := make([]string, 0, len(theme.Themes)+1)
	names = append(names, "auto")
	// Sort deterministically — catppuccin family first, then community themes.
	ordered := []string{
		"catppuccin-mocha",
		"catppuccin-latte",
		"catppuccin-frappe",
		"catppuccin-macchiato",
		"tokyo-night",
		"dracula",
		"nord",
		"gruvbox-dark",
		"rose-pine",
		"kanagawa-wave",
	}
	for _, name := range ordered {
		if _, ok := theme.Themes[name]; ok {
			names = append(names, name)
		}
	}
	return names
}

var _ tea.Model = Settings{}
