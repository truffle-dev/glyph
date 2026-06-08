// Package gauge renders a horizontal bar showing where a numeric
// value sits inside a [min, max] range, with optional threshold-zone
// color transitions, a label, a units suffix, and a numeric readout.
// Use it for read-only indicators where the question is "where is the
// reading right now in this range": CPU usage, disk capacity, voice
// volume, signal strength, queue depth.
//
// Gauges are stateless. View renders a fixed-width bar plus a readout.
// There is no interactive Update; pair with a parent model that
// recomputes Value on every tick.
//
// A gauge differs from progress-bar (which models task progress as a
// 0-1 ratio with a single fill color) and from range-slider (which is
// interactive). Reach for gauge when the value is a measurement in a
// range, not a percentage of completion.
package gauge

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Gauge is a read-only horizontal range indicator.
type Gauge struct {
	theme   theme.Theme
	min     float64
	max     float64
	value   float64
	width   int
	label   string
	units   string
	showVal bool
	warnAt  float64
	critAt  float64
	useTier bool
	fillCh  string
	emptyCh string
}

// New constructs a Gauge with min=0, max=100, value=0, width=20 and
// the readout enabled.
func New(t theme.Theme) Gauge {
	return Gauge{
		theme:   t,
		min:     0,
		max:     100,
		value:   0,
		width:   20,
		showVal: true,
		fillCh:  "█",
		emptyCh: "░",
	}
}

// WithMin sets the lower bound of the range.
func (g Gauge) WithMin(min float64) Gauge { g.min = min; return g }

// WithMax sets the upper bound of the range.
func (g Gauge) WithMax(max float64) Gauge { g.max = max; return g }

// WithValue sets the current value. Clamping happens at render time so
// the caller can pass a raw reading.
func (g Gauge) WithValue(v float64) Gauge { g.value = v; return g }

// WithWidth sets the bar width in cells (excluding label/readout).
// Minimum 1.
func (g Gauge) WithWidth(w int) Gauge {
	if w < 1 {
		w = 1
	}
	g.width = w
	return g
}

// WithLabel sets the leading label rendered before the bar.
func (g Gauge) WithLabel(s string) Gauge { g.label = s; return g }

// WithUnits sets the trailing units suffix on the readout (e.g. "%",
// "MB", "ms").
func (g Gauge) WithUnits(s string) Gauge { g.units = s; return g }

// WithThresholds defines warning and critical zones as a fraction of
// the span [min, max]. Both arguments are 0-1 ratios. When the value
// crosses warnFrac, the fill switches to the warning color; when it
// crosses critFrac, it switches to the error color. Passing both zero
// disables tiered coloring and uses the theme Primary color.
func (g Gauge) WithThresholds(warnFrac, critFrac float64) Gauge {
	if warnFrac < 0 {
		warnFrac = 0
	}
	if critFrac < 0 {
		critFrac = 0
	}
	if warnFrac > 1 {
		warnFrac = 1
	}
	if critFrac > 1 {
		critFrac = 1
	}
	g.warnAt = warnFrac
	g.critAt = critFrac
	g.useTier = warnFrac > 0 || critFrac > 0
	return g
}

// WithReadout toggles the trailing numeric readout.
func (g Gauge) WithReadout(show bool) Gauge { g.showVal = show; return g }

// WithFillRune replaces the default filled-cell glyph.
func (g Gauge) WithFillRune(r string) Gauge {
	if r != "" {
		g.fillCh = r
	}
	return g
}

// WithEmptyRune replaces the default empty-cell glyph.
func (g Gauge) WithEmptyRune(r string) Gauge {
	if r != "" {
		g.emptyCh = r
	}
	return g
}

// Percent returns the value's position in the [min, max] range as a
// 0-1 ratio. Values outside the range clamp.
func (g Gauge) Percent() float64 {
	span := g.max - g.min
	if span <= 0 {
		return 0
	}
	frac := (g.value - g.min) / span
	if frac < 0 {
		return 0
	}
	if frac > 1 {
		return 1
	}
	return frac
}

// Value returns the current value as set, without clamping. The
// readout uses this so the caller can see the raw reading even when
// it falls outside [min, max].
func (g Gauge) Value() float64 { return g.value }

// View renders the gauge.
func (g Gauge) View() string {
	p := g.Percent()
	filled := int(float64(g.width) * p)
	if filled > g.width {
		filled = g.width
	}
	if filled < 0 {
		filled = 0
	}
	empty := g.width - filled

	color := g.theme.Primary
	if g.useTier {
		switch {
		case g.critAt > 0 && p >= g.critAt:
			color = g.theme.Error
		case g.warnAt > 0 && p >= g.warnAt:
			color = g.theme.Warning
		default:
			color = g.theme.Success
		}
	}

	fillStyle := lipgloss.NewStyle().Foreground(color)
	emptyStyle := lipgloss.NewStyle().Foreground(g.theme.TextMuted)

	var b strings.Builder
	if g.label != "" {
		b.WriteString(g.label)
		b.WriteString(" ")
	}
	b.WriteString("[")
	b.WriteString(fillStyle.Render(strings.Repeat(g.fillCh, filled)))
	b.WriteString(emptyStyle.Render(strings.Repeat(g.emptyCh, empty)))
	b.WriteString("]")
	if g.showVal {
		readout := fmt.Sprintf(" %g", g.value)
		if g.units != "" {
			readout += g.units
		}
		b.WriteString(readout)
	}
	return b.String()
}
