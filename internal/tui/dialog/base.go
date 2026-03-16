package dialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/ansi"

	tuictx "github.com/heesungjang/kommit/internal/tui/context"
)

// borderOverhead is the vertical space consumed by the lipgloss rounded border
// (top + bottom = 2) plus vertical Padding(1, 2) (top=1 + bottom=1 = 2).
const borderOverhead = 4

// scrollGutterWidth is the horizontal space reserved for the scrollbar gutter
// (1 space gap + 1 scrollbar character).
const scrollGutterWidth = 2

// scrollTrackChar and scrollThumbChar are the block characters used to render
// the scrollbar on the right edge of scrollable dialogs.
const (
	scrollTrackChar = "░"
	scrollThumbChar = "█"
)

// Base provides shared layout, max-height enforcement, scrolling and scrollbar
// rendering for all dialog types. Each concrete dialog embeds a Base and
// delegates its View rendering through Base.Render().
type Base struct {
	Title        string // dialog title (rendered bold + blue)
	HintText     string // footer hint line (e.g. "enter: confirm  esc: cancel")
	DefaultWidth int    // desired dialog content width
	MinWidth     int    // minimum dialog content width
	TermWidth    int    // current terminal width  (legacy; prefer ctx)
	TermHeight   int    // current terminal height (legacy; prefer ctx)
	ScrollOffset int    // current vertical scroll offset into content lines

	// ctx is the shared ProgramContext pointer. When set, TermWidth/TermHeight
	// are read from ctx on every render, so dialogs automatically adapt to
	// terminal resizes without receiving WindowSizeMsg.
	ctx *tuictx.ProgramContext
}

// NewBase creates a Base with sensible defaults. The termWidth and termHeight
// parameters are used as initial values; if a ProgramContext is later set via
// SetContext, those values will be read from the context instead.
func NewBase(title, hint string, defaultWidth, minWidth, termWidth, termHeight int) Base {
	return Base{
		Title:        title,
		HintText:     hint,
		DefaultWidth: defaultWidth,
		MinWidth:     minWidth,
		TermWidth:    termWidth,
		TermHeight:   termHeight,
	}
}

// NewBaseWithContext creates a Base using a shared ProgramContext for dimensions.
func NewBaseWithContext(title, hint string, defaultWidth, minWidth int, ctx *tuictx.ProgramContext) Base {
	return Base{
		Title:        title,
		HintText:     hint,
		DefaultWidth: defaultWidth,
		MinWidth:     minWidth,
		TermWidth:    ctx.ScreenWidth,
		TermHeight:   ctx.ScreenHeight,
		ctx:          ctx,
	}
}

// SetContext attaches a ProgramContext to the Base. Subsequent calls to
// termW() and termH() will read dimensions from the context.
func (b *Base) SetContext(ctx *tuictx.ProgramContext) {
	b.ctx = ctx
}

// termW returns the current terminal width, preferring the context if set.
func (b *Base) termW() int {
	if b.ctx != nil {
		return b.ctx.ScreenWidth
	}
	return b.TermWidth
}

// termH returns the current terminal height, preferring the context if set.
func (b *Base) termH() int {
	if b.ctx != nil {
		return b.ctx.ScreenHeight
	}
	return b.TermHeight
}

// DialogWidth returns the clamped dialog content width (the lipgloss Width
// value for the outer frame style).
func (b *Base) DialogWidth() int {
	w := b.DefaultWidth
	tw := b.termW()
	// +2 for border, leave 2 columns margin on each side of the terminal
	if w+2 > tw-2 {
		w = tw - 4
	}
	if w < b.MinWidth {
		w = b.MinWidth
	}
	return w
}

// InnerWidth returns the usable content width inside the dialog frame,
// accounting for left+right padding (Padding(1,2) = 2+2 = 4).
func (b *Base) InnerWidth() int {
	return b.DialogWidth() - 4
}

// ContentWidth returns the width available for content lines. When content
// needs scrolling, the scrollbar gutter is reserved; otherwise it equals
// InnerWidth.
func (b *Base) ContentWidth(needsScroll bool) int {
	w := b.InnerWidth()
	if needsScroll {
		w -= scrollGutterWidth
	}
	if w < 10 {
		w = 10
	}
	return w
}

