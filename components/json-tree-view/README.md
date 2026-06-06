# JSON Tree View

> Interactive, collapsible tree view for arbitrary JSON values. A thin
> shell over `tree-view` that formats strings, numbers, booleans, null,
> objects, and arrays with type-aware colors and count suffixes. The
> same component handles a 4-key API response, a multi-megabyte log
> record, or the body of a webhook payload — what changes is the
> caller's data, not the component.

## Install

```bash
glyph add json-tree-view
```

This copies `json-tree-view.go` (and its test file) into your repo at
the path your `glyph.json` aliases declare, along with its sibling
`tree-view` and `theme` dependencies. After install, the files are
yours: edit them, refactor them, rename them.

## Hello, world

```go
package main

import (
	"fmt"

	jsontreeview "github.com/truffle-dev/glyph/components/json-tree-view"
)

func main() {
	body := []byte(`{
		"name": "phantom",
		"version": 7,
		"active": true,
		"owner": null,
		"tags": ["a", "b", "c"],
		"limits": {"cpu": 2, "mem": 512}
	}`)
	m := jsontreeview.New().
		WithJSON(body).
		WithExpandedDepth(2).
		WithSize(50, 20)
	fmt.Println(m.View())
}
```

## API surface

Package: `jsontreeview`

**Types**

- `Model`
- `SelectMsg`

**Functions and methods**

- `New`
- `WithTheme`
- `WithValue`
- `WithJSON`
- `WithRootKey`
- `WithSortKeys`
- `WithSize`
- `WithExpandAll`
- `WithCollapseAll`
- `WithExpandedDepth`
- `WithHighlightCursor`
- `WithTitle`
- `WithPlaceholder`
- `WithRootVisible`
- `Cursor`
- `SelectedPath`
- `SelectedNode`
- `SelectedValue`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- glyph component `tree-view` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

`WithJSON` runs `json.Unmarshal` into an `any` and returns the Model
unchanged on parse error; callers that need parse-error reporting
should unmarshal first and call `WithValue`. Objects render as
`key {N}` branches sorted alphabetically by default (`WithSortKeys(false)`
turns sorting off, at which point Go's nondeterministic map iteration
order applies). Arrays render as `key [N]` branches with `[i]` child
keys. Strings come back quoted via `strconv.Quote`; integer-valued
floats collapse to integer form (so `7.0` prints as `7`); booleans
and null get their literal text. The root row uses `$` by default —
override with `WithRootKey`. The component carries the underlying JSON
value on every Node's `Value`, so `SelectedValue()` and the wrapped
`SelectMsg.Value` give you the original `map[string]any` / `[]any` /
scalar without re-walking the tree. All navigation (cursor, expand,
collapse, scroll, keyboard bindings) forwards directly to the embedded
`tree-view`.

## See also

- [components/tree-view](../tree-view) — the recursive collapsible-tree primitive this wraps
- [components/file-tree](../file-tree) — file-system specialization of tree-view
- [registry manifest](./json-tree-view.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
