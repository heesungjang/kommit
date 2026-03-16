package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heesungjang/kommit/internal/ai"
	"github.com/heesungjang/kommit/internal/auth"
	"github.com/heesungjang/kommit/internal/config"
	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/components"
	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/customcmd"
	"github.com/heesungjang/kommit/internal/tui/dialog"
	"github.com/heesungjang/kommit/internal/tui/keys"
	"github.com/heesungjang/kommit/internal/tui/msgs"
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

// autoFetchInterval is how often the app silently fetches from remotes,
// keeping tracking refs (and ahead/behind counts) up to date.
const autoFetchInterval = 5 * time.Minute

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
	pendingRetryOp    string
	pendingRetryForce bool
}

// NewApp creates a new App rooted at the given repository.
// If repo is nil the app starts in workspace mode.
func NewApp(repo *git.Repository, cfg *config.Config) App {
	ctx := tuictx.New(cfg, repo)
	// Set the package-level Active theme so components that haven't been
	// migrated to use ctx.Theme yet continue to work.
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
		ctx.ActiveKeyContext = keys.ContextLog
		a.page = pageLog
		a.mainView = pages.NewLogPage(ctx, 80, 24)
	} else {
		// Workspace mode: no repo provided, show workspace overview.
		ctx.ActiveKeyContext = keys.ContextWorkspace
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
		a.scheduleAutoFetch(),
	)
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

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
		a.ctx.ActiveKeyContext = keys.ContextLog
		// Track in recent repos.
		a.ctx.Config.AddRecentRepo(msg.path)
		if err := config.Save(a.ctx.Config); err != nil {
			a.toast, _ = a.toast.ShowError("Failed to save config: " + err.Error())
		}
		// Reconstruct LogPage with the new repo.
		a.mainView = pages.NewLogPage(a.ctx, a.width, a.pageHeight())
		return a, tea.Batch(
			a.mainView.Init(),
			a.loadBranchInfo(),
			a.loadPullRequests(),
			a.schedulePoll(),
			a.scheduleAutoFetch(),
		)

	// -- Switch back to already-loaded repo view (from workspace) ------------
	case backToRepoMsg:
		if a.repo != nil && a.mainView != nil {
			a.page = pageLog
			a.ctx.ActiveKeyContext = keys.ContextLog
		}
		return a, nil

	// -- Switch to workspace view ---------------------------------------------
	case showWorkspaceMsg:
		a.page = pageWorkspace
		a.ctx.ActiveKeyContext = keys.ContextWorkspace
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

	case showCreatePRDialogMsg:
		d := dialog.NewCreatePR(msg.headBranch, msg.baseBranch, msg.remoteBranches, msg.pctx)
		a.dialog = d
		a.showDialog = true
		return a, tea.Batch(d.Init(), a.loadPRStats(msg.baseBranch), a.checkBranchPushed())

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
				if err := ai.SetAPIKey(provider, apiKey); err != nil {
					return msgs.ToastMsg{Message: "Failed to save API key: " + err.Error(), IsError: true}
				}
			}
			// Save provider change to config.
			if cfg != nil {
				cfgCopy := *cfg
				if err := config.Save(&cfgCopy); err != nil {
					return msgs.ToastMsg{Message: "Failed to save config: " + err.Error(), IsError: true}
				}
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
			if err := ai.SetAPIKey("copilot", ghToken); err != nil {
				return msgs.ToastMsg{Message: "Failed to save Copilot token: " + err.Error(), IsError: true}
			}
			if cfg != nil {
				cfgCopy := *cfg
				if err := config.Save(&cfgCopy); err != nil {
					return msgs.ToastMsg{Message: "Failed to save config: " + err.Error(), IsError: true}
				}
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
			return pages.AICommitErrorMsg{Err: fmt.Errorf("copilot login: %s", errMsg)}
		}

	case dialog.CopilotOAuthCancelMsg:
		a.showDialog = false
		a.dialog = nil
		return a, func() tea.Msg {
			return pages.AICommitErrorMsg{Err: fmt.Errorf("copilot login canceled")}
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
			if err := auth.RemoveAccount(host); err != nil {
				return msgs.ToastMsg{Message: "Failed to remove account: " + err.Error(), IsError: true}
			}
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
			force := a.pendingRetryForce
			a.pendingRetryOp = ""
			a.pendingRetryForce = false
			var toastCmd tea.Cmd
			a.toast, toastCmd = a.toast.ShowSuccess("Logged in as @" + acct.Username + " — retrying " + op + "...")
			return a, tea.Batch(toastCmd, a.doGitOp(op, force))
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
			a.pendingRetryForce = false
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
			// Auto-prompt login on auth failure when no account exists.
			// Check auth errors BEFORE push-rejected, because permission/auth
			// errors can also contain "failed to push some refs".
			if isAuthError(msg.err) && !a.hasAccountForOrigin() {
				provider := a.detectOriginProvider()
				if provider != "" {
					a.pendingRetryOp = msg.op
					a.pendingRetryForce = msg.force
					dlg := dialog.NewAccountLoginForProvider(provider, a.ctx)
					a.dialog = dlg
					a.showDialog = true
					return a, dlg.Init()
				}
			}

			// Smart force push: if a normal push was rejected, offer force push.
			if msg.op == "push" && !msg.force && isPushRejected(msg.err) {
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

	case dialog.CreatePRStatsMsg, dialog.CreatePRBranchPushedMsg, dialog.CreatePRPushDoneMsg:
		if a.dialog != nil {
			var cmd tea.Cmd
			a.dialog, cmd = a.dialog.Update(msg)
			return a, cmd
		}
		return a, nil

	case dialog.CreatePRRefreshStatsMsg:
		return a, a.loadPRStats(msg.BaseBranch)

	case dialog.CreatePRPushRequestMsg:
		return a, a.pushHeadBranch()

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

	// -- Toast request (from pages or dialogs) --------------------------------
	case msgs.ToastMsg:
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
			if err := config.Save(cfg); err != nil {
				var toastCmd tea.Cmd
				a.toast, toastCmd = a.toast.ShowError("Workspace created but failed to save: " + err.Error())
				return a, toastCmd
			}
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
			if err := config.Save(cfg); err != nil {
				var toastCmd tea.Cmd
				a.toast, toastCmd = a.toast.ShowError("Failed to save config: " + err.Error())
				return a, toastCmd
			}
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
			if err := config.Save(cfg); err != nil {
				var toastCmd tea.Cmd
				a.toast, toastCmd = a.toast.ShowError("Failed to save config: " + err.Error())
				return a, toastCmd
			}
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
			if err := config.Save(cfg); err != nil {
				var toastCmd tea.Cmd
				a.toast, toastCmd = a.toast.ShowError("Failed to save config: " + err.Error())
				return a, toastCmd
			}
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
				if err := config.Save(cfg); err != nil {
					var toastCmd tea.Cmd
					a.toast, toastCmd = a.toast.ShowError("Failed to save config: " + err.Error())
					return a, toastCmd
				}
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

	// -- Background auto-fetch -----------------------------------------------
	case autoFetchTickMsg:
		return a, a.doBackgroundFetch()

	case autoFetchDoneMsg:
		fetchCmds := []tea.Cmd{a.scheduleAutoFetch()}
		if msg.err == nil {
			// Refresh ahead/behind counts now that tracking refs are updated.
			fetchCmds = append(fetchCmds, a.loadBranchInfo())
		}
		return a, tea.Batch(fetchCmds...)

	// -- Mouse events --------------------------------------------------------
	case tea.MouseMsg:
		if a.showDialog && a.dialog != nil {
			return a, nil
		}

		// Ignore clicks on chrome areas (action bar at top, hint bar + status bar at bottom).
		actionBarH := 1
		if msg.Y < actionBarH || msg.Y >= a.height-2 {
			return a, nil
		}

		// Make Y page-relative so downstream handlers don't need to know about chrome.
		msg.Y -= actionBarH

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
						a.ctx.ActiveKeyContext = keys.ContextLog
					}
					return a, nil
				}
				// Switch to workspace view.
				return a, a.switchToWorkspace()
			case key.Matches(msg, a.keys.Help):
				kctx := a.ctx.ActiveKeyContext
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

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View renders the full application layout:
//
//	main view  (all remaining space)
//	toast      (optional overlay)
//	status bar (1 line)
func (a App) View() string {
	t := a.ctx.Theme

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
	sb := a.statusBar.SetFocusLabel(keys.ContextLabel(a.ctx.ActiveKeyContext))

	// Set contextual hint bar extra message for active workflows
	hb := a.hintBar
	if sb.IsBisecting() {
		hb = hb.SetExtra("BISECTING  B:mark good/bad/skip")
	} else if sb.IsComparing() {
		hb = hb.SetExtra("Select a commit to compare")
	}
	actionBarView := a.actionBar.SetContext(a.ctx.ActiveKeyContext).View()
	hintBarView := hb.SetKeyContext(a.ctx.ActiveKeyContext).View()
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
