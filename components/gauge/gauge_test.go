package gauge

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestPercentBasic(t *testing.T) {
	g := New(theme.Default).WithMin(0).WithMax(100).WithValue(47)
	got := g.Percent()
	if got < 0.46 || got > 0.48 {
		t.Fatalf("Percent(0..100, 47) = %v, want ~0.47", got)
	}
}

func TestPercentClampsHigh(t *testing.T) {
	g := New(theme.Default).WithMin(0).WithMax(100).WithValue(150)
	if got := g.Percent(); got != 1 {
		t.Fatalf("Percent above max should clamp to 1, got %v", got)
	}
}

func TestPercentClampsLow(t *testing.T) {
	g := New(theme.Default).WithMin(0).WithMax(100).WithValue(-5)
	if got := g.Percent(); got != 0 {
		t.Fatalf("Percent below min should clamp to 0, got %v", got)
	}
}

func TestPercentZeroSpan(t *testing.T) {
	g := New(theme.Default).WithMin(50).WithMax(50).WithValue(50)
	if got := g.Percent(); got != 0 {
		t.Fatalf("Zero-span gauge returns 0, got %v", got)
	}
}

func TestPercentNegativeSpan(t *testing.T) {
	g := New(theme.Default).WithMin(100).WithMax(0).WithValue(50)
	if got := g.Percent(); got != 0 {
		t.Fatalf("Negative-span gauge returns 0, got %v", got)
	}
}

func TestPercentShiftedRange(t *testing.T) {
	g := New(theme.Default).WithMin(-50).WithMax(50).WithValue(0)
	got := g.Percent()
	if got < 0.49 || got > 0.51 {
		t.Fatalf("Percent(-50..50, 0) = %v, want ~0.5", got)
	}
}

func TestWidthClampsToOne(t *testing.T) {
	g := New(theme.Default).WithWidth(0)
	if g.width != 1 {
		t.Fatalf("Width 0 should clamp to 1, got %d", g.width)
	}
	g = New(theme.Default).WithWidth(-3)
	if g.width != 1 {
		t.Fatalf("Negative width should clamp to 1, got %d", g.width)
	}
}

func TestViewIncludesLabel(t *testing.T) {
	g := New(theme.Default).WithLabel("CPU").WithValue(50)
	out := g.View()
	if !strings.HasPrefix(out, "CPU ") {
		t.Fatalf("Label missing from output: %q", out)
	}
}

func TestViewIncludesUnits(t *testing.T) {
	g := New(theme.Default).WithValue(75).WithUnits("%")
	out := g.View()
	if !strings.Contains(out, "75%") {
		t.Fatalf("Units missing from readout: %q", out)
	}
}

func TestViewSuppressesReadout(t *testing.T) {
	g := New(theme.Default).WithValue(33).WithReadout(false)
	out := g.View()
	if strings.Contains(out, "33") {
		t.Fatalf("Readout should be suppressed: %q", out)
	}
}

func TestViewFillCount(t *testing.T) {
	g := New(theme.Default).WithMin(0).WithMax(10).WithValue(5).WithWidth(10).WithReadout(false).WithLabel("")
	out := g.View()
	fills := strings.Count(out, "█")
	empties := strings.Count(out, "░")
	if fills+empties != 10 {
		t.Fatalf("Bar width != 10: fills=%d empties=%d total=%d", fills, empties, fills+empties)
	}
	if fills != 5 {
		t.Fatalf("Expected 5 filled cells at 50%%, got %d", fills)
	}
}

func TestViewFullBar(t *testing.T) {
	g := New(theme.Default).WithMin(0).WithMax(10).WithValue(10).WithWidth(8).WithReadout(false).WithLabel("")
	out := g.View()
	if got := strings.Count(out, "█"); got != 8 {
		t.Fatalf("Expected 8 filled cells at 100%%, got %d", got)
	}
}

func TestViewEmptyBar(t *testing.T) {
	g := New(theme.Default).WithMin(0).WithMax(10).WithValue(0).WithWidth(8).WithReadout(false).WithLabel("")
	out := g.View()
	if got := strings.Count(out, "░"); got != 8 {
		t.Fatalf("Expected 8 empty cells at 0%%, got %d", got)
	}
}

func TestThresholdsClampNegative(t *testing.T) {
	g := New(theme.Default).WithThresholds(-0.5, -1)
	if g.warnAt != 0 || g.critAt != 0 {
		t.Fatalf("Negative thresholds should clamp to 0, got warn=%v crit=%v", g.warnAt, g.critAt)
	}
	if g.useTier {
		t.Fatalf("Both thresholds zero should leave useTier false")
	}
}

func TestThresholdsClampHigh(t *testing.T) {
	g := New(theme.Default).WithThresholds(2, 5)
	if g.warnAt != 1 || g.critAt != 1 {
		t.Fatalf("Thresholds above 1 should clamp to 1, got warn=%v crit=%v", g.warnAt, g.critAt)
	}
}

func TestThresholdsEnableTier(t *testing.T) {
	g := New(theme.Default).WithThresholds(0.75, 0.9)
	if !g.useTier {
		t.Fatalf("Non-zero thresholds should enable tiered coloring")
	}
}

func TestFillRuneOverride(t *testing.T) {
	g := New(theme.Default).WithFillRune("=").WithEmptyRune("-").WithValue(50).WithWidth(10).WithLabel("").WithReadout(false)
	out := g.View()
	if !strings.Contains(out, "=") || !strings.Contains(out, "-") {
		t.Fatalf("Custom runes not used in output: %q", out)
	}
}

func TestEmptyRuneOverrideRejectsEmpty(t *testing.T) {
	g := New(theme.Default).WithFillRune("").WithEmptyRune("")
	if g.fillCh == "" || g.emptyCh == "" {
		t.Fatalf("Empty rune override should be ignored: fill=%q empty=%q", g.fillCh, g.emptyCh)
	}
}

func TestValueReturnsRaw(t *testing.T) {
	g := New(theme.Default).WithMin(0).WithMax(10).WithValue(42)
	if got := g.Value(); got != 42 {
		t.Fatalf("Value should return raw input, got %v", got)
	}
}
