# Accordion

> Vertical stack of titled, collapsible sections with a focused-cursor
> model. Single-expanded by default — opening a new section closes the
> previous one — or independent via `WithAllowMultiple(true)`. The same
> component renders a settings panel, a per-error stack-trace browser,
> or a release-notes summary; what changes is the section data, not the
> component.

## Install

```bash
glyph add accordion
```

This copies `accordion.go` (and its test file) into your repo at the
path your `glyph.json` aliases declare, along with the sibling `theme`
dependency. After install, the files are yours: edit them, refactor
them, rename them.

## Hello, world

```go
package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/accordion"
)

func main() {
	m := accordion.New().
		WithSections(
			accordion.Section{
				Title: "Overview",
				Body:  "Phantom is a persistent agent substrate.\nIt runs Truffle and similar daemons.",
				Value: "ov",
			},
			accordion.Section{
				Title: "Wire format",
				Body:  "Deterministic CBOR over QUIC, content-addressed blobs.",
				Value: "wf",
			},
			accordion.Section{
				Title: "Storage",
				Body:  "Local sled, indexes derived from the blob hash.",
				Value: "st",
			},
		).
		WithExpanded(0).
		WithSize(60, 12)
	fmt.Println(m.View())
}
```

## API surface

Package: `accordion`

**Types**

- `Section`
- `Model`
- `SelectMsg`

**Functions and methods**

- `New`
- `WithTheme`
- `WithSections`
- `WithAllowMultiple`
- `WithFocused`
- `WithExpanded`
- `WithExpandAll`
- `WithCollapseAll`
- `WithSize`
- `WithHighlightCursor`
- `WithPlaceholder`
- `Focused`
- `FocusedSection`
- `IsExpanded`
- `ExpandedIndices`
- `Sections`
- `Init`
- `Update`
- `View`

## Keys

| Key                 | Action                                        |
|---------------------|-----------------------------------------------|
| `up` / `k`          | Move focus to the previous section (wraps).   |
| `down` / `j`        | Move focus to the next section (wraps).       |
| `tab` / `shift+tab` | Same as down / up.                            |
| `home` / `g`        | Jump to the first section.                    |
| `end` / `G`         | Jump to the last section.                     |
| `enter`             | Toggle the focused section; emit `SelectMsg`. |
| `space`             | Toggle the focused section silently.          |
| `right` / `l`       | Expand the focused section if collapsed.      |
| `left` / `h`        | Collapse the focused section if expanded.     |

## Dependencies

- glyph component `theme` (installed automatically)
- `github.com/charmbracelet/bubbletea@v1.3.10`
- `github.com/charmbracelet/lipgloss@v1.1.0`

## Notes

In single-expanded mode (the default), opening any section closes every
other section. `WithAllowMultiple(true)` makes each section's expanded
state independent. Switching from independent back to single keeps the
focused section open if it was expanded; otherwise it keeps the first
expanded section it finds. `WithExpandAll` opens every section in
multiple mode and falls back to opening the last section in single mode
(the same shape the user would land on after expanding through the
list). `WithSize(w, h)` clips both axes; the offset scrolls so the
focused header stays visible when the total row count exceeds the
height. The body of an expanded section renders two spaces in from the
header line, one rendered line per `\n`-separated body line.

## See also

- [components/tree-view](../tree-view) — the recursive collapsible-tree primitive accordion's degenerate single-level cousin
- [components/tabs](../tabs) — single-active selection across a horizontal row instead of a vertical stack
- [registry manifest](./accordion.json) — the JSON contract `glyph add` reads

## License

MIT, same as the rest of glyph.
