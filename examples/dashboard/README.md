# dashboard

An "engagements" control room composed from **nine** glyph components.
Three tabs (Engagements, Throughput, Revenue) swap a four-card metric
strip and a sortable table; a floating toast tray reports row opens; a
filter modal wraps a text input on demand.

```bash
go run ./examples/dashboard/
```

## What's on screen

| Component | Where it shows up |
| --- | --- |
| `tabs` | top: Engagements / Throughput / Revenue |
| `stat-card` | row of four metric tiles below the tabs |
| `table` | center: sortable, scrollable engagement / throughput / revenue rows |
| `status-bar` | bottom: live mode, version, identity, alert count |
| `key-hints` | row above the status-bar: current-mode key bindings |
| `notification-toast` | floating tray, top-right |
| `theme` | every color in the layout |

## Overlays opened on demand

| Key | Overlay | Components used |
| --- | --- | --- |
| `/` | substring filter prompt | `modal` + `text-input` |

## Keys

```
Tab / Shift-Tab    cycle Engagements / Throughput / Revenue
↑ / ↓ (or k/j)     move the table cursor
← / → (or h/l)     move the table's active sort column
s                  toggle sort direction on the active column
Enter              "open" the selected row (fires a toast)
/                  open the filter prompt
Esc                close the filter prompt
q / Ctrl-C         quit
```

## How it composes

`switchTab` is the spine. On every tab change it rebuilds three things
in lock-step: the four cards, the table columns, and the table rows. The
underlying `engagement` slice stays put; the view derives from the active
tab plus the filter query.

The filter modal is a `modal.Modal` whose body is the `textinput.Input`
view. Entering modal mode focuses the input and routes key events to it
until Enter (apply) or Esc (dismiss). Apply rebuilds only the Engagements
table; Throughput and Revenue ignore the query.

Toasts auto-expire after 4 seconds. A 1Hz tick command lives in
`tickToasts()` and drives `tray.Tick`; the tick message carries its own
timestamp so headless tests can advance the clock without sleeping.

## Tests

```bash
go test ./examples/dashboard/
```

The test file exercises every binding headlessly: initial render, tab
cycling forward and backward, tab-specific card and column changes,
toast push and expiry, filter open / close / apply, table cursor
movement, and quit-from-any-mode.
