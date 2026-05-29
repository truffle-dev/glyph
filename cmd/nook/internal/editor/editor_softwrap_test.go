package editor

import (
	"os"
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/cmd/nook/internal/dochi"
	"github.com/truffle-dev/glyph/cmd/nook/internal/highlight"
	"github.com/truffle-dev/glyph/cmd/nook/internal/inlayhint"
	"github.com/truffle-dev/glyph/components/theme"
)

func writeTestFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}

func newSoftWrapPane(t *testing.T) Pane {
	t.Helper()
	p := NewPane(theme.Default)
	p.width = 30
	p.height = 6
	return p
}

func TestSoftWrapDefaultOff(t *testing.T) {
	p := NewPane(theme.Default)
	if p.SoftWrap() {
		t.Fatalf("SoftWrap default should be false")
	}
}

func TestSetSoftWrapToggles(t *testing.T) {
	p := NewPane(theme.Default).SetSoftWrap(true)
	if !p.SoftWrap() {
		t.Fatalf("SetSoftWrap(true) did not stick")
	}
	p = p.SetSoftWrap(false)
	if p.SoftWrap() {
		t.Fatalf("SetSoftWrap(false) did not clear")
	}
}

func TestContentWidthGutterAware(t *testing.T) {
	p := newSoftWrapPane(t)
	p.buf.Lines = []string{strings.Repeat("a", 80)}
	cwLines := p.contentWidth()
	p2 := p.SetLineNumbers(false)
	cwNoLines := p2.contentWidth()
	if cwNoLines <= cwLines {
		t.Fatalf("contentWidth without line numbers should be wider: with=%d without=%d", cwLines, cwNoLines)
	}
}

func TestContentWidthClampsAtOne(t *testing.T) {
	p := NewPane(theme.Default)
	p.width = 1
	p.buf.Lines = []string{"x"}
	if cw := p.contentWidth(); cw < 1 {
		t.Fatalf("contentWidth should clamp at 1, got %d", cw)
	}
}

func TestVisualRowsBeforeSoftWrapOff(t *testing.T) {
	p := newSoftWrapPane(t)
	p.buf.Lines = []string{"a", strings.Repeat("b", 200), "c", "d"}
	if got := p.visualRowsBefore(2); got != 2 {
		t.Fatalf("visualRowsBefore(2) softwrap-off = %d want 2", got)
	}
}

func TestVisualRowsBeforeSoftWrapOnCountsWraps(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	long := strings.Repeat("a", 200)
	p.buf.Lines = []string{"short", long, "short2", "short3"}
	cw := p.contentWidth()
	expectedRow1 := 1 + (200+cw-1)/cw
	if got := p.visualRowsBefore(2); got != expectedRow1 {
		t.Fatalf("visualRowsBefore(2) softwrap-on = %d want %d", got, expectedRow1)
	}
	if got := p.visualRowsBefore(0); got != 0 {
		t.Fatalf("visualRowsBefore(0)=%d want 0", got)
	}
}

func TestCursorSubRowSoftWrapOffIsZero(t *testing.T) {
	p := newSoftWrapPane(t)
	p.buf.Lines = []string{strings.Repeat("a", 200)}
	if got := p.cursorSubRow(0, 150); got != 0 {
		t.Fatalf("cursorSubRow softwrap-off = %d want 0", got)
	}
}

func TestCursorSubRowSoftWrapOnFindsSubRow(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	long := strings.Repeat("a", 200)
	p.buf.Lines = []string{long}
	cw := p.contentWidth()
	if got := p.cursorSubRow(0, 0); got != 0 {
		t.Fatalf("cursorSubRow col=0 = %d want 0", got)
	}
	if got := p.cursorSubRow(0, cw-1); got != 0 {
		t.Fatalf("cursorSubRow col=cw-1 = %d want 0", got)
	}
	if got := p.cursorSubRow(0, cw); got != 1 {
		t.Fatalf("cursorSubRow col=cw = %d want 1", got)
	}
	if got := p.cursorSubRow(0, 2*cw); got != 2 {
		t.Fatalf("cursorSubRow col=2*cw = %d want 2", got)
	}
}

