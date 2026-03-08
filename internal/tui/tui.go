package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nicholascross/opengit/internal/git"
)

// Run launches the opengit TUI, blocking until the user quits.
func Run(repo *git.Repository) error {
	app := NewApp(repo)
	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
