package pages

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/anim"
	"github.com/heesungjang/kommit/internal/tui/keys"
	"github.com/heesungjang/kommit/internal/tui/styles"
	"github.com/heesungjang/kommit/internal/tui/syntax"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// DiffViewer manages the center diff panel state: scrollable diff content,
// hunk navigation, and visual (line-selection) mode. It is owned by LogPage
// and delegates rendering and key handling from the parent.
type DiffViewer struct {
	// Content
	Lines []string // pre-split diff lines for rendering
	Path  string   // file path shown in diff header

	// Scroll
	ScrollY int // vertical scroll offset (line index)
	ScrollX int // horizontal scroll offset (character offset)

	// Hunk navigation
	HunkStarts     []int      // line indices where each hunk starts (@@)
	Hunks          []git.Hunk // parsed hunk data for staging operations
	CurrentHunkIdx int        // currently selected hunk index

	// WIP context
	IsWIP    bool // true if this diff is from WIP context
	IsStaged bool // within WIP, whether staged or unstaged

	// Visual (line-selection) mode
	VisualMode   bool // true when in visual selection mode
	VisualCursor int  // current cursor position (line index)
	VisualAnchor int  // anchor point where selection started

	// Active state
	Active     bool // true when center panel is showing diff instead of graph
	SideBySide bool // true to render side-by-side instead of inline

	// Syntax highlighting
	highlighter *syntax.Highlighter // nil when no lexer found for file type
}

// Reset clears all diff viewer state, returning to inactive.
func (d *DiffViewer) Reset() {
	d.Active = false
	d.Lines = nil
	d.Path = ""
	d.ScrollY = 0
	d.ScrollX = 0
	d.HunkStarts = nil
	d.Hunks = nil
	d.CurrentHunkIdx = 0
	d.IsWIP = false
	d.IsStaged = false
	d.VisualMode = false
	d.VisualCursor = 0
	d.VisualAnchor = 0
	d.highlighter = nil
}

// ResetScroll resets scroll offsets without clearing content.
func (d *DiffViewer) ResetScroll() {
	d.ScrollY = 0
	d.ScrollX = 0
	d.VisualMode = false
}

// SetContent loads new diff content, parses hunk headers, and activates the viewer.
// syntaxTheme is the chroma style name for syntax highlighting (empty = default).
func (d *DiffViewer) SetContent(path, diff string, isWIP, isStaged bool, syntaxTheme ...string) {
	d.Path = path
	d.Lines = strings.Split(diff, "\n")
	d.Active = true
	d.IsWIP = isWIP
	d.IsStaged = isStaged
	d.ScrollY = 0
	d.ScrollX = 0
	d.CurrentHunkIdx = 0
	d.VisualMode = false
	d.VisualCursor = 0
	d.VisualAnchor = 0

	// Parse hunk headers to find hunk start positions and build hunk data.
	d.HunkStarts = nil
	d.Hunks = nil
	for i, line := range d.Lines {
		if !strings.HasPrefix(line, "@@") {
			continue
		}
		d.HunkStarts = append(d.HunkStarts, i)
		oldStart, newStart := parseDiffHunkNums(line)
		hunk := git.Hunk{
			StartOld: oldStart,
			StartNew: newStart,
		}
		// Collect hunk lines until next hunk header or end.
		for j := i + 1; j < len(d.Lines); j++ {
			nextLine := d.Lines[j]
			if strings.HasPrefix(nextLine, "@@") || strings.HasPrefix(nextLine, "diff --git") {
				break
			}
			hunk.Lines = append(hunk.Lines, nextLine)
		}
		// Compute CountOld / CountNew from the lines.
		for _, hl := range hunk.Lines {
			if hl != "" {
				switch hl[0] {
				case '-':
					hunk.CountOld++
				case '+':
					hunk.CountNew++
				default:
					hunk.CountOld++
					hunk.CountNew++
				}
			} else {
				hunk.CountOld++
				hunk.CountNew++
			}
		}
		d.Hunks = append(d.Hunks, hunk)
	}

	// Initialize syntax highlighter for this file type.
	// Pass the full diff lines so chroma can tokenize with multi-line context.
	sTheme := "catppuccin-mocha"
	if len(syntaxTheme) > 0 && syntaxTheme[0] != "" {
		sTheme = syntaxTheme[0]
	}
	d.highlighter = syntax.New(path, sTheme, d.Lines)
}

// isInvisibleHeader returns true for diff metadata lines that are never
// rendered in the viewport (diff --git, index, ---, +++). These lines are
// skipped during rendering and should also be skipped when scrolling so
// the user doesn't press j/k through invisible lines.
func isInvisibleHeader(line string) bool {
	return strings.HasPrefix(line, "diff --git") ||
		strings.HasPrefix(line, "index ") ||
		strings.HasPrefix(line, "--- ") ||
		strings.HasPrefix(line, "+++ ")
}

