# Status Bar

> Single-line three-segment status bar.

![status-bar preview](../../visuals/out/status-bar.gif)

## Install

```bash
glyph add status-bar
```

This copies `status-bar.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `status-bar` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	statusbar "github.com/truffle-dev/glyph/components/status-bar"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	bar := statusbar.New(theme.Default).
		WithWidth(80).
		WithLeft(statusbar.Item{Text: "glyph"}).
		WithCenter(statusbar.Item{Text: "main"}).
		WithRight(statusbar.Item{Text: "OK", Style: statusbar.StyleSuccess})
	fmt.Println(bar.View())
}
```

## API surface

Package: `statusbar`

**Types**

- `Style`
- `Item`
- `Bar`

**Functions and methods**

- `New`
- `WithWidth`
- `WithSeparator`
- `WithLeft`
- `WithCenter`
- `WithRight`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Compose with your main view: lipgloss.JoinVertical(lipgloss.Left, mainView, bar.View()). Item styles available: Default, Primary, Success, Warning, Error, Muted.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/status-bar/story](./story) — runnable story binary (`go run -tags glyph_story ./components/status-bar/story/`)
- [registry manifest](./status-bar.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
