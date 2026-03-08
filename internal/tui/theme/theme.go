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
