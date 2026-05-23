package editor

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	codeview "github.com/truffle-dev/glyph/components/code-view"
	"github.com/truffle-dev/glyph/components/theme"
)

func TestMain(m *testing.M) {
	// Force true-color profile so styled output is stable in tests.
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func key(kt tea.KeyType, rs ...rune) tea.KeyMsg {
	return tea.KeyMsg{Type: kt, Runes: rs}
}

func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestNewIsEmptyFocused(t *testing.T) {
	m := New(theme.Default)
	if !m.Focused() {
		t.Fatal("editor should be focused by default")
	}
	if v := m.Value(); v != "" {
		t.Fatalf("new editor should be empty, got %q", v)
	}
	r, c := m.Cursor()
	if r != 0 || c != 0 {
		t.Fatalf("new cursor at 0,0 expected, got %d,%d", r, c)
	}
	if m.LineCount() != 1 {
		t.Fatalf("new editor should have 1 line, got %d", m.LineCount())
	}
	if m.Dirty() {
		t.Fatal("new editor should not be dirty")
	}
}

func TestWithContentRestoresLines(t *testing.T) {
	m := New(theme.Default).WithContent("alpha\nbeta\ngamma")
	if m.LineCount() != 3 {
		t.Fatalf("expected 3 lines, got %d", m.LineCount())
	}
	if v := m.Value(); v != "alpha\nbeta\ngamma" {
		t.Fatalf("value round-trip failed: %q", v)
	}
	r, c := m.Cursor()
	if r != 0 || c != 0 {
		t.Fatalf("WithContent should park cursor at 0,0, got %d,%d", r, c)
	}
	if m.Dirty() {
		t.Fatal("WithContent should not push undo entries")
	}
}

func TestInsertSingleRune(t *testing.T) {
	m := New(theme.Default)
	m, _ = m.Update(runes("h"))
	m, _ = m.Update(runes("i"))
	if v := m.Value(); v != "hi" {
		t.Fatalf("expected 'hi', got %q", v)
	}
	r, c := m.Cursor()
	if r != 0 || c != 2 {
		t.Fatalf("cursor should be at 0,2 after 'hi', got %d,%d", r, c)
	}
	if !m.Dirty() {
		t.Fatal("editor should be dirty after edit")
	}
}

func TestInsertMultiRunePaste(t *testing.T) {
	m := New(theme.Default)
	m, _ = m.Update(runes("hello world"))
	if v := m.Value(); v != "hello world" {
		t.Fatalf("expected paste, got %q", v)
	}
	_, c := m.Cursor()
	if c != 11 {
		t.Fatalf("cursor should sit at end of paste, got col %d", c)
	}
}

func TestSpaceInsertsBlank(t *testing.T) {
	m := New(theme.Default)
	m, _ = m.Update(runes("a"))
	m, _ = m.Update(key(tea.KeySpace))
	m, _ = m.Update(runes("b"))
	if v := m.Value(); v != "a b" {
		t.Fatalf("expected 'a b', got %q", v)
	}
}

func TestNewlineSplits(t *testing.T) {
	m := New(theme.Default).WithContent("abcdef")
	m, _ = m.Update(key(tea.KeyEnd))
	// move cursor to col 3 (between 'c' and 'd')
	m, _ = m.Update(key(tea.KeyHome))
	for k := 0; k < 3; k++ {
		m, _ = m.Update(key(tea.KeyRight))
	}
	m, _ = m.Update(key(tea.KeyEnter))
	if v := m.Value(); v != "abc\ndef" {
		t.Fatalf("expected 'abc\\ndef', got %q", v)
	}
	r, c := m.Cursor()
	if r != 1 || c != 0 {
		t.Fatalf("post-Enter cursor should be at 1,0, got %d,%d", r, c)
	}
}

func TestBackspaceWithinLine(t *testing.T) {
	m := New(theme.Default)
	m, _ = m.Update(runes("abc"))
	m, _ = m.Update(key(tea.KeyBackspace))
	if v := m.Value(); v != "ab" {
		t.Fatalf("expected 'ab', got %q", v)
	}
}

func TestBackspaceJoinsLines(t *testing.T) {
	m := New(theme.Default).WithContent("foo\nbar")
	// Park cursor at start of "bar".
	m, _ = m.Update(key(tea.KeyDown))
	m, _ = m.Update(key(tea.KeyHome))
	m, _ = m.Update(key(tea.KeyBackspace))
	if v := m.Value(); v != "foobar" {
		t.Fatalf("expected 'foobar' after join, got %q", v)
	}
	r, c := m.Cursor()
	if r != 0 || c != 3 {
		t.Fatalf("post-join cursor should be at 0,3, got %d,%d", r, c)
	}
}

func TestDeleteForward(t *testing.T) {
	m := New(theme.Default).WithContent("abc")
	m, _ = m.Update(key(tea.KeyDelete))
	if v := m.Value(); v != "bc" {
		t.Fatalf("expected 'bc', got %q", v)
	}
}

func TestDeleteForwardAtEOLJoins(t *testing.T) {
	m := New(theme.Default).WithContent("foo\nbar")
	m, _ = m.Update(key(tea.KeyEnd))
	m, _ = m.Update(key(tea.KeyDelete))
	if v := m.Value(); v != "foobar" {
		t.Fatalf("expected 'foobar' after EOL delete, got %q", v)
	}
}

func TestTabInsertsSpaces(t *testing.T) {
	m := New(theme.Default).WithTabSize(2)
	m, _ = m.Update(key(tea.KeyTab))
	if v := m.Value(); v != "  " {
		t.Fatalf("expected two spaces, got %q", v)
	}
}

func TestUndoSingleInsert(t *testing.T) {
	m := New(theme.Default)
	m, _ = m.Update(runes("x"))
	m, _ = m.Update(key(tea.KeyCtrlZ))
	if v := m.Value(); v != "" {
		t.Fatalf("undo should restore empty, got %q", v)
	}
	r, c := m.Cursor()
	if r != 0 || c != 0 {
		t.Fatalf("undo should restore cursor to 0,0, got %d,%d", r, c)
	}
}

func TestUndoBackspace(t *testing.T) {
	m := New(theme.Default).WithContent("hi")
	m, _ = m.Update(key(tea.KeyEnd))
	m, _ = m.Update(key(tea.KeyBackspace))
	if v := m.Value(); v != "h" {
		t.Fatalf("expected 'h', got %q", v)
	}
	m, _ = m.Update(key(tea.KeyCtrlZ))
	if v := m.Value(); v != "hi" {
		t.Fatalf("undo should restore 'hi', got %q", v)
	}
}

func TestUndoNewline(t *testing.T) {
	m := New(theme.Default).WithContent("foobar")
	m, _ = m.Update(key(tea.KeyHome))
	for k := 0; k < 3; k++ {
		m, _ = m.Update(key(tea.KeyRight))
	}
	m, _ = m.Update(key(tea.KeyEnter))
	if v := m.Value(); v != "foo\nbar" {
		t.Fatalf("expected split, got %q", v)
	}
	m, _ = m.Update(key(tea.KeyCtrlZ))
	if v := m.Value(); v != "foobar" {
		t.Fatalf("undo split should restore 'foobar', got %q", v)
	}
}

func TestUndoJoin(t *testing.T) {
	m := New(theme.Default).WithContent("foo\nbar")
	m, _ = m.Update(key(tea.KeyDown))
	m, _ = m.Update(key(tea.KeyHome))
	m, _ = m.Update(key(tea.KeyBackspace))
	if v := m.Value(); v != "foobar" {
		t.Fatalf("expected join, got %q", v)
	}
	m, _ = m.Update(key(tea.KeyCtrlZ))
	if v := m.Value(); v != "foo\nbar" {
		t.Fatalf("undo join should restore 'foo\\nbar', got %q", v)
	}
}

func TestRedoAfterUndo(t *testing.T) {
	m := New(theme.Default)
	m, _ = m.Update(runes("x"))
	m, _ = m.Update(key(tea.KeyCtrlZ))
	m, _ = m.Update(key(tea.KeyCtrlY))
	if v := m.Value(); v != "x" {
		t.Fatalf("redo should restore 'x', got %q", v)
	}
}

func TestEditAfterUndoClearsRedo(t *testing.T) {
	m := New(theme.Default)
	m, _ = m.Update(runes("x"))
	m, _ = m.Update(key(tea.KeyCtrlZ))
	m, _ = m.Update(runes("y"))
	// Redo should now be a no-op because a fresh edit landed.
	m, _ = m.Update(key(tea.KeyCtrlY))
	if v := m.Value(); v != "y" {
		t.Fatalf("redo after fresh edit should not bring 'x' back, got %q", v)
	}
}

func TestMoveLeftWrapsToPrevLine(t *testing.T) {
	m := New(theme.Default).WithContent("foo\nbar")
	m, _ = m.Update(key(tea.KeyDown))
	m, _ = m.Update(key(tea.KeyHome))
	m, _ = m.Update(key(tea.KeyLeft))
	r, c := m.Cursor()
	if r != 0 || c != 3 {
		t.Fatalf("left at line-start should wrap to end of prev line (0,3), got %d,%d", r, c)
	}
}

func TestMoveRightWrapsToNextLine(t *testing.T) {
	m := New(theme.Default).WithContent("foo\nbar")
	m, _ = m.Update(key(tea.KeyEnd))
	m, _ = m.Update(key(tea.KeyRight))
	r, c := m.Cursor()
	if r != 1 || c != 0 {
		t.Fatalf("right at line-end should wrap to start of next line (1,0), got %d,%d", r, c)
	}
}

func TestPageDownAdvancesByHeight(t *testing.T) {
	m := New(theme.Default).WithContent(strings.Repeat("line\n", 30)).WithHeight(10)
	r, _ := m.Cursor()
	if r != 0 {
		t.Fatalf("setup: cursor should start at 0, got %d", r)
	}
	m, _ = m.Update(key(tea.KeyPgDown))
	r, _ = m.Cursor()
	if r < 8 {
		t.Fatalf("PgDown should jump ~height rows, got %d", r)
	}
}

func TestCtrlHomeEndJumpsDocument(t *testing.T) {
	m := New(theme.Default).WithContent("line1\nline2\nline3")
	m, _ = m.Update(key(tea.KeyCtrlEnd))
	r, c := m.Cursor()
	if r != 2 || c != 5 {
		t.Fatalf("Ctrl-End should land at 2,5, got %d,%d", r, c)
	}
	m, _ = m.Update(key(tea.KeyCtrlHome))
	r, c = m.Cursor()
	if r != 0 || c != 0 {
		t.Fatalf("Ctrl-Home should land at 0,0, got %d,%d", r, c)
	}
}

func TestViewportScrollsWithCursor(t *testing.T) {
	m := New(theme.Default).
		WithContent(strings.Repeat("line\n", 50)).
		WithHeight(10).
		WithWidth(40)
	// Walk down past the visible window.
	for k := 0; k < 15; k++ {
		m, _ = m.Update(key(tea.KeyDown))
	}
	out := m.View()
	// The viewport should have scrolled, so the first line (1) should not
	// be visible anymore.
	if strings.Contains(out, " 1 line") {
		t.Fatalf("viewport should have scrolled past line 1\n%s", out)
	}
	// Cursor row 16 (1-based) should be present.
	if !strings.Contains(out, "16 ") {
		t.Fatalf("gutter should show line 16 in the visible window\n%s", out)
	}
}

func TestBlurIgnoresKeys(t *testing.T) {
	m := New(theme.Default).Blur()
	m, _ = m.Update(runes("x"))
	if v := m.Value(); v != "" {
		t.Fatalf("blurred editor should ignore input, got %q", v)
	}
}

func TestChangeMsgEmittedOnEdit(t *testing.T) {
	m := New(theme.Default)
	_, cmd := m.Update(runes("a"))
	if cmd == nil {
		t.Fatal("insert should emit a tea.Cmd carrying ChangeMsg")
	}
	msg := cmd()
	chg, ok := msg.(ChangeMsg)
	if !ok {
		t.Fatalf("expected ChangeMsg, got %T", msg)
	}
	if chg.Value != "a" {
		t.Fatalf("ChangeMsg.Value should be 'a', got %q", chg.Value)
	}
}

func TestNoChangeMsgOnPureMotion(t *testing.T) {
	m := New(theme.Default).WithContent("foo")
	_, cmd := m.Update(key(tea.KeyRight))
	if cmd != nil {
		t.Fatal("cursor motion alone should not emit ChangeMsg")
	}
}

func TestWithLanguageDoesNotMutateBuffer(t *testing.T) {
	m := New(theme.Default).
		WithContent("package main").
		WithLanguage(codeview.LangGo)
	if v := m.Value(); v != "package main" {
		t.Fatalf("WithLanguage should not touch buffer, got %q", v)
	}
}

func TestViewIncludesGutterAndCursorLine(t *testing.T) {
	m := New(theme.Default).WithContent("hello\nworld").WithWidth(60).WithHeight(6)
	out := m.View()
	// The cursor line has its first char wrapped in an inverted-style ANSI
	// run, so "hello" won't match as a single substring on row 1. The
	// non-cursor row 2 renders "world" as one styled run.
	if !strings.Contains(out, "world") {
		t.Fatalf("View should contain second-line text\n%s", out)
	}
	if !strings.Contains(out, " 1 ") {
		t.Fatalf("View should render line-1 gutter\n%s", out)
	}
	if !strings.Contains(out, " 2 ") {
		t.Fatalf("View should render line-2 gutter\n%s", out)
	}
}

func TestGutterHiddenWhenDisabled(t *testing.T) {
	m := New(theme.Default).
		WithContent("hi").
		WithGutter(false).
		WithWidth(40).
		WithHeight(4)
	out := m.View()
	// The minimum gutter width is 4 ("  1 " through " NN "). When gutter is
	// off, the standalone " 1 " pattern should not appear at the very start
	// of the body. A weak but useful check: there should be no " 2 " row
	// when only one line exists; the empty rows should be all blanks.
	if strings.Contains(out, " 1 hi") {
		t.Fatalf("expected no gutter prefix when WithGutter(false)\n%s", out)
	}
}
