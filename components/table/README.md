# Table

> Column-aligned data table with sortable headers, keyboard navigation, internal scrolling, and optional row selection.

![table preview](../../visuals/out/table.gif)

## Install

```bash
glyph add table
```

This copies `table.go` (and its test file) into your repo at the path your
`glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `table` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/table"
)

func main() {
	t := table.New().
		WithColumns(
			table.Column{Key: "repo", Title: "Repo", Sortable: true},
			table.Column{Key: "owner", Title: "Owner", Sortable: true},
			table.Column{Key: "stars", Title: "Stars", Align: table.AlignRight, Sortable: true},
		).
		WithRows(
			table.Row{Cells: []string{"glyph", "truffle-dev", "12"}, Value: "glyph"},
			table.Row{Cells: []string{"vigil", "baudsmithstudios", "47"}, Value: "vigil"},
			table.Row{Cells: []string{"voltagent", "VoltAgent", "284"}, Value: "voltagent"},
		).
		WithSize(60, 8).
		WithRowSelection(true)
	fmt.Println(t.View())
}
```

## API surface

Package: `table`

**Types**

- `Column`
- `Row`
- `Align`
- `Model`
- `SelectMsg`
- `SortMsg`
- `CursorMsg`

**Functions and methods**

- `New`
- `WithColumns`
- `WithRows`
- `WithSize`
- `WithSelectedRow`
- `WithSortBy`
- `WithRowSelection`
- `WithHighlightCursor`
- `WithTheme`
- `Cursor`
- `SelectedRow`
- `SortBy`
- `Rows`
- `Columns`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

Up/Down or j/k move the cursor; Left/Right or h/l shift the "active" column underline; `s` toggles ascending/descending sort on the active column (only when `Sortable: true`); PgUp/PgDn page; Home/g and End/G jump to the ends; Enter emits `table.SelectMsg{Row, Index}` when `WithRowSelection(true)`. Columns with `Width: 0` auto-fit to the widest cell (capped at 40 cells with a 2-cell padding); overflow is absorbed by proportional shrink of auto columns first, then right-truncation of fixed columns with an ellipsis. Sort comparison is numeric when both cells parse as floats, otherwise byte-wise lex; empties sort last in ascending and first in descending; sort is stable so equal keys keep insertion order.

## See also

- [examples/showcase](../../examples/showcase) — single-binary TUI composing the seven main surfaces
- [components/table/story](./story) — runnable story binary (`go run -tags glyph_story ./components/table/story/`)
- [registry manifest](./table.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
