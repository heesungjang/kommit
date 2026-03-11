package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/heesungjang/kommit/internal/ai"
	"github.com/heesungjang/kommit/internal/config"
	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/components"
	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/customcmd"
	"github.com/heesungjang/kommit/internal/tui/dialog"
	"github.com/heesungjang/kommit/internal/tui/keys"
	"github.com/heesungjang/kommit/internal/tui/pages"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// Minimum terminal dimensions for a usable layout.
const (
	minTermWidth  = 80
	minTermHeight = 15
)

// pollInterval is how often the app checks for external git changes.
const pollInterval = 2 * time.Second

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// showDialogMsg asks the app shell to display a dialog overlay.
type showDialogMsg struct{ model tea.Model }

// hideDialogMsg closes the active dialog overlay.
type hideDialogMsg struct{}

// branchInfoMsg carries branch metadata for the status bar.
type branchInfoMsg struct {
	branch    string
	ahead     int
	behind    int
	bisecting bool
	rebasing  bool
}

// commitDoneMsg is sent when a commit operation completes.
type commitDoneMsg struct {
	err error
}

// gitOpDoneMsg is sent when a push/pull/fetch operation completes.
type gitOpDoneMsg struct {
	op  string
	err error
}

// customCmdDoneMsg is sent when a custom command finishes execution.
type customCmdDoneMsg struct {
	name   string
	output string
	err    error
	show   bool // whether to show output in a toast
}

// pollTickMsg is sent periodically to check for external git changes.
type pollTickMsg struct{}

// pollResultMsg carries the result of a background fingerprint check.
type pollResultMsg struct {
	fingerprint string
	changed     bool
}

// ---------------------------------------------------------------------------
// App model
// ---------------------------------------------------------------------------

// App is the root Bubble Tea model. It renders a single unified view
// (sidebar | commit graph | context detail) plus a status bar and overlays.
type App struct {
	ctx       *tuictx.ProgramContext // shared context (dimensions, theme, config, repo)
	repo      *git.Repository
	mainView  tea.Model // the LogPage — our single unified view
	hintBar   components.HintBar
	statusBar components.StatusBar
	keys      keys.GlobalKeys
	width     int
	height    int

	// Dialog overlay (commit message, confirm, help, etc.)
	dialog     tea.Model
	showDialog bool

	// Toast notification
	toast components.Toast

	// Auto-refresh polling — fingerprint of last known git status
	lastFingerprint string

	// Custom commands — filtered list of commands from the most recent menu.
	// These are set when a custom commands menu is opened and read when a
	// selection is made. They persist because both openCustomCommandsMenu
	// and executeCustomCommand are called from the same Update cycle
	// (the App value is copied at the start of Update and returned at the end).
	customCmdList []config.CustomCommand
	customCmdVars customcmd.TemplateVars

	// Pending commit — held while the confirm dialog is shown.
	pendingCommitMsg *dialog.CommitRequestMsg
}

// NewApp creates a new App rooted at the given repository.
func NewApp(repo *git.Repository, cfg *config.Config) App {
	ctx := tuictx.New(cfg, repo)
	// Set the package-level Active theme so existing code that reads
	// theme.Active directly continues to work during the migration.
	theme.Active = ctx.Theme

	// Apply user keybinding overrides from config.
	if len(cfg.Keybinds.Custom) > 0 {
		keys.ApplyOverrides(cfg.Keybinds.Custom)
	}

	keys.ActiveContext = keys.ContextLog
	return App{
		ctx:       ctx,
		repo:      repo,
		mainView:  pages.NewLogPage(ctx, 80, 24), // initial size; updated on WindowSizeMsg
		hintBar:   components.NewHintBar(),
		statusBar: components.NewStatusBar(),
		keys:      keys.NewGlobalKeys(),
		toast:     components.NewToast(),
	}
}

// Init initializes the application.
func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.mainView.Init(),
		a.loadBranchInfo(),
		a.schedulePoll(),
	)
}

