package sparklinechart

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestNewDefaults(t *testing.T) {
	c := New(theme.Default)
	if c.Values() != nil {
		t.Errorf("New should start with nil values, got %v", c.Values())
	}
	if _, ok := c.Latest(); ok {
		t.Errorf("New.Latest should report no data")
	}
}

func TestWithValuesCopiesInput(t *testing.T) {
	src := []float64{1, 2, 3}
	c := New(theme.Default).WithValues(src)
	src[0] = 99
	got := c.Values()
	if got[0] != 1 {
		t.Errorf("WithValues should copy input slice; got mutated %v", got)
	}
}

func TestWithWidthClampsToOne(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{1, 2, 3, 4, 5}).WithWidth(0)
	out := c.View()
	if utf8RuneCount(out) != 1 {
		t.Errorf("WithWidth(0) should clamp to 1, got %d runes in %q", utf8RuneCount(out), out)
	}
}

func TestLatestEmptyReturnsFalse(t *testing.T) {
	c := New(theme.Default)
	v, ok := c.Latest()
	if ok || v != 0 {
		t.Errorf("Latest on empty series should be (0, false), got (%v, %v)", v, ok)
	}
}

func TestLatestReturnsLastValue(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{10, 20, 30})
	v, ok := c.Latest()
	if !ok || v != 30 {
		t.Errorf("Latest should be (30, true), got (%v, %v)", v, ok)
	}
}

func TestRangeEmptySeriesIsZero(t *testing.T) {
	lo, hi := New(theme.Default).Range()
	if lo != 0 || hi != 0 {
		t.Errorf("Range on empty series should be (0, 0), got (%v, %v)", lo, hi)
	}
}

func TestRangeAutoScalesFromData(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{5, 1, 9, 3, 7})
	lo, hi := c.Range()
	if lo != 1 || hi != 9 {
		t.Errorf("Range should auto-scale to (1, 9), got (%v, %v)", lo, hi)
	}
}

func TestRangeHonorsManualMin(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{5, 1, 9}).WithMin(0)
	lo, hi := c.Range()
	if lo != 0 || hi != 9 {
		t.Errorf("WithMin should pin lower to 0, got (%v, %v)", lo, hi)
	}
}

func TestRangeHonorsManualMax(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{5, 1, 9}).WithMax(100)
	lo, hi := c.Range()
	if lo != 1 || hi != 100 {
		t.Errorf("WithMax should pin upper to 100, got (%v, %v)", lo, hi)
	}
}

func TestRangeHonorsBothOverrides(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{5, 1, 9}).WithMin(0).WithMax(10)
	lo, hi := c.Range()
	if lo != 0 || hi != 10 {
		t.Errorf("Both overrides should win, got (%v, %v)", lo, hi)
	}
}

func TestViewEmptySeriesRendersBlank(t *testing.T) {
	c := New(theme.Default)
	if got := stripANSI(c.View()); got != "" {
		t.Errorf("empty series should render blank, got %q", got)
	}
}

func TestViewOneCellPerValueWhenUnderWidth(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{1, 2, 3}).WithWidth(20)
	bars := stripANSI(c.View())
	if utf8RuneCount(bars) != 3 {
		t.Errorf("3 values, width 20 should render 3 cells, got %d in %q", utf8RuneCount(bars), bars)
	}
}

func TestViewTakesLastWidthValuesWhenOverflow(t *testing.T) {
	c := New(theme.Default).
		WithValues([]float64{0, 0, 0, 0, 0, 9, 9, 9}).
		WithWidth(3)
	bars := stripANSI(c.View())
	if utf8RuneCount(bars) != 3 {
		t.Errorf("width 3 should render exactly 3 cells, got %d in %q", utf8RuneCount(bars), bars)
	}
	for _, r := range bars {
		if string(r) != "█" {
			t.Errorf("last 3 values are 9s and 9 is max; expected ████, got %q", bars)
			break
		}
	}
}

func TestViewFlatSeriesDoesNotPanic(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{5, 5, 5, 5})
	out := stripANSI(c.View())
	if utf8RuneCount(out) != 4 {
		t.Errorf("flat series should still render 4 cells, got %d in %q", utf8RuneCount(out), out)
	}
}

