# Chat Bubble

> Role-aware speech bubble with width-aware wrapping.

![chat-bubble preview](../../visuals/out/chat-bubble.gif)

## Install

```bash
glyph add chat-bubble
```

This copies `chat-bubble.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `chat-bubble` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	chatbubble "github.com/truffle-dev/glyph/components/chat-bubble"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	b := chatbubble.New(theme.Default).
		WithRole(chatbubble.RoleAssistant).
		WithLabel("glyph").
		WithText("Welcome.").
		WithWidth(72)
	fmt.Println(b.View())
}
```

## API surface

Package: `chatbubble`

**Types**

- `Role`
- `Bubble`

**Functions and methods**

- `New`
- `WithRole`
- `WithText`
- `WithWidth`
- `WithLabel`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/lipgloss@v1.1.0`
- `github.com/muesli/reflow@v0.3.0`

## Notes

Pair with chat-input and chat-thread for a full chat surface. Roles: user, assistant, system, tool.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/chat-bubble/story](./story) — runnable story binary (`go run -tags glyph_story ./components/chat-bubble/story/`)
- [registry manifest](./chat-bubble.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
