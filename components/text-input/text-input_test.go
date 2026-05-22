package textinput

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestPlaceholderShownWhenEmpty(t *testing.T) {
	i := New(theme.Default).WithPlaceholder("commit message…").WithWidth(40)
	if !strings.Contains(i.View(), "commit message") {
		t.Errorf("expected placeholder in view, got %q", i.View())
	}
}

func TestPlaceholderHiddenAfterTyping(t *testing.T) {
	i := New(theme.Default).WithPlaceholder("commit message…").WithWidth(40).WithValue("hello")
	out := i.View()
	if strings.Contains(out, "commit message") {
		t.Errorf("expected placeholder hidden once value present, got %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected value in view, got %q", out)
	}
}

func TestWithValueSplitsLines(t *testing.T) {
	i := New(theme.Default).WithValue("first\nsecond\nthird")
	if got := i.Value(); got != "first\nsecond\nthird" {
		t.Errorf("Value round-trip failed: %q", got)
	}
	row, col := i.Cursor()
	if row != 2 || col != 5 {
		t.Errorf("expected cursor at end of last line (2,5), got (%d,%d)", row, col)
	}
}

func TestEnterInsertsNewline(t *testing.T) {
	i := New(theme.Default).WithValue("abcdef")
	// Move cursor to position 3 (after "abc")
	for k := 0; k < 3; k++ {
		i, _ = i.Update(tea.KeyMsg{Type: tea.KeyLeft})
	}
	next, _ := i.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := next.Value(); got != "abc\ndef" {
		t.Errorf("expected 'abc\\ndef', got %q", got)
	}
	row, col := next.Cursor()
	if row != 1 || col != 0 {
		t.Errorf("expected cursor at (1,0) after newline, got (%d,%d)", row, col)
	}
}

func TestCtrlDEmitsSubmitMsg(t *testing.T) {
	i := New(theme.Default).WithValue("multi\nline")
	_, cmd := i.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if cmd == nil {
		t.Fatal("expected a tea.Cmd on ctrl-d")
	}
	sub, ok := cmd().(SubmitMsg)
	if !ok {
		t.Fatalf("expected SubmitMsg, got %T", cmd())
	}
	if sub.Value != "multi\nline" {
		t.Errorf("expected submit value 'multi\\nline', got %q", sub.Value)
	}
}

func TestCtrlDIgnoredWhenEmpty(t *testing.T) {
	i := New(theme.Default)
	_, cmd := i.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if cmd != nil {
		t.Error("expected no command on ctrl-d with empty value")
	}
}

func TestEscEmitsCancelMsg(t *testing.T) {
	i := New(theme.Default).WithValue("draft")
	_, cmd := i.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a tea.Cmd on esc")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Errorf("expected CancelMsg, got %T", cmd())
	}
}

func TestBackspaceDeletesRune(t *testing.T) {
	i := New(theme.Default).WithValue("abc")
	next, _ := i.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if next.Value() != "ab" {
		t.Errorf("expected 'ab', got %q", next.Value())
	}
}

func TestBackspaceJoinsLines(t *testing.T) {
	i := New(theme.Default).WithValue("first\nsecond")
	// Cursor is at end of "second". Move to start of "second".
	for k := 0; k < 6; k++ {
		i, _ = i.Update(tea.KeyMsg{Type: tea.KeyLeft})
	}
	row, col := i.Cursor()
	if row != 1 || col != 0 {
		t.Fatalf("setup: expected cursor at (1,0), got (%d,%d)", row, col)
	}
	next, _ := i.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if got := next.Value(); got != "firstsecond" {
		t.Errorf("expected join 'firstsecond', got %q", got)
	}
	row, col = next.Cursor()
	if row != 0 || col != 5 {
		t.Errorf("expected cursor at (0,5) after join, got (%d,%d)", row, col)
	}
}

func TestCtrlUKillsLineUpToCursor(t *testing.T) {
	i := New(theme.Default).WithValue("hello world")
	// Cursor at end. Move left 5 to land between "hello " and "world".
	for k := 0; k < 5; k++ {
		i, _ = i.Update(tea.KeyMsg{Type: tea.KeyLeft})
	}
	next, _ := i.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if got := next.Value(); got != "world" {
		t.Errorf("expected 'world' after ctrl-u, got %q", got)
	}
	row, col := next.Cursor()
	if row != 0 || col != 0 {
		t.Errorf("expected cursor at (0,0) after ctrl-u, got (%d,%d)", row, col)
	}
}

