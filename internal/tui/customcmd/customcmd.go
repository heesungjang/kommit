// Package customcmd handles execution of user-defined shell commands
// configured in the opengit config file.
package customcmd

import (
	"bytes"
	"os/exec"
	"strings"
	"text/template"

	"github.com/heesungjang/kommit/internal/config"
)

// TemplateVars holds the variables available for template substitution
// in custom command strings.
type TemplateVars struct {
	Hash      string // current commit hash
	ShortHash string // abbreviated hash
	Branch    string // current branch name
	Path      string // selected file path
	RepoRoot  string // repository root directory
	Subject   string // commit subject
	Author    string // commit author
}

// Expand substitutes template variables in the command string.
// Template syntax is Go's text/template: {{.Hash}}, {{.Branch}}, etc.
func Expand(cmdStr string, vars TemplateVars) (string, error) {
	// Fast path: if no template delimiters, return as-is.
	if !strings.Contains(cmdStr, "{{") {
		return cmdStr, nil
	}
	tmpl, err := template.New("cmd").Parse(cmdStr)
	if err != nil {
		return cmdStr, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return cmdStr, err
	}
	return buf.String(), nil
}

// Run executes a shell command and returns its combined output.
// The command is run via /bin/sh -c for shell expansion support.
func Run(cmdStr, repoDir string) (string, error) {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// RunInteractive returns an *exec.Cmd suitable for tea.ExecProcess,
// allowing the command to take over the terminal (for interactive commands).
func RunInteractive(cmdStr, repoDir string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = repoDir
	return cmd
}

// FilterByContext returns only commands that match the given context.
// If a command has an empty context, it matches "global".
func FilterByContext(cmds []config.CustomCommand, ctx string) []config.CustomCommand {
	var result []config.CustomCommand
	for _, c := range cmds {
		cmdCtx := c.Context
		if cmdCtx == "" {
			cmdCtx = "global"
		}
		if cmdCtx == ctx || cmdCtx == "global" {
			result = append(result, c)
		}
	}
	return result
}
