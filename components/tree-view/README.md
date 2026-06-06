# Tree View

> Generic, recursive collapsible tree of nodes. The flexible primitive
> beneath `file-tree`, beneath the forthcoming `json-tree-view`, and
> beneath any agent-state explorer, org chart, call graph, or build
> dependency surface. A Node has a Label, an arbitrary Value, and zero
> or more Children. The component never assumes file-system semantics.

## Install

```bash
glyph add tree-view
```

This copies `tree-view.go` (and its test file) into your repo at the
path your `glyph.json` aliases declare. After install, the file is
yours: edit it, refactor it, rename it. There is no `tree-view` library
to keep in sync.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/tree-view"
)

func main() {
	root := treeview.Node{
		Label: "project",
		Children: []treeview.Node{
			{
				Label: "src",
				Children: []treeview.Node{
					{Label: "main.go"},
					{Label: "util.go"},
				},
			},
			{
				Label: "docs",
				Children: []treeview.Node{
					{Label: "README.md"},
				},
			},
			{Label: "go.mod"},
		},
	}
	m := treeview.New().
		WithRoot(root).
		WithExpandedDepth(2).
		WithSize(40, 10)
	fmt.Println(m.View())
}
```

## API surface

Package: `treeview`

**Types**

- `Node`
- `Model`
- `SelectMsg`
- `CursorMsg`

**Functions and methods**

- `New`
- `WithTheme`
- `WithRoot`
- `WithExpandedDepth`
- `WithExpandAll`
- `WithCollapseAll`
- `WithSize`
- `WithRootVisible`
- `WithHighlightCursor`
- `WithTitle`
- `WithPlaceholder`
- `WithIndent`
- `Cursor`
- `SelectedPath`
- `SelectedNode`
- `IsExpanded`
- `Init`
- `Update`
- `View`

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

A `Node` is a branch when it has children and a leaf when it does not;
the component does not need a separate flag. Paths are slash-joined
zero-based child indices (`"0/2/1"` is the second grandchild of the
third child of the root), which makes them stable across label changes
and free of name-collision concerns. The expanded set lives on the
Model, not the Node, so a tree can be re-rooted without losing its
open/closed shape. `Up`/`Down`/`j`/`k` step by one row; `PgUp`/`PgDn`
move by half the visible height; `Home`/`g` and `End`/`G` jump to the
ends; `Right`/`l` expands a collapsed branch under the cursor; `Left`/`h`
collapses an expanded branch or jumps the cursor to the parent row
otherwise; `Enter` toggles a branch and always emits `SelectMsg`;
`Space` toggles a branch silently. `WithRootVisible(false)` hides the
root and renders its children at depth 0, useful when a single virtual
container shouldn't take a row. `WithExpandedDepth(n)` opens every
branch whose depth is strictly less than n; `WithExpandAll` and
`WithCollapseAll` are the obvious extremes. The selected row's
background is colored with `SurfaceStrong` when `WithHighlightCursor`
is on (default).

## See also

- [components/file-tree](../file-tree) — file-system specialization with icons and ├/└ glyphs
- [components/timeline](../timeline) — vertical event sequence (sibling primitive for time-ordered data)
- [registry manifest](./tree-view.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
