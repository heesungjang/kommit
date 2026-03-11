package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette and visual style of the application.
type Theme struct {
	Name string

	// Base colors
	Base     lipgloss.Color
	Mantle   lipgloss.Color
	Crust    lipgloss.Color
	Surface0 lipgloss.Color
	Surface1 lipgloss.Color
	Surface2 lipgloss.Color
	Overlay0 lipgloss.Color
	Overlay1 lipgloss.Color
	Text     lipgloss.Color
	Subtext0 lipgloss.Color
	Subtext1 lipgloss.Color

	// Accent colors
	Red       lipgloss.Color
	Green     lipgloss.Color
	Yellow    lipgloss.Color
	Blue      lipgloss.Color
	Mauve     lipgloss.Color
	Pink      lipgloss.Color
	Teal      lipgloss.Color
	Sky       lipgloss.Color
	Peach     lipgloss.Color
	Maroon    lipgloss.Color
	Lavender  lipgloss.Color
	Flamingo  lipgloss.Color
	Rosewater lipgloss.Color
	Sapphire  lipgloss.Color

	// Diff background colors (accent blended with base at ~20%)
	DiffAddedLineBg   lipgloss.Color // green-tinted bg for + lines
	DiffRemovedLineBg lipgloss.Color // red-tinted bg for - lines
	DiffHunkHeaderBg  lipgloss.Color // bg for @@ hunk header lines
}

// Semantic color accessors for git-specific use cases.

// DiffAdded returns the color for added lines.
func (t Theme) DiffAdded() lipgloss.Color { return t.Green }

// DiffRemoved returns the color for removed lines.
func (t Theme) DiffRemoved() lipgloss.Color { return t.Red }

// DiffContext returns the color for context lines.
func (t Theme) DiffContext() lipgloss.Color { return t.Subtext0 }

// DiffHunkHeader returns the color for hunk headers.
func (t Theme) DiffHunkHeader() lipgloss.Color { return t.Blue }

// DiffAddedBg returns the background color for added (+) lines.
func (t Theme) DiffAddedBg() lipgloss.Color { return t.DiffAddedLineBg }

// DiffRemovedBg returns the background color for removed (-) lines.
func (t Theme) DiffRemovedBg() lipgloss.Color { return t.DiffRemovedLineBg }

// DiffHunkBg returns the background color for hunk header (@@ ) lines.
func (t Theme) DiffHunkBg() lipgloss.Color { return t.DiffHunkHeaderBg }

// DiffLineNum returns the color for diff line numbers (context lines).
func (t Theme) DiffLineNum() lipgloss.Color { return t.Overlay0 }

// DiffLineNumSep returns the color for the line number gutter separator.
func (t Theme) DiffLineNumSep() lipgloss.Color { return t.Overlay0 }

// StatusModified returns the color for modified files.
func (t Theme) StatusModified() lipgloss.Color { return t.Yellow }

// StatusAdded returns the color for added files.
func (t Theme) StatusAdded() lipgloss.Color { return t.Green }

// StatusDeleted returns the color for deleted files.
func (t Theme) StatusDeleted() lipgloss.Color { return t.Red }

// StatusRenamed returns the color for renamed files.
func (t Theme) StatusRenamed() lipgloss.Color { return t.Blue }

// StatusConflict returns the color for conflicted files.
func (t Theme) StatusConflict() lipgloss.Color { return t.Maroon }

// StatusUntracked returns the color for untracked files.
func (t Theme) StatusUntracked() lipgloss.Color { return t.Subtext0 }

// BranchCurrent returns the color for the current branch.
func (t Theme) BranchCurrent() lipgloss.Color { return t.Green }

// BranchLocal returns the color for local branches.
func (t Theme) BranchLocal() lipgloss.Color { return t.Text }

// BranchRemote returns the color for remote branches.
func (t Theme) BranchRemote() lipgloss.Color { return t.Mauve }

// Focused returns the color for focused elements.
func (t Theme) Focused() lipgloss.Color { return t.Blue }

// TagColor returns the color for tags.
func (t Theme) TagColor() lipgloss.Color { return t.Yellow }

// HeadColor returns the color for HEAD indicator.
func (t Theme) HeadColor() lipgloss.Color { return t.Sky }

// Dimmed returns the color for dimmed/inactive elements.
func (t Theme) Dimmed() lipgloss.Color { return t.Surface2 }

// Active is the current theme.
var Active = CatppuccinMocha()

// IsDark returns true if the theme has a dark background (luminance < 50%).
func (t Theme) IsDark() bool {
	hex := string(t.Base)
	if hex != "" && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return true // assume dark
	}
	r := hexDigit(hex[0])<<4 | hexDigit(hex[1])
	g := hexDigit(hex[2])<<4 | hexDigit(hex[3])
	b := hexDigit(hex[4])<<4 | hexDigit(hex[5])
	// Perceived luminance (ITU-R BT.601)
	lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	return lum < 128
}

func hexDigit(c byte) uint8 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

// ColorOverrides maps theme color field names to hex color strings.
// Used by config to override individual theme colors.
type ColorOverrides struct {
	Base      string
	Mantle    string
	Crust     string
	Surface0  string
	Surface1  string
	Surface2  string
	Overlay0  string
	Overlay1  string
	Text      string
	Subtext0  string
	Subtext1  string
	Red       string
	Green     string
	Yellow    string
	Blue      string
	Mauve     string
	Pink      string
	Teal      string
	Sky       string
	Peach     string
	Maroon    string
	Lavender  string
	Flamingo  string
	Rosewater string
	Sapphire  string
}

// applyOverride sets the target to the hex color if non-empty.
func applyOverride(target *lipgloss.Color, hex string) {
	if hex != "" {
		c := lipgloss.Color(hex)
		*target = c
	}
}

// ApplyOverrides applies non-empty color overrides to the theme.
func (t *Theme) ApplyOverrides(o ColorOverrides) {
	applyOverride(&t.Base, o.Base)
	applyOverride(&t.Mantle, o.Mantle)
	applyOverride(&t.Crust, o.Crust)
	applyOverride(&t.Surface0, o.Surface0)
	applyOverride(&t.Surface1, o.Surface1)
	applyOverride(&t.Surface2, o.Surface2)
	applyOverride(&t.Overlay0, o.Overlay0)
	applyOverride(&t.Overlay1, o.Overlay1)
	applyOverride(&t.Text, o.Text)
	applyOverride(&t.Subtext0, o.Subtext0)
	applyOverride(&t.Subtext1, o.Subtext1)
	applyOverride(&t.Red, o.Red)
	applyOverride(&t.Green, o.Green)
	applyOverride(&t.Yellow, o.Yellow)
	applyOverride(&t.Blue, o.Blue)
	applyOverride(&t.Mauve, o.Mauve)
	applyOverride(&t.Pink, o.Pink)
	applyOverride(&t.Teal, o.Teal)
	applyOverride(&t.Sky, o.Sky)
	applyOverride(&t.Peach, o.Peach)
	applyOverride(&t.Maroon, o.Maroon)
	applyOverride(&t.Lavender, o.Lavender)
	applyOverride(&t.Flamingo, o.Flamingo)
	applyOverride(&t.Rosewater, o.Rosewater)
	applyOverride(&t.Sapphire, o.Sapphire)
}
