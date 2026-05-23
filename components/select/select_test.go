package selectinput

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func opts(labels ...string) []Option {
	out := make([]Option, len(labels))
	for i, l := range labels {
		out[i] = Option{Label: l}
	}
	return out
}

func sample() []Option {
	return []Option{
		{Label: "claude-opus-4-7", Hint: "opus", Value: "opus-4-7"},
		{Label: "claude-sonnet-4-6", Hint: "sonnet", Value: "sonnet-4-6"},
		{Label: "claude-haiku-4-5", Hint: "haiku", Value: "haiku-4-5"},
		{Label: "gpt-5", Hint: "gpt", Value: "gpt-5"},
		{Label: "gpt-4o", Hint: "gpt", Value: "gpt-4o"},
	}
}

func TestNew_DefaultsAreSafe(t *testing.T) {
	s := New(theme.Default)
	if _, ok := s.Selected(); ok {
		t.Fatal("Selected should be false on empty Select")
	}
	// View should not panic on an empty Select.
	out := s.View()
	if out == "" {
		t.Fatal("View should produce some output even when empty")
	}
	// Tiny sizes are clamped, not honoured verbatim.
	s2 := New(theme.Default).WithSize(0, 0)
	if _ = s2.View(); s2.height < 1 {
		t.Fatalf("height should clamp to at least 1, got %d", s2.height)
	}
}

func TestWithOptions_AndCursorClamps(t *testing.T) {
	s := New(theme.Default).WithOptions(sample()).WithSelected(99)
	o, ok := s.Selected()
	if !ok {
		t.Fatal("Selected should be ok on non-empty Select")
	}
	if s.Cursor() != len(sample())-1 {
		t.Fatalf("WithSelected(99) should clamp to %d, got %d", len(sample())-1, s.Cursor())
	}
	if o.Label != "gpt-4o" {
		t.Fatalf("expected last option label gpt-4o, got %q", o.Label)
	}

	s2 := New(theme.Default).WithOptions(sample()).WithSelected(-5)
	if s2.Cursor() != 0 {
		t.Fatalf("WithSelected(-5) should clamp to 0, got %d", s2.Cursor())
	}
}

func TestUpdate_ArrowKeys(t *testing.T) {
	s := New(theme.Default).WithOptions(opts("a", "b", "c"))

	// Up at 0 clamps.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyUp})
	if s.Cursor() != 0 {
		t.Fatalf("Up at 0 should clamp, got %d", s.Cursor())
	}

	// Down advances.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyDown})
	if s.Cursor() != 1 {
		t.Fatalf("Down should advance to 1, got %d", s.Cursor())
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyDown})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyDown}) // already at last
	if s.Cursor() != 2 {
		t.Fatalf("Down past end should clamp, got %d", s.Cursor())
	}

	// Home / End.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyHome})
	if s.Cursor() != 0 {
		t.Fatalf("Home should go to 0, got %d", s.Cursor())
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if s.Cursor() != 2 {
		t.Fatalf("End should go to last, got %d", s.Cursor())
	}

	// j/k as runes when filter is off.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if s.Cursor() != 1 {
		t.Fatalf("k should step back to 1, got %d", s.Cursor())
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if s.Cursor() != 2 {
		t.Fatalf("j should step forward to 2, got %d", s.Cursor())
	}
}

func TestUpdate_Enter_EmitsSelectMsg(t *testing.T) {
	s := New(theme.Default).WithOptions(sample()).WithSelected(2)
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a tea.Cmd")
	}
	msg := cmd()
	sel, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", msg)
	}
	if sel.Index != 2 {
		t.Fatalf("expected Index 2, got %d", sel.Index)
	}
	if sel.Option.Value != "haiku-4-5" {
		t.Fatalf("expected Value haiku-4-5, got %q", sel.Option.Value)
	}
}

func TestUpdate_Tab_AlsoCommits(t *testing.T) {
	s := New(theme.Default).WithOptions(sample())
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Fatal("Tab should produce a tea.Cmd")
	}
	if _, ok := cmd().(SelectMsg); !ok {
		t.Fatalf("Tab should emit SelectMsg, got %T", cmd())
	}
}

func TestUpdate_Esc_EmitsCancelMsg(t *testing.T) {
	s := New(theme.Default).WithOptions(sample())
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should produce a tea.Cmd")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Fatalf("Esc should emit CancelMsg, got %T", cmd())
	}
}

func TestUpdate_Enter_NoOpOnEmpty(t *testing.T) {
	s := New(theme.Default)
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("Enter on empty Select should be a no-op, got %v", cmd())
	}
}

