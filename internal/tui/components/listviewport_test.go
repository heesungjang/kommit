package components

import "testing"

// ---------------------------------------------------------------------------
// NewListViewport
// ---------------------------------------------------------------------------

func TestNewListViewport_InitialState(t *testing.T) {
	vp := NewListViewport(10, 5)
	if vp.Cursor() != 0 {
		t.Errorf("initial cursor = %d, want 0", vp.Cursor())
	}
	if vp.ScrollOffset() != 0 {
		t.Errorf("initial scroll = %d, want 0", vp.ScrollOffset())
	}
}

func TestNewListViewport_NegativeVisibleCount(t *testing.T) {
	vp := NewListViewport(10, -3)
	if vp.ScrollOffset() != 0 {
		t.Errorf("scroll = %d, want 0", vp.ScrollOffset())
	}
}

// ---------------------------------------------------------------------------
// CursorDown / CursorUp
// ---------------------------------------------------------------------------

func TestCursorDown(t *testing.T) {
	vp := NewListViewport(5, 3)
	vp.CursorDown()
	if vp.Cursor() != 1 {
		t.Errorf("after CursorDown: cursor = %d, want 1", vp.Cursor())
	}
}

func TestCursorDown_AtBottom(t *testing.T) {
	vp := NewListViewport(3, 3)
	vp.SetCursor(2)
	vp.CursorDown()
	if vp.Cursor() != 2 {
		t.Errorf("CursorDown at bottom: cursor = %d, want 2", vp.Cursor())
	}
}

func TestCursorUp(t *testing.T) {
	vp := NewListViewport(5, 3)
	vp.SetCursor(3)
	vp.CursorUp()
	if vp.Cursor() != 2 {
		t.Errorf("after CursorUp: cursor = %d, want 2", vp.Cursor())
	}
}

func TestCursorUp_AtTop(t *testing.T) {
	vp := NewListViewport(5, 3)
	vp.CursorUp()
	if vp.Cursor() != 0 {
		t.Errorf("CursorUp at top: cursor = %d, want 0", vp.Cursor())
	}
}

// ---------------------------------------------------------------------------
// PageDown / PageUp
// ---------------------------------------------------------------------------

func TestPageDown(t *testing.T) {
	vp := NewListViewport(20, 5)
	vp.PageDown()
	if vp.Cursor() != 5 {
		t.Errorf("after PageDown: cursor = %d, want 5", vp.Cursor())
	}
}

func TestPageDown_ClampAtEnd(t *testing.T) {
	vp := NewListViewport(8, 5)
	vp.SetCursor(5)
	vp.PageDown()
	if vp.Cursor() != 7 {
		t.Errorf("PageDown clamped: cursor = %d, want 7", vp.Cursor())
	}
}

func TestPageUp(t *testing.T) {
	vp := NewListViewport(20, 5)
	vp.SetCursor(10)
	vp.PageUp()
	if vp.Cursor() != 5 {
		t.Errorf("after PageUp: cursor = %d, want 5", vp.Cursor())
	}
}

func TestPageUp_ClampAtStart(t *testing.T) {
	vp := NewListViewport(20, 5)
	vp.SetCursor(2)
	vp.PageUp()
	if vp.Cursor() != 0 {
		t.Errorf("PageUp clamped: cursor = %d, want 0", vp.Cursor())
	}
}

// ---------------------------------------------------------------------------
// HalfPageDown / HalfPageUp
// ---------------------------------------------------------------------------

func TestHalfPageDown(t *testing.T) {
	vp := NewListViewport(20, 10)
	vp.HalfPageDown()
	if vp.Cursor() != 5 {
		t.Errorf("HalfPageDown: cursor = %d, want 5", vp.Cursor())
	}
}

func TestHalfPageUp(t *testing.T) {
	vp := NewListViewport(20, 10)
	vp.SetCursor(10)
	vp.HalfPageUp()
	if vp.Cursor() != 5 {
		t.Errorf("HalfPageUp: cursor = %d, want 5", vp.Cursor())
	}
}

// ---------------------------------------------------------------------------
// Home / End
// ---------------------------------------------------------------------------

