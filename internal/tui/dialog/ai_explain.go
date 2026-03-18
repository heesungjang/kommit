package dialog

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuictx "github.com/heesungjang/kommit/internal/tui/context"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// AIExplainUpdateMsg is forwarded by the app shell to the open AIExplain
// dialog when the AI response (or error) arrives.
type AIExplainUpdateMsg struct {
	Explanation string // non-empty on success
	Err         error  // non-nil on failure
}

// AIExplainCloseMsg is emitted when the user dismisses the dialog.
type AIExplainCloseMsg struct{}

// aiExplainTickMsg is the internal tick for the skeleton shimmer animation.
type aiExplainTickMsg struct{}

// ---------------------------------------------------------------------------
// Animation constants
// ---------------------------------------------------------------------------

const (
	explainTickInterval  = 120 * time.Millisecond
	explainShimmerRadius = 5
)

// ---------------------------------------------------------------------------
// AIExplain dialog
// ---------------------------------------------------------------------------

// AIExplain shows an AI-generated explanation of a commit's diff.
// It starts in a loading state with a skeleton shimmer animation and
// transitions to either a result or error state when AIExplainUpdateMsg
// is received.
type AIExplain struct {
	Base Base

	shortHash string // e.g. "abc1234"
	subject   string // commit subject line

	// State
	loading     bool
	errMsg      string
	result      string
	shimmerTick int // counter for skeleton shimmer animation
}

// NewAIExplain creates a new AI explanation dialog in loading state.
func NewAIExplain(shortHash, subject string, ctx *tuictx.ProgramContext) AIExplain {
	return AIExplain{
		Base:      NewBaseWithContext("AI Explanation — "+shortHash, "esc: close  j/k: scroll", 60, 30, ctx),
		shortHash: shortHash,
		subject:   subject,
		loading:   true,
	}
}

func (d AIExplain) Init() tea.Cmd {
	// Start the shimmer animation tick.
	return tea.Tick(explainTickInterval, func(time.Time) tea.Msg {
		return aiExplainTickMsg{}
	})
}

func (d AIExplain) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case aiExplainTickMsg:
		if d.loading {
			d.shimmerTick++
			// Schedule the next tick.
			return d, tea.Tick(explainTickInterval, func(time.Time) tea.Msg {
				return aiExplainTickMsg{}
			})
		}
		return d, nil

	case AIExplainUpdateMsg:
		d.loading = false
		if msg.Err != nil {
			d.errMsg = msg.Err.Error()
		} else {
			d.result = msg.Explanation
		}
		return d, nil

	case tea.KeyMsg:
		totalLines := len(d.buildContentLines())

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q"))):
			return d, func() tea.Msg { return AIExplainCloseMsg{} }
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			d.Base.ScrollDown(1, totalLines)
			return d, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			d.Base.ScrollUp(1)
			return d, nil
		}

		if d.Base.HandleScrollKeys(msg, totalLines) {
			return d, nil
		}
	}
	return d, nil
}

func (d AIExplain) View() string {
	return d.Base.Render(d.buildContentLines())
}

