package tui

import (
	"strings"

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

// ---------------------------------------------------------------------------
// Page identifiers
// ---------------------------------------------------------------------------

// PageID uniquely identifies each top-level page/tab.
type PageID int

const (
	PageStatus PageID = iota
	PageLog
	PageBranches
	PageRemotes
	PageStash
	PagePRs
	PageCI
	PageWorkspace
)

// pageLabels maps page IDs to their display names shown in the tab bar.
var pageLabels = map[PageID]string{
	PageStatus:    "Status",
	PageLog:       "Log",
	PageBranches:  "Branches",
	PageRemotes:   "Remotes",
	PageStash:     "Stash",
	PagePRs:       "PRs",
	PageCI:        "CI",
	PageWorkspace: "Workspace",
}

// orderedPages defines the tab order.
var orderedPages = []PageID{
	PageStatus, PageLog, PageBranches, PageRemotes,
	PageStash, PagePRs, PageCI, PageWorkspace,
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// switchPageMsg requests a page change from within a sub-model.
type switchPageMsg struct{ page PageID }

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

// ---------------------------------------------------------------------------
// App model
// ---------------------------------------------------------------------------

// App is the root Bubble Tea model that manages page routing, the tab bar,
// the status bar, dialog overlays, and toast notifications.
type App struct {
	repo       *git.Repository
	activePage PageID
	pages      map[PageID]tea.Model // lazy-initialised page models
	tabs       components.Tabs
	statusBar  components.StatusBar
	keys       keys.GlobalKeys
	width      int
	height     int
	err        error

	// Dialog overlay (commit message, confirm, help, etc.)
	dialog     tea.Model
	showDialog bool

	// Toast notification
	toast components.Toast
}

// NewApp creates a new App rooted at the given repository.
func NewApp(repo *git.Repository) App {
	tabItems := make([]components.TabItem, len(orderedPages))
	for i, pid := range orderedPages {
		tabItems[i] = components.TabItem{
			Key:   string(rune('1' + i)),
			Label: pageLabels[pid],
		}
	}

	return App{
		repo:       repo,
		activePage: PageStatus,
		pages:      make(map[PageID]tea.Model),
		tabs:       components.NewTabsWithItems(tabItems),
		statusBar:  components.NewStatusBar(),
		keys:       keys.NewGlobalKeys(),
		toast:      components.NewToast(),
	}
}

// Init initialises the application — loads the first page and status bar info.
func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.ensurePage(PageStatus).Init(),
		a.loadBranchInfo(),
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
		a.tabs = a.tabs.SetSize(msg.Width)
		a.statusBar = a.statusBar.SetSize(msg.Width)
		a.toast = a.toast.SetWidth(msg.Width)
		// Propagate to active page so it can resize its panels.
		if page, ok := a.pages[a.activePage]; ok {
			updated, cmd := page.Update(msg)
			a.pages[a.activePage] = updated
			cmds = append(cmds, cmd)
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

	// -- Help dialog close ---------------------------------------------------
	case dialog.HelpCloseMsg:
		a.showDialog = false
		a.dialog = nil
		return a, nil

	// -- Confirm dialog result -----------------------------------------------
	case dialog.ConfirmResultMsg:
		a.showDialog = false
		a.dialog = nil
		// Delegate handling to the active page
		if page, ok := a.pages[a.activePage]; ok {
			updated, cmd := page.Update(msg)
			a.pages[a.activePage] = updated
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	// -- Commit dialog -------------------------------------------------------
	case dialog.CommitRequestMsg:
		a.showDialog = false
		a.dialog = nil
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
		// Refresh status page
		cmds = append(cmds, func() tea.Msg { return pages.RefreshStatusMsg{} })
		return a, tea.Batch(cmds...)

	// -- Commit dialog request from status page ------------------------------
	case pages.RequestCommitDialogMsg:
		dlg := dialog.NewCommitMsg(msg.StagedCount, a.width, a.height)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	// -- Confirm dialog request from pages -----------------------------------
	case pages.RequestConfirmMsg:
		dlg := dialog.NewConfirm(msg.ID, msg.Title, msg.Message, a.width, a.height)
		a.dialog = dlg
		a.showDialog = true
		return a, dlg.Init()

	// -- Page switch request -------------------------------------------------
	case switchPageMsg:
		return a.switchTo(msg.page)

	// -- Tab bar changed (from Tabs component) -------------------------------
	case components.TabChangedMsg:
		if msg.Index >= 0 && msg.Index < len(orderedPages) {
			return a.switchTo(orderedPages[msg.Index])
		}
		return a, nil

	// -- Branch info loaded --------------------------------------------------
	case branchInfoMsg:
		a.statusBar = a.statusBar.SetBranch(msg.branch).SetAheadBehind(msg.ahead, msg.behind)
		return a, nil

	// -- Refresh status after mutations --------------------------------------
	case pages.RefreshStatusMsg:
		if page, ok := a.pages[PageStatus]; ok {
			updated, cmd := page.Update(msg)
			a.pages[PageStatus] = updated
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, a.loadBranchInfo())
		return a, tea.Batch(cmds...)

	// -- Key events ----------------------------------------------------------
	case tea.KeyMsg:
		// Force quit is always available.
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
			return a, func() tea.Msg {
				return showDialogMsg{model: dialog.NewHelp(a.width, a.height)}
			}
		case key.Matches(msg, a.keys.Tab1):
			return a.switchTo(PageStatus)
		case key.Matches(msg, a.keys.Tab2):
			return a.switchTo(PageLog)
		case key.Matches(msg, a.keys.Tab3):
			return a.switchTo(PageBranches)
		case key.Matches(msg, a.keys.Tab4):
			return a.switchTo(PageRemotes)
		case key.Matches(msg, a.keys.Tab5):
			return a.switchTo(PageStash)
		case key.Matches(msg, a.keys.Tab6):
			return a.switchTo(PagePRs)
		case key.Matches(msg, a.keys.Tab7):
			return a.switchTo(PageCI)
		case key.Matches(msg, a.keys.Tab8):
			return a.switchTo(PageWorkspace)
		}
	}

	// -- Fallthrough: route to dialog or active page -------------------------
	if a.showDialog && a.dialog != nil {
		updated, cmd := a.dialog.Update(msg)
		a.dialog = updated
		cmds = append(cmds, cmd)
	} else if page, ok := a.pages[a.activePage]; ok {
		updated, cmd := page.Update(msg)
		a.pages[a.activePage] = updated
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

// View renders the full application layout:
//
//	tab bar  (1 line)
//	page     (remaining space)
//	toast    (optional overlay)
//	status   (1 line)
func (a App) View() string {
	t := theme.Active

	tabBar := a.tabs.View()
	statusBar := a.statusBar.View()

	// Height available for the page content.
	pageHeight := a.height - lipgloss.Height(tabBar) - lipgloss.Height(statusBar)
	if pageHeight < 0 {
		pageHeight = 0
	}

	// Render the active page.
	pageView := ""
	if page, ok := a.pages[a.activePage]; ok {
		pageView = page.View()
	}
	pageView = lipgloss.NewStyle().
		Width(a.width).
		Height(pageHeight).
		MaxHeight(pageHeight).
		Background(t.Base).
		Render(pageView)

	// Compose the layout.
	layout := lipgloss.JoinVertical(lipgloss.Left, tabBar, pageView, statusBar)

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

	// Overlay toast if visible — composited into layout, not appended.
	if a.toast.Visible() {
		toastRendered := a.toast.View()
		if toastRendered != "" {
			toastW := lipgloss.Width(toastRendered)
			toastH := lipgloss.Height(toastRendered)
			// Position at bottom-right, one row above the status bar.
			posX := a.width - toastW - 1
			if posX < 0 {
				posX = 0
			}
			posY := a.height - toastH - 1 // 1 row above status bar
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

// switchTo changes the active page, lazily initialising it if needed.
func (a App) switchTo(pid PageID) (tea.Model, tea.Cmd) {
	a.activePage = pid
	a.tabs = a.tabs.SetActive(int(pid))
	page := a.ensurePage(pid)
	cmd := page.Init()
	return a, cmd
}

// ensurePage returns the page model for pid, creating it if it doesn't exist.
func (a *App) ensurePage(pid PageID) tea.Model {
	if p, ok := a.pages[pid]; ok {
		return p
	}

	var page tea.Model
	switch pid {
	case PageStatus:
		page = pages.NewStatusPage(a.repo, a.width, a.pageHeight())
	case PageLog:
		page = pages.NewLogPage(a.repo, a.width, a.pageHeight())
	case PageBranches:
		page = pages.NewBranchesPage(a.repo, a.width, a.pageHeight())
	case PageRemotes:
		page = pages.NewRemotesPage(a.repo, a.width, a.pageHeight())
	case PageStash:
		page = pages.NewStashPage(a.repo, a.width, a.pageHeight())
	default:
		// Placeholder for pages not yet implemented (PRs, CI, Workspace).
		page = pages.NewPlaceholderPage(pageLabels[pid], a.width, a.pageHeight())
	}
	a.pages[pid] = page
	return page
}

// pageHeight returns the height available for page content.
func (a App) pageHeight() int {
	h := a.height - 2 // tab bar + status bar
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
// Both are newline-separated strings. Uses ANSI-aware truncation so that
// styled text with escape sequences is handled correctly.
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

		// Left portion of the base line (columns 0..x-1), ANSI-aware.
		left := ansi.Truncate(bLine, x, "")
		// Right portion of the base line (columns x+oWidth..end), ANSI-aware.
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
