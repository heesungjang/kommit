package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/anim"
	"github.com/heesungjang/kommit/internal/tui/dialog"
	"github.com/heesungjang/kommit/internal/tui/styles"
	"github.com/heesungjang/kommit/internal/tui/theme"
	"github.com/heesungjang/kommit/internal/tui/utils"
)

// ---------------------------------------------------------------------------
// Commit list — key handlers, render, commands, commit operations
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// logLoadedMsg carries the result of loading the commit log.
type logLoadedMsg struct {
	commits   []git.CommitInfo
	graphRows []git.GraphRow
	hasWIP    bool // true if a synthetic WIP entry was prepended
	err       error
}

// logMoreLoadedMsg carries additional commits loaded via pagination.
type logMoreLoadedMsg struct {
	commits   []git.CommitInfo
	graphRows []git.GraphRow // graph for the FULL combined list
	err       error
}

// commitDetailMsg carries the result of loading a single commit's detail.
type commitDetailMsg struct {
	commit git.CommitInfo
	diff   *git.DiffResult
	err    error
}

// centerDiffMsg carries a loaded diff to display in the center panel.
type centerDiffMsg struct {
	path     string
	diff     string
	err      error
	isWIP    bool // true when this diff is from WIP context (staged/unstaged)
	isStaged bool // within WIP, whether this is a staged diff
}

// commitOpDoneMsg is sent after a revert or cherry-pick completes successfully.
type commitOpDoneMsg struct {
	op string // "revert" or "cherry-pick"
}

// undoTargetMsg carries the undo target hash back to the UI thread so it can
// be stored in pendingUndoHash before showing the confirm dialog.
type undoTargetMsg struct {
	hash      string
	shortHash string
	message   string
}

// redoTargetMsg carries the redo target hash back to the UI thread so it can
// be stored in pendingRedoHash before showing the confirm dialog.
type redoTargetMsg struct {
	hash      string
	shortHash string
	message   string
}

// safeResetMsg triggers a stash-bracketed hard reset: stash uncommitted
// changes, reset --hard, then pop stash. This preserves working directory
// changes while performing undo/redo operations.
type safeResetMsg struct {
	hash  string
	short string
	op    string // "Undo" or "Redo"
}

// ---------------------------------------------------------------------------
// Key handlers
// ---------------------------------------------------------------------------

// loadDetailForCursor loads either WIP or commit detail depending on cursor position.
func (l LogPage) loadDetailForCursor() tea.Cmd {
	if l.isWIPSelected() {
		return l.loadWIPDetail()
	}
	if l.cursor >= 0 && l.cursor < len(l.commits) {
		return l.loadCommitDetail(l.commits[l.cursor])
	}
	return nil
}

// loadDetailForCursorMaybeMore loads the detail for the current cursor AND
// triggers pagination if the cursor is within 20 items of the end.
func (l *LogPage) loadDetailForCursorMaybeMore() tea.Cmd {
	detailCmd := l.loadDetailForCursor()
	// Trigger pagination when within 20 items of the end.
	threshold := 20
	remaining := len(l.commits) - 1 - l.cursor
	if remaining <= threshold && l.canLoadMore && !l.loadingMore {
		l.loadingMore = true
		moreCmd := l.loadLogMore()
		return tea.Batch(detailCmd, moreCmd)
	}
	return detailCmd
}

func (l LogPage) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When center is showing a diff, j/k scroll the diff and Esc exits.
	if l.diffViewer.Active {
		// Delegate all diff-mode keys to the DiffViewer.
		handled, cmd := l.diffViewer.HandleKeys(msg, l.navKeys, l.repo, l.height)
		if handled {
			// HandleKeys.Reset() means Esc was pressed — return focus to detail.
			if !l.diffViewer.Active {
				l.focus = focusLogDetail
			}
			return l, cmd
		}
		return l, nil
	}

	// Normal commit list navigation (graph mode).
	switch {
	case key.Matches(msg, l.navKeys.Down):
		if l.cursor < len(l.commits)-1 {
			l.cursor++
			cmd := l.loadDetailForCursorMaybeMore()
			return l, cmd
		}
	case key.Matches(msg, l.navKeys.Up):
		if l.cursor > 0 {
			l.cursor--
			cmd := l.loadDetailForCursor()
			return l, cmd
		}
	case key.Matches(msg, l.navKeys.PageDown):
		l.cursor += 10
		if l.cursor >= len(l.commits) {
			l.cursor = len(l.commits) - 1
		}
		if l.cursor < 0 {
			l.cursor = 0
		}
		cmd := l.loadDetailForCursorMaybeMore()
		return l, cmd
	case key.Matches(msg, l.navKeys.PageUp):
		l.cursor -= 10
		if l.cursor < 0 {
			l.cursor = 0
		}
		cmd := l.loadDetailForCursor()
		return l, cmd
	case key.Matches(msg, l.navKeys.Home):
		l.cursor = 0
		cmd := l.loadDetailForCursor()
		return l, cmd
	case key.Matches(msg, l.navKeys.End):
		if len(l.commits) > 0 {
			l.cursor = len(l.commits) - 1
			cmd := l.loadDetailForCursorMaybeMore()
			return l, cmd
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		return l, l.loadLog()
	case key.Matches(msg, key.NewBinding(key.WithKeys("H"))):
		// Scroll graph left
		if l.graphScrollX > 0 {
			l.graphScrollX--
		}
		return l, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("L"))):
		// Scroll graph right
		maxScroll := l.maxGraphWidth() - l.graphViewportCols()
		if maxScroll < 0 {
			maxScroll = 0
		}
		if l.graphScrollX < maxScroll {
			l.graphScrollX++
		}
		return l, nil
	}

	// Undo/Redo — works regardless of selection
	switch {
	case key.Matches(msg, l.commitOpsKeys.Undo):
		return l, l.doUndo()
	case key.Matches(msg, l.commitOpsKeys.Redo):
		return l, l.doRedo()
	}

	// Bisect menu — works regardless of selection
	if key.Matches(msg, l.commitOpsKeys.BisectMenu) {
		return l, l.showBisectMenu()
	}

	// Compare mode toggle
	if key.Matches(msg, l.commitOpsKeys.CompareRef) {
		return l.handleCompareToggle()
	}

	// Commit operations — only on past (non-WIP) commits
	if !l.isWIPSelected() && l.cursor >= 0 && l.cursor < len(l.commits) {
		commit := l.commits[l.cursor]
		short := commit.ShortHash
		if short == "" {
			short = commit.Hash
			if len(short) > 7 {
				short = short[:7]
			}
		}
		switch {
		case key.Matches(msg, l.commitOpsKeys.Revert):
			l.pendingOpHash = commit.Hash
			return l, func() tea.Msg {
				return RequestConfirmMsg{
					ID:      "revert-commit",
					Title:   "Revert Commit?",
					Message: "Revert " + short + " " + commit.Subject + "?",
				}
			}
		case key.Matches(msg, l.commitOpsKeys.CherryPick):
			l.pendingOpHash = commit.Hash
			return l, func() tea.Msg {
				return RequestConfirmMsg{
					ID:      "cherry-pick-commit",
					Title:   "Cherry-Pick?",
					Message: "Cherry-pick " + short + " onto current branch?",
				}
			}
		case key.Matches(msg, l.commitOpsKeys.CopyHash):
			return l, l.copyToClipboard(commit.Hash)
		case key.Matches(msg, l.commitOpsKeys.ResetMenu):
			return l, l.showResetMenu(commit, short)
		case key.Matches(msg, l.commitOpsKeys.Squash):
			l.pendingOpHash = commit.Hash
			l.pendingOpAction = "squash"
			return l, l.showRebaseConfirm(commit, short, "squash")
		case key.Matches(msg, l.commitOpsKeys.Fixup):
			l.pendingOpHash = commit.Hash
			l.pendingOpAction = "fixup"
			return l, l.showRebaseConfirm(commit, short, "fixup")
		case key.Matches(msg, l.commitOpsKeys.Drop):
			l.pendingOpHash = commit.Hash
			l.pendingOpAction = "drop"
			return l, l.showRebaseConfirm(commit, short, "drop")
		}
	}

	return l, nil
}