func (d AIExplain) buildContentLines() []string {
	t := d.Base.ctx.Theme
	w := d.Base.InnerWidth()

	blank := lipgloss.NewStyle().Background(t.Surface0).Width(w).Render("")

	// --- Loading state with skeleton shimmer ---
	if d.loading {
		var lines []string
		lines = append(lines, blank)

		// Commit subject
		subj := lipgloss.NewStyle().
			Foreground(t.Overlay0).Background(t.Surface0).Width(w).
			Render("  " + truncateStr(d.subject, w-4))
		lines = append(lines, subj)
		lines = append(lines, blank)

		// Separator
		sep := lipgloss.NewStyle().
			Foreground(t.Surface2).Background(t.Surface0).Width(w).
			Render("  " + strings.Repeat("─", w-4))
		lines = append(lines, sep)
		lines = append(lines, blank)

		// Skeleton shimmer bars — simulate paragraph lines
		barWidth := w - 4
		if barWidth < 10 {
			barWidth = 10
		}
		shimmerPos := explainShimmerPos(d.shimmerTick, barWidth)

		barLens := []int{
			barWidth * 9 / 10,
			barWidth * 7 / 10,
			barWidth * 8 / 10,
			barWidth * 5 / 10,
		}
		for i, blen := range barLens {
			bar := explainShimmerBar(blen, barWidth, shimmerPos, i*2, t.Surface2, t.Overlay1, t.Surface0)
			styled := lipgloss.NewStyle().Background(t.Surface0).Width(w).Render("  " + bar)
			lines = append(lines, styled)
		}

		lines = append(lines, blank)
		return lines
	}

	// --- Error state ---
	if d.errMsg != "" {
		errLine := lipgloss.NewStyle().
			Foreground(t.Red).Background(t.Surface0).Width(w).
			Render("  Error: " + d.errMsg)
		hint := lipgloss.NewStyle().
			Foreground(t.Overlay0).Background(t.Surface0).Width(w).
			Render("  Press esc to close")
		return []string{blank, errLine, blank, hint, blank}
	}

	// --- Result state ---
	var lines []string
	lines = append(lines, blank)

	// Commit subject as context header
	subj := lipgloss.NewStyle().
		Foreground(t.Blue).Background(t.Surface0).Width(w).Bold(true).
		Render("  " + truncateStr(d.subject, w-4))
	lines = append(lines, subj)
	lines = append(lines, blank)

	// Separator
	sep := lipgloss.NewStyle().
		Foreground(t.Surface2).Background(t.Surface0).Width(w).
		Render("  " + strings.Repeat("─", w-4))
	lines = append(lines, sep)
	lines = append(lines, blank)

	// Word-wrap the explanation text to fit the dialog width
	contentWidth := w - 4 // 2 chars padding on each side
	if contentWidth < 20 {
		contentWidth = 20
	}
	for _, paragraph := range strings.Split(d.result, "\n") {
		if paragraph == "" {
			lines = append(lines, blank)
			continue
		}
		for _, wl := range wrapLine(paragraph, contentWidth) {
			styled := lipgloss.NewStyle().
				Foreground(t.Text).Background(t.Surface0).Width(w).
				Render("  " + wl)
			lines = append(lines, styled)
		}
	}

	lines = append(lines, blank)
	return lines
}

// ---------------------------------------------------------------------------
// Shimmer animation helpers (same algorithm as wip_panel_anim.go)
// ---------------------------------------------------------------------------

// explainShimmerPos returns the shimmer center position using cosine easing.
func explainShimmerPos(tick, barWidth int) int {
	pad := explainShimmerRadius + 4
	totalRange := barWidth + 2*pad
	period := 20
	phase := tick % period
	t := float64(phase) / float64(period)
	eased := (1.0 - math.Cos(t*math.Pi)) / 2.0
	return int(eased*float64(totalRange)) - pad
}

// explainShimmerBar renders a skeleton bar with a cosine-gradient shimmer.
func explainShimmerBar(barLen, totalWidth, shimmerPos, lineOffset int, dimColor, brightColor, bg lipgloss.Color) string {
	center := shimmerPos - lineOffset
	dimHex := string(dimColor)
	brightHex := string(brightColor)

	var b strings.Builder
	for i := range barLen {
		dist := center - i
		if dist < 0 {
			dist = -dist
		}

		var fg lipgloss.Color
		if dist > explainShimmerRadius {
			fg = dimColor
		} else {
			ratio := (1.0 + math.Cos(float64(dist)*math.Pi/float64(explainShimmerRadius))) / 2.0
			fg = lipgloss.Color(explainBlendHex(dimHex, brightHex, ratio))
		}
		b.WriteString(lipgloss.NewStyle().Foreground(fg).Background(bg).Render("█"))
	}

	if pad := totalWidth - barLen; pad > 0 {
		b.WriteString(lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad)))
	}
	return b.String()
}

// explainBlendHex blends two "#RRGGBB" hex colors by ratio (0.0=a, 1.0=b).
func explainBlendHex(a, b string, ratio float64) string {
	ar, ag, ab := explainParseHex(a)
	br, bg, bb := explainParseHex(b)

	lerp := func(from, to int) int {
		v := float64(from) + (float64(to)-float64(from))*ratio
		return max(0, min(255, int(v)))
	}

	return fmt.Sprintf("#%02x%02x%02x", lerp(ar, br), lerp(ag, bg), lerp(ab, bb))
}

func explainParseHex(s string) (r, g, b int) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return 0, 0, 0
	}
	_, _ = fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

// ---------------------------------------------------------------------------
// Text helpers
// ---------------------------------------------------------------------------

// truncateStr truncates a string to maxLen, adding "..." if needed.
func truncateStr(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// wrapLine breaks a single line of text into multiple lines, each no wider
// than maxWidth. It splits on word boundaries when possible.
func wrapLine(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) <= maxWidth {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	lines = append(lines, current)
	return lines
}

var _ tea.Model = AIExplain{}
