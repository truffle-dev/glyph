# Modal

> Overlay container with title, body, footer, and a configurable close key.

![modal preview](../../visuals/out/modal.gif)

## Install

```bash
glyph add modal
```

This copies `modal.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `modal` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/modal"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	m := modal.New(theme.Default).
		WithTitle("Unsaved changes").
		WithBody("You have uncommitted edits. Save before quitting?").
		WithFooter("Enter to save · Esc to cancel").
		WithSize(48, 8)
	fmt.Println(m.View())
}
```

## API surface

Package: `modal`

**Types**

- `CloseMsg`
- `Modal`

**Functions and methods**

- `New`
- `WithTitle`
- `WithBody`
- `WithSize`
- `WithCloseKey`
- `WithFooter`
- `Width`
- `Height`
- `ContentWidth`
- `ContentHeight`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Render m.View() into a string, then lipgloss.Place(parentW, parentH, lipgloss.Center, lipgloss.Center, m.View(), lipgloss.WithWhitespaceChars(" ")) over your background. The modal emits modal.CloseMsg on Esc by default.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/modal/story](./story) — runnable story binary (`go run -tags glyph_story ./components/modal/story/`)
- [registry manifest](./modal.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
