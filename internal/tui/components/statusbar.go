package components

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/heesungjang/kommit/internal/tui/styles"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ansiBgRe matches ANSI escape sequences that set a background color.
var ansiBgRe = regexp.MustCompile(
	`\x1b\[(?:4[0-7]|49|10[0-7]|48;5;\d+|48;2;\d+;\d+;\d+)m`,
)

// StatusBar displays branch info, ahead/behind counts, and help hints.
type StatusBar struct {
	branch     string
	ahead      int
	behind     int
	clean      bool
	repoDir    string
	width      int
	bisecting  bool
	rebasing   bool
	comparing  string // non-empty when comparing commits (shows base hash)
	focusLabel string // name of the focused panel (e.g. "Sidebar", "Commits")
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

// SetBisecting sets whether a bisect session is active.
func (sb StatusBar) SetBisecting(bisecting bool) StatusBar {
	sb.bisecting = bisecting
	return sb
}

// SetRebasing sets whether a rebase is in progress.
func (sb StatusBar) SetRebasing(rebasing bool) StatusBar {
	sb.rebasing = rebasing
	return sb
}

// SetComparing sets the compare base hash (empty to clear).
func (sb StatusBar) SetComparing(hash string) StatusBar {
	sb.comparing = hash
	return sb
}

// SetFocusLabel sets the label for the currently focused panel.
func (sb StatusBar) SetFocusLabel(label string) StatusBar {
	sb.focusLabel = label
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

// IsBisecting returns whether a bisect session is active.
func (sb StatusBar) IsBisecting() bool {
	return sb.bisecting
}

// IsComparing returns whether compare mode is active.
func (sb StatusBar) IsComparing() bool {
	return sb.comparing != ""
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

	bg := t.Surface0

	// IMPORTANT: Do NOT set .Background(bg) on individual segment styles.
	// Lipgloss combines fg+bg into compound SGR sequences (e.g.
	// \x1b[1;38;2;R;G;B;48;2;R;G;Bm) which the ansiBgRe regex cannot
	// strip. Instead, only set .Foreground() here and let the ANSI
	// patching block below uniformly apply the background.

	// Left side: branch info
	branchStr := lipgloss.NewStyle().
		Foreground(t.Green).
		Bold(true).
		Render(" " + sb.branch)

	// Ahead/behind
	var abParts []string
	if sb.ahead > 0 {
		abParts = append(abParts, lipgloss.NewStyle().Foreground(t.Green).Render(fmt.Sprintf("+%d", sb.ahead)))
	}
	if sb.behind > 0 {
		abParts = append(abParts, lipgloss.NewStyle().Foreground(t.Red).Render(fmt.Sprintf("-%d", sb.behind)))
	}
	abStr := ""
	if len(abParts) > 0 {
		abStr = " " + strings.Join(abParts, "/")
	}

	// Clean/dirty status
	var statusStr string
	if sb.clean {
		statusStr = lipgloss.NewStyle().Foreground(t.Green).Render(" clean")
	} else {
		statusStr = lipgloss.NewStyle().Foreground(t.Yellow).Render(" dirty")
	}

	leftContent := branchStr + abStr + statusStr

	// State indicators
	if sb.bisecting {
		leftContent += lipgloss.NewStyle().Foreground(t.Red).Bold(true).Render("  BISECTING")
	}
	if sb.rebasing {
		leftContent += lipgloss.NewStyle().Foreground(t.Peach).Bold(true).Render("  REBASING")
	}
	if sb.comparing != "" {
		leftContent += lipgloss.NewStyle().Foreground(t.Mauve).Bold(true).Render("  COMPARING:" + sb.comparing)
	}

	// Add repo path if set
	if sb.repoDir != "" {
		leftContent += lipgloss.NewStyle().Foreground(t.Overlay0).Render("  " + sb.repoDir)
	}

	// Right side: focus label + help hint
	rightParts := ""
	if sb.focusLabel != "" {
		rightParts += lipgloss.NewStyle().Foreground(t.Blue).Bold(true).Render(sb.focusLabel) + "  "
	}
	rightParts += styles.KeyStyle().Render("?") + styles.KeyHintStyle().Render(":help")
	helpHint := rightParts

	// Calculate padding between left and right
	leftWidth := lipgloss.Width(leftContent)
	rightWidth := lipgloss.Width(helpHint)
	padding := sb.width - leftWidth - rightWidth - 2 // -2 for left/right margin spaces
	if padding < 1 {
		padding = 1
	}

	padStr := strings.Repeat(" ", padding)
	content := " " + leftContent + padStr + helpHint + " "

	// Force background on every cell using ANSI patching.
	// Strip all foreign bg codes, then re-insert our bg after every reset.
	bgSeq := hexToBgSeq(string(bg))
	if bgSeq != "" {
		reset := "\x1b[0m"
		// Strip any foreign background sequences
		content = ansiBgRe.ReplaceAllString(content, "")
		// Re-insert our bg after every reset
		content = strings.ReplaceAll(content, reset, reset+bgSeq)
		// Ensure full line is covered
		w := lipgloss.Width(content)
		if w < sb.width {
			content += strings.Repeat(" ", sb.width-w)
		}
		content = bgSeq + content + reset
	}

	return content
}

// hexToBgSeq converts a hex color string (e.g. "#313244") to an ANSI 24-bit
// background escape sequence.
func hexToBgSeq(hex string) string {
	if hex != "" && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return ""
	}
	var r, g, b int
	_, _ = fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}
