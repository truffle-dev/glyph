package symbolsearch

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

func defaultTheme() theme.Theme { return theme.Theme{} }

func TestPromptOpenAndClose(t *testing.T) {
	p := New()
	if p.IsOpen() {
		t.Fatal("new prompt should be closed")
	}
	p = p.Open("nook")
	if !p.IsOpen() {
		t.Fatal("opened prompt should report open")
	}
	if p.Value() != "" {
		t.Fatalf("opened prompt value = %q want empty", p.Value())
	}
	p = p.Close()
	if p.IsOpen() {
		t.Fatal("closed prompt should report closed")
	}
}

func TestPromptOpenWithSeed(t *testing.T) {
	p := New().OpenWith("nook", "Handle")
	if p.Value() != "Handle" {
		t.Fatalf("seeded value = %q want Handle", p.Value())
	}
	if p.Cursor() != 6 {
		t.Fatalf("seeded cursor = %d want 6", p.Cursor())
	}
}

func TestPromptTypeAndBackspace(t *testing.T) {
	p := New().Open("ws")
	p = p.Type('A').Type('B').Type('C')
	if p.Value() != "ABC" {
		t.Fatalf("value %q want ABC", p.Value())
	}
	p = p.Backspace()
	if p.Value() != "AB" {
		t.Fatalf("after backspace %q want AB", p.Value())
	}
}

func TestPromptAcceptsPunctuation(t *testing.T) {
	// Workspace queries often include "." (qualified) and "/" (path).
	// Symbol queries can also have "*" for wildcards in some servers.
	p := New().Open("ws")
	for _, r := range "User.Hello/*" {
		p = p.Type(r)
	}
	if p.Value() != "User.Hello/*" {
		t.Fatalf("value %q want %q", p.Value(), "User.Hello/*")
	}
}

func TestPromptRejectsControlRunes(t *testing.T) {
	p := New().Open("ws")
	p = p.Type('a').Type('\n').Type('b').Type(0x1b).Type('c')
	if p.Value() != "abc" {
		t.Fatalf("value %q want abc (controls filtered)", p.Value())
	}
}

func TestPromptMovementKeys(t *testing.T) {
	p := New().Open("ws")
	for _, r := range "abcde" {
		p = p.Type(r)
	}
	if p.Cursor() != 5 {
		t.Fatalf("cursor %d want 5", p.Cursor())
	}
	p = p.MoveHome()
	if p.Cursor() != 0 {
		t.Fatalf("home cursor %d want 0", p.Cursor())
	}
	p = p.MoveRight().MoveRight()
	if p.Cursor() != 2 {
		t.Fatalf("right×2 cursor %d want 2", p.Cursor())
	}
	p = p.MoveLeft()
	if p.Cursor() != 1 {
		t.Fatalf("left cursor %d want 1", p.Cursor())
	}
	p = p.MoveEnd()
	if p.Cursor() != 5 {
		t.Fatalf("end cursor %d want 5", p.Cursor())
	}
}

func TestPromptInsertMidline(t *testing.T) {
	p := New().Open("ws")
	for _, r := range "AC" {
		p = p.Type(r)
	}
	// Move between A and C, insert B.
	p = p.MoveLeft().Type('B')
	if p.Value() != "ABC" {
		t.Fatalf("value %q want ABC", p.Value())
	}
}

func TestPromptDeleteAndClear(t *testing.T) {
	p := New().Open("ws")
	for _, r := range "Hello" {
		p = p.Type(r)
	}
	p = p.MoveHome().Delete()
	if p.Value() != "ello" {
		t.Fatalf("delete head value %q want ello", p.Value())
	}
	p = p.Clear()
	if p.Value() != "" || p.Cursor() != 0 {
		t.Fatalf("clear: value=%q cursor=%d want empty,0", p.Value(), p.Cursor())
	}
}

func TestPromptWithError(t *testing.T) {
	p := New().Open("ws").WithError("no matches")
	view := p.View(defaultTheme(), 60)
	if !strings.Contains(view, "no matches") {
		t.Fatalf("error message not in view")
	}
	// Typing should clear the error.
	p = p.Type('a')
	view = p.View(defaultTheme(), 60)
	if strings.Contains(view, "no matches") {
		t.Fatalf("error not cleared on Type")
	}
}

func TestPromptViewClosedReturnsEmpty(t *testing.T) {
	p := New() // closed
	if got := p.View(defaultTheme(), 60); got != "" {
		t.Fatalf("closed view = %q want empty", got)
	}
}

func TestPromptViewIncludesLabel(t *testing.T) {
	p := New().Open("my-repo")
	view := p.View(defaultTheme(), 60)
	if !strings.Contains(view, "my-repo") {
		t.Fatalf("view missing label, got:\n%s", view)
	}
}

func TestPromptValueTrimsSpace(t *testing.T) {
	p := New().Open("ws")
	for _, r := range "   Handle  " {
		p = p.Type(r)
	}
	if p.Value() != "Handle" {
		t.Fatalf("trimmed %q want Handle", p.Value())
	}
	if p.Raw() != "   Handle  " {
		t.Fatalf("raw %q want %q", p.Raw(), "   Handle  ")
	}
}
