# Panel

> Bordered container with optional title and footer.

![panel preview](../../visuals/out/panel.gif)

## Install

```bash
glyph add panel
```

This copies `panel.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `panel` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/panel"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	p := panel.New(theme.Default).
		WithTitle("Logs").
		WithFooter("3 entries").
		WithContent("...")
	fmt.Println(p.View())
}
```

## API surface

Package: `panel`

**Types**

- `Variant`
- `Panel`

**Functions and methods**

- `New`
- `WithTitle`
- `WithFooter`
- `WithContent`
- `WithWidth`
- `WithHeight`
- `WithVariant`
- `WithPadding`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Panel is pure render: no Update, no Cmd. Compose other components by setting their View() output as the content. Use WithWidth/WithHeight to clamp the outer dimensions, or omit both for natural sizing.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/panel/story](./story) — runnable story binary (`go run -tags glyph_story ./components/panel/story/`)
- [registry manifest](./panel.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
