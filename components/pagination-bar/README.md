# Pagination Bar

> Single-line page indicator with prev/next chevrons, a "page of total"
> label, and an optional "(N items)" suffix. Pair it with a table, a
> log viewer, a search-result list, or any windowed view; the same
> component works for 3 pages or 3000.

## Install

```bash
glyph add pagination-bar
```

This copies `pagination-bar.go` (and its test file) into your repo at
the path your `glyph.json` aliases declare, along with the sibling
`theme` dependency. After install, the files are yours: edit them,
refactor them, rename them.

## Hello, world

```go
package main

import (
	"fmt"

	paginationbar "github.com/truffle-dev/glyph/components/pagination-bar"
)

func main() {
	m := paginationbar.New().
		WithTotalItems(247).
		WithPerPage(20).
		WithPage(0)
	fmt.Println(m.View())
	// ‹ 1 of 13 › (247 items)
}
```

## API surface

Package: `paginationbar`

**Types**

- `Model`
- `PageChangedMsg`

**Functions and methods**

- `New`
- `WithTheme`
- `WithTotal`
- `WithPage`
- `WithTotalItems`
- `WithPerPage`
- `WithWidth`
- `WithWrap`
- `WithPrefix`
- `WithItemsLabel`
- `Page`
- `Total`
- `PageNumber`
- `AtStart`
- `AtEnd`
- `VisibleRange`
- `Init`
- `Update`
- `View`

## Keys

| Key           | Action                                             |
|---------------|----------------------------------------------------|
| `right` / `l` | Move to the next page (no-op at end without wrap). |
| `left` / `h`  | Move to the previous page (no-op at start).        |
| `end` / `G`   | Jump to the last page.                             |
| `home` / `g`  | Jump to the first page.                            |

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

`Page()` is 0-indexed for code; the rendered "page of total" label is
1-indexed because that's what a reader expects. `PageChangedMsg` fires
only on actual page changes, so consumers can listen to the message
without filtering no-ops at either edge. The chevron at an edge dims to
the muted color when wrap is off; turning wrap on with `WithWrap(true)`
keeps both chevrons bright and motion wraps in both directions. The
"(N items)" suffix appears only when `WithTotalItems` has been called;
pair it with `WithPerPage(k)` and the component derives both
`Total()` and `VisibleRange()` for you so the parent doesn't have to
do the division. `WithPrefix("Results ")` adds a muted label before
the chevron row when the bar is part of a wider status line.

## See also

- [components/tabs](../tabs) — horizontal single-active selector across labels rather than a single position in a range
- [components/timeline](../timeline) — vertical event log that this bar might paginate over
- [registry manifest](./pagination-bar.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
