# Sparkline Chart

> Single-line vertical-bar mini-chart over a one-dimensional series.
> Map a stream of float64 values to one of eight unicode block heights,
> with auto-scaling, optional pinning, and a latest-value readout. Pair
> it with a status bar, a metrics panel, or a log header when a number
> is worth tracking at a glance over time.

## Install

```bash
glyph add sparkline-chart
```

This copies `sparkline_chart.go` (and its test file) into your repo at
the path your `glyph.json` aliases declare, along with the sibling
`theme` dependency. After install, the files are yours: edit them,
refactor them, rename them.

## Hello, world

```go
package main

import (
	"fmt"

	sparklinechart "github.com/truffle-dev/glyph/components/sparkline-chart"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	cpu := []float64{12, 18, 22, 19, 24, 31, 28, 33, 41, 38, 45, 52, 47}
	c := sparklinechart.New(theme.Default).
		WithValues(cpu).
		WithWidth(13).
		WithLabel("cpu").
		WithLatest(true).
		WithLatestFormat("%.0f").
		WithLatestSuffix("%")
	fmt.Println(c.View())
	// cpu ▁▂▃▂▃▅▄▅▇▆█▇▇ 47%
}
```

## API surface

Package: `sparklinechart`

**Types**

- `Chart`

**Functions and methods**

- `New`
- `WithTheme`
- `WithValues`
- `WithWidth`
- `WithMin`
- `WithMax`
- `WithColor`
- `WithLabel`
- `WithLatest`
- `WithLatestFormat`
- `WithLatestSuffix`
- `Values`
- `Latest`
- `Range`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

The chart is a pure-render value type, not a `tea.Model`: there is no
`Init`, no `Update`, no keyboard. The caller redraws by constructing a
new chart with the new values and calling `View()` again. That matches
the way streaming metrics flow into a status bar — the caller already
has its own update loop and doesn't want the chart to fight for the
keyboard.

`WithValues` copies the input slice; the caller may mutate the
original after the call without affecting the chart's render. When the
series is longer than `WithWidth` the rightmost width values are
rendered, so the chart reads as a fixed-width window over the most
recent data. When the series is shorter the chart renders only the
available cells, left-aligned, without padding the rest of the width.

Auto-scaling derives the y-range from the data with the data minimum
mapping to ▁ and the data maximum mapping to █. Pin either edge with
`WithMin(v)` or `WithMax(v)` to lock the chart's vertical scale: a CPU
percentage probably wants `WithMin(0).WithMax(100)` so the bar heights
stay legible across quiet and busy moments. Flat-line series (all
values equal) render at the bottom of the range without dividing by
zero.

`WithLatest(true)` appends the most recent value after the bars in the
muted color. Combine with `WithLatestFormat("%.0f")` or any printf
format and `WithLatestSuffix("ms")` to add units. `WithColor` overrides
the foreground tint of the bars when status colors are more legible
than the theme's Primary (think Success for "healthy", Warning for
"warming up", Error for "saturated").

## See also

- [components/progress-bar](../progress-bar) — determinate single-value bar with a percentage readout instead of a series
- [components/stat-card](../stat-card) — labeled single-value tile that pairs well with a sparkline below the number
- [registry manifest](./sparkline-chart.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