func TestHome(t *testing.T) {
	vp := NewListViewport(20, 5)
	vp.SetCursor(15)
	vp.Home()
	if vp.Cursor() != 0 {
		t.Errorf("Home: cursor = %d, want 0", vp.Cursor())
	}
	if vp.ScrollOffset() != 0 {
		t.Errorf("Home: scroll = %d, want 0", vp.ScrollOffset())
	}
}

func TestEnd(t *testing.T) {
	vp := NewListViewport(20, 5)
	vp.End()
	if vp.Cursor() != 19 {
		t.Errorf("End: cursor = %d, want 19", vp.Cursor())
	}
}

func TestEnd_ZeroItems(t *testing.T) {
	vp := NewListViewport(0, 5)
	vp.End()
	if vp.Cursor() != 0 {
		t.Errorf("End with 0 items: cursor = %d, want 0", vp.Cursor())
	}
}

// ---------------------------------------------------------------------------
// SetCursor
// ---------------------------------------------------------------------------

func TestSetCursor(t *testing.T) {
	vp := NewListViewport(10, 5)
	vp.SetCursor(7)
	if vp.Cursor() != 7 {
		t.Errorf("SetCursor(7): cursor = %d, want 7", vp.Cursor())
	}
}

func TestSetCursor_ClampsHigh(t *testing.T) {
	vp := NewListViewport(5, 3)
	vp.SetCursor(100)
	if vp.Cursor() != 4 {
		t.Errorf("SetCursor(100) clamped: cursor = %d, want 4", vp.Cursor())
	}
}

func TestSetCursor_ClampsNegative(t *testing.T) {
	vp := NewListViewport(5, 3)
	vp.SetCursor(-5)
	if vp.Cursor() != 0 {
		t.Errorf("SetCursor(-5) clamped: cursor = %d, want 0", vp.Cursor())
	}
}

// ---------------------------------------------------------------------------
// SetItemCount
// ---------------------------------------------------------------------------

func TestSetItemCount_ShrinkClampsCursor(t *testing.T) {
	vp := NewListViewport(10, 5)
	vp.SetCursor(8)
	vp.SetItemCount(5)
	if vp.Cursor() != 4 {
		t.Errorf("after shrink: cursor = %d, want 4", vp.Cursor())
	}
}

func TestSetItemCount_GrowLeaveCursor(t *testing.T) {
	vp := NewListViewport(5, 3)
	vp.SetCursor(2)
	vp.SetItemCount(20)
	if vp.Cursor() != 2 {
		t.Errorf("after grow: cursor = %d, want 2", vp.Cursor())
	}
}

func TestSetItemCount_ZeroItems(t *testing.T) {
	vp := NewListViewport(10, 5)
	vp.SetCursor(5)
	vp.SetItemCount(0)
	if vp.Cursor() != 0 {
		t.Errorf("zero items: cursor = %d, want 0", vp.Cursor())
	}
}

// ---------------------------------------------------------------------------
// SetVisibleCount
// ---------------------------------------------------------------------------

func TestSetVisibleCount_ClampsScroll(t *testing.T) {
	vp := NewListViewport(20, 5)
	vp.SetCursor(19) // scroll will be at 15
	vp.SetVisibleCount(10)
	start, end := vp.ViewportRange()
	if vp.Cursor() < start || vp.Cursor() >= end {
		t.Errorf("cursor %d not in viewport [%d, %d)", vp.Cursor(), start, end)
	}
}

func TestSetVisibleCount_NegativeClampedToZero(t *testing.T) {
	vp := NewListViewport(10, 5)
	vp.SetVisibleCount(-1)
	// Should not panic; visibleCount treated as 0.
	start, end := vp.ViewportRange()
	if start != end {
		t.Errorf("viewport range with visible=0: want empty, got [%d, %d)", start, end)
	}
}

// ---------------------------------------------------------------------------
// ViewportRange
// ---------------------------------------------------------------------------

func TestViewportRange_Normal(t *testing.T) {
	vp := NewListViewport(20, 5)
	start, end := vp.ViewportRange()
	if start != 0 || end != 5 {
		t.Errorf("range = [%d, %d), want [0, 5)", start, end)
	}
}

