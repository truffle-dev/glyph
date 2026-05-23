# Select

> Bounded single-choice popover with optional typeahead filter.

![select preview](../../visuals/out/select.gif)

## Install

```bash
glyph add select
```

This copies `select.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `select` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	selectinput "github.com/truffle-dev/glyph/components/select"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	s := selectinput.New(theme.Default).
		WithTitle("Choose a model").
		WithOptions([]selectinput.Option{
			{Label: "Opus 4.7", Hint: "deep thinker"},
			{Label: "Sonnet 4.6", Hint: "fast and balanced"},
			{Label: "Haiku 4.5", Hint: "quick replies"},
		}).
		WithSize(48, 6)
	fmt.Println(s.View())
}
```

## API surface

Package: `selectinput`

**Types**

- `Option`
- `SelectMsg`
- `CancelMsg`
- `Select`

**Functions and methods**

- `New`
- `WithOptions`
- `WithSelected`
- `WithSize`
- `WithTitle`
- `WithPlaceholder`
- `WithFilter`
- `Cursor`
- `Selected`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Build options with []select.Option{{Label, Hint, Value}}. Up/Down walks; Enter commits with selectinput.SelectMsg{Option, Index}; Esc cancels with selectinput.CancelMsg. Turn on WithFilter(true) for substring typeahead.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/select/story](./story) — runnable story binary (`go run -tags glyph_story ./components/select/story/`)
- [registry manifest](./select.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
