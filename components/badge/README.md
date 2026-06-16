# Badge

> Compact status pill. The small colored label that marks a row
> `LIVE`, a build `PASS`, a release `BETA`, or a count `3 NEW`. Six
> semantic variants in a filled or outline appearance, tinted from the
> theme so it drops onto any surface. Use it where a status needs
> weight but not a whole line: table cells, list rows, status bars,
> the corner of a `stat-card`.

## Install

```bash
glyph add badge
```

This copies `badge.go` (and its test file) into your repo at the
path your `glyph.json` aliases declare, along with the sibling
`theme` dependency. After install, the files are yours: edit them,
refactor them, rename them.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/badge"
)

func main() {
	fmt.Println(badge.New("LIVE").Success().Render())
	fmt.Println(badge.New("beta").Warning().Outline().Uppercase().Render())
	fmt.Println(badge.New("v0.49").Render()) // neutral chip
}
```

## API surface

Package: `badge`

**Types**

- `Badge`
- `Variant` (`Neutral`, `Primary`, `Success`, `Warning`, `Error`, `Info`)

**Functions and methods**

- `New`
- `WithTheme`
- `WithVariant`
- `Neutral` / `Primary` / `Success` / `Warning` / `Error` / `Info`
- `Outline` / `Filled`
- `Uppercase`
- `Label`
- `Render`

## Variants

| Variant   | Meaning                                  |
|-----------|------------------------------------------|
| `Neutral` | Quiet label, no status color (the zero). |
| `Primary` | The dominant accent state.               |
| `Success` | Healthy, passing, completed.             |
| `Warning` | Degraded, attention needed.              |
| `Error`   | Failed or blocking.                      |
| `Info`    | Neutral-informational.                   |

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

A `Badge` is an immutable value, not a `tea.Model`: there is no
`Update` and no message. Every option returns a new `Badge`, so a
chain like `New("x").Success().Outline()` never mutates the receiver
and a base badge can be reused as a template. Call `Render` from
inside your own `View` and place the string wherever a label belongs.

Filled is the default appearance: the variant color fills the
background and the label sits on it in the theme's `TextInverse`.
`Outline()` switches to a rounded border in the variant color with no
fill, a lighter touch for busy surfaces; `Filled()` switches back.
`Neutral` is special-cased to render as a quiet `Surface` chip with
normal text rather than inverse-on-accent, so a version tag or a count
does not shout.

`Uppercase()` folds the label at render time, the common shape for
status pills (`live` to `LIVE`); `Label()` still returns the original
casing. An empty label renders as `""`, so a conditional badge can be
concatenated into a row without a call-site guard.

For a yes/no control the user changes under the keyboard, reach for
`toggle`; `badge` is a read-only label the parent owns.

## See also

- [components/stat-card](../stat-card) — the number this badge often sits beside
- [components/status-bar](../status-bar) — the row this badge often sits inside
- [components/toggle](../toggle) — the interactive sibling, where this badge is read-only
- [registry manifest](./badge.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
