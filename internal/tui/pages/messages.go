package pages

// This file defines message types that are shared between pages and the app
// shell. They are used for requesting dialogs, signaling state changes, and
// triggering git operations from any page or component.

// RequestCommitDialogMsg is emitted so the App shell can open the commit dialog.
type RequestCommitDialogMsg struct {
	StagedCount int
}

// RequestAmendDialogMsg is emitted so the App shell can open the commit dialog
// pre-filled with the previous commit message.
type RequestAmendDialogMsg struct {
	StagedCount int
}

// RequestConfirmMsg is emitted so the App shell can open a confirm dialog.
type RequestConfirmMsg struct {
	ID      string
	Title   string
	Message string
}

// RequestToastMsg asks the app shell to display a toast notification.
type RequestToastMsg struct {
	Message string
	IsError bool
}

// RequestGitOpMsg requests a push/pull/fetch operation from the app shell.
type RequestGitOpMsg struct {
	Op    string // "push", "pull", or "fetch"
	Force bool   // force push with --force-with-lease
}

// RequestTextInputMsg asks the app shell to display a text input dialog.
type RequestTextInputMsg struct {
	ID           string
	Title        string
	Placeholder  string
	InitialValue string
}

// RefreshStatusMsg tells all pages to reload their data (e.g., after a commit
// or external git change).
type RefreshStatusMsg struct{}

// StatusDirtyMsg is sent by pages to inform the app shell of dirty state.
type StatusDirtyMsg struct {
	Dirty bool
}

// CompareStateMsg notifies the app shell about compare mode state.
type CompareStateMsg struct {
	Active bool
	Hash   string // short hash of compare base
}

// RequestMenuMsg asks the app shell to display a menu dialog.
type RequestMenuMsg struct {
	ID      string
	Title   string
	Options []MenuOption
}

// MenuOption represents a single option in a menu dialog request.
type MenuOption struct {
	Label       string
	Description string
	Key         string
}

// RequestCustomCmdMenuMsg asks the app shell to show the custom commands menu
// filtered by the current context.
type RequestCustomCmdMenuMsg struct {
	Context string // "commit", "file", "branch", or "global"
	// Template variables for expanding command templates.
	Hash      string
	ShortHash string
	Branch    string
	Path      string
	Subject   string
	Author    string
}
