# Gauge

> Read-only horizontal bar showing where a numeric value sits inside
> a `[min, max]` range. A filled track on the left, an empty track on
> the right, optional warning and critical color zones, a leading
> label, a trailing units suffix, and a numeric readout. Use it for
> CPU usage, disk capacity, signal strength, queue depth, any reading
> inside a known range.

## Install

```bash
glyph add gauge
```

This copies `gauge.go` (and its test file) into your repo at the path
your `glyph.json` aliases declare, along with the sibling `theme`
dependency. After install, the files are yours: edit them, refactor
them, rename them.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/gauge"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	g := gauge.New(theme.Default).
		WithMin(0).
		WithMax(100).
		WithValue(82).
		WithLabel("CPU").
		WithUnits("%").
		WithThresholds(0.75, 0.9)
	fmt.Println(g.View())
	// CPU [████████████████░░░░] 82%
}
```

## API surface

Package: `gauge`

**Types**

- `Gauge`

**Functions and methods**

- `New`
- `WithMin`
- `WithMax`
- `WithValue`
- `WithWidth`
- `WithLabel`
- `WithUnits`
- `WithThresholds`
- `WithReadout`
- `WithFillRune`
- `WithEmptyRune`
- `Value`
- `Percent`
- `View`

## Thresholds

`WithThresholds(warnFrac, critFrac)` reads two fractions in `[0, 1]`.
The bar color picks one of three theme colors based on the current
`Percent()`:

| Range                            | Theme color    |
|----------------------------------|----------------|
| `0 <= p < warnFrac`              | `Success`      |
| `warnFrac <= p < critFrac`       | `Warning`      |
| `critFrac <= p <= 1`             | `Error`        |

Passing both fractions as zero disables tiered coloring and uses
`Primary` instead, which reads as neutral information rather than a
status signal.

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

`Percent()` clamps to `[0, 1]` so callers can pass raw readings
without pre-validating; the bar never overshoots or underflows.
`Value()` returns the raw input untouched, which the readout uses so
out-of-range readings stay visible to the operator instead of silently
clamping.

A gauge is stateless. There is no `Update`; the parent recomputes
`Value` each tick and re-renders. Pair with `time.Tick` from Bubble
Tea when the underlying reading polls (system metrics, network probes,
queue depth) and with a static value when the reading is one-shot
(disk free at startup, build status at end).

When `max == min` (a degenerate zero-span range) `Percent` returns
`0` and the bar renders empty, rather than dividing by zero. The same
holds for inverted ranges where `max < min`.

`WithFillRune` and `WithEmptyRune` swap the default `█` and `░` for
a different aesthetic. Common alternatives are `▰`/`▱`, `■`/`□`, or
`=`/`-` for ASCII-only contexts. Passing an empty string is rejected
silently so the bar never collapses.

## See also

- [components/progress-bar](../progress-bar) — task completion as `[0, 1]`, single fill color, no thresholds
- [components/range-slider](../range-slider) — interactive value picker over a range
- [components/stat-card](../stat-card) — single big number plus trend, no range
- [registry manifest](./gauge.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