func TestViewportRange_ItemsLessThanVisible(t *testing.T) {
	vp := NewListViewport(3, 10)
	start, end := vp.ViewportRange()
	if start != 0 || end != 3 {
		t.Errorf("range = [%d, %d), want [0, 3)", start, end)
	}
}

func TestViewportRange_ZeroItems(t *testing.T) {
	vp := NewListViewport(0, 5)
	start, end := vp.ViewportRange()
	if start != 0 || end != 0 {
		t.Errorf("range = [%d, %d), want [0, 0)", start, end)
	}
}

func TestViewportRange_AfterScroll(t *testing.T) {
	vp := NewListViewport(20, 5)
	vp.SetCursor(10)
	start, end := vp.ViewportRange()
	if start > 10 || end <= 10 {
		t.Errorf("cursor 10 not visible in [%d, %d)", start, end)
	}
	if end-start != 5 {
		t.Errorf("visible count = %d, want 5", end-start)
	}
}

// ---------------------------------------------------------------------------
// ScrollDown / ScrollUp (viewport scroll without cursor movement)
// ---------------------------------------------------------------------------

func TestScrollDown(t *testing.T) {
	vp := NewListViewport(20, 5)
	vp.ScrollDown(3)
	if vp.ScrollOffset() != 3 {
		t.Errorf("ScrollDown(3): scroll = %d, want 3", vp.ScrollOffset())
	}
	// Cursor should be moved into visible range.
	if vp.Cursor() < vp.ScrollOffset() {
		t.Errorf("cursor %d below scroll offset %d", vp.Cursor(), vp.ScrollOffset())
	}
}

func TestScrollDown_ClampAtMax(t *testing.T) {
	vp := NewListViewport(10, 5)
	vp.ScrollDown(100)
	// Max scroll = 10 - 5 = 5
	if vp.ScrollOffset() != 5 {
		t.Errorf("ScrollDown(100): scroll = %d, want 5", vp.ScrollOffset())
	}
}

func TestScrollUp(t *testing.T) {
	vp := NewListViewport(20, 5)
	vp.SetCursor(15)
	scroll := vp.ScrollOffset()
	vp.ScrollUp(2)
	if vp.ScrollOffset() != scroll-2 {
		t.Errorf("ScrollUp(2): scroll = %d, want %d", vp.ScrollOffset(), scroll-2)
	}
}

func TestScrollUp_ClampAtZero(t *testing.T) {
	vp := NewListViewport(20, 5)
	vp.ScrollUp(100)
	if vp.ScrollOffset() != 0 {
		t.Errorf("ScrollUp(100): scroll = %d, want 0", vp.ScrollOffset())
	}
}

// ---------------------------------------------------------------------------
// IsAtTop / IsAtBottom
// ---------------------------------------------------------------------------

func TestIsAtTop(t *testing.T) {
	vp := NewListViewport(10, 5)
	if !vp.IsAtTop() {
		t.Error("new viewport should be at top")
	}
	vp.CursorDown()
	if vp.IsAtTop() {
		t.Error("after CursorDown, should not be at top")
	}
}

func TestIsAtBottom(t *testing.T) {
	vp := NewListViewport(5, 3)
	if vp.IsAtBottom() {
		t.Error("new viewport should not be at bottom (5 items)")
	}
	vp.End()
	if !vp.IsAtBottom() {
		t.Error("after End, should be at bottom")
	}
}

func TestIsAtBottom_ZeroItems(t *testing.T) {
	vp := NewListViewport(0, 5)
	if !vp.IsAtBottom() {
		t.Error("0-item viewport should report at bottom")
	}
}

func TestIsAtBottom_SingleItem(t *testing.T) {
	vp := NewListViewport(1, 5)
	if !vp.IsAtBottom() {
		t.Error("single-item viewport should be at bottom")
	}
	if !vp.IsAtTop() {
		t.Error("single-item viewport should be at top")
	}
}

// ---------------------------------------------------------------------------
// Auto-scroll: cursor movement adjusts scroll
// ---------------------------------------------------------------------------

