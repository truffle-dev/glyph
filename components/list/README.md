# List

> Vertical selectable list with cursor highlight, optional hints, disabled items, and internal scrolling.

![list preview](../../visuals/out/list.gif)

## Install

```bash
glyph add list
```

This copies `list.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `list` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/list"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	l := list.New(theme.Default).
		WithHeight(8).
		WithItems([]list.Item{
			{Label: "Inbox", Hint: "12 unread"},
			{Label: "Drafts"},
		})
	fmt.Println(l.View())
}
```

## API surface

Package: `list`

**Types**

- `Item`
- `List`

**Functions and methods**

- `New`
- `WithItems`
- `WithHeight`
- `WithWidth`
- `WithCursor`
- `Update`
- `View`
- `Selected`
- `Cursor`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Build items with []list.Item{{Label, Hint, Disabled, Value}}. Up/down or k/j walk; Home/g and End/G jump. Read Selected() to drive the detail panel your parent model renders.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/list/story](./story) — runnable story binary (`go run -tags glyph_story ./components/list/story/`)
- [registry manifest](./list.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