// Update processes messages.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// -- Window resize -------------------------------------------------------
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Update the shared context so all components see the new size.
		chromeHeight := 2 // hint bar + status bar
		a.ctx.SetScreenSize(msg.Width, msg.Height, chromeHeight)
		a.hintBar = a.hintBar.SetWidth(msg.Width)
		a.statusBar = a.statusBar.SetSize(msg.Width)
		a.toast = a.toast.SetWidth(msg.Width)
		// Propagate to main view with adjusted height (minus status bar).
		pageMsg := tea.WindowSizeMsg{
			Width:  msg.Width,
			Height: a.pageHeight(),
		}
		var cmd tea.Cmd
		a.mainView, cmd = a.mainView.Update(pageMsg)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)

	// -- Toast dismiss -------------------------------------------------------
	case components.ToastDismissMsg:
		a.toast = a.toast.Dismiss()
		return a, nil

	// -- Dialog lifecycle ----------------------------------------------------
	case showDialogMsg:
		a.dialog = msg.model
		a.showDialog = true
		return a, a.dialog.Init()

	case hideDialogMsg:
		a.showDialog = false
		a.dialog = nil
		return a, nil

	// -- Command palette result -----------------------------------------------
	case dialog.CommandPaletteResultMsg:
		a.showDialog = false
		a.dialog = nil
		return a, a.dispatchPaletteAction(msg.Action)

	case dialog.CommandPaletteCloseMsg:
		a.showDialog = false
		a.dialog = nil
		return a, nil

	// -- Help dialog close ---------------------------------------------------
	case dialog.HelpCloseMsg:
		a.showDialog = false
		a.dialog = nil
		return a, nil

	// -- Settings dialog close -----------------------------------------------
	case dialog.SettingsCloseMsg:
		a.showDialog = false
		a.dialog = nil
		return a, nil

	// -- AI Setup dialog result -----------------------------------------------
	case dialog.AISetupResultMsg:
		a.showDialog = false
		a.dialog = nil

		// Copilot uses a browser OAuth flow — open the Copilot OAuth dialog.
		if msg.Provider == "copilot" && msg.APIKey == "__copilot_oauth__" {
			cfg := a.ctx.Config
			if cfg != nil {
				cfg.AI.Provider = "copilot"
				cfg.AI.Model = ai.DefaultModel("copilot")
			}
			dlg := dialog.NewCopilotOAuth(a.ctx)
			a.dialog = dlg
			a.showDialog = true
			return a, dlg.Init()
		}

		// Save the API key to credentials file and update config.
		provider := msg.Provider
		apiKey := msg.APIKey
		cfg := a.ctx.Config
		if cfg != nil {
			cfg.AI.Provider = provider
			cfg.AI.Model = ai.DefaultModel(provider)
		}
		// Save credentials, update config on disk, then trigger AI generation.
		return a, func() tea.Msg {
			// Save API key to auth.json (separate from config).
			if apiKey != "" {
				_ = ai.SetAPIKey(provider, apiKey)
			}
			// Save provider change to config.
			if cfg != nil {
				cfgCopy := *cfg
				_ = config.Save(&cfgCopy)
			}
			// Re-trigger AI commit generation now that we have a key.
			return pages.RequestAICommitMsg{}
		}

	case dialog.AISetupCancelMsg:
		a.showDialog = false
		a.dialog = nil
		// Reset the aiGenerating flag by sending an error back to the LogPage.
		return a, func() tea.Msg {
			return pages.AICommitErrorMsg{Err: fmt.Errorf("AI setup canceled")}
		}

	// -- Copilot OAuth dialog result -----------------------------------------
	case dialog.CopilotOAuthResultMsg:
		a.showDialog = false
		a.dialog = nil
		ghToken := msg.GitHubToken
		cpToken := msg.CopilotToken
		cfg := a.ctx.Config
		return a, func() tea.Msg {
			// Save the GitHub token for future refresh, and the Copilot token for API calls.
			_ = ai.SetAPIKey("copilot", ghToken)
			if cfg != nil {
				cfgCopy := *cfg
				_ = config.Save(&cfgCopy)
			}
			// Set the Copilot bearer token in-memory for this session.
			if cfg != nil {
				cfg.AI.APIKey = cpToken
			}
			return pages.RequestAICommitMsg{}
		}

	case dialog.CopilotOAuthErrorMsg:
		a.showDialog = false
		a.dialog = nil
		errMsg := msg.Err.Error()
		return a, func() tea.Msg {
			return pages.AICommitErrorMsg{Err: fmt.Errorf("Copilot login: %s", errMsg)}
		}

	case dialog.CopilotOAuthCancelMsg:
		a.showDialog = false
		a.dialog = nil
		return a, func() tea.Msg {
			return pages.AICommitErrorMsg{Err: fmt.Errorf("Copilot login canceled")}
		}

	// -- Settings change (theme swap, toggles, etc.) -------------------------
	case dialog.SettingsChangeMsg:
		return a, a.applySettingsChange(msg)

	// -- Settings updated — forward to main view even while dialog is open --
	case pages.SettingsUpdatedMsg:
		var cmd tea.Cmd
		a.mainView, cmd = a.mainView.Update(msg)
		return a, cmd

	// -- Confirm dialog result -----------------------------------------------
	case dialog.ConfirmResultMsg:
		a.showDialog = false
		a.dialog = nil
		// Handle quit confirmation.
		if msg.ID == "quit" {
			if msg.Confirmed {
				return a, tea.Quit
			}
			return a, nil
		}
		// Handle custom command confirmations
		if strings.HasPrefix(msg.ID, "customcmd-") {
			if msg.Confirmed {
				// Find the command by name
				name := strings.TrimPrefix(msg.ID, "customcmd-")
				for i, cc := range a.customCmdList {
					if cc.Name == name {
						// Re-run executeCustomCommand but skip the confirm step.
						// Copy the command and clear Confirm to avoid infinite loop.
						cc.Confirm = false
						a.customCmdList[i] = cc
						return a, a.executeCustomCommand(i)
					}
				}
			}
			return a, nil
		}
		// Handle commit confirmation.
		if msg.ID == "commit" {
			pending := a.pendingCommitMsg
			a.pendingCommitMsg = nil
			if msg.Confirmed && pending != nil {
				if pending.Amend {
					return a, a.doCommitAmend(pending.Message)
				}
				return a, a.doCommit(pending.Message)
			}
			// Canceled — restore the commit message in the WIP panel.
			if pending != nil {
				summary := pending.Message
				desc := ""
				if idx := strings.Index(summary, "\n\n"); idx >= 0 {
					desc = strings.TrimSpace(summary[idx+2:])
					summary = summary[:idx]
				}
				return a, func() tea.Msg {
					return pages.RestoreCommitMsg{Summary: summary, Description: desc}
				}
			}
			return a, nil
		}
		// Handle git op confirmations (push/pull/force-push)
		if msg.Confirmed && strings.HasPrefix(msg.ID, "gitop-") {
			suffix := strings.TrimPrefix(msg.ID, "gitop-")
			force := false
			op := suffix
			if suffix == "force-push" {
				op = "push"
				force = true
			}
			label := capitalize(op)
			if force {
				label = "Force p" + label[1:] // "Force pushing..."
			}
			var toastCmd tea.Cmd
			a.toast, toastCmd = a.toast.ShowInfo(label + "ing...")
			return a, tea.Batch(toastCmd, a.doGitOp(op, force))
		}
		// Forward all other confirmations to the main view
		var cmd tea.Cmd
		a.mainView, cmd = a.mainView.Update(msg)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)

	// -- Commit dialog -------------------------------------------------------
	case dialog.CommitRequestMsg:
		a.showDialog = false
		a.dialog = nil
		// Store the pending commit and show a confirmation dialog.
		a.pendingCommitMsg = &msg
		title := "Commit?"
		body := msg.Message
		if msg.Amend {
			title = "Amend commit?"
		}
		// Truncate the body shown in the confirm dialog to keep it readable.
		if len(body) > 120 {
			body = body[:117] + "..."
		}
		dlg := dialog.NewConfirm("commit", title, body, a.ctx)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	case dialog.CommitCancelMsg:
		a.showDialog = false
		a.dialog = nil
		return a, nil

	case commitDoneMsg:
		if msg.err != nil {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.ShowError("Commit failed: " + msg.err.Error())
			cmds = append(cmds, cmd)
		} else {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.ShowSuccess("Commit created")
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, func() tea.Msg { return pages.RefreshStatusMsg{} })
		return a, tea.Batch(cmds...)

	// -- Commit dialog request -----------------------------------------------
	case pages.RequestCommitDialogMsg:
		dlg := dialog.NewCommitMsg(msg.StagedCount, a.ctx)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	// -- Confirm dialog request ----------------------------------------------
	case pages.RequestConfirmMsg:
		dlg := dialog.NewConfirm(msg.ID, msg.Title, msg.Message, a.ctx)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	// -- Menu dialog request -------------------------------------------------
	case pages.RequestMenuMsg:
		var menuOpts []dialog.MenuOption
		for _, o := range msg.Options {
			menuOpts = append(menuOpts, dialog.MenuOption{Label: o.Label, Description: o.Description, Key: o.Key})
		}
		dlg := dialog.NewMenu(msg.ID, msg.Title, menuOpts, a.ctx)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	// -- Menu dialog result --------------------------------------------------
	case dialog.MenuResultMsg:
		a.showDialog = false
		a.dialog = nil
		if msg.ID == "custom-commands" {
			return a, a.executeCustomCommand(msg.Index)
		}
		var cmd tea.Cmd
		a.mainView, cmd = a.mainView.Update(msg)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)

	case dialog.MenuCancelMsg:
		a.showDialog = false
		a.dialog = nil
		return a, nil

	// -- Amend commit dialog request -----------------------------------------
	case pages.RequestAmendDialogMsg:
		repo := a.repo
		stagedCount := msg.StagedCount
		ctx := a.ctx
		return a, func() tea.Msg {
			info, err := repo.LastCommit()
			if err != nil || info == nil {
				return showDialogMsg{model: dialog.NewCommitMsg(stagedCount, ctx)}
			}
			prevMsg := info.Subject
			if info.Body != "" {
				prevMsg = info.Subject + "\n\n" + info.Body
			}
			return showDialogMsg{model: dialog.NewCommitMsgAmend(stagedCount, prevMsg, ctx)}
		}

	// -- Git push/pull/fetch request -----------------------------------------
	case pages.RequestGitOpMsg:
		// Push and pull require confirmation; fetch is non-destructive.
		if msg.Op == "push" || msg.Op == "pull" {
			id := "gitop-" + msg.Op
			title := capitalize(msg.Op) + "?"
			body := "Are you sure you want to " + msg.Op + "?"
			if msg.Force {
				id = "gitop-force-push"
				title = "Force Push?"
				body = "Force push with --force-with-lease?\nThis will overwrite remote history."
			}
			dlg := dialog.NewConfirm(id, title, body, a.ctx)
			a.dialog = dlg
			a.showDialog = true
			return a, dlg.Init()
		}
		var toastCmd tea.Cmd
		a.toast, toastCmd = a.toast.ShowInfo(capitalize(msg.Op) + "ing...")
		return a, tea.Batch(toastCmd, a.doGitOp(msg.Op, false))

	case gitOpDoneMsg:
		if msg.err != nil {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.ShowError(capitalize(msg.op) + " failed: " + friendlyGitError(msg.op, msg.err))
			cmds = append(cmds, cmd)
		} else {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.ShowSuccess(capitalize(msg.op) + " complete")
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, func() tea.Msg { return pages.RefreshStatusMsg{} })
		return a, tea.Batch(cmds...)

	// -- Text input dialog request -------------------------------------------
	case pages.RequestTextInputMsg:
		dlg := dialog.NewTextInput(msg.ID, msg.Title, msg.Placeholder, msg.InitialValue, a.ctx)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	// -- Text input result — close dialog, route to main view ---------------
	case dialog.TextInputResultMsg:
		a.showDialog = false
		a.dialog = nil
		var cmd tea.Cmd
		a.mainView, cmd = a.mainView.Update(msg)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)

	// -- Text input cancel — close dialog, route to main view ---------------
	case dialog.TextInputCancelMsg:
		a.showDialog = false
		a.dialog = nil
		var cmd tea.Cmd
		a.mainView, cmd = a.mainView.Update(msg)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)

	// -- Custom command done --------------------------------------------------
	case customCmdDoneMsg:
		if msg.err != nil {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.ShowError(msg.name + ": " + msg.err.Error())
			cmds = append(cmds, cmd)
		} else if msg.show && msg.output != "" {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.ShowSuccess(msg.name + ": " + msg.output)
			cmds = append(cmds, cmd)
		} else {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.ShowSuccess(msg.name + " completed")
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, func() tea.Msg { return pages.RefreshStatusMsg{} })
		return a, tea.Batch(cmds...)

	// -- Custom command menu request from pages ------------------------------
	case pages.RequestCustomCmdMenuMsg:
		vars := customcmd.TemplateVars{
			Hash:      msg.Hash,
			ShortHash: msg.ShortHash,
			Branch:    msg.Branch,
			Path:      msg.Path,
			Subject:   msg.Subject,
			Author:    msg.Author,
		}
		if cmd := a.buildCustomCommandsMenu(msg.Context, vars); cmd != nil {
			return a, cmd
		}

	// -- Compare state indicator ---------------------------------------------
	case pages.CompareStateMsg:
		if msg.Active {
			a.statusBar = a.statusBar.SetComparing(msg.Hash)
		} else {
			a.statusBar = a.statusBar.SetComparing("")
		}
		return a, nil

	// -- Status dirty indicator ---------------------------------------------
	case pages.StatusDirtyMsg:
		a.statusBar = a.statusBar.SetClean(!msg.Dirty)
		return a, nil

	// -- AI commit message request from WIP panel --------------------------
	case pages.RequestAICommitMsg:
		return a, a.generateAICommitMessage()

	// -- AI commit result — route back to main view ----------------------
	case pages.AICommitResultMsg:
		var cmd tea.Cmd
		a.mainView, cmd = a.mainView.Update(msg)
		return a, cmd

	case pages.AICommitErrorMsg:
		var cmd tea.Cmd
		a.mainView, cmd = a.mainView.Update(msg)
		return a, cmd

	// -- Settings change request from pages (e.g. V key toggle) ------------
	case pages.RequestSettingsChangeMsg:
		return a, a.applySettingsChange(dialog.SettingsChangeMsg{
			Key:   msg.Key,
			Value: msg.Value,
		})

	// -- Toast request from pages -------------------------------------------
	case pages.RequestToastMsg:
		if msg.IsError {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.ShowError(msg.Message)
			return a, cmd
		}
		var cmd tea.Cmd
		a.toast, cmd = a.toast.ShowSuccess(msg.Message)
		return a, cmd

	// -- Branch info loaded --------------------------------------------------
	case branchInfoMsg:
		a.statusBar = a.statusBar.SetBranch(msg.branch).SetAheadBehind(msg.ahead, msg.behind).
			SetBisecting(msg.bisecting).SetRebasing(msg.rebasing)
		return a, nil

	// -- Refresh after mutations ---------------------------------------------
	case pages.RefreshStatusMsg:
		var cmd tea.Cmd
		a.mainView, cmd = a.mainView.Update(msg)
		cmds = append(cmds, cmd, a.loadBranchInfo())
		return a, tea.Batch(cmds...)

	// -- Auto-refresh polling ------------------------------------------------
	case pollTickMsg:
		return a, a.checkFingerprint()

	case pollResultMsg:
		a.lastFingerprint = msg.fingerprint
		if msg.changed {
			cmds = append(cmds, func() tea.Msg { return pages.RefreshStatusMsg{} })
		}
		cmds = append(cmds, a.schedulePoll())
		return a, tea.Batch(cmds...)

	// -- Mouse events --------------------------------------------------------
	case tea.MouseMsg:
		if a.showDialog && a.dialog != nil {
			return a, nil
		}

		statusBarY := a.height - 1
		if msg.Y >= statusBarY {
			return a, nil // clicks on status bar — ignore
		}

		// All mouse events go to the main view (no tab bar to intercept).
		var cmd tea.Cmd
		a.mainView, cmd = a.mainView.Update(msg)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)

	// -- Key events ----------------------------------------------------------
	case tea.KeyMsg:
		if key.Matches(msg, a.keys.ForceQuit) {
			return a, tea.Quit
		}

		// If a dialog is shown, route input there exclusively.
		if a.showDialog && a.dialog != nil {
			updated, cmd := a.dialog.Update(msg)
			a.dialog = updated
			return a, cmd
		}

		// Skip global shortcuts when an inline editor is active (e.g., commit message).
		editing := false
		if lp, ok := a.mainView.(interface{ IsEditing() bool }); ok {
			editing = lp.IsEditing()
		}

		if !editing {
			// Global shortcuts.
			switch {
			case key.Matches(msg, a.keys.Quit):
				dlg := dialog.NewConfirm("quit", "Quit?", "Are you sure you want to quit?", a.ctx)
				a.dialog = dlg
				a.showDialog = true
				return a, dlg.Init()
			case key.Matches(msg, a.keys.Help):
				kctx := keys.ActiveContext
				pctx := a.ctx
				return a, func() tea.Msg {
					return showDialogMsg{model: dialog.NewHelp(kctx, pctx)}
				}
			case key.Matches(msg, a.keys.CommandPalette):
				return a, a.openCommandPalette()
			case key.Matches(msg, a.keys.CustomCommands):
				if cmd := a.buildCustomCommandsMenu("global", customcmd.TemplateVars{}); cmd != nil {
					return a, cmd
				}
			case key.Matches(msg, a.keys.Settings):
				return a, a.openSettings()
			}
		}

	}

	// -- Fallthrough: route to dialog or main view ---------------------------
	if a.showDialog && a.dialog != nil {
		updated, cmd := a.dialog.Update(msg)
		a.dialog = updated
		cmds = append(cmds, cmd)
	} else {
		var cmd tea.Cmd
		a.mainView, cmd = a.mainView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

// View renders the full application layout:
//
//	main view  (all remaining space)
//	toast      (optional overlay)
//	status bar (1 line)
func (a App) View() string {
	t := theme.Active

	// Terminal size guard.
	if a.width > 0 && a.height > 0 && (a.width < minTermWidth || a.height < minTermHeight) {
		msg := fmt.Sprintf(
			"Terminal too small\n\nCurrent:  %d x %d\nMinimum:  %d x %d\n\nPlease resize your terminal.",
			a.width, a.height, minTermWidth, minTermHeight,
		)
		return lipgloss.Place(
			a.width, a.height,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().
				Foreground(t.Yellow).
				Background(t.Base).
				Bold(true).
				Align(lipgloss.Center).
				Render(msg),
			lipgloss.WithWhitespaceBackground(t.Base),
		)
	}

	// Update status bar with current focus label
	sb := a.statusBar.SetFocusLabel(keys.ContextLabel(keys.ActiveContext))

	// Set contextual hint bar extra message for active workflows
	hb := a.hintBar
	if sb.IsBisecting() {
		hb = hb.SetExtra("BISECTING  B:mark good/bad/skip")
	} else if sb.IsComparing() {
		hb = hb.SetExtra("Select a commit to compare")
	}
	hintBarView := hb.View()
	statusBar := sb.View()

	// Height available for the main view.
	pageHeight := a.height - lipgloss.Height(hintBarView) - lipgloss.Height(statusBar)
	if pageHeight < 0 {
		pageHeight = 0
	}

	pageView := a.mainView.View()
	pageView = lipgloss.NewStyle().
		Width(a.width).
		Height(pageHeight).
		Background(t.Base).
		Render(pageView)

	// Compose the layout: main view + hint bar + status bar.
	layout := lipgloss.NewStyle().
		Width(a.width).
		Height(a.height).
		Background(t.Base).
		Render(lipgloss.JoinVertical(lipgloss.Left, pageView, hintBarView, statusBar))

	// Overlay dialog if active — composite on top of the layout so the app
	// remains visible behind the dialog (same technique used by toasts).
	if a.showDialog && a.dialog != nil {
		dlg := a.dialog.View()
		dlgW := lipgloss.Width(dlg)
		dlgH := lipgloss.Height(dlg)
		posX := (a.width - dlgW) / 2
		posY := (a.height - dlgH) / 2
		if posX < 0 {
			posX = 0
		}
		if posY < 0 {
			posY = 0
		}
		layout = overlayAt(layout, dlg, posX, posY, a.width)
	}

	// Overlay toast if visible.
	if a.toast.Visible() {
		toastRendered := a.toast.View()
		if toastRendered != "" {
			toastW := lipgloss.Width(toastRendered)
			toastH := lipgloss.Height(toastRendered)
			posX := a.width - toastW - 1
			if posX < 0 {
				posX = 0
			}
			posY := a.height - toastH - 1
			if posY < 0 {
				posY = 0
			}
			layout = overlayAt(layout, toastRendered, posX, posY, a.width)
		}
	}

	return layout
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// pageHeight returns the height available for the main view.
func (a App) pageHeight() int {
	h := a.height - 2 // hint bar (1 line) + status bar (1 line)
	if h < 0 {
		return 0
	}
	return h
}

// loadBranchInfo returns a Cmd that refreshes the status bar with branch info.
func (a App) loadBranchInfo() tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		branch, _ := repo.Head()
		ahead, behind, _ := repo.AheadBehind()
		return branchInfoMsg{
			branch:    branch,
			ahead:     ahead,
			behind:    behind,
			bisecting: repo.IsBisecting(),
			rebasing:  repo.IsRebasing(),
		}
	}
}