// MaxContentHeight returns the maximum number of content lines that can be
// displayed inside the dialog frame. This accounts for border, padding, title
// (with 1 blank line after), and footer (with 1 blank line before).
func (b *Base) MaxContentHeight() int {
	// title takes ~1 line + 1 padding line below = 2
	// footer takes ~1 line + 1 padding line above = 2
	// border + padding overhead = borderOverhead (4)
	// leave 2 lines of terminal margin
	chrome := borderOverhead + 2 + 2 + 2
	h := b.termH() - chrome
	if h < 3 {
		h = 3
	}
	return h
}

// ClampScroll ensures ScrollOffset is within valid bounds for the given total
// number of content lines.
func (b *Base) ClampScroll(totalLines int) {
	maxVisible := b.MaxContentHeight()
	maxScroll := totalLines - maxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if b.ScrollOffset > maxScroll {
		b.ScrollOffset = maxScroll
	}
	if b.ScrollOffset < 0 {
		b.ScrollOffset = 0
	}
}

// ScrollDown moves the scroll offset down by n lines.
func (b *Base) ScrollDown(n, totalLines int) {
	b.ScrollOffset += n
	b.ClampScroll(totalLines)
}

// ScrollUp moves the scroll offset up by n lines.
func (b *Base) ScrollUp(n int) {
	b.ScrollOffset -= n
	if b.ScrollOffset < 0 {
		b.ScrollOffset = 0
	}
}

// EnsureVisible adjusts ScrollOffset so that the line at idx is visible within
// the viewport. This is used by dialogs with a cursor (e.g. Menu) to keep the
// selected item in view.
func (b *Base) EnsureVisible(idx, totalLines int) {
	maxVisible := b.MaxContentHeight()
	if totalLines <= maxVisible {
		b.ScrollOffset = 0
		return
	}
	// If idx is above the viewport, scroll up to it.
	if idx < b.ScrollOffset {
		b.ScrollOffset = idx
	}
	// If idx is below the viewport, scroll down.
	if idx >= b.ScrollOffset+maxVisible {
		b.ScrollOffset = idx - maxVisible + 1
	}
	b.ClampScroll(totalLines)
}

// HandleScrollKeys processes common scroll key events (pgup/pgdn, ctrl+d/ctrl+u).
// Returns true if the key was consumed.
// Dialogs that manage their own cursor (like Menu) should NOT call this for
// j/k keys and instead handle cursor + EnsureVisible themselves.
func (b *Base) HandleScrollKeys(msg tea.KeyMsg, totalLines int) bool {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+d", "pgdown"))):
		b.ScrollDown(10, totalLines)
		return true
	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+u", "pgup"))):
		b.ScrollUp(10)
		return true
	}
	return false
}

