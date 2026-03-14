package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// Shared style helpers that reference the active theme.

// ClipPanel ensures a rendered panel string has exactly targetHeight lines.
// Lipgloss Height() only pads and never clips, and Width() can introduce
// wrapping that adds unexpected lines. This function defensively clips the
// output so that JoinHorizontal never receives mismatched panel heights.
func ClipPanel(rendered string, targetHeight int) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) > targetHeight {
		lines = lines[:targetHeight]
	}
	return strings.Join(lines, "\n")
}

// PanelBorderWidth is the horizontal overhead added by PanelStyle's border.
// lipgloss Width() includes padding but excludes border, so the outer rendered
// width = Width(w) + PanelBorderWidth.
const PanelBorderWidth = 2

// PanelBorderHeight is the vertical overhead added by PanelStyle's border.
const PanelBorderHeight = 2

// PanelPaddingWidth is the horizontal padding inside PanelStyle (Padding(0, 1) = 1 left + 1 right).
const PanelPaddingWidth = 2

// PanelStyle returns the base style for a panel/pane.
func PanelStyle(focused bool) lipgloss.Style {
	t := theme.Active
	borderColor := t.Surface1
	if focused {
		borderColor = t.Blue
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(t.Base).
		Background(t.Base).
		Padding(0, 1)
}

// PanelStyleColor returns a panel style with an explicit border color.
// Used by the border animation system to render interpolated border colors.
func PanelStyleColor(borderColor lipgloss.Color) lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(t.Base).
		Background(t.Base).
		Padding(0, 1)
}

// TitleStyle returns the style for panel titles.
func TitleStyle(focused bool) lipgloss.Style {
	t := theme.Active
	fg := t.Subtext0
	if focused {
		fg = t.Blue
	}
	return lipgloss.NewStyle().
		Foreground(fg).
		Background(t.Base).
		Bold(true).
		Padding(0, 1)
}

// PanelTitle renders a panel title with a right-aligned shortcut key indicator.
// The shortcutKey is shown as e.g. "[1]" in the top-right area of the title.
func PanelTitle(label, shortcutKey string, focused bool, width int) string {
	t := theme.Active
	fg := t.Subtext0
	if focused {
		fg = t.Blue
	}

	keyTag := "[" + shortcutKey + "]"
	keyFg := t.Overlay0
	if focused {
		keyFg = t.Blue
	}
	keyStyle := lipgloss.NewStyle().Foreground(keyFg).Background(t.Base)

	titleStyle := lipgloss.NewStyle().
		Foreground(fg).
		Background(t.Base).
		Bold(true)

	// Compute available space: width minus key tag length minus 1 space gap
	labelWidth := width - len(keyTag) - 1
	if labelWidth < 4 {
		// Not enough room for key tag; just render the title
		return titleStyle.Width(width).Render(label)
	}

	// Truncate label so it never wraps to a second line (which would
	// leave a transparent gap next to the key tag).
	if len(label) > labelWidth {
		if labelWidth > 1 {
			label = label[:labelWidth-1] + "…"
		} else {
			label = label[:labelWidth]
		}
	}

	leftPart := titleStyle.Width(labelWidth).Render(label)
	rightPart := keyStyle.Render(keyTag)
	gap := lipgloss.NewStyle().Background(t.Base).Render(" ")

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPart, gap, rightPart)
}

// SelectedStyle returns the style for selected list items.
func SelectedStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Surface1).
		Bold(true)
}

// NormalStyle returns the style for normal list items.
func NormalStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Base)
}

// DimStyle returns the style for dimmed/secondary text.
func DimStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Base)
}

// StatusBarStyle returns the style for the status bar.
func StatusBarStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Subtext1).
		Background(t.Surface0).
		Padding(0, 1)
}

// TabStyle returns the style for tab items.
func TabStyle(active bool) lipgloss.Style {
	t := theme.Active
	if active {
		return lipgloss.NewStyle().
			Foreground(t.Blue).
			Background(t.Mantle).
			Bold(true).
			Padding(0, 1)
	}
	return lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Mantle).
		Padding(0, 1)
}

// KeyHintStyle returns the style for keybinding hints.
func KeyHintStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Subtext0)
}

// KeyStyle returns the style for the key itself in a hint.
func KeyStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Mauve).
		Bold(true)
}

// ErrorStyle returns the style for error messages.
func ErrorStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Red).
		Background(t.Base).
		Bold(true)
}

// SuccessStyle returns the style for success messages.
func SuccessStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Green).
		Background(t.Base)
}

// WarningStyle returns the style for warning messages.
func WarningStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().
		Foreground(t.Yellow).
		Background(t.Base)
}

// FileStatusColor returns the color for a file based on its status code.
func FileStatusColor(code byte) lipgloss.Color {
	t := theme.Active
	switch code {
	case 'M':
		return t.StatusModified()
	case 'A':
		return t.StatusAdded()
	case 'D':
		return t.StatusDeleted()
	case 'R':
		return t.StatusRenamed()
	case 'U':
		return t.StatusConflict()
	case '?':
		return t.StatusUntracked()
	default:
		return t.Text
	}
}

