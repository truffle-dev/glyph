# Table (virtualized)

> Column-aligned table over a `RowProvider`, with O(visible) render
> cost regardless of source size. Drop in for log explorers, query
> result viewers, or any surface where the row count outgrows the
> screen by orders of magnitude.

## Install

```bash
glyph add table-virtualized
```

This copies `table_virtualized.go` (and its test file) into your repo
at the path your `glyph.json` aliases declare. After install, the file
is yours: edit it, refactor it, rename it. There is no
`table-virtualized` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	tv "github.com/truffle-dev/glyph/components/table-virtualized"
)

func main() {
	rows := make(tv.SliceProvider, 1_000_000)
	for i := range rows {
		rows[i] = tv.Row{Cells: []string{
			fmt.Sprintf("%d", i),
			fmt.Sprintf("event-%d", i),
		}}
	}

	t := tv.New().
		WithColumns(
			tv.Column{Key: "id", Title: "ID", Width: 10, Align: tv.AlignRight},
			tv.Column{Key: "name", Title: "Event", Width: 24},
		).
		WithRows(rows).
		WithSize(60, 12).
		WithRowSelection(true)

	fmt.Println(t.View())
}
```

A million-row slice renders in constant time because `View()` only
materializes the visible window. Replace `SliceProvider` with a
channel-fed buffer, a SQLite cursor, or an mmap'd file and the cost
profile is unchanged.

## API surface

Package: `tablevirtualized`

**Types**

- `Column`
- `Row`
- `Align`
- `RowProvider`
- `SliceProvider`
- `Model`
- `SelectMsg`
- `CursorMsg`

**Functions and methods**

- `New`
- `WithColumns`
- `WithRows`
- `WithSize`
- `WithSelectedRow`
- `WithRowSelection`
- `WithHighlightCursor`
- `WithTheme`
- `Cursor`
- `SelectedRow`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Implement `RowProvider` (`Len() int` and `At(i int) Row`) over your
source and pass it to `WithRows`. Columns require explicit `Width`
(widths under 4 cells are clamped to 4). The model does not auto-fit
across the source; that would defeat virtualization on large inputs.
Sort is the caller's responsibility. Up/Down or j/k move the cursor;
PgUp/PgDn page by the visible row count; Home/g and End/G jump to the
ends; Enter emits `tablevirtualized.SelectMsg{Row, Index}` when
`WithRowSelection(true)`. When the dataset extends past the visible
window, an arrow glyph (`▲` or `▼`) replaces the first or last visible
row to signal more rows above or below. A nil provider is safe and
renders the placeholder.

## See also

- [components/table](../table) — finite, in-memory variant with sort and auto-fit
- [registry manifest](./table-virtualized.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
