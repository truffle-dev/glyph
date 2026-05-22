# Diff View

> Color-coded unified diff renderer.

![diff-view preview](../../visuals/out/diff-view.gif)

## Install

```bash
glyph add diff-view
```

This copies `diff-view.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `diff-view` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	diffview "github.com/truffle-dev/glyph/components/diff-view"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	raw := `--- a/server.go
+++ b/server.go
@@ -1,3 +1,4 @@
 package server
+
 func Run() {}
`
	d := diffview.New(theme.Default).
		WithSize(96, 18).
		WithLines(diffview.ParseUnified(raw))
	fmt.Println(d.View())
}
```

## API surface

Package: `diffview`

**Types**

- `Kind`
- `Line`
- `View`

**Functions and methods**

- `New`
- `WithLines`
- `WithSize`
- `WithLineNumbers`
- `Offset`
- `Init`
- `Update`
- `View`
- `TotalLines`
- `ParseUnified`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Pass WithLines(ParseUnified(diff)) to render `git diff -u` output. To render a diff you generated yourself, construct []Line directly. Up/Down scrolls one line, PgUp/PgDn scrolls a window, Home/End jumps. Lines longer than the body width truncate with an ellipsis.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/diff-view/story](./story) — runnable story binary (`go run -tags glyph_story ./components/diff-view/story/`)
- [registry manifest](./diff-view.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
