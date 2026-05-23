# Kbd

> Render terminal-styled keycap glyphs for single keys, chords, and key sequences.

![kbd preview](../../visuals/out/kbd.gif)

## Install

```bash
glyph add kbd
```

This copies `kbd.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `kbd` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/kbd"
)

func main() {
	fmt.Println(kbd.Render("Ctrl"))
	fmt.Println(kbd.Chord("Ctrl", "K"))
	fmt.Println(kbd.Sequence(kbd.Chord("g"), kbd.Chord("g")))
}
```

## API surface

Package: `kbd`

**Types**

- `Style`

**Constants**

- `ChordSeparator`
- `SequenceSeparator`

**Functions**

- `Render`
- `RenderStyled`
- `Chord`
- `Sequence`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Stateless rendering primitive. Call `kbd.Render("Ctrl")` or `kbd.Chord("Ctrl", "K")` from inside your existing View() and place the result wherever a keycap belongs. The atom reads `theme.Default` automatically; pass a `Style` to `RenderStyled` when you need to override a single color. Compose into hint rows with `kbd.Sequence(kbd.Chord("g"), kbd.Chord("g"))` for keystroke series.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/kbd/story](./story) — runnable story binary (`go run -tags glyph_story ./components/kbd/story/`)
- [registry manifest](./kbd.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
