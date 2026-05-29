package createprompt

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestNewIsClosed(t *testing.T) {
	p := New()
	if p.Open() {
		t.Fatalf("expected new prompt to be closed")
	}
	if p.Value() != "" {
		t.Fatalf("expected empty value, got %q", p.Value())
	}
}

func TestWithParentOpensWithEmptyValue(t *testing.T) {
	p := New().WithParent("internal/foo")
	if !p.Open() {
		t.Fatalf("expected open prompt")
	}
	if p.Value() != "" {
		t.Fatalf("expected empty value, got %q", p.Value())
	}
	if p.ParentRel() != "internal/foo" {
		t.Errorf("ParentRel: got %q, want %q", p.ParentRel(), "internal/foo")
	}
}

func TestCloseReopensEmpty(t *testing.T) {
	p := New().WithParent(".").Type('a').Type('b')
	p = p.Close()
	if p.Open() {
		t.Fatalf("expected closed")
	}
	if p.Value() != "" {
		t.Fatalf("expected close to clear value, got %q", p.Value())
	}
	p = p.WithParent(".")
	if p.Value() != "" {
		t.Fatalf("expected reopened prompt to be empty, got %q", p.Value())
	}
}

func TestTypeAcceptsPathRunes(t *testing.T) {
	p := New().WithParent(".")
	for _, r := range "foo/bar.go" {
		p = p.Type(r)
	}
	if p.Value() != "foo/bar.go" {
		t.Errorf("expected %q, got %q", "foo/bar.go", p.Value())
	}
}

func TestTypeAcceptsDotfile(t *testing.T) {
	p := New().WithParent(".")
	for _, r := range ".env" {
		p = p.Type(r)
	}
	if p.Value() != ".env" {
		t.Errorf("expected .env, got %q", p.Value())
	}
}

func TestTypeAcceptsTrailingSlash(t *testing.T) {
	p := New().WithParent(".")
	for _, r := range "newdir/" {
		p = p.Type(r)
	}
	if p.Value() != "newdir/" {
		t.Errorf("expected newdir/, got %q", p.Value())
	}
}

func TestTypeAcceptsHyphenAndUnderscore(t *testing.T) {
	p := New().WithParent(".")
	for _, r := range "my-thing_name.go" {
		p = p.Type(r)
	}
	if p.Value() != "my-thing_name.go" {
		t.Errorf("got %q", p.Value())
	}
}

func TestTypeRejectsControlChars(t *testing.T) {
	p := New().WithParent(".")
	p = p.Type('\x00')
	p = p.Type('\x1b')
	p = p.Type('\t')
	if p.Value() != "" {
		t.Errorf("expected control chars to be rejected, got %q", p.Value())
	}
}

func TestBackspaceDeletesLeft(t *testing.T) {
	p := New().WithParent(".").Type('a').Type('b').Type('c')
	p = p.Backspace()
	if p.Value() != "ab" {
		t.Errorf("after backspace expected ab, got %q", p.Value())
	}
	p = p.Backspace()
	p = p.Backspace()
	if p.Value() != "" {
		t.Errorf("expected empty after 3 backspaces, got %q", p.Value())
	}
	p = p.Backspace()
	if p.Value() != "" {
		t.Errorf("backspace on empty should be a no-op")
	}
}

func TestCursorMovement(t *testing.T) {
	p := New().WithParent(".").Type('a').Type('b').Type('c')
	p = p.MoveHome()
	p = p.Type('z')
	if p.Value() != "zabc" {
		t.Errorf("expected zabc, got %q", p.Value())
	}
	p = p.MoveEnd()
	p = p.Type('!')
	if p.Value() != "zabc!" {
		t.Errorf("expected zabc!, got %q", p.Value())
	}
	p = p.MoveLeft().MoveLeft()
	p = p.Type('X')
	if p.Value() != "zabXc!" {
		t.Errorf("expected zabXc!, got %q", p.Value())
	}
	p = p.MoveRight()
	p = p.Type('Y')
	if p.Value() != "zabXcY!" {
		t.Errorf("expected zabXcY!, got %q", p.Value())
	}
}

func TestErrorIsClearedOnInput(t *testing.T) {
	p := New().WithParent(".").Type('a')
	p = p.WithError("path exists")
	p = p.Type('b')
	out := p.View(theme.Default, 60)
	if strings.Contains(out, "path exists") {
		t.Errorf("expected error to be cleared by Type, view still contains it")
	}
	p = p.WithError("again")
	p = p.Backspace()
	out = p.View(theme.Default, 60)
	if strings.Contains(out, "again") {
		t.Errorf("expected error to be cleared by Backspace, view still contains it")
	}
}

func TestViewClosedReturnsEmpty(t *testing.T) {
	p := New()
	if got := p.View(theme.Default, 60); got != "" {
		t.Errorf("closed prompt should View as empty, got %q", got)
	}
}

func TestViewRendersTitleWhenOpen(t *testing.T) {
	p := New().WithParent("internal/foo")
	out := p.View(theme.Default, 60)
	if !strings.Contains(out, "internal/foo") {
		t.Errorf("expected parent in title, got: %q", out)
	}
}

func TestViewRendersValue(t *testing.T) {
	p := New().WithParent(".")
	for _, r := range "bar.go" {
		p = p.Type(r)
	}
	out := p.View(theme.Default, 60)
	if !strings.Contains(out, "bar.go") {
		t.Errorf("expected typed value in view, got: %q", out)
	}
}

func TestViewShowsErrorRow(t *testing.T) {
	p := New().WithParent(".").WithError("oh no")
	out := p.View(theme.Default, 60)
	if !strings.Contains(out, "oh no") {
		t.Errorf("expected error row to render, got: %q", out)
	}
}

func TestViewHandlesNarrowWidth(t *testing.T) {
	p := New().WithParent(".").Type('a')
	out := p.View(theme.Default, 4)
	if out == "" {
		t.Errorf("expected non-empty output even at narrow width")
	}
}
