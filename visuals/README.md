# Visual recordings

Sources for the README gallery and the headline reel. The per-component
GIFs in `out/` ship in-repo so the gallery resolves on github.com; the
casts are intermediate and regenerated locally.

## Gallery (per-component GIFs)

Each `components/<name>/story/` directory has either an interactive
`main.go` (build tag `glyph_story`) or a non-interactive `snap.go`
(build tag `glyph_snap`). The renderer prefers `snap.go` when present:
its single-shot `View()` is what we want for a static snapshot.

```bash
bash visuals/render-cast.sh
```

The script walks `components/*/story/`, builds the binary with the
right tag, records with asciinema, and renders to GIF with agg. The
output lands in `visuals/out/<name>.gif` and is committed.

Two non-obvious choices live in `render-cast.sh`:

- `TERM=screen-256color` is passed to asciinema. termenv skips the
  OSC 11 background-color query for screen/tmux/dumb TERMs, which
  matters because any binary that imports `bubbletea` triggers that
  query at package init via `lipgloss.HasDarkBackground()`. Inside a
  recorder that has no real terminal to answer, the query stalls
  for `OSCTimeout` (≈5 s) before the first frame paints — long
  enough that agg renders blank GIFs for short clips.
- `GLYPH_COLS`, `GLYPH_ROWS`, `GLYPH_THEME`, `GLYPH_FONT_SIZE`
  environment variables override the defaults (96 × 32, monokai, 18px).

Interactive stories that never exit on their own should add a
`snap.go` next to `main.go` so the gallery captures a clean frame
instead of the alt-screen state.

## Reel

The headline demo is `examples/reel/`, a self-playing TUI that walks
every component. It exits on its own, so it records straight from
the story binary:

```bash
go build -o /tmp/reel ./examples/reel/
TERM=screen-256color asciinema rec \
  --cols 100 --rows 30 --command /tmp/reel \
  visuals/casts/reel.cast
agg --theme monokai --font-size 18 \
  visuals/casts/reel.cast visuals/out/reel.gif
```

## Why asciinema + agg, not vhs

vhs is Charm's tape-based renderer and would be the natural fit, but
it spawns a headless Chrome to capture frames and that Chrome isn't
always installable on the recording host. asciinema (Python) plus agg
(Rust, single-binary) cover the same ground without a browser dep.
The two pipelines are interchangeable; tapes under `visuals/tapes/`
work with `vhs <tape>` if you have it.
