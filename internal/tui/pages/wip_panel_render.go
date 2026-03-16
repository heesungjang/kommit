package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/heesungjang/kommit/internal/ai"
	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui/anim"
	"github.com/heesungjang/kommit/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func (l LogPage) renderWIPDetail(width, height int) string {
	focused := l.focus == focusLogDetail
	iw := width - styles.PanelPaddingWidth
	ph := height - styles.PanelBorderHeight
	t := l.ctx.Theme

	bgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).MaxWidth(iw).Width(iw).Render(s)
	}

	// ---------------------------------------------------------------
	// Build file list sections (top part — will be clipped/padded)
	// ---------------------------------------------------------------
	// sectionTitle renders a section header with a right-aligned shortcut key tag.
	sectionTitle := func(label, shortcut string, active bool) string {
		fg := t.Subtext0
		if active {
			fg = t.Blue
		}
		keyTag := "[" + shortcut + "]"
		keyStyle := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base)
		titleStyle := lipgloss.NewStyle().Foreground(fg).Background(t.Base).Bold(true)
		labelWidth := iw - len(keyTag) - 1
		if labelWidth < 4 {
			return titleStyle.Width(iw).Render(label)
		}
		if len(label) > labelWidth {
			if labelWidth > 1 {
				label = label[:labelWidth-1] + "…"
			} else {
				label = label[:labelWidth]
			}
		}
		leftPart := titleStyle.Width(labelWidth).Render(label)
		rightPart := keyStyle.Render(keyTag)
		gap := lipgloss.NewStyle().Background(t.Base).Render(" ")
		return lipgloss.JoinHorizontal(lipgloss.Top, leftPart, gap, rightPart)
	}

	// --- Helper: render a file line ---
	renderFileLine := func(f git.FileStatus, idx int, cursorIdx int, isFocused bool, staged bool) string {
		icon := f.StatusIcon()
		code := f.UnstagedCode
		statsMap := l.wipUnstagedStats
		if staged {
			code = f.StagedCode
			statsMap = l.wipStagedStats
		}
		color := styles.FileStatusColor(code)

		selected := idx == cursorIdx && isFocused
		bg := t.Base
		prefix := "  "
		if selected {
			bg = t.Surface1
			prefix = "▸ "
		}

		iconStr := lipgloss.NewStyle().Foreground(color).Render(icon)
		pathStr := lipgloss.NewStyle().Foreground(t.Text).Render(" " + f.Path)
		if selected {
			iconStr = lipgloss.NewStyle().Foreground(color).Bold(true).Render(icon)
			pathStr = lipgloss.NewStyle().Foreground(t.Text).Bold(true).Render(" " + f.Path)
		}

		var statStr string
		if st, ok := statsMap[f.Path]; ok && (st.Added > 0 || st.Removed > 0) {
			var statParts []string
			if st.Added > 0 {
				statParts = append(statParts, lipgloss.NewStyle().Foreground(t.Green).Render(fmt.Sprintf("+%d", st.Added)))
			}
			if st.Removed > 0 {
				statParts = append(statParts, lipgloss.NewStyle().Foreground(t.Red).Render(fmt.Sprintf("-%d", st.Removed)))
			}
			statStr = " " + strings.Join(statParts, " ")
		}

		lineContent := lipgloss.NewStyle().MaxWidth(iw).Render(prefix + iconStr + pathStr + statStr)
		return fillBg(lineContent, bg, iw)
	}

	// --- Build unstaged file lines ---
	unstagedFocused := focused && l.wipFocus == wipFocusUnstaged
	var unstagedFileLines []string
	if len(l.wipUnstaged) == 0 {
		unstagedFileLines = append(unstagedFileLines, styles.DimStyle().Width(iw).Render("  Working tree clean"))
	} else {
		for i, f := range l.wipUnstaged {
			unstagedFileLines = append(unstagedFileLines, renderFileLine(f, i, l.wipUnstagedCursor, unstagedFocused, false))
		}
	}

	// --- Build staged file lines ---
	stagedFocused := focused && l.wipFocus == wipFocusStaged
	var stagedFileLines []string
	if len(l.wipStaged) == 0 {
		stagedFileLines = append(stagedFileLines, styles.DimStyle().Width(iw).Render("  No files staged"))
	} else {
		for i, f := range l.wipStaged {
			stagedFileLines = append(stagedFileLines, renderFileLine(f, i, l.wipStagedCursor, stagedFocused, true))
		}
	}

	// ---------------------------------------------------------------
	// Build commit area (bottom part — pinned to panel bottom)
	// Wrapped in a single border container that highlights when focused.
	// ---------------------------------------------------------------
	commitFocused := focused && l.wipFocus == wipFocusCommit

	// Inner width for content inside the outer commit container border
	ciw := iw - 2 // -2 for outer container border
	if ciw < 10 {
		ciw = 10
	}
	cBgLine := func(s string) string {
		return lipgloss.NewStyle().Background(t.Base).Width(ciw).Render(s)
	}

	var commitInner []string

	// Header: ─○ Commit (or ─○ Amend Commit) with [c] shortcut
	commitHeaderLabel := "─○ Commit"
	if l.aiGenerating {
		commitHeaderLabel = "─○ " + l.spinner.View()
	} else if l.commitAmend {
		commitHeaderLabel = "─○ Amend Commit"
	}
	{
		fg := t.Subtext0
		if commitFocused {
			fg = t.Blue
		}
		keyTag := "[c]"
		keyStyle := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base)
		titleStyle := lipgloss.NewStyle().Foreground(fg).Background(t.Base).Bold(true)
		labelWidth := ciw - len(keyTag) - 1
		if labelWidth < 4 {
			labelWidth = 4
		}
		label := commitHeaderLabel
		if lipgloss.Width(label) > labelWidth {
			label = label[:labelWidth-1] + "…"
		}
		gap := ciw - lipgloss.Width(label) - lipgloss.Width(keyTag)
		if gap < 1 {
			gap = 1
		}
		gapStyle := lipgloss.NewStyle().Background(t.Base)
		commitInner = append(commitInner, lipgloss.NewStyle().Background(t.Base).Width(ciw).Render(
			titleStyle.Render(label)+gapStyle.Render(strings.Repeat(" ", gap))+keyStyle.Render(keyTag),
		))
	}
	commitInner = append(commitInner, cBgLine("")) // margin bottom

	if l.aiGenerating {
		// Skeleton loading — shimmer sweep while AI generates.
		inputWidth := ciw - 2
		if inputWidth < 6 {
			inputWidth = 6
		}

		bg := t.Surface0

		// Cosine-eased shimmer position: accelerates through the middle,
		// decelerates at the edges for a premium feel.
		shimmerPos := shimmerEasePos(l.skeletonTick, inputWidth)

		renderShimmer := func(barLen, lineOffset int) string {
			return renderShimmerBar(barLen, inputWidth, shimmerPos, lineOffset, t.Surface2, t.Overlay1, bg)
		}

		// Summary skeleton — single shimmer bar
		summaryBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Surface2).
			BorderBackground(bg).
			Background(bg).
			Width(ciw - 2).
			Render(renderShimmer(inputWidth*2/3, 0))
		commitInner = append(commitInner, cBgLine(summaryBox))

		// Description skeleton — 3 lines with staggered shimmer
		descLines := lipgloss.JoinVertical(lipgloss.Left,
			renderShimmer(inputWidth*4/5, 2),
			renderShimmer(inputWidth*3/5, 4),
			renderShimmer(inputWidth*2/5, 6),
		)
		descBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Surface2).
			BorderBackground(bg).
			Background(bg).
			Width(ciw - 2).
			Render(descLines)
		commitInner = append(commitInner, cBgLine(descBox))
	} else {
		// Summary input — single-line
		inputWidth := ciw - 2 // -2 for individual input border
		if inputWidth < 6 {
			inputWidth = 6
		}
		l.commitSummary.Width = inputWidth

		summaryBorder := l.borderAnim.Color(anim.BorderCommitSummary, t.Surface2, t.Blue)
		summaryView := fillBg(l.commitSummary.View(), t.Surface0, inputWidth)
		summaryBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(summaryBorder).
			BorderBackground(t.Surface0).
			Background(t.Surface0).
			Width(ciw - 2). // -2 for input border
			Render(summaryView)
		commitInner = append(commitInner, cBgLine(summaryBox))

		// Description textarea — multi-line
		l.commitDesc.SetWidth(inputWidth)

		descBorder := l.borderAnim.Color(anim.BorderCommitDesc, t.Surface2, t.Blue)
		descView := fillBg(l.commitDesc.View(), t.Surface0, inputWidth)
		descBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(descBorder).
			BorderBackground(t.Surface0).
			Background(t.Surface0).
			Width(ciw - 2). // -2 for input border
			Render(descView)
		commitInner = append(commitInner, cBgLine(descBox))
	}

	// Wrap commit area in a single outer container border
	containerBorder := l.borderAnim.Color(anim.BorderCommitOuter, t.Surface2, t.Blue)
	innerContent := lipgloss.JoinVertical(lipgloss.Left, commitInner...)
	commitContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(containerBorder).
		BorderBackground(t.Base).
		Background(t.Base).
		Width(iw - 2). // outer border width
		Render(innerContent)

	// ---------------------------------------------------------------
	// Measure actual commit box height (instead of hardcoding).
	// The rendered commitContent may vary depending on widget output.
	// ---------------------------------------------------------------
	commitBoxHeight := strings.Count(commitContent, "\n") + 1

	// ---------------------------------------------------------------
	// Title
	// ---------------------------------------------------------------
	titleStr := styles.PanelTitle(
		"Working Changes",
		"3", focused, iw,
	)
	titleGap := lipgloss.NewStyle().Background(t.Base).Width(iw).Render("")

	// ---------------------------------------------------------------
	// Build the context-dependent hint line (placed OUTSIDE the commit box)
	// ---------------------------------------------------------------

	// Resolve the active AI model name for display in AI-related hints.
	aiModel := ""
	if l.ctx != nil && l.ctx.Config != nil {
		aiModel = l.ctx.Config.AI.Model
		if aiModel == "" {
			aiModel = ai.DefaultModel(l.ctx.Config.AI.Provider)
		}
	}

	var hintText string
	if commitFocused && l.commitEditing {
		hintText = "Enter:commit  Tab:desc  Esc:stop"
		if l.commitField == 1 {
			hintText = "ctrl+s:commit  Tab:summary  Esc:stop"
		}
	} else if commitFocused {
		hintText = "Enter:edit  A:amend  D:clear  ctrl+g:AI"
		if aiModel != "" {
			hintText += " · " + aiModel
		}
	} else {
		// ctrl+g works from any WIP sub-focus when not editing
		hintText = "ctrl+g:AI commit"
		if aiModel != "" {
			hintText += " · " + aiModel
		}
	}
	hintsLine := lipgloss.NewStyle().Background(t.Base).Width(iw).PaddingLeft(1).Render(
		styles.KeyHintStyle().Render(hintText),
	)
	hintHeight := strings.Count(hintsLine, "\n") + 1

	// ---------------------------------------------------------------
	// Split file area into two independent sub-viewports so the staged
	// section header is ALWAYS visible, even with many unstaged files.
	// Layout: title(1) + gap(1) + [unstaged viewport] + separator(1)
	//         + [staged viewport] + commitBox + hints
	// ---------------------------------------------------------------
	separatorHeight := 1
	fileAreaHeight := ph - 2 - separatorHeight - commitBoxHeight - hintHeight
	if fileAreaHeight < 2 {
		fileAreaHeight = 2
	}

	// Each sub-viewport gets: header(1) + margin(1) + file lines.
	// Chrome per section = 2 (header + margin).
	unstagedChrome := 2
	stagedChrome := 2

	// Allocate heights proportionally — staged gets at least enough to show
	// header + margin + 1 visible file line (min 3), unstaged gets the rest.
	unstagedDataLines := len(unstagedFileLines) // actual file rows (or 1 placeholder)
	stagedDataLines := len(stagedFileLines)

	// Minimum staged viewport: chrome + 1 file line
	minStagedHeight := stagedChrome + 1
	if minStagedHeight > fileAreaHeight-unstagedChrome-1 {
		minStagedHeight = fileAreaHeight - unstagedChrome - 1
		if minStagedHeight < 1 {
			minStagedHeight = 1
		}
	}

	// Desired heights (if there were unlimited space)
	wantUnstaged := unstagedChrome + unstagedDataLines
	wantStaged := stagedChrome + stagedDataLines
	totalWant := wantUnstaged + wantStaged

	var unstagedViewH, stagedViewH int
	if totalWant <= fileAreaHeight {
		// Everything fits — no scrolling needed
		unstagedViewH = wantUnstaged
		stagedViewH = fileAreaHeight - unstagedViewH
	} else {
		// Not everything fits — allocate proportionally with staged minimum
		stagedViewH = minStagedHeight
		remaining := fileAreaHeight - stagedViewH
		if remaining < unstagedChrome+1 {
			remaining = unstagedChrome + 1
			stagedViewH = fileAreaHeight - remaining
		}
		// If staged has few items, give more to unstaged
		if wantStaged < stagedViewH {
			stagedViewH = wantStaged
			remaining = fileAreaHeight - stagedViewH
		}
		unstagedViewH = remaining
	}

	// Ensure minimums
	if unstagedViewH < 1 {
		unstagedViewH = 1
	}
	if stagedViewH < 1 {
		stagedViewH = 1
	}

	// --- Render unstaged sub-viewport ---
	unstagedHeader := sectionTitle(fmt.Sprintf("▾ Unstaged Files (%d)", len(l.wipUnstaged)), "u", unstagedFocused)
	unstagedMargin := bgLine("")
	unstagedContentH := unstagedViewH - unstagedChrome
	if unstagedContentH < 0 {
		unstagedContentH = 0
	}

	// Scroll unstaged viewport to keep cursor visible
	if len(unstagedFileLines) > unstagedContentH && unstagedContentH > 0 {
		if l.wipUnstagedScroll > l.wipUnstagedCursor {
			l.wipUnstagedScroll = l.wipUnstagedCursor
		}
		if l.wipUnstagedCursor >= l.wipUnstagedScroll+unstagedContentH {
			l.wipUnstagedScroll = l.wipUnstagedCursor - unstagedContentH + 1
		}
		maxScroll := len(unstagedFileLines) - unstagedContentH
		if l.wipUnstagedScroll > maxScroll {
			l.wipUnstagedScroll = maxScroll
		}
		if l.wipUnstagedScroll < 0 {
			l.wipUnstagedScroll = 0
		}
	} else {
		l.wipUnstagedScroll = 0
	}

	// Clip unstaged file lines to viewport
	visibleUnstaged := unstagedFileLines
	if len(visibleUnstaged) > unstagedContentH && unstagedContentH > 0 {
		end := l.wipUnstagedScroll + unstagedContentH
		if end > len(visibleUnstaged) {
			end = len(visibleUnstaged)
		}
		visibleUnstaged = visibleUnstaged[l.wipUnstagedScroll:end]
	}
	// Pad to fill the viewport
	for len(visibleUnstaged) < unstagedContentH {
		visibleUnstaged = append(visibleUnstaged, bgLine(""))
	}

	// Build scroll indicator for unstaged
	unstagedScrollHint := ""
	if len(unstagedFileLines) > unstagedContentH && unstagedContentH > 0 {
		if l.wipUnstagedScroll > 0 {
			unstagedScrollHint += "↑"
		}
		if l.wipUnstagedScroll+unstagedContentH < len(unstagedFileLines) {
			if unstagedScrollHint != "" {
				unstagedScrollHint += " "
			}
			unstagedScrollHint += "↓"
		}
	}
	if unstagedScrollHint != "" {
		scrollTag := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render(" " + unstagedScrollHint)
		unstagedHeader = sectionTitle(fmt.Sprintf("▾ Unstaged (%d) %s", len(l.wipUnstaged), scrollTag), "u", unstagedFocused)
	}

	unstagedSection := make([]string, 0, 2+len(visibleUnstaged))
	unstagedSection = append(unstagedSection, unstagedHeader)
	unstagedSection = append(unstagedSection, unstagedMargin)
	unstagedSection = append(unstagedSection, visibleUnstaged...)

	// --- Separator ---
	separator := bgLine(lipgloss.NewStyle().Foreground(t.Surface2).Background(t.Base).Render(strings.Repeat("─", iw)))

	// --- Render staged sub-viewport ---
	stagedHeader := sectionTitle(fmt.Sprintf("▾ Staged Files (%d)", len(l.wipStaged)), "s", stagedFocused)
	stagedMargin := bgLine("")
	stagedContentH := stagedViewH - stagedChrome
	if stagedContentH < 0 {
		stagedContentH = 0
	}

	// Scroll staged viewport to keep cursor visible
	if len(stagedFileLines) > stagedContentH && stagedContentH > 0 {
		if l.wipStagedScroll > l.wipStagedCursor {
			l.wipStagedScroll = l.wipStagedCursor
		}
		if l.wipStagedCursor >= l.wipStagedScroll+stagedContentH {
			l.wipStagedScroll = l.wipStagedCursor - stagedContentH + 1
		}
		maxScroll := len(stagedFileLines) - stagedContentH
		if l.wipStagedScroll > maxScroll {
			l.wipStagedScroll = maxScroll
		}
		if l.wipStagedScroll < 0 {
			l.wipStagedScroll = 0
		}
	} else {
		l.wipStagedScroll = 0
	}

	// Clip staged file lines to viewport
	visibleStaged := stagedFileLines
	if len(visibleStaged) > stagedContentH && stagedContentH > 0 {
		end := l.wipStagedScroll + stagedContentH
		if end > len(visibleStaged) {
			end = len(visibleStaged)
		}
		visibleStaged = visibleStaged[l.wipStagedScroll:end]
	}
	// Pad to fill the viewport
	for len(visibleStaged) < stagedContentH {
		visibleStaged = append(visibleStaged, bgLine(""))
	}

	// Build scroll indicator for staged
	stagedScrollHint := ""
	if len(stagedFileLines) > stagedContentH && stagedContentH > 0 {
		if l.wipStagedScroll > 0 {
			stagedScrollHint += "↑"
		}
		if l.wipStagedScroll+stagedContentH < len(stagedFileLines) {
			if stagedScrollHint != "" {
				stagedScrollHint += " "
			}
			stagedScrollHint += "↓"
		}
	}
	if stagedScrollHint != "" {
		scrollTag := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base).Render(" " + stagedScrollHint)
		stagedHeader = sectionTitle(fmt.Sprintf("▾ Staged (%d) %s", len(l.wipStaged), scrollTag), "s", stagedFocused)
	}

	stagedSection := make([]string, 0, 2+len(visibleStaged))
	stagedSection = append(stagedSection, stagedHeader)
	stagedSection = append(stagedSection, stagedMargin)
	stagedSection = append(stagedSection, visibleStaged...)

	// ---------------------------------------------------------------
	// Assemble the file area from the two sub-viewports
	// ---------------------------------------------------------------
	fileAreaParts := make([]string, 0, len(unstagedSection)+1+len(stagedSection))
	fileAreaParts = append(fileAreaParts, unstagedSection...)
	fileAreaParts = append(fileAreaParts, separator)
	fileAreaParts = append(fileAreaParts, stagedSection...)
	fileContent := strings.Join(fileAreaParts, "\n")

	// ---------------------------------------------------------------
	// Assemble: title + gap + file area + commit area + hints
	// ---------------------------------------------------------------
	full := lipgloss.JoinVertical(lipgloss.Left, titleStr, titleGap, fileContent, commitContent, hintsLine)

	// Safety clip to panel height
	if cl := strings.Split(full, "\n"); len(cl) > ph {
		full = strings.Join(cl[:ph], "\n")
	}
	return styles.ClipPanel(styles.PanelStyleColor(l.borderAnim.Color(anim.BorderRight, t.Surface1, t.Blue)).Width(width).Height(ph).Render(full), height)
}
