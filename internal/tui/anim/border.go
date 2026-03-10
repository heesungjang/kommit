// Package anim provides lightweight animation primitives for the TUI.
package anim

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AnimInterval is the tick rate for border animations (~60fps).
const AnimInterval = 16 * time.Millisecond

// animStep is the progress increment per frame. 8 frames = 128ms total.
const animStep = 0.125

// BorderID identifies a specific animated border.
type BorderID int

const (
	BorderSidebar       BorderID = iota // left panel border
	BorderCenter                        // center panel border
	BorderRight                         // right panel border
	BorderCommitOuter                   // WIP commit container border
	BorderCommitSummary                 // WIP commit summary input border
	BorderCommitDesc                    // WIP commit description input border
	borderCount                         // sentinel — must be last
)

// BorderAnimTickMsg is the tick message that drives animation frames.
type BorderAnimTickMsg struct{}

// BorderAnim tracks transition progress for all animated borders.
type BorderAnim struct {
	progress [borderCount]float64 // 0.0 = unfocused, 1.0 = focused
	target   [borderCount]float64 // desired end state
	active   bool                 // true when any border is mid-transition
}

// SetFocus sets the target state for a border. If the target differs from
// the current progress, the animation is marked active.
func (b *BorderAnim) SetFocus(id BorderID, focused bool) {
	t := 0.0
	if focused {
		t = 1.0
	}
	if b.target[id] != t {
		b.target[id] = t
		b.active = true
	}
}

// Tick advances all borders one step toward their targets. Returns true if
// any border is still animating (caller should schedule another tick).
func (b *BorderAnim) Tick() bool {
	b.active = false
	for i := BorderID(0); i < borderCount; i++ {
		if b.progress[i] == b.target[i] {
			continue
		}
		if b.progress[i] < b.target[i] {
			b.progress[i] += animStep
			if b.progress[i] >= b.target[i] {
				b.progress[i] = b.target[i]
			} else {
				b.active = true
			}
		} else {
			b.progress[i] -= animStep
			if b.progress[i] <= b.target[i] {
				b.progress[i] = b.target[i]
			} else {
				b.active = true
			}
		}
	}
	return b.active
}

// Active reports whether any border is currently mid-transition.
func (b *BorderAnim) Active() bool {
	return b.active
}

// Color returns the interpolated color for a border, lerping between
// unfocused and focused colors based on the current progress.
func (b *BorderAnim) Color(id BorderID, unfocused, focused lipgloss.Color) lipgloss.Color {
	t := b.progress[id]
	if t <= 0 {
		return unfocused
	}
	if t >= 1 {
		return focused
	}
	return lerpColor(unfocused, focused, t)
}

// ScheduleTick returns a tea.Cmd that sends a BorderAnimTickMsg after
// AnimInterval. Use this to drive the animation loop.
func ScheduleTick() tea.Cmd {
	return tea.Tick(AnimInterval, func(_ time.Time) tea.Msg {
		return BorderAnimTickMsg{}
	})
}

// lerpColor linearly interpolates between two lipgloss hex colors.
func lerpColor(a, b lipgloss.Color, t float64) lipgloss.Color {
	ar, ag, ab := parseHex(string(a))
	br, bg, bb := parseHex(string(b))

	r := ar + (br-ar)*t
	g := ag + (bg-ag)*t
	bl := ab + (bb-ab)*t

	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", clamp8(r), clamp8(g), clamp8(bl)))
}

// parseHex parses a "#RRGGBB" hex string into float64 RGB components [0,255].
func parseHex(s string) (r, g, b float64) {
	if len(s) < 7 || s[0] != '#' {
		return 0, 0, 0
	}
	var ri, gi, bi int
	fmt.Sscanf(s[1:], "%02x%02x%02x", &ri, &gi, &bi)
	return float64(ri), float64(gi), float64(bi)
}

// clamp8 clamps a float64 to [0,255] and converts to uint8.
func clamp8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
