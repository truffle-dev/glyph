package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

// openGoBuf creates a tmp .go file with body, opens it in a Pane sized
// for tests, and returns the Pane positioned at (0, 0).
func openGoBuf(t *testing.T, body string) Pane {
	t.Helper()
	d := t.TempDir()
	path := filepath.Join(d, "main.go")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewPane(theme.Default).WithSize(60, 8).Open(path).Focus()
}

func TestToggleCommentSingleLineComments(t *testing.T) {
	p := openGoBuf(t, "x := 1\n")
	p.row, p.col = 0, 6
	p = p.ToggleComment()
	if got := p.Line(0); got != "// x := 1" {
		t.Errorf("Line 0 = %q, want %q", got, "// x := 1")
	}
	// cursor was at col 6 in "x := 1"; after "// " inserted at col 0,
	// cursor shifts to col 9.
	if p.col != 9 {
		t.Errorf("cursor col = %d, want 9", p.col)
	}
	if !p.Dirty() {
		t.Errorf("buffer should be dirty after toggle")
	}
}

func TestToggleCommentSingleLineUncomments(t *testing.T) {
	p := openGoBuf(t, "// x := 1\n")
	p.row, p.col = 0, 9
	p = p.ToggleComment()
	if got := p.Line(0); got != "x := 1" {
		t.Errorf("Line 0 = %q, want %q", got, "x := 1")
	}
	// cursor at col 9 was the end; after "// " removal cursor shifts
	// back to col 6.
	if p.col != 6 {
		t.Errorf("cursor col = %d, want 6", p.col)
	}
}

func TestToggleCommentRoundTripRestoresState(t *testing.T) {
	p := openGoBuf(t, "x := 1\ny := 2\n")
	p.row, p.col = 1, 4
	startRow, startCol := p.row, p.col
	p = p.ToggleComment()
	p = p.ToggleComment()
	if p.Line(0) != "x := 1" {
		t.Errorf("Line 0 = %q after round-trip", p.Line(0))
	}
	if p.Line(1) != "y := 2" {
		t.Errorf("Line 1 = %q after round-trip", p.Line(1))
	}
	if p.row != startRow || p.col != startCol {
		t.Errorf("cursor moved during round-trip: got (%d,%d), want (%d,%d)",
			p.row, p.col, startRow, startCol)
	}
}

func TestToggleCommentSelectionTogglesRange(t *testing.T) {
	p := openGoBuf(t, "x := 1\ny := 2\nz := 3\n")
	// Select rows 0..2 (entire file minus trailing blank).
	p.row, p.col = 0, 0
	p = p.SelectAll()
	// SelectAll puts cursor at the end of the last non-empty row.
	p = p.ToggleComment()
	for i, want := range []string{"// x := 1", "// y := 2", "// z := 3"} {
		if got := p.Line(i); got != want {
			t.Errorf("Line %d = %q, want %q", i, got, want)
		}
	}
	if !p.HasSelection() {
		t.Errorf("selection should be preserved after toggle")
	}
}

func TestToggleCommentNoopForUnknownFiletype(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "page.html")
	if err := os.WriteFile(path, []byte("<p>hi</p>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := NewPane(theme.Default).WithSize(40, 4).Open(path).Focus()
	dirtyBefore := p.Dirty()
	before := p.Line(0)
	p = p.ToggleComment()
	if p.Line(0) != before {
		t.Errorf("Line 0 changed for unknown filetype: got %q, want %q",
			p.Line(0), before)
	}
	if p.Dirty() != dirtyBefore {
		t.Errorf("Dirty flag flipped for no-op toggle")
	}
}

func TestToggleCommentPreservesIndent(t *testing.T) {
	p := openGoBuf(t, "func F() {\n\tx := 1\n\ty := 2\n}\n")
	// Select rows 1..2 (the two indented assignments).
	p.row, p.col = 1, 0
	p.selecting = true
	p.anchorRow, p.anchorCol = 1, 0
	p.row, p.col = 2, len(p.Line(2))
	p = p.ToggleComment()
	if got := p.Line(1); got != "\t// x := 1" {
		t.Errorf("Line 1 = %q, want %q", got, "\t// x := 1")
	}
	if got := p.Line(2); got != "\t// y := 2" {
		t.Errorf("Line 2 = %q, want %q", got, "\t// y := 2")
	}
}

func TestToggleCommentBlankLineInMiddle(t *testing.T) {
	p := openGoBuf(t, "a\n\nb\n")
	p.row, p.col = 0, 0
	p.selecting = true
	p.anchorRow, p.anchorCol = 0, 0
	p.row, p.col = 2, 1
	p = p.ToggleComment()
	if got := p.Line(0); got != "// a" {
		t.Errorf("Line 0 = %q, want %q", got, "// a")
	}
	if got := p.Line(1); got != "" {
		t.Errorf("Line 1 = %q, want empty", got)
	}
	if got := p.Line(2); got != "// b" {
		t.Errorf("Line 2 = %q, want %q", got, "// b")
	}
}

func TestToggleCommentCursorBeforeInsertionDoesNotMove(t *testing.T) {
	p := openGoBuf(t, "    x := 1\n")
	p.row, p.col = 0, 2 // cursor in the indent, before insertion column 4
	p = p.ToggleComment()
	if got := p.Line(0); got != "    // x := 1" {
		t.Errorf("Line 0 = %q, want %q", got, "    // x := 1")
	}
	// Cursor was at col 2 (inside the indent); insertion was at col 4.
	// So the cursor stays at col 2.
	if p.col != 2 {
		t.Errorf("cursor col = %d, want 2 (cursor was before insertion column)", p.col)
	}
}

func TestToggleCommentUncommentClampsCursorInPrefix(t *testing.T) {
	p := openGoBuf(t, "    // x := 1\n")
	// Cursor lands inside the prefix (col 5 == middle of "// ").
	p.row, p.col = 0, 5
	p = p.ToggleComment()
	if got := p.Line(0); got != "    x := 1" {
		t.Errorf("Line 0 = %q, want %q", got, "    x := 1")
	}
	// Cursor was inside the removed prefix [4, 7); should clamp to
	// the deletion column 4.
	if p.col != 4 {
		t.Errorf("cursor col = %d, want 4 (clamp to deletion column)", p.col)
	}
}

func TestToggleCommentSelectionAnchorAdjusted(t *testing.T) {
	p := openGoBuf(t, "x := 1\ny := 2\n")
	p.row, p.col = 0, 6
	p.selecting = true
	p.anchorRow, p.anchorCol = 0, 6
	p.row, p.col = 1, 6
	p = p.ToggleComment()
	// Both anchor (row 0, col 6) and head (row 1, col 6) get bumped
	// by len("// ") = 3 so the selection still spans the same visual
	// characters.
	if p.anchorCol != 9 {
		t.Errorf("anchor col = %d, want 9", p.anchorCol)
	}
	if p.col != 9 {
		t.Errorf("head col = %d, want 9", p.col)
	}
}

func TestToggleCommentMarksBufferVersionDirty(t *testing.T) {
	p := openGoBuf(t, "x\n")
	v0 := p.bufVer
	p = p.ToggleComment()
	if p.bufVer == v0 {
		t.Errorf("bufVer should advance after toggle: %d → %d", v0, p.bufVer)
	}
}