// countVisibleLines counts non-header lines up to (but not including) index n.
func countVisibleLines(lines []string, n int) int {
	count := 0
	for i := 0; i < n && i < len(lines); i++ {
		if !isInvisibleHeader(lines[i]) {
			count++
		}
	}
	return count
}

// maxScroll returns the maximum scroll offset.
func (d *DiffViewer) maxScroll() int {
	ms := len(d.Lines) - 10
	if ms < 0 {
		ms = 0
	}
	// Don't land on invisible header lines.
	for ms > 0 && ms < len(d.Lines) && isInvisibleHeader(d.Lines[ms]) {
		ms--
	}
	return ms
}

// clampScrollY ensures ScrollY is within valid bounds and not on an
// invisible header line.
func (d *DiffViewer) clampScrollY() {
	ms := d.maxScroll()
	if d.ScrollY > ms {
		d.ScrollY = ms
	}
	if d.ScrollY < 0 {
		d.ScrollY = 0
	}
	// Skip forward past invisible header lines.
	for d.ScrollY < len(d.Lines) && isInvisibleHeader(d.Lines[d.ScrollY]) {
		d.ScrollY++
	}
	// Re-clamp in case we overshot.
	if d.ScrollY > ms {
		d.ScrollY = ms
	}
}

// HandleKeys processes key events when the diff viewer is active.
// Returns (handled bool, model LogPage, cmd tea.Cmd).
// If handled is false, the key should be processed by the parent.
func (d *DiffViewer) HandleKeys(msg tea.KeyMsg, navKeys keys.NavigationKeys, repo *git.Repository, height int) (handled bool, cmd tea.Cmd) {
	ms := d.maxScroll()

	// Visual (line-selection) mode
	if d.VisualMode {
		return d.handleVisualKeys(msg, navKeys, repo, height)
	}

	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		d.Reset()
		return true, nil
	case key.Matches(msg, navKeys.Down):
		if d.ScrollY < ms {
			d.ScrollY++
			// Skip invisible header lines so each j press scrolls visible content.
			for d.ScrollY < len(d.Lines) && d.ScrollY < ms && isInvisibleHeader(d.Lines[d.ScrollY]) {
				d.ScrollY++
			}
		}
		return true, nil
	case key.Matches(msg, navKeys.Up):
		if d.ScrollY > 0 {
			d.ScrollY--
			// Skip invisible header lines backwards.
			for d.ScrollY > 0 && isInvisibleHeader(d.Lines[d.ScrollY]) {
				d.ScrollY--
			}
		}
		return true, nil
	case key.Matches(msg, navKeys.PageDown):
		d.ScrollY += 10
		d.clampScrollY()
		return true, nil
	case key.Matches(msg, navKeys.PageUp):
		d.ScrollY -= 10
		d.clampScrollY()
		return true, nil
	case key.Matches(msg, navKeys.Home):
		d.ScrollY = 0
		d.clampScrollY()
		return true, nil
	case key.Matches(msg, navKeys.End):
		d.ScrollY = ms
		return true, nil
	case key.Matches(msg, navKeys.Left): // h — pan left
		d.ScrollX -= 4
		if d.ScrollX < 0 {
			d.ScrollX = 0
		}
		return true, nil
	case key.Matches(msg, navKeys.Right): // l — pan right
		d.ScrollX += 4
		return true, nil

	// Toggle visual (line-selection) mode
	case key.Matches(msg, key.NewBinding(key.WithKeys("v"))):
		if d.IsWIP && len(d.HunkStarts) > 0 {
			d.VisualMode = true
			d.VisualCursor = d.ScrollY
			d.VisualAnchor = d.ScrollY
		}
		return true, nil

	// Toggle side-by-side / inline diff view
	case key.Matches(msg, key.NewBinding(key.WithKeys("V"))):
		d.SideBySide = !d.SideBySide
		mode := "inline"
		if d.SideBySide {
			mode = "side-by-side"
		}
		return true, func() tea.Msg {
			return RequestSettingsChangeMsg{Key: "appearance.diffMode", Value: mode}
		}

	// Toggle fullscreen diff mode
	case key.Matches(msg, key.NewBinding(key.WithKeys("f"))):
		return true, func() tea.Msg { return DiffFullscreenToggleMsg{} }

	// Hunk navigation
	case key.Matches(msg, key.NewBinding(key.WithKeys("n", "]"))):
		if len(d.HunkStarts) > 0 && d.CurrentHunkIdx < len(d.HunkStarts)-1 {
			d.CurrentHunkIdx++
			d.ScrollY = d.HunkStarts[d.CurrentHunkIdx]
			d.clampScrollY()
		}
		return true, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("N", "["))):
		if len(d.HunkStarts) > 0 && d.CurrentHunkIdx > 0 {
			d.CurrentHunkIdx--
			d.ScrollY = d.HunkStarts[d.CurrentHunkIdx]
			d.clampScrollY()
		}
		return true, nil

	// Hunk staging — only in WIP diff context
	case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
		if d.IsWIP && !d.IsStaged && d.CurrentHunkIdx < len(d.Hunks) {
			hunk := d.Hunks[d.CurrentHunkIdx]
			path := d.Path
			return true, stageHunkCmd(repo, path, hunk)
		}
		return true, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("u"))):
		if d.IsWIP && d.IsStaged && d.CurrentHunkIdx < len(d.Hunks) {
			hunk := d.Hunks[d.CurrentHunkIdx]
			path := d.Path
			return true, unstageHunkCmd(repo, path, hunk)
		}
		return true, nil
	}
	return false, nil
}