// detailTabCount is the number of tabs in the commit detail panel.
const detailTabCount = 3

// detailTabNames are the tab labels for the commit detail panel.
var detailTabNames = []string{"Files", "Message", "Stats"}

func (l LogPage) handleDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// PR detail view — limited keybindings
	if l.viewingPR && l.viewedPR != nil {
		return l.handlePRDetailKeys(msg)
	}

	if l.isWIPSelected() {
		return l.handleWIPDetailKeys(msg)
	}

	// Tab switching: left/right arrows cycle through detail tabs
	switch {
	case key.Matches(msg, l.navKeys.Right):
		l.detailTab = (l.detailTab + 1) % detailTabCount
		l.detailTabScroll = 0
		return l, nil
	case key.Matches(msg, l.navKeys.Left):
		l.detailTab = (l.detailTab - 1 + detailTabCount) % detailTabCount
		l.detailTabScroll = 0
		return l, nil
	}

	if key.Matches(msg, key.NewBinding(key.WithKeys("esc"))) {
		l.diffViewer.Active = false
		l.diffViewer.Lines = nil
		l.diffViewer.Path = ""
		l.diffViewer.ScrollY = 0
		l.diffViewer.ScrollX = 0
		return l, nil
	}

	// Compare toggle also works from the detail panel
	if key.Matches(msg, l.commitOpsKeys.CompareRef) {
		return l.handleCompareToggle()
	}

	// Tab-specific keys
	switch l.detailTab {
	case 0: // Files tab
		switch {
		case key.Matches(msg, l.navKeys.Select): // Enter — view diff for selected file
			if len(l.detailFiles) > 0 && l.detailFileCursor < len(l.detailFiles) {
				l.diffViewer.ScrollY = 0
				l.diffViewer.ScrollX = 0
				return l, l.loadCenterDiff()
			}
		case key.Matches(msg, l.navKeys.Down):
			if l.detailFileCursor < len(l.detailFiles)-1 {
				l.detailFileCursor++
			}
		case key.Matches(msg, l.navKeys.Up):
			if l.detailFileCursor > 0 {
				l.detailFileCursor--
			}
		case key.Matches(msg, l.navKeys.Home):
			l.detailFileCursor = 0
		case key.Matches(msg, l.navKeys.End):
			if len(l.detailFiles) > 0 {
				l.detailFileCursor = len(l.detailFiles) - 1
			}
		case key.Matches(msg, l.navKeys.PageDown):
			if l.diffViewer.Active {
				l.diffViewer.ScrollY += 10
				maxScroll := len(l.diffViewer.Lines) - 10
				if maxScroll < 0 {
					maxScroll = 0
				}
				if l.diffViewer.ScrollY > maxScroll {
					l.diffViewer.ScrollY = maxScroll
				}
			} else {
				l.detailFileCursor += 10
				if l.detailFileCursor >= len(l.detailFiles) {
					l.detailFileCursor = len(l.detailFiles) - 1
				}
				if l.detailFileCursor < 0 {
					l.detailFileCursor = 0
				}
			}
		case key.Matches(msg, l.navKeys.PageUp):
			if l.diffViewer.Active {
				l.diffViewer.ScrollY -= 10
				if l.diffViewer.ScrollY < 0 {
					l.diffViewer.ScrollY = 0
				}
			} else {
				l.detailFileCursor -= 10
				if l.detailFileCursor < 0 {
					l.detailFileCursor = 0
				}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
			// Open selected file in editor
			if len(l.detailFiles) > 0 && l.detailFileCursor < len(l.detailFiles) {
				path := l.detailFiles[l.detailFileCursor].NewPath
				if path == "" {
					path = l.detailFiles[l.detailFileCursor].OldPath
				}
				if path != "" {
					return l, l.openInEditor(path)
				}
			}
		}
	case 1, 2: // Message or Stats tab — j/k scroll
		switch {
		case key.Matches(msg, l.navKeys.Down):
			l.detailTabScroll++
		case key.Matches(msg, l.navKeys.Up):
			if l.detailTabScroll > 0 {
				l.detailTabScroll--
			}
		case key.Matches(msg, l.navKeys.PageDown):
			l.detailTabScroll += 10
		case key.Matches(msg, l.navKeys.PageUp):
			l.detailTabScroll -= 10
			if l.detailTabScroll < 0 {
				l.detailTabScroll = 0
			}
		case key.Matches(msg, l.navKeys.Home):
			l.detailTabScroll = 0
		}
	}
	return l, nil
}

// ---------------------------------------------------------------------------
// Render — commit list (center panel)
// ---------------------------------------------------------------------------

