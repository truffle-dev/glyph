# release-explorer

An interactive GitHub-style release browser composed from **six glyph
components**: list, tabs, markdown-viewer, status-bar, key-hints, and theme.

```bash
go run ./examples/release-explorer/
```

## What's on screen

| Component | Where it shows up |
| --- | --- |
| `status-bar` | top: app name, repo label, cursor / total, selected tag |
| `list` | left (36 wide): release tags with publish-date hints |
| `tabs` | right top: Body, Assets, Meta |
| `markdown-viewer` | right body: the rendered release notes |
| `key-hints` | bottom: release / tab / scroll / first-last / quit |
| `theme` | every color in the layout |

The list selects the release, the tabs route the right panel, the
markdown-viewer renders the release body, and the status-bar reports
the cursor position and selected tag. Assets and Meta are formatted
text inside the same tab frame — they read against the same surface
without needing their own primitive.

## Keys

```
↑ / ↓ (or k/j)     move the release cursor (refreshes the right panel)
← / →              switch the right tab (Body / Assets / Meta)
pgup / pgdn        scroll the body viewer (Body tab only)
g                  jump to first release
G                  jump to last release
q / Ctrl-C         quit
```

## How it composes

`refreshBody` re-points the markdown-viewer at the current release's
body. `refreshChrome` rewrites the status-bar (`release-explorer ·
truffle-dev/glyph` on the left, `cursor / total` in the center, the
release tag on the right) and the key-hints bar at the bottom. The
right panel switches its inner content based on which tab is active:
the markdown-viewer's `View()` for Body, formatted text for Assets and
Meta.

Tabs own their own `← →` bindings, the list owns `↑ ↓ j k g G`, and
the markdown-viewer owns `pgup pgdn`. The parent model routes
`pgup/pgdn` only when the Body tab is active so scrolling the body
while the Assets tab is foregrounded is a no-op rather than a hidden
state change. `q` and `ctrl+c` quit.

## What this demo is testing

A composition demo earns its place when none of the components needed
a wrapper. Everything on screen is one of the six primitives with its
declared `With*` builders and its declared `Update` method. The parent
model owns four things: the synthetic release fixture, the layout
normalizer (`joinHorizontal` + `normalize` for ANSI-aware width
padding), the tab-aware key routing, and three small panel formatters
(Body / Assets / Meta).

## Files

- `cmd_main.go` — runnable `main()`; instantiates the program in alt-screen.
- `main.go` — model, view, update, plus right-panel composition.
- `fixture.go` — 7-release synthetic dataset modeled on glyph's history.
- `main_test.go` — release selection, tab switching, surface rendering.
