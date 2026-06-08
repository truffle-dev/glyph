# Range Slider

> Single-line horizontal slider over a continuous numeric range. A
> filled track on the left, an unfilled track on the right, a thumb
> at the current position, and an optional value readout on the
> trailing edge. Use it for volume, opacity, brightness, time
> offsets, threshold tuning, anywhere a number needs a position more
> than a typed value.

## Install

```bash
glyph add range-slider
```

This copies `range-slider.go` (and its test file) into your repo at
the path your `glyph.json` aliases declare, along with the sibling
`theme` dependency. After install, the files are yours: edit them,
refactor them, rename them.

## Hello, world

```go
package main

import (
	"fmt"

	rangeslider "github.com/truffle-dev/glyph/components/range-slider"
)

func main() {
	m := rangeslider.New().
		WithMin(0).
		WithMax(100).
		WithStep(1).
		WithValue(75).
		WithUnits("%").
		WithLabel("Volume: ")
	fmt.Println(m.View())
	// Volume: ━━━━━━━━━━━━━━●─────  75%
}
```

## API surface

Package: `rangeslider`

**Types**

- `Model`
- `ValueChangedMsg`

**Functions and methods**

- `New`
- `WithTheme`
- `WithMin`
- `WithMax`
- `WithStep`
- `WithValue`
- `WithWidth`
- `WithPrecision`
- `WithFormatter`
- `WithDisabled`
- `WithLabel`
- `WithUnits`
- `WithShowValue`
- `Value`
- `Min`
- `Max`
- `Step`
- `Percent`
- `Disabled`
- `AtMin`
- `AtMax`
- `Init`
- `Update`
- `View`

## Keys

| Key             | Action                                          |
|-----------------|-------------------------------------------------|
| `right` / `l`   | Step the value up by one `step`.                |
| `left` / `h`    | Step the value down by one `step`.              |
| `end` / `G`     | Jump to `max`.                                  |
| `home` / `g`    | Jump to `min`.                                  |
| `pgdown` / `J`  | Jump up by 10×`step`.                           |
| `pgup` / `K`    | Jump down by 10×`step`.                         |

Motion clamps at both ends; nothing wraps. A disabled slider ignores
every key.

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

`Value()` is the live number the parent reads back. `Percent()`
returns `(value - min) / (max - min)`, which is what most callers
actually want when wiring the slider into a sibling renderer (a
preview pane, a derived bar, a numeric formatter). `ValueChangedMsg`
fires only on actual changes, so a `Update` at the edge returns a
nil command and no message reaches the consumer.

Fractional ranges work as-is: `WithMin(0).WithMax(1).WithStep(0.01)`
with `WithPrecision(2)` renders `0.42` cleanly and steps in 0.01
increments. For non-decimal display, pass `WithFormatter(func(v
float64) string { ... })` and the slider stops calling
`fmt.Sprintf`. `WithUnits("%")` is the common case and avoids a
custom formatter.

The track width defaults to 20 cells; clamp it to 3 minimum so the
thumb always has room. `WithShowValue(false)` hides the trailing
readout when the parent is rendering its own value display.
`WithDisabled(true)` renders the whole bar in the muted color and
silently drops every key event.

## See also

- [components/progress-bar](../progress-bar) — a read-only completion bar, where this slider is interactive
- [components/pagination-bar](../pagination-bar) — discrete page indicator, where this slider is continuous
- [registry manifest](./range-slider.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
