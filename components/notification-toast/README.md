# Notification Toast

> Stacked, dismissible notifications with level-aware coloring.

![notification-toast preview](../../visuals/out/notification-toast.gif)

## Install

```bash
glyph add notification-toast
```

This copies `notification-toast.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `notification-toast` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"
	"time"

	notificationtoast "github.com/truffle-dev/glyph/components/notification-toast"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	tray := notificationtoast.New(theme.Default).
		WithWidth(48).
		WithMaxItems(3)
	tray = tray.Push(notificationtoast.Toast{
		ID:        "build-1",
		Level:     notificationtoast.LevelSuccess,
		Title:     "Success",
		Message:   "Build complete.",
		ExpiresAt: time.Now().Add(6 * time.Second),
	})
	fmt.Println(tray.View())
}
```

## API surface

Package: `notificationtoast`

**Types**

- `Level`
- `Toast`
- `Tray`

**Functions and methods**

- `New`
- `WithWidth`
- `WithMaxItems`
- `Push`
- `Dismiss`
- `DismissAll`
- `Tick`
- `Toasts`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`
- `github.com/muesli/reflow@v0.3.0`

## Notes

Drive expiry from a tea.Cmd that sends a tick message every second: tr = tr.Tick(time.Now()). The tray renders nothing when empty; lay it over your main view with lipgloss.JoinVertical or .Place to position it.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/notification-toast/story](./story) — runnable story binary (`go run -tags glyph_story ./components/notification-toast/story/`)
- [registry manifest](./notification-toast.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
