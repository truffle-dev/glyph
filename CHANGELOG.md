# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