// Render produces the final styled dialog string. contentLines must be flat:
// every entry must be a single terminal line (no embedded newlines). The
// method applies scrolling, pads every line to full width with the dialog
// background, attaches a scrollbar when content overflows, and wraps
// everything in the standard dialog frame.
//
// Rendering follows the bubbles/viewport pattern: every region (title, body,
// footer) is rendered at exact dimensions using Width+MaxWidth and
// Height+MaxHeight so the dialog is fully opaque with no transparent gaps.
func (b *Base) Render(contentLines []string) string {
	t := b.ctx.Theme
	dialogWidth := b.DialogWidth()
	innerWidth := b.InnerWidth()
	maxVisible := b.MaxContentHeight()
	totalLines := len(contentLines)

	// Clamp scroll offset.
	b.ClampScroll(totalLines)

	// Determine if scrolling is needed.
	needsScroll := totalLines > maxVisible
	contentW := b.ContentWidth(needsScroll)

	// Pad every content line to the full content width with Surface0 bg so
	// there are no transparent gaps that let the app bleed through.
	padded := make([]string, totalLines)
	for i, line := range contentLines {
		padded[i] = padLine(line, contentW, t.Surface0)
	}

	// Slice to visible viewport.
	visibleCount := maxVisible
	if visibleCount > totalLines {
		visibleCount = totalLines
	}
	visibleStart := b.ScrollOffset
	visibleEnd := visibleStart + visibleCount
	if visibleEnd > totalLines {
		visibleEnd = totalLines
	}
	visible := padded[visibleStart:visibleEnd]

	// Attach scrollbar gutter if content overflows.
	if needsScroll {
		visible = b.attachScrollbar(visible, totalLines, maxVisible)
	}

	// Join visible content lines into the body string.
	bodyStr := ""
	if len(visible) > 0 {
		bodyStr = lipgloss.JoinVertical(lipgloss.Left, visible...)
	}

	// Wrap the body in exact dimensions (viewport pattern):
	//   Height() pads short content with blank lines to fill the area.
	//   MaxHeight() clips overflow as a safety net.
	//   Width()+MaxWidth() ensure every line is the same width.
	//   Background() fills all padding cells with the dialog color.
	body := lipgloss.NewStyle().
		Width(innerWidth).
		MaxWidth(innerWidth).
		Height(visibleCount).
		MaxHeight(visibleCount).
		Background(t.Surface0).
		Render(bodyStr)

	// Build title — exact inner width, opaque background.
	title := lipgloss.NewStyle().
		Foreground(t.Blue).
		Background(t.Surface0).
		Bold(true).
		Width(innerWidth).
		MaxWidth(innerWidth).
		Padding(0, 0, 1, 0).
		Render(b.Title)

	// Build footer hint — exact inner width, opaque background.
	footer := lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Surface0).
		Width(innerWidth).
		MaxWidth(innerWidth).
		Padding(1, 0, 0, 0).
		Render(b.HintText)

	// Assemble: title + body + footer.
	content := lipgloss.JoinVertical(lipgloss.Left, title, body, footer)

	// Apply the standard dialog frame. MaxHeight is the final safety net
	// to clip the entire dialog if the chrome math is slightly off.
	return lipgloss.NewStyle().
		Width(dialogWidth).
		MaxHeight(b.termH()-2).
		Padding(1, 2).
		Background(t.Surface0).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Blue).
		BorderBackground(t.Surface0).
		Render(content)
}

// padLine pads (or truncates) a rendered line to exactly targetWidth visible
// cells, filling with spaces styled with the given background color.
func padLine(line string, targetWidth int, bg lipgloss.Color) string {
	// Measure the visible (printed) width of the line.
	w := ansi.PrintableRuneWidth(line)
	if w >= targetWidth {
		return line
	}
	pad := strings.Repeat(" ", targetWidth-w)
	padStyled := lipgloss.NewStyle().Background(bg).Render(pad)
	return line + padStyled
}

// attachScrollbar appends a scrollbar gutter to the right side of each visible
// line. The scrollbar uses block characters: ░ for the track and █ for the
// thumb.
func (b *Base) attachScrollbar(visible []string, totalLines, maxVisible int) []string {
	t := b.ctx.Theme
	trackHeight := len(visible)
	if trackHeight == 0 {
		return visible
	}

	// Compute thumb size and position.
	thumbSize := trackHeight * maxVisible / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}
	thumbPos := 0
	if totalLines > maxVisible {
		thumbPos = (trackHeight - thumbSize) * b.ScrollOffset / (totalLines - maxVisible)
		if thumbPos+thumbSize > trackHeight {
			thumbPos = trackHeight - thumbSize
		}
		if thumbPos < 0 {
			thumbPos = 0
		}
	}

	trackStyle := lipgloss.NewStyle().
		Foreground(t.Surface2).
		Background(t.Surface0)
	thumbStyle := lipgloss.NewStyle().
		Foreground(t.Blue).
		Background(t.Surface0)

	result := make([]string, len(visible))
	gap := lipgloss.NewStyle().Background(t.Surface0).Render(" ")
	for i, line := range visible {
		ch := scrollTrackChar
		style := trackStyle
		if i >= thumbPos && i < thumbPos+thumbSize {
			ch = scrollThumbChar
			style = thumbStyle
		}
		result[i] = line + gap + style.Render(ch)
	}
	return result
}

// FlattenLines splits a rendered string that may contain embedded newlines
// into individual line entries. This is a utility for dialog buildContentLines
// methods to ensure every returned entry is a single terminal line.
func FlattenLines(rendered string) []string {
	return strings.Split(rendered, "\n")
}
