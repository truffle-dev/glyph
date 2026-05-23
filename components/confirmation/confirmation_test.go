package confirmation

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestNew_DefaultsAreSafe(t *testing.T) {
	c := New(theme.Default)
	if c.FocusedYes() {
		t.Fatalf("default focus must be No, got Yes")
	}
	if c.yesLabel != "Yes" {
		t.Fatalf("default yes label should be %q, got %q", "Yes", c.yesLabel)
	}
	if c.noLabel != "No" {
		t.Fatalf("default no label should be %q, got %q", "No", c.noLabel)
	}
	c = c.WithWidth(5)
	if c.width != 20 {
		t.Fatalf("width should clamp to 20 minimum, got %d", c.width)
	}
}

func TestWithDefault_TrueFocusesYes(t *testing.T) {
	c := New(theme.Default).WithDefault(true)
	if !c.FocusedYes() {
		t.Fatalf("WithDefault(true) should focus Yes")
	}
	c = c.WithDefault(false)
	if c.FocusedYes() {
		t.Fatalf("WithDefault(false) should focus No")
	}
}

func TestUpdate_TabMovesFocus(t *testing.T) {
	c := New(theme.Default) // No focused
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !c.FocusedYes() {
		t.Fatalf("Tab should move focus to Yes")
	}
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyTab})
	if c.FocusedYes() {
		t.Fatalf("Tab should wrap back to No")
	}
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if !c.FocusedYes() {
		t.Fatalf("Shift+Tab should move focus to Yes")
	}
}

func TestUpdate_LeftRightMoveFocus(t *testing.T) {
	c := New(theme.Default) // No focused
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRight})
	if !c.FocusedYes() {
		t.Fatalf("Right should move focus to Yes")
	}
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if c.FocusedYes() {
		t.Fatalf("Left should move focus back to No")
	}
	// h/l vim-style
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if !c.FocusedYes() {
		t.Fatalf("'l' should move focus to Yes")
	}
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if c.FocusedYes() {
		t.Fatalf("'h' should move focus to No")
	}
}

func TestUpdate_YKeyCommitsYes(t *testing.T) {
	c := New(theme.Default) // No focused
	c, cmd := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !c.FocusedYes() {
		t.Fatalf("'y' should focus Yes")
	}
	if cmd == nil {
		t.Fatal("'y' must emit a tea.Cmd")
	}
	msg := cmd()
	cm, ok := msg.(ConfirmMsg)
	if !ok {
		t.Fatalf("expected ConfirmMsg, got %T", msg)
	}
	if !cm.Value {
		t.Fatalf("ConfirmMsg.Value must be true for 'y'")
	}

	// Uppercase Y also commits.
	c2 := New(theme.Default)
	_, cmd2 := c2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	if cmd2 == nil {
		t.Fatal("'Y' must emit a tea.Cmd")
	}
	if cm2, _ := cmd2().(ConfirmMsg); !cm2.Value {
		t.Fatalf("'Y' must commit with Value=true")
	}
}

func TestUpdate_NKeyCommitsNo(t *testing.T) {
	c := New(theme.Default).WithDefault(true) // Yes focused
	c, cmd := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if c.FocusedYes() {
		t.Fatalf("'n' should focus No")
	}
	if cmd == nil {
		t.Fatal("'n' must emit a tea.Cmd")
	}
	cm, ok := cmd().(ConfirmMsg)
	if !ok {
		t.Fatalf("expected ConfirmMsg, got %T", cmd())
	}
	if cm.Value {
		t.Fatalf("ConfirmMsg.Value must be false for 'n'")
	}

	// Uppercase N also commits.
	c2 := New(theme.Default).WithDefault(true)
	_, cmd2 := c2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	if cmd2 == nil {
		t.Fatal("'N' must emit a tea.Cmd")
	}
	if cm2, _ := cmd2().(ConfirmMsg); cm2.Value {
		t.Fatalf("'N' must commit with Value=false")
	}
}

func TestUpdate_EnterCommitsFocused(t *testing.T) {
	// No focused -> Enter emits Value=false.
	c := New(theme.Default)
	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter must emit a tea.Cmd")
	}
	if cm, _ := cmd().(ConfirmMsg); cm.Value {
		t.Fatalf("Enter on No-focused must emit Value=false")
	}

	// Yes focused -> Enter emits Value=true.
	c2 := New(theme.Default).WithDefault(true)
	_, cmd2 := c2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd2 == nil {
		t.Fatal("Enter must emit a tea.Cmd")
	}
	if cm, _ := cmd2().(ConfirmMsg); !cm.Value {
		t.Fatalf("Enter on Yes-focused must emit Value=true")
	}
}

func TestUpdate_EscEmitsCancel(t *testing.T) {
	c := New(theme.Default)
	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc must emit a tea.Cmd")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd())
	}
}

func TestUpdate_IgnoresNonKeyMessages(t *testing.T) {
	c := New(theme.Default).WithDefault(true)
	c2, cmd := c.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if c2.FocusedYes() != c.FocusedYes() {
		t.Fatalf("non-key message must not change focus")
	}
	if cmd != nil {
		t.Fatal("non-key message must not emit a tea.Cmd")
	}
}

func TestView_RendersPromptAndButtons(t *testing.T) {
	c := New(theme.Default).WithPrompt("Discard unsaved changes?")
	out := c.View()
	if !strings.Contains(out, "Discard unsaved changes?") {
		t.Errorf("View should contain the prompt, got %q", out)
	}
	if !strings.Contains(out, "Yes") {
		t.Errorf("View should contain the Yes label, got %q", out)
	}
	if !strings.Contains(out, "No") {
		t.Errorf("View should contain the No label, got %q", out)
	}
	if !strings.Contains(out, "y/n") {
		t.Errorf("View should contain the y/n hint, got %q", out)
	}
}

func TestView_DangerousStyleApplied(t *testing.T) {
	c := New(theme.Default).
		WithPrompt("Delete the production database?").
		WithDangerous(true).
		WithDefault(true) // focus Yes so the danger color renders strongly
	out := c.View()

	// The dangerous focused Yes button paints background = theme.Error. Compare
	// the substring lipgloss emits when rendering with that background.
	danger := lipgloss.NewStyle().
		Foreground(theme.Default.TextInverse).
		Background(theme.Default.Error).
		Bold(true).
		Render(" Yes ")
	if !strings.Contains(out, danger) {
		t.Errorf("View with WithDangerous(true) and Yes focused should contain the theme.Error background sequence, got %q", out)
	}
}

func TestView_ReflowsLongPrompt(t *testing.T) {
	long := strings.Repeat("alpha bravo charlie delta echo foxtrot ", 10) // ~390 chars
	long = long[:200]
	c := New(theme.Default).WithPrompt(long).WithWidth(40)
	out := c.View()
	if !strings.Contains(out, "\n") {
		t.Fatalf("long prompt at width 40 should wrap to multiple lines, got %q", out)
	}

	// Sanity: at least one prompt line should be present and the wrap should
	// produce more than the three trailing structural lines (blank, buttons,
	// blank, hint).
	lines := strings.Split(out, "\n")
	if len(lines) < 6 {
		t.Fatalf("expected several wrapped lines plus buttons + hint, got %d lines: %q", len(lines), out)
	}
}
