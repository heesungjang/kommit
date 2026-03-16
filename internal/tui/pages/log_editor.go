package pages

import (
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// openInEditor returns a tea.Cmd that suspends the TUI and opens the given
// file in the user's preferred editor.
func (l LogPage) openInEditor(path string) tea.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vim"
	}

	// Build absolute path relative to the repo root.
	abs := filepath.Join(l.repo.Path(), path)
	c := exec.Command(editor, abs)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}
