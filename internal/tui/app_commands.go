package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heesungjang/kommit/internal/ai"
	"github.com/heesungjang/kommit/internal/auth"
	"github.com/heesungjang/kommit/internal/config"
	"github.com/heesungjang/kommit/internal/git"
	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/customcmd"
	"github.com/heesungjang/kommit/internal/tui/dialog"
	"github.com/heesungjang/kommit/internal/tui/icons"
	"github.com/heesungjang/kommit/internal/tui/keys"
	"github.com/heesungjang/kommit/internal/tui/msgs"
	"github.com/heesungjang/kommit/internal/tui/pages"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

// openSettings creates and shows the settings dialog.
func (a App) openSettings() tea.Cmd {
	cfg := a.ctx.Config
	pctx := a.ctx
	return func() tea.Msg {
		return showDialogMsg{model: dialog.NewSettings(cfg, pctx)}
	}
}

// applySettingsChange handles a settings change: updates the in-memory config,
// applies the change to the running app, and persists to disk.
func (a App) applySettingsChange(msg dialog.SettingsChangeMsg) tea.Cmd {
	cfg := a.ctx.Config
	if cfg == nil {
		return nil
	}

	themeChanged := false

	switch msg.Key {
	case "theme":
		// Hot-swap theme: update config, rebuild theme, update context.
		cfg.Theme = msg.Value
		newTheme := theme.Get(msg.Value)
		// Apply color overrides from config.
		o := cfg.Appearance.ThemeColors
		newTheme.ApplyOverrides(theme.ColorOverrides{
			Base: o.Base, Mantle: o.Mantle, Crust: o.Crust,
			Surface0: o.Surface0, Surface1: o.Surface1, Surface2: o.Surface2,
			Overlay0: o.Overlay0, Overlay1: o.Overlay1,
			Text: o.Text, Subtext0: o.Subtext0, Subtext1: o.Subtext1,
			Red: o.Red, Green: o.Green, Yellow: o.Yellow, Blue: o.Blue,
			Mauve: o.Mauve, Pink: o.Pink, Teal: o.Teal, Sky: o.Sky,
			Peach: o.Peach, Maroon: o.Maroon, Lavender: o.Lavender,
			Flamingo: o.Flamingo, Rosewater: o.Rosewater, Sapphire: o.Sapphire,
		})
		theme.Active = newTheme
		a.ctx.Theme = newTheme
		a.ctx.Styles = tuictx.InitStyles(newTheme)
		themeChanged = true

	case "appearance.diffMode":
		cfg.Appearance.DiffMode = msg.Value

	case "appearance.showGraph":
		cfg.Appearance.ShowGraph = msg.Value == "true"

	case "appearance.compactLog":
		cfg.Appearance.CompactLog = msg.Value == "true"

	case "appearance.nerdFonts":
		cfg.Appearance.NerdFonts = msg.Value == "true"
		ic := icons.ForConfig(cfg.Appearance.NerdFonts)
		icons.Active = ic
		a.ctx.Icons = ic

	case "ai.provider":
		cfg.AI.Provider = msg.Value
		// Auto-reset model to the new provider's default so we don't send
		// e.g. a Claude model name to the OpenAI API.
		cfg.AI.Model = ai.DefaultModel(msg.Value)

	case "ai.model":
		cfg.AI.Model = msg.Value
	}

	// Notify pages of the change so they can sync local state.
	settingKey := msg.Key
	settingValue := msg.Value
	cmds := []tea.Cmd{
		func() tea.Msg {
			return pages.SettingsUpdatedMsg{Key: settingKey, Value: settingValue}
		},
	}

	// Force a full screen clear on theme changes so the terminal repaints
	// all lines atomically instead of updating them one-by-one (which causes
	// a visible top-to-bottom "wipe" effect on many terminals).
	if themeChanged {
		cmds = append(cmds, tea.ClearScreen)
	}

	// Persist config to disk — but skip for preview changes (the user
	// hasn't confirmed yet, and they might cancel).
	if !msg.Preview {
		cfgCopy := *cfg
		cmds = append(cmds, func() tea.Msg {
			if err := config.Save(&cfgCopy); err != nil {
				return msgs.ToastMsg{Message: "Settings applied but failed to save: " + err.Error(), IsError: true}
			}
			return nil
		})
	}

	return tea.Batch(cmds...)
}