func (l LogPage) renderCommitList(width, height int) string {
	t := theme.Active
	innerWidth := width - styles.PanelPaddingWidth
	commitCount := len(l.commits)
	if l.hasWIP {
		commitCount-- // exclude synthetic WIP entry from count
	}

	// Title — include filter query when active
	commitSearching := l.searching && l.searchPanel == focusLogList
	titleLabel := fmt.Sprintf("Commits (%d)", commitCount)
	if l.commitFilterQuery != "" && !commitSearching {
		titleLabel = fmt.Sprintf("Commits %q (%d)", l.commitFilterQuery, commitCount)
	}
	titleStr := styles.PanelTitle(
		titleLabel,
		"2", l.focus == focusLogList, innerWidth,
	)

	var lines []string
	if len(l.commits) == 0 {
		lines = append(lines, styles.DimStyle().Width(innerWidth).Render("  No commits yet. Make your first commit with 'c' on the Status page."))
	}

	// Inline search bar takes 1 line when active
	searchHeight := 0
	if commitSearching {
		searchHeight = 1
	}

	// Viewport windowing: only render visible commits, following the cursor.
	ph := height - styles.PanelBorderHeight
	// Visible lines: panel height minus title (1) minus title gap (1) minus hints area (2: empty + hints) minus search
	visibleCount := ph - 4 - searchHeight
	if visibleCount < 1 {
		visibleCount = 1
	}

	// Compute viewport offset so the cursor is always visible.
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
	end := offset + visibleCount
	if end > len(l.commits) {
		end = len(l.commits)
	}

	// Check if graph display is enabled in settings.
	showGraph := l.ctx == nil || l.ctx.Config == nil || l.ctx.Config.Appearance.ShowGraph

	// Compute max graph width across ALL commits (not just visible) for consistent column layout.
	graphWidth := 0
	if showGraph {
		for _, gr := range l.graphRows {
			w := len(gr.Cells)
			if w > graphWidth {
				graphWidth = w
			}
		}
	}
	// Each graph cell takes 1 character, plus 1 space separator after graph
	graphColWidth := 0
	if graphWidth > 0 {
		graphColWidth = graphWidth + 1 // graph chars + 1 space
	}
	// Cap graph to ~30% of inner width, bounded [10, 40]
	maxGraph := innerWidth * 30 / 100
	if maxGraph > 40 {
		maxGraph = 40
	}
	if maxGraph < 10 {
		maxGraph = 10
	}
	if graphColWidth > maxGraph {
		graphColWidth = maxGraph
	}

	// Check if compact log is enabled (hides date + author columns).
	compactLog := l.ctx != nil && l.ctx.Config != nil && l.ctx.Config.Appearance.CompactLog

	hashWidth := 8
	dateWidth := 10
	authorWidth := 15
	if compactLog {
		dateWidth = 0
		authorWidth = 0
	}
	separators := 5 // spaces between columns
	if compactLog {
		separators = 3 // fewer columns = fewer separators
	}
	subjectWidth := innerWidth - graphColWidth - hashWidth - dateWidth - authorWidth - separators
	if subjectWidth < 10 {
		subjectWidth = 10
	}

	for i := offset; i < end; i++ {
		c := l.commits[i]
		isWIP := l.hasWIP && i == 0
		selected := i == l.cursor && l.focus == focusLogList
		isCompareBase := l.compareBase != nil && c.Hash == l.compareBase.Hash && c.Hash != ""
		bg := t.Base
		if isCompareBase {
			bg = t.Surface0
		}
		if selected {
			bg = t.Surface1
		}
		bgS := lipgloss.NewStyle().Background(bg)

		// Render graph prefix with horizontal scroll viewport
		graphStr := ""
		if i < len(l.graphRows) && graphColWidth > 0 {
			gr := l.graphRows[i]
			displayCols := graphColWidth - 1 // -1 for trailing separator space
			scrollX := l.graphScrollX
			truncLeft := scrollX > 0
			truncRight := graphWidth > scrollX+displayCols

			// Determine the cell range to render
			startCol := scrollX
			endCol := scrollX + displayCols
			if truncLeft {
				startCol++ // reserve first column for ◂ indicator
			}
			if truncRight {
				endCol-- // reserve last column for ▸ indicator
			}

			var graphParts []string

			// Left scroll indicator
			if truncLeft {
				graphParts = append(graphParts, lipgloss.NewStyle().
					Foreground(t.Subtext0).Background(bg).Render("◂"))
			}

			// Render visible graph cells
			for j := startCol; j < endCol; j++ {
				if j < len(gr.Cells) {
					cell := gr.Cells[j]
					color := styles.GraphColor(cell.Column)
					graphParts = append(graphParts, lipgloss.NewStyle().
						Foreground(color).Background(bg).Render(cell.Char))
				} else {
					graphParts = append(graphParts, bgS.Render(" "))
				}
			}

			// Right scroll indicator
			if truncRight {
				graphParts = append(graphParts, lipgloss.NewStyle().
					Foreground(t.Subtext0).Background(bg).Render("▸"))
			}

			// Pad if total parts < displayCols (graph fits entirely)
			for len(graphParts) < displayCols {
				graphParts = append(graphParts, bgS.Render(" "))
			}

			graphStr = lipgloss.JoinHorizontal(lipgloss.Top, graphParts...)
		}

		sp := bgS.Render(" ")

		if isWIP {
			// WIP row — special rendering
			wipLabel := "Working Changes"
			fileCount := len(l.wipStaged) + len(l.wipUnstaged)
			if fileCount > 0 {
				wipLabel += fmt.Sprintf(" [%d files]", fileCount)
			}
			wipSubject := lipgloss.NewStyle().Foreground(t.Peach).Background(bg).Italic(true).
				Width(innerWidth - graphColWidth - 2).Render(wipLabel)

			var line string
			if graphStr != "" {
				line = lipgloss.JoinHorizontal(lipgloss.Top, sp, graphStr, sp, wipSubject)
			} else {
				line = lipgloss.JoinHorizontal(lipgloss.Top, sp, wipSubject)
			}

			lineStyle := lipgloss.NewStyle().Background(bg).Width(innerWidth)
			if selected {
				lineStyle = lineStyle.Bold(true)
			}
			line = lineStyle.Render(line)
			lines = append(lines, line)
			continue
		}

		// Compare base marker
		compareMarker := ""
		if isCompareBase {
			compareMarker = lipgloss.NewStyle().Foreground(t.Mauve).Background(bg).Bold(true).Render("▶")
		}

		hash := lipgloss.NewStyle().Foreground(t.Yellow).Background(bg).Width(hashWidth).Render(c.ShortHash)

		// Date and author columns (hidden in compact mode)
		var datePart, authorPart string
		if !compactLog {
			datePart = lipgloss.NewStyle().Foreground(t.Overlay0).Background(bg).Width(dateWidth).Render(c.Date.Format("2006-01-02"))
			authorPart = lipgloss.NewStyle().Foreground(t.Teal).Background(bg).Width(authorWidth).Render(truncate(c.Author, authorWidth))
		}

		// Render ref badges and compute remaining subject width
		badges := styles.RenderRefBadges(c.Refs, bg)
		badgeWidth := lipgloss.Width(badges)
		effectiveSubjectWidth := subjectWidth
		if badgeWidth > 0 {
			effectiveSubjectWidth = subjectWidth - badgeWidth - 1
			if effectiveSubjectWidth < 4 {
				// Badges too wide — skip them to avoid line overflow
				badges = ""
				badgeWidth = 0
				effectiveSubjectWidth = subjectWidth
			}
		}
		subject := lipgloss.NewStyle().Foreground(t.Text).Background(bg).Width(effectiveSubjectWidth).Render(truncate(c.Subject, effectiveSubjectWidth))

		// Build the commit row from parts.
		var parts []string
		if graphStr != "" {
			parts = append(parts, sp, graphStr, sp, compareMarker, hash)
		} else {
			parts = append(parts, bgS.Render("  "), compareMarker, hash)
		}
		if !compactLog {
			parts = append(parts, sp, datePart, sp, authorPart)
		}
		if badgeWidth > 0 {
			parts = append(parts, sp, badges)
		}
		parts = append(parts, sp, subject)
		line := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

		lineStyle := lipgloss.NewStyle().Background(bg).Width(innerWidth)
		if selected {
			lineStyle = lineStyle.Bold(true)
		}
		line = lineStyle.Render(line)

		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	// Pad content to exactly visibleCount lines so hints are pinned to the bottom
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > visibleCount {
		contentLines = contentLines[:visibleCount]
	}
	bgEmpty := lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render("")
	for len(contentLines) < visibleCount {
		contentLines = append(contentLines, bgEmpty)
	}
	content = strings.Join(contentLines, "\n")

	// Compact status line (key hints shown in global hint bar)
	statusParts := ""
	if l.compareBase != nil {
		statusParts = "COMPARING  W:exit"
	}
	if len(l.commits) > visibleCount {
		if statusParts != "" {
			statusParts += "  "
		}
		statusParts += fmt.Sprintf("%d/%d", l.cursor+1, len(l.commits))
	}
	hints := lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render(
		styles.KeyHintStyle().Render(statusParts),
	)
	titleGap := lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render("")
	emptyLine := lipgloss.NewStyle().Background(t.Base).Width(innerWidth).Render("")

	// Build inline search bar when active
	var searchBar string
	if commitSearching {
		searchBar = lipgloss.NewStyle().
			Foreground(t.Text).
			Background(t.Surface0).
			Width(innerWidth).
			Render(l.searchInput.View())
	}

	var full string
	if commitSearching {
		full = lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, searchBar, content, emptyLine, hints)
	} else {
		full = lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, emptyLine, hints)
	}
	// Clip to panel height so all panels stay the same outer height.
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.PanelStyleColor(l.borderAnim.Color(anim.BorderCenter, t.Surface1, t.Blue)).Width(width).Height(ph).Render(full)
}

// ---------------------------------------------------------------------------
// Render — commit detail (right panel)
// ---------------------------------------------------------------------------

