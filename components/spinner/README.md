# Spinner

> Animated single-glyph progress indicator with an optional label.

![spinner preview](../../visuals/out/spinner.gif)

## Install

```bash
glyph add spinner
```

This copies `spinner.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `spinner` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/spinner"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	s := spinner.New(theme.Default).
		WithStyle(spinner.StyleDots).
		WithLabel("Working")
	fmt.Println(s.View())
}
```

## API surface

Package: `spinner`

**Types**

- `Style`
- `TickMsg`
- `Spinner`

**Functions and methods**

- `New`
- `WithStyle`
- `WithLabel`
- `WithInterval`
- `WithColor`
- `WithID`
- `Init`
- `Update`
- `View`
- `Frame`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Call Init to start the tick. Forward TickMsg through your model's Update so it reaches the spinner's Update. Use WithID when composing more than one spinner so each gets its own tick stream.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/spinner/story](./story) — runnable story binary (`go run -tags glyph_story ./components/spinner/story/`)
- [registry manifest](./spinner.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
