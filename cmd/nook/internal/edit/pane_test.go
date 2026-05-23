package edit

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func TestOpenResets(t *testing.T) {
	p := NewPane(theme.Default, nil)
	p = p.Open("/repo/main.go", 4, "x := 1")
	if p.State() != StateComposing {
		t.Fatalf("expected StateComposing after Open, got %d", p.State())
	}
	if p.Proposed() != "" {
		t.Fatalf("expected empty proposed, got %q", p.Proposed())
	}
}

func TestComposingTypeAndBackspace(t *testing.T) {
	p := NewPane(theme.Default, nil).Open("a.go", 0, "x := 1")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if p.prompt != "make" {
		t.Fatalf("expected prompt 'make', got %q", p.prompt)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.prompt != "mak" {
		t.Fatalf("expected prompt 'mak' after backspace, got %q", p.prompt)
	}
}

func TestEscEmitsCancel(t *testing.T) {
	p := NewPane(theme.Default, nil).Open("a.go", 0, "x := 1")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected Cmd from Esc")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd())
	}
}

func TestEnterWithoutPromptIsNoop(t *testing.T) {
	p := NewPane(theme.Default, nil).Open("a.go", 0, "x")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("expected no Cmd when prompt empty, got %v", cmd())
	}
}

func TestSubmitWithoutClientErrors(t *testing.T) {
	p := NewPane(theme.Default, nil).Open("a.go", 0, "x := 1")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("rename to y")})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.State() != StateError {
		t.Fatalf("expected StateError without client, got %d", p.State())
	}
	if !strings.Contains(p.errMsg, "claude CLI") {
		t.Fatalf("expected claude CLI error, got %q", p.errMsg)
	}
}

func TestStreamDeltaAndDoneTransition(t *testing.T) {
	p := NewPane(theme.Default, nil).Open("a.go", 3, "x := 1")
	p.state = StateStreaming
	p, _ = p.Update(streamDeltaMsg{text: "y := 1"})
	if p.Proposed() != "y := 1" {
		t.Fatalf("expected proposed accumulation, got %q", p.Proposed())
	}
	p, _ = p.Update(streamDoneMsg{})
	if p.State() != StateReview {
		t.Fatalf("expected StateReview after done, got %d", p.State())
	}
}

func TestReviewEnterEmitsAccept(t *testing.T) {
	p := NewPane(theme.Default, nil).Open("a.go", 7, "old")
	p.state = StateReview
	p.proposed = "new"
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected Cmd")
	}
	m, ok := cmd().(AcceptMsg)
	if !ok {
		t.Fatalf("expected AcceptMsg, got %T", cmd())
	}
	if m.Path != "a.go" || m.Line != 7 || m.NewText != "new" {
		t.Fatalf("AcceptMsg mismatch: %+v", m)
	}
}

func TestReviewRRetries(t *testing.T) {
	p := NewPane(theme.Default, nil).Open("a.go", 1, "x")
	p.state = StateReview
	p.proposed = "y"
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if p.State() != StateComposing {
		t.Fatalf("expected StateComposing after retry, got %d", p.State())
	}
	if p.Proposed() != "" {
		t.Fatalf("expected proposed cleared, got %q", p.Proposed())
	}
}

func TestStreamErrorTransitionsToError(t *testing.T) {
	p := NewPane(theme.Default, nil).Open("a.go", 0, "x")
	p.state = StateStreaming
	p, _ = p.Update(streamDoneMsg{err: errFake("boom")})
	if p.State() != StateError {
		t.Fatalf("expected StateError, got %d", p.State())
	}
	if p.errMsg != "boom" {
		t.Fatalf("expected errMsg 'boom', got %q", p.errMsg)
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }

func TestViewRenders(t *testing.T) {
	p := NewPane(theme.Default, nil).WithSize(50, 10).Open("a.go", 0, "x")
	out := p.View()
	if !strings.Contains(out, "instruction") {
		t.Fatalf("expected 'instruction' in view:\n%s", out)
	}
	if !strings.Contains(out, "edit a.go:1") {
		t.Fatalf("expected header in view:\n%s", out)
	}
}

func TestTrimToHandlesShort(t *testing.T) {
	if trimTo("abc", 10) != "abc" {
		t.Fatalf("short string passed through unchanged")
	}
	got := trimTo("hello world", 6)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis truncation, got %q", got)
	}
}
