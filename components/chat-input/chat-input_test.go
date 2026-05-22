package chatinput

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestPlaceholderShownWhenEmpty(t *testing.T) {
	i := New(theme.Default).WithPlaceholder("type a message").WithWidth(40)
	if !strings.Contains(i.View(), "type a message") {
		t.Errorf("expected placeholder in view, got %q", i.View())
	}
}

func TestPlaceholderHiddenAfterTyping(t *testing.T) {
	i := New(theme.Default).WithPlaceholder("type a message").WithWidth(40).WithValue("hello")
	out := i.View()
	if strings.Contains(out, "type a message") {
		t.Errorf("expected placeholder hidden once value present, got %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected value in view, got %q", out)
	}
}

func TestEnterEmitsSubmitMsg(t *testing.T) {
	i := New(theme.Default).WithValue("hello")
	next, cmd := i.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a tea.Cmd on submit")
	}
	msg := cmd()
	sub, ok := msg.(SubmitMsg)
	if !ok {
		t.Fatalf("expected SubmitMsg, got %T", msg)
	}
	if sub.Value != "hello" {
		t.Errorf("expected submit value 'hello', got %q", sub.Value)
	}
	if next.Value() != "" {
		t.Errorf("expected input cleared after submit, got %q", next.Value())
	}
}

func TestEnterIgnoredWhenEmpty(t *testing.T) {
	i := New(theme.Default)
	_, cmd := i.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected no command on enter with empty value")
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

func TestCtrlUClearsValue(t *testing.T) {
	i := New(theme.Default).WithValue("some text")
	next, _ := i.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if next.Value() != "" {
		t.Errorf("expected empty value after ctrl-u, got %q", next.Value())
	}
}

func TestRuneAppendsToValue(t *testing.T) {
	i := New(theme.Default).WithValue("he")
	next, _ := i.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("llo")})
	if next.Value() != "hello" {
		t.Errorf("expected 'hello', got %q", next.Value())
	}
}

func TestBlurredInputIgnoresKeys(t *testing.T) {
	i := New(theme.Default).Blur()
	next, cmd := i.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
	if cmd != nil || next.Value() != "" {
		t.Errorf("expected blurred input to ignore keys, got value=%q cmd=%v", next.Value(), cmd)
	}
}

func TestMinWidthClamped(t *testing.T) {
	i := New(theme.Default).WithWidth(1)
	if i.View() == "" {
		t.Error("expected non-empty render even with sub-minimum width")
	}
}
