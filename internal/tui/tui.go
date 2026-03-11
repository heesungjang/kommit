package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/heesungjang/kommit/internal/config"
	"github.com/heesungjang/kommit/internal/git"
)

// Run launches the opengit TUI, blocking until the user quits.
func Run(repo *git.Repository, cfg *config.Config) error {
	app := NewApp(repo, cfg)
	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
