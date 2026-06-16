# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- nook's workspace diagnostics panel can now filter by severity. Pressing
  `f` in the focused panel cycles the threshold: all → errors only →
  errors + warnings → all. A derived `shown` list drives navigation,
  `Selected`, and rendering, so the cursor only ever lands on a visible
  row; `Count` stays the workspace total while the new `Shown` reports the
  filtered count. The header trails the active label (`workspace
  diagnostics  (errors only)`) and the empty state explains when a filter,
  not an empty workspace, is hiding rows. Matches the severity toggle Zed
  and VSCode put on their problems panels.
- nook's multibuffer can now jump file-to-file. `]` moves the cursor to
  the next fragment header and `[` to the previous one, so reviewing a
  large diff no longer means holding the arrow key through every line of
  one file to reach the next. From the middle of a section the upward jump
  lands on that section's own header and the downward jump on the next
  section's, the same `[[` / `]]` asymmetry vim uses. Matches Zed's
  excerpt-to-excerpt navigation in its multibuffer.

### Fixed

- nook LSP completions now honor the server's ordering. `Completion` in
  `cmd/nook/internal/lsp` captures each item's `SortText` and `Preselect`
  and sorts before returning: preselect wins outright, then items order by
  `SortText` with `Label` as the fallback key when a server omits it, and
  ties stay stable in server order. gopls packs relevance into `SortText`,
  so the popup now lands on the most relevant candidate instead of whatever
  order the wire happened to deliver. Matches the LSP spec's client-sort
  requirement that VSCode and Zed both follow.
