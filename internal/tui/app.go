package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/heesungjang/kommit/internal/ai"
	"github.com/heesungjang/kommit/internal/auth"
	"github.com/heesungjang/kommit/internal/config"
	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/hosting"
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

// stashDoneMsg is sent when a stash save/pop operation completes.
type stashDoneMsg struct {
	op  string // "save" or "pop"
	err error
}

// customCmdDoneMsg is sent when a custom command finishes execution.
type customCmdDoneMsg struct {
	name   string
	output string
	err    error
	show   bool // whether to show output in a toast
}

// accountsChangedMsg is sent after an account login or logout to refresh state.
type accountsChangedMsg struct{}

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

// activePage tracks which page is currently shown.
type activePage int

const (
	pageLog       activePage = iota // normal repo view
	pageWorkspace                   // workspace overview
)

// App is the root Bubble Tea model. It renders a single unified view
// (sidebar | commit graph | context detail) plus a status bar and overlays.
type App struct {
	ctx       *tuictx.ProgramContext // shared context (dimensions, theme, config, repo)
	repo      *git.Repository
	mainView  tea.Model // the LogPage — our single unified view
	actionBar components.ActionBar
	hintBar   components.HintBar
	statusBar components.StatusBar
	keys      keys.GlobalKeys
	width     int
	height    int

	// Page management
	page          activePage
	workspacePage tea.Model // the WorkspacePage — workspace overview

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

	// Pending push error — held while the force-push retry dialog is shown.
	pendingPushErr error

	// Pending retry op — the git operation to retry after a successful login.
	pendingRetryOp string
}

// NewApp creates a new App rooted at the given repository.
// If repo is nil the app starts in workspace mode.
func NewApp(repo *git.Repository, cfg *config.Config) App {
	ctx := tuictx.New(cfg, repo)
	// Set the package-level Active theme so existing code that reads
	// theme.Active directly continues to work during the migration.
	theme.Active = ctx.Theme

	// Apply user keybinding overrides from config.
	if len(cfg.Keybinds.Custom) > 0 {
		keys.ApplyOverrides(cfg.Keybinds.Custom)
	}

	a := App{
		ctx:       ctx,
		repo:      repo,
		actionBar: components.NewActionBar(),
		hintBar:   components.NewHintBar(),
		statusBar: components.NewStatusBar(),
		keys:      keys.NewGlobalKeys(),
		toast:     components.NewToast(),
	}

	// Always create the workspace page so it's available for switching.
	wsPage := pages.NewWorkspacePage(ctx, 80, 24)
	a.workspacePage = wsPage

	if repo != nil {
		// Normal mode: open directly to repo view.
		keys.ActiveContext = keys.ContextLog
		a.page = pageLog
		a.mainView = pages.NewLogPage(ctx, 80, 24)
	} else {
		// Workspace mode: no repo provided, show workspace overview.
		keys.ActiveContext = keys.ContextWorkspace
		a.page = pageWorkspace
		// mainView stays nil — we use workspacePage in this mode.
	}

	return a
}