// overlayAt composites |overlay| on top of |base| at character position (x, y).
func overlayAt(base, overlay string, x, y, _ int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for i, oLine := range overlayLines {
		row := y + i
		if row < 0 || row >= len(baseLines) {
			continue
		}

		bLine := baseLines[row]
		oWidth := lipgloss.Width(oLine)

		left := ansi.Truncate(bLine, x, "")
		right := ansi.TruncateLeft(bLine, x+oWidth, "")

		baseLines[row] = left + oLine + right
	}

	return strings.Join(baseLines, "\n")
}

// doCommit runs the commit asynchronously.
func (a App) doCommit(message string) tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		err := repo.Commit(message)
		return commitDoneMsg{err: err}
	}
}

// doCommitAmend runs an amend commit asynchronously.
func (a App) doCommitAmend(message string) tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		err := repo.CommitAmend(message)
		return commitDoneMsg{err: err}
	}
}

// doGitOp runs a push/pull/fetch operation asynchronously.
func (a App) doGitOp(op string, force bool) tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		var err error
		switch op {
		case "push":
			if force {
				err = repo.ForcePush("", "")
			} else {
				// Auto-detect missing upstream and set it.
				branch, brErr := repo.CurrentBranch()
				if brErr == nil && branch != "" && branch != "HEAD" {
					hasUp, _ := repo.HasUpstream(branch)
					if !hasUp {
						err = repo.PushSetUpstream("origin", branch)
						if err == nil {
							return gitOpDoneMsg{op: op, err: nil}
						}
					}
				}
				if err == nil {
					err = repo.Push("", "")
				}
			}
		case "pull":
			err = repo.Pull("", "")
		case "fetch":
			err = repo.Fetch()
		}
		return gitOpDoneMsg{op: op, err: err}
	}
}

