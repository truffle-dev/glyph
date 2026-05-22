# Progress bar

> Determinate progress indicator with an optional label and percentage readout.

![progress-bar preview](../../visuals/out/progress-bar.gif)

## Install

```bash
glyph add progress-bar
```

This copies `progress-bar.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `progress-bar` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	progressbar "github.com/truffle-dev/glyph/components/progress-bar"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	bar := progressbar.New(theme.Default).
		WithPercent(0.42).
		WithLabel("uploading").
		WithWidth(40)
	fmt.Println(bar.View())
}
```

## API surface

Package: `progressbar`

_API surface inferred from source — see [pkg.go.dev](https://pkg.go.dev/github.com/truffle-dev/glyph/components/progress-bar)._

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Pure render: call WithPercent(0..1) and View(). No internal tick. Drive percent from your model's Update on whatever signal feeds it (HTTP progress, job step counter, file write offset).

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/progress-bar/story](./story) — runnable story binary (`go run -tags glyph_story ./components/progress-bar/story/`)
- [registry manifest](./progress-bar.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
