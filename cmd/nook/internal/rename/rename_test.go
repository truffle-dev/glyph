package rename

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestNewIsClosed(t *testing.T) {
	t.Parallel()
	p := New()
	if p.Open() {
		t.Error("New() should be closed")
	}
	if got := p.View(theme.Light, 60); got != "" {
		t.Errorf("closed view should be empty, got %q", got)
	}
}

func TestWithCurrentOpensWithCursorAtEnd(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("Alpha", "main.go")
	if !p.Open() {
		t.Error("WithCurrent should open the prompt")
	}
	if p.Value() != "Alpha" {
		t.Errorf("Value() = %q, want Alpha", p.Value())
	}
	if p.cursor != 5 {
		t.Errorf("cursor = %d, want 5", p.cursor)
	}
}

func TestCloseEmptiesValue(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("Alpha", "main.go").Close()
	if p.Open() {
		t.Error("Close() should close the prompt")
	}
	if p.Value() != "" {
		t.Errorf("after Close, Value() = %q", p.Value())
	}
}

func TestBackspaceDeletesAndTypeInserts(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("Alpha", "main.go")
	for i := 0; i < 5; i++ {
		p = p.Backspace()
	}
	p = p.Type('B').Type('e').Type('t').Type('a')
	if p.Value() != "Beta" {
		t.Errorf("Value() = %q, want Beta", p.Value())
	}
}

func TestTypeRejectsLeadingDigit(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("", "main.go")
	p = p.Type('1')
	if p.Value() != "" {
		t.Errorf("leading digit should be rejected; got %q", p.Value())
	}
	p = p.Type('a').Type('1')
	if p.Value() != "a1" {
		t.Errorf("digit after letter should be accepted; got %q", p.Value())
	}
}

func TestTypeRejectsNonIdentifierChars(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("", "main.go")
	for _, r := range " -+!@#$%^&*()[]{}<>/\\,;:\"'`~?\t\n" {
		p = p.Type(r)
	}
	if p.Value() != "" {
		t.Errorf("non-identifier chars should be rejected; got %q", p.Value())
	}
}

func TestTypeAcceptsUnicodeLetters(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("", "main.go")
	p = p.Type('A').Type('é').Type('日').Type('_').Type('0')
	if p.Value() != "Aé日_0" {
		t.Errorf("unicode identifier rejected; got %q", p.Value())
	}
}

func TestCursorMoves(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("hello", "main.go")
	if p.cursor != 5 {
		t.Fatalf("initial cursor = %d, want 5", p.cursor)
	}
	p = p.MoveHome()
	if p.cursor != 0 {
		t.Errorf("MoveHome cursor = %d, want 0", p.cursor)
	}
	p = p.MoveRight().MoveRight()
	if p.cursor != 2 {
		t.Errorf("after 2 MoveRight cursor = %d, want 2", p.cursor)
	}
	p = p.MoveEnd()
	if p.cursor != 5 {
		t.Errorf("MoveEnd cursor = %d, want 5", p.cursor)
	}
	p = p.MoveLeft()
	if p.cursor != 4 {
		t.Errorf("MoveLeft cursor = %d, want 4", p.cursor)
	}
}

func TestInsertInMiddle(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("Alha", "main.go").MoveHome()
	p = p.MoveRight().MoveRight() // cursor between Al|ha
	p = p.Type('p')
	if p.Value() != "Alpha" {
		t.Errorf("Value() = %q, want Alpha", p.Value())
	}
}

func TestErrorSurface(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("Foo", "main.go").WithError("name already taken")
	out := p.View(theme.Light, 80)
	if !strings.Contains(out, "name already taken") {
		t.Errorf("error message not in view:\n%s", out)
	}
}

func TestErrorClearsOnNextEdit(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("Foo", "main.go").WithError("oops")
	p = p.Type('x')
	if p.err != "" {
		t.Errorf("typing should clear error; err = %q", p.err)
	}
}

func TestViewIncludesCurrentAndArrow(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("Alpha", "main.go")
	out := p.View(theme.Light, 80)
	if !strings.Contains(out, "Alpha") {
		t.Errorf("view missing current name:\n%s", out)
	}
	if !strings.Contains(out, "→") {
		t.Errorf("view missing arrow:\n%s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("view missing path hint:\n%s", out)
	}
}

func TestValueTrimsWhitespace(t *testing.T) {
	t.Parallel()
	p := New().WithCurrent("  hello  ", "x")
	if p.Value() != "hello" {
		t.Errorf("Value() = %q, want hello", p.Value())
	}
}