// ---------------------------------------------------------------------------
// Command palette
// ---------------------------------------------------------------------------

// openCommandPalette builds and shows the command palette dialog.
func (a App) openCommandPalette() tea.Cmd {
	// Build entries from registered key bindings.
	actionNames := keys.ActionNames()
	entries := make([]dialog.PaletteEntry, 0, len(actionNames))
	for _, name := range actionNames {
		entry := dialog.PaletteEntry{
			Action: name,
			Label:  formatActionLabel(name),
		}
		// Look up the current key binding for this action.
		if binding := keys.LookupBinding(name); binding != nil {
			entry.Key = binding.Help().Key
		}
		entries = append(entries, entry)
	}

	// Add custom commands if configured.
	if a.ctx.Config != nil {
		for _, cc := range a.ctx.Config.CustomCommands {
			entries = append(entries, dialog.PaletteEntry{
				Action:      "custom:" + cc.Name,
				Label:       cc.Name,
				Description: cc.Description,
				Key:         cc.Key,
			})
		}
	}

	pctx := a.ctx
	return func() tea.Msg {
		return showDialogMsg{model: dialog.NewCommandPalette(entries, pctx)}
	}
}

// dispatchPaletteAction executes the action selected from the command palette.
func (a App) dispatchPaletteAction(action string) tea.Cmd {
	// Handle custom commands.
	if strings.HasPrefix(action, "custom:") {
		name := strings.TrimPrefix(action, "custom:")
		if a.ctx.Config != nil {
			for i, cc := range a.ctx.Config.CustomCommands {
				if cc.Name == name {
					a.customCmdList = a.ctx.Config.CustomCommands
					a.customCmdVars = customcmd.TemplateVars{}
					if a.repo != nil {
						a.customCmdVars.RepoRoot = a.repo.Path()
						if br, err := a.repo.Head(); err == nil {
							a.customCmdVars.Branch = br
						}
					}
					return a.executeCustomCommand(i)
				}
			}
		}
		return nil
	}

	// Handle specific actions that need direct dispatch.
	if action == "pr.create" {
		return a.openCreatePRDialog()
	}

	// For key binding actions, we synthesize the key press.
	binding := keys.LookupBinding(action)
	if binding == nil {
		return nil
	}
	// Get the first key from the binding and synthesize a KeyMsg.
	bindKeys := binding.Keys()
	if len(bindKeys) == 0 {
		return nil
	}
	// Send a synthetic key press to the main view.
	keyStr := bindKeys[0]
	return func() tea.Msg {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	}
}

// formatActionLabel converts a canonical action name like "nav.pageDown" to
// a human-readable label like "Navigation: Page Down".
func formatActionLabel(action string) string {
	parts := strings.SplitN(action, ".", 2)
	if len(parts) != 2 {
		return action
	}

	// Map context prefixes to human labels.
	contextLabels := map[string]string{
		"global": "Global",
		"nav":    "Navigation",
		"status": "Status",
		"branch": "Branch",
		"commit": "Commit",
		"diff":   "Diff",
		"stash":  "Stash",
		"remote": "Remote",
		"pr":     "Pull Request",
	}

	ctx := parts[0]
	name := parts[1]
	label, ok := contextLabels[ctx]
	if !ok {
		label = capitalize(ctx)
	}

	// Convert camelCase to Title Case.
	var words []string
	word := strings.Builder{}
	for i, c := range name {
		if i > 0 && c >= 'A' && c <= 'Z' {
			words = append(words, word.String())
			word.Reset()
		}
		if i == 0 {
			word.WriteRune(c - 32 + 32) // keep as-is, capitalize below
		} else {
			word.WriteRune(c)
		}
	}
	words = append(words, word.String())

	// Capitalize first letter of each word.
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}

	return label + ": " + strings.Join(words, " ")
}