// handleVisualKeys handles keys in visual (line-selection) mode.
func (d *DiffViewer) handleVisualKeys(msg tea.KeyMsg, navKeys keys.NavigationKeys, repo *git.Repository, height int) (handled bool, cmd tea.Cmd) {
	maxLine := len(d.Lines) - 1
	if maxLine < 0 {
		maxLine = 0
	}

	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "v"))):
		d.VisualMode = false
		return true, nil

	case key.Matches(msg, navKeys.Down):
		if d.VisualCursor < maxLine {
			d.VisualCursor++
			// Auto-scroll to keep cursor visible
			contentHeight := height - styles.PanelBorderHeight - 4
			if contentHeight < 1 {
				contentHeight = 1
			}
			if d.VisualCursor >= d.ScrollY+contentHeight {
				d.ScrollY = d.VisualCursor - contentHeight + 1
			}
		}
		return true, nil

	case key.Matches(msg, navKeys.Up):
		if d.VisualCursor > 0 {
			d.VisualCursor--
			if d.VisualCursor < d.ScrollY {
				d.ScrollY = d.VisualCursor
			}
		}
		return true, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
		if d.IsWIP && !d.IsStaged {
			cmd := d.stageSelectedLines(repo)
			d.VisualMode = false
			return true, cmd
		}
		return true, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("u"))):
		if d.IsWIP && d.IsStaged {
			cmd := d.unstageSelectedLines(repo)
			d.VisualMode = false
			return true, cmd
		}
		return true, nil
	}
	return true, nil
}

// ScrollDiff scrolls the diff by n lines (positive = down, negative = up).
// Used by parent key handlers (detail panel, WIP panels) for PageDown/PageUp.
func (d *DiffViewer) ScrollDiff(n int) {
	d.ScrollY += n
	d.clampScrollY()
}

// ScrollMouseWheel handles mouse wheel scrolling.
func (d *DiffViewer) ScrollMouseWheel(delta int) {
	d.ScrollY += delta
	d.clampScrollY()
}

// VisualSelectionRange returns the ordered (lo, hi) range of the visual selection.
func (d *DiffViewer) VisualSelectionRange() (lo, hi int) {
	lo, hi = d.VisualAnchor, d.VisualCursor
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi
}

// stageSelectedLines stages only the visually-selected lines.
func (d *DiffViewer) stageSelectedLines(repo *git.Repository) tea.Cmd {
	lo, hi := d.VisualSelectionRange()
	path := d.Path

	type hunkSelection struct {
		hunk    git.Hunk
		indices map[int]bool
	}
	var selections []hunkSelection

	for hIdx, hunk := range d.Hunks {
		if hIdx >= len(d.HunkStarts) {
			break
		}
		hunkStart := d.HunkStarts[hIdx] + 1 // +1 to skip @@ header line
		hunkEnd := hunkStart + len(hunk.Lines) - 1

		if hi < hunkStart || lo > hunkEnd {
			continue
		}

		selected := make(map[int]bool)
		for lineIdx := range hunk.Lines {
			diffLineIdx := hunkStart + lineIdx
			if diffLineIdx >= lo && diffLineIdx <= hi {
				selected[lineIdx] = true
			}
		}
		if len(selected) > 0 {
			selections = append(selections, hunkSelection{hunk: hunk, indices: selected})
		}
	}

	if len(selections) == 0 {
		return nil
	}

	return func() tea.Msg {
		for _, sel := range selections {
			err := repo.StageLines(path, sel.hunk, sel.indices)
			if err != nil {
				return RequestToastMsg{Message: "Stage lines failed: " + err.Error(), IsError: true}
			}
		}
		return wipStageResultMsg{err: nil}
	}
}

