# Adapters

This document is the spec for porting glyph to a TUI framework other than
Bubble Tea. It describes what an adapter must ship, how the registry is
structured, and how the CLI selects between frames.

v0.1 ships a single frame, `bubbletea`. v0.2 adds the first non-Go adapter
(target: `ratatui`). v0.3 and later add `textual` (Python) and `ink`
(TypeScript). The registry shape does not change between v0.1 and v0.2;
what changes is the catalog grows and a frame selector appears on the
demo site.

## Scope

An adapter is a per-framework reimplementation of every v0.1 component,
shipped through the same registry contract. A user who runs
`glyph init --frame ratatui` and then `glyph add chat-bubble` gets the
Rust source for a `ratatui` chat bubble, not the Go source for a Bubble
Tea chat bubble. Component names stay stable across frames so that
documentation and discussion work the same regardless of language.

What an adapter is **not**: a translation layer at runtime, a shared
component runtime, a wrapper that loads Go from Rust. The copy-paste
model means each frame is independently sourced. The contract is the
catalog, not the code.

## Contract

An adapter ships three things:

1. A registry tree at a stable URL, in the shape documented in
   `schema/registry-item.json` and `schema/glyph.json`. Every item carries
   a `frame` field whose value names the framework (`ratatui`, `textual`,
   `ink`, or a value reserved through a PR to this repo).
2. The component source files referenced by every manifest, served as
   static text under the registry URL.
3. A demo surface, ideally a sibling page at `truffleagent.com/glyph` so
   each frame has a visual gallery comparable to the Bubble Tea cards.

The CLI is intentionally not adapter-aware in code. An adapter does not
need to ship a fork of the CLI; it ships a registry URL the user points
at via `glyph init --registry`.

## Registry layout (proposed for v0.2)

For a frame named `<frame>`, the registry tree looks like:

```
r/<frame>/registry.json         catalog of items for this frame
r/<frame>/<name>.json           per-item manifest
r/<frame>/<name>/<file>         component source
```

The v0.1 Bubble Tea registry currently lives at `r/<name>.json` with no
frame prefix, because there is only one frame. When v0.2 introduces the
first non-Bubble-Tea adapter, the Bubble Tea registry moves to
`r/bubbletea/` and the plain `r/<name>.json` URLs continue to resolve
to it as permanent aliases so existing `glyph add` calls do not break.
Until that move, adapter packs can choose to host their own registries
at any URL and have consumers set `glyph.json` `registry` directly.

## Frame selection

The consumer's `glyph.json` carries the chosen frame:

```json
{
  "frame": "ratatui",
  "module": "myorg/myapp",
  "aliases": { ... },
  "theme": "default"
}
```

`glyph init --frame <frame>` sets the field. The frame value is one of
the enum entries in `schema/glyph.json` `properties.frame.enum`. Adding
a new frame requires a one-line PR to the schema.

The consumer can point at a third-party adapter pack by editing the
`registry` field of `glyph.json` after `init`, or by passing
`--registry <url>` on each `glyph add` call. The CLI fetches manifests
from `<url>/<name>.json` and sources from each manifest's `files[].url`.

A component manifest must set `frame` to the same value the consumer's
config uses. The CLI does not currently enforce equality, but the demo
site and future tooling will, and the JSON Schema's enum catches typos.

## Manifest shape (per-frame)

Every adapter writes manifests against `schema/registry-item.json`. The
fields are stable across frames; what changes are the values:

- `name` — the component name. Same across frames. `chat-bubble` is
  `chat-bubble` whether the source is Go, Rust, Python, or TypeScript.
- `frame` — the framework name. Must match the consumer's `glyph.json`.
- `type` — `glyph:component`, `glyph:lib`, `glyph:theme`, etc.
- `version` — semver for this item under this frame. Versions are
  independent across frames: the Rust chat-bubble can be at `0.2.1`
  while the Go chat-bubble is at `0.1.0`.
- `dependencies` — language-native packages. For Go: module paths with
  `@version` (`github.com/charmbracelet/lipgloss@v1.1.0`). For Rust:
  crate names with versions (`ratatui@0.26`). For Python: pip names
  with versions. For TypeScript: npm package names.
- `registryDependencies` — names of other glyph items the component
  needs (e.g. `theme`). The CLI resolves these recursively before
  installing.
- `files` — list of source files. Each entry has `path` (the URL-relative
  path inside the registry), `url` (override; usually omitted and
  defaulted), `type` (`glyph:component`, `glyph:test`, etc.), and
  `target` (an `@alias/...` path that resolves through the consumer's
  `glyph.json` aliases).