// ---------------------------------------------------------------------------
// Custom commands
// ---------------------------------------------------------------------------

// buildCustomCommandsMenu creates a tea.Cmd that opens the custom commands menu.
// It also sets a.customCmdList and a.customCmdVars so that executeCustomCommand
// can look up the selected command when the menu result arrives.
// IMPORTANT: This must be called in the Update method so that the modified `a`
// is returned to Bubble Tea (value receiver semantics).
func (a *App) buildCustomCommandsMenu(ctxName string, vars customcmd.TemplateVars) tea.Cmd {
	cfg := a.ctx.Config
	if cfg == nil || len(cfg.CustomCommands) == 0 {
		return func() tea.Msg {
			return pages.RequestToastMsg{Message: "No custom commands configured", IsError: false}
		}
	}

	if ctxName == "" {
		ctxName = "global"
	}
	filtered := customcmd.FilterByContext(cfg.CustomCommands, ctxName)
	if len(filtered) == 0 {
		return func() tea.Msg {
			return pages.RequestToastMsg{Message: "No custom commands for context: " + ctxName, IsError: false}
		}
	}

	// Store the filtered list and vars for executeCustomCommand.
	a.customCmdList = filtered
	a.customCmdVars = vars
	// Fill in branch if not provided.
	if a.customCmdVars.Branch == "" && a.repo != nil {
		if br, err := a.repo.Head(); err == nil {
			a.customCmdVars.Branch = br
		}
	}
	if a.customCmdVars.RepoRoot == "" && a.repo != nil {
		a.customCmdVars.RepoRoot = a.repo.Path()
	}

	// Build menu options.
	opts := make([]dialog.MenuOption, 0, len(filtered))
	for i, c := range filtered {
		desc := c.Description
		if desc == "" {
			desc = c.Command
		}
		shortKey := ""
		if i < 9 {
			shortKey = fmt.Sprintf("%d", i+1)
		}
		opts = append(opts, dialog.MenuOption{
			Label:       c.Name,
			Description: desc,
			Key:         shortKey,
		})
	}

	pctx := a.ctx
	return func() tea.Msg {
		return showDialogMsg{model: dialog.NewMenu("custom-commands", "Custom Commands", opts, pctx)}
	}
}

// executeCustomCommand runs the selected custom command (by index into customCmdList).
func (a App) executeCustomCommand(idx int) tea.Cmd {
	if idx < 0 || idx >= len(a.customCmdList) {
		return nil
	}
	cc := a.customCmdList[idx]
	vars := a.customCmdVars
	repoDir := ""
	if a.repo != nil {
		repoDir = a.repo.Path()
	}

	// Expand template variables.
	expanded, err := customcmd.Expand(cc.Command, vars)
	if err != nil {
		name := cc.Name
		return func() tea.Msg {
			return customCmdDoneMsg{name: name, err: err}
		}
	}

	// If the command requires confirmation, open a confirm dialog first.
	if cc.Confirm {
		pctx := a.ctx
		name := cc.Name
		return func() tea.Msg {
			body := "Run command?\n\n" + expanded
			return showDialogMsg{model: dialog.NewConfirm("customcmd-"+name, name, body, pctx)}
		}
	}

	// If the command is interactive (suspend TUI), use tea.ExecProcess.
	if cc.Suspend {
		cmd := customcmd.RunInteractive(expanded, repoDir)
		name := cc.Name
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			return customCmdDoneMsg{name: name, err: err}
		})
	}

	// Run in background and capture output.
	name := cc.Name
	showOutput := cc.ShowOutput
	return func() tea.Msg {
		output, runErr := customcmd.Run(expanded, repoDir)
		return customCmdDoneMsg{
			name:   name,
			output: output,
			err:    runErr,
			show:   showOutput,
		}
	}
}

// ---------------------------------------------------------------------------
// Repo / workspace switching
// ---------------------------------------------------------------------------