// unstageSelectedLines unstages only the visually-selected lines.
func (d *DiffViewer) unstageSelectedLines(repo *git.Repository) tea.Cmd {
	lo, hi := d.VisualSelectionRange()
	path := d.Path

	type hunkSelection struct {
		hunk    git.Hunk
		indices map[int]bool
	}
	var selections []hunkSelection

	for hIdx, hunk := range d.Hunks {
		if hIdx >= len(d.HunkStarts) {
			break
		}
		hunkStart := d.HunkStarts[hIdx] + 1
		hunkEnd := hunkStart + len(hunk.Lines) - 1

		if hi < hunkStart || lo > hunkEnd {
			continue
		}

		selected := make(map[int]bool)
		for lineIdx := range hunk.Lines {
			diffLineIdx := hunkStart + lineIdx
			if diffLineIdx >= lo && diffLineIdx <= hi {
				selected[lineIdx] = true
			}
		}
		if len(selected) > 0 {
			selections = append(selections, hunkSelection{hunk: hunk, indices: selected})
		}
	}

	if len(selections) == 0 {
		return nil
	}

	return func() tea.Msg {
		for _, sel := range selections {
			err := repo.UnstageLines(path, sel.hunk, sel.indices)
			if err != nil {
				return RequestToastMsg{Message: "Unstage lines failed: " + err.Error(), IsError: true}
			}
		}
		return wipStageResultMsg{err: nil}
	}
}

// sideBySideRow represents one visual row in side-by-side mode.
// Each row has an optional old (left) and new (right) entry.
type sideBySideRow struct {
	oldLine string // text content (empty string means blank filler)
	newLine string
	oldNum  int // line number (0 means no number to show)
	newNum  int
	oldIdx  int // original index into DiffViewer.Lines (-1 = no mapping)
	newIdx  int
	oldType byte // '+', '-', ' ', '@', or 0 for blank
	newType byte
	isMeta  bool // similarity/rename metadata line
	isHunk  bool // hunk header
}

// buildSideBySideRows pairs diff lines into left/right rows for side-by-side rendering.
// Context lines appear on both sides, consecutive -/+ blocks are paired, and
// any unpaired lines get a blank filler on the opposite side.
func (d *DiffViewer) buildSideBySideRows() []sideBySideRow {
	var rows []sideBySideRow
	oldNum := 0
	newNum := 0
	inHunk := false

	i := 0
	for i < len(d.Lines) {
		line := d.Lines[i]

		// Skip file-level headers entirely
		if strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "index ") {
			i++
			continue
		}

		// Metadata lines (similarity, rename, new file, etc.)
		if strings.HasPrefix(line, "similarity ") || strings.HasPrefix(line, "rename ") ||
			strings.HasPrefix(line, "new file") || strings.HasPrefix(line, "deleted file") ||
			strings.HasPrefix(line, "old mode") || strings.HasPrefix(line, "new mode") {
			rows = append(rows, sideBySideRow{
				oldLine: line, newLine: line,
				oldType: ' ', newType: ' ',
				oldIdx: -1, newIdx: -1,
				isMeta: true,
			})
			i++
			continue
		}

		// Hunk header
		if strings.HasPrefix(line, "@@") {
			inHunk = true
			oldNum, newNum = parseDiffHunkNums(line)
			rows = append(rows, sideBySideRow{
				oldLine: stripHunkContext(line),
				newLine: stripHunkContext(line),
				oldType: '@', newType: '@',
				oldIdx: -1, newIdx: -1,
				isHunk: true,
			})
			i++
			continue
		}

		if !inHunk {
			rows = append(rows, sideBySideRow{
				oldLine: line, newLine: line,
				oldType: ' ', newType: ' ',
				oldIdx: -1, newIdx: -1,
			})
			i++
			continue
		}

		// Inside a hunk — collect consecutive -/+ blocks and pair them
		lineType := byte(' ')
		if line != "" {
			lineType = line[0]
		}

		if lineType == '-' {
			// Collect all consecutive '-' lines
			var removed []struct {
				text string
				num  int
				idx  int // original index into d.Lines
			}
			for i < len(d.Lines) {
				l := d.Lines[i]
				if l == "" || l[0] != '-' {
					break
				}
				oldNum++
				removed = append(removed, struct {
					text string
					num  int
					idx  int
				}{l, 0, i})
				i++
			}
			// Fix numbering: oldNum was incremented, the stored number should be the value before increment
			for idx := range removed {
				removed[idx].num = oldNum - len(removed) + idx
			}

			// Now collect any immediately following '+' lines
			var added []struct {
				text string
				num  int
				idx  int // original index into d.Lines
			}
			for i < len(d.Lines) {
				l := d.Lines[i]
				if l == "" || l[0] != '+' {
					break
				}
				newNum++
				added = append(added, struct {
					text string
					num  int
					idx  int
				}{l, newNum, i})
				i++
			}

			// Pair them up
			maxLen := len(removed)
			if len(added) > maxLen {
				maxLen = len(added)
			}
			for j := 0; j < maxLen; j++ {
				row := sideBySideRow{oldIdx: -1, newIdx: -1}
				if j < len(removed) {
					row.oldLine = removed[j].text
					row.oldNum = removed[j].num
					row.oldType = '-'
					row.oldIdx = removed[j].idx
				}
				if j < len(added) {
					row.newLine = added[j].text
					row.newNum = added[j].num
					row.newType = '+'
					row.newIdx = added[j].idx
				}
				rows = append(rows, row)
			}
			continue
		}

		if lineType == '+' {
			// Standalone '+' line (not preceded by '-')
			newNum++
			rows = append(rows, sideBySideRow{
				newLine: line, newNum: newNum, newType: '+',
				oldIdx: -1, newIdx: i,
			})
			i++
			continue
		}

		// Context line
		oldNum++
		newNum++
		rows = append(rows, sideBySideRow{
			oldLine: line, newLine: line,
			oldNum: oldNum, newNum: newNum,
			oldType: ' ', newType: ' ',
			oldIdx: i, newIdx: i,
		})
		i++
	}

	return rows
}

