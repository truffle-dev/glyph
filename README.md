# glyph

Beautifully designed components for the terminal. Yours to copy, paste, own.

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![test](https://github.com/truffle-dev/glyph/actions/workflows/test.yml/badge.svg)](https://github.com/truffle-dev/glyph/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/truffle-dev/glyph.svg)](https://pkg.go.dev/github.com/truffle-dev/glyph)
[![Version](https://img.shields.io/badge/version-0.1.0--dev-blue)](https://github.com/truffle-dev/glyph/releases)

![glyph reel](visuals/out/reel.gif)

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
| `list` | Vertical selectable list with cursor highlight, optional hints, disabled items, and internal scrolling. |
| `progress-bar` | Determinate progress indicator with an optional label and percentage readout. Color- and glyph-tunable. |
| `key-hints` | Compact footer of key-and-description pairs. The bottom-row cheatsheet every TUI grows into. |

## Gallery

Every screenshot below is a recording of the component's own `story/` binary running in a real terminal. No mockups, no Figma, no compositing — the same Bubble Tea output you get after `glyph add <name>`.

<table>
<tr>
<td width="50%"><a href="components/theme/"><img src="visuals/out/theme.gif" alt="theme" /></a><br/><sub><b>theme</b> — Token palette every component reads from.</sub></td>
<td width="50%"><a href="components/chat-bubble/"><img src="visuals/out/chat-bubble.gif" alt="chat-bubble" /></a><br/><sub><b>chat-bubble</b> — Role-aware speech bubble with width-aware wrapping.</sub></td>
</tr>
<tr>
<td><a href="components/chat-input/"><img src="visuals/out/chat-input.gif" alt="chat-input" /></a><br/><sub><b>chat-input</b> — Single-line chat prompt with placeholder, cursor, focus.</sub></td>
<td><a href="components/chat-thread/"><img src="visuals/out/chat-thread.gif" alt="chat-thread" /></a><br/><sub><b>chat-thread</b> — Vertically scrolling conversation surface.</sub></td>
</tr>
<tr>
<td><a href="components/command-palette/"><img src="visuals/out/command-palette.gif" alt="command-palette" /></a><br/><sub><b>command-palette</b> — Filterable modal command picker.</sub></td>
<td><a href="components/markdown-viewer/"><img src="visuals/out/markdown-viewer.gif" alt="markdown-viewer" /></a><br/><sub><b>markdown-viewer</b> — Scrollable terminal markdown.</sub></td>
</tr>
<tr>
<td><a href="components/log-stream/"><img src="visuals/out/log-stream.gif" alt="log-stream" /></a><br/><sub><b>log-stream</b> — Bounded color-coded log view that tails like <code>tail -f</code>.</sub></td>
<td><a href="components/diff-view/"><img src="visuals/out/diff-view.gif" alt="diff-view" /></a><br/><sub><b>diff-view</b> — Unified-diff renderer with line numbers.</sub></td>
</tr>
<tr>
<td><a href="components/notification-toast/"><img src="visuals/out/notification-toast.gif" alt="notification-toast" /></a><br/><sub><b>notification-toast</b> — Stacked dismissible notifications.</sub></td>
<td><a href="components/status-bar/"><img src="visuals/out/status-bar.gif" alt="status-bar" /></a><br/><sub><b>status-bar</b> — Single-line three-segment status bar.</sub></td>
</tr>
<tr>
<td><a href="components/spinner/"><img src="visuals/out/spinner.gif" alt="spinner" /></a><br/><sub><b>spinner</b> — Animated single-glyph indicator. Five styles.</sub></td>
<td><a href="components/tabs/"><img src="visuals/out/tabs.gif" alt="tabs" /></a><br/><sub><b>tabs</b> — Horizontal labeled tab row primitive.</sub></td>
</tr>
<tr>
<td><a href="components/panel/"><img src="visuals/out/panel.gif" alt="panel" /></a><br/><sub><b>panel</b> — Bordered container with optional title and footer.</sub></td>
<td><a href="components/list/"><img src="visuals/out/list.gif" alt="list" /></a><br/><sub><b>list</b> — Vertical selectable list with cursor highlight.</sub></td>
</tr>
<tr>
<td><a href="components/progress-bar/"><img src="visuals/out/progress-bar.gif" alt="progress-bar" /></a><br/><sub><b>progress-bar</b> — Determinate progress indicator.</sub></td>
<td><a href="components/key-hints/"><img src="visuals/out/key-hints.gif" alt="key-hints" /></a><br/><sub><b>key-hints</b> — Compact footer of key-and-description pairs.</sub></td>
</tr>
<tr>
<td><a href="components/text-input/"><img src="visuals/out/text-input.gif" alt="text-input" /></a><br/><sub><b>text-input</b> — Multi-line text input with 2D cursor, word jumps, kill-to-cursor.</sub></td>
<td></td>
</tr>
</table>

Browse all seventeen with live demos at [truffleagent.com/glyph](https://truffleagent.com/glyph).

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

## Philosophy

The terminal is having a renaissance. Bubble Tea, ratatui, Textual, gum, lazygit, atuin, claude code — the list of TUIs people use daily is longer than it has been in a decade.

What's missing is a shared component vocabulary. Every team rewrites the chat surface, the command palette, the diff view, the status bar. Each rewrite is a little different, a little worse, and the team owes one more library upgrade for each one they import.

shadcn/ui solved this on the web by inverting the model. You don't import a library; you copy the source into your repo and own it. Updates are deliberate. Customization is direct. The library has no upgrade path because there is no library, only your code.

glyph applies the same shape to the terminal. Components are source files you copy. They reference a small theme module you also own. They have no runtime dependency on glyph. If glyph disappears, your app still works.

The rules that fall out of that bet:

1. **Copy, don't depend.** Every component is downloadable as source. No glyph runtime dependency. Delete glyph after install and your app still works.
2. **One framework at a time.** v0.1 is Bubble Tea. Adapters for ratatui, Textual, and Ink follow. We won't dilute the launch.
3. **Tokens, not hardcoded colors.** Every component references `theme.Default`. Theming a whole app is one file change.
4. **Stories are tests are screenshots.** A component without a story file doesn't ship. Stories drive the screenshot pipeline and the demo site equally.
5. **Quiet, not loud.** No emojis, no marketing phrases, no exclamation marks. Earn attention with the work.

## What's next

v0.1 ships sixteen Bubble Tea components and the CLI. The shape that follows:

- **v0.2 — form and structure.** Text input, select / dropdown, modal, confirmation dialog, code view with syntax highlighting (via chroma), table, file tree, breadcrumb, key-binding chord. The set rounds out the toolkit a config-heavy or browse-heavy TUI needs.
- **v0.3 — first non-Bubble-Tea adapter.** Likely ratatui (Rust), based on demand. The registry shape already encodes `frame`, so each component re-ships as a sibling source file with the same manifest contract.
- **v0.4 and beyond.** Textual (Python) and Ink (TypeScript). The catalog grows, the registry shape stays.

The registry contract is stable. What grows is the catalog.

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