// switchToRepo opens a repository and switches to the log page. If path is
// empty, it switches back to the last opened repo (if one is loaded).
func (a App) switchToRepo(path string) tea.Cmd {
	if path == "" {
		// Switch back to current repo view if we have one.
		if a.repo != nil {
			// Return a command that sends backToRepoMsg so the switch
			// happens in the Update() method (not on a value-receiver copy).
			return func() tea.Msg { return backToRepoMsg{} }
		}
		return nil
	}

	repoPath := expandPath(path)
	return func() tea.Msg {
		repo, err := git.Open(repoPath)
		if err != nil {
			return pages.RequestToastMsg{Message: "Cannot open: " + err.Error(), IsError: true}
		}
		return switchToRepoMsg{repo: repo, path: repoPath}
	}
}

// backToRepoMsg tells the app to switch back to the log page for the
// currently loaded repo (without reopening it).
type backToRepoMsg struct{}

// switchToRepoMsg carries a successfully opened repo from the background.
type switchToRepoMsg struct {
	repo *git.Repository
	path string
}

// switchToWorkspace switches to the workspace page.
func (a App) switchToWorkspace() tea.Cmd {
	return func() tea.Msg {
		return showWorkspaceMsg{}
	}
}

type showWorkspaceMsg struct{}

// doWorkspaceBulkOp runs fetch or pull on a list of repo paths concurrently.
// It attempts to resolve credentials for each repo's origin remote so that
// private repos are supported.
func (a App) doWorkspaceBulkOp(op string, paths []string) tea.Cmd {
	return func() tea.Msg {
		type result struct {
			path string
			err  error
		}
		ch := make(chan result, len(paths))
		for _, p := range paths {
			go func(repoPath string) {
				repo, err := git.Open(repoPath)
				if err != nil {
					ch <- result{path: repoPath, err: err}
					return
				}
				// Try to resolve credentials for this repo's origin.
				user, token := "", ""
				if remoteURL, urlErr := repo.RemoteURL("origin"); urlErr == nil && remoteURL != "" {
					if acct := auth.AccountForRemote(remoteURL); acct != nil && acct.Token != "" {
						user, token = acct.GitUser, acct.Token
					}
				}
				switch op {
				case "fetch":
					if token != "" {
						err = repo.FetchAuth(user, token)
					} else {
						err = repo.Fetch()
					}
				case "pull":
					if token != "" {
						err = repo.PullAuth("", "", user, token)
					} else {
						err = repo.Pull("", "")
					}
				}
				ch <- result{path: repoPath, err: err}
			}(p)
		}

		var errors []string
		for range paths {
			r := <-ch
			if r.err != nil {
				errors = append(errors, r.path+": "+r.err.Error())
			}
		}

		return pages.WorkspaceBulkOpDoneMsg{
			Op:     op,
			Total:  len(paths),
			Failed: len(errors),
			Errors: errors,
		}
	}
}

// handleWorkspaceTextInput processes text input results for workspace operations.
func (a App) handleWorkspaceTextInput(id, value string) tea.Cmd {
	switch {
	case id == "workspace-new":
		return func() tea.Msg {
			return pages.RequestNewWorkspaceMsg{Name: value}
		}
	case strings.HasPrefix(id, "workspace-rename-"):
		idxStr := strings.TrimPrefix(id, "workspace-rename-")
		idx := 0
		_, _ = fmt.Sscanf(idxStr, "%d", &idx)
		return func() tea.Msg {
			return pages.RequestRenameWorkspaceMsg{WorkspaceIndex: idx, NewName: value}
		}
	case strings.HasPrefix(id, "workspace-addrepo-"):
		idxStr := strings.TrimPrefix(id, "workspace-addrepo-")
		idx := 0
		_, _ = fmt.Sscanf(idxStr, "%d", &idx)
		return func() tea.Msg {
			return pages.RequestAddRepoToWorkspaceMsg{WorkspaceIndex: idx, RepoPath: value}
		}
	}
	return nil
}

// expandPath expands ~ to the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	// Try making it absolute.
	abs, err := filepath.Abs(path)
	if err == nil {
		return abs
	}
	return path
}
