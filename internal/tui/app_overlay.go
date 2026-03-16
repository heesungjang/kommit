package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// overlayAt composites |overlay| on top of |base| at character position (x, y).
// It detects the overlay's background color and re-injects it after every SGR
// reset within the overlay lines, preventing transparent cells from letting the
// base content bleed through.
func overlayAt(base, overlay string, x, y, _ int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Detect the overlay's background color SGR from the rendered content.
	bgSGR := extractBgSGR(overlay)

	for i, oLine := range overlayLines {
		row := y + i
		if row < 0 || row >= len(baseLines) {
			continue
		}

		bLine := baseLines[row]
		oWidth := lipgloss.Width(oLine)

		left := ansi.Truncate(bLine, x, "")
		right := ansi.TruncateLeft(bLine, x+oWidth, "")

		// Fill background on every printable cell that lacks one so
		// transparent cells don't let the base content bleed through.
		if bgSGR != "" {
			oLine = fillBgCells(oLine, bgSGR)
		}

		baseLines[row] = left + "\033[0m" + bgSGR + oLine + right
	}

	return strings.Join(baseLines, "\n")
}

// extractBgSGR scans an ANSI string for the first background color SGR
// parameter and returns it as a standalone SGR sequence (e.g. "\033[48;2;49;50;68m").
// Returns "" if no background color is found.
func extractBgSGR(s string) string {
	for i := 0; i < len(s); i++ {
		// Look for ESC [ ... m sequences (CSI SGR).
		if s[i] != '\033' || i+1 >= len(s) || s[i+1] != '[' {
			continue
		}
		// Found ESC[, now collect params until 'm' or a non-param byte.
		j := i + 2
		for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == ';') {
			j++
		}
		if j >= len(s) || s[j] != 'm' {
			i = j
			continue
		}
		// s[i+2:j] is the params string, s[j] == 'm'.
		params := s[i+2 : j]
		if bg := extractBgFromParams(params); bg != "" {
			return "\033[" + bg + "m"
		}
		i = j
	}
	return ""
}

// extractBgFromParams extracts the background color portion from a
// semicolon-separated SGR parameter string. Returns the background params
// (e.g. "48;2;49;50;68") or "" if no background is present.
func extractBgFromParams(params string) string {
	parts := strings.Split(params, ";")
	for idx := 0; idx < len(parts); idx++ {
		p := parts[idx]
		// Basic 3-bit / 4-bit background: 40-47 (not 48=extended, not 49=default)
		if len(p) == 2 && p[0] == '4' && p[1] >= '0' && p[1] <= '7' {
			return p
		}
		if len(p) == 3 && p[0] == '1' && p[1] == '0' && p[2] >= '0' && p[2] <= '7' {
			return p
		}
		// Extended background: 48;5;N or 48;2;R;G;B
		if p == "48" && idx+1 < len(parts) {
			if parts[idx+1] == "5" && idx+2 < len(parts) {
				// 256-color: 48;5;N
				return parts[idx] + ";" + parts[idx+1] + ";" + parts[idx+2]
			}
			if parts[idx+1] == "2" && idx+4 < len(parts) {
				// True color: 48;2;R;G;B
				return parts[idx] + ";" + parts[idx+1] + ";" + parts[idx+2] + ";" + parts[idx+3] + ";" + parts[idx+4]
			}
		}
	}
	return ""
}

// fillBgCells walks an ANSI string and ensures every printable cell has the
// given background color active. It tracks the current SGR state and injects
// bgSGR before any run of printable characters that lacks an explicit
// background. This handles all transparent cells — not just those after resets.
func fillBgCells(line, bgSGR string) string {
	var buf strings.Builder
	buf.Grow(len(line) + 256)

	hasBg := false
	i := 0
	for i < len(line) {
		// Check for CSI sequence: ESC [
		if line[i] == '\033' && i+1 < len(line) && line[i+1] == '[' {
			// Collect parameter bytes (0x30-0x3F) and intermediate bytes (0x20-0x2F)
			// until a final byte (0x40-0x7E).
			j := i + 2
			for j < len(line) && ((line[j] >= '0' && line[j] <= '?') || (line[j] >= ' ' && line[j] <= '/')) {
				j++
			}
			if j < len(line) {
				seq := line[i : j+1]
				if line[j] == 'm' {
					// SGR sequence — update background tracking state.
					hasBg = updateBgState(line[i+2:j], hasBg)
				}
				buf.WriteString(seq)
				i = j + 1
				continue
			}
			// Unterminated sequence — copy ESC and continue.
			buf.WriteByte(line[i])
			i++
			continue
		}

		// Skip other ESC sequences (OSC, etc.) — copy through.
		if line[i] == '\033' {
			buf.WriteByte(line[i])
			i++
			continue
		}

		// Control characters — copy through without bg injection.
		if line[i] < ' ' {
			buf.WriteByte(line[i])
			i++
			continue
		}

		// Printable character — inject bgSGR if no background is active.
		if !hasBg {
			buf.WriteString(bgSGR)
			hasBg = true
		}
		buf.WriteByte(line[i])
		i++
	}
	return buf.String()
}

// updateBgState parses a semicolon-separated SGR parameter string and returns
// whether a background color is active after applying the parameters.
func updateBgState(params string, hasBg bool) bool {
	// Empty params means implicit reset ("\033[m").
	if params == "" {
		return false
	}

	parts := strings.Split(params, ";")
	for idx := 0; idx < len(parts); idx++ {
		p := parts[idx]
		switch {
		case p == "0" || p == "":
			// Explicit reset or empty sub-param — clears all attributes.
			hasBg = false
		case p == "49":
			// Default background color — clears background.
			hasBg = false
		case len(p) == 2 && p[0] == '4' && p[1] >= '0' && p[1] <= '7':
			// Basic background: 40-47.
			hasBg = true
		case p == "48":
			// Extended background (48;5;N or 48;2;R;G;B) — skip sub-params.
			hasBg = true
			if idx+1 < len(parts) && parts[idx+1] == "5" {
				idx += 2 // skip 5;N
			} else if idx+1 < len(parts) && parts[idx+1] == "2" {
				idx += 4 // skip 2;R;G;B
			}
		case len(p) == 3 && p[0] == '1' && p[1] == '0' && p[2] >= '0' && p[2] <= '7':
			// Bright background: 100-107.
			hasBg = true
		}
		// All other params (foreground, bold, etc.) don't affect bg state.
	}
	return hasBg
}