func TestViewSpansAllEightLevels(t *testing.T) {
	values := []float64{0, 1, 2, 3, 4, 5, 6, 7}
	c := New(theme.Default).WithValues(values).WithWidth(8)
	bars := stripANSI(c.View())
	want := strings.Join(blockGlyphs, "")
	if bars != want {
		t.Errorf("evenly-spaced 0..7 should map to %q, got %q", want, bars)
	}
}

func TestViewSingleValueRendersOneCell(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{42})
	bars := stripANSI(c.View())
	if utf8RuneCount(bars) != 1 {
		t.Errorf("single value should render 1 cell, got %d in %q", utf8RuneCount(bars), bars)
	}
}

func TestViewRendersLabelPrefix(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{1, 2, 3}).WithLabel("cpu")
	out := stripANSI(c.View())
	if !strings.HasPrefix(out, "cpu ") {
		t.Errorf("View should start with label prefix, got %q", out)
	}
}

func TestViewLatestOff(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{1, 2, 3})
	out := stripANSI(c.View())
	if strings.Contains(out, "3") {
		t.Errorf("latest off (default) should omit the numeric suffix, got %q", out)
	}
}

func TestViewLatestOnDefaultFormat(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{1.0, 2.0, 3.0}).WithLatest(true)
	out := stripANSI(c.View())
	if !strings.HasSuffix(out, " 3.0") {
		t.Errorf("latest on should suffix with %q-formatted last value, got %q", "%.1f", out)
	}
}

func TestViewLatestCustomFormatAndSuffix(t *testing.T) {
	c := New(theme.Default).
		WithValues([]float64{40, 50, 60}).
		WithLatest(true).
		WithLatestFormat("%.0f").
		WithLatestSuffix("ms")
	out := stripANSI(c.View())
	if !strings.HasSuffix(out, " 60ms") {
		t.Errorf("custom format+suffix should produce 60ms, got %q", out)
	}
}

func TestWithLatestFormatEmptyFallsBackToDefault(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{2.5}).WithLatest(true).WithLatestFormat("")
	out := stripANSI(c.View())
	if !strings.HasSuffix(out, " 2.5") {
		t.Errorf("empty format should fall back to %%.1f, got %q", out)
	}
}

func TestWithColorDoesNotCorruptRender(t *testing.T) {
	c := New(theme.Default).WithValues([]float64{1, 2, 3}).WithColor(lipgloss.Color("#ff0000"))
	if utf8RuneCount(stripANSI(c.View())) != 3 {
		t.Errorf("WithColor should not affect cell count")
	}
}

func TestWithThemeOverride(t *testing.T) {
	c := New(theme.Default).WithTheme(theme.Default).WithValues([]float64{1, 2})
	if utf8RuneCount(stripANSI(c.View())) != 2 {
		t.Errorf("WithTheme should not affect cell count")
	}
}

func TestManualMinClampsBelowRangeValue(t *testing.T) {
	c := New(theme.Default).
		WithValues([]float64{-5, 0, 5}).
		WithMin(0).
		WithMax(10).
		WithWidth(3)
	bars := stripANSI(c.View())
	if utf8RuneCount(bars) != 3 {
		t.Errorf("manual range should still render 3 cells, got %q", bars)
	}
	if string([]rune(bars)[0]) != blockGlyphs[0] {
		t.Errorf("below-min value should clamp to lowest glyph, got %q", bars)
	}
}

func TestManualMaxClampsAboveRangeValue(t *testing.T) {
	c := New(theme.Default).
		WithValues([]float64{0, 5, 100}).
		WithMin(0).
		WithMax(10).
		WithWidth(3)
	bars := stripANSI(c.View())
	if string([]rune(bars)[2]) != blockGlyphs[len(blockGlyphs)-1] {
		t.Errorf("above-max value should clamp to highest glyph, got %q", bars)
	}
}

// utf8RuneCount counts runes, since unicode bar glyphs span multiple bytes.
func utf8RuneCount(s string) int {
	return len([]rune(s))
}

// stripANSI removes ANSI escape sequences for plain-text assertions.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
