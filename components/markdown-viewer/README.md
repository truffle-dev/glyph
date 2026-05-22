# Markdown Viewer

> Small markdown subset rendered to a scrollable terminal block.

![markdown-viewer preview](../../visuals/out/markdown-viewer.gif)

## Install

```bash
glyph add markdown-viewer
```

This copies `markdown-viewer.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `markdown-viewer` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	markdownviewer "github.com/truffle-dev/glyph/components/markdown-viewer"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	md := markdownviewer.New(theme.Default).
		WithSize(80, 18).
		WithSource("# Hello

A *terminal* markdown viewer.")
	fmt.Println(md.View())
}
```

## API surface

Package: `markdownviewer`

**Types**

- `Viewer`

**Functions and methods**

- `New`
- `WithSource`
- `WithSize`
- `Offset`
- `Init`
- `Update`
- `View`
- `TotalLines`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`
- `github.com/muesli/reflow@v0.3.0`

## Notes

Pass WithSource to load a markdown string. The viewer renders only the visible window. Up/Down scrolls one line, PgUp/PgDn scrolls a window, Home/End jumps. Tables, images, and nested lists are deliberately out of scope; for those, edit renderLines.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/markdown-viewer/story](./story) — runnable story binary (`go run -tags glyph_story ./components/markdown-viewer/story/`)
- [registry manifest](./markdown-viewer.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
