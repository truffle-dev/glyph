# nook roadmap

nook is a terminal-native code editor built entirely from glyph
components. The bet: a single static binary with the editing surface of
a modern GPU editor and an AI workflow that runs through your own
`claude` CLI, with none of the Electron weight, none of the per-seat
cost, and none of the plugin fragility. Parity target is Zed. Collab and
liveshare are explicitly out of scope.

`go install github.com/truffle-dev/glyph/cmd/nook@latest`

Per-version detail lives in the git log and `CHANGELOG.md`. This file is
forward-looking: where nook is, and what is left to reach parity. Keep it
honest — an item moves to "done" only when the package exists and its
tests are green, not when it is planned.

## Operating constraints

These are load-bearing and do not bend for a feature.

- **First paint is never blocked.** Constructors are constant-time. Any
  walk, read, or index runs in an async `tea.Cmd` behind a placeholder.
  Opening a single file must feel instant, the same as `vi file`.
- **AI runs through the user's `claude` CLI** in
  `--print --output-format stream-json` mode. No direct API calls, no
  API key in nook, no vendored model SDK. If the CLI is absent the editor
  still works; only the AI panes go quiet.
- **One binary, no runtime deps.** Syntax, LSP wiring, git, and the
  debug adapter client are all in-tree or shell out to tools the user
  already has (`gopls`, `git`, a DAP server).

## Done (v0.50.0)

The editing core and most of the language-intelligence surface are in
place. Grouped by area, each backed by a package under
`cmd/nook/internal/`:

- **Editing** — multi-line editor, soft wrap, auto-pair, bracket-match
  highlight, comment toggling, indent guides, code folding, find and
  replace, register-based clipboard.
- **Buffers & layout** — tab bar, buffer manager, multibuffer, file
  tree with file operations, fuzzy finder, picker.
- **Language intelligence (LSP)** — hover, signature help with overload
  cycling, completion, code actions, rename, find-references, call
  hierarchy, inlay hints, document outline, symbol search, semantic
  tokens, diagnostics.
- **Syntax** — chroma-backed highlighting (Go, TS/JS, Python, Rust,
  Markdown, and more).
- **Git** — gutter signs, inline blame.
- **Debug** — DAP wire client, breakpoints, F-key bindings.
- **AI** — composer, prompt creation, ghost-text suggestions, AI
  history, per-project AI rules.
- **Shell & navigation** — embedded terminal, navigation history, tasks.
- **Config & chrome** — per-project config inheritance with live reload,
  theme picker, settings, welcome card, searchable keymap overlay,
  markdown preview.

## Remaining for parity

Roughly in priority order. None of these has a package yet.

1. **Split panes.** Horizontal and vertical view splits over the same or
   different buffers. The single biggest layout gap against Zed. Has to
   respect the first-paint rule and the recursive pump pattern used for
   the existing panes.
2. **Full multi-cursor.** Add-cursor-at-next-match (ctrl+d),
   select-all-occurrences (alt+d), stack-above/below (ctrl+↑/↓),
   split-selection-into-lines (alt+i), multi-line edit at every cursor,
   and consistent placement of the primary within its own match all
   work. What remains for a first class mode: column/box cursors (drag
   or keyboard column selection).
3. **Vim mode.** Modal editing as an opt-in layer. Gated on an explicit
   product decision before build — it is a large surface and should not
   land half-done.
4. **Tree-sitter highlighting.** Upgrade the syntax backend from chroma
   to tree-sitter (via WASM) for incremental, error-tolerant parsing.
   chroma stays the fallback for grammars not yet wired.
5. **Minimap.** Optional document overview gutter. Lowest priority of the
   five; useful but not load-bearing for the editing experience.

## How to pick the next slice

Read the package list above against the repo, choose the highest item
whose absence you can confirm, and ship it surgically: one feature, its
tests green, the binary still starting instantly. Update this file when
the slice lands.
