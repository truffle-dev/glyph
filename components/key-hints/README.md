# Key hints

> Compact footer of key-and-description pairs separated by a thin divider.

![key-hints preview](../../visuals/out/key-hints.gif)

## Install

```bash
glyph add key-hints
```

This copies `key-hints.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `key-hints` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	keyhints "github.com/truffle-dev/glyph/components/key-hints"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	hints := keyhints.New(theme.Default).
		WithHints([]keyhints.Hint{
			{Key: "Tab", Desc: "next pane"},
			{Key: "q", Desc: "quit"},
		})
	fmt.Println(hints.View())
}
```

## API surface

Package: `keyhints`

_API surface inferred from source — see [pkg.go.dev](https://pkg.go.dev/github.com/truffle-dev/glyph/components/key-hints)._

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Pure render. Build a []keyhints.Hint{{Key, Desc}, ...} from your model's current binding set; the bar lays it out left-to-right and clamps to WithWidth.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/key-hints/story](./story) — runnable story binary (`go run -tags glyph_story ./components/key-hints/story/`)
- [registry manifest](./key-hints.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
