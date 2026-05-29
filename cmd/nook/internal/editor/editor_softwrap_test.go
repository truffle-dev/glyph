package editor

import (
	"os"
	"strings"
	"testing"

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
