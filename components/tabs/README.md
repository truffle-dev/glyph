# Tabs

> Horizontal row of labeled tabs with one active label.

![tabs preview](../../visuals/out/tabs.gif)

## Install

```bash
glyph add tabs
```

This copies `tabs.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `tabs` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/tabs"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	t := tabs.New(theme.Default).
		WithTabs([]string{"chat", "logs", "diff"}).
		WithActive(0)
	fmt.Println(t.View())
}
```

## API surface

Package: `tabs`

**Types**

- `Tabs`

**Functions and methods**

- `New`
- `WithTabs`
- `WithActive`
- `WithWidth`
- `Update`
- `View`
- `Active`
- `Labels`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Construct with New(theme).WithTabs([...]).WithActive(0). Forward tea.KeyMsg through Update and read Active() to switch the panel your parent model renders below the tab row.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/tabs/story](./story) — runnable story binary (`go run -tags glyph_story ./components/tabs/story/`)
- [registry manifest](./tabs.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
