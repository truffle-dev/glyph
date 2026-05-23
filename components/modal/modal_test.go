package modal

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestNew_DefaultsAreSafe(t *testing.T) {
	m := New(theme.Default)
	if m.Width() < minWidth {
		t.Fatalf("default width should be >= %d, got %d", minWidth, m.Width())
	}
	if m.Height() < minHeight {
		t.Fatalf("default height should be >= %d, got %d", minHeight, m.Height())
	}
	// Empty body must render without panicking and produce a valid box.
	out := m.View()
	rows := strings.Split(out, "\n")
	if len(rows) != m.Height() {
		t.Fatalf("expected %d rows for height %d, got %d", m.Height(), m.Height(), len(rows))
	}
	for _, row := range rows {
		if got := lipgloss.Width(row); got != m.Width() {
			t.Errorf("row width %d != Width() %d in %q", got, m.Width(), row)
		}
	}
}

func TestWithSize_ClampsToMin(t *testing.T) {
	m := New(theme.Default).WithSize(2, 1)
	if m.Width() != minWidth {
		t.Errorf("expected width clamped to %d, got %d", minWidth, m.Width())
	}
	if m.Height() != minHeight {
		t.Errorf("expected height clamped to %d, got %d", minHeight, m.Height())
	}
}

func TestContentWidthHeight_AccountForBorderAndFooter(t *testing.T) {
	m := New(theme.Default).WithSize(30, 10)
	if got := m.ContentWidth(); got != 28 {
		t.Errorf("ContentWidth() = %d, expected %d (30 - 2 border)", got, 28)
	}
	if got := m.ContentHeight(); got != 8 {
		t.Errorf("ContentHeight() without footer = %d, expected 8", got)
	}
	withFooter := m.WithFooter("hint")
	if got := withFooter.ContentHeight(); got != 7 {
		t.Errorf("ContentHeight() with footer = %d, expected 7", got)
	}
	// ContentWidth should not change when a footer is set.
	if got := withFooter.ContentWidth(); got != 28 {
		t.Errorf("ContentWidth() with footer = %d, expected 28", got)
	}
}

func TestUpdate_CloseKey_EmitsCloseMsg(t *testing.T) {
	m := New(theme.Default)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should produce a non-nil command")
	}
	if _, ok := cmd().(CloseMsg); !ok {
		t.Fatalf("expected CloseMsg, got %T", cmd())
	}
}

func TestUpdate_CloseKey_Configurable(t *testing.T) {
	m := New(theme.Default).WithCloseKey("q")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should produce a non-nil command when configured as close key")
	}
	if _, ok := cmd().(CloseMsg); !ok {
		t.Fatalf("expected CloseMsg from q, got %T", cmd())
	}
	// Esc must NOT emit CloseMsg when only "q" is configured.
	_, cmd2 := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd2 != nil {
		t.Fatalf("Esc should not emit when close key is %q, got cmd msg %T", "q", cmd2())
	}
}

func TestUpdate_CloseKey_Empty_NoCloseEmitted(t *testing.T) {
	m := New(theme.Default).WithCloseKey("")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatalf("empty close key should disable close-on-key; got cmd msg %T", cmd())
	}
}

func TestUpdate_UnhandledKey_PassesThrough(t *testing.T) {
	m := New(theme.Default)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Fatalf("unhandled key must not emit a command, got %T", cmd())
	}
	// Non-key messages also pass through.
	updated2, cmd2 := m.Update(struct{ tag string }{tag: "anything"})
	if cmd2 != nil {
		t.Fatalf("non-key message must not emit a command, got %T", cmd2())
	}
	// Both updates should return an equivalent modal (no panic).
	if updated.Width() != m.Width() || updated2.Width() != m.Width() {
		t.Fatalf("unhandled messages must not mutate dimensions")
	}
}

func TestView_RendersTitleAndBody(t *testing.T) {
	m := New(theme.Default).
		WithSize(40, 8).
		WithTitle("Confirm").
		WithBody("Are you sure?")
	out := m.View()
	if !strings.Contains(out, "Confirm") {
		t.Errorf("View should include title, got %q", out)
	}
	if !strings.Contains(out, "Are you sure?") {
		t.Errorf("View should include body, got %q", out)
	}
}

func TestView_RendersFooter(t *testing.T) {
	m := New(theme.Default).
		WithSize(40, 8).
		WithFooter("esc cancel · enter save")
	out := m.View()
	if !strings.Contains(out, "esc cancel") {
		t.Errorf("View should include footer text, got %q", out)
	}
	// Output should still be (Height) rows wide.
	rows := strings.Split(out, "\n")
	if len(rows) != m.Height() {
		t.Fatalf("expected %d rows with footer, got %d", m.Height(), len(rows))
	}
}

func TestView_TruncatesTallBody(t *testing.T) {
	body := strings.Join([]string{"a", "b", "c", "d", "e", "f", "g", "h"}, "\n")
	m := New(theme.Default).WithSize(20, 5).WithBody(body)
	// height=5: top + 3 content rows + bottom. Body has 8 lines.
	out := m.View()
	rows := strings.Split(out, "\n")
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d (%q)", len(rows), out)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("expected truncation indicator … in tall body, got %q", out)
	}
	// First few body lines should be present.
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Errorf("expected first body lines present, got %q", out)
	}
}

func TestView_RoundedCorners(t *testing.T) {
	m := New(theme.Default).WithSize(20, 5)
	out := m.View()
	for _, g := range []string{tlCorner, trCorner, blCorner, brCorner} {
		if !strings.Contains(out, g) {
			t.Errorf("View should include corner %q, got %q", g, out)
		}
	}
}
