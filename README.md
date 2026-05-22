# glyph

Beautifully designed components for the terminal. Yours to copy, paste, own.

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/truffle-dev/glyph.svg)](https://pkg.go.dev/github.com/truffle-dev/glyph)
[![Version](https://img.shields.io/badge/version-0.1.0--dev-blue)](https://github.com/truffle-dev/glyph/releases)

A copy-paste component library for terminal UIs. Install the CLI, run `glyph add chat-thread`, and a chat surface drops into your repo as plain Go source you own. No glyph runtime dependency. No version pinning. No magic.

The model is shadcn/ui, ported to the terminal. The components are built for [Bubble Tea](https://github.com/charmbracelet/bubbletea) in v0.1. Adapters for ratatui, Textual, and Ink follow.

## Install

```bash
go install github.com/truffle-dev/glyph/cmd/glyph@latest
cd path/to/your/project
glyph init
glyph add chat-thread
```

That's the whole onboarding. The third command writes Go files into `internal/ui/` (or wherever your `glyph.json` aliases say) and runs `go get` for the upstream libraries the component needs.

## Components (v0.1)

| Component | Description |
|---|---|
| `theme` | Token palette every component reads from. Edit one file to retheme an entire app. |
| `chat-bubble` | Role-aware speech bubble with width-aware wrapping. user / assistant / system / tool. |
| `chat-input` | Single-line chat prompt with placeholder, cursor, focus state, submit and cancel bindings. |
| `chat-thread` | Vertically scrolling conversation surface. Composes `chat-bubble`. Arrow keys, PgUp/PgDn, Home/End. |
| `command-palette` | Filterable modal command picker. Substring matcher by default; swap in your own. |
| `markdown-viewer` | Scrollable terminal markdown. Headings, paragraphs, bullets, blockquotes, code, links. |
| `log-stream` | Bounded color-coded log view that tails like `tail -f`. Level filter, capacity ring. |
| `diff-view` | Unified-diff renderer with line numbers, color-coded additions and removals. Ships with a `ParseUnified` helper. |
| `notification-toast` | Stacked dismissible notifications with level-aware coloring and per-toast TTLs. |
| `status-bar` | Single-line three-segment status bar. Left fills from left, right anchors right, truncates left first under pressure. |
| `spinner` | Animated single-glyph indicator with an optional label. Five styles: dots, line, arc, pulse, bounce. |
| `tabs` | Horizontal labeled tab row primitive. Arrow keys or Tab cycle with wrap. Parent owns the panels below. |
| `panel` | Bordered container with optional title and footer. The workhorse layout primitive: wrap any view in one. |

## How it works

Glyph is two things: a CLI and a static registry.

The CLI reads a local `glyph.json` in your repo and resolves paths via aliases:

```json
{
  "$schema": "https://truffleagent.com/glyph/schema/glyph.json",
  "frame": "bubbletea",
  "module": "github.com/your-org/your-app",
  "aliases": {
    "components": "internal/ui",
    "lib": "internal/uilib",
    "hooks": "internal/uihooks"
  },
  "theme": "default",
  "registry": "https://truffleagent.com/glyph/r"
}
```

`glyph add chat-thread` fetches `truffleagent.com/glyph/r/chat-thread.json`, walks the dependency graph (chat-thread depends on chat-bubble which depends on theme), and writes each file into the alias-resolved path. Import paths are rewritten so the files reference your module, not glyph's.

After install, the files are yours. Edit them. Refactor them. Delete the prompt prefix in `chat-input.go` and replace it with your project's logo. The library has no opinion.

## Design principles

1. **Copy, don't depend.** Every component is downloadable as source. No glyph runtime dependency. Delete glyph after install and your app still works.
2. **One framework at a time.** v0.1 is Bubble Tea. Adapters for ratatui, Textual, and Ink follow. We won't dilute the launch.
3. **Tokens, not hardcoded colors.** Every component references `theme.Default`. Theming a whole app is one file change.
4. **Stories are tests are screenshots.** A component without a story file doesn't ship. Stories drive the screenshot pipeline and the demo site equally.
5. **Quiet, not loud.** No emojis, no marketing phrases, no exclamation marks. Earn attention with the work.

## Demo site

[truffleagent.com/glyph](https://truffleagent.com/glyph) browses every component with a live SVG preview, the install command, the full source, and the JSON manifest.

## Run the showcase locally

`examples/showcase/` is a single-binary TUI demo: five tabs (Chat, Commands, Markdown, Logs, Diff), a status bar, and a toast overlay. The tab primitive, spinner, and panel each ship with their own runnable story under `components/<name>/story/`.

```bash
go run ./examples/showcase
```

Tab cycles tabs forward, Shift-Tab cycles back. On any non-chat tab, `t` fires a toast and `l` appends a log entry. `q` or Ctrl-C quits.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). The fastest first contribution is a new component: copy `components/chat-bubble/` as a template, replace the body, add a story file, and open a PR.

## Credits

The shape of glyph is borrowed from [shadcn/ui](https://ui.shadcn.com), which solved this distribution problem for React. The terminal needed the same answer.

Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) by Charm.

## License

[MIT](LICENSE)
