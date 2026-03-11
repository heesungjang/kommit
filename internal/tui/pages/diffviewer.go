package pages

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nicholascross/opengit/internal/git"
	"github.com/nicholascross/opengit/internal/tui/anim"
	"github.com/nicholascross/opengit/internal/tui/keys"
	"github.com/nicholascross/opengit/internal/tui/styles"
	"github.com/nicholascross/opengit/internal/tui/theme"
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
	Active bool // true when center panel is showing diff instead of graph
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
}

// ResetScroll resets scroll offsets without clearing content.
func (d *DiffViewer) ResetScroll() {
	d.ScrollY = 0
	d.ScrollX = 0
	d.VisualMode = false
}

// SetContent loads new diff content, parses hunk headers, and activates the viewer.
func (d *DiffViewer) SetContent(path, diff string, isWIP, isStaged bool) {
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
		if strings.HasPrefix(line, "@@") {
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
				if len(hl) > 0 {
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
	}
}

// maxScroll returns the maximum scroll offset.
func (d *DiffViewer) maxScroll() int {
	ms := len(d.Lines) - 10
	if ms < 0 {
		ms = 0
	}
	return ms
}

// clampScrollY ensures ScrollY is within valid bounds.
func (d *DiffViewer) clampScrollY() {
	ms := d.maxScroll()
	if d.ScrollY > ms {
		d.ScrollY = ms
	}
	if d.ScrollY < 0 {
		d.ScrollY = 0
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
		}
		return true, nil
	case key.Matches(msg, navKeys.Up):
		if d.ScrollY > 0 {
			d.ScrollY--
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
func (d *DiffViewer) VisualSelectionRange() (int, int) {
	lo, hi := d.VisualAnchor, d.VisualCursor
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

// Render renders the diff viewer panel.
func (d *DiffViewer) Render(width, height int, focused bool, borderAnim anim.BorderAnim) string {
	t := theme.Active
	iw := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight

	// Title
	titleLabel := "Diff"
	if d.Path != "" {
		titleLabel = "Diff: " + d.Path
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

	// Apply scroll offset
	startLine := d.ScrollY
	if startLine > len(d.Lines) {
		startLine = len(d.Lines)
	}
	if startLine < 0 {
		startLine = 0
	}

	contentHeight := ph - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	endLine := startLine + contentHeight
	if endLine > len(d.Lines) {
		endLine = len(d.Lines)
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

	var sections []string
	for i := 0; i < endLine; i++ {
		line := ""
		if i < len(d.Lines) {
			line = d.Lines[i]
		}

		lineType := byte(' ')
		if len(line) > 0 {
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

			rendered := expandTabs(line, 4)
			rendered = horizontalSlice(rendered, scrollX, contentWidth)
			contentStr = styles.DiffLineStyle(lineType).Width(contentWidth).Render(rendered)
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

	// Scroll and hint line
	scrollInfo := ""
	if len(d.Lines) > contentHeight {
		scrollInfo = fmt.Sprintf("  [%d/%d lines]", startLine+1, len(d.Lines))
	}
	emptyLine := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")

	var hintParts string
	if d.VisualMode {
		vLo, vHi := d.VisualSelectionRange()
		sel := vHi - vLo + 1
		hintParts = fmt.Sprintf("VISUAL  j/k:extend  %d lines", sel)
		if d.IsStaged {
			hintParts += "  u:unstage lines"
		} else {
			hintParts += "  s:stage lines"
		}
		hintParts += "  Esc/v:cancel"
	} else {
		hintParts = "j/k:scroll  h/l:pan  Esc:back  g/G:top/bottom"
		if len(d.HunkStarts) > 0 {
			hunkInfo := fmt.Sprintf("  n/N:hunk [%d/%d]", d.CurrentHunkIdx+1, len(d.HunkStarts))
			hintParts += hunkInfo
		}
		if d.IsWIP {
			if d.IsStaged {
				hintParts += "  u:unstage hunk"
			} else {
				hintParts += "  s:stage hunk"
			}
			hintParts += "  v:select lines"
		}
		hintParts += scrollInfo
	}
	hints := lipgloss.NewStyle().Background(t.Base).Width(iw).Render(
		styles.KeyHintStyle().Render(hintParts),
	)

	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, content, emptyLine, hints)
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
func expandTabs(s string, tabWidth int) string {
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