// DiffLineStyle returns the style for a diff line based on its leading character.
func DiffLineStyle(lineType byte) lipgloss.Style {
	t := theme.Active
	switch lineType {
	case '+':
		return lipgloss.NewStyle().Foreground(t.DiffAdded()).Background(t.DiffAddedBg())
	case '-':
		return lipgloss.NewStyle().Foreground(t.DiffRemoved()).Background(t.DiffRemovedBg())
	case '@':
		return lipgloss.NewStyle().Foreground(t.DiffHunkHeader()).Background(t.Base).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(t.DiffContext()).Background(t.Base)
	}
}

// DiffCurrentHunkStyle returns the style for the currently selected hunk header.
func DiffCurrentHunkStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().Foreground(t.Blue).Background(t.Base).Bold(true)
}

// DiffFileHeaderStyle returns the style for diff file header lines
// (diff --git, ---, +++).
func DiffFileHeaderStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Bold(true)
}

// DiffMetaStyle returns the style for diff metadata lines (index, similarity, etc.).
func DiffMetaStyle() lipgloss.Style {
	t := theme.Active
	return lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base)
}

// DiffLineNumStyle returns the style for line numbers in the diff gutter.
func DiffLineNumStyle(lineType byte) lipgloss.Style {
	t := theme.Active
	switch lineType {
	case '+':
		return lipgloss.NewStyle().Foreground(t.DiffAdded()).Background(t.DiffAddedBg())
	case '-':
		return lipgloss.NewStyle().Foreground(t.DiffRemoved()).Background(t.DiffRemovedBg())
	case '@':
		return lipgloss.NewStyle().Foreground(t.DiffLineNum()).Background(t.Base)
	default:
		return lipgloss.NewStyle().Foreground(t.DiffLineNum()).Background(t.Base)
	}
}

// DiffGutterSepStyle returns the style for the gutter separator character.
func DiffGutterSepStyle(lineType byte) lipgloss.Style {
	t := theme.Active
	var bg lipgloss.Color
	switch lineType {
	case '+':
		bg = t.DiffAddedBg()
	case '-':
		bg = t.DiffRemovedBg()
	case '@':
		bg = t.Base
	default:
		bg = t.Base
	}
	return lipgloss.NewStyle().Foreground(t.DiffLineNumSep()).Background(bg)
}

// RefType classifies a git decoration string.
type RefType int

const (
	RefLocalBranch RefType = iota
	RefRemoteBranch
	RefTag
	RefHead
)

// ParsedRef holds a classified ref name and its display text.
type ParsedRef struct {
	Type    RefType
	Display string
}

// ParseRef classifies a raw git decoration string (e.g. "HEAD -> main",
// "origin/main", "tag: v1.0") and returns a ParsedRef.
func ParseRef(raw string) ParsedRef {
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "HEAD -> "):
		branch := strings.TrimPrefix(raw, "HEAD -> ")
		return ParsedRef{Type: RefLocalBranch, Display: branch}
	case raw == "HEAD":
		return ParsedRef{Type: RefHead, Display: "HEAD"}
	case strings.HasPrefix(raw, "tag: "):
		tag := strings.TrimPrefix(raw, "tag: ")
		return ParsedRef{Type: RefTag, Display: tag}
	case strings.Contains(raw, "/"):
		// remote branch like origin/main
		return ParsedRef{Type: RefRemoteBranch, Display: raw}
	default:
		return ParsedRef{Type: RefLocalBranch, Display: raw}
	}
}

// RefBadgeColor returns the foreground color for a given ref type.
func RefBadgeColor(rt RefType) lipgloss.Color {
	t := theme.Active
	switch rt {
	case RefLocalBranch:
		return t.BranchCurrent()
	case RefRemoteBranch:
		return t.BranchRemote()
	case RefTag:
		return t.TagColor()
	case RefHead:
		return t.HeadColor()
	default:
		return t.Text
	}
}

// RenderRefBadge renders a single ref as colored bracketed text "[refname]".
func RenderRefBadge(ref ParsedRef, bg lipgloss.Color) string {
	fg := RefBadgeColor(ref.Type)
	return lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Render("[" + ref.Display + "]")
}

// RenderRefBadges parses and renders all refs as space-separated colored badges.
// Returns empty string if refs is empty.
func RenderRefBadges(refs []string, bg lipgloss.Color) string {
	if len(refs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(refs))
	for _, r := range refs {
		parsed := ParseRef(r)
		parts = append(parts, RenderRefBadge(parsed, bg))
	}
	sp := lipgloss.NewStyle().Background(bg).Render(" ")
	return strings.Join(parts, sp)
}

// GraphColors returns a palette of colors for graph branch tracks.
// Colors cycle based on column index.
func GraphColor(column int) lipgloss.Color {
	t := theme.Active
	palette := []lipgloss.Color{
		t.Green,
		t.Blue,
		t.Mauve,
		t.Peach,
		t.Teal,
		t.Pink,
		t.Yellow,
		t.Lavender,
	}
	return palette[column%len(palette)]
}

// FileListIcon returns a status icon character for a DiffFile status string.
func FileListIcon(status string) string {
	switch status {
	case "added":
		return "A"
	case "deleted":
		return "D"
	case "modified":
		return "M"
	case "renamed":
		return "R"
	default:
		return "?"
	}
}

// FileListColor returns the color for a DiffFile status string.
func FileListColor(status string) lipgloss.Color {
	t := theme.Active
	switch status {
	case "added":
		return t.StatusAdded()
	case "deleted":
		return t.StatusDeleted()
	case "modified":
		return t.StatusModified()
	case "renamed":
		return t.StatusRenamed()
	default:
		return t.Text
	}
}