func TestAltLeftJumpsWords(t *testing.T) {
	i := New(theme.Default).WithValue("hello brave new world")
	// Cursor at col 21 (end). Alt+Left should land at col 16 (start of "world").
	next, _ := i.Update(tea.KeyMsg{Type: tea.KeyLeft, Alt: true})
	_, col := next.Cursor()
	if col != 16 {
		t.Errorf("expected col 16 (start of 'world'), got %d", col)
	}
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyLeft, Alt: true})
	_, col = next.Cursor()
	if col != 12 {
		t.Errorf("expected col 12 (start of 'new'), got %d", col)
	}
}

func TestAltRightJumpsWords(t *testing.T) {
	i := New(theme.Default).WithValue("hello brave new world")
	// Move cursor to col 0.
	i, _ = i.Update(tea.KeyMsg{Type: tea.KeyHome})
	next, _ := i.Update(tea.KeyMsg{Type: tea.KeyRight, Alt: true})
	_, col := next.Cursor()
	if col != 6 {
		t.Errorf("expected col 6 (start of 'brave'), got %d", col)
	}
}

func TestHomeEnd(t *testing.T) {
	in := New(theme.Default).WithValue("first\nsecond line")
	in, _ = in.Update(tea.KeyMsg{Type: tea.KeyHome})
	row, col := in.Cursor()
	if row != 1 || col != 0 {
		t.Errorf("expected (1,0) after Home on last line, got (%d,%d)", row, col)
	}
	in, _ = in.Update(tea.KeyMsg{Type: tea.KeyEnd})
	row, col = in.Cursor()
	if row != 1 || col != 11 {
		t.Errorf("expected (1,11) after End, got (%d,%d)", row, col)
	}
}

func TestUpDownClampsColumn(t *testing.T) {
	i := New(theme.Default).WithValue("ab\nlonger second")
	// Cursor lands at end of "longer second" (row 1, col 13). Up should clamp to col 2.
	next, _ := i.Update(tea.KeyMsg{Type: tea.KeyUp})
	row, col := next.Cursor()
	if row != 0 || col != 2 {
		t.Errorf("expected (0,2) after Up clamp, got (%d,%d)", row, col)
	}
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	row, col = next.Cursor()
	if row != 1 || col != 2 {
		t.Errorf("expected (1,2) after Down, got (%d,%d)", row, col)
	}
}

func TestRuneAppendsAtCursor(t *testing.T) {
	i := New(theme.Default).WithValue("he")
	next, _ := i.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("llo")})
	if next.Value() != "hello" {
		t.Errorf("expected 'hello', got %q", next.Value())
	}
}

func TestSpaceInserts(t *testing.T) {
	i := New(theme.Default).WithValue("hi")
	next, _ := i.Update(tea.KeyMsg{Type: tea.KeySpace})
	if next.Value() != "hi " {
		t.Errorf("expected 'hi ', got %q", next.Value())
	}
}

func TestBlurredInputIgnoresKeys(t *testing.T) {
	i := New(theme.Default).Blur()
	next, cmd := i.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
	if cmd != nil || next.Value() != "" {
		t.Errorf("expected blurred input to ignore keys, got value=%q cmd=%v", next.Value(), cmd)
	}
}

func TestReset(t *testing.T) {
	i := New(theme.Default).WithValue("line1\nline2")
	r := i.Reset()
	if r.Value() != "" {
		t.Errorf("expected empty after Reset, got %q", r.Value())
	}
	row, col := r.Cursor()
	if row != 0 || col != 0 {
		t.Errorf("expected cursor (0,0) after Reset, got (%d,%d)", row, col)
	}
}

func TestMinWidthClamped(t *testing.T) {
	i := New(theme.Default).WithWidth(1)
	if i.View() == "" {
		t.Error("expected non-empty render even with sub-minimum width")
	}
}

func TestMinHeightClamped(t *testing.T) {
	i := New(theme.Default).WithHeight(0)
	if i.View() == "" {
		t.Error("expected non-empty render even with sub-minimum height")
	}
}