func TestFilter_NarrowsOptions(t *testing.T) {
	s := New(theme.Default).WithOptions(sample()).WithFilter(true)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("son")})
	o, ok := s.Selected()
	if !ok {
		t.Fatal("filtered select should still have a selection")
	}
	if !strings.Contains(strings.ToLower(o.Label), "sonnet") {
		t.Fatalf("expected a sonnet match, got %q", o.Label)
	}
	// Walking down stays within filtered set (only one match here).
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyDown})
	o2, _ := s.Selected()
	if o2.Label != o.Label {
		t.Fatalf("down with one match should stay put, got %q", o2.Label)
	}

	// Multi-match filter: 'gpt' returns gpt-5 + gpt-4o.
	s2 := New(theme.Default).WithOptions(sample()).WithFilter(true)
	s2, _ = s2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("gpt")})
	s2, _ = s2.Update(tea.KeyMsg{Type: tea.KeyDown})
	o3, _ := s2.Selected()
	if o3.Label != "gpt-4o" {
		t.Fatalf("expected second gpt match gpt-4o, got %q", o3.Label)
	}
}

func TestFilter_NoMatch_CursorStillSafe(t *testing.T) {
	s := New(theme.Default).WithOptions(sample()).WithFilter(true)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zzz")})
	o, ok := s.Selected()
	if ok {
		t.Fatalf("expected Selected to be false on no-match, got %+v", o)
	}
	if o.Label != "" || o.Value != "" {
		t.Fatalf("expected zero Option on no-match, got %+v", o)
	}
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("Enter on no-match should be a no-op, got %v", cmd())
	}
}

func TestFilter_Backspace_NarrowsBack(t *testing.T) {
	s := New(theme.Default).WithOptions(sample()).WithFilter(true)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zz")})
	if _, ok := s.Selected(); ok {
		t.Fatal("expected no match after typing zz")
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if s.filter != "" {
		t.Fatalf("expected empty filter after two backspaces, got %q", s.filter)
	}
	if _, ok := s.Selected(); !ok {
		t.Fatal("expected a selection after clearing filter")
	}
}

func TestFilter_CtrlU_ClearsFilter(t *testing.T) {
	s := New(theme.Default).WithOptions(sample()).WithFilter(true)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("son")})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if s.filter != "" {
		t.Fatalf("Ctrl-U should clear the filter, got %q", s.filter)
	}
}

func TestFilter_OffIgnoresRuneTyping(t *testing.T) {
	s := New(theme.Default).WithOptions(sample())
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("xyz")})
	if s.filter != "" {
		t.Fatalf("filter-off Select should ignore runes, got %q", s.filter)
	}
}

func TestView_RendersTitleAndOptions(t *testing.T) {
	s := New(theme.Default).
		WithOptions(sample()).
		WithSize(40, 5).
		WithTitle("Pick a model")
	out := s.View()
	if !strings.Contains(out, "Pick a model") {
		t.Errorf("View should contain the title, got %q", out)
	}
	for _, want := range []string{"claude-opus-4-7", "opus", "claude-sonnet-4-6"} {
		if !strings.Contains(out, want) {
			t.Errorf("View should include %q, got %q", want, out)
		}
	}
	if !strings.Contains(out, "›") {
		t.Errorf("View should include a cursor marker, got %q", out)
	}
}

func TestUpdate_IgnoresNonKeyMessages(t *testing.T) {
	s := New(theme.Default).WithOptions(sample())
	s2, cmd := s.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if s2.Cursor() != s.Cursor() {
		t.Fatalf("non-key msg should not move cursor")
	}
	if cmd != nil {
		t.Fatal("non-key msg should not produce a cmd")
	}
}

func TestSelectMsg_FallsBackToLabelWhenValueEmpty(t *testing.T) {
	s := New(theme.Default).WithOptions([]Option{{Label: "no-value-here"}})
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should emit a SelectMsg")
	}
	sel := cmd().(SelectMsg)
	if sel.Option.Value != "no-value-here" {
		t.Fatalf("expected Value to fall back to Label, got %q", sel.Option.Value)
	}
}

func TestView_ScrollsWithCursor(t *testing.T) {
	s := New(theme.Default).
		WithOptions(opts("a", "b", "c", "d", "e", "f", "g", "h")).
		WithSize(20, 3)
	for i := 0; i < 5; i++ {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	out := s.View()
	// Cursor is at 5 ("f"). Window of 3 means d/e/f should be visible
	// and a/b/c hidden.
	for _, want := range []string{"d", "e", "f"} {
		if !strings.Contains(out, want) {
			t.Errorf("scrolled view should include %q, got %q", want, out)
		}
	}
}