func (l LogPage) renderCommitDetail(width, height int) string {
	if l.isWIPSelected() {
		return l.renderWIPDetail(width, height)
	}

	focused := l.focus == focusLogDetail
	iw := width - styles.PanelPaddingWidth // inner width
	panelTitle := "Detail"
	if l.compareBase != nil {
		baseShort := l.compareBase.ShortHash
		if baseShort == "" && len(l.compareBase.Hash) > 7 {
			baseShort = l.compareBase.Hash[:7]
		}
		panelTitle = "Compare: " + baseShort + "→"
	}
	titleStr := styles.PanelTitle(panelTitle, "3", focused, iw)
	ph := height - styles.PanelBorderHeight

	t := theme.Active
	titleGap := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")

	if l.detailCommit == nil {
		content := styles.DimStyle().Width(iw).Render("Select a commit to view details")
		return styles.ClipPanel(styles.PanelStyleColor(l.borderAnim.Color(anim.BorderRight, t.Surface1, t.Blue)).Width(width).Height(ph).Render(
			lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content),
		), height)
	}
	c := l.detailCommit

	bgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).MaxWidth(iw).Width(iw).Render(s)
	}

	// --- Commit metadata header (always shown) ---
	var metaSections []string

	// Line 1: short hash (yellow, bold) + compact date (dimmed)
	hashStr := lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).Bold(true).Render(c.ShortHash)
	dateStr := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render("  " + c.Date.Format("Mon Jan 2 2006"))
	metaSections = append(metaSections, bgLine(hashStr+dateStr))

	// Line 2: author name
	metaSections = append(metaSections, bgLine(lipgloss.NewStyle().Foreground(t.Teal).Background(t.Base).Render(c.Author)))

	// Line 3: ref badges (only if refs exist)
	if len(c.Refs) > 0 {
		metaSections = append(metaSections, bgLine(styles.RenderRefBadges(c.Refs, t.Base)))
	}

	// Tab bar
	tabBar := l.renderDetailTabBar(iw, t)
	metaSections = append(metaSections, tabBar)

	// Separator
	metaSections = append(metaSections, bgLine(lipgloss.NewStyle().Foreground(t.Surface2).Background(t.Base).Render(strings.Repeat("─", iw))))

	metaContent := lipgloss.JoinVertical(lipgloss.Left, metaSections...)
	metaLineCount := strings.Count(metaContent, "\n") + 1

	// Compact status line (key hints shown in global hint bar)
	detailStatus := ""
	if l.compareBase != nil {
		detailStatus = "COMPARING"
	}
	hints := lipgloss.NewStyle().Background(t.Base).Width(iw).Render(
		styles.KeyHintStyle().Render(detailStatus),
	)
	hintHeight := strings.Count(hints, "\n") + 1
	emptyLine := bgLine("")

	// Budget: panel height minus title, title gap, empty line, and hints.
	contentBudget := ph - 3 - hintHeight
	if contentBudget < 1 {
		contentBudget = 1
	}

	// Body area gets whatever remains after metadata
	bodyBudget := contentBudget - metaLineCount
	if bodyBudget < 1 {
		bodyBudget = 1
	}

	// --- Tab content ---
	var bodySections []string
	switch l.detailTab {
	case 0: // Files tab
		bodySections = l.renderDetailFilesTab(iw, bodyBudget, focused, t, bgLine)
	case 1: // Message tab
		bodySections = l.renderDetailMessageTab(iw, bodyBudget, c, t, bgLine)
	case 2: // Stats tab
		bodySections = l.renderDetailStatsTab(iw, bodyBudget, t, bgLine)
	}

	// Combine metadata + body
	allSections := make([]string, 0, len(metaSections)+len(bodySections))
	allSections = append(allSections, metaSections...)
	allSections = append(allSections, bodySections...)
	content := lipgloss.JoinVertical(lipgloss.Left, allSections...)

	// Pad content to exactly contentBudget lines so hints are pinned to the bottom
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > contentBudget {
		contentLines = contentLines[:contentBudget]
	}
	for len(contentLines) < contentBudget {
		contentLines = append(contentLines, bgLine(""))
	}
	content = strings.Join(contentLines, "\n")

	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, emptyLine, hints)
	// Clip to panel height so all panels stay the same outer height.
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.ClipPanel(styles.PanelStyleColor(l.borderAnim.Color(anim.BorderRight, t.Surface1, t.Blue)).Width(width).Height(ph).Render(full), height)
}

// renderDetailTabBar renders the tab selector for the detail panel.
func (l LogPage) renderDetailTabBar(iw int, t theme.Theme) string {
	var parts []string
	for i, name := range detailTabNames {
		if i == l.detailTab {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(t.Blue).Background(t.Base).Bold(true).
				Render(" "+name+" "))
		} else {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(t.Overlay0).Background(t.Base).
				Render(" "+name+" "))
		}
		if i < len(detailTabNames)-1 {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(t.Surface2).Background(t.Base).
				Render("│"))
		}
	}
	bar := strings.Join(parts, "")
	return lipgloss.NewStyle().Background(t.Base).MaxWidth(iw).Width(iw).Render(bar)
}

