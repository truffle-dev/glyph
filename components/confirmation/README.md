# Confirmation

> Two-button yes/no prompt with focus-managed buttons, y/n shortcuts, dangerous-action styling, and prompt reflow.

![confirmation preview](../../visuals/out/confirmation.gif)

## Install

```bash
glyph add confirmation
```

This copies `confirmation.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `confirmation` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/confirmation"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	c := confirmation.New(theme.Default).
		WithPrompt("Delete this conversation?").
		WithYesLabel("Delete").
		WithDangerous(true).
		WithWidth(48)
	fmt.Println(c.View())
}
```

## API surface

Package: `confirmation`

**Types**

- `ConfirmMsg`
- `CancelMsg`
- `Confirm`

**Functions and methods**

- `New`
- `WithPrompt`
- `WithYesLabel`
- `WithNoLabel`
- `WithDefault`
- `WithDangerous`
- `WithWidth`
- `FocusedYes`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`
- `github.com/muesli/reflow@v0.3.0`

## Notes

Tab/Left/Right walk between Yes and No; y/Y and n/N commit with one keystroke regardless of focus; Enter commits the focused button. Emits confirmation.ConfirmMsg{Value bool} on commit, confirmation.CancelMsg on Esc. Set WithDangerous(true) to style Yes with theme.Error for destructive actions.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/confirmation/story](./story) — runnable story binary (`go run -tags glyph_story ./components/confirmation/story/`)
- [registry manifest](./confirmation.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
