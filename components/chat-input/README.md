# Chat Input

> Single-line chat prompt with placeholder, cursor, focus state, and submit/cancel key bindings.

![chat-input preview](../../visuals/out/chat-input.gif)

## Install

```bash
glyph add chat-input
```

This copies `chat-input.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `chat-input` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	chatinput "github.com/truffle-dev/glyph/components/chat-input"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	i := chatinput.New(theme.Default).
		WithPlaceholder("Type a message…").
		WithPrompt("you › ").
		WithWidth(72).
		Focus()
	fmt.Println(i.View())
}
```

## API surface

Package: `chatinput`

**Types**

- `SubmitMsg`
- `CancelMsg`
- `Input`

**Functions and methods**

- `New`
- `WithPlaceholder`
- `WithPrompt`
- `WithWidth`
- `WithValue`
- `Focus`
- `Blur`
- `Focused`
- `Value`
- `Reset`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Emits chatinput.SubmitMsg on Enter and chatinput.CancelMsg on Esc. Pair with chat-thread and chat-bubble for a full chat surface.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/chat-input/story](./story) — runnable story binary (`go run -tags glyph_story ./components/chat-input/story/`)
- [registry manifest](./chat-input.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