// renderDetailFilesTab renders the file list content for the Files tab.
func (l LogPage) renderDetailFilesTab(iw, budget int, focused bool, t theme.Theme, _ func(string) string) []string {
	var sections []string

	if len(l.detailFiles) == 0 {
		sections = append(sections, styles.DimStyle().Width(iw).Render("  No files changed"))
		return sections
	}

	fileHeaderLines := 1
	fileVisibleCount := budget - fileHeaderLines
	if fileVisibleCount < 1 {
		fileVisibleCount = 1
	}

	scrollInfo := ""
	if len(l.detailFiles) > fileVisibleCount {
		scrollInfo = fmt.Sprintf(" [%d/%d]", l.detailFileCursor+1, len(l.detailFiles))
	}
	fileLabel := fmt.Sprintf("Files changed (%d)%s", len(l.detailFiles), scrollInfo)
	sections = append(sections, lipgloss.NewStyle().Foreground(t.Subtext0).Background(t.Base).Bold(true).Width(iw).Render(fileLabel))

	// Viewport windowing
	offset := 0
	if l.detailFileCursor >= fileVisibleCount {
		offset = l.detailFileCursor - fileVisibleCount + 1
	}
	if offset > len(l.detailFiles)-fileVisibleCount {
		offset = len(l.detailFiles) - fileVisibleCount
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + fileVisibleCount
	if end > len(l.detailFiles) {
		end = len(l.detailFiles)
	}

	for i := offset; i < end; i++ {
		f := l.detailFiles[i]
		icon := styles.FileListIcon(f.Status)
		color := styles.FileListColor(f.Status)
		path := f.NewPath
		if f.Status == "deleted" {
			path = f.OldPath
		}
		if f.Status == "renamed" && f.OldPath != f.NewPath {
			path = f.OldPath + " → " + f.NewPath
		}

		selected := i == l.detailFileCursor && focused
		bg := t.Base
		prefix := "  "
		if selected {
			bg = t.Surface1
			prefix = "▸ "
		}

		iconStr := lipgloss.NewStyle().Foreground(color).Render(icon)
		pathStr := lipgloss.NewStyle().Foreground(t.Text).Render(" " + path)

		added, removed := f.Stats()
		var statStr string
		if added > 0 || removed > 0 {
			var statParts []string
			if added > 0 {
				statParts = append(statParts, lipgloss.NewStyle().Foreground(t.Green).Render(fmt.Sprintf("+%d", added)))
			}
			if removed > 0 {
				statParts = append(statParts, lipgloss.NewStyle().Foreground(t.Red).Render(fmt.Sprintf("-%d", removed)))
			}
			statStr = " " + strings.Join(statParts, " ")
		}

		lineContent := lipgloss.NewStyle().MaxWidth(iw).Render(prefix + iconStr + pathStr + statStr)
		sections = append(sections, fillBg(lineContent, bg, iw))
	}
	return sections
}

// renderDetailMessageTab renders the full commit message for the Message tab.
func (l LogPage) renderDetailMessageTab(iw, budget int, c *git.CommitInfo, t theme.Theme, bgLine func(string) string) []string {
	lines := make([]string, 0, 16)

	// Subject
	lines = append(lines, bgLine(""))
	subjectWrapped := ansi.Wordwrap(c.Subject, iw, "")
	for _, wl := range strings.Split(subjectWrapped, "\n") {
		lines = append(lines, bgLine(lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Bold(true).Render(wl)))
	}

	// Body
	if c.Body != "" {
		lines = append(lines, bgLine(""))
		for _, bodyLine := range strings.Split(c.Body, "\n") {
			bodyWrapped := ansi.Wordwrap(bodyLine, iw, "")
			for _, wl := range strings.Split(bodyWrapped, "\n") {
				lines = append(lines, bgLine(lipgloss.NewStyle().Foreground(t.Subtext0).Background(t.Base).Render(wl)))
			}
		}
	} else {
		lines = append(lines, bgLine(""))
		lines = append(lines, bgLine(styles.DimStyle().Render("  (no body)")))
	}

	// Full hash
	lines = append(lines, bgLine(""))
	lines = append(lines, bgLine(lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render("Hash: "+c.Hash)))

	// Parents
	if len(c.Parents) > 0 {
		lines = append(lines, bgLine(lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render("Parents: "+strings.Join(c.Parents, ", "))))
	}

	// Author email
	if c.AuthorEmail != "" {
		lines = append(lines, bgLine(lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render("Email: "+c.AuthorEmail)))
	}

	// Apply scroll
	if l.detailTabScroll > 0 {
		if l.detailTabScroll >= len(lines) {
			lines = nil
		} else {
			lines = lines[l.detailTabScroll:]
		}
	}

	// Truncate to budget
	if len(lines) > budget {
		lines = lines[:budget]
	}

	return lines
}

// renderDetailStatsTab renders diff statistics for the Stats tab.
func (l LogPage) renderDetailStatsTab(iw, budget int, t theme.Theme, bgLine func(string) string) []string {
	lines := make([]string, 0, len(l.detailFiles)+4)

	if len(l.detailFiles) == 0 {
		lines = append(lines, bgLine(styles.DimStyle().Render("  No files changed")))
		return lines
	}

	// Summary line
	totalAdded, totalRemoved := 0, 0
	for _, f := range l.detailFiles {
		a, r := f.Stats()
		totalAdded += a
		totalRemoved += r
	}
	summary := fmt.Sprintf("%d files changed", len(l.detailFiles))
	if totalAdded > 0 {
		summary += fmt.Sprintf(", %d insertions(+)", totalAdded)
	}
	if totalRemoved > 0 {
		summary += fmt.Sprintf(", %d deletions(-)", totalRemoved)
	}
	lines = append(lines, bgLine(lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Bold(true).Render(summary)))
	lines = append(lines, bgLine(""))

	// Per-file stats with bar graph
	maxChanges := 0
	for _, f := range l.detailFiles {
		a, r := f.Stats()
		if a+r > maxChanges {
			maxChanges = a + r
		}
	}

	barMaxWidth := iw / 3
	if barMaxWidth < 10 {
		barMaxWidth = 10
	}
	if barMaxWidth > 30 {
		barMaxWidth = 30
	}

	for _, f := range l.detailFiles {
		path := f.NewPath
		if f.Status == "deleted" {
			path = f.OldPath
		}
		added, removed := f.Stats()
		total := added + removed

		// Stat bar
		barWidth := 0
		if maxChanges > 0 {
			barWidth = total * barMaxWidth / maxChanges
		}
		if barWidth == 0 && total > 0 {
			barWidth = 1
		}
		addBar := 0
		if total > 0 {
			addBar = added * barWidth / total
		}
		remBar := barWidth - addBar

		bar := lipgloss.NewStyle().Foreground(t.Green).Render(strings.Repeat("+", addBar)) +
			lipgloss.NewStyle().Foreground(t.Red).Render(strings.Repeat("-", remBar))

		// Truncate path to leave room for stats
		maxPath := iw - barMaxWidth - 10
		if maxPath < 10 {
			maxPath = 10
		}
		if len(path) > maxPath {
			path = "~" + path[len(path)-maxPath+1:]
		}

		statNum := fmt.Sprintf("%4d", total)
		line := lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Render(path) +
			lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render(" "+statNum+" ") +
			bar

		lines = append(lines, bgLine(line))
	}

	// Apply scroll
	if l.detailTabScroll > 0 {
		if l.detailTabScroll >= len(lines) {
			lines = nil
		} else {
			lines = lines[l.detailTabScroll:]
		}
	}

	// Truncate to budget
	if len(lines) > budget {
		lines = lines[:budget]
	}

	return lines
}

// ---------------------------------------------------------------------------
// Stash diff render
// ---------------------------------------------------------------------------

func (l LogPage) renderStashDiff(width, height int) string {
	focused := l.focus == focusLogDetail
	iw := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight
	t := theme.Active

	titleStr := styles.PanelTitle(
		fmt.Sprintf("Stash Info (stash@{%d})", l.stashDiffIndex),
		"3", focused, iw,
	)

	var sections []string

	bgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).MaxWidth(iw).Width(iw).Render(s)
	}

	if l.stashDiffContent == "" {
		sections = append(sections, styles.DimStyle().Width(iw).Render("Loading stash info..."))
	} else {
		sections = append(sections, bgLine(lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).Bold(true).Render(
			fmt.Sprintf("stash@{%d}", l.stashDiffIndex),
		)))
		sections = append(sections, bgLine(""))
		sections = append(sections, bgLine(styles.DimStyle().Render("Diff is shown in the center panel.")))
	}

	titleGap := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Compact status line (key hints shown in global hint bar)
	hints := lipgloss.NewStyle().Background(t.Base).Width(iw).Render(
		styles.KeyHintStyle().Render(""),
	)
	hintHeight := strings.Count(hints, "\n") + 1
	emptyLine := bgLine("")

	// Budget: panel height minus title, title gap, empty line, and hints.
	contentBudget := ph - 3 - hintHeight
	if contentBudget < 1 {
		contentBudget = 1
	}

	// Pad content to exactly contentBudget lines so hints are pinned to the bottom
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > contentBudget {
		contentLines = contentLines[:contentBudget]
	}
	for len(contentLines) < contentBudget {
		contentLines = append(contentLines, bgLine(""))
	}
	content = strings.Join(contentLines, "\n")

	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, emptyLine, hints)
	// Clip to panel height so all panels stay the same outer height.
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.ClipPanel(styles.PanelStyleColor(l.borderAnim.Color(anim.BorderRight, t.Surface1, t.Blue)).Width(width).Height(ph).Render(full), height)
}

// handlePRDetailKeys handles keyboard input when the PR detail view is active.
func (l LogPage) handlePRDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("o"))):
		// Open PR in browser
		if l.viewedPR != nil && l.viewedPR.URL != "" {
			prURL := l.viewedPR.URL
			return l, func() tea.Msg {
				if err := utils.OpenBrowser(prURL); err != nil {
					return RequestToastMsg{Message: "Failed to open browser: " + err.Error(), IsError: true}
				}
				return RequestToastMsg{Message: "Opened PR in browser"}
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		// Exit PR view
		l.viewingPR = false
		l.viewedPR = nil
		return l, nil
	}
	return l, nil
}

