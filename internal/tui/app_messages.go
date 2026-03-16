package tui

import tea "github.com/charmbracelet/bubbletea"

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
	op    string
	force bool
	err   error
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

// autoFetchTickMsg triggers a silent background fetch from remotes.
type autoFetchTickMsg struct{}

// autoFetchDoneMsg carries the result of a background fetch.
type autoFetchDoneMsg struct {
	err error
}
