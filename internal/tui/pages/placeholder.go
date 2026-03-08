package pages

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nicholascross/opengit/internal/tui/styles"
	"github.com/nicholascross/opengit/internal/tui/theme"
)

// PlaceholderPage is used for pages that are not yet implemented.
type PlaceholderPage struct {
	title  string
	width  int
	height int
}

// NewPlaceholderPage creates a placeholder page with the given title.
func NewPlaceholderPage(title string, width, height int) PlaceholderPage {
	return PlaceholderPage{
		title:  title,
		width:  width,
		height: height,
	}
}

func (p PlaceholderPage) Init() tea.Cmd { return nil }

func (p PlaceholderPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		p.width = msg.Width
		p.height = msg.Height
	}
	return p, nil
}

func (p PlaceholderPage) View() string {
	t := theme.Active
	titleStr := styles.TitleStyle(true).Render(p.title)
	body := lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.Base).
		Padding(2, 4).
		Render("This page is not yet implemented.")

	pw := p.width - styles.PanelBorderWidth
	ph := p.height - styles.PanelBorderHeight
	return styles.PanelStyle(true).Width(pw).Height(ph).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleStr, body),
	)
}

var _ tea.Model = PlaceholderPage{}