// renderSideBySide renders the diff content in a side-by-side two-column layout.
func (d *DiffViewer) renderSideBySide(iw, contentHeight int) string {
	t := theme.Active

	rows := d.buildSideBySideRows()

	// Each half: "NNNN │ <content>"
	const sideGutter = 6 // 4-digit number + space + separator
	const dividerWidth = 1
	halfWidth := (iw - dividerWidth) / 2
	if halfWidth < sideGutter+5 {
		halfWidth = sideGutter + 5
	}
	sideContentWidth := halfWidth - sideGutter
	if sideContentWidth < 4 {
		sideContentWidth = 4
	}

	// Map scroll offset: d.ScrollY is in terms of original diff lines.
	// For side-by-side, we use it as a row index into our paired rows.
	startRow := d.ScrollY
	if startRow < 0 {
		startRow = 0
	}
	if startRow > len(rows) {
		startRow = len(rows)
	}

	endRow := startRow + contentHeight
	if endRow > len(rows) {
		endRow = len(rows)
	}

	divStyle := lipgloss.NewStyle().Foreground(t.Surface1).Background(t.Base)
	scrollX := d.ScrollX

	var sections []string
	for ri := startRow; ri < endRow; ri++ {
		row := rows[ri]

		var leftStr, rightStr string

		if row.isHunk {
			// Hunk header spans both sides
			hunkGutter := styles.DiffGutterSepStyle('@').Render(strings.Repeat("─", sideGutter-1) + "┤")
			rendered := expandTabs(row.oldLine, 4)
			rendered = horizontalSlice(rendered, scrollX, sideContentWidth)
			hunkContent := styles.DiffLineStyle('@').Width(sideContentWidth).Render(rendered)
			leftStr = lipgloss.JoinHorizontal(lipgloss.Top, hunkGutter, hunkContent)
			rightStr = lipgloss.JoinHorizontal(lipgloss.Top, hunkGutter, hunkContent)
		} else if row.isMeta {
			gutter := styles.DiffGutterSepStyle(' ').Render(strings.Repeat(" ", sideGutter))
			rendered := expandTabs(row.oldLine, 4)
			rendered = horizontalSlice(rendered, scrollX, sideContentWidth)
			metaContent := styles.DiffMetaStyle().Width(sideContentWidth).Render(rendered)
			leftStr = lipgloss.JoinHorizontal(lipgloss.Top, gutter, metaContent)
			rightStr = lipgloss.JoinHorizontal(lipgloss.Top, gutter, metaContent)
		} else {
			leftStr = d.renderSideBySideHalf(row.oldLine, row.oldNum, row.oldType, row.oldIdx, sideGutter, sideContentWidth, scrollX)
			rightStr = d.renderSideBySideHalf(row.newLine, row.newNum, row.newType, row.newIdx, sideGutter, sideContentWidth, scrollX)
		}

		divider := divStyle.Render("│")
		fullLine := lipgloss.JoinHorizontal(lipgloss.Top, leftStr, divider, rightStr)
		sections = append(sections, fullLine)
	}

	if len(sections) == 0 {
		return ""
	}
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Pad to contentHeight
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > contentHeight {
		contentLines = contentLines[:contentHeight]
	}
	bgEmpty := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")
	for len(contentLines) < contentHeight {
		contentLines = append(contentLines, bgEmpty)
	}
	return strings.Join(contentLines, "\n")
}

// renderSideBySideHalf renders one half (left or right) of a side-by-side row.
// lineIdx is the original index into d.Lines for syntax highlighting (-1 = no mapping).
func (d *DiffViewer) renderSideBySideHalf(text string, lineNum int, lineType byte, lineIdx, gutterW, contentW, scrollX int) string {
	t := theme.Active

	// Blank filler row (when one side has no corresponding line)
	if lineType == 0 {
		bg := lipgloss.NewStyle().Background(t.Surface0).Foreground(t.Surface0)
		gutter := bg.Render(strings.Repeat(" ", gutterW))
		fill := bg.Width(contentW).Render("")
		return lipgloss.JoinHorizontal(lipgloss.Top, gutter, fill)
	}

	// Line number
	numStr := "    "
	if lineNum > 0 {
		numStr = fmt.Sprintf("%4d", lineNum)
	}
	numStyled := styles.DiffLineNumStyle(lineType).Render(numStr)
	sep := styles.DiffGutterSepStyle(lineType).Render(" │")
	gutter := lipgloss.JoinHorizontal(lipgloss.Top, numStyled, sep)

	// Content — use syntax highlighting when available.
	contentStr := d.renderDiffContent(lineIdx, text, lineType, scrollX, contentW)

	return lipgloss.JoinHorizontal(lipgloss.Top, gutter, contentStr)
}

