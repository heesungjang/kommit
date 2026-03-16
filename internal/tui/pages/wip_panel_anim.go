package pages

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// shimmerRadius is the half-width of the shimmer highlight in characters.
// The full highlight spans 2*shimmerRadius+1 characters. A larger radius
// gives a softer, more premium-feeling sweep.
const shimmerRadius = 5

// shimmerEasePos returns the shimmer center position for the given tick,
// using a cosine ease-in-out curve so the highlight accelerates through
// the middle and glides to a stop at the edges.
func shimmerEasePos(tick, barWidth int) int {
	// Total sweep range: bar width + lead-in/out padding.
	pad := shimmerRadius + 4
	totalRange := barWidth + 2*pad

	// Period in ticks for one full sweep. At ~120ms/tick this gives
	// a ~2.4s sweep which feels natural.
	period := 20
	phase := tick % period

	// Cosine ease-in-out: maps linear phase [0, period) to [0, totalRange).
	// cos goes from 1 → -1 over [0, π], so (1-cos)/2 goes 0 → 1.
	t := float64(phase) / float64(period) // 0.0 → 1.0
	eased := (1.0 - math.Cos(t*math.Pi)) / 2.0
	return int(eased*float64(totalRange)) - pad
}

// renderShimmerBar renders a skeleton bar of █ characters with a smooth
// shimmer highlight sweeping left-to-right. Each character is individually
// colored with a cosine-based gradient centered on the shimmer position,
// creating a soft glowing wave effect.
func renderShimmerBar(barLen, totalWidth, shimmerPos, lineOffset int, dimColor, brightColor, bg lipgloss.Color) string {
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
		if dist > shimmerRadius {
			fg = dimColor
		} else {
			// Cosine falloff: 1.0 at center → 0.0 at radius edge.
			ratio := (1.0 + math.Cos(float64(dist)*math.Pi/float64(shimmerRadius))) / 2.0
			fg = lipgloss.Color(blendHex(dimHex, brightHex, ratio))
		}
		b.WriteString(lipgloss.NewStyle().Foreground(fg).Background(bg).Render("█"))
	}

	// Pad remaining width with background.
	if pad := totalWidth - barLen; pad > 0 {
		b.WriteString(lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad)))
	}
	return b.String()
}

// blendHex blends two hex color strings. ratio 0.0 returns color a,
// ratio 1.0 returns color b. Expects "#RRGGBB" format.
func blendHex(a, b string, ratio float64) string {
	ar, ag, ab := parseHexColor(a)
	br, bg, bb := parseHexColor(b)

	lerp := func(from, to int) int {
		v := float64(from) + (float64(to)-float64(from))*ratio
		return max(0, min(255, int(v)))
	}

	return fmt.Sprintf("#%02x%02x%02x", lerp(ar, br), lerp(ag, bg), lerp(ab, bb))
}

// parseHexColor parses a "#RRGGBB" string into RGB components.
func parseHexColor(s string) (r, g, b int) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return 0, 0, 0
	}
	_, _ = fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}
