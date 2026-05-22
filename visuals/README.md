# Visual recordings

Source-of-truth for glyph's marketing demos lives here. Rendered output
(`.cast`, `.gif`, `.svg`) is regenerated from these sources and is not
committed; see `.gitignore`.

## Tapes

`tapes/<name>.tape` is a [vhs](https://github.com/charmbracelet/vhs)
script. Each tape produces a `.gif` in `out/`.

```bash
vhs visuals/tapes/spinner.tape   # writes visuals/out/spinner.gif
```

## Reel

The headline demo is `examples/reel/`, a 30-second self-playing TUI that
walks every component the library ships. Record it with asciinema and
render to a GIF with [agg](https://github.com/asciinema/agg):

```bash
go build -o /tmp/reel ./examples/reel/
asciinema rec --cols 100 --rows 30 --command /tmp/reel visuals/casts/reel.cast
agg --theme monokai --font-size 18 visuals/casts/reel.cast visuals/out/reel.gif
```

The reel exits on its own. The recorded final frame is the diff scene —
not an empty buffer — by design.
