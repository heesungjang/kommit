// Package utils provides shared utility functions for the TUI.
package utils

import (
	"os/exec"
	"runtime"
	"strings"
)

// CopyToClipboard copies text to the system clipboard.
// Returns an error if the clipboard command fails or is unsupported.
func CopyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return errUnsupportedOS
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// OpenBrowser opens a URL in the default browser.
// Returns an error if the browser command fails.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("open", url)
	}
	return cmd.Start()
}
