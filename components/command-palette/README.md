# Command Palette

> Filterable modal command picker.

![command-palette preview](../../visuals/out/command-palette.gif)

## Install

```bash
glyph add command-palette
```

This copies `command-palette.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `command-palette` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	commandpalette "github.com/truffle-dev/glyph/components/command-palette"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	p := commandpalette.New(theme.Default).
		WithCommands([]commandpalette.Command{
			{ID: "save", Title: "Save file", Keybinding: "ctrl+s"},
			{ID: "open", Title: "Open file", Keybinding: "ctrl+o"},
		}).
		WithSize(72, 14)
	fmt.Println(p.View())
}
```

## API surface

Package: `commandpalette`

**Types**

- `Command`
- `SelectMsg`
- `CancelMsg`
- `Matcher`
- `Palette`

**Functions and methods**

- `New`
- `WithCommands`
- `WithFilter`
- `WithSize`
- `WithTitle`
- `WithPlaceholder`
- `WithMatcher`
- `Filter`
- `Cursor`
- `Init`
- `Update`
- `View`
- `SubstringMatcher`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Emits commandpalette.SelectMsg on Enter with the chosen Command, commandpalette.CancelMsg on Esc. Pass WithCommands to load the list. Substitute the substring matcher with WithMatcher for fuzzy ranking.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/command-palette/story](./story) — runnable story binary (`go run -tags glyph_story ./components/command-palette/story/`)
- [registry manifest](./command-palette.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
