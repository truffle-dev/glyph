# Stat Card

> Dashboard metric tile with label, value, trend, and sublabel.

![stat-card preview](../../visuals/out/stat-card.gif)

## Install

```bash
glyph add stat-card
```

This copies `stat-card.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `stat-card` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	statcard "github.com/truffle-dev/glyph/components/stat-card"
)

func main() {
	merged := statcard.New().
		WithLabel("Merged PRs").
		WithValue("88").
		WithDelta("+12").
		WithTrend(statcard.TrendUp).
		WithSublabel("this month")

	revenue := statcard.New().
		WithLabel("Revenue").
		WithValue("$0").
		WithDelta("-100%").
		WithTrend(statcard.TrendDown).
		WithSublabel("since signup").
		WithEmphasis(true)

	row := lipgloss.JoinHorizontal(lipgloss.Top, merged.View(), "  ", revenue.View())
	fmt.Println(row)
}
```

## API surface

Package: `statcard`

**Types**

- `Trend`
- `Model`

**Functions and methods**

- `New`
- `NewWithTheme`
- `WithLabel`
- `WithValue`
- `WithDelta`
- `WithTrend`
- `WithSublabel`
- `WithWidth`
- `WithEmphasis`
- `Width`
- `Height`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Build cards with the immutable `With…` chain and compose a row using
`lipgloss.JoinHorizontal(lipgloss.Top, c1.View(), c2.View())`. `WithWidth(0)`
auto-sizes the tile to its widest row; a positive value fixes outer width
and truncates long content with an ellipsis. `WithEmphasis(true)` swaps in
a rounded border over a stronger surface background — reserve it for one
or two primary tiles per row.

The stat-card has no state and ignores every message in `Update`. Embed
the `Model` in the parent's struct and rebuild it on each tick by pointing
the builders at fresh data.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/stat-card/story](./story) — runnable story binary (`go run -tags glyph_story ./components/stat-card/story/`)
- [registry manifest](./stat-card.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
