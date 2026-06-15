# nook — roadmap and shipped ledger

This is the living record of what nook has shipped and what the open
candidates are. It lives in the repo on purpose: the previous roadmap
lived in an out-of-tree working directory and was lost when the host
was rebuilt. Anything load-bearing for nook's direction belongs here,
under version control, where a rebuild cannot erase it.

`spec.md` is the original design intent (Phase 1/2/3, architecture, the
build order for the day-1 binary). It is frozen as a design document.
This file tracks reality against it.

## Where nook is now

- Latest release: **v0.48.0** (2026-06-11). Tags run v0.1.0 → v0.48.0.
- `cmd/nook/internal/`: 53 panes/packages.
- `components/`: 38 glyph components (1 unreleased: `toggle`).
- Binary builds via goreleaser on every `v*` tag (13 assets).

nook is well past the spec's Phase 1 ("vim with picker + search + git +
terminal") and most of Phase 2 (LSP, AI panel, multibuffer). The work
now is depth and Zed-parity polish, not bring-up.

## Shipped ledger

Grouped by arc. One line per capability; the tag is where it landed.

### Phase 1 — the usable binary (v0.1–v0.29)

- Host model, layout, focus routing, keymaps.
- Picker (fuzzy file + buffer), ripgrep search pane, git pane
  (status/stage/commit/diff), PTY terminal pane.
- Editor pane with selection, clipboard, multi-cursor arc.

### Phase 2 — LSP + AI + structure (v0.30–v0.33)

- DAP debug client: breakpoints, F-key bindings, status dbg segment
  (v0.30.0). Debug adapter is shipped, not dropped.
- Signature help overlay (v0.30.0).
- Selection + register-first clipboard, VSCode empty-line semantics
  (v0.33.0).

### Depth tier (v0.34–v0.48)

- v0.34.0 — project-wide find/replace.
- v0.35.0 — lightning startup: async file-tree build (no first-paint I/O).
- v0.36.0 — LSP document highlights (`dochi`).
- v0.37.0 — call hierarchy.
- v0.38.0 — live config watch + theme hot-reload (`configwatch`).
- v0.39.0 — comment toggle (`comment`).
- v0.40.0 — auto-pair brackets/quotes (`autopair`).
- v0.41.0 — signature-help overload cycling.
- v0.42.0 — per-project config inheritance (`.nook/config.toml` merge).
- v0.43.0 — indent guides.
- v0.44.0 / v0.44.1 — file operations in the tree (create/rename/delete,
  `O_EXCL` create).
- v0.45.0 — soft wrap (`WrapPoints`, `LineRowCount`).
- v0.46.0 — bracket-pair highlight (`Match`).
- v0.47.0 — data-and-display component tier: sparkline-chart,
  pagination-bar, accordion, json-tree-view, tree-view, timeline,
  table-virtualized.
- v0.48.0 — value-in-a-range pair: gauge (read) + range-slider (edit),
  metrics-explorer + release-explorer examples.

### Unreleased on dev

- `components/toggle` — boolean switch, completing the form-input family
  (text-input / select / range-slider / toggle). Carries its own
  CHANGELOG entry, README, and manifest. Folds into the next release;
  re-sync the registry when it ships.

## Standing constraints (do not regress)

These are operator-stated DNA. They override convenience.

1. **Nothing blocks nook's first paint.** Constructors are constant-time.
   Any directory walk, sync I/O, or heavy init before the first frame is
   a regression. Use a placeholder plus an async `tea.Cmd` (the v0.35.0
   lazy-filetree pattern; the LSP/git/blame async pattern). nook must
   open a single file as fast as a vim invocation.
2. **A new `cmd/<x>` binary needs its goreleaser entry in the same
   commit.** Otherwise the tag ships without that binary.
3. **No Anthropic API key, ever.** AI wedges shell out to the local
   `claude` CLI in `--print --output-format stream-json` mode. No
   `@anthropic-ai/sdk`, no direct `api.anthropic.com` calls, no separate
   key. Tests stay hermetic via a stub shell binary.
4. **Components carry their own CHANGELOG entry at commit time.** Two
   components in v0.48.0 had to backfill entries into the release commit
   because they didn't; toggle did it right.

## Release ritual (reusable)

1. Release commit updating `CHANGELOG.md` (move `[Unreleased]` into the
   new version section).
2. Gates in order: `gofmt -l .` clean, CI green on the release commit
   **before** tagging.
3. SSH-signed `v*` tag. goreleaser fires on the tag.
4. Watch goreleaser to green; count the release assets (expect 13).
5. Re-sync the registry: `go run ./tools/build -src components -out r`,
   copy into truffleagent-site `public/glyph/r/`, deploy. The registry
   is not auto-deployed on glyph pushes.

## Candidate next work

These are open candidates against Zed parity, not a committed sprint.
The parity target is Zed; collab/liveshare is explicitly out of scope.
Most of the original wave plan (tabs, find/replace, file tree, code
actions, rename, git gutter, multibuffer, inlay hints) is already
shipped, so the remaining surface is narrower than it once was.

- **vim mode** — repeatedly raised, repeatedly deferred pending an
  explicit operator decision. nook is positioned partly as a fast
  single-file editor (a vim replacement for editing a dotfile), so a
  real modal layer is the largest open question. Needs operator sign-off
  before building, not a unilateral start.
- **Settings/themes surface** — config inheritance and theme hot-reload
  ship; a discoverable settings UI on top does not yet.
- **Depth passes on shipped panes** — completion ranking, diagnostics
  presentation, multibuffer ergonomics. Each is a self-contained slice.

Pick the smallest finishable slice that advances parity; one capability,
its tests, its CHANGELOG line, one commit.

## Autonomous engine status

The `nook-evolve` cron that once advanced this roadmap hourly is **not
present** in the scheduler (verified 2026-06-11 and 2026-06-15; the other
~20 jobs survive, so this was selective, not a blanket loss). Until an
operator decision restores it, nook advances by hand during clean
heartbeat hours. The open question to the operator: revive the cron
pointed at this in-repo roadmap, or keep hand-driving. This file is the
durable target either path needs.
