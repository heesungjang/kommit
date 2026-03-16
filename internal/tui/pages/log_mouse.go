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
			// Right panel has no scrollable diff; no-op for now
			l.focus = focusLogDetail
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
			// Right panel has no scrollable diff; no-op for now
			l.focus = focusLogDetail
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
