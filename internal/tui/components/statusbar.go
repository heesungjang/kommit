package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nicholascross/opengit/internal/tui/styles"
	"github.com/nicholascross/opengit/internal/tui/theme"
)

// StatusBar displays branch info, ahead/behind counts, and help hints.
type StatusBar struct {
	branch  string
	ahead   int
	behind  int
	clean   bool
	repoDir string
	width   int
}

// NewStatusBar creates a new StatusBar component.
func NewStatusBar() StatusBar {
	return StatusBar{
		branch: "main",
		clean:  true,
		width:  80,
	}
}

// SetBranch sets the current branch name.
func (sb StatusBar) SetBranch(branch string) StatusBar {
	sb.branch = branch
	return sb
}

// SetAheadBehind sets the ahead/behind counts.
func (sb StatusBar) SetAheadBehind(ahead, behind int) StatusBar {
	sb.ahead = ahead
	sb.behind = behind
	return sb
}

// SetClean sets whether the working tree is clean.
func (sb StatusBar) SetClean(clean bool) StatusBar {
	sb.clean = clean
	return sb
}

// SetRepoDir sets the repository directory path.
func (sb StatusBar) SetRepoDir(dir string) StatusBar {
	sb.repoDir = dir
	return sb
}

// SetSize sets the width of the status bar.
func (sb StatusBar) SetSize(width int) StatusBar {
	sb.width = width
	return sb
}

// Branch returns the current branch name.
func (sb StatusBar) Branch() string {
	return sb.branch
}

// Init implements tea.Model.
func (sb StatusBar) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (sb StatusBar) Update(msg tea.Msg) (StatusBar, tea.Cmd) {
	return sb, nil
}

// View implements tea.Model.
func (sb StatusBar) View() string {
	t := theme.Active
	barStyle := styles.StatusBarStyle().Width(sb.width)

	bg := t.Surface0
	bgStyle := lipgloss.NewStyle().Background(bg)

	// Left side: branch info
	branchStr := lipgloss.NewStyle().
		Foreground(t.Green).
		Background(bg).
		Bold(true).
		Render(" " + sb.branch)

	// Ahead/behind
	var abParts []string
	if sb.ahead > 0 {
		abParts = append(abParts, lipgloss.NewStyle().Foreground(t.Green).Background(bg).Render(fmt.Sprintf("+%d", sb.ahead)))
	}
	if sb.behind > 0 {
		abParts = append(abParts, lipgloss.NewStyle().Foreground(t.Red).Background(bg).Render(fmt.Sprintf("-%d", sb.behind)))
	}
	abStr := ""
	if len(abParts) > 0 {
		sep := bgStyle.Render("/")
		abStr = bgStyle.Render(" ") + strings.Join(abParts, sep)
	}

	// Clean/dirty status
	var statusStr string
	if sb.clean {
		statusStr = lipgloss.NewStyle().Foreground(t.Green).Background(bg).Render(" clean")
	} else {
		statusStr = lipgloss.NewStyle().Foreground(t.Yellow).Background(bg).Render(" dirty")
	}

	leftContent := branchStr + abStr + statusStr

	// Add repo path if set
	if sb.repoDir != "" {
		leftContent += lipgloss.NewStyle().Foreground(t.Overlay0).Background(bg).Render("  " + sb.repoDir)
	}

	// Right side: help hint
	helpHint := styles.KeyStyle().Background(bg).Render("?") +
		styles.KeyHintStyle().Background(bg).Render(":help")

	// Calculate padding between left and right
	leftWidth := lipgloss.Width(leftContent)
	rightWidth := lipgloss.Width(helpHint)
	padding := sb.width - leftWidth - rightWidth - 2 // -2 for barStyle padding
	if padding < 1 {
		padding = 1
	}

	padStr := bgStyle.Render(strings.Repeat(" ", padding))
	content := leftContent + padStr + helpHint

	return barStyle.Render(content)
}
