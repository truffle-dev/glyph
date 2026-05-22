package progressbar

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestNewDefaults(t *testing.T) {
	b := New(theme.Default)
	if b.Percent() != 0 {
		t.Errorf("expected initial percent 0, got %v", b.Percent())
	}
	if got := lipgloss.Width(b.View()); got < 30 {
		t.Errorf("default width 30 should yield at least 30 cells, got %d", got)
	}
}

func TestPercentClampsBelow(t *testing.T) {
	b := New(theme.Default).WithPercent(-0.5)
	if b.Percent() != 0 {
		t.Fatalf("negative percent should clamp to 0, got %v", b.Percent())
	}
}

func TestPercentClampsAbove(t *testing.T) {
	b := New(theme.Default).WithPercent(1.5)
	if b.Percent() != 1 {
		t.Fatalf("over-range percent should clamp to 1, got %v", b.Percent())
	}
}

func TestWidthMinimum(t *testing.T) {
	b := New(theme.Default).WithWidth(0)
	out := b.View()
	if out == "" {
		t.Fatal("View should still render at min width")
	}
}

func TestViewContainsLabel(t *testing.T) {
	b := New(theme.Default).WithLabel("uploading").WithPercent(0.3)
	out := b.View()
	if !strings.Contains(out, "uploading") {
		t.Fatalf("View should include label, got %q", out)
	}
}

func TestViewShowsPercentByDefault(t *testing.T) {
	b := New(theme.Default).WithPercent(0.42)
	out := b.View()
	if !strings.Contains(out, "42%") {
		t.Fatalf("View should include 42%%, got %q", out)
	}
}

func TestViewHidesPercentWhenOff(t *testing.T) {
	b := New(theme.Default).WithPercent(0.42).WithPercentDisplay(false)
	out := b.View()
	if strings.Contains(out, "%") {
		t.Fatalf("View with display off should not include %%, got %q", out)
	}
}

func TestFullFillRendersAllFillRunes(t *testing.T) {
	b := New(theme.Default).WithPercent(1).WithWidth(10).WithRunes("#", ".")
	out := b.View()
	if !strings.Contains(out, strings.Repeat("#", 10)) {
		t.Fatalf("at percent 1.0 with width 10 the bar should be 10 fill runes, got %q", out)
	}
	if strings.Contains(out, ".") {
		t.Fatalf("full bar should have no empty runes, got %q", out)
	}
}

func TestEmptyFillRendersAllEmptyRunes(t *testing.T) {
	b := New(theme.Default).WithPercent(0).WithWidth(8).WithRunes("#", ".")
	out := b.View()
	if !strings.Contains(out, strings.Repeat(".", 8)) {
		t.Fatalf("empty bar should be all empty runes, got %q", out)
	}
}

func TestHalfFillRendersHalfFillRunes(t *testing.T) {
	b := New(theme.Default).WithPercent(0.5).WithWidth(10).WithRunes("#", ".")
	out := b.View()
	if !strings.Contains(out, strings.Repeat("#", 5)) {
		t.Errorf("half bar should have 5 fill runes, got %q", out)
	}
	if !strings.Contains(out, strings.Repeat(".", 5)) {
		t.Errorf("half bar should have 5 empty runes, got %q", out)
	}
}

func TestWithRunesIgnoresEmptyStrings(t *testing.T) {
	b := New(theme.Default).WithRunes("", "")
	out := b.View()
	if out == "" {
		t.Fatal("View should not be empty when WithRunes is called with empty strings (defaults kept)")
	}
}

func TestFillColorAccepted(t *testing.T) {
	b := New(theme.Default).WithFillColor(theme.Default.Success).WithPercent(0.5)
	if b.View() == "" {
		t.Fatal("View should render with custom fill color")
	}
}
