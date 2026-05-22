# Chat Thread

> Vertically scrolling conversation surface.

![chat-thread preview](../../visuals/out/chat-thread.gif)

## Install

```bash
glyph add chat-thread
```

This copies `chat-thread.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `chat-thread` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	chatbubble "github.com/truffle-dev/glyph/components/chat-bubble"
	chatthread "github.com/truffle-dev/glyph/components/chat-thread"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	t := chatthread.New(theme.Default).WithSize(72, 12)
	t = t.Append(chatthread.Message{
		Role:  chatbubble.RoleAssistant,
		Label: "glyph",
		Text:  "Welcome.",
	})
	fmt.Println(t.View())
}
```

## API surface

Package: `chatthread`

**Types**

- `Message`
- `Thread`

**Functions and methods**

- `New`
- `WithSize`
- `WithMessages`
- `Append`
- `Messages`
- `ScrollUp`
- `ScrollDown`
- `ScrollToBottom`
- `ScrollToTop`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- glyph component `chat-bubble` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Pair with chat-input for a full chat surface. Drive Append() from your tea.Model when the input fires SubmitMsg.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/chat-thread/story](./story) — runnable story binary (`go run -tags glyph_story ./components/chat-thread/story/`)
- [registry manifest](./chat-thread.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
