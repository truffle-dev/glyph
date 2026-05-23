# log-viewer

A journalctl-style live log viewer composed from **nine** glyph components.
A steady stream of log entries flows across four sources (`server`, `auth`,
`ratelim`, `db`); the operator filters by level, source, and substring, and
can pause and resume the feed.

```bash
go run ./examples/log-viewer/
```

## What's on screen

| Component | Where it shows up |
| --- | --- |
| `status-bar` | top: live/paused mode, active filter, entry count |
| `tabs` | level filter row: All / Info+ / Warn+ / Error+ |
| `log-stream` | middle: scrollable timestamped log feed |
| `key-hints` | bottom: current-mode key bindings |
| `notification-toast` | floating tray, top-right |
| `theme` | every color in the layout |

## Overlays opened on demand

| Key | Overlay | Components used |
| --- | --- | --- |
| `Ctrl-F` | source picker | `select` |
| `/` | substring search prompt | `panel` + `text-input` |

## Keys

```
Tab          cycle level filter (All → Info+ → Warn+ → Error+)
Shift-Tab    cycle backwards
Ctrl-F       open source picker (filter by component)
/            open substring search prompt
Space        pause / resume the live feed
Ctrl-L       clear the buffer
↑ / ↓        scroll the log
q / Ctrl-C   quit
```

## How it composes

Each user action toggles state on the model and (when applicable) calls
`rebuildStream()`, which clears the visible `log-stream` and re-appends only
the entries matching the current level + source + query filter. The raw
buffer (`m.all`) keeps every entry that ever ticked in, capped at 2000.

The pause behavior is a clean Bubble Tea idiom: the tick command keeps
firing on a `tea.Tick` schedule, but the `tickMsg` branch in `Update` checks
`!m.paused` before synthesizing new entries.

## Tests

```bash
go test ./examples/log-viewer/
```

The test file exercises every binding headlessly: initial render, tick
appending, pause, level cycling, source filter, substring search, clear,
and quit.
