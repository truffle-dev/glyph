# Log Stream

> Bounded, color-coded log view that tails like `tail -f`.

![log-stream preview](../../visuals/out/log-stream.gif)

## Install

```bash
glyph add log-stream
```

This copies `log-stream.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `log-stream` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	logstream "github.com/truffle-dev/glyph/components/log-stream"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	s := logstream.New(theme.Default).
		WithSize(96, 16).
		WithMinLevel(logstream.LevelInfo)
	s = s.Append(logstream.Entry{
		Level:   logstream.LevelWarn,
		Source:  "auth",
		Message: "deprecated token format",
	})
	fmt.Println(s.View())
}
```

## API surface

Package: `logstream`

**Types**

- `Level`
- `Entry`
- `Stream`

**Functions and methods**

- `String`
- `New`
- `WithCapacity`
- `WithSize`
- `WithMinLevel`
- `WithTimestamps`
- `WithTimeFormat`
- `Append`
- `Clear`
- `Entries`
- `Offset`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`
- `github.com/muesli/reflow@v0.3.0`

## Notes

Call Append(Entry{Time, Level, Source, Message}) on every new log line. The view auto-tails unless the user has scrolled up. WithCapacity caps the ring; WithMinLevel hides entries below the threshold (the buffer is unchanged).

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/log-stream/story](./story) — runnable story binary (`go run -tags glyph_story ./components/log-stream/story/`)
- [registry manifest](./log-stream.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
