package tabs

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestNewIsEmpty(t *testing.T) {
	tb := New(theme.Default)
	if len(tb.Labels()) != 0 {
		t.Fatalf("expected empty label set on New, got %d", len(tb.Labels()))
	}
	if tb.Active() != 0 {
		t.Fatalf("expected active 0 on New, got %d", tb.Active())
	}
}

func TestWithTabsResetsActiveWhenOutOfRange(t *testing.T) {
	tb := New(theme.Default).WithTabs([]string{"a", "b", "c"}).WithActive(2)
	tb = tb.WithTabs([]string{"x", "y"})
	if tb.Active() != 0 {
		t.Fatalf("active index should reset to 0 when shrinking the label set, got %d", tb.Active())
	}
}

func TestWithActiveClampsOutOfRange(t *testing.T) {
	tb := New(theme.Default).WithTabs([]string{"a", "b", "c"})
	if got := tb.WithActive(-5).Active(); got != 0 {
		t.Errorf("negative index should clamp to 0, got %d", got)
	}
	if got := tb.WithActive(99).Active(); got != 2 {
		t.Errorf("over-range index should clamp to last, got %d", got)
	}
}

func TestWithActiveOnEmptyStaysZero(t *testing.T) {
	tb := New(theme.Default).WithActive(7)
	if tb.Active() != 0 {
		t.Fatalf("active must stay 0 on empty label set, got %d", tb.Active())
	}
}

func TestUpdateCyclesRight(t *testing.T) {
	tb := New(theme.Default).WithTabs([]string{"a", "b", "c"})
	tb, _ = tb.Update(tea.KeyMsg{Type: tea.KeyRight})
	if tb.Active() != 1 {
		t.Fatalf("right should move to 1, got %d", tb.Active())
	}
}

func TestUpdateCyclesLeftWraps(t *testing.T) {
	tb := New(theme.Default).WithTabs([]string{"a", "b", "c"})
	tb, _ = tb.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if tb.Active() != 2 {
		t.Fatalf("left from 0 should wrap to last (2), got %d", tb.Active())
	}
}

func TestUpdateCyclesRightWraps(t *testing.T) {
	tb := New(theme.Default).WithTabs([]string{"a", "b"}).WithActive(1)
	tb, _ = tb.Update(tea.KeyMsg{Type: tea.KeyRight})
	if tb.Active() != 0 {
		t.Fatalf("right from last should wrap to 0, got %d", tb.Active())
	}
}

func TestUpdateTabAndShiftTab(t *testing.T) {
	tb := New(theme.Default).WithTabs([]string{"a", "b", "c"})
	tb, _ = tb.Update(tea.KeyMsg{Type: tea.KeyTab})
	if tb.Active() != 1 {
		t.Fatalf("tab should advance to 1, got %d", tb.Active())
	}
	tb, _ = tb.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if tb.Active() != 0 {
		t.Fatalf("shift+tab should return to 0, got %d", tb.Active())
	}
}

func TestUpdateIgnoresOtherKeys(t *testing.T) {
	tb := New(theme.Default).WithTabs([]string{"a", "b"}).WithActive(1)
	tb, cmd := tb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if tb.Active() != 1 {
		t.Fatalf("unrelated keys should not change active, got %d", tb.Active())
	}
	if cmd != nil {
		t.Fatal("Update never returns a tea.Cmd")
	}
}

func TestUpdateIgnoresNonKeyMessages(t *testing.T) {
	tb := New(theme.Default).WithTabs([]string{"a", "b"}).WithActive(1)
	tb2, cmd := tb.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if tb2.Active() != 1 {
		t.Fatalf("non-key message must not change active, got %d", tb2.Active())
	}
	if cmd != nil {
		t.Fatal("Update never returns a tea.Cmd")
	}
}

func TestViewIncludesAllLabels(t *testing.T) {
	tb := New(theme.Default).WithTabs([]string{"chat", "logs", "diff"})
	out := tb.View()
	for _, label := range []string{"chat", "logs", "diff"} {
		if !strings.Contains(out, label) {
			t.Errorf("View should include label %q, got %q", label, out)
		}
	}
	if !strings.Contains(out, strings.TrimSpace(Separator)) {
		t.Errorf("View should include the separator glyph, got %q", out)
	}
}

func TestViewEmptyReturnsEmpty(t *testing.T) {
	tb := New(theme.Default)
	if tb.View() != "" {
		t.Fatalf("View on empty Tabs should be empty string, got %q", tb.View())
	}
}
