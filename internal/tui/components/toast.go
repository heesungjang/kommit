package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nicholascross/opengit/internal/tui/theme"
)

// ToastLevel indicates the severity of a toast notification.
type ToastLevel int

const (
	// ToastInfo is an informational toast (blue).
	ToastInfo ToastLevel = iota
	// ToastSuccess is a success toast (green).
	ToastSuccess
	// ToastError is an error toast (red).
	ToastError
)

// ToastDuration is how long a toast stays visible.
const ToastDuration = 3 * time.Second

// ToastDismissMsg is sent when a toast should be dismissed.
type ToastDismissMsg struct{}

// Toast displays brief notification messages that auto-dismiss.
type Toast struct {
	message string
	level   ToastLevel
	visible bool
	width   int
}

// NewToast creates a new Toast component.
func NewToast() Toast {
	return Toast{
		width: 40,
	}
}

// Show displays a toast message at the given level and returns a command
// that will dismiss it after ToastDuration.
func (t Toast) Show(msg string, level ToastLevel) (Toast, tea.Cmd) {
	t.message = msg
	t.level = level
	t.visible = true
	return t, t.dismissAfter(ToastDuration)
}

// ShowInfo is a convenience method to show an info toast.
func (t Toast) ShowInfo(msg string) (Toast, tea.Cmd) {
	return t.Show(msg, ToastInfo)
}

// ShowSuccess is a convenience method to show a success toast.
func (t Toast) ShowSuccess(msg string) (Toast, tea.Cmd) {
	return t.Show(msg, ToastSuccess)
}

// ShowError is a convenience method to show an error toast.
func (t Toast) ShowError(msg string) (Toast, tea.Cmd) {
	return t.Show(msg, ToastError)
}

// Dismiss hides the toast.
func (t Toast) Dismiss() Toast {
	t.visible = false
	t.message = ""
	return t
}

// Visible returns whether the toast is currently showing.
func (t Toast) Visible() bool {
	return t.visible
}

// SetWidth sets the maximum width for the toast.
func (t Toast) SetWidth(width int) Toast {
	t.width = width
	return t
}

// Init implements tea.Model.
func (t Toast) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (t Toast) Update(msg tea.Msg) (Toast, tea.Cmd) {
	switch msg.(type) {
	case ToastDismissMsg:
		t.visible = false
		t.message = ""
	}
	return t, nil
}

// View implements tea.Model.
func (t Toast) View() string {
	if !t.visible || t.message == "" {
		return ""
	}

	th := theme.Active

	var fg, bg lipgloss.Color
	switch t.level {
	case ToastSuccess:
		fg = th.Green
		bg = th.Surface0
	case ToastError:
		fg = th.Red
		bg = th.Surface0
	case ToastInfo:
		fg = th.Blue
		bg = th.Surface0
	default:
		fg = th.Text
		bg = th.Surface0
	}

	icon := toastIcon(t.level)

	toastStyle := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Bold(true).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(fg).
		MaxWidth(t.width)

	return toastStyle.Render(icon + " " + t.message)
}

// ViewAtPosition renders the toast positioned at the bottom-right corner
// within the given total width and height.
func (t Toast) ViewAtPosition(totalWidth, totalHeight int) string {
	if !t.visible || t.message == "" {
		return ""
	}

	rendered := t.View()
	renderedWidth := lipgloss.Width(rendered)

	// Right-align the toast
	padding := totalWidth - renderedWidth
	if padding < 0 {
		padding = 0
	}

	return lipgloss.NewStyle().
		PaddingLeft(padding).
		Render(rendered)
}

func toastIcon(level ToastLevel) string {
	switch level {
	case ToastSuccess:
		return "+"
	case ToastError:
		return "x"
	case ToastInfo:
		return "i"
	default:
		return "-"
	}
}

func (t Toast) dismissAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return ToastDismissMsg{}
	})
}

// Message returns the current toast message.
func (t Toast) Message() string {
	return t.message
}

// Level returns the current toast level.
func (t Toast) Level() ToastLevel {
	return t.level
}
