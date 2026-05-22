package list

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func items(labels ...string) []Item {
	out := make([]Item, len(labels))
	for i, l := range labels {
		out[i] = Item{Label: l}
	}
	return out
}

func TestNewIsEmpty(t *testing.T) {
	l := New(theme.Default)
	if _, _, ok := l.Selected(); ok {
		t.Fatal("Selected should be false on empty list")
	}
}

func TestSelectedReturnsCursorItem(t *testing.T) {
	l := New(theme.Default).WithItems(items("a", "b", "c"))
	it, idx, ok := l.Selected()
	if !ok {
		t.Fatal("Selected should be ok on non-empty list")
	}
	if it.Label != "a" || idx != 0 {
		t.Fatalf("expected first item at index 0, got %q at %d", it.Label, idx)
	}
}

func TestDownArrowAdvancesCursor(t *testing.T) {
	l := New(theme.Default).WithItems(items("a", "b", "c"))
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyDown})
	if l.Cursor() != 1 {
		t.Fatalf("expected cursor 1 after down, got %d", l.Cursor())
	}
}

func TestUpArrowMovesCursor(t *testing.T) {
	l := New(theme.Default).WithItems(items("a", "b", "c"))
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyDown})
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyDown})
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyUp})
	if l.Cursor() != 1 {
		t.Fatalf("expected cursor 1 after down,down,up, got %d", l.Cursor())
	}
}

func TestCursorClampsAtEnds(t *testing.T) {
	l := New(theme.Default).WithItems(items("a", "b"))
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyUp}) // already at 0
	if l.Cursor() != 0 {
		t.Errorf("cursor should clamp at 0, got %d", l.Cursor())
	}
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyDown})
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyDown}) // already at last
	if l.Cursor() != 1 {
		t.Errorf("cursor should clamp at last, got %d", l.Cursor())
	}
}

func TestHomeAndEndKeys(t *testing.T) {
	l := New(theme.Default).WithItems(items("a", "b", "c", "d"))
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if l.Cursor() != 3 {
		t.Fatalf("End should go to last, got %d", l.Cursor())
	}
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyHome})
	if l.Cursor() != 0 {
		t.Fatalf("Home should go to first, got %d", l.Cursor())
	}
}

func TestDisabledItemsAreSkipped(t *testing.T) {
	l := New(theme.Default).WithItems([]Item{
		{Label: "a"},
		{Label: "b", Disabled: true},
		{Label: "c"},
	})
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyDown})
	if l.Cursor() != 2 {
		t.Fatalf("cursor should skip disabled item to land on 2, got %d", l.Cursor())
	}
}

func TestWithItemsStartsOnFirstEnabled(t *testing.T) {
	l := New(theme.Default).WithItems([]Item{
		{Label: "a", Disabled: true},
		{Label: "b"},
		{Label: "c"},
	})
	if l.Cursor() != 1 {
		t.Fatalf("cursor should land on first enabled (1), got %d", l.Cursor())
	}
}

func TestViewIncludesAllVisibleLabels(t *testing.T) {
	l := New(theme.Default).WithItems(items("alpha", "beta", "gamma"))
	out := l.View()
	for _, label := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(out, label) {
			t.Errorf("View should include %q, got %q", label, out)
		}
	}
}

func TestViewWindowScrollsWithCursor(t *testing.T) {
	l := New(theme.Default).
		WithItems(items("a", "b", "c", "d", "e", "f", "g", "h")).
		WithHeight(3)
	for i := 0; i < 5; i++ {
		l, _ = l.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	out := l.View()
	// Cursor is at 5 ("f"), window height 3 means rows d/e/f should be visible
	// and a/b/c hidden.
	for _, want := range []string{"d", "e", "f"} {
		if !strings.Contains(out, want) {
			t.Errorf("scrolled window should include %q, got %q", want, out)
		}
	}
	for _, gone := range []string{"a", "b", "c"} {
		if strings.Contains(out, gone) {
			t.Errorf("scrolled window should NOT include %q, got %q", gone, out)
		}
	}
}

func TestViewEmptyShowsEmptyPlaceholder(t *testing.T) {
	l := New(theme.Default)
	out := l.View()
	if !strings.Contains(out, "empty") {
		t.Fatalf("empty list view should mention empty, got %q", out)
	}
}

func TestUpdateIgnoresNonKeyMessages(t *testing.T) {
	l := New(theme.Default).WithItems(items("a", "b"))
	l2, cmd := l.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if l2.Cursor() != l.Cursor() {
		t.Fatalf("non-key msg should not move cursor")
	}
	if cmd != nil {
		t.Fatal("Update never returns a tea.Cmd")
	}
}

func TestSelectedExposesValue(t *testing.T) {
	l := New(theme.Default).WithItems([]Item{
		{Label: "first", Value: 42},
		{Label: "second", Value: "hello"},
	})
	it, _, _ := l.Selected()
	if it.Value.(int) != 42 {
		t.Fatalf("expected Value 42, got %v", it.Value)
	}
}

func TestHintRenders(t *testing.T) {
	l := New(theme.Default).WithItems([]Item{
		{Label: "Tab", Hint: "next pane"},
	})
	out := l.View()
	if !strings.Contains(out, "next pane") {
		t.Fatalf("View should include hint, got %q", out)
	}
}
