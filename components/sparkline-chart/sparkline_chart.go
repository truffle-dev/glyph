// Package sparklinechart renders a one-line vertical-bar mini-chart over
// a series of float64 values. It maps each value to one of eight unicode
// block heights (▁▂▃▄▅▆▇█), auto-scales the y-range from the data, and
// renders an optional label prefix and latest-value suffix.
//
// Sparkline charts are pure render: the caller updates Values() and the
// chart redraws. There is no internal animation or tick. Pair it with a
// status bar, a metrics panel, or a log header when a one-dimensional
// series is worth tracking at a glance over time.
//
// When the series is longer than the chart width the rightmost width
// values are rendered, so the chart reads as a fixed-width window over
// the most recent data. When the series is shorter than the width the
// chart renders only the available cells, left-aligned, without padding.
package sparklinechart

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// blockGlyphs maps level 0..7 to the eight unicode block-eighth heights.
var blockGlyphs = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// Chart is a single-line sparkline mini-chart.
type Chart struct {
	theme        theme.Theme
	values       []float64
	width        int
	min          float64
	max          float64
	hasManualMin bool
	hasManualMax bool
	color        lipgloss.Color
	hasColor     bool
	label        string
	showLatest   bool
	latestFormat string
	latestSuffix string
}

// New constructs a Chart with the theme's Primary color, a 24-cell width,
// auto-scaled range, and no label.
func New(t theme.Theme) Chart {
	return Chart{
		theme:        t,
		width:        24,
		latestFormat: "%.1f",
	}
}

// WithTheme overrides the theme palette.
func (c Chart) WithTheme(t theme.Theme) Chart { c.theme = t; return c }

// WithValues sets the data series. Values are copied; the caller may
// mutate the original slice after the call.
func (c Chart) WithValues(v []float64) Chart {
	c.values = append([]float64(nil), v...)
	return c
}

// WithWidth sets the cell width of the chart. Values < 1 clamp to 1.
func (c Chart) WithWidth(w int) Chart {
	if w < 1 {
		w = 1
	}
	c.width = w
	return c
}

// WithMin pins the lower bound of the y-range. Without WithMin the chart
// auto-scales from the data minimum.
func (c Chart) WithMin(v float64) Chart {
	c.min = v
	c.hasManualMin = true
	return c
}

// WithMax pins the upper bound of the y-range. Without WithMax the chart
// auto-scales from the data maximum.
func (c Chart) WithMax(v float64) Chart {
	c.max = v
	c.hasManualMax = true
	return c
}

// WithColor overrides the foreground color of the bars. Without this
// override the chart uses the theme's Primary color.
func (c Chart) WithColor(col lipgloss.Color) Chart {
	c.color = col
	c.hasColor = true
	return c
}

// WithLabel sets a text prefix rendered before the bars in the muted
// color. Pass "" to omit.
func (c Chart) WithLabel(s string) Chart { c.label = s; return c }

// WithLatest toggles the latest-value suffix rendered after the bars.
// Default is off.
func (c Chart) WithLatest(on bool) Chart { c.showLatest = on; return c }

// WithLatestFormat overrides the printf format for the latest-value
// suffix. Default is "%.1f".
func (c Chart) WithLatestFormat(f string) Chart {
	if f == "" {
		f = "%.1f"
	}
	c.latestFormat = f
	return c
}

// WithLatestSuffix sets a units string rendered after the latest value,
// e.g. "ms", "MB", "%". Pass "" for none.
func (c Chart) WithLatestSuffix(s string) Chart { c.latestSuffix = s; return c }

// Values returns the data series.
func (c Chart) Values() []float64 { return c.values }

// Latest returns the most recent value and true when the series is
// non-empty; (0, false) otherwise.
func (c Chart) Latest() (float64, bool) {
	if len(c.values) == 0 {
		return 0, false
	}
	return c.values[len(c.values)-1], true
}

// Range returns the effective (min, max) used to scale values into the
// eight block heights. With both manual overrides absent it auto-scales
// from the data; with either present the manual value wins on that
// side.
func (c Chart) Range() (float64, float64) {
	lo, hi := c.dataRange()
	if c.hasManualMin {
		lo = c.min
	}
	if c.hasManualMax {
		hi = c.max
	}
	return lo, hi
}

// View renders the sparkline.
func (c Chart) View() string {
	fg := c.theme.Primary
	if c.hasColor {
		fg = c.color
	}
	barStyle := lipgloss.NewStyle().Foreground(fg)
	labelStyle := lipgloss.NewStyle().Foreground(c.theme.TextMuted)

	bars := c.renderBars()
	parts := []string{}
	if c.label != "" {
		parts = append(parts, labelStyle.Render(c.label))
	}
	if bars != "" {
		parts = append(parts, barStyle.Render(bars))
	}
	if c.showLatest {
		if v, ok := c.Latest(); ok {
			latest := fmt.Sprintf(c.latestFormat, v) + c.latestSuffix
			parts = append(parts, labelStyle.Render(latest))
		}
	}
	return strings.Join(parts, " ")
}

func (c Chart) renderBars() string {
	if len(c.values) == 0 {
		return ""
	}
	view := c.values
	if len(view) > c.width {
		view = view[len(view)-c.width:]
	}
	lo, hi := c.Range()
	span := hi - lo
	var b strings.Builder
	for _, v := range view {
		var level int
		if span <= 0 {
			level = 0
		} else {
			ratio := (v - lo) / span
			if ratio < 0 {
				ratio = 0
			}
			if ratio > 1 {
				ratio = 1
			}
			level = int(ratio*float64(len(blockGlyphs)-1) + 0.5)
		}
		b.WriteString(blockGlyphs[level])
	}
	return b.String()
}

func (c Chart) dataRange() (float64, float64) {
	if len(c.values) == 0 {
		return 0, 0
	}
	lo, hi := c.values[0], c.values[0]
	for _, v := range c.values[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return lo, hi
}
