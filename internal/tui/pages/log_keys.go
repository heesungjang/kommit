package pages

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/heesungjang/kommit/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (l LogPage) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When search input is active, route keys to it.
	if l.searching {
		return l.handleSearchKeys(msg)
	}

	// When commit message editor is active, route ALL keys directly to it.
	// This prevents q/p/1/2/3/etc. from triggering global actions while typing.
	if l.IsEditing() {
		return l.handleWIPCommitKeys(msg)
	}

	isTab := key.Matches(msg, key.NewBinding(key.WithKeys("tab")))
	isShiftTab := key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab")))

	// Exit fullscreen diff mode when switching panels.
	if l.diffFullscreen && (isTab || isShiftTab) {
		l.diffFullscreen = false
		return l, nil
	}

	if isTab || isShiftTab {
		// When WIP is selected and we're in the detail panel, let the WIP
		// sub-focus system handle tab internally (unstaged -> staged -> then next panel).
		if l.focus == focusLogDetail && l.isWIPSelected() {
			return l.handleWIPDetailKeys(msg)
		}

		// Three-panel cycle: sidebar -> center -> right -> sidebar
		if isTab {
			switch l.focus {
			case focusSidebar:
				l.focus = focusLogList
			case focusLogList:
				l.focus = focusLogDetail
				// When entering WIP detail, set initial sub-focus.
				if l.isWIPSelected() {
					if len(l.wipUnstaged) == 0 && len(l.wipStaged) > 0 {
						l.wipFocus = wipFocusStaged
					} else {
						l.wipFocus = wipFocusUnstaged
					}
				}
			case focusLogDetail:
				l.focus = focusSidebar
			}
		} else { // shift+tab
			switch l.focus {
			case focusSidebar:
				l.focus = focusLogDetail
				if l.isWIPSelected() {
					// Land on the commit area (last sub-panel in cycle)
					// Selected but not editing — user can press Enter to start typing
					l.wipFocus = wipFocusCommit
					l.commitSummary.Blur()
					l.commitDesc.Blur()
					l.commitEditing = false
				}
			case focusLogList:
				l.focus = focusSidebar
			case focusLogDetail:
				l.focus = focusLogList
			}
		}
		return l, nil
	}

	// Direct panel focus keys: 1 = sidebar, 2 = center, 3 = right
	// Exit fullscreen diff mode when using direct panel focus keys.
	if l.diffFullscreen {
		if key.Matches(msg, key.NewBinding(key.WithKeys("1", "3"))) {
			l.diffFullscreen = false
		}
	}
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("1"))):
		l.focus = focusSidebar
		return l, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("2"))):
		l.focus = focusLogList
		return l, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("3"))):
		l.focus = focusLogDetail
		if l.isWIPSelected() {
			if len(l.wipUnstaged) == 0 && len(l.wipStaged) > 0 {
				l.wipFocus = wipFocusStaged
			} else {
				l.wipFocus = wipFocusUnstaged
			}
		}
		return l, nil
	}

	// When selecting a commit in the list, clear stash/PR view mode
	if l.focus == focusLogList {
		l.viewingStash = false
		l.viewingPR = false
		l.viewedPR = nil
	}

	// Global push/pull/fetch — available when center or right panel is focused
	// AND when we're NOT on a non-WIP commit in the center panel (to avoid
	// conflicting with commit ops keys like f=fixup, p=push, etc.).
	// When sidebar is focused, let it handle p/a/etc. contextually.
	// When the diff viewer is active, let it handle f=fullscreen, etc.
	if l.focus != focusSidebar && !l.diffViewer.Active && (l.focus != focusLogList || l.isWIPSelected() || len(l.commits) == 0) {
		switch {
		case key.Matches(msg, l.remoteKeys.Push):
			return l, func() tea.Msg { return RequestGitOpMsg{Op: "push"} }
		case key.Matches(msg, l.remoteKeys.ForcePush):
			return l, func() tea.Msg { return RequestGitOpMsg{Op: "push", Force: true} }
		case key.Matches(msg, l.remoteKeys.Pull):
			return l, func() tea.Msg { return RequestGitOpMsg{Op: "pull"} }
		case key.Matches(msg, l.remoteKeys.Fetch):
			return l, func() tea.Msg { return RequestGitOpMsg{Op: "fetch"} }
		}
	}

	// Search — available from sidebar and commit list (not detail panel, not in diff mode)
	if key.Matches(msg, key.NewBinding(key.WithKeys("/"))) && !l.diffViewer.Active {
		if l.focus == focusSidebar || l.focus == focusLogList {
			l.searching = true
			l.searchPanel = l.focus
			// Set context-aware placeholder, width, and pre-fill with existing filter
			pw := l.panelLayout()
			switch l.focus {
			case focusSidebar:
				l.searchInput.Placeholder = "filter branches, tags..."
				l.searchInput.SetValue(l.sidebar.Filter())
				l.searchInput.Width = pw.sidebar - styles.PanelPaddingWidth - 2 // -2 for prompt
			case focusLogList:
				l.searchInput.Placeholder = "search commits..."
				l.searchInput.SetValue(l.commitFilterQuery)
				l.searchInput.Width = pw.center - styles.PanelPaddingWidth - 2
			}
			l.searchInput.CursorEnd()
			l.searchInput.Focus()
			return l, l.searchInput.Focus()
		}
	}

	switch l.focus {
	case focusSidebar:
		return l.handleSidebarKeys(msg)
	case focusLogList:
		return l.handleListKeys(msg)
	case focusLogDetail:
		return l.handleDetailKeys(msg)
	}
	return l, nil
}

func (l LogPage) handleSidebarKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	l.sidebar, cmd = l.sidebar.Update(msg)
	return l, cmd
}

// IsSearching returns true when the search input is active.
func (l LogPage) IsSearching() bool {
	return l.searching
}

// handleSearchKeys handles keyboard input while the search bar is active.
func (l LogPage) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		// Cancel search — close input and clear filter for this panel
		l.searching = false
		l.searchInput.Blur()
		switch l.searchPanel {
		case focusLogList:
			if l.commitFilterQuery != "" {
				l.commitFilterQuery = ""
				return l, l.loadLog() // reload unfiltered
			}
		case focusSidebar:
			if l.sidebar.Filter() != "" {
				l.sidebar = l.sidebar.ClearFilter()
			}
		}
		return l, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		// Apply search
		l.searching = false
		l.searchInput.Blur()
		query := l.searchInput.Value()
		if query == "" {
			// Empty query — clear filter for this panel
			switch l.searchPanel {
			case focusLogList:
				l.commitFilterQuery = ""
				return l, l.loadLog()
			case focusSidebar:
				l.sidebar = l.sidebar.ClearFilter()
			}
			return l, nil
		}
		// Apply filter depending on which panel initiated search
		switch l.searchPanel {
		case focusLogList:
			l.commitFilterQuery = query
			return l, l.loadLogFiltered(query)
		case focusSidebar:
			l.sidebar = l.sidebar.SetFilter(query)
			return l, nil
		}
		return l, nil
	}

	// Forward all other keys to the text input
	var cmd tea.Cmd
	l.searchInput, cmd = l.searchInput.Update(msg)
	return l, cmd
}
