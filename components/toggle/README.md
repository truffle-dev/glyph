# Toggle

> Single-line boolean switch with keyboard control. A knob that sits
> on the right when on and the left when off, recolored to the theme
> Success accent when on and the muted color when off, with an
> optional leading label and trailing caption. Use it for the on/off
> settings a `range-slider` is too wide for: word wrap, dark mode,
> autosave, a feature flag, anywhere a value is just yes or no.

## Install

```bash
glyph add toggle
```

This copies `toggle.go` (and its test file) into your repo at the
path your `glyph.json` aliases declare, along with the sibling
`theme` dependency. After install, the files are yours: edit them,
refactor them, rename them.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/toggle"
)

func main() {
	m := toggle.New().
		WithOn(true).
		WithLabel("Wrap: ")
	fmt.Println(m.View())
	// Wrap: ──●  On
}
```

## API surface

Package: `toggle`

**Types**

- `Model`
- `ToggleChangedMsg`

**Functions and methods**

- `New`
- `WithTheme`
- `WithOn`
- `WithDisabled`
- `WithLabel`
- `WithOnLabel`
- `WithOffLabel`
- `WithShowText`
- `On`
- `Disabled`
- `Init`
- `Update`
- `View`

## Keys

| Key                 | Action                          |
|---------------------|---------------------------------|
| `space` / `enter`   | Flip the current state.         |
| `right` / `l` / `y` | Force on.                       |
| `left` / `h` / `n`  | Force off.                      |

A key that lands on the value already held emits no message. A
disabled toggle ignores every key.

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

`On()` is the live boolean the parent reads back. `ToggleChangedMsg`
fires only on actual changes, so an `Update` that re-asserts the held
state (a `right` while already on) returns a nil command and no
message reaches the consumer. This lets a parent wire a side effect
to the message without guarding against no-op keystrokes.

`WithOnLabel` and `WithOffLabel` rename the trailing captions; the
defaults are `On` and `Off`. `WithShowText(false)` drops the caption
entirely when the parent renders its own label. `WithLabel` adds a
muted prefix like `Wrap: ` so the switch reads inline with a form.
`WithDisabled(true)` renders the knob and track in the muted color
and silently drops every key event.

The component is stateful but tiny: it owns one bool and the key
bindings. For a non-interactive yes/no readout, a `stat-card` or a
plain styled string is lighter; reach for `toggle` when the value
needs to change under the keyboard.

## See also

- [components/range-slider](../range-slider) — the numeric sibling, where this toggle is boolean
- [components/select](../select) — one-of-N choice, where this toggle is one-of-two
- [registry manifest](./toggle.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
