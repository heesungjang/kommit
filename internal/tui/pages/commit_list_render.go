package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/anim"
	"github.com/heesungjang/kommit/internal/tui/styles"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Render — commit list (center panel)
// ---------------------------------------------------------------------------

func (l LogPage) renderCommitList(width, height int) string {
	t := l.ctx.Theme
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

	// Use the cached max graph width (updated on data load, not per frame).
	graphWidth := 0
	if showGraph {
		graphWidth = l.cachedGraphMax
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

	t := l.ctx.Theme
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
	t := l.ctx.Theme

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

// renderPRDetail renders the pull request detail in the right panel.
func (l LogPage) renderPRDetail(width, height int) string {
	focused := l.focus == focusLogDetail
	iw := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight
	t := l.ctx.Theme
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