// schedulePoll returns a Cmd that sends pollTickMsg after pollInterval.
func (a App) schedulePoll() tea.Cmd {
	return tea.Tick(pollInterval, func(_ time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

// checkFingerprint runs a lightweight git status check in the background.
func (a App) checkFingerprint() tea.Cmd {
	repo := a.repo
	prev := a.lastFingerprint
	return func() tea.Msg {
		fp := repo.StatusFingerprint()
		return pollResultMsg{
			fingerprint: fp,
			changed:     prev != "" && fp != prev,
		}
	}
}

// capitalize returns s with the first letter uppercased.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// friendlyGitError returns a user-friendly message for common git errors.
// If the error is not recognized, the original message is returned.
func friendlyGitError(op string, err error) string {
	msg := err.Error()
	lower := strings.ToLower(msg)

	switch op {
	case "push":
		switch {
		case strings.Contains(lower, "rejected"):
			return "Push rejected — remote has new commits. Pull first."
		case strings.Contains(lower, "no upstream"):
			return "No upstream branch configured."
		case strings.Contains(lower, "authentication") || strings.Contains(lower, "permission denied"):
			return "Authentication failed. Check your credentials."
		case strings.Contains(lower, "could not resolve host"):
			return "Cannot reach remote. Check your network connection."
		case strings.Contains(lower, "does not match any"):
			return "Branch not found on remote."
		}
	case "pull":
		switch {
		case strings.Contains(lower, "not possible because you have unmerged"):
			return "Pull failed — resolve merge conflicts first."
		case strings.Contains(lower, "uncommitted changes"):
			return "Pull failed — commit or stash your changes first."
		case strings.Contains(lower, "authentication") || strings.Contains(lower, "permission denied"):
			return "Authentication failed. Check your credentials."
		case strings.Contains(lower, "could not resolve host"):
			return "Cannot reach remote. Check your network connection."
		case strings.Contains(lower, "conflict"):
			return "Pull completed with merge conflicts. Resolve them to continue."
		}
	case "fetch":
		switch {
		case strings.Contains(lower, "authentication") || strings.Contains(lower, "permission denied"):
			return "Authentication failed. Check your credentials."
		case strings.Contains(lower, "could not resolve host"):
			return "Cannot reach remote. Check your network connection."
		}
	}
	return msg
}

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
			_ = config.Save(&cfgCopy)
			return nil
		})
	}

	return tea.Batch(cmds...)
}

