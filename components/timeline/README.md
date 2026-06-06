# Timeline

> Vertical sequence of events with status dots, an optional pre-formatted
> time gutter, and multi-line bodies. Drop in for deploy history, audit
> logs, oncall feeds, and agent-run replays.

## Install

```bash
glyph add timeline
```

This copies `timeline.go` (and its test file) into your repo at the
path your `glyph.json` aliases declare. After install, the file is
yours: edit it, refactor it, rename it. There is no `timeline` library
to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/timeline"
)

func main() {
	t := timeline.New().
		WithEvents(
			timeline.Event{Time: "13:42", Title: "Deploy started", Status: timeline.StatusInfo},
			timeline.Event{Time: "13:43", Title: "Tests passed", Body: "All green on staging.", Status: timeline.StatusSuccess},
			timeline.Event{Time: "13:45", Title: "Deploy complete", Status: timeline.StatusSuccess},
		).
		WithSize(60, 12)
	fmt.Println(t.View())
}
```

## API surface

Package: `timeline`

**Types**

- `Event`
- `Status`
- `Model`
- `SelectMsg`
- `CursorMsg`

**Functions and methods**

- `New`
- `WithTheme`
- `WithEvents`
- `WithSize`
- `WithSelectedEvent`
- `WithTimeColumn`
- `WithHighlightCursor`
- `Cursor`
- `SelectedEvent`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Time strings are pre-formatted; the component does not parse times, so
absolute clocks (`13:42`), relative durations (`2m ago`), and git-style
labels (`3 days`) all flow through the same gutter uniformly. Status
colors the dot only (`StatusSuccess`, `StatusWarning`, `StatusError`,
`StatusInfo`, `StatusNeutral`). Body may contain newlines; each line
renders under the title in the muted gutter style with a `│` continuation
bar. Up/Down or j/k step the cursor by one event; PgUp/PgDn move by half
the visible height; Home/g and End/G jump to the ends; Enter emits
`timeline.SelectMsg{Event, Index}`. The selected event title is
highlighted with a `SurfaceStrong` background when `WithHighlightCursor`
is on (default). The model auto-sizes the time gutter to the widest
`Time` string up to 20 cells; columns under 1 cell collapse to a single
ellipsis.

## See also

- [components/log-stream](../log-stream) — tail-style streaming log view (related, but for high-throughput per-second arrivals rather than discrete events)
- [registry manifest](./timeline.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
