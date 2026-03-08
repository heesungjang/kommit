package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/nicholascross/opengit/internal/git"
	"github.com/nicholascross/opengit/internal/tui/components"
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
	branch string
	ahead  int
	behind int
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
	repo      *git.Repository
	mainView  tea.Model // the LogPage — our single unified view
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
func NewApp(repo *git.Repository) App {
	keys.ActiveContext = keys.ContextLog
	return App{
		repo:      repo,
		mainView:  pages.NewLogPage(repo, 80, 24), // initial size; updated on WindowSizeMsg
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
		dlg := dialog.NewCommitMsg(msg.StagedCount, a.width, a.height)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	// -- Confirm dialog request ----------------------------------------------
	case pages.RequestConfirmMsg:
		dlg := dialog.NewConfirm(msg.ID, msg.Title, msg.Message, a.width, a.height)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	// -- Amend commit dialog request -----------------------------------------
	case pages.RequestAmendDialogMsg:
		repo := a.repo
		stagedCount := msg.StagedCount
		w, h := a.width, a.height
		return a, func() tea.Msg {
			info, err := repo.LastCommit()
			if err != nil || info == nil {
				return showDialogMsg{model: dialog.NewCommitMsg(stagedCount, w, h)}
			}
			prevMsg := info.Subject
			if info.Body != "" {
				prevMsg = info.Subject + "\n\n" + info.Body
			}
			return showDialogMsg{model: dialog.NewCommitMsgAmend(stagedCount, prevMsg, w, h)}
		}

	// -- Git push/pull/fetch request -----------------------------------------
	case pages.RequestGitOpMsg:
		var toastCmd tea.Cmd
		a.toast, toastCmd = a.toast.ShowInfo(capitalize(msg.Op) + "ing...")
		return a, tea.Batch(toastCmd, a.doGitOp(msg.Op))

	case gitOpDoneMsg:
		if msg.err != nil {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.ShowError(capitalize(msg.op) + " failed: " + msg.err.Error())
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
		dlg := dialog.NewTextInput(msg.ID, msg.Title, msg.Placeholder, msg.InitialValue, a.width, a.height)
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
		a.statusBar = a.statusBar.SetBranch(msg.branch).SetAheadBehind(msg.ahead, msg.behind)
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

		// Global shortcuts.
		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.Help):
			ctx := keys.ActiveContext
			return a, func() tea.Msg {
				return showDialogMsg{model: dialog.NewHelp(ctx, a.width, a.height)}
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

	statusBar := a.statusBar.View()

	// Height available for the main view.
	pageHeight := a.height - lipgloss.Height(statusBar)
	if pageHeight < 0 {
		pageHeight = 0
	}

	pageView := a.mainView.View()
	pageView = lipgloss.NewStyle().
		Width(a.width).
		Height(pageHeight).
		Background(t.Base).
		Render(pageView)

	// Compose the layout: main view + status bar (no tab bar).
	layout := lipgloss.JoinVertical(lipgloss.Left, pageView, statusBar)

	// Overlay dialog if active.
	if a.showDialog && a.dialog != nil {
		dlg := a.dialog.View()
		layout = lipgloss.Place(
			a.width, a.height,
			lipgloss.Center, lipgloss.Center,
			dlg,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceBackground(t.Base),
		)
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
	h := a.height - 1 // status bar only (no tab bar)
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
		return branchInfoMsg{branch: branch, ahead: ahead, behind: behind}
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
func (a App) doGitOp(op string) tea.Cmd {
	repo := a.repo
	return func() tea.Msg {
		var err error
		switch op {
		case "push":
			err = repo.Push("", "")
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
