// Package msgs defines message types shared across TUI packages
// (pages, dialog, app) to avoid import cycles.
package msgs

// ToastMsg asks the app shell to display a toast notification.
type ToastMsg struct {
	Message string
	IsError bool
}
