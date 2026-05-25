# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
