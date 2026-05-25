package editor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/cmd/nook/internal/clip"
	"github.com/truffle-dev/glyph/components/theme"
)

// shiftRight extends the selection one character to the right. Used as a
// concise way to build a single-row selection in tests.
func shiftRight(p Pane, n int) Pane {
	for i := 0; i < n; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	}
	return p
}

// shiftDown extends the selection one row down.
func shiftDown(p Pane, n int) Pane {
	for i := 0; i < n; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	}
	return p
}

// seedPane returns a fresh focused pane containing the supplied lines, joined
// by literal newlines via Enter so the buffer's row/col bookkeeping matches a
// real edit session. Cursor lands at row 0, col 0 after a JumpTo(0, 0).
func seedPane(lines ...string) Pane {
	p := NewPane(theme.Default).WithSize(60, 20).Focus()
	for i, l := range lines {
		p, _ = p.Update(runeMsg(l))
		if i < len(lines)-1 {
			p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
		}
	}
	p = p.JumpTo(0, 0)
	return p
}

func TestShiftRight_StartsAndExtendsSelection(t *testing.T) {
	p := seedPane("abcdef")
	p = shiftRight(p, 3)
	if !p.HasSelection() {
		t.Fatal("expected selection after shift+right")
	}
	if got := p.SelectionText(); got != "abc" {
		t.Fatalf("SelectionText = %q, want %q", got, "abc")
	}
	sr, sc, er, ec := p.SelectionRange()
	if sr != 0 || sc != 0 || er != 0 || ec != 3 {
		t.Fatalf("SelectionRange = (%d,%d)-(%d,%d), want (0,0)-(0,3)", sr, sc, er, ec)
	}
}

func TestShiftDown_MultiRowSelection(t *testing.T) {
	p := seedPane("hello", "world", "again")
	p = shiftDown(p, 2)
	if !p.HasSelection() {
		t.Fatal("expected selection after shift+down")
	}
	// Anchor at (0,0), head at (2,0). The selection covers "hello\nworld\n".
	if got := p.SelectionText(); got != "hello\nworld\n" {
		t.Fatalf("SelectionText = %q, want %q", got, "hello\nworld\n")
	}
}

func TestSelectAll_CoversBuffer(t *testing.T) {
	p := seedPane("aa", "bb", "cc")
	p = p.SelectAll()
	if !p.HasSelection() {
		t.Fatal("SelectAll should produce a selection")
	}
	if got := p.SelectionText(); got != "aa\nbb\ncc" {
		t.Fatalf("SelectionText = %q, want %q", got, "aa\nbb\ncc")
	}
}

func TestMovement_CollapsesSelection(t *testing.T) {
	p := seedPane("abcdef")
	p = shiftRight(p, 3)
	if !p.HasSelection() {
		t.Fatal("precondition: should have selection")
	}
	// Left collapses to start.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if p.HasSelection() {
		t.Fatal("Left should collapse selection")
	}
	if p.CursorRow() != 0 || p.CursorCol() != 0 {
		t.Fatalf("cursor after Left collapse = (%d,%d), want (0,0)", p.CursorRow(), p.CursorCol())
	}

	// Re-select, then Right collapses to end.
	p = shiftRight(p, 3)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRight})
	if p.HasSelection() {
		t.Fatal("Right should collapse selection")
	}
	if p.CursorCol() != 3 {
		t.Fatalf("cursor col after Right collapse = %d, want 3", p.CursorCol())
	}
}

func TestRune_ReplacesSelection(t *testing.T) {
	p := seedPane("abcdef")
	p = shiftRight(p, 3)
	p, _ = p.Update(runeMsg("X"))
	if p.Line(0) != "Xdef" {
		t.Fatalf("after typing X: line = %q, want %q", p.Line(0), "Xdef")
	}
	if p.HasSelection() {
		t.Fatal("typing should clear selection")
	}
}

func TestBackspace_DeletesSelection(t *testing.T) {
	p := seedPane("abcdef")
	p = shiftRight(p, 3)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.Line(0) != "def" {
		t.Fatalf("after backspace on selection: line = %q, want %q", p.Line(0), "def")
	}
	if p.HasSelection() {
		t.Fatal("backspace should clear selection")
	}
}

