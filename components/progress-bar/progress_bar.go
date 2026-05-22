// Package progressbar renders a determinate progress indicator with an
// optional label, percentage readout, and color-tunable fill.
//
// Progress bars are pure render: the caller updates Percent() and
// the bar redraws. There is no internal animation or tick; pair with
// the spinner component if you need an indeterminate indicator.
package progressbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Bar is a determinate progress indicator.
type Bar struct {
	theme     theme.Theme
	percent   float64
	width     int
	label     string
	showPct   bool
	fillColor lipgloss.Color
	fillRune  string
	emptyRune string
}

// New constructs a Bar using the theme's Primary color for the fill.
func New(t theme.Theme) Bar {
	return Bar{
		theme:     t,
		width:     30,
		showPct:   true,
		fillColor: t.Primary,
		fillRune:  "█",
		emptyRune: "░",
	}
}

// WithPercent sets the fill ratio. Values are clamped into [0, 1].
func (b Bar) WithPercent(p float64) Bar {
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	b.percent = p
	return b
}

// WithWidth sets the bar width in cells (excluding label/percentage).
// Minimum 1.
func (b Bar) WithWidth(w int) Bar {
	if w < 1 {
		w = 1
	}
	b.width = w
	return b
}

// WithLabel sets text rendered before the bar. Pass "" to omit.
func (b Bar) WithLabel(label string) Bar { b.label = label; return b }

// WithPercent display toggle. Default is on.
func (b Bar) WithPercentDisplay(show bool) Bar { b.showPct = show; return b }

// WithFillColor overrides the fill color. Useful for status bars where
// you want a Success/Warning/Error-tinted progress strip.
func (b Bar) WithFillColor(c lipgloss.Color) Bar { b.fillColor = c; return b }

// WithRunes overrides the fill and empty glyphs. Defaults to block ░ and █.
func (b Bar) WithRunes(fill, empty string) Bar {
	if fill != "" {
		b.fillRune = fill
	}
	if empty != "" {
		b.emptyRune = empty
	}
	return b
}

// Percent returns the current fill ratio.
func (b Bar) Percent() float64 { return b.percent }

// View renders the bar.
func (b Bar) View() string {
	filled := int(float64(b.width)*b.percent + 0.5)
	if filled > b.width {
		filled = b.width
	}
	fill := lipgloss.NewStyle().Foreground(b.fillColor).Render(strings.Repeat(b.fillRune, filled))
	empty := lipgloss.NewStyle().Foreground(b.theme.Border).Render(strings.Repeat(b.emptyRune, b.width-filled))
	bar := fill + empty

	parts := []string{}
	if b.label != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(b.theme.Text).Render(b.label))
	}
	parts = append(parts, bar)
	if b.showPct {
		pct := fmt.Sprintf("%3d%%", int(b.percent*100+0.5))
		parts = append(parts, lipgloss.NewStyle().Foreground(b.theme.TextMuted).Render(pct))
	}
	return strings.Join(parts, " ")
}