// Render renders the diff viewer panel.
func (d *DiffViewer) Render(width, height int, focused bool, borderAnim anim.BorderAnim) string {
	t := theme.Active
	iw := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight

	// Title
	titleLabel := "Diff"
	if d.SideBySide {
		titleLabel = "Diff (split)"
	}
	if d.Path != "" {
		titleLabel += ": " + d.Path
	}
	maxLabel := iw - 5
	if maxLabel > 0 && len(titleLabel) > maxLabel {
		titleLabel = titleLabel[:maxLabel] + "…"
	}
	titleStr := styles.PanelTitle(titleLabel, "2", focused, iw)
	titleGap := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")

	if len(d.Lines) == 0 {
		content := styles.DimStyle().Width(iw).Render("  No diff content")
		full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content)
		if cl := strings.Split(full, "\n"); len(cl) > ph {
			full = strings.Join(cl[:ph], "\n")
		}
		return styles.ClipPanel(styles.PanelStyleColor(borderAnim.Color(anim.BorderCenter, t.Surface1, t.Blue)).Width(width).Height(ph).Render(full), height)
	}

	contentHeight := ph - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Side-by-side mode
	if d.SideBySide {
		content := d.renderSideBySide(iw, contentHeight)

		// Compact status line (key hints are shown in the global hint bar)
		statusParts := ""
		if len(d.HunkStarts) > 0 {
			statusParts = fmt.Sprintf("hunk %d/%d", d.CurrentHunkIdx+1, len(d.HunkStarts))
		}
		if len(d.Lines) > contentHeight {
			if statusParts != "" {
				statusParts += "  "
			}
			statusParts += fmt.Sprintf("row %d/%d", d.ScrollY+1, len(d.Lines))
		}
		emptyLine := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")
		statusLine := lipgloss.NewStyle().Background(t.Base).Width(iw).Render(
			styles.KeyHintStyle().Render(statusParts),
		)

		full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, emptyLine, statusLine)
		if cl := strings.Split(full, "\n"); len(cl) > ph {
			full = strings.Join(cl[:ph], "\n")
		}
		return styles.ClipPanel(styles.PanelStyleColor(borderAnim.Color(anim.BorderCenter, t.Surface1, t.Blue)).Width(width).Height(ph).Render(full), height)
	}

	// Apply scroll offset
	startLine := d.ScrollY
	if startLine > len(d.Lines) {
		startLine = len(d.Lines)
	}
	if startLine < 0 {
		startLine = 0
	}

	// --- Line number computation ---
	const gutterWidth = 11 // "NNNN NNNN │" = 4+1+4+1+1 = 11
	contentWidth := iw - gutterWidth
	if contentWidth < 10 {
		contentWidth = 10
	}

	oldNum := 0
	newNum := 0
	inHunk := false
	hunkIdx := -1

	// Iterate through all diff lines, collecting visible sections.
	// The loop continues until we have contentHeight visible lines or
	// exhaust the diff. This ensures skipped header lines (diff --git,
	// index, ---, +++) don't waste viewport slots.
	var sections []string
	for i := 0; i < len(d.Lines) && len(sections) < contentHeight; i++ {
		line := ""
		if i < len(d.Lines) {
			line = d.Lines[i]
		}

		lineType := byte(' ')
		if line != "" {
			lineType = line[0]
		}

		isDiffHeader := strings.HasPrefix(line, "diff --git")
		isFileOld := strings.HasPrefix(line, "--- ")
		isFileNew := strings.HasPrefix(line, "+++ ")
		isIndex := strings.HasPrefix(line, "index ")
		isSimilarity := strings.HasPrefix(line, "similarity ") || strings.HasPrefix(line, "rename ") || strings.HasPrefix(line, "new file") || strings.HasPrefix(line, "deleted file") || strings.HasPrefix(line, "old mode") || strings.HasPrefix(line, "new mode")
		isHunkHeader := strings.HasPrefix(line, "@@")

		if isHunkHeader {
			inHunk = true
			hunkIdx++
			oldNum, newNum = parseDiffHunkNums(line)
		}
		isCurrentHunk := hunkIdx >= 0 && hunkIdx == d.CurrentHunkIdx

		if i < startLine {
			if inHunk && !isHunkHeader && !isDiffHeader && !isFileOld && !isFileNew && !isIndex && !isSimilarity {
				switch lineType {
				case '+':
					newNum++
				case '-':
					oldNum++
				default:
					oldNum++
					newNum++
				}
			}
			continue
		}

		var gutterStr string
		var contentStr string
		scrollX := d.ScrollX

		if isDiffHeader || isFileOld || isFileNew || isIndex {
			continue
		} else if isSimilarity {
			gutterStr = styles.DiffGutterSepStyle(' ').Render(strings.Repeat(" ", gutterWidth))
			rendered := expandTabs(line, 4)
			rendered = horizontalSlice(rendered, scrollX, contentWidth)
			contentStr = styles.DiffMetaStyle().Width(contentWidth).Render(rendered)
		} else if isHunkHeader {
			if isCurrentHunk {
				gutterStr = styles.DiffGutterSepStyle('@').Render("►" + strings.Repeat("─", gutterWidth-2) + "┤")
			} else {
				gutterStr = styles.DiffGutterSepStyle('@').Render(strings.Repeat("─", gutterWidth-1) + "┤")
			}
			rendered := expandTabs(stripHunkContext(line), 4)
			rendered = horizontalSlice(rendered, scrollX, contentWidth)
			if isCurrentHunk {
				contentStr = styles.DiffCurrentHunkStyle().Width(contentWidth).Render(rendered)
			} else {
				contentStr = styles.DiffLineStyle('@').Width(contentWidth).Render(rendered)
			}
		} else if inHunk {
			oldStr := "    "
			newStr := "    "
			switch lineType {
			case '+':
				newStr = fmt.Sprintf("%4d", newNum)
				newNum++
			case '-':
				oldStr = fmt.Sprintf("%4d", oldNum)
				oldNum++
			default:
				oldStr = fmt.Sprintf("%4d", oldNum)
				newStr = fmt.Sprintf("%4d", newNum)
				oldNum++
				newNum++
			}

			numOldStyled := styles.DiffLineNumStyle(lineType).Render(oldStr)
			space := styles.DiffGutterSepStyle(lineType).Render(" ")
			numNewStyled := styles.DiffLineNumStyle(lineType).Render(newStr)
			sep := styles.DiffGutterSepStyle(lineType).Render(" │")
			gutterStr = lipgloss.JoinHorizontal(lipgloss.Top, numOldStyled, space, numNewStyled, sep)

			contentStr = d.renderDiffContent(i, line, lineType, scrollX, contentWidth)
		} else {
			gutterStr = styles.DiffGutterSepStyle(' ').Render(strings.Repeat(" ", gutterWidth))
			rendered := expandTabs(line, 4)
			rendered = horizontalSlice(rendered, scrollX, contentWidth)
			contentStr = styles.DiffLineStyle(lineType).Width(contentWidth).Render(rendered)
		}

		// Visual mode highlighting
		if d.VisualMode {
			vLo, vHi := d.VisualSelectionRange()
			if i >= vLo && i <= vHi {
				marker := lipgloss.NewStyle().Foreground(t.Blue).Background(t.Surface1).Bold(true).Render("▌")
				fullLine := lipgloss.JoinHorizontal(lipgloss.Top, marker, gutterStr, contentStr)
				sections = append(sections, fullLine)
				continue
			}
			if i == d.VisualCursor {
				marker := lipgloss.NewStyle().Foreground(t.Blue).Background(t.Base).Render("▸")
				fullLine := lipgloss.JoinHorizontal(lipgloss.Top, marker, gutterStr, contentStr)
				sections = append(sections, fullLine)
				continue
			}
		}

		fullLine := lipgloss.JoinHorizontal(lipgloss.Top, gutterStr, contentStr)
		sections = append(sections, fullLine)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Pad content to exactly contentHeight lines
	diffContentLines := strings.Split(content, "\n")
	if len(diffContentLines) > contentHeight {
		diffContentLines = diffContentLines[:contentHeight]
	}
	bgEmpty := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")
	for len(diffContentLines) < contentHeight {
		diffContentLines = append(diffContentLines, bgEmpty)
	}
	content = strings.Join(diffContentLines, "\n")

	// Compact status line (key hints shown in global hint bar)
	emptyLine := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")

	var statusParts string
	if d.VisualMode {
		vLo, vHi := d.VisualSelectionRange()
		sel := vHi - vLo + 1
		statusParts = fmt.Sprintf("VISUAL  %d lines selected", sel)
		if d.IsStaged {
			statusParts += "  u:unstage  Esc:cancel"
		} else {
			statusParts += "  s:stage  Esc:cancel"
		}
	} else {
		if len(d.HunkStarts) > 0 {
			statusParts = fmt.Sprintf("hunk %d/%d", d.CurrentHunkIdx+1, len(d.HunkStarts))
		}
		if len(d.Lines) > contentHeight {
			if statusParts != "" {
				statusParts += "  "
			}
			visStart := countVisibleLines(d.Lines, startLine) + 1
			visTotal := countVisibleLines(d.Lines, len(d.Lines))
			statusParts += fmt.Sprintf("line %d/%d", visStart, visTotal)
		}
	}
	statusLine := lipgloss.NewStyle().Background(t.Base).Width(iw).Render(
		styles.KeyHintStyle().Render(statusParts),
	)

	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, emptyLine, statusLine)
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.ClipPanel(styles.PanelStyleColor(borderAnim.Color(anim.BorderCenter, t.Surface1, t.Blue)).Width(width).Height(ph).Render(full), height)
}