// ---------------------------------------------------------------------------
// AI commit message generation
// ---------------------------------------------------------------------------

// generateAICommitMessage gathers the staged diff and sends it to the
// configured AI provider to generate a commit message. The result is
// delivered back as an AICommitResultMsg or AICommitErrorMsg.
func (a App) generateAICommitMessage() tea.Cmd {
	cfg := a.ctx.Config
	if cfg == nil {
		return func() tea.Msg {
			return pages.AICommitErrorMsg{Err: fmt.Errorf("no configuration loaded")}
		}
	}

	// Resolve API key: config/env > saved credentials.
	apiKey := ai.GetAPIKey(cfg.AI.Provider, cfg.AI.APIKey)
	if apiKey == "" && cfg.AI.Provider != "openai-compatible" {
		// No API key — open the AI Setup dialog for first-time configuration.
		// Reset aiGenerating on the LogPage since we're not actually generating.
		pctx := a.ctx
		return func() tea.Msg {
			return showDialogMsg{model: dialog.NewAISetup(pctx)}
		}
	}

	// Build provider config with resolved key.
	aiCfg := cfg.AI
	aiCfg.APIKey = apiKey

	repo := a.ctx.Repo
	isCopilot := cfg.AI.Provider == "copilot"
	return func() tea.Msg {
		// For Copilot, the saved credential is the GitHub OAuth token.
		// We need to exchange it for a short-lived Copilot bearer token
		// before each generation (the bearer token expires frequently).
		if isCopilot && aiCfg.APIKey != "" {
			exchCtx, exchCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer exchCancel()
			cpToken, err := ai.ExchangeForCopilotToken(exchCtx, aiCfg.APIKey)
			if err != nil {
				return pages.AICommitErrorMsg{Err: fmt.Errorf("copilot token refresh: %w", err)}
			}
			aiCfg.APIKey = cpToken.Token
		}

		// Get staged diff as raw text for the AI prompt.
		diff, err := repo.DiffStagedRaw()
		if err != nil {
			return pages.AICommitErrorMsg{Err: fmt.Errorf("get staged diff: %w", err)}
		}
		if strings.TrimSpace(diff) == "" {
			return pages.AICommitErrorMsg{Err: fmt.Errorf("no staged changes")}
		}

		// Get diff stat for file-level summary.
		stat, _ := repo.DiffStatStagedRaw()

		// Create provider and generate.
		provider, err := ai.NewProvider(&aiCfg)
		if err != nil {
			return pages.AICommitErrorMsg{Err: err}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		msg, err := provider.GenerateCommitMessage(ctx, diff, stat)
		if err != nil {
			return pages.AICommitErrorMsg{Err: err}
		}

		return pages.AICommitResultMsg{
			Summary:     msg.Summary,
			Description: msg.Description,
		}
	}
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
