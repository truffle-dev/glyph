package statcard

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMain forces TrueColor so lipgloss emits ANSI escapes deterministically
// regardless of whether the test process is attached to a TTY.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func TestRendersLabelAndValue(t *testing.T) {
	out := New().
		WithLabel("Open issues").
		WithValue("12").
		View()
	// Label is uppercased on the rendered line; the source string "Open issues"
	// won't appear, but the upper form will.
	if !strings.Contains(out, "OPEN ISSUES") {
		t.Errorf("expected uppercased label in output, got %q", out)
	}
	if !strings.Contains(out, "12") {
		t.Errorf("expected value in output, got %q", out)
	}
}

func TestTrendUpShowsGlyphAndDelta(t *testing.T) {
	out := New().
		WithLabel("PRs").
		WithValue("88").
		WithDelta("+12").
		WithTrend(TrendUp).
		View()
	if !strings.Contains(out, glyphUp) {
		t.Errorf("expected up-glyph %q in output, got %q", glyphUp, out)
	}
	if !strings.Contains(out, "+12") {
		t.Errorf("expected delta text in output, got %q", out)
	}
}

func TestTrendDownShowsDownGlyph(t *testing.T) {
	out := New().
		WithLabel("Revenue").
		WithValue("$0").
		WithDelta("-100%").
		WithTrend(TrendDown).
		View()
	if !strings.Contains(out, glyphDown) {
		t.Errorf("expected down-glyph %q in output, got %q", glyphDown, out)
	}
	if !strings.Contains(out, "-100%") {
		t.Errorf("expected delta in output, got %q", out)
	}
}

func TestTrendNeutralShowsEmDash(t *testing.T) {
	out := New().
		WithLabel("In flight").
		WithValue("39").
		WithDelta("0").
		WithTrend(TrendNeutral).
		View()
	if !strings.Contains(out, glyphNeutral) {
		t.Errorf("expected neutral-glyph %q in output, got %q", glyphNeutral, out)
	}
}

func TestSublabelRendersInItalic(t *testing.T) {
	out := New().
		WithLabel("Followers").
		WithValue("12").
		WithSublabel("this week").
		View()
	if !strings.Contains(out, "this week") {
		t.Errorf("expected sublabel text in output, got %q", out)
	}
	// Italic in lipgloss emits the ANSI SGR 3 sequence ("\x1b[3m" or
	// "\x1b[…;3m"). Look for an italic SGR somewhere in the line.
	if !containsItalicSGR(out) {
		t.Errorf("expected italic ANSI styling on the trend row, got %q", out)
	}
}

func TestEmphasisProducesDifferentBytes(t *testing.T) {
	base := New().WithLabel("Revenue").WithValue("$0").View()
	emph := New().WithLabel("Revenue").WithValue("$0").WithEmphasis(true).View()
	if base == emph {
		t.Errorf("expected emphasis to change rendered bytes, but both were equal:\n%q", base)
	}
}

func TestFixedWidthClampsOuter(t *testing.T) {
	out := New().
		WithLabel("Latency p99").
		WithValue("142ms").
		WithSublabel("last 5 min").
		WithWidth(20).
		View()
	for _, row := range strings.Split(out, "\n") {
		if got := lipgloss.Width(row); got != 20 {
			t.Errorf("row %q has width %d, expected 20", row, got)
		}
	}
}

func TestAutoWidthExpandsForLongerLabel(t *testing.T) {
	short := New().WithLabel("PRs").WithValue("1").View()
	long := New().WithLabel("Outstanding pull requests this fortnight").WithValue("1").View()
	sw := outerWidth(short)
	lw := outerWidth(long)
	if !(lw > sw) {
		t.Errorf("expected long-label card (%d) to be wider than short-label card (%d)", lw, sw)
	}
}

func TestWindowSizeMsgIsNoop(t *testing.T) {
	before := New().WithLabel("PRs").WithValue("1").WithWidth(24)
	after, cmd := before.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	if cmd != nil {
		t.Errorf("expected nil cmd from WindowSizeMsg, got %v", cmd)
	}
	if before.View() != after.View() {
		t.Errorf("expected View() unchanged after WindowSizeMsg; before != after")
	}
	if before.Width() != after.Width() {
		t.Errorf("expected Width() unchanged after WindowSizeMsg; got %d -> %d", before.Width(), after.Width())
	}
}

func TestWidthAndHeightQueries(t *testing.T) {
	// Fixed-width card: Width() should equal the configured outer width.
	m := New().
		WithLabel("PRs").
		WithValue("1").
		WithDelta("+1").
		WithTrend(TrendUp).
		WithSublabel("today").
		WithWidth(24)
	if got := m.Width(); got != 24 {
		t.Errorf("Width() = %d, want 24", got)
	}
	// Height: 2 borders + 2 padding + 1 label + 1 value + 1 trend = 7
	if got := m.Height(); got != 7 {
		t.Errorf("Height() = %d, want 7", got)
	}
	// No trend row → Height should be 6.
	m2 := New().WithLabel("PRs").WithValue("1").WithWidth(24)
	if got := m2.Height(); got != 6 {
		t.Errorf("Height() without trend = %d, want 6", got)
	}
	// Auto-width: Width() returns autoOuterWidth — exactly content + padding + borders.
	m3 := New().WithLabel("AB").WithValue("XY")
	// Inner is max(2, 2) = 2; outer = 2 + 2 padX + 2 borders = 6.
	if got := m3.Width(); got != 6 {
		t.Errorf("auto Width() = %d, want 6", got)
	}
}

func TestRowsCountMatchesHeight(t *testing.T) {
	m := New().
		WithLabel("Followers").
		WithValue("12").
		WithDelta("+3").
		WithTrend(TrendUp).
		WithSublabel("this week").
		WithWidth(28)
	out := m.View()
	rows := strings.Split(out, "\n")
	if len(rows) != m.Height() {
		t.Errorf("rendered rows = %d, Height() = %d", len(rows), m.Height())
	}
}

func TestInitReturnsNilCmd(t *testing.T) {
	if cmd := New().Init(); cmd != nil {
		t.Errorf("expected Init() == nil, got %v", cmd)
	}
}

// outerWidth measures the visible cell width of the widest row.
func outerWidth(s string) int {
	max := 0
	for _, row := range strings.Split(s, "\n") {
		if w := lipgloss.Width(row); w > max {
			max = w
		}
	}
	return max
}

// containsItalicSGR looks for the italic SGR (3) anywhere in an
// ANSI-styled string. Italic appears either standalone ("\x1b[3m") or
// as part of a combined SGR sequence ("\x1b[3;38;2;…m" / "…;3;…m" /
// "…;3m"). We walk every escape sequence and inspect its parameter list.
func containsItalicSGR(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			continue
		}
		// Find end of SGR sequence at 'm'.
		j := i + 1
		if j >= len(s) || s[j] != '[' {
			continue
		}
		j++
		start := j
		for j < len(s) && s[j] != 'm' {
			j++
		}
		if j >= len(s) {
			break
		}
		params := s[start:j]
		for _, p := range strings.Split(params, ";") {
			if p == "3" {
				return true
			}
		}
		i = j
	}
	return false
}
