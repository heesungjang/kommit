package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/nicholascross/opengit/internal/config"
	"github.com/nicholascross/opengit/internal/git"
	"github.com/nicholascross/opengit/internal/tui/components"
	tuictx "github.com/nicholascross/opengit/internal/tui/context"
	"github.com/nicholascross/opengit/internal/tui/dialog"
	"github.com/nicholascross/opengit/internal/tui/keys"
	"github.com/nicholascross/opengit/internal/tui/pages"
	"github.com/nicholascross/opengit/internal/tui/theme"
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
	toolbar   components.Toolbar
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
		toolbar:   components.NewToolbar(),
		statusBar: components.NewStatusBar(),
		keys:      keys.NewGlobalKeys(),
		toast:     components.NewToast(),
	}
}

// Init initialises the application.
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
		a.ctx.SetScreenSize(msg.Width, msg.Height, 2) // 2 = toolbar + statusbar
		a.toolbar = a.toolbar.SetWidth(msg.Width)
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

	// -- Help dialog close ---------------------------------------------------
	case dialog.HelpCloseMsg:
		a.showDialog = false
		a.dialog = nil
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
		if msg.Amend {
			return a, a.doCommitAmend(msg.Message)
		}
		return a, a.doCommit(msg.Message)

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

	toolBar := a.toolbar.View()
	statusBar := a.statusBar.View()

	// Height available for the main view.
	pageHeight := a.height - lipgloss.Height(toolBar) - lipgloss.Height(statusBar)
	if pageHeight < 0 {
		pageHeight = 0
	}

	pageView := a.mainView.View()
	pageView = lipgloss.NewStyle().
		Width(a.width).
		Height(pageHeight).
		Background(t.Base).
		Render(pageView)

	// Compose the layout: main view + toolbar + status bar.
	layout := lipgloss.NewStyle().
		Width(a.width).
		Height(a.height).
		Background(t.Base).
		Render(lipgloss.JoinVertical(lipgloss.Left, pageView, toolBar, statusBar))

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
	h := a.height - 2 // toolbar (1 line) + status bar (1 line)
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
func overlayAt(base, overlay string, x, y, totalWidth int) string {
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