// ---------------------------------------------------------------------------
// Command helpers (package-level to avoid capturing LogPage)
// ---------------------------------------------------------------------------

// stageHunkCmd stages a single diff hunk.
func stageHunkCmd(repo *git.Repository, path string, hunk git.Hunk) tea.Cmd {
	return func() tea.Msg {
		err := repo.StageHunk(path, hunk)
		if err != nil {
			return RequestToastMsg{Message: "Stage hunk failed: " + err.Error(), IsError: true}
		}
		return wipStageResultMsg{err: nil}
	}
}

// unstageHunkCmd unstages a single diff hunk.
func unstageHunkCmd(repo *git.Repository, path string, hunk git.Hunk) tea.Cmd {
	return func() tea.Msg {
		err := repo.UnstageHunk(path, hunk)
		if err != nil {
			return RequestToastMsg{Message: "Unstage hunk failed: " + err.Error(), IsError: true}
		}
		return wipStageResultMsg{err: nil}
	}
}

// ---------------------------------------------------------------------------
// Pure helper functions (shared between DiffViewer and legacy code)
// ---------------------------------------------------------------------------

// parseDiffHunkNums extracts the old and new starting line numbers from a
// @@ -old,count +new,count @@ hunk header line.
func parseDiffHunkNums(line string) (oldStart, newStart int) {
	idx := strings.Index(line, "@@")
	if idx < 0 {
		return 1, 1
	}
	rest := line[idx+2:]
	idx2 := strings.Index(rest, "@@")
	if idx2 <= 0 {
		return 1, 1
	}
	rangeInfo := strings.TrimSpace(rest[:idx2])
	parts := strings.Fields(rangeInfo)
	for _, part := range parts {
		if strings.HasPrefix(part, "-") {
			nums := strings.SplitN(part[1:], ",", 2)
			if len(nums) >= 1 {
				if n, err := strconv.Atoi(nums[0]); err == nil {
					oldStart = n
				}
			}
		} else if strings.HasPrefix(part, "+") {
			nums := strings.SplitN(part[1:], ",", 2)
			if len(nums) >= 1 {
				if n, err := strconv.Atoi(nums[0]); err == nil {
					newStart = n
				}
			}
		}
	}
	if oldStart == 0 {
		oldStart = 1
	}
	if newStart == 0 {
		newStart = 1
	}
	return
}

