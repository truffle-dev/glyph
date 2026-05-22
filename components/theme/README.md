# Theme

> Token palette every glyph component reads from.

![theme preview](../../visuals/out/theme.gif)

## Install

```bash
glyph add theme
```

This copies `theme.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `theme` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	t := theme.Default     // dark
	// t := theme.Light    // warm paper
	fmt.Println(t.Primary) // lipgloss.Color
}
```

## API surface

Package: `theme`

**Types**

- `Theme`

## Dependencies

- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Edit tokens.go to retheme. Default is a dark terminal palette; Light is a warm-paper alternative.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/theme/story](./story) — runnable story binary (`go run -tags glyph_story ./components/theme/story/`)
- [registry manifest](./theme.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