- nook's workspace diagnostics panel now shows the diagnostic code, not
  just the source. `collectDiagnosticEntries` was dropping the LSP `Code`
  field on the floor, so a `[gopls]` row never told you *which* check
  fired. `diagnostics.Entry` gains a `Code`, a `CodeString` helper converts
  the spec's `int32 | string` wire value, and rows now render `[gopls:
  SA1019]` / `[rustc: E0277]` — the provenance label degrades cleanly to
  source-only or code-only when a server sends just one. Matches how Zed
  and VSCode surface the code so it can be looked up or suppressed.

## [0.50.0] — 2026-06-16

The status primitive. A badge is the small colored pill that marks a
row `LIVE`, a build `PASS`, or a job `WARN`. The display family already
had `stat-card` (a number with a trend) and `status-bar` (a full row of
segments), but nothing for the atom in between: a single labelled chip
that carries one piece of semantic state. `badge` fills that gap, and
it sits next to `kbd` as the library's second stateless display
primitive.

### Added

- `components/badge` — value-based stateless status pill. Six semantic
  variants (`Neutral`, `Primary`, `Success`, `Warning`, `Error`,
  `Info`) selected by convenience setters or `WithVariant`, each mapped
  to the matching theme accent. `Filled` paints the accent as the
  background, `Outline` draws a rounded border in the accent instead;
  `Uppercase` folds the label. `Neutral` is special-cased to a quiet
  surface chip so a default badge stays calm. An empty label renders to
  `""`, so a conditional badge needs no call-site guard. Like `kbd`, it
  is a package-level immutable builder, not a `tea.Model`: theme-aware,
  no state, one `Render()` call. Sits between `stat-card` and
  `status-bar` in the display family.

## [0.49.0] — 2026-06-16

The boolean primitive that completes the form-input family. Until now
the family covered `text-input` (a string), `select` (one-of-N), and
`range-slider` (a number), but had no single-line control for a yes/no
value. `toggle` fills that gap so a settings surface built from glyph
no longer has to fake a boolean with a two-item select.

### Added

- `components/toggle` — single-line boolean switch with keyboard
  control: space and enter flip the state, left/h/n force off,
  right/l/y force on. The knob sits on the right and recolors to the
  theme Success accent when on, on the left and muted when off.
  Optional leading label and custom on/off captions; `WithDisabled`
  renders muted and drops keys. `ToggleChangedMsg` fires only on
  actual changes. Completes the form-input family alongside
  `text-input` (string), `select` (one-of-N), and `range-slider`
  (numeric) with the missing boolean primitive.

## [0.48.0] — 2026-06-11

The value-in-a-range pair: `gauge` reads a measurement, `range-slider`
edits one. The gauge is a stateless render of where a reading sits in
`[min, max]` with threshold-zone color transitions; the slider owns the
value, the bounds, the step, and the keyboard. They land together so the
read-only and interactive shapes of the same question ship in the same
release. Two composition examples — `metrics-explorer` and
`release-explorer` — exercise the v0.47.0 data-and-display tier end to
end, and the README documents the `go install` PATH step for
non-default `GOPATH` setups.

### Added

- `components/gauge` — read-only horizontal bar showing where a numeric
  value sits inside a `[min, max]` range: CPU usage, disk capacity,
  signal strength, queue depth. Optional threshold zones recolor the
  fill as the reading crosses configured boundaries; `WithLabel` adds a
  muted prefix and the numeric readout takes a units suffix. Stateless
  with no `Update` — pair with a parent model that recomputes `Value`
  on every tick. Differs from `progress-bar` (task progress as a 0–1
  ratio with a single fill color) and from `range-slider`
  (interactive): reach for gauge when the value is a measurement in a
  range, not a percentage of completion.
- `components/range-slider` — single-line horizontal slider over a
  continuous numeric range with keyboard navigation. Owns the current
  value, the `[min, max]` bounds, the step size, and the keys:
  `left`/`h` and `right`/`l` step by one, `home`/`g` and `end`/`G` jump
  to the bounds, `pgup`/`K` and `pgdown`/`J` step by 10×. Motion clamps
  at both ends; each motion that changes the value emits
  `ValueChangedMsg`. `WithPrecision` and `WithFormatter` control the
  readout, `WithShowValue` toggles it, and `WithDisabled` freezes the
  bar and mutes the track.
- `examples/metrics-explorer` — SRE-style services dashboard composing the
  v0.47.0 data-and-display tier: a `table-virtualized` of 47 fake services
  with an inline `sparkline-chart` in the p99 column, a `pagination-bar`
  below the table, and a right panel that points a `timeline` at recent
  rollout events and a `json-tree-view` at the selected service's config.
  Demonstrates the v0.47.0 composition shape: each component reads its
  declared inputs and emits its declared messages; the parent model is
  one row-refresh hook and one detail-refresh hook, with no shadowed
  keymaps.
- `examples/release-explorer` — interactive release browser composing
  `list` + `tabs` + `markdown-viewer` + `status-bar` + `key-hints` +
  `theme`. Left pane lists releases with publish-date hints; right pane
  routes through Body (rendered markdown), Assets (formatted asset list),
  and Meta (release metadata). Ships with a 7-release synthetic fixture
  modeled on glyph's own history so the surfaces stay legible without a
  network call or `gh` authentication. A smaller composition than
  metrics-explorer: every component takes its declared inputs via
  `With*` builders, the parent owns the layout normalizer and three
  small panel formatters.

### Changed

- README — documented that `$(go env GOPATH)/bin` must be on `PATH`
  after `go install`, not just `~/go/bin`, which differs when `GOPATH`
  is customized.

## [0.47.0] — 2026-06-06

Seven new components close the data-and-display tier: `sparkline-chart`,
`pagination-bar`, `accordion`, `json-tree-view`, `tree-view`, `timeline`,
and `table-virtualized`. The pure-render primitives (sparkline) stay
value-typed; the keyboard-driven primitives (tree, accordion, table,
timeline, pagination) are `tea.Model` with explicit `*Msg` types for
selection and motion. `tree-view` and `json-tree-view` ship together so
the generic recursive shape and the JSON specialization land in the
same release; `table-virtualized` ships alongside the existing `table`
so the in-memory and `O(visible)` shapes are both available without
either being load-bearing for the other.

### Added

- `components/sparkline-chart` — single-line vertical-bar mini-chart over a
  one-dimensional float64 series. Maps each value to one of eight unicode
  block heights (▁▂▃▄▅▆▇█) and auto-scales the y-range from the data;
  pin either side with `WithMin(v)` or `WithMax(v)` for a fixed scale.
  When the series exceeds `WithWidth` the rightmost width values render
  as a fixed-width window over recent data. `WithLatest(true)` appends
  the most recent value after the bars, formatted via `WithLatestFormat`
  with a units suffix from `WithLatestSuffix`. `WithLabel` adds a muted
  prefix; `WithColor` overrides the bar tint when status colors read
  more clearly than the theme's Primary. Pure-render value type with no
  `Update`; redraw by constructing a new chart with the new values.
- `components/pagination-bar` — single-line page indicator with prev/next
  chevrons, a 1-indexed `page of total` label, and an optional
  `(N items)` suffix. `Page()` is 0-indexed for code; `PageNumber()` is
  1-indexed for display. `PageChangedMsg` fires only when the page
  actually changes, so consumers can listen without filtering no-ops at
  either edge. `WithPerPage(k)` + `WithTotalItems(n)` derive `Total()`
  automatically and let `VisibleRange()` return the `[start, end)` item
  indices for the current page. `WithWrap(true)` wraps motion in both
  directions; the default is no wrap. Edge chevrons dim to the muted
  color when motion would be a no-op.
- `components/accordion` — vertical stack of titled, collapsible sections
  with a focused-cursor model. Single-expanded by default (opening a new
  section closes the previous), or independent via
  `WithAllowMultiple(true)`. Up / down move focus, `Enter` toggles and
  emits `SelectMsg{Section, Index, Expanded}`, `Space` toggles silently,
  `Right`/`l` expands, `Left`/`h` collapses, `Home`/`End` jump to first
  or last, `Tab`/`Shift+Tab` cycle with wrap. `WithSize(w, h)` clips both
  axes and scrolls so the focused header stays on screen. Switching from
  multiple to single mode keeps the focused section open if it was
  expanded, falling back to the first expanded sibling. Empty section
  list shows a customizable placeholder.
- `components/json-tree-view` — interactive, collapsible tree view for
  arbitrary JSON values. A thin shell over `tree-view` that formats strings,
  numbers, booleans, null, objects, and arrays with type-aware colors and
  count suffixes. Objects render as `key {N}` branches sorted alphabetically
  by default; arrays render as `key [N]` branches with `[i]` child keys.
  Integer-valued floats collapse (`7.0` prints as `7`); strings come back
  quoted. `WithJSON(b)` parses raw bytes; `WithValue(v)` takes the `any`
  that `json.Unmarshal` returns. `SelectMsg` carries the underlying JSON
  `Value` alongside the wrapped `treeview.Node`. The JSON specialization
  of `tree-view`, the way `file-tree` is the file-system specialization.
- `components/tree-view` — generic, recursive collapsible tree of nodes. The
  flexible primitive beneath `file-tree`, beneath `json-tree-view`, and
  beneath any agent-state explorer, org chart, call graph, or build
  dependency surface. A `Node` has a `Label`, an arbitrary `Value`, and
  zero or more `Children`; absence of children makes a node a leaf. Paths
  are slash-joined zero-based child indices, stable across label changes.
  `WithExpandedDepth(n)` opens every branch up to depth n; `WithExpandAll`
  / `WithCollapseAll` are the extremes. `Right`/`l` expands, `Left`/`h`
  collapses or jumps to the parent, `Enter` toggles a branch and emits
  `SelectMsg{Node, Path, Index}`, `Space` toggles silently.
- `components/timeline` — vertical sequence of events with status dots, a
  pre-formatted time gutter, and multi-line bodies. Time strings are not
  parsed; absolute clocks, relative durations, and git-style labels all flow
  through the same gutter. Status colors the dot only. Up/Down navigates by
  one event; PgUp/PgDn by half the visible height; Enter emits
  `SelectMsg{Event, Index}`. Drop in for deploy history, audit logs, oncall
  feeds, and agent-run replays.
- `components/table-virtualized` — column-aligned table over a `RowProvider`
  (`Len() int` plus `At(i int) Row`), with `O(visible)` render cost regardless
  of source size. Caller declares column widths; sort is the caller's
  responsibility. The model handles cursor, scroll window, scroll affordance,
  and the render of the visible band. Drop in for log explorers, query result
  viewers, and any surface where the row count outgrows the screen by orders
  of magnitude. Sibling to `components/table`, which keeps its finite,
  in-memory shape with auto-fit and built-in sort.

## [0.46.0] — 2026-05-29

Bracket-pair highlight. The cursor sits on or just past a `(`, `[`, or `{`
(or its closer), and nook paints both ends of the pair with a Primary
foreground over the SurfaceStrong band so the eye finds the match in one
beat. Works across lines, across soft-wrap sub-rows, and for all three
universal pairs.

Anchor selection follows Zed convention: rune-at-cursor wins so the
block cursor's "under here" position is the answer in the dense case,
with rune-before-cursor as the fallback when the cursor sits past the
last rune on a line. The cursor cell pre-empts the band on its own
rune (so the cursor stays visible at the bracket), and the match end
always paints so the eye still finds where the pair closes.

Suppressed during selection (a held Shift+motion grows a band that
would fight the pair) and during multi-cursor (a single highlight
would be ambiguous about which cursor it belonged to). String- and
comment-aware exclusion is intentionally deferred; bracket counting
inside string literals is wrong by definition but the caller only
paints when both endpoints exist, so the damage is "no highlight"
rather than "wrong highlight."

New pure-package primitive at `cmd/nook/internal/bracketmatch`:

- `Match(lines, row, col) (anchor, match, kind, ok)` performs the
  stack-based depth walk forward when the anchor is an opener and
  backward when it is a closer. Mismatched-kind brackets are ignored,
  so an inner paren pair never confuses an outer bracket pair search.
  Scan-budget capped at 1 MiB so a pathological unmatched brace can't
  stall first paint on a megabyte buffer.

## [0.45.0] — 2026-05-29

Soft wrap. `Alt+Z` wraps long logical lines onto multiple visual rows
instead of horizontally scrolling. The gutter and line marker render
once at the top of each logical row; continuation rows show a blank
gutter plus a vertical bar so the reader can see at a glance where
wraps happened. Every render decoration the editor draws — syntax
spans, find-match marks, document-highlight bands, inlay hints,
selection, multi-cursors, indent guides — slices through the wrap
points so each sub-row paints the right contributions. Default off;
add `soft_wrap = true` under `[editor]` in `~/.config/nook/config.toml`
to start with soft wrap on.

New pure-package primitive at `cmd/nook/internal/softwrap`:

- `WrapPoints(line, width, tabWidth)` returns the byte indices where
  each visual row of a single logical line begins. Prefers breaks at
  the last whitespace boundary so trailing spaces consume into the
  prior row, falls back to a hard-break before the overflowing rune,
  and force-places a single rune wider than `width` followed by an
  immediate wrap. Multi-byte runes are never split.
- `LineRowCount(line, width, tabWidth)` is the count-only convenience
  used by visual-row arithmetic.

The editor pane gains `softWrap bool` plus a sub-row offset that
preserves both logical and visual scroll precision: the existing
logical-row `offset` stays the source of truth for fold / git-gutter
/ blame lookups, and a separate `subOffset` indexes the wrapped
sub-row within `offset` so the viewport can scroll partway through a
wrapped line without sticking to logical-row granularity.
`ensureVisible` and the scroll math gate on `softWrap` — when off,
the v0.44 byte-for-byte behavior is preserved.

Seven package-private slicing helpers in
`editor_softwrap_render.go` translate the row-level decoration
channels into per-sub-row contributions. Boundary rule: items at
exactly the wrap point (col == subEnd) belong to the next sub-row,
except EOL items (col == lineLen) which attach to the last sub-row.
Selection tail propagates only onto the last visible sub-row. Indent
guides paint on sub-row 0 only in v1.

27 softwrap-package tests cover empty / single char / exact-width /
wrap-at-last-space / no-space-hard-break / multibyte / leading-tabs /
zero-and-negative tabWidth / single-rune-wider-than-width plus a
fuzz pass over Greek and Japanese sample text × eight widths. 17
editor-package tests cover defaults / accessors / contentWidth /
visual-row math / cursorSubRow / logicalAtVisualOffset / scroll
behavior / `Open()` reset. 15 slice-3 tests cover the seven slicing
helpers and end-to-end View() integration (line-number-on-first-
sub-row-only, sub-row count matches expected, subOffset skips
leading sub-rows, empty + short rows don't crash). Config and
bufman gain `SoftWrap` / `WithSoftWrap` propagation; the help
overlay names `Alt+Z` under Settings.

## [0.44.1] — 2026-05-29

Cross-platform fix for the v0.44.0 path-create edge case. `CreatePath`
now rejects a leading `/` as an absolute path even on Windows, where
`filepath.IsAbs("/")` returns false. The previous build let the bare
slash fall through to the empty-name branch, which made `TestCreatePathEmptyName`
fail on the Windows runner. The runtime behavior on Linux and macOS is
unchanged.

## [0.44.0] — 2026-05-29

File operations in the tree. Focus the file tree (`Ctrl+B`), press `a`,
and a small modal asks for a path relative to the selection. Type
`bar.go` for a file, `internal/foo/` for a directory, or
`internal/foo/leaf.go` for a leaf inside fresh intermediate
directories. Enter commits the create; the tree refreshes and reveals
the new entry; if it's a file, it opens in a buffer. Esc cancels with
no filesystem touch.

Parent-directory resolution mirrors VSCode and Zed: if the selected
row is a directory, the new entry lands inside it; if it's a file, the
entry lands next to it; if nothing is selected, the entry lands at the
workspace root.

New pure-package primitive at `cmd/nook/internal/filetreeops`:

- `CreatePath(parentDir, input)` runs the create with `O_EXCL` for files
  and `os.MkdirAll` for directories (and for any parent dirs along a
  nested input). Rejects absolute paths, ".." traversal, empty names;
  refuses to overwrite an existing entry. Returns a typed
  `CreateResult{Path, IsDir}` plus a sentinel-friendly error set
  (`ErrEmptyName`, `ErrAbsolutePath`, `ErrInvalidName`, `ErrPathExists`).

The file-tree pane intercepts `a` when focused and emits
`filetree.CreatePromptMsg{ParentDir}` with the resolved parent. The
host opens a new `cmd/nook/internal/createprompt.Prompt` modal seeded
with the parent label; the prompt accepts path-legal runes (letters,
digits, dot, underscore, hyphen, slash, plus printable Unicode) and
rejects control characters and NUL. Collisions and validation errors
keep the prompt open with a terse label mapped from the filetreeops
sentinels.

13 filetreeops-package tests cover file, directory trailing-slash,
nested file, nested dir, collision (file and dir), empty / whitespace,
absolute path refusal, "../" traversal at multiple positions, trim,
dotfile names, and double-slash collapse. 17 createprompt-package
tests cover open / close / re-open, all path-rune classes, control-char
rejection, backspace and cursor moves, error clear-on-input, and view
rendering. 8 host tests cover prompt arming from `CreatePromptMsg`,
"." parent label at root, file create + buffer open path, dir create
+ tree refresh path, Esc cancel without filesystem touch, empty value
keeps the prompt open, collision error keeps it open without
overwriting, and nested file materializing parent directories.

## [0.43.0] — 2026-05-29

Indent guides. Every editor row paints faint vertical glyphs at the
tab-stop columns of its leading whitespace (`│` at columns `tab_width`,
`2*tab_width`, ...), and the cursor row's enclosing indent zone gets a
brighter primary-color version of the same glyph. The result reads at
a glance how Zed, VSCode, and Cursor render the same layer.

The guide is decoration, never the foreground signal. Selection bands,
search-match highlights, document-highlight bands, and the cursor cell
all win their cell — the guide simply yields. Column 0 is never a
guide site because the gutter already separates content from the file
edge, so depth-1 rows stay visually quiet.

New pure-package primitives under `cmd/nook/internal/indentguide`:

- `VisualGuideCols(raw, tabWidth)` returns the display columns where
  guide glyphs paint for a row. Walks the leading whitespace, expands
  tabs to `tabWidth` columns, and emits every multiple of `tabWidth`
  strictly less than the expanded width.
- `LeadingWhitespaceVisualWidth(raw, tabWidth)` is the display-column
  width of a row's leading whitespace after tab expansion. Used by
  the guide computation and exposed for any caller that needs the
  same number (e.g. picking the next snippet stop column).
- `ActiveGuideCol(visualCol, tabWidth)` returns the column of the
  guide that the cursor at `visualCol` currently sits within, or -1
  when no guide should highlight. The rule is
  `floor(visualCol / tabWidth) * tabWidth` when
  `visualCol >= tabWidth`, else -1.

The editor `Pane` gains an `indentGuides bool` field (defaults to
true), accessible via `SetIndentGuides(bool)` / `IndentGuides()`.
`bufman.WithIndentGuides(bool)` propagates the flag to every open
pane and any opened later. `EditorConfig` gains `IndentGuides bool`
(toml `indent_guides`, default true), so the user file, project
file, and live reload all see the same toggle.

24 indentguide-package tests cover paint rules, depth-1 quiet,
all-whitespace rows, mixed tab + space leading whitespace, zero/
negative tabWidth, tabWidth=2 / 4 / 8 variants, and ActiveGuideCol
boundary behavior. 12 editor-package tests cover default-on, toggle,
paint counts under tab characters, suppression under cursor /
selection, and multi-row cursor moves. Config tests gain coverage
of the new field across the Default / Load / Merge surfaces, and a
new host test asserts alt+, flips the active buffer's guides off
when the file changes.

## [0.42.0] — 2026-05-29

Per-project config inheritance. A `.nook/config.toml` sitting at the
project root now layers on top of `~/.config/nook/config.toml`. The
project file uses the same `[editor]` schema as the user file, and
any field it explicitly sets — `tab_width`, `format_on_save`,
`line_numbers`, `inlay_hints`, `theme` — wins over the user's choice
for that field only. Fields the project file omits fall through to
the user's setting (or to the built-in default when the user file
also omits them). The `.nook/` directory is the same one that
already holds `tasks.toml` (v0.19.0), so the per-project settings
live alongside the per-project task list for discoverability.

New pure-package primitives under `cmd/nook/internal/config`:

- `LoadRaw(path)` decodes a config file WITHOUT applying the
  `Default()` seeds or the safety reapplies that `Load` performs. It
  returns the parsed Config (every field at its Go zero unless the
  file set it explicitly) plus `toml.MetaData` so callers can
  distinguish "absent in source" from "explicit Go zero." That
  distinction is essential for bool fields like `format_on_save`: a
  project setting it explicitly to `false` must override a user
  default of `true`, and without the metadata, "absent" and
  "explicit false" look identical.
- `Merge(base, overlay, overlayMeta)` per-field overlays only the
  keys metadata reports as defined. Safety reapplies (`TabWidth > 0`,
  `Theme != ""`) run at the end so the result is always usable.
- `ProjectPath(root)` returns `<root>/.nook/config.toml`.

The host layers both files at every read point. `newModel` calls
`config.Load(userPath)` for the seeded user config, then
`config.LoadRaw(projectPath)` + `config.Merge` to fold the project
overlay in. A project parse error surfaces as `project config: …
(using user settings)` so the user can distinguish a malformed
`.nook/config.toml` from a malformed `~/.config/nook/config.toml`.

`Init` fires a parallel `configwatch.WatchCmd` against the project
path, so both files are polled independently. The `TickMsg` handler
dispatches on `msg.Path` and re-arms the matching watch; whichever
file moved triggers a full `reloadConfig`, which re-runs
`loadMergedConfig(userPath, projectPath)` to produce the merged
result from current disk state. Status hints carry a scope suffix:
"settings reloaded (user + project)", "(user)", or "(project)" so
the user can see which scopes shaped the current settings.

13 new config-package tests cover the merge matrix (empty overlay
passthrough, full overlay wins, partial overlay per-field wins,
explicit-false bool, explicit-zero TabWidth, explicit-empty Theme,
base non-mutation, realistic two-layer merge) plus the LoadRaw
shape itself. 5 new host tests cover newModel layering, project-only
config, project parse error surfacing, alt+, picking up project
edits, and TickMsg routing on project path.

## [0.41.0] — 2026-05-28

Signature-help overload cycling. When the floating parameter-hint
overlay is open and the function has more than one matching overload
(any function that the language server returns multiple `Signatures`
for), `alt+↓` advances to the next overload and `alt+↑` steps back.
Both wrap around the list. The status line below the signature already
shows an "n of m" counter, so the user can see which overload they're
on. Pressing `esc` still dismisses the overlay; pressing `)` still
closes it.

The pane stays a pure value type: `signature.Pane.NextOverload()` and
`PrevOverload()` return Pane copies with `ActiveSignature` rotated
modulo `len(Signatures)`. Closed-pane and single-signature panes
short-circuit as no-ops, so the host can call the mutators
unconditionally on the keypress without checking first.

A fresh `Open()` call resets `ActiveSignature` to whatever the server
nominated, so when typing past the current parameter triggers another
`textDocument/signatureHelp` round-trip, the new response lands on the
server's choice instead of fighting the user's previous manual
selection.

The host wires the arrows just after the existing `esc` dismissal
block. The match is `m.sigPane.IsOpen() && km.Alt && km.Type == KeyDown
| KeyUp`, so alt+arrow with no overlay open falls through to the
editor as normal cursor motion.

## [0.40.0] — 2026-05-28

Auto-pair brackets and quotes. Typing an opener — `(` `[` `{` `"` `'`
`` ` `` — inserts the matching closer and parks the cursor between the
two so the next keystroke lands inside. Typing the closer on top of an
already-paired closer skips over it instead of inserting a duplicate.
Backspacing when the cursor sits between an empty auto-pair deletes
both halves at once. The behavior matches Zed and VS Code.

Word-rune suppression keeps the feature from getting in the way:

- Typing `(` mid-identifier (next char is a letter / digit / underscore)
  inserts a literal `(` without auto-pairing — `foo` plus cursor-at-1
  plus `(` becomes `f(oo`, not `f()oo`.
- Typing `'` after a word rune (so `it` then `'`) leaves the quote
  unpaired — contractions like `it's` and `don't` survive intact. Same
  rule for `"` and `` ` ``.
- Typing a symmetric quote on top of the same quote skips over it (the
  ShouldSkip path) instead of double-pairing.

Selection and multi-cursor modes bypass auto-pair so semantics stay
simple in those modes; the rune just inserts at the cursor / at every
extra cursor.

The implementation:

- `cmd/nook/internal/autopair` is a new pure package. `OpenerFor(r)` is
  the opener-to-closer map; `ShouldPair(line, col, r)` decides whether
  to insert the closer alongside an opener; `ShouldSkip(line, col, r)`
  decides whether typing a closer should skip over an existing one;
  `IsEmptyPair(line, col)` reports whether the cursor sits between an
  opener at col-1 and the matching closer at col. The rules are
  unicode-aware: `utf8.DecodeRuneInString` / `DecodeLastRuneInString`
  read the next / prev rune at the byte cursor so a Greek letter or
  any multi-byte word rune still suppresses pairing correctly.
- `editor.Pane.applyAutoPair(r)` is the single-rune insert hook,
  consulted from the `KeyRunes` case in `Update`. Skip-over advances
  the cursor by one column without bumping `bufVer` (no buffer
  mutation). Pair-insert calls `insertRunes([]rune{r, close})` and
  decrements the cursor by one so it lands between the pair.
- `KeyBackspace` consults `autopair.IsEmptyPair(p.buf.Lines[p.row],
  p.col)` before falling through to `delBack`. When true, the path
  calls `delForward` (eat the closer) then `delBack` (eat the opener)
  so the user is back at the empty slot they were about to fill.
- Multi-cursor mode (`len(p.extras) > 0`) and selection mode bypass
  `applyAutoPair` entirely and fall through to the existing
  `insertRunesAllCursors` / `DeleteSelection + insertRunes` paths.
- Help overlay grows one row in the Editing section: `( [ { " ' `` —
  Auto-pair: typing an opener inserts its closer`.

Auto-pair is a typing-feel feature, not a key-binding feature: it has
no overlay, no separate keybind, no toggle. It changes what `(` does at
the microscopic level. The change is small per keystroke and easy to
miss; in practice, you stop noticing it within a few minutes, which is
the goal.

## [0.39.0] — 2026-05-28

Comment toggle (Ctrl+/). Select any range — or just park the cursor on
a row — and press Ctrl+/ to flip the lines between commented and
uncommented. Twenty-four extensions and four canonical basenames know
their prefix: `// ` for Go / Rust / Zig / C / C++ / Java / Swift /
Kotlin / Scala / Dart / C# / F# / JS / TS / JSX / TSX / PHP / Groovy,
`# ` for Python / Bash / Ruby / Perl / R / Julia / Elixir / Elm / TOML
/ YAML / Terraform / HCL / Makefile / Dockerfile / Rakefile / Gemfile,
`-- ` for Lua / Haskell / SQL / Ada / VHDL, `" ` for Vim, `; ` for
Lisp / Scheme / Clojure / Emacs Lisp, `% ` for TeX / BibTeX.
Block-comment-only languages (HTML, CSS, XML) and unknown extensions
no-op silently — no status noise, no destructive surprise.

The implementation:

- `cmd/nook/internal/comment` is a new pure package. `Prefix(path)`
  reads the extension (or basename for Makefile et al.) and returns
  the line-comment marker, with a trailing space so toggled code reads
  like the surrounding language rather than a greppable banner.
  `ToggleLines(lines, prefix)` returns the new slice and an op
  (Comment, Uncomment, Noop); blank lines pass through unchanged in
  both directions, and the "are we already commented" check inspects
  only non-blank lines.
- Indented blocks anchor at the minimum non-blank indent so all rows
  share the comment column. Match Zed and VS Code: an uncommented-
  then-recommented block returns to where it started.
- `editor.Pane.ToggleComment()` walks the active selection (or the
  cursor row when nothing's selected), copies the rows out, runs them
  through `ToggleLines`, writes them back in place, advances `bufVer`,
  marks the buffer dirty, and re-applies syntax highlight + ensures
  visibility. Cursor accounting is handled by `adjustCursorAfterToggle`:
  compute the leftmost-diff column, apply the signed delta to columns
  at or right of the diff, clamp into `[diffCol, len(newLine)]`. The
  selection anchor is adjusted alongside the head so the visual
  selection survives the toggle.
- Host binding lands at `tea.KeyCtrlUnderscore`. xterm-style emulators
  fold both Ctrl+/ and Ctrl+_ into byte 0x1F (US, Unit Separator), so
  a single case covers both for the same operation. Go-file buffers
  fire `lspChangeCmd` after the toggle so gopls diagnostics track the
  change live.
- Help overlay grows one row in the Editing section: `ctrl+/ — Toggle
  line comment on selection or cursor row`.

### Added

- `cmd/nook/internal/comment` package with `Prefix(path string) string`
  and `ToggleLines(lines []string, prefix string) ([]string, Op)`.
- `editor.Pane.ToggleComment() Pane` chainable transform on the editor
  pane, including cursor and selection-anchor adjustment.
- Host `Ctrl+/` keybinding for the comment toggle.

### Changed

- Help overlay's Editing section now lists `ctrl+/`.

### Notes

- `editor.Pane.Theme() theme.Theme` (added in v0.38.0) was used to
  validate live theme propagation in unit tests; it stays exported.

## [0.38.0] — 2026-05-28

Live config + theme reload. Edit `~/.config/nook/config.toml` while
nook is open and the new settings take effect on the next render —
no restart, no `alt+,` reach. Switch `theme = "tokyo-night"` to
`theme = "catppuccin-mocha"` and every open buffer, the file tree, the
welcome card, the markdown preview, the tab bar, and the status row all
adopt the new palette together. Tab width, line numbers, format-on-save,
and inlay hints follow the same path.

The implementation:

- `cmd/nook/internal/configwatch` is a new package that polls the
  config file once a second. `Fingerprint{ModTime, Size, Exists}` is
  the change cue — size catches edits on filesystems with coarse
  mtimes (ext4 relatime, NFS) where two saves within the same second
  would otherwise look identical. `Snapshot(path)` reads the cue;
  `WatchCmd(path, last)` returns a tea.Cmd that ticks, re-snapshots,
  and emits a `TickMsg` whose `Changed()` method tells the host
  whether to reload. The host re-arms the cmd from its `Update`
  handler so the loop self-sustains until the program exits.
- Polling instead of fsnotify. Config files are tiny and change rarely,
  the platform surface (inotify/FSEvents/RDCW + atomic-rename editor
  dance) carries more risk than 1 Hz buys back, and avoiding the new
  direct dependency keeps the nook binary lean.
- `editor.Pane.SetTheme(t) Pane` joins fifteen other panes that now
  take a live theme swap: bufman, multibuffer, term, diagnostics, git,
  picker, edit, finder, composer, outline, search, tasks, filetree,
  and the markdown preview (which rebuilds its wrapped glyph viewer
  so the new palette propagates through the embedded markdown
  component). `model.applyTheme(t)` walks every theme-holding field
  in one place, which means future panes opt in by adding one
  SetTheme call to that method.
- `markdownviewer.Source() string` is the small additive shim on the
  glyph public surface that lets mdpreview rebuild the viewer with
  the new theme while preserving the rendered markdown source.

Manual `alt+,` still works and now lands on the same code path, so
the keybind is an "I know I edited the file, refresh now" shortcut
rather than the only way in. The earlier "restart to apply" status
hint is gone — theme changes apply live, and the new status reads
`theme switched to <name>` when the user changes themes,
`settings reloaded` for anything else.

## [0.37.0] — 2026-05-28

LSP call hierarchy. `alt+k` and `alt+K` turn an identifier under the
cursor into a call graph and surface the results in the multibuffer
pane the same way find-references does — one row per call site, the
caller name in the suffix, the multi-file context already wired.
`alt+k` asks gopls "who calls this?" (incoming); `alt+K` asks "what
does this call?" (outgoing). The pair shares wiring with `alt+u`
find-references: prepareCallHierarchy first, then a direction-specific
roundtrip, then `BuildFragments` collapses results into the
multibuffer.Fragment slices the pane already knows how to render.
Implementation lives in `cmd/nook/internal/callhierarchy`; the LSP
client picks up `PrepareCallHierarchy`, `IncomingCalls`, and
`OutgoingCalls` on the existing protocol@v0.12.0 surface, and the
CallHierarchy capability is advertised in Initialize so gopls knows
to answer.

## [0.36.0] — 2026-05-25

LSP document highlights. When the cursor settles for a quarter second
over an identifier in a Go file, `nook` now asks gopls for every
occurrence of that identifier in the current buffer and paints each
one with a subtle background band. Move the cursor to a variable named
`config` and every other `config` on screen lights up — readers, writers,
and uses alike. It's the same comprehension aid VS Code, Zed, and IDEA
have called "highlight occurrences" for two decades, and it's the
fastest way to answer "where else is this used in this file?" without
a full Find References roundtrip.

The implementation:

- `cmd/nook/internal/dochi` is a new package that turns LSP
  `DocumentHighlight` responses into a `map[int][]Span` keyed on row
  so the editor's per-row render loop has an O(spans) lookup. Multi-
  line highlights are flattened at line boundaries; an `End == -1`
  sentinel signals "extend to row length."
- `lsp.Client.DocumentHighlight` advertises the capability during
  initialize and forwards the typed `textDocument/documentHighlight`
  call through the go.lsp.dev protocol package.
- `lookup.DocumentHighlightCmd` issues the request and stamps each
  response with the `paneVer` it was minted for.
- `editor.Pane.SetDocumentHighlights(hi, paneVer)` is the staleness
  gate — a response that arrives after the user typed is silently
  dropped, so the band never lags behind the cursor.
- The renderer threads a `runeDocHi []bool` parallel slice through
  the existing row-emit loop and adds one stage to the precedence
  chain (matchActive > selection > matchOther > dochi > plain), so
  search and selection still win when both apply.
- The host model debounces cursor-settle with a `dochiSettleMsg` /
  `dochiGen` counter pair — every key event clears the overlay and
  arms a fresh 250ms tick, and only the most recent tick to fire
  issues the LSP request.

The overlay is decorative and degrades silently when the server
resolves nothing, which is the right shape for a comprehension aid:
if it can't tell you anything useful, it gets out of the way.

## [0.35.0] — 2026-05-25

Lightning startup. Before this release, opening `nook` against a large
directory (the worst case being `nook ~/.zshrc`, where the project root
resolves to `$HOME` and the walker descends into every `~/repos/*` and
`~/Downloads`) blocked the first paint on a synchronous recursive
file-system walk. On a machine with a deep home directory that meant
~1 second of blank terminal before the editor drew anything. Anywhere
else, the gap was smaller but always present.

The file-tree pane now constructs in constant time and walks the file
system in a goroutine. `filetree.New` returns a pane that reports
`Built() == false` and renders a `Scanning…` placeholder; `Init`
batches `filetree.BuildTreeCmd(root)` alongside the existing file-list
and git-status commands. When the resulting `BuildTreeMsg` lands, the
host calls `Pane.SetNode(node)` which binds the tree. A `Reveal` issued
before the tree is built is queued on `pendingReveal` and replayed by
the next `SetNode`. The `BuildTreeMsg` handler discards stale walks
whose root no longer matches (forward-proofing future re-root paths).

`Pane.Refresh` was synchronous; it's now `Pane.RefreshCmd()` returning
a `tea.Cmd` so the open-tree path in `toggleTree` no longer blocks on
the walk either.

A new regression test (`TestStartupNotGatedOnTreeWalk`) measures
`newModel($HOME) + Init()` end-to-end with a 200ms budget. On this
machine the actual wall-clock is ~300µs — three orders of magnitude
below the pre-refactor cost. The test fails loudly if anyone wires
synchronous file-system work back into the startup path.

## [0.34.0] — 2026-05-25

Project-wide find/replace in `nook`. The Alt+F search pane already
streamed ripgrep matches and let Enter open a hit; v0.34 adds the
second half. With results visible, Alt+R flips the pane into replace
mode: a second prompt row appears beneath the header showing the
replacement field; typing builds it; Enter applies the replacement to
every recorded match across the workspace; Esc collapses back to
result navigation without throwing away what you typed. Up/Down still
move the cursor while in replace mode so you can see what's about to
be rewritten.

The new `cmd/nook/internal/search/replace.go` does the disk pass.
Matches are grouped by path; per-file the body is read once, lines
split (preserving the original trailing-newline state byte-exact —
files like `package.json` that ship without one stay that way), hits
on each line are sorted right-to-left by byte column so earlier
column indices stay valid after the rewrite changes line length, and
the file is written once. Corrupt hits (out-of-range line, zero or
negative span, span past row end) are skipped silently so a stale
match list from a competing edit can't panic or corrupt a file. The
caller gets `Result{FilesChanged, ReplacementsApplied, PathsTouched}`
back; the host iterates `PathsTouched` to call
`bufman.RefreshIfOpen` so any of the rewritten files that are
currently open reload immediately and show the new bytes.

Status hint surfaces `replaced N occurrence(s) in M file(s)` on
success or `replace error: <wrapped read/write error>` on partial
failure; the search pane closes either way so the user lands back on
their editor. Help overlay gains a new `Project search (alt+f)`
section documenting the typing → results → Alt+R → Enter flow.

Match struct gains `Len int` (byte length of the first submatch),
populated from ripgrep's `submatches[].end - .start`. This is the
field ApplyAll needs to know what span to replace; consumers that
only care about navigation can ignore it.

No in-app undo for replace: git is the safety net. The help overlay
notes this so users know to commit before a workspace-wide rewrite
of a common token. Sixteen new tests pin the apply semantics
(single-file, same-line-multi, UTF-8 byte spans, missing trailing
newline preserved, empty-replacement-deletes-span, multi-file,
corrupt-hit skip, write-error path) plus eight Pane tests for
EnterReplace gating, mode-collapse on Esc, ApplyMsg emission on
Enter, replacement preservation across mode toggles, cursor
navigation while replacing, and View rendering the replace row.

## [0.33.1] — 2026-05-25

Windows clipboard paste fix. v0.33.0 shipped with three Windows CI test
failures around `Ctrl+V`: `aINSbc` came back as `aINS`, `REPLACED world`
as `REPLACED`, and a three-line paste added an extra newline before the
trailing character. The root cause was PowerShell's `Get-Clipboard`
helper always appending CRLF to its output, which the previous CRLF
normalization in `cmd/nook/internal/clip.readFromOS` left as a trailing
LF. The paste then split the line. macOS (pbpaste) and Linux without a
helper were already green because pbpaste preserves content as-is and
the no-helper path falls back to the in-process register byte-exact.

Fix: a new `normalizeOSText([]byte) string` helper runs CRLF→LF first,
then `strings.TrimSuffix(s, "\n")` to strip exactly one trailing
newline. This matches `wl-paste --no-newline` semantics already used on
Wayland, and matches how every IDE paste behaves (a copied line never
re-inserts its line-end). Six new clip-package tests pin the
normalization shape: CRLF stripped, trailing LF stripped, interior LFs
preserved, no-trailing-newline unchanged, interior CRLF lowered to LF,
double-trailing-newline reduced to exactly one.

No new API surface, no version-bump dependency, no behavior change on
Linux or macOS hosts.

## [0.33.0] — 2026-05-25

Selections and a working clipboard land in `nook`. Shift+arrow extends a
selection from an anchor; Ctrl+A selects the whole buffer; Ctrl+C copies,
Ctrl+X cuts, Ctrl+V pastes. With no selection active, Ctrl+C and Ctrl+X
operate on the current line (the VSCode default), including the trailing
newline so a follow-up paste re-inserts the line above wherever the
cursor lands. A subsequent movement key collapses the selection to the
appropriate side (Left collapses to start, Right to end, vertical keys
clear it).

The new package `cmd/nook/internal/clip` is a register-first clipboard:
every `Set` always updates an in-process register, then makes a
best-effort 500ms shell-out to whichever helper is on the host —
`wl-copy`, `xclip`, `xsel`, `pbcopy`, `clip.exe`, or PowerShell's
`Get-Clipboard`. `Get` reads the OS clipboard when a helper exists and
falls back to the register otherwise. This keeps cut / copy / paste
fully functional in headless containers, in CI, and in tests, while
still round-tripping with VS Code, Slack, and the rest of the host's
applications when a helper is installed.

The editor's `Pane` grows a `selecting` flag plus `anchorRow` /
`anchorCol` and a small helper surface — `SelectAll`, `ClearSelection`,
`CollapseToLeft` / `CollapseToRight`, `DeleteSelection`,
`SelectionRange`, `SelectionText`, `HasSelection`. The selection
renders with a `Primary` background and `TextInverse` foreground;
on multi-row selections a trailing-space sentinel paints the
newline-included cells so the eye reads "this line is part of the
selection" without ambiguity. The render path's style precedence
runs Cursor > Active Match > Selection > Other Match > Plain
syntax, so a `Ctrl+F` hit under the selection stays salient.

Backspace, Delete, Enter, Tab, Space, and rune-insertion all delete
the active selection first when one exists, which preserves the
"select and type to replace" idiom every modern editor implements.
Esc clears a selection in preference to extra cursors; multi-cursor
extras and selections are mutually exclusive (any Shift+motion
clears extras).

The help overlay grows a "Selection / Clipboard" section listing
every binding. Twenty-six new tests cover the selection model, the
clip package's round-trip and newline preservation, and end-to-end
host bindings for Ctrl+A / C / X / V across both selection and
empty-selection paths.

## [0.32.0] — 2026-05-25

Navigation history (vim-style jump list) lands in `nook`. Every
cross-file or significant in-file jump now records the cursor's
from-position into a bounded ring buffer; `Alt+-` walks back through
those positions like vim's `Ctrl-O`, and `Alt+=` walks forward like
`Ctrl-I`. After three jumps `a → b → c → d`, pressing `Alt+-` three
times retraces `d → c → b → a`; pressing `Alt+=` walks the other way.

A new package, `cmd/nook/internal/navhistory`, owns the data structure:
an `Entry{Path, Row, Col}` triple, a `History` with an `idx` cursor in
`[0, len(entries)]`, and three Push rules evaluated in order
(predecessor-match collapses duplicate pushes onto the same position;
forward-match advances `idx` without rewriting; otherwise truncate the
forward tail and append). Capacity defaults to 100 entries and the
oldest is evicted on overflow. Sixteen tests cover the zero value, the
empty-path no-op, single and multi-push back/forward sequences, the
vim-style truncate-on-fresh-push, both duplicate-collapse paths,
capacity eviction, position readouts, current-entry accessor,
`CanBack`/`CanForward` boundaries, `Reset`, default capacity for
zero/negative inputs, snapshot-is-copy semantics, and a realistic
multi-file jump scenario (`main.go → pkg/util.go → pkg/types.go`, two
backs, fresh push to `pkg/log.go`).

The host wires push points at every existing jump site: project search
results (`search.OpenMsg`), LSP go-to-definition (`lookup.DefinitionMsg`),
multibuffer row selection (`multibuffer.OpenAtMsg`, which covers
workspace symbol search via `Ctrl+T`, find-references via `Alt+u`, and
the uncommitted-changes multibuf via `Alt+m`), the workspace
diagnostics panel (`diagnostics.OpenAtMsg`), and outline jumps
(`outline.JumpMsg`). Each site calls `pushNavCurrent` immediately
before the cursor moves, so the from-position is the one Alt+- returns
to. Two new helper methods, `navJumpBack` and `navJumpForward`, open
the entry's buffer (re-using `bufman.OpenOrSwitch` so it works
cross-file), call `JumpTo` with 1-based row/col, refresh the file tree
reveal, and trigger LSP/gutter/inlay/blame refreshes the same way every
other jump path does.

The status bar gains a `nav N/M` segment that surfaces only while the
user is walking the list (suppressed at past-end where `pos == total`),
and the `?` help overlay grows a "Navigate" section with both bindings.
Status hints on each jump call out direction and remaining list
position: `jump 2/3 ← util.go:5:1`. The push-point set is conservative
in the same way vim's is — opening a file from the file tree doesn't
add to the list, only explicit jumps do.

## [0.31.0] — 2026-05-25

LSP semantic tokens land in `nook`. The editor now overlays the language
server's `textDocument/semanticTokens/full` response on top of chroma's
syntactic highlight, so parameters, properties, enum members, namespaces,
and read-only declarations get their own colors instead of bleeding into
the surrounding identifier color.

A new package, `cmd/nook/internal/semtok`, decodes LSP's delta-encoded
`uint32` 5-tuples (`[deltaLine, deltaStart, length, tokenType,
tokenModifiers]`) into resolved tokens via the server's legend. `Decode`
is a pure function; `semtok.Token` carries `Line`, `Col`, `Length`, the
resolved type name, and the resolved modifier set. Eleven tests cover
empty input, malformed length, single-token, same-line and cross-line
delta unrolling, modifier bit flags, modifier bits beyond the legend
range, out-of-range type indices, and a realistic Go snippet.

The LSP wedge gains a `SemanticTokensFull(ctx, path)` method driven via
the raw `jsonrpc2.Conn` (the protocol library at v0.12.0 doesn't expose
the typed call) plus a capability advertisement at initialize: 22 token
types and 10 modifiers, `Range: false`, `Full: true`. The server's
legend comes back through the initialize result and is parsed into the
`semtok.Legend` the decoder consumes.

`internal/lookup` adds `SemanticTokensCmd`/`SemanticTokensMsg` carrying
`PaneVer` (the editor's buffer-revision counter at request-issue time),
so a response that arrives after the user has typed harmlessly drops on
the floor at overlay-apply time. The editor stores chroma as a floor
layer and merges semantic tokens on top, splitting or replacing the
underlying spans as needed; the merge is also pure-functional, defined
on `highlight.MergeSemantic(base, tokens)`. Five new theme tokens
(`SyntaxParameter`, `SyntaxProperty`, `SyntaxEnumMember`, `SyntaxNamespace`,
`SyntaxReadonly`) ship across all five built-in themes (Default,
TokyoNight, CatppuccinMocha, RosePine, Light).

Host wiring fires the request on `lspOpened` (initial overlay as soon
as gopls finishes parsing) and after each `didChange` (via
`tea.Sequence` so the change lands at gopls before the
semanticTokens/full request goes out, avoiding a stale read). When the
buffer advances past the request's pinned bufVer, the editor silently
drops the overlay; the next semantic-tokens response repaints the
correct ranges.

## [0.30.0] — 2026-05-25

Debug adapter support lands in `nook`. The editor speaks the
[Debug Adapter Protocol](https://microsoft.github.io/debug-adapter-protocol/)
over stdio with `dlv dap`, so Go programs can be debugged from inside
the editor with breakpoints, step-over / step-in / step-out, pause,
and a stop marker that follows the program counter through the
running stack.

The wire client is hand-rolled (`cmd/nook/internal/dap`) over the
Content-Length framing JSON-RPC envelope DAP shares with LSP. No
`go-dap` dependency. The client exposes `Initialize`, `Launch`,
`SetBreakpoints`, `ConfigurationDone`, `Continue`, `Pause`, `Next`,
`StepIn`, `StepOut`, `StackTrace`, `Terminate`, `Disconnect`, and a
single `Events()` channel that surfaces `stopped`, `continued`,
`output`, `terminated`, and `exited` envelopes as typed values. A
`NewWithStreams` constructor lets tests drive the client with a pair
of `io.Pipe` halves and an in-process fake adapter, so the
event-loop wiring is exercised under `go test` with no `dlv` on the
host.

Breakpoints live in `cmd/nook/internal/breakpoints`, a per-path
`map[int]bool` behind a `sync.RWMutex`. Lines are 1-based on the
wire and in the store, 0-based when the editor paints the gutter,
so the model does the conversion once at the boundary and the rest
of the code reads the natural form for its layer.

Six new bindings hang off the editor:

```
F9            toggle breakpoint at cursor row
F5            launch (when no session) / continue (when paused)
alt+F5        terminate the running session
F6            pause a running session
F10           step over
F11           step in
alt+F11       step out
```

(`Shift+F5` is the conventional DAP terminate key but `bubbletea`
doesn't carry `Shift+F<n>` through the input stream, so terminate
moves to `alt+F5` and step-out moves to `alt+F11`. Documented in the
`?` help overlay's new "Debug (Go via delve)" section.)

The gutter column now resolves with a fixed precedence: the program
counter marker `▶` wins, then the breakpoint dot `●`, then the git
change sigil. The status bar carries a new `dbg:<state> ●N` segment
showing the current adapter state (`launching` / `running` /
`paused` / `terminated`) and the live breakpoint count, colored by
state. When a `stopped` event arrives the editor jumps to the top
stack frame, opens the file if it isn't already on a tab, and lands
the cursor on the paused line.

## [0.29.0] — 2026-05-25

LSP snippet completions land in `nook`. When a language server returns
a completion item with `insertTextFormat: 2` (the Snippet format), the
body parses through `snippets.Expand` and the editor enters snippet
mode at the first tabstop. `Tab` and `Shift+Tab` walk placeholders,
`Esc` exits. Bridges the v0.18.0 snippet engine to the v0.6.0 LSP
completion accept path.

The `Completion()` client capability now advertises
`completionItem.snippetSupport: true`, so servers like gopls, rust-
analyzer, and pyright actually send snippet-format completions instead
of plain text. The `CompletionItem` type carries an `InsertTextFormat`
field that round-trips through `completionItem/resolve` so the snippet
flag is preserved when the documentation side panel re-fetches a
highlighted item.

The accept-path now dispatches on `InsertTextFormat`. Plain-text items
take the original delete-prefix-then-insert path; snippet items take
`editor.Pane.ExpandSnippet(prefixStart, exp)` which performs the
range delete and insert atomically and sets up the placeholder ring.
A status-bar hint reads `inserted <label> — Tab to advance, Esc to
exit` so the snippet mode is legible to a user who didn't know they
just triggered one.

Also fixes a latent off-by-one in the plain-text branch of
`acceptCompletion`: `editor.Pane.JumpTo` is 1-based, but the prefix-
rewind path was passing 0-based row/col. The bug had been live since
v0.6.0 but only surfaced once the snippet tests exercised the
surrounding code. Cross-referenced the other JumpTo call sites in
main.go (lines 1134, 2398, 2775) to confirm they were already
1-based-correct.

## [0.28.0] — 2026-05-25

`nook` now opens single files from the CLI. Three shapes work:

```
nook file.go              # open one file; root is its parent dir
nook ~/.zshrc             # files outside any project work too
nook newfile.txt          # vim-style: creates an empty buffer,
                          # save writes the file (mkdir -p first)
nook a.go b.go c.go       # multi-file; first is active,
                          # alt+] / alt+[ switch between buffers
```

The previous behavior (`nook` with no args, or `nook some/dir` for
a project root) is unchanged. Argument parsing lives in a new
`parseStartup` function with unit-tested cases for every shape
(directories, files, missing-but-creatable paths, trailing-slash
directory references, mixed multi-arg input). Pre-opened buffers
get the same first-frame treatment as a picker-opened file: LSP
attach for `.go` paths, git gutter computation, gopls inlay hints
when the toggle is on, and inline blame. The status bar surfaces
`opened <path>` or `opened N files (alt+] / alt+[ to switch)` so
the launch reads as deliberate, not as a side effect.

This closes the "you can't quickly edit your dotfiles" gap. `nook
~/.config/nook/config.toml` lands directly on the file in two
keystrokes, behaving like `vim` for the open-and-edit-one-file
case while still giving you the full IDE if you launch into a
project root.

## [0.27.0] — 2026-05-25

LSP completion documentation side panel in `nook`. Open the completion
popup (Ctrl+Space) and a small bordered pane appears to the right of the
menu showing the highlighted item's resolved documentation. Move the
selection with `↑` / `↓` and the side pane re-fetches via
`completionItem/resolve`, so the docs always match the row the user is
sitting on. Dismiss the popup (Esc, Enter to accept, or any non-nav key)
and the doc pane closes with it.

Resolve responses are pinned to the item label that was highlighted when
the request fired. A late response that arrived after the user scrolled
past gets dropped by the staleness gate instead of painting docs for a
row that is no longer visible. The pane is best-effort by design: a
server that doesn't implement resolve, or returns an item with no detail
and no documentation, silently leaves the pane closed rather than
writing to the status bar.

The pane composes from the same theme tokens as the rest of the editor
(rounded border with the theme `Border` color, `Surface` background) and
sits in the same float layout as the completion popup, only rendered
when the screen is wide enough to fit `menu + gap + min(36) doc`. Layout
is responsive: the doc pane width matches the popup width so the two
boxes read as one widget; height matches the popup row count plus the
border budget.

LSP documentation has two on-wire shapes — a plain string or a
`MarkupContent { kind, value }` struct — and the client accepts either.
The opaque `data` field that some servers (gopls, rust-analyzer) attach
to completion items gets round-tripped through `completionItem/resolve`
verbatim via `json.RawMessage` so the server can find its bookkeeping
when it answers. The client advertises `completion.resolveSupport` with
`["documentation", "detail"]` so servers know which properties they can
defer until resolve time.

The kind tag in the side panel's header matches the outline pane's
scheme — `fn` for function, `mt` for method, `st` for struct, and
twenty-one other two-letter tags — so the editor reads consistently
across symbol surfaces. Detail lines that duplicate the label (some
servers echo) are suppressed to keep the header compact. Long
documentation gets word-wrapped to the inner width with hard-break for
over-long tokens, and clamped at the pane's row budget with a trailing
" …" row to signal truncation.

The completedoc package is unit-tested across twenty-plus cases covering
open/close state, size clamping, label/detail/documentation rendering,
suppression of redundant detail, kind-tag coverage for all twenty-four
named completion kinds, line clamping with ellipsis, paragraph wrapping
with hard-break, and view refusal below the minimum width. The resolve
client path is tested across documentation merging, original-field
preservation, polymorphic shape decoding, and kind round-tripping.

## [0.26.0] — 2026-05-25

LSP signature help in `nook`. Type `(` inside a function call and a
small bordered overlay appears at the bottom of the editor showing
the call's signature with the parameter you're about to fill bolded
and highlighted. Type a `,` and the highlight slides to the next
parameter. Type `)` or press `Esc` and the overlay vanishes. The
trigger is intentionally narrow — signature help is a hint, not a
flow — so the only keys that open or close it are the call-expression
delimiters and Esc. While the overlay is up, every other key (typing
arguments, deleting, moving) leaves it alone.

Two label shapes both work. Gopls's legacy form sends the parameter
label as a plain string and expects the client to find it inside the
parent signature string; rust-analyzer and typescript-language-server
send a `[start, end]` offset pair pointing into the parent signature
directly. The client advertises `parameterInformation.labelOffsetSupport`
so servers that can use offsets do, and the decoder tries the offset
form first, falling back to substring search on the string form.
Either way the active parameter's range gets reverse-styled with the
theme primary, and the row that paints it is rune-indexed so multi-byte
identifiers don't shift the highlight.

Multiple overloads are honored — the LSP response's `activeSignature`
index selects which one shows, and a small "n of m" italic counter
appears under the signature when more than one is available. Overload
cycling between them (Alt+↑/↓ on the overlay) is deferred to a later
release; for now the server's choice is the one the user sees.

Signature-level and parameter-level documentation both render under the
signature, wrapped at the overlay's inner width and clamped at four
lines so a chatty typescript-language-server response can't blow out
the screen. When the two docs match exactly (gopls sometimes echoes the
summary into both fields) the duplicate is suppressed.

### Added

- `cmd/nook/internal/signature` package: pure-value `Pane` overlay with
  `New()`, `WithSize(w)`, `Open(info)`, `Close()`, `IsOpen()`, `Info()`,
  `ActiveSignature()`, and `View(theme)`. Renders a bordered surface
  with the signature label, an optional "n of m" overload counter for
  multi-signature responses, the active parameter's documentation, and
  the signature-level documentation (each independently word-wrapped
  and line-clamped). Helpers `clampLine`, `wrapAndClamp`, and
  `formatCounter` cover the truncation and label shape.

- `lsp.Client.SignatureHelp(ctx, path, line, col)` calls
  `textDocument/signatureHelp` and returns a strongly-typed
  `SignatureInfo{Signatures, ActiveSignature}` regardless of which
  label shape the server uses. Decoder handles polymorphic
  `Documentation` (string or `MarkupContent`) and polymorphic
  `Parameter.Label` (string substring or `[start, end]` offset pair),
  per the LSP spec.

- `SignatureHelpTextDocumentClientCapabilities` is advertised in
  `Initialize`, including
  `parameterInformation.labelOffsetSupport: true` and
  `activeParameterSupport: true`. Servers that respect these (most
  modern ones) send offset-form labels and per-signature active
  parameter indexes; servers that don't still work via the substring
  fallback.

- `lookup.SignatureHelpCmd` / `lookup.SignatureHelpMsg`: standard
  async LSP wrapper following the established `HoverCmd` / `HoverMsg`
  pattern (2-second timeout, nil-client guard, stale-request discard
  via path/row/col pin in the host model).

- `(` listed in the keymap under "Language server" with the description
  "Signature help (parameter hints auto-fire on '(', close on ')' or
  esc)".

### Changed

- `nook` host model carries an open/closed `sigPane` plus the
  request-pin (`sigReqPath`, `sigReqRow`, `sigReqCol`) so late
  responses to a paren the user has since closed are silently
  discarded.

## [0.25.0] — 2026-05-25

File outline modal in `nook`. Press `Ctrl+\` inside any file and a
floating modal pops up listing every top-level and nested symbol gopls
(or any other LSP) reports for the buffer: functions, methods, structs,
constants, interfaces, the whole document symbol tree. Type to filter
case-insensitively. Up/down/Home/End/PgUp/PgDn move the cursor; Enter
jumps the editor to the symbol's definition row; Esc closes without
moving. The modal opens with the cursor pre-positioned on whichever
symbol encloses the current editor row, so the second keystroke is
already useful.

This is the single-file companion to the workspace symbol search
(`Ctrl+T`, shipped in 0.23.0). Where workspace symbol crosses the
project boundary and asks the server for every matching name, the
outline modal stays inside the active buffer and asks for the
hierarchical document symbol tree — cheaper, faster, and showing the
nesting (methods grouped under their receivers, nested types indented
under parents). Both keys land you on a definition; pick whichever
matches the question you're holding in your head.

The decoder handles both LSP response shapes: the modern hierarchical
`DocumentSymbol[]` (with `selectionRange` and `children`) and the
legacy flat `SymbolInformation[]` (parented by `containerName`). The
client capability advertises both. Symbol kinds are rendered as short
two-letter tags (`fn`, `mt`, `st`, `tp`, `vr`, `cn`, `if`, `en`, ...)
to keep the row dense without losing the kind information.

Per-file symbol trees are cached after the first fetch, so repeated
`Ctrl+\` presses on the same file are instant. The cache is
invalidated on save — a fresh document symbol request goes out the
next time the user opens the outline after editing.

### Added

- `cmd/nook/internal/outline` package: pure-value `Pane` modal with
  `New(t)`, `WithSize(w,h)`, `Open(path, syms, atRow)`,
  `OpenError(path, msg)`, `Close()`, `IsOpen()`, and `Update(msg)`.
  Emits `JumpMsg{Path, Row, Col}` on Enter and `CancelMsg{}` on Esc.
  Pure helpers `Flatten(syms)` (DFS with depth annotation) and
  `EnclosingIndex(flat, row)` (deepest containing symbol) for tests
  and direct callers.

- `lsp.Client.DocumentSymbol(ctx, path)` fetches the document symbol
  tree for one file. Decodes both hierarchical `DocumentSymbol[]` and
  flat `SymbolInformation[]` responses; the flat variant is
  reconstructed into a one-level tree via `ContainerName`.
  `DocumentSymbolClientCapabilities` advertises both shapes plus the
  full `SymbolKind` enum.

- `Ctrl+\` keybind in `nook`. Opens the file outline modal for the
  active buffer. Falls back to a friendly error pane when no LSP is
  attached (no .go file open, or gopls hasn't started yet).

### Changed

- Help overlay (`?`) lists the new `Ctrl+\` binding under "LSP &
  Navigation".

## [0.24.0] — 2026-05-25

Repo-level AI conventions in `nook`. Drop a `.nookrules` (nook-native)
or `.cursorrules` (Cursor compat) file at the workspace root and every
AI wedge — ghost-text autocomplete, Cmd+K inline edits, Ctrl+L
multi-file composer — picks up the prose conventions you've written:
"use tabs, never wrap at 80 cols," "fmt.Errorf wraps the underlying
err," "imports stay grouped stdlib/third-party/local," whatever your
project already standardizes. A Cursor user dropping nook into their
existing repo gets the same model behavior with zero re-onboarding.

The loader looks for `.nookrules` first, then falls back to
`.cursorrules`. Both missing is the common case and a no-op — the AI
wedges run unchanged. A whitespace-only file is treated as absent so
an empty `touch .nookrules` doesn't register a useless status chip.

A small `rules:nookrules` / `rules:cursorrules` chip appears on the
right end of the status bar when a file is loaded, so the user knows
the rules are live. Loaded once at startup; a future tick can add a
file-system watcher if reload-without-restart turns out to matter.

The rules are appended to each wedge's task-specific system prompt
under a "Repository conventions" trailer. Each wedge keeps its
specialized prompt (one-line edits for `edit`, fenced multi-file
blocks for `composer`, single-line continuations for `ghost`) but
inherits the user's prose conventions on top.

### Added

- `cmd/nook/internal/airules` package: `Load(root)` returning
  `(Source, content, error)` where `Source` is `SourceNookrules`,
  `SourceCursorrules`, or `SourceNone`; `AugmentSystemPrompt(base,
  rules)` for the system-prompt trailer composition (empty rules is
  a no-op); `StatusChip(source)` for the status-bar label.
- `edit.Pane.WithRules(s)` setter wiring the workspace rules into
  Cmd+K inline-edit requests.
- `composer.Pane.WithRules(s)` setter wiring the workspace rules
  into Ctrl+L multi-file composer requests.
- `ghost.Manager.SetRules(s)` setter wiring the workspace rules into
  inline ghost-text autocomplete requests.
- Status-bar chip rendering `rules:nookrules` / `rules:cursorrules`
  next to the diagnostic counts.

## [0.23.0] — 2026-05-24

Workspace symbol search in `nook`. Press Ctrl+T, type a query, hit
Enter, and every matching symbol (functions, types, methods, fields,
constants) across the entire workspace opens in the multibuffer
overlay. The pane's body is the same Fragment shape v0.13.0 designed —
one window per hit, the symbol's declaration line marked `Added`,
three context lines above and below. The fragment header reads "func
handleSearch" or "method User.Hello" so a results page works as a
project outline even before you Enter on a row.

Single-keystroke refinement is the muscle memory: Ctrl+T re-opens the
modal with your previous query and the cursor at the end, so amending
"Server" to "ServerStart" is two keystrokes. The prompt accepts any
printable rune — gopls method-qualified queries like "T.M",
rust-analyzer fuzzy queries like "fmt#print", path-style queries like
"http/handler" — none of which would fit through the identifier-only
filter the rename modal uses.

LSP wiring goes through `protocol.WorkspaceSymbolParams` and
`SymbolInformation`, with the response distilled into a host-friendly
`WorkspaceSymbolKind` enum (Function, Method, Class, Struct, Variable,
Constant, Field, Property, Interface, Enum, EnumMember, Namespace,
Package, TypeParameter, File) so future overlays can branch on kind
without depending on the protocol package. The kind is rendered as a
compact label (`func`, `method`, `var`, `const`, `struct`, `iface`)
woven into the fragment suffix.

Mirrors Zed and VS Code's Ctrl+T project-symbol shortcut, but rendered
through nook's multibuffer pane so the result reads as a real outline
rather than a single-line picker dropdown. Picked over DAP debugging
(item 18, multi-session swing) and per-language LSP fan-out (still
roadmap) because the v0.13.0 Fragment shape was designed precisely
for this expansion — the multibuffer pane absorbs the new loader
with zero changes to the pane code.

### Added

- `cmd/nook/internal/symbolsearch` package: `Prompt` modal (open,
  close, type/backspace/delete/clear, move home/end/left/right,
  error display) accepting any printable rune so language-server-
  specific query syntax passes through; `BuildFragments(syms,
  contextLines, reader)` turning `[]lsp.WorkspaceSymbol` into
  `[]multibuffer.Fragment` with windows merged on overlap and a
  `kind container.name` suffix per fragment; `FindSymbolsCmd(client,
  query, contextLines, reader)` tea.Cmd factory wrapping the LSP call
  with timeout, source-tag, and FragmentsMsg envelope; `OSReader`
  default plus tests against in-memory map readers.
- `lsp.Client.WorkspaceSymbol(ctx, query)` wrapper around
  `workspace/symbol`, returning host-friendly
  `[]WorkspaceSymbol{Name, Kind, Container, Path, Line, Col}` with
  `WorkspaceSymbolKind` enum + `Short()` label and `mapSymbolKind`
  protocol → host conversion; `WorkspaceSymbolClientCapabilities`
  declared at Initialize so servers actually emit results.
- `nook` host: `overlaySymbolSearch` overlay, `symbolPrompt` and
  `lastSymQuery` fields, `openSymbolSearch` handler (re-opens with
  the last query, surfaces "no LSP" hint), `routeSymbolSearch` for
  the modal's key handling (Enter fires, Esc cancels, Backspace /
  Delete / Ctrl+U / Home / End / Ctrl+A / Ctrl+E / arrows for input
  editing). Modal renders centered over the workspace at 64-column
  cap; results land in the multibuffer overlay titled "symbols
  matching <query>".
- Ctrl+T binding in the global key switch + `ctrl+t` line in the
  Language server section of the help overlay + `help_test.go`
  mustHave list extended to include `ctrl+t`.

## [0.22.0] — 2026-05-24

Find references via LSP in `nook`. Place the cursor on an identifier,
press Alt+U, and every workspace site that mentions the symbol opens in
the same multibuffer overlay the diff loader uses. Each hit ships with
three lines of context above and below; hits that fall close together
on the same file merge into one continuous fragment so a method called
on lines 7 and 9 reads as a single 4-12 strip rather than two
overlapping ones. The pane title carries the symbol you searched for
("references to ParseConfig"), and the existing Enter-jumps-Esc-closes
contract carries straight through from the diff loader.

The request goes through the gopls client over the standard
`textDocument/references` channel with `IncludeDeclaration: true`, so
the declaration site and every call site are visible from one keystroke.
A 3-second timeout matches the hover and definition wedges already in
the LSP package; results are sliced into multibuffer.Fragments by a
pure `BuildFragments(locs, contextLines, reader)` computation so the
loader is fully unit-testable against an in-memory file reader.

Hit rows are painted with the multibuffer.Added marker so the diff
pane's highlight color the user already recognizes for "+" lines now
also marks "this is the line that referenced your symbol." Context rows
carry multibuffer.Context. Files that fail to read (permissions, race,
deletion mid-search) produce a single-line placeholder fragment with
the error text so the user still sees the hit and Enter still routes to
the editor, where the real error will surface in a more useful place.

Symbol extraction is Go-style identifier matching: `[A-Za-z_]
[A-Za-z0-9_]*` with all-digit spans rejected, so the cursor parked on a
numeric literal returns "no identifier under cursor" instead of an
empty multibuffer. The cursor can sit anywhere within or at the right
edge of the identifier and still resolve, matching how editors place
the caret after the last keystroke. No LSP attached, no buffer open,
or no identifier under the cursor all surface as status-bar hints
rather than opening a hollow overlay.

Why Alt+U. Cursor and VS Code default to Shift+F12, but F12 is not
universally-reliable across terminals (alt-screen + function-key
encoding varies by emulator). Alt+u is the portable surface and the
mnemonic is "usages." The binding sits next to the other alt-prefixed
LSP keys (alt+i hover, alt+y inlay hints, alt+enter code actions) so
muscle memory clusters cleanly.

### Added

- `cmd/nook/internal/findrefs`: pure-Go references-to-fragments loader.
  `Symbol(source, row, col)` extracts the identifier at a 0-indexed
  position; `BuildFragments(locs, contextLines, reader)` groups
  locations by file, slices a configurable context window around each
  hit, merges overlapping or touching windows, marks hit rows with
  `multibuffer.Added`, and emits a sorted `[]multibuffer.Fragment`.
- `findrefs.FindReferencesCmd(client, path, row, col, contextLines,
  reader)` returns a `tea.Cmd` that resolves to a
  `multibuffer.FragmentsMsg{Source: "references"}` carrying the
  fragments (or an error wrapped in the same message). `ErrNoClient`
  fires when the language server is not yet attached.
- `findrefs.OSReader` is the default file reader; tests inject
  map-backed readers via the `Reader` callback type.
- `findrefs.DefaultContextLines` is `3` to mirror `git diff
  --unified=3` — the same window the multibuffer pane already shows
  for diff fragments.
- `lsp.Client.References(ctx, path, row, col)` wires
  `textDocument/references` with `IncludeDeclaration: true` over the
  existing gopls connection; converts results into the package's
  internal `Location` shape.
- Host wiring: `Alt+U` calls `findReferencesAtCursor`, which extracts
  the identifier under the cursor, opens the multibuffer overlay with
  title `"references to <symbol>"`, and dispatches the LSP query. No
  identifier, no buffer, or no LSP each emit a status-bar hint rather
  than opening a hollow overlay.
- Help overlay entry under "Language server": `alt+u` → "Find
  references to identifier under cursor."

## [0.21.0] — 2026-05-24

GitLens-style inline blame for `nook`'s editor pane. Open any file under
a git work tree, press Alt+B, and the cursor row now shows a dim italic
strip after the end of the line: `Author • 3 days ago • fix: handle
empty inputs`. Move the cursor to a different row and the strip follows;
toggle off and the editor returns to a clean canvas. Lines you haven't
committed yet render as `(uncommitted)` so the working-tree edits are
visible without confusion about whose name to expect.

The blame data is computed once per file via `git blame --porcelain`,
wrapped into a `tea.Cmd` that posts back a `BlameMsg` with the
working-tree row → metadata map. The host caches the result on the
active editor pane and re-fetches on file open, save, or buffer switch
so the strip stays in sync with the most recent commits. Visibility is
a separate switch from data, so toggling Alt+B doesn't re-shell git for
buffers you've already opened in the session.

The strip is width-aware: when the line of code already fills the
viewport budget (a 40-column line on a 40-column pane, for example),
the blame strip is elided rather than wrapped. When summaries are too
long for the remaining budget, they're truncated with an ellipsis at
rune boundaries. The relative-time bucketing reads as English ("just
now", "1 minute ago", "yesterday", "3 weeks ago", "2 years ago") and
falls silent for the zero-time sentinel that uncommitted lines carry.

Files outside a git work tree (and binary files git can't blame) produce
an empty map; the editor pane shows nothing and the toggle is harmless.
The path on every `BlameMsg` is compared against the active buffer's
path before applying, so a late response for an old buffer is dropped
cleanly when the user has already moved on.

### Added

- `cmd/nook/internal/inlineblame`: `Line` struct (SHA / Author / Email /
  Time / Summary), `IsUncommitted()` predicate, `UncommittedSHA` constant
  for the all-zero working-tree SHA.
- `Compute(ctx, root, path)` shells out to `git rev-parse
  --is-inside-work-tree` then `git blame --porcelain`; returns an empty
  map (no error) for paths outside a work tree.
- `Parse(out)` walks porcelain output and fills repeat-SHA references
  from the first sighting, so abbreviated header lines pick up author /
  time / summary from the earlier full record.
- `HumanizeSince(now, t)` buckets the elapsed gap into short relative
  phrases; returns empty string for zero time.
- `Render(line, now, maxSummary)` formats `Author • relative-time •
  Summary` with ellipsis truncation at rune boundaries.
- `BlameCmd(root, path)` tea.Cmd factory + `BlameMsg{Path, Lines, Err}`
  with path round-tripped so late responses can be dropped.
- `editor.Pane.SetBlame` / `SetBlameVisible` / `BlameVisible` / `BlameAt`
  setters and accessors; `View()` injects the dim italic strip after the
  cursor row when both data and visibility are present.
- Host wiring: `model.blameOn` toggle, `refreshBlameCmd` /
  `refreshBlameOnPathCmd` / `applyBlame` / `setBlameVisibility` helpers,
  `BlameMsg` case in `Update`. Refresh fires on file open and save and
  on every buffer switch.
- `Alt+B` keybinding toggles inline blame; status line reports
  `"inline blame: on"` / `"inline blame: off"`.
- Help overlay entry under a new "Git" section listing `alt+b` →
  "Toggle inline blame on cursor row (GitLens-style)."

## [0.20.0] — 2026-05-24

Per-file conversation history for `nook`'s Ctrl+L composer. Every
finished composer turn is now appended to an in-memory store keyed by
the active file's absolute path, and the next time the composer opens
on that same file, the prior instructions and responses are folded
into the system prompt as "Prior turns on this file (oldest first):"
context. The model treats the new instruction as a follow-up on the
established thread instead of a cold ask, so "now wire them together"
or "rename the helper you just added" lands without re-pasting the
earlier exchange.

Each path keeps the eight most recent turns (the store cap) and the
prompt header replays the six most recent (the budget cap) — two
knobs so storage and prompt size can diverge if either needs to grow
or shrink later. When the composer opens, the pane header shows
"N prior turn(s) on this file (alt+h clear)" only when N > 0, so the
empty case is silent. Alt+H clears the history for the current file
and reports "cleared N turn(s)" or "no per-file history" in the
composer's status line.

History scope is the file: switch buffers and the composer rescopes
to the new path's transcript; if the new path has no history, the
header line disappears. Empty paths and error-terminated streams are
no-ops — history only records on clean completion.

### Added

- `cmd/nook/internal/aihistory`: thread-safe `Store` keyed by file
  path with configurable per-path cap (`DefaultMaxPerPath = 8`),
  `Append`, `Turns` (defensive copy), `Count`, `Clear`, `ClearAll`.
- `composer.Pane.WithHistory` and `WithActivePath` setters that
  refresh the cached count without taking the store lock on every
  `View()`.
- `buildUserPrompt` prepends a "Prior turns" section (cap 6) when
  the store has matching transcripts.
- `streamDoneMsg` success handler records `{Instruction, Response, At}`
  on the active path; error or empty prompt skips the write.
- `Alt+H` keybinding in `StateComposing` clears the active path's
  history and surfaces the result in `statusOn`.
- Help overlay entry under "AI wedges" listing `alt+h` →
  "Clear composer history for the current file."

## [0.19.0] — 2026-05-24

Tasks for `nook`. Alt+T pops a picker over `.nook/tasks.toml` (a VSCode
`tasks.json` analog rendered in TOML); Enter runs the selected task and
flips the pane into a streaming output viewport tagged with stdout /
stderr per-line. Ctrl+C kills the running process; Esc closes the
overlay (and kills if a task is still live). When `.nook/tasks.toml` is
missing, Go projects get four sensible defaults out of the box —
`go test ./...`, `go build ./...`, `go vet ./...`, `go mod tidy` — so
the first run on a fresh repo just works.

The TOML accepts either an `[[task]]` array of tables (preferred for
real config files) or a `tasks = [...]` inline-table array (handy for
one-liners), and every task supports a `name`, `command`, `args`,
optional `cwd` (resolved relative to the project root), `description`,
and a `[task.env]` table for per-task environment overrides. Malformed
config surfaces in the pane header but doesn't block the overlay —
the user still sees the default tasks and can run one while they fix
their TOML.

Why Alt+T and not Ctrl+Shift+B: every terminal collapses Ctrl+Shift+B
to Ctrl+B, which is nook's file-tree toggle. Alt+T (mnemonic: "tasks")
is the portable surface — same shape as the alt+m / alt+p / alt+v /
alt+y bindings already in the keymap.

### Added

- `cmd/nook/internal/tasks`: TOML loader with `[[task]]` and `tasks=[]`
  forms, `Defaults` for Go projects, `LoadOrDefaults` with graceful
  fallback when config is missing or malformed.
- `tasks.Runner`: `exec.CommandContext`-backed supervisor with stdout /
  stderr streaming via 1 MB-buffered `bufio.Scanner`, monotonic run-ID
  for stale-message discarding, env overrides, and project-root-relative
  `Cwd` resolution.
- `tasks.Pane`: two-mode overlay (`ModeList` picker + `ModeOutput`
  streaming viewport) with output buffer ring-capped at 4 000 lines,
  per-stream colors, scrollback (PgUp / PgDn / Home / End), and a
  status-colored exit summary.
- `Alt+T` host keybinding that loads tasks at open time so on-disk
  edits are picked up between invocations without a restart.
- Help overlay entry under "Tasks" listing the five bindings.

## [0.18.0] — 2026-05-24

VSCode-format snippets for `nook`. Type a prefix in any buffer, hit
`Alt+J`, and the prefix is replaced with a parsed snippet body whose
tabstops you can cycle through with Tab / Shift+Tab. `Esc` exits
snippet mode; any non-snippet keystroke auto-exits and falls through
to normal editing.

The shipped library covers Go (`fn`, `iferr`, `tfn`), TypeScript /
JavaScript (`afn`, `intf`), Python (`def`, `main`), Markdown
(`link`, `code`), Rust (`fn`), and a global scope (`todo`, `fixme`)
keyed off the active buffer's file extension. The grammar is VSCode-
compatible — `$1`, `${1:default}`, `$0` for the final cursor, and the
variables `$TM_FILENAME`, `$TM_FILENAME_BASE`, `$CURRENT_YEAR`,
`$CURRENT_DATE` — so existing snippet packs port over without
translation.

Users can drop their own VSCode snippet JSON files under
`~/.config/nook/snippets/<scope>.json` (e.g. `go.json`, `ts.json`,
`global.json`); the host loads them at startup and overlays them
on top of the built-in defaults. Missing directories are silently
skipped — first-run nook ships with sensible defaults out of the box.

Why `Alt+J` and not `Tab`: Tab is already taken by ghost-text accept
and would compete with snippet expansion the moment you both want a
suggestion *and* a snippet expansion in the same buffer. Ctrl+J
collapses to Enter (byte `0x0a`) in most terminals; Ctrl+Shift+V is
reserved for paste. `Alt+J` (mnemonic: "jot") is the portable surface.

### Added

- `cmd/nook/internal/snippets`: VSCode-format parser + library with
  exact-prefix lookup, scope routing by file extension, JSON
  `LoadFile` / `LoadDir`, and a 12-entry default pack.
- Editor-level snippet mode: `editor.Pane.ExpandSnippet`,
  `SnippetNext`, `SnippetPrev`, `SnippetExit`, `InSnippetMode`,
  `CurrentSnippetTabstop`, and a `SnippetTabstop` value type.
- `Alt+J` host keybinding that resolves the prefix-before-cursor
  against the active buffer's language scope and hands the matched
  expansion to the editor.
- `~/.config/nook/snippets/<scope>.json` overlay loaded at startup.
- Help overlay entry under "Snippets" listing the four bindings.

## [0.17.0] — 2026-05-24

Markdown preview pane for `nook`. Alt+V toggles a right-column preview
of the active `.md` / `.markdown` buffer, rendered by glyph's existing
`components/markdown-viewer` Bubble Tea snippet. Headings, lists,
blockquotes, code blocks, inline emphasis, and links all render with
the active theme's tokens.

The pane is read-only — editing still happens in the editor — and
refreshes automatically when the previewed buffer hits disk via Ctrl+S
or Alt+S. Non-markdown buffers don't open the pane: the toggle shows
a one-line status hint instead, so the keystroke never silently fails.

Preview competes for the right column the same way git, terminal, and
composer already do — opening one closes the others. PgUp / PgDn /
Home / End scroll the preview when it owns focus. Esc closes it and
returns focus to the editor.

Why Alt+V and not Ctrl+Shift+V: most terminals reserve Ctrl+Shift+V
for paste, which is unrecoverable. Alt+V mirrors how alt+m / alt+p /
alt+y already work for adjacent panes and overlays.

### Added

- `cmd/nook/internal/mdpreview`: thin nook pane wrapping the reusable
  `components/markdown-viewer` Viewer with nook's `Focus / Blur /
  WithSize` conventions and an `IsMarkdownPath` extension gate.
- `Alt+V` host keybinding wired into the right-pane competition with
  git / term / composer.
- Refresh-on-save: `editor.SavedMsg` for the previewed path re-seeds
  the pane from the active buffer's contents.
- Help overlay entry under "Panes".

## [0.16.0] — 2026-05-24

Workspace-wide diagnostics panel for `nook`. The editor already shows
gutter markers and a per-file `E/W` count in the status bar, but neither
surface answers the question every IDE user asks twenty times a day:
*what's broken across the whole project right now?* Alt+P opens a
focused overlay listing every diagnostic LSP has published, sorted by
severity then file then line, with the source server in italics next to
each row. Enter on a row opens the file at the diagnostic's source site
and closes the overlay. Esc dismisses without moving.

The list is rebuilt from the host's existing `m.diagnostics` map every
time the overlay opens, so it reflects the current LSP state rather
than a snapshot. No new collection wiring was needed — the workspace
store has been accumulating publishDiagnostics events since v0.6.

Why Alt+P and not the more familiar Ctrl+Shift+M: terminals collapse
Ctrl+Shift+M into Ctrl+M (Enter), which is unrecoverable inside
bubbletea. Alt+P is the portable surface (mnemonic: "problems") and
mirrors how alt+m/alt+y/alt+, already work for adjacent overlays.

### Added

- `cmd/nook/internal/diagnostics` — new package. `Severity` enum (LSP
  values 1–4), `Entry{Path, Row, Col, Severity, Source, Message}` row
  type, `Sort([]Entry) []Entry` stable sort (severity ASC, path ASC,
  row ASC, col ASC), `Pane` model with `NewPane`/`WithSize`/
  `WithEntries`/`Focus`/`Blur` builders, `Update` handling Esc →
  `CancelMsg`, Enter → `OpenAtMsg`, Up/Down/Home/End/PgUp/PgDn
  navigation. View renders a bordered card with a "workspace
  diagnostics" header, severity counts (`N errors  N warnings  N info
  N hints`), and one row per entry with a severity-colored single-
  letter badge, theme-muted relative path with 1-indexed
  `:row:col`, italic source tag, and the message. Cursor row paints
  on `SurfaceStrong`. ANSI-aware cell-width truncation keeps long
  messages from overflowing the card.
- `cmd/nook/main.go` — new `overlayDiagnostics` overlay, `diagPane
  diagnostics.Pane` model field, `alt+p` handler that calls
  `collectDiagnosticEntries()` (walks `m.diagnostics`,
  one `diagnostics.Entry` per LSP diagnostic, message newlines
  collapsed to spaces). `diagnostics.OpenAtMsg` handler calls
  `bufman.OpenOrSwitch`, jumps the editor to `msg.Row+1, msg.Col+1`,
  blurs the overlay pane, restores diagnostics to the active editor,
  and triggers gopls/gutter/inlay refresh. `diagnostics.CancelMsg`
  handler blurs and re-focuses the editor.
- `cmd/nook/internal/help` — new "Problems" section in the keymap
  overlay covering alt+p, navigation, enter, and esc.

### Tests

- `cmd/nook/internal/diagnostics/diagnostics_test.go` — 21 tests
  covering severity mark/color helpers, sort stability (severity →
  path → row → col), input non-mutation, pane state machine
  (focus/blur, cursor clamp on WithEntries shrink/empty, blurred-pane
  key suppression), all six key paths (Esc, Enter, Up, Down, Home,
  End, PgUp, PgDn) with boundary clamping, view edge cases (empty
  state, header counts, 1-indexed location, message and source
  inclusion), formatLocation under three root configurations (within,
  outside, no root), and the ANSI-CSI strip + cell truncation
  helpers.
- `cmd/nook/main_test.go` — `TestAltPOpensDiagnostics`,
  `TestDiagnosticsCollectsAllOpenBuffers` (multiple paths and
  severities round-trip through the map),
  `TestDiagnosticsCollectStripsMessageNewlines`,
  `TestDiagnosticsOpenAtMsgJumpsAndCloses` (uses real git fixture
  repo + bufman + JumpTo),
  `TestDiagnosticsCancelMsgClosesOverlay`,
  `TestDiagnosticsOverlayRoutesKeys` (Esc through overlay routing
  produces a `CancelMsg` cmd).

## [0.15.0] — 2026-05-24

Settings file and themes for `nook`. A user-editable
`~/.config/nook/config.toml` now drives the editor knobs and the color
palette so the IDE can be reshaped without recompiling. Four new themes
ride alongside the existing pair (`default`, `light`): `tokyo-night`,
`catppuccin-mocha`, and `rose-pine`. `alt+,` re-reads the file at
runtime; editor toggles (format-on-save, inlay hints, tab width, line
numbers) take effect immediately. A theme change is detected and
surfaced as a status hint asking the user to restart, since deeply-
themed sub-panes aren't live-reskinned in v0.15.0.

The file is optional. A missing config is silently equivalent to the
baseline defaults, and unknown keys are accepted so a forward-compat
key from a newer nook doesn't break an older one. A malformed file
prints the parse error in the status bar and the editor still opens
with default settings — the file is a knob, not a gate.

### Added

- `components/theme` — three new palettes: `TokyoNight` (folke/tokyonight
  `night` variant), `CatppuccinMocha` (catppuccin/palette
  `catppuccin-mocha` spec), `RosePine` (rose-pine/rose-pine `main`
  variant). New `theme.ByName(name) (Theme, bool)` and `theme.Names()
  []string` registry surface so consumers (settings UI, future
  `--list-themes` flag) can look up by stable identifier without
  hard-coding a switch.
- `cmd/nook/internal/config` — new package. `Config{Editor:
  EditorConfig{TabWidth, FormatOnSave, LineNumbers, InlayHints,
  Theme}}` deserialized from TOML; `Default()` returns the baseline,
  `Path()` resolves `$XDG_CONFIG_HOME/nook/config.toml` (mirroring
  alacritty / helix / zellij), `Load(path)` returns
  `(Config, ErrNotFound)` when the file is absent so the host can
  fall back silently. Unknown keys are tolerated. Tests cover all
  the failure shapes (missing file, partial file, malformed TOML,
  zero-value TabWidth, empty Theme, unknown forward-compat keys).
- `editor.Pane.SetTabWidth(int)` + `SetLineNumbers(bool)` plus
  matching readers, threaded into `renderHighlightedRow` so tab
  expansion follows the configured width and the gutter respects
  the `line_numbers` flag. `bufman.Manager.WithTabWidth` /
  `WithLineNumbers` propagate to every open pane and every pane
  opened later.
- Host wiring in `cmd/nook/main.go`: `newModel` reads config at
  startup and surfaces a status hint for unknown themes or parse
  errors. `reloadConfig` re-reads on `alt+,` and applies the
  runtime-mutable knobs (format-on-save, inlay hints, tab width,
  line numbers); theme changes are detected and routed to a
  "restart to apply" hint. `lookup.FormattingCmd` now passes the
  configured tab width instead of a hard-coded 4.
- `cmd/nook/internal/help` — new "Settings" section with the
  `alt+,` reload binding.

## [0.14.0] — 2026-05-24

Inlay hints for `nook`. When gopls is attached and the cursor lands on
a Go file, type annotations and parameter names appear as faint italic
glyphs woven into the source: `x := 42` reads `x := 42`*` : int`*,
`f(name, count)` reads `f(`*`name=`*`name, `*`count=`*`count)`. The
hints are decorative — they never change the file on disk and the
underlying bytes the editor saves match exactly what was typed.

`alt+y` toggles the layer on and off (default: on). Stale responses
(typed past the request) are discarded by comparing the LSP didChange
version the request carried; toggling off clears existing hints
without firing a request, so a wedged language server can't strand
glyphs on screen.

### Added

- `cmd/nook/internal/inlayhint` — new package built around
  `Hint{Row, Col, Label, Kind, PaddingLeft, PaddingRight}` plus
  `Kind` enum (`KindType`, `KindParameter`). `ByRow(hints)`
  collates a `[]Hint` into the `map[int][]Hint` shape the editor
  consumes. Unit tests cover empty input, multi-row binning, and
  in-row stable sort by column.
- `lsp.Client.InlayHint(ctx, path, startLine, endLine)` — drives
  `textDocument/inlayHint` via raw `jsonrpc2.Conn.Call`
  (`go.lsp.dev/protocol@v0.12.0` doesn't export the type yet).
  Returns `[]inlayhint.Hint`. Initialization now passes the gopls
  `hints` configuration (parameterNames, assignVariableTypes,
  constantValues, rangeVariableTypes, compositeLiteralTypes,
  compositeLiteralFields, functionTypeParameters) so the server
  actually emits hints instead of returning empty results.
- `lookup.InlayHintCmd(client, path, version, startLine, endLine)`
  — mirrors the HoverCmd shape. 2s timeout; nil-client and error
  paths still return a message so the host can surface a stale
  marker without panicking.
- `editor.Pane.SetInlayHints(map[int][]inlayhint.Hint)` and
  `InlayHintsAt(row int)` — pane-local store. `renderHighlightedRow`
  now threads hints through, mapping raw byte columns to expanded
  display columns (so a hint anchored at `x := 42` lands after the
  `2`, not in the middle of a tab expansion). Hints are decorative;
  when the row's width budget is tight, hints drop from the back
  first before any source glyph is trimmed.
- Host wiring in `cmd/nook/main.go`: `inlayHintsOn bool` (defaults
  true), `refreshInlayHintsCmd` and `clearInlayHints` helpers,
  `applyInlayHints` with stale-version discard, `lookup.InlayHintMsg`
  routing in `Update`. Refresh triggers on picker open, save, search
  jump, definition jump-land, and multibuffer open. `alt+y` toggles
  the layer with status-bar feedback.
- `cmd/nook/internal/help` — new "Toggle gopls inlay hints" entry
  in the Language server section.

## [0.13.0] — 2026-05-24

Multibuffer view for `nook`. Zed's signature surface: every hunk in
the working tree, stitched together into one scrollable list, opened
with `alt+m`. Each fragment shows its file path, line range, the
function suffix git stamps onto the hunk header, and the new-file
lines marked `+` (added) or ` ` (context). Enter on any row jumps to
that file at that line and focuses the editor; `esc` closes the
overlay without opening anything.

The first slice is read-only — a complete and useful surface on its
own. Editable multibuffer (edits flowing back to source) is deferred
to a later release.

### Added

- `cmd/nook/internal/multibuffer` — new package built around
  `Fragment{Path, StartLine, EndLine, Lines, Suffix}` and `Pane`
  with the standard nook component shape (`NewPane`, `WithSize`,
  `Focus`/`Blur`, `Update`, `View`). 36 unit tests cover Parse over
  empty input, single/multi-hunk, multi-file, pure-deletion-skipped,
  no-newline-at-eof metadata, omitted-count default, absolute-path
  preservation, plus row-build / cursor-skip / Selected / Update
  routing / View rendering / live `git diff` end-to-end.
- `multibuffer.LoadDiffCmd(root, base)` — runs
  `git diff --no-color --unified=3 [base]` and returns
  `FragmentsMsg` with the parsed hunks.
- Host wiring in `cmd/nook/main.go`: `overlayMultibuffer` enum,
  `multibufPane` field, `alt+m` key case, `FragmentsMsg` loader,
  `OpenAtMsg` jump-and-close, `CancelMsg` close-without-opening.
- `cmd/nook/internal/help` — new "Multibuffer" section with
  `alt+m`, navigation, Enter, and Esc.

## [0.12.0] — 2026-05-24

Multi-cursor editing inside `nook`. The primary cursor at `(p.row, p.col)`
is joined by zero or more extras (`p.extras []extraCursor`) that take
the same edits in lockstep. `ctrl+d` finds the next whole-word
occurrence of the identifier under the primary and adds a cursor at
the end of that match (with buffer-end wrap). `ctrl+↑` / `ctrl+↓`
stack a cursor on the row above / below the topmost / bottommost
existing cursor — Zed-style column edit. `esc` clears extras without
closing the editor; any movement key (arrows, home/end, pgup/pgdn)
collapses to the primary.

Edits at multiple cursors are processed front-to-back in `(row, col)`
order, with each shift function (`shiftAfterInsertRunes`,
`shiftAfterDeleteChar`, `shiftAfterInsertNewline`,
`shiftAfterMergeWithAbove`, `shiftAfterMergeWithBelow`) keeping later
positions consistent with the in-place buffer mutation. Insert,
backspace, delete, enter, tab, and space all fan out across every
cursor. `applyAtAllCursors` dedups before and after each pass so
cursors that converge after an edit collapse naturally.

### Added

- `editor.Pane.AddNextMatchCursor()` — `ctrl+d`. Whole-word identifier
  search (`isWordChar` boundary check), forward-from-latest with a
  buffer-end wrap and head-of-latest-row tail.
- `editor.Pane.AddCursorBelow()` / `AddCursorAbove()` — `ctrl+↓` /
  `ctrl+↑`. Stacks the new cursor at the primary's column, clamped to
  the target row's length.
- `editor.Pane.ClearExtraCursors()`, `ExtraCursorCount()`,
  `AllCursorPositions()` — host inspection helpers.
- Editor key cases: `KeyCtrlD` adds next-match, `KeyCtrlUp` /
  `KeyCtrlDown` stack vertically, `KeyEsc` clears extras when present.
- `Multi-cursor` section in the `?` help overlay.

### Changed

- Edit primitives (backspace, delete, enter, tab, space, runes) now
  branch on `len(p.extras) > 0` and dispatch to
  `backspaceAllCursors`, `delForwardAllCursors`, `newlineAllCursors`,
  or `insertRunesAllCursors`. Single-cursor paths are unchanged.
- Movement keys (arrows, home, end, ctrl+a, ctrl+e, pgup, pgdn) clear
  extras before applying — explicit collapse on any cursor motion.
- `renderHighlightedRow` now takes `cursorCols []int` and a separate
  `primaryCol int`. Every extra cursor on a visible row paints a
  cursor cell; ghost-text rendering stays anchored to the primary.

## [0.11.0] — 2026-05-24

Per-line git gutter inside `nook`. Each row in the editor now carries a
two-character marker column: the leading char surfaces the working-tree
diff state for that line (added, modified, or deleted-above), the
trailing char keeps its LSP diagnostic sigil. Both signals are visible
simultaneously and the column width is unchanged. The host computes
markers by shelling out to `git diff --no-color --unified=0` against
the index (and against `/dev/null` for untracked files), so the gutter
matches whatever `git diff` shows on the same file. Refreshed on
buffer open, save, picker/search/filetree open, and go-to-definition
landing.

### Added

- `cmd/nook/internal/gitgutter` — `Marker` enum (`None`, `Added`,
  `Modified`, `DeletedAbove`), `Compute(ctx, root, path)` end-to-end
  pipeline, `Parse(diff)` pure unified-diff parser, `MarkerCmd(root,
  path)` `tea.Cmd` factory, and `MarkersMsg{Path, Markers, Err}` for
  host dispatch with stale-path discard.
- `editor.Pane.SetLineMarkers(rows)` / `LineMarkerAt(row)` accessors
  parallel to the existing diagnostic accessors; the `View()` render
  loop composes the two-character marker (git sigil + diagnostic
  sigil) into a fixed-width column.

### Changed

- Buffer-open sites (`picker.SelectMsg`, `search.OpenMsg`,
  `filetree.OpenMsg`, `lookup.DefinitionMsg`) and `editor.SavedMsg`
  now fire `gitgutter.MarkerCmd` so markers reflect the current
  working-tree state.

## [0.10.0] — 2026-05-24

Code actions and rename inside `nook`. `alt+enter` asks the language
server for the code actions available at the cursor (quickfix,
refactor, source-organize) and renders them in a centered popup;
`enter` applies the selected `WorkspaceEdit` across every affected
file, opening new buffers or rewriting on disk as needed. `f2` opens
a rename prompt seeded with the identifier under the cursor, runs
`textDocument/prepareRename` first to confirm the symbol is renameable
(falling back to a source-walk when gopls returns the
`defaultBehavior:true` placeholder), then sends `textDocument/rename`
and applies the workspace-wide edit. Errors surface in the prompt so
the user can retype without losing context.

### Added

- `cmd/nook/internal/codeaction` — `Popup` value type holding the
  current LSP `CodeActionItem` slice with theme-aware rounded-border
  view, cursor up/down navigation, accept/dismiss helpers, and a
  `Selected()` accessor that refuses items marked `Disabled` (returning
  the reason for the host to surface as a status message). Mirrors the
  `complete.Popup` shape so the host's overlay routing stays uniform.
- `cmd/nook/internal/rename` — `Prompt` value type with a small text
  input limited to identifier characters (Unicode letter / digit /
  underscore, leading-digit rejected), placeholder seeding from the
  symbol under cursor, cursor home/end/left/right, `WithError` for
  server-side failures (clears on next edit), and a `View` that shows
  `<current> → <new>` with the source path so the user knows what
  they're renaming.
- `lsp.Client.CodeAction(path, row, col)` and
  `lsp.Client.PrepareRename(path, row, col)` and
  `lsp.Client.Rename(path, row, col, newName)` — three new typed
  request helpers on the LSP client that convert protocol-level
  `CodeActionItem` / `PrepareRenameResult` / `WorkspaceEdit` into
  internal types so the editor never imports `go.lsp.dev/protocol`.
- `lookup.CodeActionCmd` / `lookup.PrepareRenameCmd` / `lookup.RenameCmd`
  — `tea.Cmd` factories with the established 2-second timeout +
  nil-client guard pattern; each echoes the request coordinates back
  in the response so the host can discard stale answers if the cursor
  moved.
- Host integration in `cmd/nook/main.go`: `caPopup`, `caReqPath/Row/Col`,
  `renamePrompt`, and `pendingRename` fields on the model;
  `overlayCodeAction` / `overlayRename` overlay states; key routing for
  `alt+enter`, `f2`, and the prompt's own editing keys;
  `applyWorkspaceEdit` helper that opens an existing buffer for each
  affected path (firing `lspChangeCmd` to keep gopls in sync) or writes
  to disk via `os.WriteFile` when no buffer is open. Status messages
  report the file count: `applied (3 files)`.
- 15 new host tests in `cmd/nook/main_test.go` covering popup
  arming, stale-response discard, empty-result status, accept-applies-
  to-open-buffer, refuse-disabled, prepareRename-with-range and
  prepareRename-with-zero-range fallback, unavailable-shows-status,
  prompt-accept-fires-rename, esc-cancels, workspace-edit-across-open-
  buffer-and-disk, error-keeps-overlay-open, F2-only-when-buffer-open,
  and alt+enter-triggers-code-actions.

### New keys

- `alt+enter` — request LSP code actions at the cursor. The popup
  lists quickfix / refactor / source-action items; arrow keys
  navigate, enter applies, esc dismisses. Items the server marked
  disabled (e.g. "no enclosing function") surface the reason in the
  status bar instead of applying a no-op edit.
- `f2` — rename the symbol under the cursor. Opens a prompt seeded
  with the current identifier; `enter` commits, `esc` cancels.
  Workspace-wide: every file that touches the symbol gets the new
  name, whether it's an open buffer or only on disk.

### Changed

- Help overlay (`?`) "Language server" section lists `alt+enter` and
  `f2` so the new actions are discoverable.

### Notes

- `prepareRename` is best-effort. gopls returns `{defaultBehavior:true}`
  for renameable identifiers, which decodes to a zero `Range`; the
  host falls back to a source-byte walk from the cursor using
  `isIdentByte` to find the identifier boundaries. Servers that
  explicitly return `Available:false` close the prompt and surface
  "rename not available here" instead.
- Workspace edits are applied atomically per-file: each open buffer's
  edits are computed in source order via `lsp.ApplyWorkspaceEdit`
  before being written, so an edit list that touches three files
  either all-applies or surfaces the first failure.

## [0.9.0] — 2026-05-24

File tree pane inside `nook`. `ctrl+b` toggles a persistent left-side
tree that walks the project root (skipping `.git`, `node_modules`,
`vendor`, `dist`, `target`, and dotdirs), groups directories before
files, and emits a host-level `OpenMsg` when the user presses enter on
a file leaf. Opening a buffer from the picker, project search, or
go-to-definition reveals it in the tree so the cursor always tracks the
active file.

### Added

- `cmd/nook/internal/filetree` — `Pane` value wrapping the glyph
  `components/file-tree` Model with file-system walking, theme-aware
  rounded border, focus/blur gate, and a `BuildTree(root)` helper that
  applies the picker's ignore rules so the two surfaces stay
  consistent. The pane lifts the glyph `SelectMsg` for file leaves
  into a `filetree.OpenMsg` carrying the absolute path; directory
  expand/collapse stays inside the pane. `Reveal(absPath)` expands the
  ancestor chain and moves the cursor onto the target row;
  `Refresh()` rebuilds the tree while preserving the cursor.
- Host integration in `cmd/nook/main.go`: `treePane filetree.Pane` +
  `showTree bool` on the model. `toggleTree()` opens with focus and
  closes with blur; the layout allocates `min(40, max(22, width/5))`
  columns when the tree is visible and shrinks the tree first if the
  editor would drop below the 20-col minimum. Picker / project-search
  / go-to-def / alt+]/alt+[ buffer switches all call `treePane.Reveal`
  so the tree cursor mirrors the active file.
- 11 package tests for `filetree` (ignore rules, dirs-before-files,
  recursion, reveal-and-cursor, focus/blur gating, enter-on-file vs
  enter-on-directory, refresh-preserves-cursor, view-includes-project-
  name, view-empty-when-too-small) and 6 host tests
  (`TestCtrlBTogglesTreePane`, `TestTreePaneEscapeBlursButKeepsVisible`,
  `TestTreeOpenMsgOpensBuffer`, `TestTreeViewRenderedWhenShown`,
  `TestTreeShrinksWhenEditorWouldStarve`,
  `TestTreeRoutesKeysOnlyWhenFocused`).

### New keys

- `ctrl+b` — toggle the file tree pane. Opening also focuses the tree
  so arrow keys navigate it immediately; closing returns focus to the
  editor.
- `esc` (when the tree is focused) — blur the tree without closing it.
  Matches the Cursor / VS Code muscle memory where esc on the explorer
  returns focus to the editor while keeping the side panel visible.
- `enter` (on a file leaf in the tree) — open the file in a new buffer
  (or switch to the existing one), blur the tree, focus the editor.
  Directory rows expand/collapse in place.

### Changed

- Help overlay (`?`) Panes section lists `ctrl+b` so the new pane is
  discoverable.
- `editorSize()` mirrors the tree-aware allocation in `resize()` so the
  welcome card clamps to the editor width when the tree is open.

### Notes

- The walk is eager at construction and on `Refresh()`. For
  ~10k-file projects that's well under 50 ms; lazy expansion is a
  later swap behind the same `BuildTree` contract if profiles ever
  demand it.
- The tree pane respects the same minimum dimensions as the rest of
  the host: below `60 × 12` the layout falls back to the full-editor
  view, and the tree's own `View()` returns `""` when it is allocated
  less than `12 × 4`.

## [0.8.0] — 2026-05-24

Format-on-save inside `nook`. `ctrl+s` now runs the LSP's
`textDocument/formatting` request before writing the buffer when a
language server is attached, so saving a `.go` file with gopls running
produces the same output as `gofmt -w`. `alt+s` saves without
formatting, and `alt+shift+s` toggles the behavior for the session.

### Added

- `lsp.Client.Formatting(ctx, path, tabSize, insertSpaces)` —
  `textDocument/formatting` request returning `[]lsp.TextEdit`
  (Go-native struct with start/end line/col + new text). Nil-guarded so
  binding the save key when no LSP is attached degrades cleanly.
- `lsp.Apply(source, edits)` — pure-function edit applicator. Sorts
  edits descending by start offset and splices each into the source
  byte slice, so overlap-free LSP responses round-trip into the
  buffer without index drift. Clamps out-of-range positions to the
  source end so a stale response can't panic.
- `lookup.FormattingCmd` + `lookup.FormattingMsg` — async wrapper
  matching the existing hover/definition/completion pattern. Echoes
  the buffer version back in the message so the host can discard a
  reply that arrived after the user typed.
- Tests covering `Apply` (empty, whole-file, multi-edit descending,
  insert-only, multiline, out-of-range clamp), the nil-client guard,
  a real gopls round-trip on a deliberately mis-indented Go fixture,
  and the host-side save paths (no-LSP plain save, stale-version
  fallback, edits applied).

### New keys

- `ctrl+s` — save (formats first when an LSP is attached and the
  format-on-save toggle is on; otherwise plain save).
- `alt+s` — save without formatting (escape hatch when the formatter
  would fight a partial edit).
- `alt+shift+s` — toggle format-on-save for the session. Default on.

### Changed

- Help overlay (`?`) Files section lists the new save bindings so the
  keymap is discoverable.

### Notes

- Format-on-save only fires for buffers whose path matches a language
  server the host has connected (today: `.go` via gopls). Other
  buffers fall through to a plain save with no status change.
- A version drift between the request and the response (user typed
  while the formatter was running) discards the edits and writes the
  current buffer with a "save (buffer changed during format)" status,
  so saving never silently overwrites in-flight edits.
- A formatter error that isn't "no language server" surfaces in the
  status bar but still writes the buffer; formatting is treated as an
  optimization, not a save prerequisite.

## [0.7.0] — 2026-05-24

In-buffer find and replace inside `nook`. `ctrl+f` opens a bottom-bar
search scoped to the active buffer, `ctrl+h` flips it into replace mode,
matches highlight in the editor with the active one boosted, and regex
plus case-sensitive toggles cover the cases sed and grep cover.

### Added

- `cmd/nook/internal/finder` — bubbletea sub-component that owns the
  find/replace state (pattern, replacement, focus, regex flag, case flag,
  match list, current index). `Search` does literal substring or regex
  matches across a buffer's lines; `ApplyReplacement` splices a single
  match (capture-group aware via `regexp.ExpandString` so `$1`, `${name}`,
  and friends work). The component exposes `Update`, `View`, `Height`,
  navigation helpers (`Next`, `Prev`, `SelectMatchAt`), and a `Mode` enum
  the host uses to render the right number of input fields.
- `editor.Pane.WithSearchMatches` / `ClearSearchMatches` — per-buffer
  match overlay that paints `surfaceStrong` on non-active matches and a
  bold inverse band on the active one. The hook reuses the existing
  per-rune emit loop so syntax colors keep their foreground while the
  match background layers underneath.
- `editor.Pane.Lines()` — convenience accessor the finder uses to drive
  the regex/substring search without reaching through buffer internals.

### New keys

- `ctrl+f` — open the find bar scoped to the active buffer.
- `ctrl+h` — open the find/replace bar scoped to the active buffer.
- `alt+f` — project-wide search (previously bound to `ctrl+f`).
- `enter` / `↓` — jump to the next match (wraps at end).
- `↑` — jump to the previous match.
- `ctrl+r` — replace the current match and advance.
- `alt+r` — replace every match in the buffer.
- `alt+x` — toggle regex mode.
- `alt+c` — toggle case sensitivity.
- `tab` — switch focus between the find and replace fields (replace
  mode only).
- `esc` — close the find bar and clear match highlights.

### Changed

- The previous `ctrl+f` binding for project-wide search moves to `alt+f`.
  Local find is the high-frequency action; the project search needed a
  modifier-bearing key so it didn't shadow the new in-buffer find.
- Help overlay (`?`) gains a "Find / Replace" section listing every
  binding the bar accepts so the keymap is discoverable.
- Status bar threads the "replaced N occurrences" / "no matches" feedback
  for the replace actions so the user sees the result without leaving
  the bar.

### Notes

- Match highlighting uses the existing theme tokens (`SurfaceStrong`
  for the band, `Primary` + `TextInverse` for the active match) so the
  overlay tracks the active theme.
- Regex mode prefixes the user pattern with `(?i)` when case-insensitive
  is on; the case toggle is a no-op for literal search when the pattern
  is all-lowercase.
- Empty patterns clear matches and the cursor without surfacing an
  error; an invalid regex shows the compile error inline in the bar.
- The find bar reserves either one row (find-only) or two rows
  (find + replace) below the editor; `resize()` subtracts those rows
  from the editor's body height so nothing scrolls under the bar.

## [0.6.0] — 2026-05-24

Language-server intelligence inside `nook`. Hover info, go-to-definition,
and a completion popup wire the existing `nook/internal/lsp` client into
keystrokes you can demo against any Go workspace gopls knows about.

### Added

- `cmd/nook/internal/lookup` — async `tea.Cmd` factories for hover,
  definition, and completion lookups. Each carries the request inputs in
  its response message so a late answer past a moved cursor gets
  discarded. nil-client returns a typed message with `errNoClient`
  instead of panicking, so the keys can be bound unconditionally.
- `cmd/nook/internal/hover` — rounded-border overlay component for the
  symbol info `lsp.Client.Hover` returns. Hard-wraps and clamps long doc
  blocks to a third of the screen height; renders empty for empty input
  so the host can drop it into the View pipeline unconditionally.
- `cmd/nook/internal/complete` — popup menu component for completion
  results. Owns selection state, viewport scrolling, and a `WordPrefix`
  helper for the identifier the host needs to delete before inserting.
- `lsp.Client.Hover`, `lsp.Client.Definition`, and `lsp.Client.Completion`
  with friendly Go-native return types (`HoverInfo`, `[]Location`,
  `[]CompletionItem`) so the editor never imports the LSP protocol
  package.
- `Hover`, `definitionProvider`, and `completionItem` client capabilities
  on initialize so gopls knows what we accept (markdown docs, link
  support off, plaintext+markdown hover, snippets off).

### New keys

- `alt+i` — hover info for the symbol under the cursor.
- `ctrl+]` — go to definition (opens the target file in a new buffer
  when it lives outside the current one).
- `ctrl+space` — completion popup. `↑/↓` navigate, `Enter` accepts, `Esc`
  dismisses, any other key dismisses and falls through to the editor.

### Changed

- Help overlay (`?`) gains a "Language server" section listing the three
  new keys.
- Status bar surfaces concise feedback for each LSP action ("no hover
  info", "jumped to file:line:col", "no completions", "inserted Println",
  etc.) so users know what gopls answered without leaving the buffer.

### Notes

- Hover and definition queries time out after 2s. gopls answers in
  sub-second on a warm workspace; the cap mostly catches a frozen server
  or a stdio deadlock.
- The completion popup is currently centered (modal float). A future
  release will anchor it next to the cursor; the popup-component API is
  already cursor-agnostic so the host can switch position without
  changing the component.

## [0.5.0] — 2026-05-24

Syntax highlighting in `nook`. Go, TypeScript, JavaScript, Python, Rust, and
Markdown render with keyword/string/comment/number/function/type/punctuation
spans the moment a file opens, with no layout shift and no perceptible delay.
The highlighter is theme-aware so the palette tracks `theme.Default` and
`theme.Light` instead of hardcoding ANSI.

### Added

- `cmd/nook/internal/highlight` — pluggable highlighter interface plus a
  [chroma](https://github.com/alecthomas/chroma) implementation covering
  250+ languages out of the box. Output is a sparse `Result{Rows: map[int][]Span}`
  the editor walks alongside the existing line buffer; spans are byte-offset,
  non-overlapping, never cross a newline, and never exceed line length.
- `components/theme`: seven new palette tokens — `SyntaxKeyword`,
  `SyntaxString`, `SyntaxComment`, `SyntaxNumber`, `SyntaxFunction`,
  `SyntaxType`, `SyntaxPunctuation`. `theme.Default` ships a VS Code-style
  dark palette; `theme.Light` ships a deeper-saturation parallel. Empty
  values fall back to the muted/text colors so the editor stays readable
  on user themes that don't opt in.

### Changed

- `editor.Pane` now caches highlight results behind a per-buffer `bufVer`
  counter. Tokens are recomputed only on actual buffer mutations, so the
  hot `View()` path doesn't retokenize per frame.
- `bufman.Manager` owns the shared `Highlighter` and rewires existing panes
  when `WithHighlighter` is called, so theme switches don't strand stale
  spans on open tabs.
- Row rendering moved into a single `renderHighlightedRow` pass that walks
  bytes once and handles spans, cursor cell, ghost text, tab expansion,
  and width clipping together. Contiguous same-kind runes batch into one
  `lipgloss.Style.Render` call to keep ANSI overhead bounded.

### Notes

- Binary size grows from ~5.6 MB → ~12.8 MB. Chroma embeds its lexer
  registry, which is what makes `go install @latest` work with zero extra
  setup. CGO stays off; tree-sitter (via WASM) remains a future drop-in
  behind the same `Highlighter` interface.

## [0.4.0] — 2026-05-24

Multiple open buffers in `nook`. Opening a second file no longer replaces the
first. Picker selections and search jumps now route through a buffer manager
that appends new buffers, switches to already-open ones, and reuses the empty
welcome pane in place on first open.

### Added

- `cmd/nook/internal/bufman` — buffer manager owning the open-buffer
  collection. Pointer receiver so tab switches stay atomic against the
  host's routing reads.
- `cmd/nook/internal/tabbar` — tab strip above the editor. Basename labels,
  parent-dir disambiguation when basenames collide, dirty marker (●), and
  overflow that walks outward from the active tab so the user always sees
  what they were last editing.
- Keymap. `alt+]` / `alt+[` cycle buffers, `ctrl+w` closes the active
  buffer (refuses to close while dirty). Closing the last buffer brings
  back the welcome card.

### Changed

- LSP tracking moved to per-path. Closing a Go buffer sends `didClose` to
  gopls and clears the per-path diagnostics + version state, so a stale
  buffer's findings don't leak into the next file.

## [0.3.1] — 2026-05-24

First-run UX for `nook`. Opening `nook .` on a fresh checkout used to land on a
blank editor with tilde-fill — accurate for someone who already knows the
keymap, hostile for a brand-new user. This release replaces that surface with a
welcome card and adds a discoverable full keymap overlay.

### Added

- `cmd/nook/internal/welcome` — centered first-run card with the `nook`
  wordmark, project name, file count, runtime status for AI and LSP
  (green dot = ready, install hint when not), and the canonical quick-start
  keys. Rendered automatically whenever no file is open.
- `cmd/nook/internal/help` — full keymap overlay grouped by job (Files,
  Editing, AI wedges, Panes, Global). Bound to `?` (only when no file is
  open so it doesn't shadow typing) and dismissed by `?` or `esc`.
- Richer status bar. Now shows hint · project · `L<row>:<col>` · dirty
  marker · LSP `nE nW` summary, separated by faint dot bullets so the
  segments group at a glance.
- `docs/nook/visuals/welcome.cast` and `welcome.gif` capturing the full
  welcome → keymap → open → edit → save flow.

## [0.3.0] — 2026-05-24

First release of `nook`, a terminal-native AI IDE built from glyph primitives
and shipped from the same module. Install with
`go install github.com/truffle-dev/glyph/cmd/nook@latest`, or grab a tagged
binary from the release assets.

### Added

- `cmd/nook` — a five-pane TUI host (file picker, search, git, editor,
  embedded terminal) wired together over a single `tea.Model`. Phase 1 of
  the proto-IDE described in `docs/nook/`.
- nook AI wedges. `Ctrl+K` inline edit, `Ctrl+L` Composer panel, and idle
  ghost-text suggestions. All three stream tokens through the user's local
  `claude` CLI subprocess — no separate Anthropic API key, no direct
  `api.anthropic.com` calls.
- nook LSP integration. `gopls` diagnostics render as a `●` gutter marker
  next to offending lines and as an `nE nW` summary in the status bar.
- nook visuals. Cast + GIF recorder under `docs/nook/visuals/` produces the
  `lsp.gif` and `tour.gif` reels on the landing page.

### Fixed

- `pathFromURI` on Windows now goes through `uri.URI.Filename()` instead of
  string trimming, so document URIs with drive letters and percent-escaped
  spaces map back to the correct local path.

### Notes

- Release artifacts now include `glyph_<version>_<os>_<arch>` and
  `nook_<version>_<os>_<arch>` tarballs (zip on Windows). Both binaries are
  built from the same module at the same tag.

## [0.2.0] — 2026-05-23

Seven new components covering forms, overlays, and data display, plus three
runnable examples that compose them. Two visual gaps from the v0.1 release
also close in this version: the per-component gallery now actually renders on
github.com, and a no-Chrome `visuals/render-cast.sh` pipeline replaces the
previous tape-based recorder.

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
- Per-component gallery GIFs under `visuals/out/<name>.gif`, now tracked in
  the repo so the README gallery resolves on github.com. The v0.1.0 entry
  named these but the files were excluded by `.gitignore`; that flip lands
  here.
- `visuals/render-cast.sh`, an asciinema + agg rendering pipeline that
  produces the gallery GIFs without needing headless Chrome. Bubble Tea
  stories that never exit on their own use a sibling `story/snap.go` with
  build tag `glyph_snap` so the recorder captures a single rendered frame.
- `Makefile` with a `ci-local` target that mirrors the GitHub Actions gates
  (test, vet, gofmt, build) for the whole tree in one command.

### Changed

- `cmd/glyph`: source-build `version` constant bumps to `0.2.0-dev`. Release
  binaries continue to be tagged by goreleaser at build time.

### Notes

- The headline reel under `examples/reel/` still walks the v0.1 primitives.
  The seven v0.2 components compose into `examples/chat-cli`,
  `examples/log-viewer`, and `examples/dashboard` instead; those have their
  own gallery GIFs. Extending the reel to a v0.2 scene is queued for a
  later patch release.

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

[Unreleased]: https://github.com/truffle-dev/glyph/compare/v0.7.0...HEAD
[0.7.0]: https://github.com/truffle-dev/glyph/releases/tag/v0.7.0
[0.6.0]: https://github.com/truffle-dev/glyph/releases/tag/v0.6.0
[0.5.0]: https://github.com/truffle-dev/glyph/releases/tag/v0.5.0
[0.4.0]: https://github.com/truffle-dev/glyph/releases/tag/v0.4.0
[0.3.1]: https://github.com/truffle-dev/glyph/releases/tag/v0.3.1
[0.3.0]: https://github.com/truffle-dev/glyph/releases/tag/v0.3.0
[0.2.0]: https://github.com/truffle-dev/glyph/releases/tag/v0.2.0
[0.1.1]: https://github.com/truffle-dev/glyph/releases/tag/v0.1.1
[0.1.0]: https://github.com/truffle-dev/glyph/releases/tag/v0.1.0
