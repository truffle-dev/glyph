# file-explorer

An IDE-style two-pane navigator composed from **seven** glyph components.
The left pane is a tree of the project; the right pane is a syntax-tinted
code-view of whichever leaf the cursor is on, with a breadcrumb above and
a status-bar plus key-hints pinned to the bottom.

```bash
go run ./examples/file-explorer/
```

## What's on screen

| Component | Where it shows up |
| --- | --- |
| `panel` | the two big bordered surfaces that frame both panes |
| `file-tree` | left pane: dirs and leaves, with cursor + expand state |
| `breadcrumb` | top of the right pane: path of the open file |
| `code-view` | right pane: source with gutter, syntax tint, and a highlighted focus line |
| `status-bar` | bottom: ● ready, current path, language, line label |
| `key-hints` | row below the status-bar: current-mode key bindings |
| `theme` | every color in the layout |

## Keys

```
↑ / ↓ (or k/j)    move the tree cursor
→ / l             expand directory
← / h             collapse directory (or jump to parent on a leaf)
Enter             toggle expand on a dir, "open" on a leaf
q / Ctrl-C        quit
```

## How it composes

`syncPreview` is the spine. Every time the tree cursor moves, the model
calls `tree.Selected()` to get the current path, looks it up in the
fixture file map, and swaps three things in lock-step: the right pane's
panel title, the breadcrumb, and the code-view body. The status-bar's
center and right slots are patched too — the center renders the path as
`project / cmd / main.go` and the right slot shows the language plus the
focus line, both via the `statusbar.Item` API.

Both panels use `panel.VariantStrong` so their borders stay visible on
darker terminals like monokai. The left pane is a fixed 34 columns; the
right pane consumes whatever's left after a one-cell gap.

The fixture is inline. `seedTree` returns the directory shape; `seedFiles`
returns a `map[string]fileEntry` keyed by the path the tree exposes via
`Selected()`. Each entry carries the source, the language, and a 1-based
`FocusLine` that the code-view highlights with `codeview.MarkHighlight`.

## Tests

```bash
go test ./examples/file-explorer/
```

The test file exercises the full pipeline headlessly: initial render
content, the starting cursor, that pressing `↓` advances the cursor *and*
syncs the preview pane, that `q` and `Ctrl-C` both quit, that every leaf
in the tree has a matching `seedFiles` entry, that pressing `→` over a
collapsed directory makes its children appear, and the two small helpers
(`prettyCrumbs`, `itoa`) round-trip correctly.