The CLI rewrites import paths inside the fetched files. For Bubble Tea
the rewrite swaps glyph's module path for the consumer's. For ratatui
the rewrite swaps Rust `use` paths. Each adapter's `add` step shares the
same `applyStyles` and rewrite hooks; the adapter only owns the source
files, not the rewrite logic. (See `cmd/glyph/rewrite.go`.)

## Component catalog (per-frame)

Every adapter ships the same baseline catalog as v0.1 Bubble Tea: theme,
chat-bubble, chat-input, chat-thread, command-palette, markdown-viewer,
log-stream, diff-view, notification-toast, status-bar, spinner, tabs,
panel, list, progress-bar, key-hints. An adapter that ships fewer is
incomplete; an adapter that ships more is fine, with the extra items
flagged in `meta.added`.

The visual fidelity bar is "same component, recognizably the same
design." The chat-bubble's role-aware coloring, the chat-thread's
arrow-key scroll, the command-palette's substring matcher — all of
these are framework-agnostic behaviors that the adapter must preserve.
Where a framework's idiom differs (Bubble Tea's `Update(msg) (Model,
Cmd)` vs ratatui's `App::on_key`), the adapter follows its own
framework's idiom and matches the behavior, not the call signature.

## Theme tokens

Every adapter ships a `theme` item whose source is the only file that
defines the palette. The token names match across frames, grouped as:

- Foundational: `Bg`, `Surface`, `SurfaceStrong`, `Border`, `BorderStrong`,
  `Text`, `TextMuted`, `TextInverse`.
- Accents: `Primary`, `PrimaryStrong`, `Accent`.
- Status: `Success`, `Warning`, `Error`, `Info`.

The adapter's components reference these tokens, not hardcoded colors.
See `components/theme/theme.go` for the canonical names and the
`Default` value.

This is the single tightest contract across the project. A user
switching frames should be able to retheme by editing one file in their
project, regardless of which language the adapter is written in.

## Submitting an adapter

The fastest path from "idea" to "adapter shipped":

1. Open an issue tagged `adapter-proposal` naming the framework and
   linking to a working `theme` + `chat-bubble` + one composite
   component (e.g. `chat-thread` or `command-palette`). The proof of
   concept is the gate, not the writeup.
2. Submit a PR to this repo that adds the frame to
   `schema/glyph.json` and seeds the `r/<frame>/` registry tree with
   the three proof-of-concept items.
3. Ship the remaining components in follow-up PRs. The catalog target
   is sixteen items, matching v0.1, before the adapter is announced.
4. Add a demo route at `truffleagent.com/glyph/<frame>` that mirrors
   the Bubble Tea gallery's structure.

Adapter authors should expect to maintain their pack. The catalog will
grow in v0.2+ (form-fields, code-view, table); adapter packs that
lag the baseline get a "lagging" badge on the demo site.

## Reserved frame values

`bubbletea`, `ratatui`, `textual`, `ink` are reserved in
`schema/glyph.json`. To reserve a new one (e.g. `tui-rs` legacy,
`prompt_toolkit` for Python alternative, `blessed` for Node legacy),
open an issue describing the proposed name, the framework's GitHub
URL, and an estimated catalog ship date. A maintainer adds it to the
enum.

## What an adapter does not need to do

- Ship its own CLI. The single `glyph` binary drives every frame.
- Run the same tests. The Bubble Tea test pattern (Bubble Tea unit
  tests + story binary) does not port across languages; each adapter
  uses its own framework's testing conventions.
- Mirror the Go-side `tools/build`. The registry is just static JSON,
  and an adapter can write its manifests by hand, by a Bun/Node script,
  or by a Python script. The validator at `tools/build` checks shape;
  the producer language is free.
- Match the Go-side `version` field semantics exactly. Independent
  semver per frame is the contract; whether the adapter starts at
  `0.1.0` or `0.2.0` is the adapter author's call.

## Reference

- `schema/registry-item.json` — item manifest schema.
- `schema/glyph.json` — consumer config schema.
- `cmd/glyph/registry.go` — Go types mirroring the manifest.
- `cmd/glyph/add.go` — installer logic. The behavior is adapter-agnostic.
- `tools/build/` — Bubble Tea registry generator. Reference; not required
  for other adapters.
- `r/registry.json`, `r/chat-bubble.json` — canonical examples.

The contract is the catalog. The catalog is the registry. The registry
is static JSON files plus their referenced source files served from a
CDN. There is nothing else under the hood.
