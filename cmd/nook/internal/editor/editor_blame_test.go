package editor

import (
	"strings"
	"testing"
	"time"

	"github.com/truffle-dev/glyph/cmd/nook/internal/inlineblame"
	"github.com/truffle-dev/glyph/components/theme"
)

func TestSetBlameAccessorRoundtrip(t *testing.T) {
	p := NewPane(theme.Default)
	if _, ok := p.BlameAt(0); ok {
		t.Errorf("unset map should return ok=false")
	}
	p = p.SetBlame(map[int]inlineblame.Line{
		0: {Author: "Jane Doe", Summary: "fix"},
		3: {Author: "John Roe", Summary: "rename"},
	})
	got, ok := p.BlameAt(3)
	if !ok || got.Author != "John Roe" {
		t.Errorf("row 3 = %+v ok=%v", got, ok)
	}
	if _, ok := p.BlameAt(99); ok {
		t.Errorf("absent row should be ok=false")
	}
	p = p.SetBlame(nil)
	if _, ok := p.BlameAt(0); ok {
		t.Errorf("after clear ok should be false")
	}
}

func TestSetBlameVisibleDefaultsOff(t *testing.T) {
	p := NewPane(theme.Default)
	if p.BlameVisible() {
		t.Errorf("default visibility should be off")
	}
	p = p.SetBlameVisible(true)
	if !p.BlameVisible() {
		t.Errorf("after SetBlameVisible(true) should be on")
	}
}

func TestViewRendersBlameOnCursorRowOnly(t *testing.T) {
	saved := nowFunc
	nowFunc = func() time.Time {
		return time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	}
	defer func() { nowFunc = saved }()
	p := NewPane(theme.Default).WithSize(80, 8).Focus()
	p = p.ReplaceAllFromString("alpha\nbeta\ngamma\n")
	p = p.SetBlame(map[int]inlineblame.Line{
		0: {
			SHA:     "abc1234abc1234abc1234abc1234abc1234abcd",
			Author:  "Jane Doe",
			Time:    nowFunc().Add(-3 * 24 * time.Hour),
			Summary: "fix: handle empty inputs",
		},
		1: {
			SHA:     "def5678def5678def5678def5678def5678defab",
			Author:  "John Roe",
			Time:    nowFunc().Add(-2 * time.Hour),
			Summary: "refactor: split utils",
		},
	})
	p = p.SetBlameVisible(true)
	// Cursor defaults to row 0; only row 0's blame should render.
	out := plain(p.View())
	if !strings.Contains(out, "Jane Doe") {
		t.Errorf("expected Jane Doe blame on cursor row 0:\n%s", out)
	}
	if strings.Contains(out, "John Roe") {
		t.Errorf("did not expect John Roe blame on non-cursor row 1:\n%s", out)
	}
	if !strings.Contains(out, "3 days ago") {
		t.Errorf("expected '3 days ago' for cursor row 0:\n%s", out)
	}
}

func TestViewHidesBlameWhenInvisible(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 8).Focus()
	p = p.ReplaceAllFromString("hello\n")
	p = p.SetBlame(map[int]inlineblame.Line{
		0: {
			SHA:     "abc1234abc1234abc1234abc1234abc1234abcd",
			Author:  "Hidden Author",
			Summary: "should not appear",
		},
	})
	// SetBlameVisible NOT called → default off.
	out := plain(p.View())
	if strings.Contains(out, "Hidden Author") {
		t.Errorf("blame should not render when SetBlameVisible(false):\n%s", out)
	}
}

func TestViewBlameSkippedWhenNoRoomLeft(t *testing.T) {
	saved := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC) }
	defer func() { nowFunc = saved }()
	// Narrow width so the long line eats all the budget.
	p := NewPane(theme.Default).WithSize(40, 8).Focus()
	long := strings.Repeat("x", 40)
	p = p.ReplaceAllFromString(long + "\n")
	p = p.SetBlame(map[int]inlineblame.Line{
		0: {
			SHA:     "abc1234abc1234abc1234abc1234abc1234abcd",
			Author:  "Jane Doe",
			Time:    nowFunc().Add(-1 * time.Hour),
			Summary: "tight squeeze",
		},
	})
	p = p.SetBlameVisible(true)
	out := plain(p.View())
	if strings.Contains(out, "Jane Doe") {
		t.Errorf("blame should be elided when the line fills the budget:\n%s", out)
	}
}

func TestViewBlameRendersUncommittedTag(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 8).Focus()
	p = p.ReplaceAllFromString("just wrote this\n")
	p = p.SetBlame(map[int]inlineblame.Line{
		0: {SHA: inlineblame.UncommittedSHA, Author: "Not Committed Yet"},
	})
	p = p.SetBlameVisible(true)
	out := plain(p.View())
	if !strings.Contains(out, "(uncommitted)") {
		t.Errorf("expected (uncommitted) tag on uncommitted row:\n%s", out)
	}
}
