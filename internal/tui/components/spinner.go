package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nicholascross/opengit/internal/tui/theme"
)

// Spinner is a simple loading spinner wrapper around bubbles spinner.
type Spinner struct {
	spinner spinner.Model
	label   string
	active  bool
}

// NewSpinner creates a new Spinner component.
func NewSpinner() Spinner {
	t := theme.Active
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(t.Blue)
	return Spinner{
		spinner: s,
		label:   "Loading...",
	}
}

// NewSpinnerWithStyle creates a spinner with a custom spinner type.
func NewSpinnerWithStyle(style spinner.Spinner) Spinner {
	t := theme.Active
	s := spinner.New()
	s.Spinner = style
	s.Style = lipgloss.NewStyle().Foreground(t.Blue)
	return Spinner{
		spinner: s,
		label:   "Loading...",
	}
}

// SetLabel sets the text label displayed next to the spinner.
func (sp Spinner) SetLabel(label string) Spinner {
	sp.label = label
	return sp
}

// Start activates the spinner.
func (sp Spinner) Start() Spinner {
	sp.active = true
	return sp
}

// Stop deactivates the spinner.
func (sp Spinner) Stop() Spinner {
	sp.active = false
	return sp
}

// Active returns whether the spinner is currently active.
func (sp Spinner) Active() bool {
	return sp.active
}

// Init implements tea.Model.
func (sp Spinner) Init() tea.Cmd {
	return sp.spinner.Tick
}

// Update implements tea.Model.
func (sp Spinner) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	if !sp.active {
		return sp, nil
	}

	var cmd tea.Cmd
	sp.spinner, cmd = sp.spinner.Update(msg)
	return sp, cmd
}

// View implements tea.Model.
func (sp Spinner) View() string {
	if !sp.active {
		return ""
	}

	t := theme.Active
	labelStyle := lipgloss.NewStyle().
		Foreground(t.Subtext0).
		Background(t.Base)

	spacer := lipgloss.NewStyle().Background(t.Base).Render(" ")
	return sp.spinner.View() + spacer + labelStyle.Render(sp.label)
}

// Tick returns the spinner's tick command for starting animation.
func (sp Spinner) Tick() tea.Cmd {
	return sp.spinner.Tick
}
