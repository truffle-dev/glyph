# metrics-explorer

An SRE-style services dashboard composed from **six data-and-display
components**: table-virtualized, sparkline-chart, gauge, pagination-bar,
timeline, and json-tree-view.

```bash
go run ./examples/metrics-explorer/
```

## What's on screen

| Component | Where it shows up |
| --- | --- |
| `status-bar` | top: app name, total services, current row range, cursor |
| `table-virtualized` | left: 47 services across 3 pages of 20 |
| `sparkline-chart` | inline in the p99 column of each table row |
| `pagination-bar` | below the table: page x of y, total items |
| `timeline` | right top: recent rollout events for the selected service |
| `gauge` | right middle: three SLO headroom bars for the selected service |
| `json-tree-view` | right bottom: collapsible config for the selected service |
| `key-hints` | bottom: row / page / first-last / refresh / quit |
| `theme` | every color in the layout |

The sparkline goes inside a virtualized table cell as a pre-rendered string.
The chart's `View()` returns unicode block glyphs plus a latest-value suffix
(`▁▂▄▆█ 84ms`), and table-virtualized takes that string straight into the
cell — no special integration needed. That composition is the v0.47.0
tier's quiet test of the primitive surface.

## Keys

```
↑ / ↓ (or k/j)     move the table cursor (refreshes the right panel)
[                  previous page
]                  next page
g                  jump to first page
G                  jump to last page
enter              re-snap the right panel from the cursor row
q / Ctrl-C         quit
```

## How it composes

`refreshTableRows` rebuilds the row provider for the current page slice.
For each visible service it renders a sparkline-chart with the service's
status color and pushes the result into the third cell. `refreshRightPanel`
re-points the timeline, the three gauges, and the json-tree-view at the
cursor's service — timeline gets the synthesized rollout sequence, the
gauges recompute SLO headroom (p99 as a percent of the service's declared
`slo.p99_ms`, error count out of an arbitrary 5-per-bucket budget, replicas
of a 10-pod ceiling), and jsontreeview gets a `map[string]any` config that
includes nested limits, rollout strategy, and SLO.

The gauge primitives use `WithThresholds(0.6, 0.8)` so the bar shifts
through theme `Success`, `Warning`, and `Error` colors as the reading
climbs through the range — the kind of at-a-glance signal SRE dashboards
default to without forcing the operator to read the readout.

The pagination-bar's own h/l bindings would collide with the table's
vim-style cursor, so the parent model binds `[ ]` for page navigation and
passes arrows + j/k to the table. `g`/`G` jump to first/last page,
mirroring the timeline / table-virtualized convention.

## What this demo is testing

A primitive earns its place when the wrapper built on top of it is a thin
shell. This demo wraps no internals: every component takes its declared
inputs through `With*` builders and emits its declared messages. The
composition is one model with a handful of refresh hooks and no shadowed
keymaps.

When the v0.47.0 tier ships into a real product, this is the shape the
operator gets to lift verbatim.

## Files

- `cmd_main.go` — runnable `main()`; instantiates the program in alt-screen.
- `main.go` — model, view, update, plus deterministic fleet generation.
- `main_test.go` — table+page navigation, surface rendering, cursor clamp.
