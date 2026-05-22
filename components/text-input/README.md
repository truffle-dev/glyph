# Text Input

> Multi-line text input with placeholder, focus, 2D cursor, Ctrl-U kill-to-cursor, Alt+Left/Right word jumps, and Home/End.

![text-input preview](../../visuals/out/text-input.gif)

## Install

```bash
glyph add text-input
```

This copies `text-input.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `text-input` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	textinput "github.com/truffle-dev/glyph/components/text-input"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	t := textinput.New(theme.Default).
		WithPlaceholder("Commit message…").
		WithWidth(72).
		WithHeight(6).
		Focus()
	fmt.Println(t.View())
}
```

## Key bindings

- `Enter` insert a newline
- `Ctrl-D` emit `textinput.SubmitMsg{Value}`
- `Esc` emit `textinput.CancelMsg{}`
- `Backspace` delete the rune before the cursor; join lines when at column 0
- `Ctrl-U` kill from start of line to the cursor
- `Ctrl-K` kill from the cursor to end of line
- `Alt+Left` / `Alt+Right` jump one word at a time
- `Left` / `Right` / `Up` / `Down` 2D cursor movement
- `Home` / `End` snap to line start / end

## API surface

Package: `textinput`

**Types**

- `SubmitMsg`
- `CancelMsg`
- `Input`

**Functions and methods**

- `New`
- `WithPlaceholder`
- `WithWidth`
- `WithHeight`
- `WithValue`
- `Focus`
- `Blur`
- `Focused`
- `Value`
- `Cursor`
- `Reset`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

`Enter` inserts a newline, so `Ctrl-D` is the accept binding. The input emits
`textinput.SubmitMsg` on accept and `textinput.CancelMsg` on Esc. Pair with
`panel` for a labeled commit-message surface, or with `key-hints` to advertise
the bindings.

## See also

- [components/chat-input](../chat-input) — single-line cousin that uses Enter to submit
- [components/text-input/story](./story) — runnable story binary (`go run -tags glyph_story ./components/text-input/story/`)
- [registry manifest](./text-input.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
