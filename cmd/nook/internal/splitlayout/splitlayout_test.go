package splitlayout

import (
	"reflect"
	"testing"
)

func TestNewSinglePane(t *testing.T) {
	tr, id := New()
	if tr.Count() != 1 {
		t.Fatalf("Count = %d, want 1", tr.Count())
	}
	if tr.Focused() != id {
		t.Fatalf("Focused = %d, want %d", tr.Focused(), id)
	}
	rects := tr.Rects(80, 24)
	if got := rects[id]; got != (Rect{0, 0, 80, 24}) {
		t.Fatalf("single pane rect = %+v, want full screen", got)
	}
}

func TestSplitColumnsGeometryAndFocus(t *testing.T) {
	tr, first := New()
	second := tr.SplitFocused(Columns)

	if tr.Focused() != second {
		t.Fatalf("focus did not move to new pane: got %d want %d", tr.Focused(), second)
	}
	if tr.Count() != 2 {
		t.Fatalf("Count = %d, want 2", tr.Count())
	}

	// width 81: divider takes 1, 80 split evenly -> 40 / 40, divider at x=40.
	rects := tr.Rects(81, 24)
	if got := rects[first]; got != (Rect{0, 0, 40, 24}) {
		t.Fatalf("first rect = %+v, want {0 0 40 24}", got)
	}
	if got := rects[second]; got != (Rect{41, 0, 40, 24}) {
		t.Fatalf("second rect = %+v, want {41 0 40 24}", got)
	}
}

func TestSplitRowsGeometry(t *testing.T) {
	tr, first := New()
	second := tr.SplitFocused(Rows)

	// height 25: divider 1, 24 split evenly -> 12 / 12, divider row at y=12.
	rects := tr.Rects(80, 25)
	if got := rects[first]; got != (Rect{0, 0, 80, 12}) {
		t.Fatalf("first rect = %+v, want {0 0 80 12}", got)
	}
	if got := rects[second]; got != (Rect{0, 13, 80, 12}) {
		t.Fatalf("second rect = %+v, want {0 13 80 12}", got)
	}
}

func TestPanesOrderStable(t *testing.T) {
	tr, a := New()
	b := tr.SplitFocused(Columns) // tree: [a | b], focus b
	tr.Focus(b)
	c := tr.SplitFocused(Rows) // b becomes [b / c], focus c

	got := tr.Panes()
	want := []PaneID{a, b, c}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Panes order = %v, want %v", got, want)
	}
}

func TestCloseFocusedCollapses(t *testing.T) {
	tr, first := New()
	second := tr.SplitFocused(Columns) // focus second
	if !tr.CloseFocused() {
		t.Fatal("CloseFocused returned false on a 2-pane tree")
	}
	if tr.Count() != 1 {
		t.Fatalf("Count after close = %d, want 1", tr.Count())
	}
	if tr.Focused() != first {
		t.Fatalf("focus after close = %d, want %d (the sibling)", tr.Focused(), first)
	}
	// The survivor reclaims the whole screen.
	if got := tr.Rects(80, 24)[first]; got != (Rect{0, 0, 80, 24}) {
		t.Fatalf("survivor rect = %+v, want full screen", got)
	}
	if _, gone := tr.Rects(80, 24)[second]; gone {
		t.Fatal("closed pane still has a rect")
	}
}

func TestCloseLastPaneRefused(t *testing.T) {
	tr, _ := New()
	if tr.CloseFocused() {
		t.Fatal("CloseFocused should refuse to close the only pane")
	}
	if tr.Count() != 1 {
		t.Fatalf("Count = %d, want 1", tr.Count())
	}
}

func TestFocusNextPrevWrap(t *testing.T) {
	tr, a := New()
	b := tr.SplitFocused(Columns)
	tr.Focus(b)
	c := tr.SplitFocused(Columns) // order: a, b, c ; focus c

	tr.Focus(a)
	tr.FocusPrev()
	if tr.Focused() != c {
		t.Fatalf("prev from first = %d, want wrap to %d", tr.Focused(), c)
	}
	tr.FocusNext()
	if tr.Focused() != a {
		t.Fatalf("next from last = %d, want wrap to %d", tr.Focused(), a)
	}
}

func TestFocusDir(t *testing.T) {
	// Build a 2x2 grid:
	//   top:    a | b
	//   bottom: c | d
	tr, a := New()
	b := tr.SplitFocused(Columns) // [a | b], focus b
	tr.Focus(a)
	c := tr.SplitFocused(Rows) // a -> [a / c], focus c
	tr.Focus(b)
	d := tr.SplitFocused(Rows) // b -> [b / d], focus d

	const w, h = 81, 25
	tr.Focus(a)
	if !tr.FocusDir(Right, w, h) || tr.Focused() != b {
		t.Fatalf("Right from a -> %d, want %d", tr.Focused(), b)
	}
	if !tr.FocusDir(Down, w, h) || tr.Focused() != d {
		t.Fatalf("Down from b -> %d, want %d", tr.Focused(), d)
	}
	if !tr.FocusDir(Left, w, h) || tr.Focused() != c {
		t.Fatalf("Left from d -> %d, want %d", tr.Focused(), c)
	}
	if !tr.FocusDir(Up, w, h) || tr.Focused() != a {
		t.Fatalf("Up from c -> %d, want %d", tr.Focused(), a)
	}
}

func TestFocusDirNoNeighbor(t *testing.T) {
	tr, a := New()
	b := tr.SplitFocused(Columns)
	tr.Focus(a)
	if tr.FocusDir(Left, 81, 24) {
		t.Fatal("Left from leftmost pane should report no move")
	}
	if tr.Focused() != a {
		t.Fatalf("focus changed on a no-op FocusDir: got %d", tr.Focused())
	}
	if tr.FocusDir(Up, 81, 24) {
		t.Fatal("Up in a columns-only layout should report no move")
	}
	_ = b
}

func TestResizeFocused(t *testing.T) {
	tr, first := New()
	second := tr.SplitFocused(Columns) // focus second (child b)

	// Focused is child b; positive delta should grow b (shrink a).
	if !tr.ResizeFocused(0.25) {
		t.Fatal("ResizeFocused returned false on a split tree")
	}
	rects := tr.Rects(101, 24) // avail 100, ratio now 0.25 -> a=25, b=75
	if got := rects[first].W; got != 25 {
		t.Fatalf("first width after resize = %d, want 25", got)
	}
	if got := rects[second].W; got != 75 {
		t.Fatalf("second width after resize = %d, want 75", got)
	}
}

func TestResizeClamps(t *testing.T) {
	tr, _ := New()
	tr.SplitFocused(Columns)
	tr.ResizeFocused(100) // absurd shrink of focused (child b)
	rects := tr.Rects(101, 24)
	// ratio clamps to maxRatio 0.95 -> a gets 95, b gets 5; neither vanishes.
	for id, r := range rects {
		if r.W <= 0 {
			t.Fatalf("pane %d collapsed to width %d after clamped resize", id, r.W)
		}
	}
}

func TestResizeSinglePaneRefused(t *testing.T) {
	tr, _ := New()
	if tr.ResizeFocused(0.1) {
		t.Fatal("ResizeFocused should report false with no parent split")
	}
}

func TestRectsTinySizeNoNegative(t *testing.T) {
	tr, _ := New()
	tr.SplitFocused(Columns)
	tr.SplitFocused(Columns) // three columns sharing too little space
	for id, r := range tr.Rects(2, 24) {
		if r.W < 0 || r.H < 0 {
			t.Fatalf("pane %d has negative size %+v in a tiny layout", id, r)
		}
	}
}
