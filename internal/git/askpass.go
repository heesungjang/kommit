package git

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// createAskpassScript creates a temporary executable script that responds to
// git credential prompts. When git asks for a username, the script returns the
// provided username. When git asks for a password, it returns the token.
//
// This approach avoids modifying any git config or credential stores — the
// script exists only for the duration of the command and is cleaned up
// afterward via the returned cleanup function.
//
// Returns the path to the script and a cleanup function.
func createAskpassScript(username, token string) (path string, cleanup func(), err error) {
	// Create a temp file with the appropriate extension.
	var script string
	var pattern string

	if runtime.GOOS == "windows" {
		pattern = "kommit-askpass-*.bat"
		// Windows batch script: check if the prompt contains "Username",
		// echo the username; otherwise echo the token.
		script = fmt.Sprintf(
			"@echo off\r\n"+
				"echo %%1 | findstr /i \"username\" >nul\r\n"+
				"if %%errorlevel%%==0 (\r\n"+
				"    echo %s\r\n"+
				") else (\r\n"+
				"    echo %s\r\n"+
				")\r\n",
			username, token)
	} else {
		pattern = "kommit-askpass-*.sh"
		// POSIX shell script. Git passes the prompt as $1.
		// We check for "username" (case-insensitive via tr).
		script = fmt.Sprintf(
			"#!/bin/sh\n"+
				"prompt=$(echo \"$1\" | tr '[:upper:]' '[:lower:]')\n"+
				"case \"$prompt\" in\n"+
				"  *username*) echo '%s' ;;\n"+
				"  *) echo '%s' ;;\n"+
				"esac\n",
			escapeShellSingleQuote(username),
			escapeShellSingleQuote(token))
	}

	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, fmt.Errorf("creating askpass script: %w", err)
	}
	path = f.Name()

	if _, writeErr := f.WriteString(script); writeErr != nil {
		f.Close()
		os.Remove(path)
		return "", nil, fmt.Errorf("writing askpass script: %w", writeErr)
	}
	f.Close()

	// Make executable (no-op on Windows for .bat files).
	if chmodErr := os.Chmod(path, 0o700); chmodErr != nil {
		os.Remove(path)
		return "", nil, fmt.Errorf("making askpass script executable: %w", chmodErr)
	}

	cleanup = func() {
		os.Remove(path)
	}
	return path, cleanup, nil
}

// escapeShellSingleQuote escapes single quotes for use inside a single-quoted
// shell string. The technique is: end the current quote, insert an escaped
// single quote, restart the quote. e.g. it's -> it'\”s
func escapeShellSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}
