package pages

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/heesungjang/kommit/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// Mouse handling
// ---------------------------------------------------------------------------

func (l LogPage) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	bw := styles.PanelBorderWidth
	pw := l.panelLayout()

	// Compute panel boundaries (matching View layout).
	sidebarEnd := pw.sidebar + bw // sidebar outer width
	centerEnd := sidebarEnd + pw.center + bw

	// Determine which zone the mouse is in.
	type zone int
	const (
		zoneSidebar zone = iota
		zoneCenter
		zoneRight
	)
	var z zone
	if msg.X < sidebarEnd {
		z = zoneSidebar
	} else if msg.X < centerEnd {
		z = zoneCenter
	} else {
		z = zoneRight
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		switch z {
		case zoneSidebar:
			l.focus = focusSidebar
			// Let sidebar handle scroll via key up equivalent
		case zoneCenter:
			if l.diffViewer.Active {
				// Scroll center diff up
				l.diffViewer.ScrollY -= 3
				if l.diffViewer.ScrollY < 0 {
					l.diffViewer.ScrollY = 0
				}
			} else if l.cursor > 0 {
				l.cursor--
				l.focus = focusLogList
				return l, l.loadDetailForCursor()
			}
		case zoneRight:
			l.focus = focusLogDetail
			l.scrollRightPanel(-3)
		}
		return l, nil

	case tea.MouseButtonWheelDown:
		switch z {
		case zoneSidebar:
			l.focus = focusSidebar
		case zoneCenter:
			if l.diffViewer.Active {
				// Scroll center diff down
				l.diffViewer.ScrollY += 3
				maxScroll := len(l.diffViewer.Lines) - 10
				if maxScroll < 0 {
					maxScroll = 0
				}
				if l.diffViewer.ScrollY > maxScroll {
					l.diffViewer.ScrollY = maxScroll
				}
			} else if l.cursor < len(l.commits)-1 {
				l.cursor++
				l.focus = focusLogList
				return l, l.loadDetailForCursor()
			}
		case zoneRight:
			l.focus = focusLogDetail
			l.scrollRightPanel(3)
		}
		return l, nil

	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return l, nil
		}
		switch z {
		case zoneSidebar:
			l.focus = focusSidebar
		case zoneCenter:
			l.focus = focusLogList
			l.viewingStash = false
			l.viewingPR = false
			l.viewedPR = nil
			itemY := msg.Y - 3 // border(1) + title(1) + gap(1)
			if itemY >= 0 {
				ph := l.height - styles.PanelBorderHeight
				visibleCount := ph - 4 // title(1) + gap(1) + emptyLine(1) + hints(1)
				if visibleCount < 1 {
					visibleCount = 1
				}
				offset := 0
				if l.cursor >= visibleCount {
					offset = l.cursor - visibleCount + 1
				}
				if offset > len(l.commits)-visibleCount {
					offset = len(l.commits) - visibleCount
				}
				if offset < 0 {
					offset = 0
				}
				clickedIdx := offset + itemY
				if clickedIdx >= 0 && clickedIdx < len(l.commits) {
					l.cursor = clickedIdx
					return l, l.loadDetailForCursor()
				}
			}
		case zoneRight:
			l.focus = focusLogDetail
			if l.isWIPSelected() && !l.viewingStash {
				return l.handleWIPMouseClick(msg, centerEnd)
			}
		}
		return l, nil
	}

	return l, nil
}

// scrollRightPanel scrolls the right panel content by delta lines.
// Positive delta scrolls down, negative scrolls up.
// It handles different right-panel modes: WIP file lists, commit detail tabs,
// stash diff, and PR detail.
func (l *LogPage) scrollRightPanel(delta int) {
	// WIP panel: scroll the focused file list
	if l.isWIPSelected() && !l.viewingStash && !l.viewingPR {
		switch l.wipFocus {
		case wipFocusUnstaged:
			if len(l.wipUnstaged) > 0 {
				l.wipUnstagedCursor += delta
				if l.wipUnstagedCursor < 0 {
					l.wipUnstagedCursor = 0
				}
				if l.wipUnstagedCursor >= len(l.wipUnstaged) {
					l.wipUnstagedCursor = len(l.wipUnstaged) - 1
				}
			}
		case wipFocusStaged:
			if len(l.wipStaged) > 0 {
				l.wipStagedCursor += delta
				if l.wipStagedCursor < 0 {
					l.wipStagedCursor = 0
				}
				if l.wipStagedCursor >= len(l.wipStaged) {
					l.wipStagedCursor = len(l.wipStaged) - 1
				}
			}
		}
		return
	}

	// Commit detail: scroll the detail tab content
	if l.detailCommit != nil {
		switch l.detailTab {
		case 0: // Files tab — scroll file cursor
			if len(l.detailFiles) > 0 {
				l.detailFileCursor += delta
				if l.detailFileCursor < 0 {
					l.detailFileCursor = 0
				}
				if l.detailFileCursor >= len(l.detailFiles) {
					l.detailFileCursor = len(l.detailFiles) - 1
				}
			}
		case 1, 2: // Message or Stats tab — scroll text content
			l.detailTabScroll += delta
			if l.detailTabScroll < 0 {
				l.detailTabScroll = 0
			}
		}
	}
}
