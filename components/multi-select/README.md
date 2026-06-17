# Multi-Select

> Bounded multi-choice list with checkbox rows, optional typeahead filter, and a selection that survives filtering.

## Install

```bash
glyph add multi-select
```

This copies `multi-select.go` (and its test file) into your repo at the path
your `glyph.json` aliases declare. After install, the file is yours: edit it,
refactor it, rename it. There is no `multi-select` library to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	multiselect "github.com/truffle-dev/glyph/components/multi-select"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	m := multiselect.New(theme.Default).
		WithTitle("Stage files").
		WithOptions([]multiselect.Option{
			{Label: "src/main.go", Hint: "modified", Value: "main"},
			{Label: "src/util.go", Hint: "modified", Value: "util"},
			{Label: "README.md", Hint: "modified", Value: "readme"},
		}).
		WithChecked([]string{"main"}).
		WithSize(48, 6)
	fmt.Println(m.View())
}
```

## Keys

- `Up`/`Down` (or `k`/`j`) walk the list.
- `Space` or `Tab` toggle the row under the cursor.
- `a` toggles every currently visible row as a group (check-all, then clear-all).
- `Home`/`End`, `PgUp`/`PgDn` jump and page.
- `Enter` commits a `ConfirmMsg`; `Esc` cancels with `CancelMsg`.
- With `WithFilter(true)`, typing narrows by substring over label and hint;
  `Backspace` and `Ctrl-U` edit the query. Selection is keyed by each option's
  resolved value, so a checked row stays checked when the filter hides and
  later reveals it.

## API surface

Package: `multiselect`

**Types**

- `Option`
- `ConfirmMsg`
- `CancelMsg`
- `MultiSelect`

**Functions and methods**

- `New`
- `WithOptions`
- `WithChecked`
- `WithSize`
- `WithTitle`
- `WithPlaceholder`
- `WithFilter`
- `Cursor`
- `Count`
- `SelectedValues`
- `SelectedOptions`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

`ConfirmMsg.Selected` and `ConfirmMsg.Values` list the checked options in their
original (unfiltered) order, so a parent can persist them without re-sorting.
`WithOptions` drops any checked value that the new option set no longer
contains, so the committed selection never names a row the user can't see.

## See also

- [components/select](../select) — the single-choice sibling
- [components/multi-select/story](./story) — runnable story binary (`go run -tags glyph_story ./components/multi-select/story/`)
- [registry manifest](./multi-select.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
