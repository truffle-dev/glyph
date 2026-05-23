# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `text-input` component. Multi-line input with placeholder, focus, 2D cursor,
  Alt+Left/Right word jumps, Ctrl-U kill-to-cursor, Ctrl-K kill-to-end-of-line,
  and `Enter` for newline / `Ctrl-D` for accept. Pairs with `panel` for a
  labeled commit-message surface.
- `select` component. Bounded single-choice popover with optional substring
  typeahead, scroll window, hint column, and inlaid title. Emits
  `selectinput.SelectMsg{Option, Index}` on commit and
  `selectinput.CancelMsg` on Esc.
- `modal` component. Border-with-title overlay container with body, footer,
  and a configurable close key. Pairs with `lipgloss.Place` to position over a
  parent view; emits `modal.CloseMsg` on Esc by default.
- `confirmation` component. Two-button yes/no prompt with focus-managed
  buttons, single-keystroke y/n shortcuts, dangerous-action styling, and
  prompt reflow via `muesli/reflow`. Emits `confirmation.ConfirmMsg{Value}`
  on commit, `confirmation.CancelMsg` on Esc.
- `kbd` component. Stateless keycap atom that renders a single key or a chord
  as Unicode-cap glyphs (`ctrl+k` → `⌃ + K`, `enter` → `⏎`, `up` → `↑`).
  Exposes `Render`, `RenderStyled`, `Chord`, and `Sequence`. No `Model`,
  no `Update`. Use inside hint rows, command palettes, and modals.
- `table` component. Sortable data grid with column alignment, numeric-aware
  sort, cursor highlight, optional row selection, PgUp/PgDn/Home/End,
  arrow-key column nav, and `s` to toggle sort. Emits `table.SelectMsg`,
  `table.SortMsg`, and `table.CursorMsg`.
- `stat-card` component. Dashboard metric tile with label, value, trend
  glyph (`▲`/`▼`/`—`), delta, sublabel, and optional emphasis treatment that
  swaps the border + surface tokens.
- `examples/chat-cli`, a single-binary agent-style REPL composing **thirteen**
  components into one chat surface: `status-bar`, `chat-thread`, `chat-bubble`,
  `chat-input`, `key-hints`, `notification-toast`, `spinner`, `command-palette`,
  `modal`, `text-input`, `confirmation`, `select`, and `theme`. Headless tests
  exercise every overlay and message-routing path.
- `examples/log-viewer`, a journalctl-style live feed composing **nine**
  components: `log-stream`, `tabs`, `status-bar`, `key-hints`,
  `notification-toast`, `panel`, `text-input`, `select`, and `theme`. Filters
  by level, source, and substring; pause/resume the live tick; clear the
  buffer. Headless tests exercise every binding.
- `examples/dashboard`, an engagements control room composing **nine**
  components: `tabs`, `stat-card`, `table`, `text-input`, `modal`,
  `status-bar`, `key-hints`, `notification-toast`, and `theme`. Three tabs
  swap the card row and table columns in lock-step; a filter modal wraps a
  text input on demand; toast tray fires on row open. Headless tests exercise
  every binding.

## [0.1.1] — 2026-05-22

Patch release. No CLI or component code changed; this release exercises the
new goreleaser pipeline so binaries are attached to the GitHub release for
the first time, and rounds out the docs surface.

### Added

- Pre-built binaries for linux, macos, and windows on amd64 and arm64,
  attached to the GitHub release as tarballs (or zip on windows), with a
  `checksums.txt` covering every archive.
- `goreleaser` config and a tag-triggered release workflow.
- Per-component `README.md` in every `components/<name>/` dir: preview GIF,
  install command, hello-world snippet matching the landing-page card, API
  surface, dependencies, and notes pulled from the JSON manifest.
- `glyph.schema.json` describing the consumer-side `glyph.json` config.
- `SECURITY.md` with a vulnerability disclosure policy.
- Dependabot config for weekly Go-module and GitHub-Actions updates.

### Changed

- `cmd/glyph`: `version` is now a mutable `var` so goreleaser can inject the
  release tag via `-ldflags "-X main.version=..."`. Source builds keep the
  `-dev` suffix.

## [0.1.0] — 2026-05-22

The first public release. Sixteen Bubble Tea components, a CLI, a static
registry, and a demo site.

### Added

- `cmd/glyph` CLI with `init`, `add`, `list`, and `version` subcommands.
- Static registry under `r/` describing every component as a JSON manifest
  with file list, dependencies, and import-rewrite rules.
- Sixteen v0.1 components, each with a runnable `story/` example and tests:
  `theme`, `chat-bubble`, `chat-input`, `chat-thread`, `command-palette`,
  `markdown-viewer`, `log-stream`, `diff-view`, `notification-toast`,
  `status-bar`, `spinner`, `tabs`, `panel`, `list`, `progress-bar`,
  `key-hints`.
- `examples/showcase`, a single-binary TUI demo composing the seven main
  surfaces into one application with tabs, a status bar, and a toast tray.
- `examples/reel`, a recorder-driven self-playing reel binary that produces
  `visuals/out/reel.gif` for the README and landing page.
- Test, vet, lint, and build CI across ubuntu, macos, and windows on every
  push and pull request.
- Issue templates for bug reports and component requests. PR template.
  `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`, `LICENSE` (MIT).
- Per-component animated GIFs under `visuals/out/<name>.gif` and a Gallery
  section in the README.
- Dependabot configuration for weekly Go-module and GitHub-Actions updates.

### Notes

- `glyph -version` and `glyph --version` both work, alongside `glyph -v` and
  `glyph version`. The CLI embeds VCS info via `runtime/debug.ReadBuildInfo`
  so `go install` builds carry a commit SHA in the version string.
- The registry contract is stable as of v0.1.0. The catalog grows; the
  shape of `r/<component>.json` does not break.

[Unreleased]: https://github.com/truffle-dev/glyph/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/truffle-dev/glyph/releases/tag/v0.1.1
[0.1.0]: https://github.com/truffle-dev/glyph/releases/tag/v0.1.0