func TestCursorSubRowOutOfRangeRowReturnsZero(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	p.buf.Lines = []string{"a"}
	if got := p.cursorSubRow(5, 0); got != 0 {
		t.Fatalf("cursorSubRow out-of-range = %d want 0", got)
	}
	if got := p.cursorSubRow(-1, 0); got != 0 {
		t.Fatalf("cursorSubRow negative row = %d want 0", got)
	}
}

func TestLogicalAtVisualOffsetSoftWrapOff(t *testing.T) {
	p := newSoftWrapPane(t)
	p.buf.Lines = []string{"a", "b", "c"}
	r, sub := p.logicalAtVisualOffset(1)
	if r != 1 || sub != 0 {
		t.Fatalf("logicalAtVisualOffset(1) softwrap-off = (%d,%d) want (1,0)", r, sub)
	}
}

func TestLogicalAtVisualOffsetInverseOfVisualRowsBefore(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	p.buf.Lines = []string{"short", strings.Repeat("a", 90), "x"}
	cw := p.contentWidth()
	rowCount := func(line string) int {
		n := len(line) / cw
		if len(line)%cw != 0 {
			n++
		}
		if n == 0 {
			n = 1
		}
		return n
	}
	visualOfRow0End := 0
	visualOfRow1End := visualOfRow0End + rowCount(p.buf.Lines[0])
	r, sub := p.logicalAtVisualOffset(visualOfRow1End)
	if r != 1 || sub != 0 {
		t.Fatalf("logicalAtVisualOffset at row-1 start = (%d,%d) want (1,0)", r, sub)
	}
	r, sub = p.logicalAtVisualOffset(visualOfRow1End + 2)
	if r != 1 || sub != 2 {
		t.Fatalf("logicalAtVisualOffset row-1 sub-2 = (%d,%d) want (1,2)", r, sub)
	}
}

func TestLogicalAtVisualOffsetPastEndClamps(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	p.buf.Lines = []string{"a", "b"}
	r, sub := p.logicalAtVisualOffset(10_000)
	if r != len(p.buf.Lines) || sub != 0 {
		t.Fatalf("logicalAtVisualOffset past end = (%d,%d) want (%d,0)", r, sub, len(p.buf.Lines))
	}
}

func TestEnsureVisibleSoftWrapOffUnchanged(t *testing.T) {
	p := newSoftWrapPane(t)
	p.buf.Lines = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}
	p.row = 9
	p.col = 0
	(&p).ensureVisible()
	if p.offset == 0 {
		t.Fatalf("ensureVisible softwrap-off should scroll, offset=%d", p.offset)
	}
	if p.subOffset != 0 {
		t.Fatalf("ensureVisible softwrap-off must not touch subOffset, got %d", p.subOffset)
	}
}

func TestEnsureVisibleSoftWrapOnBringsWrappedCursorIntoView(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	long := strings.Repeat("a", 300)
	p.buf.Lines = []string{long, "b"}
	cw := p.contentWidth()
	p.row = 0
	p.col = 4 * cw
	(&p).ensureVisible()
	visible := p.height - 1
	visualCursor := p.visualRowsBefore(0) + p.cursorSubRow(0, p.col)
	visualTop := p.visualRowsBefore(p.offset) + p.subOffset
	if visualCursor < visualTop || visualCursor >= visualTop+visible {
		t.Fatalf("ensureVisible left cursor off-screen: cursor=%d top=%d visible=%d", visualCursor, visualTop, visible)
	}
}

func TestEnsureVisibleSoftWrapOnDoesNotScrollWhenCursorAlreadyVisible(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	p.buf.Lines = []string{"hello", "world"}
	p.row = 1
	p.col = 0
	(&p).ensureVisible()
	if p.offset != 0 || p.subOffset != 0 {
		t.Fatalf("short content should not scroll: offset=%d subOffset=%d", p.offset, p.subOffset)
	}
}

