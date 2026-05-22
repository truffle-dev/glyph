package logstream

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func ts(s string) time.Time {
	t, _ := time.Parse("15:04:05", s)
	return t
}

func TestEmptyStreamShowsPlaceholder(t *testing.T) {
	s := New(theme.Default).WithSize(40, 5)
	out := s.View()
	if !strings.Contains(out, "No log entries.") {
		t.Fatalf("empty stream should show placeholder, got %q", out)
	}
}

func TestAppendsRenderInOrder(t *testing.T) {
	s := New(theme.Default).WithSize(60, 10).WithTimestamps(false)
	s = s.Append(Entry{Level: LevelInfo, Message: "first"})
	s = s.Append(Entry{Level: LevelInfo, Message: "second"})
	out := s.View()
	firstIdx := strings.Index(out, "first")
	secondIdx := strings.Index(out, "second")
	if firstIdx < 0 || secondIdx < 0 {
		t.Fatalf("both messages must render, got %q", out)
	}
	if firstIdx > secondIdx {
		t.Fatalf("first must render above second in tail view, got %q", out)
	}
}

func TestCapacityDropsOldest(t *testing.T) {
	s := New(theme.Default).WithSize(60, 10).WithCapacity(3).WithTimestamps(false)
	for i, msg := range []string{"a", "b", "c", "d"} {
		_ = i
		s = s.Append(Entry{Level: LevelInfo, Message: msg})
	}
	if got := len(s.Entries()); got != 3 {
		t.Fatalf("capacity=3 should keep 3 entries, got %d", got)
	}
	out := s.View()
	if strings.Contains(out, "a") && !strings.Contains(out, "b") {
		// "a" should be dropped; if present it's a bug
		t.Fatalf("oldest entry should be dropped, got %q", out)
	}
}

func TestMinLevelFiltersBelow(t *testing.T) {
	s := New(theme.Default).WithSize(80, 10).WithMinLevel(LevelWarn).WithTimestamps(false)
	s = s.Append(Entry{Level: LevelDebug, Message: "debug-msg"})
	s = s.Append(Entry{Level: LevelInfo, Message: "info-msg"})
	s = s.Append(Entry{Level: LevelWarn, Message: "warn-msg"})
	s = s.Append(Entry{Level: LevelError, Message: "error-msg"})
	out := s.View()
	if strings.Contains(out, "debug-msg") {
		t.Fatalf("debug should be filtered, got %q", out)
	}
	if strings.Contains(out, "info-msg") {
		t.Fatalf("info should be filtered, got %q", out)
	}
	if !strings.Contains(out, "warn-msg") {
		t.Fatalf("warn should render, got %q", out)
	}
	if !strings.Contains(out, "error-msg") {
		t.Fatalf("error should render, got %q", out)
	}
}

func TestTimestampsCanBeHidden(t *testing.T) {
	s := New(theme.Default).WithSize(60, 5).WithTimestamps(false)
	s = s.Append(Entry{Time: ts("12:34:56"), Level: LevelInfo, Message: "hello"})
	out := s.View()
	if strings.Contains(out, "12:34:56") {
		t.Fatalf("timestamp should be hidden, got %q", out)
	}
}

func TestSourceLabelRenders(t *testing.T) {
	s := New(theme.Default).WithSize(80, 5).WithTimestamps(false)
	s = s.Append(Entry{Level: LevelInfo, Source: "auth", Message: "login ok"})
	out := s.View()
	if !strings.Contains(out, "auth") {
		t.Fatalf("source label must render, got %q", out)
	}
	if !strings.Contains(out, "login ok") {
		t.Fatalf("message must render, got %q", out)
	}
}

func TestScrollUpHoldsOnAppend(t *testing.T) {
	s := New(theme.Default).WithSize(60, 3).WithTimestamps(false)
	for i := 0; i < 10; i++ {
		s = s.Append(Entry{Level: LevelInfo, Message: "msg"})
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyUp})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyUp})
	holdOffset := s.Offset()
	if holdOffset == 0 {
		t.Fatal("offset should be > 0 after KeyUp twice")
	}
	s = s.Append(Entry{Level: LevelInfo, Message: "newer"})
	if s.Offset() != holdOffset {
		t.Fatalf("appending while scrolled up must preserve offset, was %d now %d", holdOffset, s.Offset())
	}
}

func TestEndJumpsToTail(t *testing.T) {
	s := New(theme.Default).WithSize(60, 3).WithTimestamps(false)
	for i := 0; i < 10; i++ {
		s = s.Append(Entry{Level: LevelInfo, Message: "msg"})
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyHome})
	if s.Offset() == 0 {
		t.Fatal("Home should move off the tail")
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if s.Offset() != 0 {
		t.Fatalf("End must return to tail, got offset %d", s.Offset())
	}
}

func TestClearResets(t *testing.T) {
	s := New(theme.Default).WithSize(40, 5).WithTimestamps(false)
	s = s.Append(Entry{Level: LevelInfo, Message: "x"})
	s = s.Clear()
	if len(s.Entries()) != 0 {
		t.Fatal("Clear should empty entries")
	}
	if s.Offset() != 0 {
		t.Fatal("Clear should reset offset")
	}
}

func TestLevelString(t *testing.T) {
	cases := map[Level]string{
		LevelDebug: "DBUG",
		LevelInfo:  "INFO",
		LevelWarn:  "WARN",
		LevelError: "ERRO",
	}
	for lvl, want := range cases {
		if got := lvl.String(); got != want {
			t.Fatalf("Level(%d).String() = %q, want %q", lvl, got, want)
		}
	}
}
