package spinner

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestNewDefaults(t *testing.T) {
	s := New(theme.Default)
	if s.style != StyleDots {
		t.Fatalf("expected StyleDots default, got %v", s.style)
	}
	if len(s.frames) != 10 {
		t.Fatalf("dots style should have 10 frames, got %d", len(s.frames))
	}
	if s.interval != 80*time.Millisecond {
		t.Fatalf("default interval should be 80ms, got %v", s.interval)
	}
}

func TestWithStyleSwitchesFrames(t *testing.T) {
	tests := []struct {
		style Style
		count int
	}{
		{StyleDots, 10},
		{StyleLine, 4},
		{StyleArc, 6},
		{StylePulse, 4},
		{StyleBounce, 4},
	}
	for _, tc := range tests {
		s := New(theme.Default).WithStyle(tc.style)
		if len(s.frames) != tc.count {
			t.Errorf("style %v: expected %d frames, got %d", tc.style, tc.count, len(s.frames))
		}
	}
}

func TestWithStyleResetsOutOfRangeFrame(t *testing.T) {
	s := New(theme.Default)
	s.frame = 9
	s = s.WithStyle(StyleLine) // 4 frames
	if s.frame != 0 {
		t.Fatalf("frame should reset to 0 after switching to a shorter cycle, got %d", s.frame)
	}
}

func TestUpdateAdvancesFrameOnMatchingTick(t *testing.T) {
	s := New(theme.Default).WithID("primary")
	if s.frame != 0 {
		t.Fatalf("expected initial frame 0, got %d", s.frame)
	}
	s2, cmd := s.Update(TickMsg{ID: "primary", Time: time.Now()})
	if s2.frame != 1 {
		t.Fatalf("expected frame to advance to 1, got %d", s2.frame)
	}
	if cmd == nil {
		t.Fatal("Update should schedule the next tick when the tick matches")
	}
}

func TestUpdateIgnoresMismatchedID(t *testing.T) {
	s := New(theme.Default).WithID("primary")
	s2, cmd := s.Update(TickMsg{ID: "secondary", Time: time.Now()})
	if s2.frame != 0 {
		t.Fatalf("frame must not advance on mismatched ID, got %d", s2.frame)
	}
	if cmd != nil {
		t.Fatal("Update must not schedule a tick on mismatched ID")
	}
}

func TestUpdateWrapsAroundLastFrame(t *testing.T) {
	s := New(theme.Default).WithStyle(StyleLine) // 4 frames
	s.frame = 3
	s2, _ := s.Update(TickMsg{})
	if s2.frame != 0 {
		t.Fatalf("expected wrap to 0 from last frame, got %d", s2.frame)
	}
}

func TestUpdateIgnoresUnrelatedMessages(t *testing.T) {
	s := New(theme.Default)
	s.frame = 2
	s2, cmd := s.Update(tea.KeyMsg{})
	if s2.frame != 2 {
		t.Fatalf("frame must not change on non-TickMsg, got %d", s2.frame)
	}
	if cmd != nil {
		t.Fatal("Update must not return a command for non-TickMsg")
	}
}

func TestInitReturnsTickCommand(t *testing.T) {
	s := New(theme.Default)
	if s.Init() == nil {
		t.Fatal("Init must return a tea.Cmd")
	}
}

func TestViewIncludesLabel(t *testing.T) {
	s := New(theme.Default).WithLabel("Loading")
	out := s.View()
	if !strings.Contains(out, "Loading") {
		t.Fatalf("View should include the label, got %q", out)
	}
}

func TestViewWithoutLabelIsGlyphOnly(t *testing.T) {
	s := New(theme.Default)
	out := s.View()
	// Output should be one styled rune (the frame). The ANSI escape sequence
	// wraps a single visible character, so the visible length is 1.
	if lipgloss.Width(out) != 1 {
		t.Fatalf("View without label should be width 1, got width %d (%q)", lipgloss.Width(out), out)
	}
}

func TestWithIntervalEnforcesMinimum(t *testing.T) {
	s := New(theme.Default).WithInterval(1 * time.Millisecond)
	if s.interval < 16*time.Millisecond {
		t.Fatalf("interval should be clamped to 16ms minimum, got %v", s.interval)
	}
}