// renderPRDetail renders the pull request detail in the right panel.
func (l LogPage) renderPRDetail(width, height int) string {
	focused := l.focus == focusLogDetail
	iw := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight
	t := theme.Active
	pr := l.viewedPR

	titleStr := styles.PanelTitle(
		fmt.Sprintf("PR #%d", pr.Number),
		"3", focused, iw,
	)

	bgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).MaxWidth(iw).Width(iw).Render(s)
	}

	var sections []string

	// PR title with status icon
	stateColor := t.Green
	stateLabel := "Open"
	switch pr.State {
	case "closed":
		stateColor = t.Red
		stateLabel = "Closed"
	case "merged":
		stateColor = t.Mauve
		stateLabel = "Merged"
	}
	if pr.Draft {
		stateColor = t.Overlay0
		stateLabel = "Draft"
	}

	stateBadge := lipgloss.NewStyle().
		Foreground(t.Base).Background(stateColor).
		Bold(true).Padding(0, 1).
		Render(stateLabel)

	sections = append(sections, bgLine(
		lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Bold(true).
			Render(pr.Title),
	))
	sections = append(sections, bgLine(stateBadge))
	sections = append(sections, bgLine(""))

	// Meta info
	metaStyle := lipgloss.NewStyle().Foreground(t.Subtext0).Background(t.Base)
	valStyle := lipgloss.NewStyle().Foreground(t.Text).Background(t.Base)

	sections = append(sections, bgLine(
		metaStyle.Render("Author   ")+valStyle.Render(pr.Author),
	))
	sections = append(sections, bgLine(
		metaStyle.Render("Branch   ")+
			lipgloss.NewStyle().Foreground(t.Sapphire).Background(t.Base).Render(pr.HeadRef)+
			metaStyle.Render(" → ")+
			lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).Render(pr.BaseRef),
	))

	if pr.Additions > 0 || pr.Deletions > 0 {
		sections = append(sections, bgLine(
			metaStyle.Render("Changes  ")+
				lipgloss.NewStyle().Foreground(t.Green).Background(t.Base).Render(fmt.Sprintf("+%d", pr.Additions))+" "+
				lipgloss.NewStyle().Foreground(t.Red).Background(t.Base).Render(fmt.Sprintf("-%d", pr.Deletions)),
		))
	}

	if len(pr.Labels) > 0 {
		labelStr := strings.Join(pr.Labels, ", ")
		sections = append(sections, bgLine(
			metaStyle.Render("Labels   ")+
				lipgloss.NewStyle().Foreground(t.Yellow).Background(t.Base).Render(labelStr),
		))
	}

	if !pr.CreatedAt.IsZero() {
		sections = append(sections, bgLine(
			metaStyle.Render("Created  ")+valStyle.Render(pr.CreatedAt.Format("Jan 02, 2006 15:04")),
		))
	}
	if !pr.UpdatedAt.IsZero() {
		sections = append(sections, bgLine(
			metaStyle.Render("Updated  ")+valStyle.Render(pr.UpdatedAt.Format("Jan 02, 2006 15:04")),
		))
	}

	// Body / description
	if pr.Body != "" {
		sections = append(sections, bgLine(""))
		sections = append(sections, bgLine(
			lipgloss.NewStyle().Foreground(t.Subtext0).Background(t.Base).Bold(true).Render("Description"),
		))
		sections = append(sections, bgLine(""))
		bodyLines := strings.Split(pr.Body, "\n")
		for _, bl := range bodyLines {
			// Truncate long lines
			if len(bl) > iw {
				bl = bl[:iw-1] + "…"
			}
			sections = append(sections, bgLine(
				lipgloss.NewStyle().Foreground(t.Text).Background(t.Base).Render(bl),
			))
		}
	}

	// URL hint
	sections = append(sections, bgLine(""))
	sections = append(sections, bgLine(
		lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Italic(true).
			Render(pr.URL),
	))

	titleGap := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Compact status line
	hints := lipgloss.NewStyle().Background(t.Base).Width(iw).Render(
		styles.KeyHintStyle().Render("o:Open in Browser"),
	)
	hintHeight := strings.Count(hints, "\n") + 1
	emptyLine := bgLine("")

	// Budget: panel height minus title, title gap, empty line, and hints.
	contentBudget := ph - 3 - hintHeight
	if contentBudget < 1 {
		contentBudget = 1
	}

	// Pad content to exactly contentBudget lines so hints are pinned to the bottom
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > contentBudget {
		contentLines = contentLines[:contentBudget]
	}
	for len(contentLines) < contentBudget {
		contentLines = append(contentLines, bgLine(""))
	}
	content = strings.Join(contentLines, "\n")

	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, emptyLine, hints)
	// Clip to panel height so all panels stay the same outer height.
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.ClipPanel(styles.PanelStyleColor(l.borderAnim.Color(anim.BorderRight, t.Surface1, t.Blue)).Width(width).Height(ph).Render(full), height)
}

// ---------------------------------------------------------------------------
// Commands — loading
// ---------------------------------------------------------------------------

// defaultPageSize is the number of commits loaded per page.
const defaultPageSize = 200

func (l LogPage) pageSize() int {
	if l.logPageSize > 0 {
		return l.logPageSize
	}
	return defaultPageSize
}

func (l LogPage) loadLog() tea.Cmd {
	repo := l.repo
	ps := l.pageSize()
	return func() tea.Msg {
		commits, err := repo.Log(git.LogOptions{MaxCount: ps})
		if err != nil {
			return logLoadedMsg{commits: commits, err: err}
		}

		// Check for uncommitted changes — if dirty, prepend a synthetic WIP entry.
		status, statusErr := repo.Status()
		hasWIP := false
		if statusErr == nil && status != nil {
			hasWIP = len(status.StagedFiles()) > 0 || len(status.UnstagedFiles()) > 0
		}

		if hasWIP {
			wipEntry := git.CommitInfo{
				Hash:      "",
				ShortHash: "●",
				Subject:   "Working Changes",
			}
			commits = append([]git.CommitInfo{wipEntry}, commits...)
		}

		graphRows := git.ComputeGraph(commits)
		return logLoadedMsg{commits: commits, graphRows: graphRows, hasWIP: hasWIP, err: nil}
	}
}

func (l LogPage) loadLogFiltered(query string) tea.Cmd {
	repo := l.repo
	ps := l.pageSize()
	return func() tea.Msg {
		commits, err := repo.Log(git.LogOptions{MaxCount: ps, Grep: query})
		if err != nil {
			return logLoadedMsg{commits: commits, err: err}
		}

		// Check for uncommitted changes — if dirty, prepend a synthetic WIP entry.
		status, statusErr := repo.Status()
		hasWIP := false
		if statusErr == nil && status != nil {
			hasWIP = len(status.StagedFiles()) > 0 || len(status.UnstagedFiles()) > 0
		}

		if hasWIP {
			wipEntry := git.CommitInfo{
				Hash:      "",
				ShortHash: "●",
				Subject:   "Working Changes",
			}
			commits = append([]git.CommitInfo{wipEntry}, commits...)
		}

		graphRows := git.ComputeGraph(commits)
		return logLoadedMsg{commits: commits, graphRows: graphRows, hasWIP: hasWIP, err: nil}
	}
}