func TestCtrlA_SelectsAll(t *testing.T) {
	p := seedPane("a", "b")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if !p.HasSelection() {
		t.Fatal("Ctrl+A should select")
	}
	if got := p.SelectionText(); got != "a\nb" {
		t.Fatalf("SelectionText after Ctrl+A = %q, want %q", got, "a\nb")
	}
}

func TestCtrlC_CopiesSelection(t *testing.T) {
	clip.Set("")
	p := seedPane("hello world")
	p = shiftRight(p, 5)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if got := clip.Register(); got != "hello" {
		t.Fatalf("clipboard after Ctrl+C = %q, want %q", got, "hello")
	}
	// Copy must not mutate the buffer or clear the selection.
	if p.Line(0) != "hello world" {
		t.Fatalf("Ctrl+C mutated line: %q", p.Line(0))
	}
	if !p.HasSelection() {
		t.Fatal("Ctrl+C should leave the selection intact")
	}
}

func TestCtrlC_NoSelection_CopiesCurrentLineWithNewline(t *testing.T) {
	clip.Set("")
	p := seedPane("first", "second")
	// Move to row 1.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if got := clip.Register(); got != "second\n" {
		t.Fatalf("Ctrl+C on empty selection = %q, want %q", got, "second\n")
	}
}

func TestCtrlX_CutsSelection(t *testing.T) {
	clip.Set("")
	p := seedPane("hello world")
	p = shiftRight(p, 5)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if got := clip.Register(); got != "hello" {
		t.Fatalf("clipboard after Ctrl+X = %q, want %q", got, "hello")
	}
	if p.Line(0) != " world" {
		t.Fatalf("after Ctrl+X: line = %q, want %q", p.Line(0), " world")
	}
	if p.HasSelection() {
		t.Fatal("Ctrl+X should clear the selection")
	}
}

func TestCtrlX_NoSelection_CutsCurrentLine(t *testing.T) {
	clip.Set("")
	p := seedPane("a", "b", "c")
	// Move to row 1.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if got := clip.Register(); got != "b\n" {
		t.Fatalf("Ctrl+X clipboard = %q, want %q", got, "b\n")
	}
	if p.LineCount() != 2 {
		t.Fatalf("after Ctrl+X cut-line: LineCount = %d, want 2", p.LineCount())
	}
	if p.Line(0) != "a" || p.Line(1) != "c" {
		t.Fatalf("after Ctrl+X: lines = %q | %q, want %q | %q", p.Line(0), p.Line(1), "a", "c")
	}
}

func TestCtrlX_OnlyLine_LeavesEmptyBuffer(t *testing.T) {
	clip.Set("")
	p := seedPane("only line")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if got := clip.Register(); got != "only line\n" {
		t.Fatalf("clipboard = %q, want %q", got, "only line\n")
	}
	if p.LineCount() != 1 || p.Line(0) != "" {
		t.Fatalf("after cut-line on only line: lines = %d, content = %q", p.LineCount(), p.Line(0))
	}
}

func TestCtrlV_PastesAtCursor(t *testing.T) {
	clip.Set("INS")
	p := seedPane("abc")
	// Move to col 1.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRight})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	if p.Line(0) != "aINSbc" {
		t.Fatalf("after Ctrl+V: line = %q, want %q", p.Line(0), "aINSbc")
	}
}

func TestCtrlV_ReplacesSelection(t *testing.T) {
	clip.Set("REPLACED")
	p := seedPane("hello world")
	p = shiftRight(p, 5)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	if p.Line(0) != "REPLACED world" {
		t.Fatalf("after Ctrl+V over selection: line = %q, want %q", p.Line(0), "REPLACED world")
	}
	if p.HasSelection() {
		t.Fatal("Ctrl+V should clear the selection")
	}
}

func TestCtrlV_MultilinePaste(t *testing.T) {
	clip.Set("one\ntwo\nthree")
	p := seedPane("XY")
	// Move to col 1 so we splice between X and Y.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRight})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	joined := strings.Join(p.Lines(), "\n")
	want := "Xone\ntwo\nthreeY"
	if joined != want {
		t.Fatalf("after multi-line paste: %q, want %q", joined, want)
	}
}

func TestEsc_ClearsSelection(t *testing.T) {
	p := seedPane("abcdef")
	p = shiftRight(p, 3)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.HasSelection() {
		t.Fatal("Esc should clear selection")
	}
}