// Init initializes the application.
func (a App) Init() tea.Cmd {
	// Resolve logged-in username for the action bar.
	a.actionBar = a.actionBar.SetUsername(a.resolveUsername())

	if a.page == pageWorkspace {
		return a.workspacePage.Init()
	}

	return tea.Batch(
		a.mainView.Init(),
		a.loadBranchInfo(),
		a.loadPullRequests(),
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
		chromeHeight := 3 // action bar + hint bar + status bar
		a.ctx.SetScreenSize(msg.Width, msg.Height, chromeHeight)
		a.actionBar = a.actionBar.SetWidth(msg.Width)
		a.hintBar = a.hintBar.SetWidth(msg.Width)
		a.statusBar = a.statusBar.SetSize(msg.Width)
		a.toast = a.toast.SetWidth(msg.Width)
		// Propagate to active page with adjusted height (minus chrome).
		pageMsg := tea.WindowSizeMsg{
			Width:  msg.Width,
			Height: a.pageHeight(),
		}
		if a.page == pageWorkspace && a.workspacePage != nil {
			var cmd tea.Cmd
			a.workspacePage, cmd = a.workspacePage.Update(pageMsg)
			cmds = append(cmds, cmd)
		}
		if a.mainView != nil {
			var cmd tea.Cmd
			a.mainView, cmd = a.mainView.Update(pageMsg)
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	// -- Switch to repo (async result) ----------------------------------------
	case switchToRepoMsg:
		a.repo = msg.repo
		a.ctx.Repo = msg.repo
		a.page = pageLog
		keys.ActiveContext = keys.ContextLog
		// Track in recent repos.
		a.ctx.Config.AddRecentRepo(msg.path)
		_ = config.Save(a.ctx.Config)
		// Reconstruct LogPage with the new repo.
		a.mainView = pages.NewLogPage(a.ctx, a.width, a.pageHeight())
		return a, tea.Batch(
			a.mainView.Init(),
			a.loadBranchInfo(),
			a.loadPullRequests(),
			a.schedulePoll(),
		)

	// -- Switch back to already-loaded repo view (from workspace) ------------
	case backToRepoMsg:
		if a.repo != nil && a.mainView != nil {
			a.page = pageLog
			keys.ActiveContext = keys.ContextLog
		}
		return a, nil

	// -- Switch to workspace view ---------------------------------------------
	case showWorkspaceMsg:
		a.page = pageWorkspace
		keys.ActiveContext = keys.ContextWorkspace
		if wp, ok := a.workspacePage.(interface{ Sync() }); ok {
			wp.Sync()
		}
		// Resize workspace page to current dimensions.
		if a.workspacePage != nil {
			var cmd tea.Cmd
			a.workspacePage, cmd = a.workspacePage.Update(tea.WindowSizeMsg{
				Width:  a.width,
				Height: a.pageHeight(),
			})
			cmds = append(cmds, cmd, a.workspacePage.Init())
		}
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

	// -- Account login dialog -----------------------------------------------
	case dialog.RequestAccountLoginMsg:
		// Open the account login dialog (from settings or auto-prompt).
		// Keep the settings dialog state so we return to it after login.
		provider := msg.Provider
		pctx := a.ctx
		dlg := dialog.NewAccountLoginForProvider(provider, pctx)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	case dialog.RequestAccountLogoutMsg:
		// Remove the account and refresh.
		host := msg.Host
		return a, func() tea.Msg {
			_ = auth.RemoveAccount(host)
			return accountsChangedMsg{}
		}

	case dialog.AccountLoginResultMsg:
		a.showDialog = false
		a.dialog = nil
		acct := msg.Account
		a.actionBar = a.actionBar.SetUsername(acct.Username)

		// If there's a pending retry op (auto-prompt from auth failure), retry it.
		if a.pendingRetryOp != "" {
			op := a.pendingRetryOp
			a.pendingRetryOp = ""
			var toastCmd tea.Cmd
			a.toast, toastCmd = a.toast.ShowSuccess("Logged in as @" + acct.Username + " — retrying " + op + "...")
			return a, tea.Batch(toastCmd, a.doGitOp(op, false))
		}

		return a, func() tea.Msg {
			return accountsChangedMsg{}
		}

	case dialog.AccountLoginCancelMsg:
		a.showDialog = false
		a.dialog = nil
		// If there was a pending retry op, show the original error.
		if a.pendingRetryOp != "" {
			op := a.pendingRetryOp
			a.pendingRetryOp = ""
			var toastCmd tea.Cmd
			a.toast, toastCmd = a.toast.ShowError(capitalize(op) + " failed: authentication required")
			return a, toastCmd
		}
		return a, nil

	case accountsChangedMsg:
		// Refresh action bar username and re-open settings if it was open.
		a.actionBar = a.actionBar.SetUsername(a.resolveUsername())
		return a, a.openSettings()

	// -- Settings change (theme swap, toggles, etc.) -------------------------
	case dialog.SettingsChangeMsg:
		return a, a.applySettingsChange(msg)

	// -- Settings updated — forward to main view even while dialog is open --
	case pages.SettingsUpdatedMsg:
		if a.mainView != nil {
			var cmd tea.Cmd
			a.mainView, cmd = a.mainView.Update(msg)
			return a, cmd
		}
		return a, nil

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
		// Handle workspace confirmations (delete workspace, remove repo).
		if strings.HasPrefix(msg.ID, "workspace-delete-") && msg.Confirmed {
			idxStr := strings.TrimPrefix(msg.ID, "workspace-delete-")
			idx := 0
			_, _ = fmt.Sscanf(idxStr, "%d", &idx)
			return a, func() tea.Msg {
				return pages.RequestDeleteWorkspaceMsg{WorkspaceIndex: idx}
			}
		}
		if strings.HasPrefix(msg.ID, "workspace-removerepo-") && msg.Confirmed {
			idxStr := strings.TrimPrefix(msg.ID, "workspace-removerepo-")
			var wsIdx, rIdx int
			_, _ = fmt.Sscanf(idxStr, "%d-%d", &wsIdx, &rIdx)
			return a, func() tea.Msg {
				return pages.RequestRemoveRepoFromWorkspaceMsg{WorkspaceIndex: wsIdx, RepoIndex: rIdx}
			}
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
		// Handle stash save confirmation.
		if msg.ID == "stash-save" {
			if msg.Confirmed {
				repo := a.repo
				var toastCmd tea.Cmd
				a.toast, toastCmd = a.toast.ShowInfo("Stashing changes...")
				return a, tea.Batch(toastCmd, func() tea.Msg {
					err := repo.StashSave("")
					return stashDoneMsg{op: "save", err: err}
				})
			}
			return a, nil
		}
		// Handle stash pop confirmation.
		if msg.ID == "stash-pop" {
			if msg.Confirmed {
				repo := a.repo
				var toastCmd tea.Cmd
				a.toast, toastCmd = a.toast.ShowInfo("Popping stash...")
				return a, tea.Batch(toastCmd, func() tea.Msg {
					err := repo.StashPop(0)
					return stashDoneMsg{op: "pop", err: err}
				})
			}
			return a, nil
		}
		// Handle force-push retry dialog (smart force push after rejection).
		if msg.ID == "gitop-force-push-retry" {
			if msg.Confirmed {
				a.pendingPushErr = nil
				var toastCmd tea.Cmd
				a.toast, toastCmd = a.toast.ShowInfo("Force pushing...")
				return a, tea.Batch(toastCmd, a.doGitOp("push", true))
			}
			// User declined force push — show the original rejection as toast.
			origErr := a.pendingPushErr
			a.pendingPushErr = nil
			if origErr != nil {
				var cmd tea.Cmd
				a.toast, cmd = a.toast.ShowError("Push failed: " + friendlyGitError("push", origErr))
				return a, cmd
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
		if a.mainView != nil {
			var cmd tea.Cmd
			a.mainView, cmd = a.mainView.Update(msg)
			cmds = append(cmds, cmd)
		}
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
		if a.mainView != nil {
			var cmd tea.Cmd
			a.mainView, cmd = a.mainView.Update(msg)
			cmds = append(cmds, cmd)
		}
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
			// Smart force push: if a normal push was rejected, offer force push.
			if msg.op == "push" && isPushRejected(msg.err) {
				a.pendingPushErr = msg.err
				dlg := dialog.NewConfirm(
					"gitop-force-push-retry",
					"Push Rejected",
					"Remote has new commits.\nForce push with --force-with-lease?",
					a.ctx,
				)
				a.dialog = dlg
				a.showDialog = true
				cmds = append(cmds, dlg.Init())
				cmds = append(cmds, func() tea.Msg { return pages.RefreshStatusMsg{} })
				return a, tea.Batch(cmds...)
			}

			// Auto-prompt login on auth failure when no account exists.
			if isAuthError(msg.err) && !a.hasAccountForOrigin() {
				provider := a.detectOriginProvider()
				if provider != "" {
					a.pendingRetryOp = msg.op
					dlg := dialog.NewAccountLoginForProvider(provider, a.ctx)
					a.dialog = dlg
					a.showDialog = true
					return a, dlg.Init()
				}
			}

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

	// -- Stash operations (WIP panel — confirm before executing) -------------
	case pages.RequestStashSaveMsg:
		dlg := dialog.NewConfirm("stash-save", "Stash?", "Stash all uncommitted changes?", a.ctx)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	case pages.RequestStashPopMsg:
		dlg := dialog.NewConfirm("stash-pop", "Pop Stash?", "Apply and remove the latest stash entry?", a.ctx)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	case stashDoneMsg:
		label := "Stash " + msg.op
		if msg.err != nil {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.ShowError(label + " failed: " + msg.err.Error())
			cmds = append(cmds, cmd)
		} else {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.ShowSuccess(label + " complete")
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

	// -- Text input result — close dialog, route appropriately ---------------
	case dialog.TextInputResultMsg:
		a.showDialog = false
		a.dialog = nil
		// Handle workspace-related text input results.
		if strings.HasPrefix(msg.ID, "workspace-") {
			return a, a.handleWorkspaceTextInput(msg.ID, msg.Value)
		}
		if a.mainView != nil {
			var cmd tea.Cmd
			a.mainView, cmd = a.mainView.Update(msg)
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	// -- Text input cancel — close dialog, route appropriately ---------------
	case dialog.TextInputCancelMsg:
		a.showDialog = false
		a.dialog = nil
		if a.mainView != nil {
			var cmd tea.Cmd
			a.mainView, cmd = a.mainView.Update(msg)
			cmds = append(cmds, cmd)
		}
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
		if a.mainView != nil {
			var cmd tea.Cmd
			a.mainView, cmd = a.mainView.Update(msg)
			return a, cmd
		}
		return a, nil

	case pages.AICommitErrorMsg:
		if a.mainView != nil {
			var cmd tea.Cmd
			a.mainView, cmd = a.mainView.Update(msg)
			return a, cmd
		}
		return a, nil

	// -- Create PR request from sidebar/command palette ----------------------
	case pages.RequestCreatePRMsg:
		return a, a.openCreatePRDialog()

	case dialog.CreatePRSubmitMsg:
		a.showDialog = false
		a.dialog = nil
		return a, a.createPullRequest(msg)

	case dialog.CreatePRCancelMsg:
		a.showDialog = false
		a.dialog = nil
		return a, nil

	case dialog.CreatePRRequestAIMsg:
		return a, a.generatePRDescription(msg.BaseBranch)

	case dialog.CreatePRAIResultMsg:
		if a.dialog != nil {
			var cmd tea.Cmd
			a.dialog, cmd = a.dialog.Update(msg)
			return a, cmd
		}
		return a, nil

	case dialog.CreatePRAIErrorMsg:
		if a.dialog != nil {
			var cmd tea.Cmd
			a.dialog, cmd = a.dialog.Update(msg)
			return a, cmd
		}
		return a, nil

	case pages.PRCreatedMsg:
		var cmd tea.Cmd
		a.toast, cmd = a.toast.ShowSuccess(fmt.Sprintf("PR #%d created", msg.Number))
		cmds = append(cmds, cmd, a.loadPullRequests())
		return a, tea.Batch(cmds...)

	case pages.PRCreateErrorMsg:
		var cmd tea.Cmd
		a.toast, cmd = a.toast.ShowError("Create PR failed: " + msg.Err.Error())
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
		a.actionBar = a.actionBar.SetAheadBehind(msg.ahead, msg.behind)
		return a, nil

	// -- Refresh after mutations ---------------------------------------------
	case pages.RefreshStatusMsg:
		if a.mainView != nil {
			var cmd tea.Cmd
			a.mainView, cmd = a.mainView.Update(msg)
			cmds = append(cmds, cmd, a.loadBranchInfo(), a.loadPullRequests())
		}
		return a, tea.Batch(cmds...)

	// -- Workspace: open a repo -----------------------------------------------
	case pages.RequestOpenRepoMsg:
		return a, a.switchToRepo(msg.Path)

	// -- Workspace: show workspace page ---------------------------------------
	case pages.RequestShowWorkspaceMsg:
		return a, a.switchToWorkspace()

	// -- Workspace: new workspace (from text input result) --------------------
	case pages.RequestNewWorkspaceMsg:
		if msg.Name != "" {
			cfg := a.ctx.Config
			cfg.Workspaces = append(cfg.Workspaces, config.WorkspaceEntry{Name: msg.Name})
			_ = config.Save(cfg)
			if wp, ok := a.workspacePage.(interface{ Sync() }); ok {
				wp.Sync()
			}
			var toastCmd tea.Cmd
			a.toast, toastCmd = a.toast.ShowSuccess("Workspace \"" + msg.Name + "\" created")
			return a, toastCmd
		}
		return a, nil

	// -- Workspace: delete workspace ------------------------------------------
	case pages.RequestDeleteWorkspaceMsg:
		cfg := a.ctx.Config
		idx := msg.WorkspaceIndex
		if idx >= 0 && idx < len(cfg.Workspaces) {
			name := cfg.Workspaces[idx].Name
			cfg.Workspaces = append(cfg.Workspaces[:idx], cfg.Workspaces[idx+1:]...)
			_ = config.Save(cfg)
			if wp, ok := a.workspacePage.(interface{ Sync() }); ok {
				wp.Sync()
			}
			var toastCmd tea.Cmd
			a.toast, toastCmd = a.toast.ShowSuccess("Workspace \"" + name + "\" deleted")
			return a, toastCmd
		}
		return a, nil

	// -- Workspace: rename workspace ------------------------------------------
	case pages.RequestRenameWorkspaceMsg:
		cfg := a.ctx.Config
		idx := msg.WorkspaceIndex
		if idx >= 0 && idx < len(cfg.Workspaces) && msg.NewName != "" {
			cfg.Workspaces[idx].Name = msg.NewName
			_ = config.Save(cfg)
			if wp, ok := a.workspacePage.(interface{ Sync() }); ok {
				wp.Sync()
			}
		}
		return a, nil

	// -- Workspace: add repo to workspace ------------------------------------
	case pages.RequestAddRepoToWorkspaceMsg:
		cfg := a.ctx.Config
		idx := msg.WorkspaceIndex
		if idx >= 0 && idx < len(cfg.Workspaces) && msg.RepoPath != "" {
			// Expand ~ in the path.
			repoPath := expandPath(msg.RepoPath)
			// Validate it's a git repo.
			if _, err := git.Open(repoPath); err != nil {
				var toastCmd tea.Cmd
				a.toast, toastCmd = a.toast.ShowError("Not a git repository: " + repoPath)
				return a, toastCmd
			}
			// Check for duplicate.
			for _, existing := range cfg.Workspaces[idx].Repos {
				if existing == repoPath {
					var toastCmd tea.Cmd
					a.toast, toastCmd = a.toast.ShowError("Repository already in workspace")
					return a, toastCmd
				}
			}
			cfg.Workspaces[idx].Repos = append(cfg.Workspaces[idx].Repos, repoPath)
			_ = config.Save(cfg)
			if wp, ok := a.workspacePage.(interface{ Sync() }); ok {
				wp.Sync()
			}
			var toastCmd tea.Cmd
			a.toast, toastCmd = a.toast.ShowSuccess("Added to workspace")
			return a, toastCmd
		}
		return a, nil

	// -- Workspace: remove repo from workspace --------------------------------
	case pages.RequestRemoveRepoFromWorkspaceMsg:
		cfg := a.ctx.Config
		wsIdx := msg.WorkspaceIndex
		rIdx := msg.RepoIndex
		if wsIdx >= 0 && wsIdx < len(cfg.Workspaces) {
			repos := cfg.Workspaces[wsIdx].Repos
			if rIdx >= 0 && rIdx < len(repos) {
				repos = append(repos[:rIdx], repos[rIdx+1:]...)
				cfg.Workspaces[wsIdx].Repos = repos
				_ = config.Save(cfg)
				if wp, ok := a.workspacePage.(interface{ Sync() }); ok {
					wp.Sync()
				}
			}
		}
		return a, nil

	// -- Workspace: fetch all repos in a workspace ----------------------------
	case pages.RequestWorkspaceFetchAllMsg:
		cfg := a.ctx.Config
		idx := msg.WorkspaceIndex
		if idx >= 0 && idx < len(cfg.Workspaces) {
			var toastCmd tea.Cmd
			a.toast, toastCmd = a.toast.ShowInfo("Fetching all repos...")
			return a, tea.Batch(toastCmd, a.doWorkspaceBulkOp("fetch", cfg.Workspaces[idx].Repos))
		}
		return a, nil

	// -- Workspace: pull all repos in a workspace -----------------------------
	case pages.RequestWorkspacePullAllMsg:
		cfg := a.ctx.Config
		idx := msg.WorkspaceIndex
		if idx >= 0 && idx < len(cfg.Workspaces) {
			var toastCmd tea.Cmd
			a.toast, toastCmd = a.toast.ShowInfo("Pulling all repos...")
			return a, tea.Batch(toastCmd, a.doWorkspaceBulkOp("pull", cfg.Workspaces[idx].Repos))
		}
		return a, nil

	// -- Workspace: bulk op done ----------------------------------------------
	case pages.WorkspaceBulkOpDoneMsg:
		if msg.Failed > 0 {
			var toastCmd tea.Cmd
			a.toast, toastCmd = a.toast.ShowError(fmt.Sprintf("%s: %d/%d failed", capitalize(msg.Op), msg.Failed, msg.Total))
			cmds = append(cmds, toastCmd)
		} else {
			var toastCmd tea.Cmd
			a.toast, toastCmd = a.toast.ShowSuccess(fmt.Sprintf("%s: %d repos done", capitalize(msg.Op), msg.Total))
			cmds = append(cmds, toastCmd)
		}
		// Forward to workspace page to refresh statuses.
		if a.page == pageWorkspace && a.workspacePage != nil {
			var cmd tea.Cmd
			a.workspacePage, cmd = a.workspacePage.Update(msg)
			cmds = append(cmds, cmd)
		}
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

		// Route mouse events to the active page.
		if a.page == pageWorkspace && a.workspacePage != nil {
			var cmd tea.Cmd
			a.workspacePage, cmd = a.workspacePage.Update(msg)
			cmds = append(cmds, cmd)
		} else if a.mainView != nil {
			var cmd tea.Cmd
			a.mainView, cmd = a.mainView.Update(msg)
			cmds = append(cmds, cmd)
		}
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
				// In workspace mode, quit directly without confirmation.
				if a.page == pageWorkspace && a.repo == nil {
					return a, tea.Quit
				}
				dlg := dialog.NewConfirm("quit", "Quit?", "Are you sure you want to quit?", a.ctx)
				a.dialog = dlg
				a.showDialog = true
				return a, dlg.Init()
			case key.Matches(msg, a.keys.ToggleWorkspace):
				if a.page == pageWorkspace {
					// Switch back to repo view if we have a repo loaded.
					if a.repo != nil {
						a.page = pageLog
						keys.ActiveContext = keys.ContextLog
					}
					return a, nil
				}
				// Switch to workspace view.
				return a, a.switchToWorkspace()
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

	// -- Fallthrough: route to dialog or active page --------------------------
	if a.showDialog && a.dialog != nil {
		updated, cmd := a.dialog.Update(msg)
		a.dialog = updated
		cmds = append(cmds, cmd)
	} else if a.page == pageWorkspace && a.workspacePage != nil {
		var cmd tea.Cmd
		a.workspacePage, cmd = a.workspacePage.Update(msg)
		cmds = append(cmds, cmd)
	} else if a.mainView != nil {
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
	actionBarView := a.actionBar.SetContext(keys.ActiveContext).View()
	hintBarView := hb.View()
	statusBar := sb.View()

	// Height available for the main view.
	pageHeight := a.height - lipgloss.Height(actionBarView) - lipgloss.Height(hintBarView) - lipgloss.Height(statusBar)
	if pageHeight < 0 {
		pageHeight = 0
	}

	// Render the active page.
	var pageView string
	if a.page == pageWorkspace && a.workspacePage != nil {
		pageView = a.workspacePage.View()
	} else if a.mainView != nil {
		pageView = a.mainView.View()
	}
	pageView = lipgloss.NewStyle().
		Width(a.width).
		Height(pageHeight).
		Background(t.Base).
		Render(pageView)

	// Compose the layout: action bar + main view + hint bar + status bar.
	layout := lipgloss.NewStyle().
		Width(a.width).
		Height(a.height).
		Background(t.Base).
		Render(lipgloss.JoinVertical(lipgloss.Left, actionBarView, pageView, hintBarView, statusBar))

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
	h := a.height - 3 // action bar (1 line) + hint bar (1 line) + status bar (1 line)
	if h < 0 {
		return 0
	}
	return h
}

// loadBranchInfo returns a Cmd that refreshes the status bar with branch info.
func (a App) loadBranchInfo() tea.Cmd {
	repo := a.repo
	if repo == nil {
		return nil
	}
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
// It detects the overlay's background color and re-injects it after every SGR
// reset within the overlay lines, preventing transparent cells from letting the
// base content bleed through.
func overlayAt(base, overlay string, x, y, _ int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Detect the overlay's background color SGR from the rendered content.
	bgSGR := extractBgSGR(overlay)

	for i, oLine := range overlayLines {
		row := y + i
		if row < 0 || row >= len(baseLines) {
			continue
		}

		bLine := baseLines[row]
		oWidth := lipgloss.Width(oLine)

		left := ansi.Truncate(bLine, x, "")
		right := ansi.TruncateLeft(bLine, x+oWidth, "")

		// Fill background on every printable cell that lacks one so
		// transparent cells don't let the base content bleed through.
		if bgSGR != "" {
			oLine = fillBgCells(oLine, bgSGR)
		}

		baseLines[row] = left + "\033[0m" + bgSGR + oLine + right
	}

	return strings.Join(baseLines, "\n")
}

// extractBgSGR scans an ANSI string for the first background color SGR
// parameter and returns it as a standalone SGR sequence (e.g. "\033[48;2;49;50;68m").
// Returns "" if no background color is found.
func extractBgSGR(s string) string {
	for i := 0; i < len(s); i++ {
		// Look for ESC [ ... m sequences (CSI SGR).
		if s[i] != '\033' || i+1 >= len(s) || s[i+1] != '[' {
			continue
		}
		// Found ESC[, now collect params until 'm' or a non-param byte.
		j := i + 2
		for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == ';') {
			j++
		}
		if j >= len(s) || s[j] != 'm' {
			i = j
			continue
		}
		// s[i+2:j] is the params string, s[j] == 'm'.
		params := s[i+2 : j]
		if bg := extractBgFromParams(params); bg != "" {
			return "\033[" + bg + "m"
		}
		i = j
	}
	return ""
}

// extractBgFromParams extracts the background color portion from a
// semicolon-separated SGR parameter string. Returns the background params
// (e.g. "48;2;49;50;68") or "" if no background is present.
func extractBgFromParams(params string) string {
	parts := strings.Split(params, ";")
	for idx := 0; idx < len(parts); idx++ {
		p := parts[idx]
		// Basic 3-bit / 4-bit background: 40-47 (not 48=extended, not 49=default)
		if len(p) == 2 && p[0] == '4' && p[1] >= '0' && p[1] <= '7' {
			return p
		}
		if len(p) == 3 && p[0] == '1' && p[1] == '0' && p[2] >= '0' && p[2] <= '7' {
			return p
		}
		// Extended background: 48;5;N or 48;2;R;G;B
		if p == "48" && idx+1 < len(parts) {
			if parts[idx+1] == "5" && idx+2 < len(parts) {
				// 256-color: 48;5;N
				return parts[idx] + ";" + parts[idx+1] + ";" + parts[idx+2]
			}
			if parts[idx+1] == "2" && idx+4 < len(parts) {
				// True color: 48;2;R;G;B
				return parts[idx] + ";" + parts[idx+1] + ";" + parts[idx+2] + ";" + parts[idx+3] + ";" + parts[idx+4]
			}
		}
	}
	return ""
}

// fillBgCells walks an ANSI string and ensures every printable cell has the
// given background color active. It tracks the current SGR state and injects
// bgSGR before any run of printable characters that lacks an explicit
// background. This handles all transparent cells — not just those after resets.
func fillBgCells(line, bgSGR string) string {
	var buf strings.Builder
	buf.Grow(len(line) + 256)

	hasBg := false
	i := 0
	for i < len(line) {
		// Check for CSI sequence: ESC [
		if line[i] == '\033' && i+1 < len(line) && line[i+1] == '[' {
			// Collect parameter bytes (0x30-0x3F) and intermediate bytes (0x20-0x2F)
			// until a final byte (0x40-0x7E).
			j := i + 2
			for j < len(line) && ((line[j] >= '0' && line[j] <= '?') || (line[j] >= ' ' && line[j] <= '/')) {
				j++
			}
			if j < len(line) {
				seq := line[i : j+1]
				if line[j] == 'm' {
					// SGR sequence — update background tracking state.
					hasBg = updateBgState(line[i+2:j], hasBg)
				}
				buf.WriteString(seq)
				i = j + 1
				continue
			}
			// Unterminated sequence — copy ESC and continue.
			buf.WriteByte(line[i])
			i++
			continue
		}

		// Skip other ESC sequences (OSC, etc.) — copy through.
		if line[i] == '\033' {
			buf.WriteByte(line[i])
			i++
			continue
		}

		// Control characters — copy through without bg injection.
		if line[i] < ' ' {
			buf.WriteByte(line[i])
			i++
			continue
		}

		// Printable character — inject bgSGR if no background is active.
		if !hasBg {
			buf.WriteString(bgSGR)
			hasBg = true
		}
		buf.WriteByte(line[i])
		i++
	}
	return buf.String()
}

// updateBgState parses a semicolon-separated SGR parameter string and returns
// whether a background color is active after applying the parameters.
func updateBgState(params string, hasBg bool) bool {
	// Empty params means implicit reset ("\033[m").
	if params == "" {
		return false
	}

	parts := strings.Split(params, ";")
	for idx := 0; idx < len(parts); idx++ {
		p := parts[idx]
		switch {
		case p == "0" || p == "":
			// Explicit reset or empty sub-param — clears all attributes.
			hasBg = false
		case p == "49":
			// Default background color — clears background.
			hasBg = false
		case len(p) == 2 && p[0] == '4' && p[1] >= '0' && p[1] <= '7':
			// Basic background: 40-47.
			hasBg = true
		case p == "48":
			// Extended background (48;5;N or 48;2;R;G;B) — skip sub-params.
			hasBg = true
			if idx+1 < len(parts) && parts[idx+1] == "5" {
				idx += 2 // skip 5;N
			} else if idx+1 < len(parts) && parts[idx+1] == "2" {
				idx += 4 // skip 2;R;G;B
			}
		case len(p) == 3 && p[0] == '1' && p[1] == '0' && p[2] >= '0' && p[2] <= '7':
			// Bright background: 100-107.
			hasBg = true
		}
		// All other params (foreground, bold, etc.) don't affect bg state.
	}
	return hasBg
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

	// Resolve credentials for the origin remote.
	gitUser, gitToken := a.resolveGitCredentials()

	return func() tea.Msg {
		var err error
		switch op {
		case "push":
			if force {
				if gitUser != "" {
					err = repo.ForcePushAuth("", "", gitUser, gitToken)
				} else {
					err = repo.ForcePush("", "")
				}
			} else {
				// Auto-detect missing upstream and set it.
				branch, brErr := repo.CurrentBranch()
				if brErr == nil && branch != "" && branch != "HEAD" {
					hasUp, _ := repo.HasUpstream(branch)
					if !hasUp {
						if gitUser != "" {
							err = repo.PushSetUpstreamAuth("origin", branch, gitUser, gitToken)
						} else {
							err = repo.PushSetUpstream("origin", branch)
						}
						if err == nil {
							return gitOpDoneMsg{op: op, err: nil}
						}
					}
				}
				if err == nil {
					if gitUser != "" {
						err = repo.PushAuth("", "", gitUser, gitToken)
					} else {
						err = repo.Push("", "")
					}
				}
			}
		case "pull":
			if gitUser != "" {
				err = repo.PullAuth("", "", gitUser, gitToken)
			} else {
				err = repo.Pull("", "")
			}
		case "fetch":
			if gitUser != "" {
				err = repo.FetchAuth(gitUser, gitToken)
			} else {
				err = repo.Fetch()
			}
		}
		return gitOpDoneMsg{op: op, err: err}
	}
}

// resolveGitCredentials looks up the saved account matching the current repo's
// origin remote and returns the git username and token for authentication.
// Returns empty strings if no matching account is found.
func (a App) resolveGitCredentials() (username, token string) {
	if a.repo == nil {
		return "", ""
	}
	remoteURL, err := a.repo.RemoteURL("origin")
	if err != nil || remoteURL == "" {
		return "", ""
	}
	acct := auth.AccountForRemote(remoteURL)
	if acct == nil || acct.Token == "" {
		return "", ""
	}
	return acct.GitUser, acct.Token
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
	if repo == nil {
		return nil
	}
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

// isPushRejected returns true if the push error indicates a non-fast-forward
// rejection, which means force push could resolve the issue.
func isPushRejected(err error) bool {
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "rejected") ||
		strings.Contains(lower, "non-fast-forward") ||
		strings.Contains(lower, "failed to push some refs")
}

// isAuthError returns true if a git error indicates an authentication failure.
func isAuthError(err error) bool {
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "permission denied") ||
		strings.Contains(lower, "could not read username") ||
		strings.Contains(lower, "invalid credentials") ||
		strings.Contains(lower, "401") ||
		strings.Contains(lower, "403")
}

// hasAccountForOrigin returns true if there's a saved account matching the
// current repo's origin remote.
func (a App) hasAccountForOrigin() bool {
	if a.repo == nil {
		return false
	}
	remoteURL, err := a.repo.RemoteURL("origin")
	if err != nil || remoteURL == "" {
		return false
	}
	return auth.AccountForRemote(remoteURL) != nil
}

// detectOriginProvider detects the hosting provider for the origin remote.
func (a App) detectOriginProvider() auth.HostProvider {
	if a.repo == nil {
		return ""
	}
	remoteURL, err := a.repo.RemoteURL("origin")
	if err != nil || remoteURL == "" {
		return ""
	}
	host := auth.HostFromRemoteURL(remoteURL)
	return auth.ProviderForHost(host)
}

// ---------------------------------------------------------------------------
// Accounts
// ---------------------------------------------------------------------------

// resolveUsername returns the username for the hosting provider matching the
// current repo's origin remote. Returns empty string if no account matches.
func (a App) resolveUsername() string {
	if a.repo == nil {
		return ""
	}
	remoteURL, err := a.repo.RemoteURL("origin")
	if err != nil || remoteURL == "" {
		return ""
	}
	acct := auth.AccountForRemote(remoteURL)
	if acct == nil {
		return ""
	}
	return acct.Username
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
// Pull Requests
// ---------------------------------------------------------------------------

// loadPullRequests fetches open PRs from the hosting provider in the background.
// It checks the origin remote, resolves the hosting account, and calls the API.
// The result is a SidebarPRsLoadedMsg which the LogPage routes to the sidebar.
func (a App) loadPullRequests() tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		if repo == nil {
			return pages.SidebarPRsLoadedMsg{}
		}

		remoteURL, err := repo.RemoteURL("origin")
		if err != nil || remoteURL == "" {
			return pages.SidebarPRsLoadedMsg{}
		}

		// Only GitHub is supported for now.
		host := auth.HostFromRemoteURL(remoteURL)
		provider := auth.ProviderForHost(host)
		if provider != auth.ProviderGitHub {
			return pages.SidebarPRsLoadedMsg{}
		}

		acct := auth.GetAccount(host)
		if acct == nil {
			return pages.SidebarPRsLoadedMsg{}
		}

		ref, err := hosting.RepoRefFromRemoteURL(remoteURL)
		if err != nil {
			return pages.SidebarPRsLoadedMsg{Err: err}
		}

		client := hosting.NewGitHubClient(acct.Token)
		prs, err := client.ListPullRequests(ref, "open")
		return pages.SidebarPRsLoadedMsg{PRs: prs, Err: err}
	}
}

// openCreatePRDialog opens the Create PR dialog pre-filled with the current
// branch and the repository's default branch.
func (a App) openCreatePRDialog() tea.Cmd {
	repo := a.repo
	pctx := a.ctx
	return func() tea.Msg {
		headBranch, err := repo.Head()
		if err != nil {
			return pages.PRCreateErrorMsg{Err: fmt.Errorf("cannot determine current branch: %w", err)}
		}

		// Determine the default branch (target for the PR).
		baseBranch := "main" // fallback
		remoteURL, err := repo.RemoteURL("origin")
		if err == nil && remoteURL != "" {
			host := auth.HostFromRemoteURL(remoteURL)
			acct := auth.GetAccount(host)
			if acct != nil {
				ref, refErr := hosting.RepoRefFromRemoteURL(remoteURL)
				if refErr == nil {
					client := hosting.NewGitHubClient(acct.Token)
					if defaultBranch, dbErr := client.GetDefaultBranch(ref); dbErr == nil {
						baseBranch = defaultBranch
					}
				}
			}
		}

		return showDialogMsg{model: dialog.NewCreatePR(headBranch, baseBranch, pctx)}
	}
}

// createPullRequest calls the GitHub API to create a pull request.
func (a App) createPullRequest(msg dialog.CreatePRSubmitMsg) tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		remoteURL, err := repo.RemoteURL("origin")
		if err != nil || remoteURL == "" {
			return pages.PRCreateErrorMsg{Err: fmt.Errorf("no origin remote configured")}
		}

		host := auth.HostFromRemoteURL(remoteURL)
		acct := auth.GetAccount(host)
		if acct == nil {
			return pages.PRCreateErrorMsg{Err: fmt.Errorf("not logged in — log in via Settings > Accounts")}
		}

		ref, err := hosting.RepoRefFromRemoteURL(remoteURL)
		if err != nil {
			return pages.PRCreateErrorMsg{Err: err}
		}

		client := hosting.NewGitHubClient(acct.Token)
		pr, err := client.CreatePullRequest(ref, hosting.CreatePRRequest{
			Title: msg.Title,
			Body:  msg.Body,
			Head:  msg.Head,
			Base:  msg.Base,
			Draft: msg.Draft,
		})
		if err != nil {
			return pages.PRCreateErrorMsg{Err: err}
		}

		return pages.PRCreatedMsg{Number: pr.Number, URL: pr.URL}
	}
}

// ---------------------------------------------------------------------------
// Workspace helpers
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

// generatePRDescription uses the AI provider to generate a PR title and body
// from the branch diff against the given base branch.
func (a App) generatePRDescription(baseBranch string) tea.Cmd {
	cfg := a.ctx.Config
	repo := a.repo
	isCopilot := cfg != nil && cfg.AI.Provider == "copilot"

	return func() tea.Msg {
		if cfg == nil {
			return dialog.CreatePRAIErrorMsg{Err: fmt.Errorf("no configuration loaded")}
		}

		apiKey := ai.GetAPIKey(cfg.AI.Provider, cfg.AI.APIKey)
		if apiKey == "" && cfg.AI.Provider != "openai-compatible" {
			return dialog.CreatePRAIErrorMsg{Err: fmt.Errorf("no AI provider configured — set up in Settings > AI")}
		}

		// For Copilot, exchange for bearer token.
		aiCfg := cfg.AI
		aiCfg.APIKey = apiKey
		if isCopilot && aiCfg.APIKey != "" {
			exchCtx, exchCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer exchCancel()
			cpToken, err := ai.ExchangeForCopilotToken(exchCtx, aiCfg.APIKey)
			if err != nil {
				return dialog.CreatePRAIErrorMsg{Err: fmt.Errorf("copilot token refresh: %w", err)}
			}
			aiCfg.APIKey = cpToken.Token
		}

		// Get branch diff.
		diff, err := repo.DiffBranchRaw(baseBranch)
		if err != nil {
			return dialog.CreatePRAIErrorMsg{Err: fmt.Errorf("get branch diff: %w", err)}
		}
		if strings.TrimSpace(diff) == "" {
			return dialog.CreatePRAIErrorMsg{Err: fmt.Errorf("no changes between %s and HEAD", baseBranch)}
		}

		stat, _ := repo.DiffStatBranchRaw(baseBranch)
		commitLog, _ := repo.LogBranchOneline(baseBranch)

		genCtx, genCancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer genCancel()

		desc, err := ai.GeneratePRDescription(genCtx, &aiCfg, aiCfg.APIKey, diff, stat, commitLog)
		if err != nil {
			return dialog.CreatePRAIErrorMsg{Err: err}
		}

		return dialog.CreatePRAIResultMsg{Title: desc.Title, Body: desc.Body}
	}
}
