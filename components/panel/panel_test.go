package panel

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestViewIncludesContent(t *testing.T) {
	p := New(theme.Default).WithContent("hello world")
	out := p.View()
	if !strings.Contains(out, "hello world") {
		t.Fatalf("View should include content, got %q", out)
	}
}

func TestViewIncludesTitle(t *testing.T) {
	p := New(theme.Default).WithTitle("Logs").WithContent("line one")
	out := p.View()
	if !strings.Contains(out, "Logs") {
		t.Fatalf("View should include title, got %q", out)
	}
}

func TestViewIncludesFooter(t *testing.T) {
	p := New(theme.Default).WithFooter("done").WithContent("body")
	out := p.View()
	if !strings.Contains(out, "done") {
		t.Fatalf("View should include footer, got %q", out)
	}
}

func TestViewHasRoundedCorners(t *testing.T) {
	p := New(theme.Default).WithContent("x")
	out := p.View()
	for _, glyph := range []string{tlCorner, trCorner, blCorner, brCorner} {
		if !strings.Contains(out, glyph) {
			t.Errorf("View should include border corner %q, got %q", glyph, out)
		}
	}
}

func TestViewMultilineContent(t *testing.T) {
	p := New(theme.Default).WithContent("alpha\nbeta\ngamma")
	out := p.View()
	for _, line := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(out, line) {
			t.Errorf("View should include line %q, got %q", line, out)
		}
	}
	// Body lines + 2 border rows.
	if got := strings.Count(out, "\n"); got != 4 {
		t.Errorf("expected 4 newlines (5 rows), got %d in %q", got, out)
	}
}

func TestWidthClampsOuter(t *testing.T) {
	p := New(theme.Default).WithContent("hi").WithWidth(20)
	out := p.View()
	// Each rendered row should have visible width 20.
	for _, row := range strings.Split(out, "\n") {
		if got := lipgloss.Width(row); got != 20 {
			t.Errorf("row %q has width %d, expected 20", row, got)
		}
	}
}

func TestHeightClampsOuter(t *testing.T) {
	p := New(theme.Default).WithContent("a\nb\nc\nd\ne").WithHeight(5)
	out := p.View()
	rows := strings.Split(out, "\n")
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
}

func TestHeightPadsWhenContentShorter(t *testing.T) {
	p := New(theme.Default).WithContent("only one line").WithHeight(6)
	out := p.View()
	rows := strings.Split(out, "\n")
	if len(rows) != 6 {
		t.Fatalf("expected 6 rows when padding short content, got %d", len(rows))
	}
}

func TestPaddingExpandsRows(t *testing.T) {
	p := New(theme.Default).WithContent("x").WithPadding(2, 1)
	out := p.View()
	// Padding(2, 1): one blank row above, one below the "x" row, plus borders.
	rows := strings.Split(out, "\n")
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows with padding 2,1 and one content line, got %d (%q)", len(rows), out)
	}
}

func TestVariantStrongStillRenders(t *testing.T) {
	p := New(theme.Default).WithContent("x").WithVariant(VariantStrong)
	out := p.View()
	if !strings.Contains(out, tlCorner) {
		t.Fatalf("VariantStrong should still render rounded corners, got %q", out)
	}
}

func TestEmptyContentRendersValidBox(t *testing.T) {
	p := New(theme.Default).WithContent("")
	out := p.View()
	rows := strings.Split(out, "\n")
	if len(rows) < 3 {
		t.Fatalf("empty content should still render at least top, blank, bottom, got %d rows", len(rows))
	}
}

func TestNegativePaddingClampsToZero(t *testing.T) {
	p := New(theme.Default).WithContent("x").WithPadding(-3, -5)
	out := p.View()
	// Negative padding clamps to 0. Should be 3 rows total: top, one body, bottom.
	rows := strings.Split(out, "\n")
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows with clamped padding, got %d", len(rows))
	}
}