func TestScrollToShowVisualAboveTopBringsCursorTop(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	long := strings.Repeat("a", 200)
	p.buf.Lines = []string{long, "x", "y", "z"}
	p.offset = 3
	p.subOffset = 0
	(&p).scrollToShowVisual(0, 0)
	if p.offset != 0 || p.subOffset != 0 {
		t.Fatalf("scrollToShowVisual back to top = (%d,%d) want (0,0)", p.offset, p.subOffset)
	}
}

func TestOpenResetsSubOffset(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	long := strings.Repeat("a", 600)
	p.buf.Lines = []string{long}
	p.row = 0
	p.col = 15 * p.contentWidth()
	(&p).ensureVisible()
	if p.subOffset == 0 {
		t.Fatalf("test setup did not advance subOffset")
	}
	tmp := t.TempDir() + "/fresh.txt"
	if err := writeTestFile(tmp, "fresh\n"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	p = p.Open(tmp)
	if p.subOffset != 0 {
		t.Fatalf("Open left subOffset=%d, want 0", p.subOffset)
	}
}

func TestSoftWrapVisualRowsBeforeNeverExceedsLineCount(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	p.buf.Lines = []string{"a", "b"}
	if got := p.visualRowsBefore(1000); got <= 0 {
		t.Fatalf("visualRowsBefore clamp returned %d", got)
	}
}

// --- slice 3: sub-row slicing helpers -----------------------------------

func TestSliceSpansForSubrowFiltersAndShifts(t *testing.T) {
	spans := []highlight.Span{
		{Start: 0, End: 4, Kind: highlight.KindKeyword},
		{Start: 6, End: 10, Kind: highlight.KindString},
		{Start: 12, End: 18, Kind: highlight.KindFunction},
	}
	got := sliceSpansForSubrow(spans, 5, 15)
	if len(got) != 2 {
		t.Fatalf("got %d spans want 2: %+v", len(got), got)
	}
	if got[0].Start != 1 || got[0].End != 5 || got[0].Kind != highlight.KindString {
		t.Fatalf("got[0] = %+v", got[0])
	}
	if got[1].Start != 7 || got[1].End != 10 || got[1].Kind != highlight.KindFunction {
		t.Fatalf("got[1] = %+v", got[1])
	}
}

func TestSliceSpansForSubrowEmpty(t *testing.T) {
	if out := sliceSpansForSubrow(nil, 0, 10); out != nil {
		t.Fatalf("nil input returned %v", out)
	}
	if out := sliceSpansForSubrow([]highlight.Span{{Start: 0, End: 5}}, 5, 5); out != nil {
		t.Fatalf("zero-width window returned %v", out)
	}
}

func TestSliceMatchesForSubrowRemapsActiveIdx(t *testing.T) {
	marks := []Range{
		{Row: 0, Start: 0, End: 3},
		{Row: 0, Start: 5, End: 8},
		{Row: 0, Start: 12, End: 14},
	}
	got, active := sliceMatchesForSubrow(marks, 1, 4, 10)
	if len(got) != 1 {
		t.Fatalf("got %d marks want 1: %+v", len(got), got)
	}
	if got[0].Start != 1 || got[0].End != 4 {
		t.Fatalf("got[0] = %+v", got[0])
	}
	if active != 0 {
		t.Fatalf("active = %d want 0 (the originally-active mark remapped)", active)
	}
}

func TestSliceMatchesForSubrowActiveOutsideReturnsMinusOne(t *testing.T) {
	marks := []Range{
		{Row: 0, Start: 0, End: 3},
		{Row: 0, Start: 5, End: 8},
	}
	_, active := sliceMatchesForSubrow(marks, 0, 0, 4)
	if active != 0 {
		t.Fatalf("active for sub-row containing mark 0 = %d want 0", active)
	}
	_, active = sliceMatchesForSubrow(marks, 0, 4, 10)
	if active != -1 {
		t.Fatalf("active for sub-row NOT containing mark 0 = %d want -1", active)
	}
}

func TestSliceDochiForSubrowEndMinusOneExtends(t *testing.T) {
	spans := []dochi.Span{
		{Start: 2, End: -1},
	}
	got := sliceDochiForSubrow(spans, 0, 8, 20)
	if len(got) != 1 {
		t.Fatalf("got %d want 1: %+v", len(got), got)
	}
	if got[0].Start != 2 || got[0].End != 8 {
		t.Fatalf("got[0] = %+v want {Start:2 End:8}", got[0])
	}
}

func TestSliceHintsForSubrowBoundaryRule(t *testing.T) {
	hints := []inlayhint.Hint{
		{Row: 0, Col: 0, Label: "a"},
		{Row: 0, Col: 5, Label: "b"},
		{Row: 0, Col: 8, Label: "c"},
		{Row: 0, Col: 12, Label: "d"},
	}
	got := sliceHintsForSubrow(hints, 0, 8, 20, false)
	if len(got) != 2 {
		t.Fatalf("sub-row [0,8) on non-last got %d want 2: %+v", len(got), got)
	}
	got = sliceHintsForSubrow(hints, 8, 12, 20, false)
	if len(got) != 1 || got[0].Col != 0 {
		t.Fatalf("hint at boundary col=8 should land on next sub-row at col=0: %+v", got)
	}
	got = sliceHintsForSubrow([]inlayhint.Hint{{Row: 0, Col: 20, Label: "eol"}}, 12, 20, 20, true)
	if len(got) != 1 || got[0].Col != 8 {
		t.Fatalf("EOL hint on last sub-row should land at end: %+v", got)
	}
}

func TestSliceCursorsForSubrowBoundary(t *testing.T) {
	cols := []int{0, 5, 10}
	got := sliceCursorsForSubrow(cols, 0, 8, 12, false)
	if len(got) != 2 || got[0] != 0 || got[1] != 5 {
		t.Fatalf("sub-row [0,8) cursors = %v", got)
	}
	got = sliceCursorsForSubrow([]int{12}, 8, 12, 12, true)
	if len(got) != 1 || got[0] != 4 {
		t.Fatalf("EOL cursor on last sub-row = %v want [4]", got)
	}
	got = sliceCursorsForSubrow([]int{12}, 8, 12, 12, false)
	if len(got) != 0 {
		t.Fatalf("EOL cursor on non-last sub-row should be empty: %v", got)
	}
}

func TestSliceSelectionForSubrowOverlap(t *testing.T) {
	s, e, tail := sliceSelectionForSubrow(2, 6, false, 0, 8, 8, true)
	if s != 2 || e != 6 || tail {
		t.Fatalf("overlap within = (%d,%d,%v) want (2,6,false)", s, e, tail)
	}
}

func TestSliceSelectionForSubrowSelTailOnlyOnLast(t *testing.T) {
	_, _, tailNonLast := sliceSelectionForSubrow(0, 20, true, 0, 8, 20, false)
	_, _, tailLast := sliceSelectionForSubrow(0, 20, true, 12, 20, 20, true)
	if tailNonLast {
		t.Fatalf("selTail should NOT propagate on non-last sub-row")
	}
	if !tailLast {
		t.Fatalf("selTail SHOULD propagate on last sub-row when parent selTail")
	}
}

func TestSliceSelectionForSubrowNoOverlap(t *testing.T) {
	s, e, tail := sliceSelectionForSubrow(10, 12, false, 0, 5, 20, false)
	if s != -1 || e != -1 || tail {
		t.Fatalf("no overlap = (%d,%d,%v) want (-1,-1,false)", s, e, tail)
	}
}

func TestSubRowPrimaryColEOLOnLastSub(t *testing.T) {
	if got := subRowPrimaryCol(20, 12, 20, 20, true); got != 8 {
		t.Fatalf("EOL primary on last sub = %d want 8", got)
	}
	if got := subRowPrimaryCol(20, 0, 8, 20, false); got != -1 {
		t.Fatalf("EOL primary on non-last sub = %d want -1", got)
	}
	if got := subRowPrimaryCol(4, 0, 8, 20, false); got != 4 {
		t.Fatalf("mid primary = %d want 4", got)
	}
}

// --- slice 3: View() integration ----------------------------------------

func TestViewSoftWrapOffEmitsOneRowPerLogical(t *testing.T) {
	p := newSoftWrapPane(t)
	p.buf.Lines = []string{"alpha", "beta", "gamma"}
	out := p.View()
	rows := strings.Split(out, "\n")
	if len(rows) != p.height {
		t.Fatalf("softwrap off should emit p.height=%d rows, got %d", p.height, len(rows))
	}
	r1 := plain(rows[1])
	r2 := plain(rows[2])
	if !strings.Contains(r1, "alpha") || !strings.Contains(r2, "beta") {
		t.Fatalf("logical rows misplaced:\n%s", strings.Join(rows[:4], "\n"))
	}
}

func TestViewSoftWrapOnEmitsMultipleSubRowsPerLogical(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	long := strings.Repeat("a", 100)
	p.buf.Lines = []string{long}
	p.height = 8
	out := p.View()
	rows := strings.Split(out, "\n")
	visible := p.height - 1
	cw := p.contentWidth()
	expected := (100 + cw - 1) / cw
	if expected > visible {
		expected = visible
	}
	aRows := 0
	for _, r := range rows[1:] {
		if strings.Contains(r, "a") {
			aRows++
		}
	}
	if aRows < expected {
		t.Fatalf("expected at least %d sub-rows containing 'a', got %d:\n%s", expected, aRows, out)
	}
}

func TestViewSoftWrapOnLineNumberOnlyOnFirstSubRow(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	long := strings.Repeat("a", 100)
	p.buf.Lines = []string{long, "next"}
	p.height = 8
	out := p.View()
	rows := strings.Split(out, "\n")
	lineNumberRows := 0
	for _, r := range rows[1:] {
		clean := plain(r)
		if strings.Contains(clean, "1") && strings.Contains(clean, "a") {
			lineNumberRows++
		}
	}
	if lineNumberRows != 1 {
		t.Fatalf("line number '1' should appear on exactly one sub-row, found %d:\n%s", lineNumberRows, out)
	}
}

func TestViewSoftWrapOnSubOffsetSkipsLeadingSubRows(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	long := strings.Repeat("a", 200)
	p.buf.Lines = []string{long}
	p.height = 6
	p.subOffset = 2
	out := p.View()
	rows := strings.Split(out, "\n")
	visible := p.height - 1
	cw := p.contentWidth()
	totalSubs := (200 + cw - 1) / cw
	remaining := totalSubs - 2
	if remaining > visible {
		remaining = visible
	}
	aRows := 0
	for _, r := range rows[1:] {
		if strings.Contains(r, "a") {
			aRows++
		}
	}
	if aRows != remaining {
		t.Fatalf("subOffset=2 should leave %d sub-rows visible, got %d:\n%s", remaining, aRows, out)
	}
}

func TestViewSoftWrapOnRenderHandlesEmptyAndShortLines(t *testing.T) {
	p := newSoftWrapPane(t).SetSoftWrap(true)
	p.buf.Lines = []string{"", "x", ""}
	out := p.View()
	rows := strings.Split(out, "\n")
	if len(rows) != p.height {
		t.Fatalf("View should pad to p.height=%d, got %d", p.height, len(rows))
	}
	if !strings.Contains(rows[2], "x") {
		t.Fatalf("short line missing:\n%s", out)
	}
}