// loadLogMore loads the next page of commits and appends them.
func (l LogPage) loadLogMore() tea.Cmd {
	repo := l.repo
	ps := l.pageSize()
	// Skip = total real commits (exclude WIP entry).
	skip := len(l.commits)
	if l.hasWIP {
		skip-- // don't count the synthetic WIP entry
	}
	existing := make([]git.CommitInfo, len(l.commits))
	copy(existing, l.commits)
	hasWIP := l.hasWIP

	return func() tea.Msg {
		more, err := repo.Log(git.LogOptions{MaxCount: ps, Skip: skip})
		if err != nil {
			return logMoreLoadedMsg{err: err}
		}

		// Combine existing + new commits, preserving WIP entry if present.
		combined := existing
		combined = append(combined, more...)
		graphRows := git.ComputeGraph(combined)

		result := logMoreLoadedMsg{
			commits:   more,
			graphRows: graphRows,
			err:       nil,
		}
		// If we fetched fewer than a full page, there's nothing more to load.
		_ = hasWIP
		_ = ps
		return result
	}
}

func (l LogPage) loadCommitDetail(c git.CommitInfo) tea.Cmd {
	repo := l.repo
	compareBase := l.compareBase
	return func() tea.Msg {
		var diff *git.DiffResult
		var err error
		if compareBase != nil {
			// Compare mode: diff between base and selected commit
			diff, err = repo.DiffBranch(compareBase.Hash, c.Hash)
		} else {
			diff, err = repo.DiffCommit(c.Hash)
		}
		// Fetch the commit body separately (not included in bulk log format)
		body, _ := repo.CommitBody(c.Hash)
		c.Body = body
		return commitDetailMsg{commit: c, diff: diff, err: err}
	}
}

// loadCenterDiff determines the appropriate file based on context (WIP
// unstaged/staged or commit detail) and loads its diff to display in the
// center panel. It replaces the old loadWIPSelectedDiff().
func (l LogPage) loadCenterDiff() tea.Cmd {
	repo := l.repo

	// --- WIP context ---
	if l.isWIPSelected() {
		var path string
		var staged bool
		var untracked bool

		switch l.wipFocus {
		case wipFocusUnstaged:
			if len(l.wipUnstaged) > 0 && l.wipUnstagedCursor < len(l.wipUnstaged) {
				f := l.wipUnstaged[l.wipUnstagedCursor]
				path = f.Path
				staged = false
				untracked = f.IsUntracked()
			}
		case wipFocusStaged:
			if len(l.wipStaged) > 0 && l.wipStagedCursor < len(l.wipStaged) {
				path = l.wipStaged[l.wipStagedCursor].Path
				staged = true
			}
		}

		if path == "" {
			return nil
		}

		return func() tea.Msg {
			var diff string
			var err error
			if untracked {
				diff, err = repo.FileDiffUntracked(path)
			} else {
				diff, err = repo.FileDiff(path, staged)
			}
			return centerDiffMsg{path: path, diff: diff, err: err, isWIP: true, isStaged: staged}
		}
	}

	// --- Commit detail context ---
	if l.detailCommit != nil && len(l.detailFiles) > 0 && l.detailFileCursor < len(l.detailFiles) {
		f := l.detailFiles[l.detailFileCursor]
		commitHash := l.detailCommit.Hash
		filePath := f.NewPath
		if f.Status == "deleted" {
			filePath = f.OldPath
		}

		return func() tea.Msg {
			diff, err := repo.DiffCommitFile(commitHash, filePath)
			return centerDiffMsg{path: filePath, diff: diff, err: err}
		}
	}

	return nil
}

func (l LogPage) loadStashDiff(index int) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		diff, err := repo.StashShow(index)
		return stashDiffMsg{index: index, diff: diff, err: err}
	}
}

// ---------------------------------------------------------------------------
// Commit operations
// ---------------------------------------------------------------------------

