package components

// ListViewport provides cursor-tracked scrolling over a list of items.
// It manages a cursor position and a persistent scroll offset, ensuring the
// cursor is always visible within the configured viewport height. This
// consolidates the boilerplate cursor+scroll patterns used by the commit
// list, sidebar, detail file list, WIP panels, and more.
//
// Usage:
//
//	vp := NewListViewport(len(items), panelHeight)
//	vp.CursorDown()
//	start, end := vp.ViewportRange()
//	visible := items[start:end]
type ListViewport struct {
	cursor       int
	scrollOffset int
	itemCount    int
	visibleCount int
}

// NewListViewport creates a ListViewport with the given item count and visible
// height (number of items that fit on screen).
func NewListViewport(itemCount, visibleCount int) ListViewport {
	if visibleCount < 0 {
		visibleCount = 0
	}
	return ListViewport{
		itemCount:    itemCount,
		visibleCount: visibleCount,
	}
}

// Cursor returns the current cursor position.
func (v *ListViewport) Cursor() int { return v.cursor }

// ScrollOffset returns the current scroll offset (first visible index).
func (v *ListViewport) ScrollOffset() int { return v.scrollOffset }

// SetCursor sets the cursor to a specific position, clamped to valid bounds,
// and adjusts the scroll offset so the cursor is visible.
func (v *ListViewport) SetCursor(idx int) {
	v.cursor = idx
	v.clampCursor()
	v.ensureVisible()
}

// SetItemCount updates the item count (e.g. after data reload) and clamps the
// cursor and scroll offset.
func (v *ListViewport) SetItemCount(n int) {
	v.itemCount = n
	v.clampCursor()
	v.clampScroll()
	v.ensureVisible()
}

// SetVisibleCount updates the viewport height (e.g. after window resize) and
// adjusts the scroll offset.
func (v *ListViewport) SetVisibleCount(n int) {
	if n < 0 {
		n = 0
	}
	v.visibleCount = n
	v.clampScroll()
	v.ensureVisible()
}

// CursorDown moves the cursor down by one item.
func (v *ListViewport) CursorDown() {
	if v.cursor < v.itemCount-1 {
		v.cursor++
	}
	v.ensureVisible()
}

// CursorUp moves the cursor up by one item.
func (v *ListViewport) CursorUp() {
	if v.cursor > 0 {
		v.cursor--
	}
	v.ensureVisible()
}

// PageDown moves the cursor down by one page.
func (v *ListViewport) PageDown() {
	v.cursor += v.visibleCount
	v.clampCursor()
	v.ensureVisible()
}

// PageUp moves the cursor up by one page.
func (v *ListViewport) PageUp() {
	v.cursor -= v.visibleCount
	v.clampCursor()
	v.ensureVisible()
}

// HalfPageDown moves the cursor down by half a page.
func (v *ListViewport) HalfPageDown() {
	v.cursor += v.visibleCount / 2
	v.clampCursor()
	v.ensureVisible()
}

// HalfPageUp moves the cursor up by half a page.
func (v *ListViewport) HalfPageUp() {
	v.cursor -= v.visibleCount / 2
	v.clampCursor()
	v.ensureVisible()
}

// Home moves the cursor to the first item.
func (v *ListViewport) Home() {
	v.cursor = 0
	v.scrollOffset = 0
}

// End moves the cursor to the last item.
func (v *ListViewport) End() {
	if v.itemCount > 0 {
		v.cursor = v.itemCount - 1
	}
	v.ensureVisible()
}

// ViewportRange returns the [start, end) range of visible items. Use this to
// slice the item list for rendering: items[start:end].
func (v *ListViewport) ViewportRange() (start, end int) {
	v.clampScroll()
	start = v.scrollOffset
	end = start + v.visibleCount
	if end > v.itemCount {
		end = v.itemCount
	}
	if start > end {
		start = end
	}
	return start, end
}

// ScrollDown scrolls the viewport down by n lines without moving the cursor.
// The cursor is clamped to stay within the visible area.
func (v *ListViewport) ScrollDown(n int) {
	v.scrollOffset += n
	v.clampScroll()
	// Keep cursor in visible area.
	if v.cursor < v.scrollOffset {
		v.cursor = v.scrollOffset
	}
	v.clampCursor()
}

// ScrollUp scrolls the viewport up by n lines without moving the cursor.
// The cursor is clamped to stay within the visible area.
func (v *ListViewport) ScrollUp(n int) {
	v.scrollOffset -= n
	if v.scrollOffset < 0 {
		v.scrollOffset = 0
	}
	// Keep cursor in visible area.
	if v.visibleCount > 0 && v.cursor >= v.scrollOffset+v.visibleCount {
		v.cursor = v.scrollOffset + v.visibleCount - 1
	}
	v.clampCursor()
}

// IsAtTop returns true if the cursor is at the first item.
func (v *ListViewport) IsAtTop() bool { return v.cursor == 0 }

// IsAtBottom returns true if the cursor is at the last item.
func (v *ListViewport) IsAtBottom() bool {
	return v.itemCount == 0 || v.cursor == v.itemCount-1
}

// clampCursor ensures the cursor is within [0, itemCount-1].
func (v *ListViewport) clampCursor() {
	if v.itemCount == 0 {
		v.cursor = 0
		return
	}
	if v.cursor >= v.itemCount {
		v.cursor = v.itemCount - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}

// clampScroll ensures the scroll offset is within valid bounds.
func (v *ListViewport) clampScroll() {
	if v.visibleCount <= 0 || v.itemCount <= v.visibleCount {
		v.scrollOffset = 0
		return
	}
	maxScroll := v.itemCount - v.visibleCount
	if v.scrollOffset > maxScroll {
		v.scrollOffset = maxScroll
	}
	if v.scrollOffset < 0 {
		v.scrollOffset = 0
	}
}

// ensureVisible adjusts the scroll offset so the cursor is in the viewport.
func (v *ListViewport) ensureVisible() {
	if v.visibleCount <= 0 {
		return
	}
	// Cursor above viewport — scroll up.
	if v.cursor < v.scrollOffset {
		v.scrollOffset = v.cursor
	}
	// Cursor below viewport — scroll down.
	if v.cursor >= v.scrollOffset+v.visibleCount {
		v.scrollOffset = v.cursor - v.visibleCount + 1
	}
	v.clampScroll()
}