func TestAutoScroll_CursorDownPastViewport(t *testing.T) {
	vp := NewListViewport(20, 5)
	for i := 0; i < 7; i++ {
		vp.CursorDown()
	}
	if vp.Cursor() != 7 {
		t.Fatalf("cursor = %d, want 7", vp.Cursor())
	}
	start, end := vp.ViewportRange()
	if vp.Cursor() < start || vp.Cursor() >= end {
		t.Errorf("cursor %d not in viewport [%d, %d)", vp.Cursor(), start, end)
	}
}

func TestAutoScroll_CursorUpBeforeViewport(t *testing.T) {
	vp := NewListViewport(20, 5)
	vp.SetCursor(15)
	for i := 0; i < 10; i++ {
		vp.CursorUp()
	}
	if vp.Cursor() != 5 {
		t.Fatalf("cursor = %d, want 5", vp.Cursor())
	}
	start, end := vp.ViewportRange()
	if vp.Cursor() < start || vp.Cursor() >= end {
		t.Errorf("cursor %d not in viewport [%d, %d)", vp.Cursor(), start, end)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestEdge_ZeroItems(t *testing.T) {
	vp := NewListViewport(0, 5)
	// None of these should panic.
	vp.CursorDown()
	vp.CursorUp()
	vp.PageDown()
	vp.PageUp()
	vp.HalfPageDown()
	vp.HalfPageUp()
	vp.Home()
	vp.End()
	vp.ScrollDown(1)
	vp.ScrollUp(1)
	if vp.Cursor() != 0 {
		t.Errorf("cursor = %d, want 0", vp.Cursor())
	}
}

func TestEdge_SingleItem(t *testing.T) {
	vp := NewListViewport(1, 5)
	vp.CursorDown()
	if vp.Cursor() != 0 {
		t.Errorf("CursorDown on 1 item: cursor = %d, want 0", vp.Cursor())
	}
	vp.PageDown()
	if vp.Cursor() != 0 {
		t.Errorf("PageDown on 1 item: cursor = %d, want 0", vp.Cursor())
	}
	start, end := vp.ViewportRange()
	if start != 0 || end != 1 {
		t.Errorf("range = [%d, %d), want [0, 1)", start, end)
	}
}

func TestEdge_VisibleCountGreaterThanItems(t *testing.T) {
	vp := NewListViewport(3, 100)
	vp.End()
	if vp.Cursor() != 2 {
		t.Errorf("End: cursor = %d, want 2", vp.Cursor())
	}
	start, end := vp.ViewportRange()
	if start != 0 || end != 3 {
		t.Errorf("range = [%d, %d), want [0, 3)", start, end)
	}
	if vp.ScrollOffset() != 0 {
		t.Errorf("scroll = %d, want 0", vp.ScrollOffset())
	}
}

func TestEdge_ZeroVisibleCount(t *testing.T) {
	vp := NewListViewport(10, 0)
	vp.CursorDown()
	if vp.Cursor() != 1 {
		t.Errorf("cursor = %d, want 1", vp.Cursor())
	}
	start, end := vp.ViewportRange()
	if start != end {
		t.Errorf("range with visible=0: [%d, %d) should be empty", start, end)
	}
}

// ---------------------------------------------------------------------------
// Full traversal: cursor walks entire list
// ---------------------------------------------------------------------------

func TestFullTraversal(t *testing.T) {
	n := 50
	vp := NewListViewport(n, 7)
	// Walk down to the end.
	for i := 0; i < n-1; i++ {
		vp.CursorDown()
	}
	if vp.Cursor() != n-1 {
		t.Errorf("after walking down: cursor = %d, want %d", vp.Cursor(), n-1)
	}
	if !vp.IsAtBottom() {
		t.Error("should be at bottom")
	}
	// Walk back to the top.
	for i := 0; i < n-1; i++ {
		vp.CursorUp()
	}
	if vp.Cursor() != 0 {
		t.Errorf("after walking up: cursor = %d, want 0", vp.Cursor())
	}
	if !vp.IsAtTop() {
		t.Error("should be at top")
	}
}
