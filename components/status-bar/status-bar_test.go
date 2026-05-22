package statusbar

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestEmptyBarRendersPaddedWidth(t *testing.T) {
	b := New(theme.Default).WithWidth(40)
	out := b.View()
	if lipgloss.Width(out) != 40 {
		t.Fatalf("empty bar should pad to width 40, got %d", lipgloss.Width(out))
	}
}

func TestLeftSegmentRenders(t *testing.T) {
	b := New(theme.Default).WithWidth(60).WithLeft(
		Item{Text: "main.go"},
		Item{Text: "modified", Style: StyleWarning},
	)
	out := b.View()
	if !strings.Contains(out, "main.go") {
		t.Fatalf("left item missing, got %q", out)
	}
	if !strings.Contains(out, "modified") {
		t.Fatalf("second left item missing, got %q", out)
	}
}

func TestRightAnchored(t *testing.T) {
	b := New(theme.Default).WithWidth(40).WithLeft(
		Item{Text: "L"},
	).WithRight(
		Item{Text: "R"},
	)
	out := b.View()
	if lipgloss.Width(out) != 40 {
		t.Fatalf("bar should be 40 wide, got %d", lipgloss.Width(out))
	}
	idxL := strings.Index(out, "L")
	idxR := strings.LastIndex(out, "R")
	if idxL < 0 || idxR < 0 {
		t.Fatalf("left and right markers must render: %q", out)
	}
	if idxR-idxL < 10 {
		t.Fatalf("right should be anchored far from left, got %q", out)
	}
}

func TestCenterAppearsRoughlyCentered(t *testing.T) {
	b := New(theme.Default).WithWidth(60).WithLeft(
		Item{Text: "L"},
	).WithCenter(
		Item{Text: "CENTER"},
	).WithRight(
		Item{Text: "R"},
	)
	out := b.View()
	idx := strings.Index(out, "CENTER")
	if idx < 0 {
		t.Fatalf("center item missing, got %q", out)
	}
	// Strip ANSI by counting visible width up to CENTER. Easier: ensure
	// CENTER is neither at the very start nor at the very end.
	if idx < 5 || idx > len(out)-15 {
		t.Fatalf("CENTER should be roughly centered, got idx=%d in %q", idx, out)
	}
}

func TestWidthTooSmallTruncatesLeft(t *testing.T) {
	b := New(theme.Default).WithWidth(20).WithLeft(
		Item{Text: "a/very/long/path/to/main.go"},
	).WithRight(
		Item{Text: "OK", Style: StyleSuccess},
	)
	out := b.View()
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis from left truncation, got %q", out)
	}
	if !strings.Contains(out, "OK") {
		t.Fatalf("right segment should survive truncation, got %q", out)
	}
}

func TestEmptyItemTextSkipped(t *testing.T) {
	b := New(theme.Default).WithWidth(40).WithLeft(
		Item{Text: ""},
		Item{Text: "shown"},
	)
	out := b.View()
	if !strings.Contains(out, "shown") {
		t.Fatalf("non-empty item should render, got %q", out)
	}
}

func TestSeparatorBetweenItems(t *testing.T) {
	b := New(theme.Default).WithWidth(60).WithLeft(
		Item{Text: "one"},
		Item{Text: "two"},
	)
	out := b.View()
	if !strings.Contains(out, "·") {
		t.Fatalf("default separator should render between left items, got %q", out)
	}
}

func TestCustomSeparator(t *testing.T) {
	b := New(theme.Default).WithWidth(60).WithSeparator(" | ").WithLeft(
		Item{Text: "one"},
		Item{Text: "two"},
	)
	out := b.View()
	if !strings.Contains(out, "|") {
		t.Fatalf("custom separator should render, got %q", out)
	}
}

func TestAllSegmentsEmptyStillPads(t *testing.T) {
	b := New(theme.Default).WithWidth(30)
	if lipgloss.Width(b.View()) != 30 {
		t.Fatalf("width must be honored even with empty segments")
	}
}