// doRevertCommit runs git revert in the background and refreshes on completion.
func (l LogPage) doRevertCommit(hash string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.RevertCommit(hash)
		if err != nil {
			return RequestToastMsg{Message: "Revert failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "revert"}
	}
}

// doCherryPick runs git cherry-pick in the background and refreshes on completion.
func (l LogPage) doCherryPick(hash string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.CherryPick(hash)
		if err != nil {
			return RequestToastMsg{Message: "Cherry-pick failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "cherry-pick"}
	}
}

// ---------------------------------------------------------------------------
// Reset menu
// ---------------------------------------------------------------------------

func (l LogPage) showResetMenu(_ git.CommitInfo, short string) tea.Cmd {
	return func() tea.Msg {
		return RequestMenuMsg{
			ID:    "reset-menu",
			Title: "Reset to " + short,
			Options: []MenuOption{
				{Label: "Soft reset", Description: "Keep all changes staged", Key: "s"},
				{Label: "Mixed reset", Description: "Keep changes as unstaged", Key: "m"},
				{Label: "Hard reset", Description: "Discard all changes", Key: "h"},
				{Label: "Nuke working tree", Description: "Hard reset HEAD + clean untracked", Key: "n"},
			},
		}
	}
}

func (l LogPage) handleMenuResult(msg dialog.MenuResultMsg) (tea.Model, tea.Cmd) {
	switch msg.ID {
	case "reset-menu":
		return l.handleResetMenuResult(msg.Index)
	case "bisect-menu":
		return l.handleBisectMenuResult(msg.Index)
	}
	return l, nil
}

func (l LogPage) handleResetMenuResult(idx int) (tea.Model, tea.Cmd) {
	if !l.isWIPSelected() && l.cursor >= 0 && l.cursor < len(l.commits) {
		commit := l.commits[l.cursor]
		hash := commit.Hash
		short := commit.ShortHash
		if short == "" && len(hash) > 7 {
			short = hash[:7]
		}
		switch idx {
		case 0: // Soft reset
			l.pendingOpHash = hash
			l.pendingOpAction = "reset-soft"
			return l, l.doResetOp(hash, "soft", short)
		case 1: // Mixed reset
			l.pendingOpHash = hash
			l.pendingOpAction = "reset-mixed"
			return l, l.doResetOp(hash, "mixed", short)
		case 2: // Hard reset
			l.pendingOpHash = hash
			l.pendingOpAction = "reset-hard"
			return l, func() tea.Msg {
				return RequestConfirmMsg{
					ID:      "reset-hard-confirm",
					Title:   "Hard Reset?",
					Message: "Hard reset to " + short + "?\n\nThis will discard all uncommitted changes.",
				}
			}
		case 3: // Nuke
			return l, func() tea.Msg {
				return RequestConfirmMsg{
					ID:      "nuke-working-tree",
					Title:   "Nuke Working Tree?",
					Message: "Reset --hard HEAD and remove ALL untracked files?\n\nThis cannot be undone.",
				}
			}
		}
	}
	return l, nil
}

func (l LogPage) doResetOp(hash, mode, short string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		var err error
		switch mode {
		case "soft":
			err = repo.ResetSoft(hash)
		case "mixed":
			err = repo.ResetMixed(hash)
		case "hard":
			err = repo.ResetHard(hash)
		}
		if err != nil {
			return RequestToastMsg{Message: "Reset failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "Reset (" + mode + ") to " + short}
	}
}

func (l LogPage) doNukeWorkingTree() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.NukeWorkingTree()
		if err != nil {
			return RequestToastMsg{Message: "Nuke failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "Nuke working tree"}
	}
}

// ---------------------------------------------------------------------------
// Interactive rebase (one-shot actions)
// ---------------------------------------------------------------------------

func (l LogPage) showRebaseConfirm(commit git.CommitInfo, short, action string) tea.Cmd {
	subject := commit.Subject
	actionTitle := strings.ToUpper(action[:1]) + action[1:]
	return func() tea.Msg {
		return RequestConfirmMsg{
			ID:      "rebase-" + action,
			Title:   actionTitle + " Commit?",
			Message: actionTitle + " " + short + " " + subject + "?\n\nThis uses interactive rebase.",
		}
	}
}

func (l LogPage) doRebaseAction(hash, action string) tea.Cmd {
	repo := l.repo
	actionTitle := strings.ToUpper(action[:1]) + action[1:]
	return func() tea.Msg {
		err := repo.RebaseInteractiveAction(hash, action)
		if err != nil {
			return RequestToastMsg{Message: actionTitle + " failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: actionTitle}
	}
}

// ---------------------------------------------------------------------------
// Compare two commits
// ---------------------------------------------------------------------------

func (l LogPage) handleCompareToggle() (tea.Model, tea.Cmd) {
	if l.compareBase != nil {
		// Exit compare mode
		l.compareBase = nil
		return l, tea.Batch(
			func() tea.Msg { return RequestToastMsg{Message: "Compare mode off"} },
			func() tea.Msg { return CompareStateMsg{Active: false} },
		)
	}
	// Enter compare mode — mark current commit as base
	if !l.isWIPSelected() && l.cursor >= 0 && l.cursor < len(l.commits) {
		c := l.commits[l.cursor]
		l.compareBase = &c
		short := c.ShortHash
		if short == "" && len(c.Hash) > 7 {
			short = c.Hash[:7]
		}
		s := short
		return l, tea.Batch(
			func() tea.Msg {
				return RequestToastMsg{Message: "Comparing from " + s + " — select another commit"}
			},
			func() tea.Msg { return CompareStateMsg{Active: true, Hash: s} },
		)
	}
	return l, nil
}

// ---------------------------------------------------------------------------
// Bisect
// ---------------------------------------------------------------------------

func (l LogPage) showBisectMenu() tea.Cmd {
	bisecting := l.repo.IsBisecting()
	var opts []MenuOption
	if !bisecting {
		opts = []MenuOption{
			{Label: "Start bisect", Description: "Begin a bisect session", Key: "s"},
		}
	} else {
		opts = []MenuOption{
			{Label: "Mark as bad", Description: "Current commit introduces the bug", Key: "b"},
			{Label: "Mark as good", Description: "Current commit is before the bug", Key: "g"},
			{Label: "Skip", Description: "Skip this commit (untestable)", Key: "k"},
			{Label: "Reset bisect", Description: "End bisect session", Key: "r"},
		}
	}
	return func() tea.Msg {
		return RequestMenuMsg{
			ID:      "bisect-menu",
			Title:   "Bisect",
			Options: opts,
		}
	}
}

func (l LogPage) handleBisectMenuResult(idx int) (tea.Model, tea.Cmd) {
	bisecting := l.repo.IsBisecting()
	if !bisecting {
		// Only option: start
		if idx == 0 {
			return l, l.doBisectStart()
		}
		return l, nil
	}
	switch idx {
	case 0: // bad
		return l, l.doBisectMark("bad")
	case 1: // good
		return l, l.doBisectMark("good")
	case 2: // skip
		return l, l.doBisectSkip()
	case 3: // reset
		return l, func() tea.Msg {
			return RequestConfirmMsg{
				ID:      "bisect-reset",
				Title:   "End Bisect?",
				Message: "Reset the bisect session and return to the original branch?",
			}
		}
	}
	return l, nil
}

func (l LogPage) doBisectStart() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.BisectStart()
		if err != nil {
			return RequestToastMsg{Message: "Bisect start failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "Bisect started"}
	}
}

func (l LogPage) doBisectMark(markType string) tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		var out string
		var err error
		switch markType {
		case "bad":
			out, err = repo.BisectBad("")
		case "good":
			out, err = repo.BisectGood("")
		}
		if err != nil {
			return RequestToastMsg{Message: "Bisect " + markType + " failed: " + err.Error(), IsError: true}
		}
		// Check if bisect found the culprit
		msg := "Marked as " + markType
		if strings.Contains(out, "is the first bad commit") {
			// Extract first line (the culprit hash + subject)
			culprit := out
			if idx := strings.Index(out, " is the first bad commit"); idx > 0 {
				culprit = out[:idx]
				if len(culprit) > 12 {
					culprit = culprit[:12]
				}
			}
			msg = "Bisect complete! Culprit: " + culprit
		}
		return commitOpDoneMsg{op: msg}
	}
}

func (l LogPage) doBisectSkip() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		_, err := repo.BisectSkip()
		if err != nil {
			return RequestToastMsg{Message: "Bisect skip failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "Bisect: skipped commit"}
	}
}

func (l LogPage) doBisectReset() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		err := repo.BisectReset()
		if err != nil {
			return RequestToastMsg{Message: "Bisect reset failed: " + err.Error(), IsError: true}
		}
		return commitOpDoneMsg{op: "Bisect ended"}
	}
}

// ---------------------------------------------------------------------------
// Undo / Redo via reflog
// ---------------------------------------------------------------------------

func (l LogPage) doUndo() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		entries, err := repo.Reflog(20)
		if err != nil || len(entries) < 2 {
			return RequestToastMsg{Message: "Nothing to undo", IsError: true}
		}
		// entries[0] is the current state, entries[1] is the previous state
		target := entries[1]
		return undoTargetMsg{
			hash:      target.Hash,
			shortHash: target.ShortHash,
			message:   target.Message,
		}
	}
}

func (l LogPage) doUndoConfirmed() tea.Cmd {
	hash := l.pendingUndoHash
	if hash == "" {
		return func() tea.Msg {
			return RequestToastMsg{Message: "Nothing to undo", IsError: true}
		}
	}
	short := hash
	if len(short) > 7 {
		short = short[:7]
	}
	return func() tea.Msg {
		return safeResetMsg{hash: hash, short: short, op: "Undo"}
	}
}

func (l LogPage) doRedo() tea.Cmd {
	repo := l.repo
	return func() tea.Msg {
		// After an undo (reset --hard), the reflog looks like:
		// entries[0] = current state (after undo)
		// entries[1] = the reset operation itself
		// entries[2] = state before undo (what we want to redo to)
		entries, err := repo.Reflog(30)
		if err != nil || len(entries) < 3 {
			return RequestToastMsg{Message: "Nothing to redo", IsError: true}
		}
		if !strings.HasPrefix(entries[1].Action, "reset") {
			return RequestToastMsg{Message: "Nothing to redo", IsError: true}
		}
		target := entries[2]
		return redoTargetMsg{
			hash:      target.Hash,
			shortHash: target.ShortHash,
			message:   target.Message,
		}
	}
}

func (l LogPage) doRedoConfirmed() tea.Cmd {
	hash := l.pendingRedoHash
	if hash == "" {
		return func() tea.Msg {
			return RequestToastMsg{Message: "Nothing to redo", IsError: true}
		}
	}
	short := hash
	if len(short) > 7 {
		short = short[:7]
	}
	return func() tea.Msg {
		return safeResetMsg{hash: hash, short: short, op: "Redo"}
	}
}

// ---------------------------------------------------------------------------
// Clipboard
// ---------------------------------------------------------------------------

// copyToClipboard copies text to the system clipboard and shows a toast.
func (l LogPage) copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		if err := utils.CopyToClipboard(text); err != nil {
			return RequestToastMsg{Message: "Copy failed: " + err.Error(), IsError: true}
		}
		short := text
		if len(short) > 7 {
			short = short[:7]
		}
		return RequestToastMsg{Message: "Copied " + short + " to clipboard"}
	}
}
