package timeline

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sample() []Event {
	return []Event{
		{Time: "13:42", Title: "Deploy started", Status: StatusInfo},
		{Time: "13:43", Title: "Tests passed", Body: "All green on staging.", Status: StatusSuccess},
		{Time: "13:45", Title: "Deploy complete", Status: StatusSuccess},
	}
}

func TestEmptyRendersPlaceholder(t *testing.T) {
	m := New().WithSize(40, 8)
	out := m.View()
	if !strings.Contains(out, "no events") {
		t.Fatalf("expected placeholder: %q", out)
	}
}

func TestRendersAllEvents(t *testing.T) {
	m := New().WithEvents(sample()...).WithSize(60, 12)
	out := m.View()
	for _, want := range []string{"13:42", "13:43", "13:45", "Deploy started", "Tests passed", "Deploy complete", "All green on staging."} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in view: %q", want, out)
		}
	}
}

func TestCursorMovementAndClamp(t *testing.T) {
	m := New().WithEvents(sample()...).WithSize(60, 20)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Fatalf("cursor want 2, got %d", m.Cursor())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Fatalf("cursor past end should clamp to 2, got %d", m.Cursor())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.Cursor() != 0 {
		t.Fatalf("home should go to 0, got %d", m.Cursor())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.Cursor() != 2 {
		t.Fatalf("end should go to last, got %d", m.Cursor())
	}
}

func TestSelectMsgOnEnter(t *testing.T) {
	m := New().WithEvents(sample()...).WithSize(60, 20).WithSelectedEvent(1)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected SelectMsg cmd")
	}
	msg := cmd()
	sel, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", msg)
	}
	if sel.Index != 1 {
		t.Fatalf("index want 1, got %d", sel.Index)
	}
	if sel.Event.Title != "Tests passed" {
		t.Fatalf("event title want Tests passed, got %q", sel.Event.Title)
	}
}

func TestSelectedEventAccess(t *testing.T) {
	m := New().WithEvents(sample()...).WithSelectedEvent(2)
	ev, ok := m.SelectedEvent()
	if !ok {
		t.Fatalf("expected SelectedEvent ok")
	}
	if ev.Title != "Deploy complete" {
		t.Fatalf("want Deploy complete, got %q", ev.Title)
	}
}

func TestEmptyNotSelectable(t *testing.T) {
	m := New()
	if _, ok := m.SelectedEvent(); ok {
		t.Fatalf("empty timeline should not return an event")
	}
	// Keys on empty should not panic and should not produce a command.
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatalf("expected no cmd on empty enter")
	}
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	events := make([]Event, 30)
	for i := range events {
		events[i] = Event{Time: "00:00", Title: "row"}
	}
	m := New().WithEvents(events...).WithSize(40, 6)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	out := m.View()
	if !strings.Contains(out, "row") {
		t.Fatalf("expected rendered rows after End: %q", out)
	}
	// Cursor at the last event must remain inside the rendered window;
	// the view height is bounded.
	lines := strings.Count(out, "\n") + 1
	if lines > 6 {
		t.Fatalf("rendered %d lines, height budget is 6", lines)
	}
}

func TestPageDownAdvancesCursor(t *testing.T) {
	events := make([]Event, 20)
	for i := range events {
		events[i] = Event{Time: "00:00", Title: "row"}
	}
	m := New().WithEvents(events...).WithSize(40, 6)
	pre := m.Cursor()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.Cursor() <= pre {
		t.Fatalf("pgdown should advance cursor, got %d -> %d", pre, m.Cursor())
	}
}

func TestTimeColumnHidden(t *testing.T) {
	m := New().WithEvents(sample()...).WithSize(50, 12).WithTimeColumn(false)
	out := m.View()
	if strings.Contains(out, "13:42") {
		t.Fatalf("hidden time column should not render timestamps: %q", out)
	}
	if !strings.Contains(out, "Deploy started") {
		t.Fatalf("titles should still render: %q", out)
	}
}

func TestStatusDotColorDoesNotPanic(t *testing.T) {
	events := []Event{
		{Title: "n", Status: StatusNeutral},
		{Title: "s", Status: StatusSuccess},
		{Title: "w", Status: StatusWarning},
		{Title: "e", Status: StatusError},
		{Title: "i", Status: StatusInfo},
	}
	m := New().WithEvents(events...).WithSize(40, 16)
	_ = m.View()
}

func TestTruncCell(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 6, "hello…"},
		{"abc", 1, "…"},
		{"", 5, ""},
	}
	for _, c := range cases {
		if got := truncCell(c.in, c.width); got != c.want {
			t.Fatalf("truncCell(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
	}
}