// stripHunkContext removes trailing function context from git hunk headers.
func stripHunkContext(line string) string {
	idx := strings.Index(line, "@@")
	if idx < 0 {
		return line
	}
	rest := line[idx+2:]
	idx2 := strings.Index(rest, "@@")
	if idx2 < 0 {
		return line
	}
	return line[:idx+2+idx2+2]
}

// expandTabs replaces tab characters with spaces.
func expandTabs(s string, _ int) string {
	const tabWidth = 4
	var result strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			for i := 0; i < spaces; i++ {
				result.WriteByte(' ')
			}
			col += spaces
		} else {
			result.WriteRune(r)
			col++
		}
	}
	return result.String()
}

// truncateToWidth truncates a string to fit within maxWidth visible columns.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	w := 0
	for i, r := range s {
		w++
		if w > maxWidth {
			return s[:i]
		}
		_ = r
	}
	return s
}

// horizontalSlice drops the first offset visible characters from s, then
// truncates the result to width characters.
func horizontalSlice(s string, offset, width int) string {
	if offset <= 0 {
		return truncateToWidth(s, width)
	}
	skipped := 0
	start := len(s)
	for i, r := range s {
		if skipped >= offset {
			start = i
			break
		}
		skipped++
		_ = r
	}
	if start >= len(s) {
		return ""
	}
	return truncateToWidth(s[start:], width)
}

// renderDiffContent renders a diff line's content with syntax highlighting
// when a highlighter is available. lineIdx is the index into d.Lines.
// The lineType determines the background color. If no highlighter is
// available, falls back to DiffLineStyle.
func (d *DiffViewer) renderDiffContent(lineIdx int, raw string, lineType byte, scrollX, contentWidth int) string {
	if d.highlighter == nil {
		rendered := expandTabs(raw, 4)
		rendered = horizontalSlice(rendered, scrollX, contentWidth)
		return styles.DiffLineStyle(lineType).Width(contentWidth).Render(rendered)
	}

	t := theme.Active
	bg := t.Base
	defaultFG := t.DiffContext()
	switch lineType {
	case '+':
		bg = t.DiffAddedBg()
		defaultFG = t.DiffAdded()
	case '-':
		bg = t.DiffRemovedBg()
		defaultFG = t.DiffRemoved()
	}

	return d.highlighter.RenderLine(
		lineIdx, lineType, bg, defaultFG,
		scrollX, contentWidth,
		styles.DiffLineStyle(lineType),
	)
}
