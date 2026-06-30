# Split panes: host wiring plan

Status: design. The geometry foundation (`cmd/nook/internal/splitlayout`)
is built and unit-tested but not referenced by the host. This note plans
the wiring so the feature lands as a sequence of complete, green slices
instead of one multi-hour change that risks half-landing in the central
editor path.

## Where the host is today

The host renders exactly one editor viewport.

- `model.View()` assembles the body as `tree + renderMainColumn() + right`,
  joined horizontally (`main.go`, the `pieces` assembly).
- `renderMainColumn()` is `m.bufs.Active().View()` — the single active
  buffer, full editor rect.
- `editorSize()` computes that one rect (tree width on the left, right
  pane on the right, a 20-column floor).
- Every editing path keys off `m.bufs.Active()` (cursor moves, saves,
  completion, LSP requests, find/replace — grep `m.bufs.Active(` shows
  ~60 call sites).

`bufman.Manager` is a single-active-buffer model. The tab bar cycles the
active buffer; it never shows two buffers at once.

## The central entanglement

Two facts make split panes more than a render change:

1. **One size for all buffers.** `bufman.WithSize(w, h)` loops over every
   open buffer and sets them all to the same `w, h`:

   ```go
   func (m *Manager) WithSize(w, h int) {
       m.width, m.height = w, h
       for i := range m.panes {
           m.panes[i] = m.panes[i].WithSize(w, h)
       }
   }
   ```

   `editor.Pane.View()` takes no arguments; it renders at its stored
   size. So two on-screen panes need two different stored sizes, which
   the blanket `WithSize` actively fights. The wiring must size each
   *bound* buffer to its own pane rect, and the `WindowSizeMsg` handler
   must stop calling the blanket setter once a split is live.

2. **Editing routes through the active buffer.** Rewriting all ~60
   `m.bufs.Active()` call sites to take a "which pane" argument would be a
   huge, error-prone diff. The cheap win: **make pane focus drive
   `m.bufs.Switch(idx)`.** The focused pane's buffer *is* the active
   buffer, so every existing handler operates on the focused pane with
   zero changes. Focus and active-buffer become one concept.

## Hard decisions, pinned now

- **One buffer per pane, distinct, in v1.** Each split pane binds to a
  different `bufman` buffer index. Showing the *same* buffer in two panes
  with independent scroll/cursor needs detachable viewport state on
  `editor.Pane` and is explicitly deferred (it is the harder half and not
  required for the headline Zed-parity win of "see two files at once").
- **Focus == active buffer.** Single source of truth. No per-pane cursor
  bookkeeping in the host; `bufman` already owns each buffer's cursor.
- **Dividers are free geometry.** `Tree.Rects()` already reserves a
  1-cell gap between panes; `Tree.Dividers()` returns the line positions.
  Rendering is lipgloss, no new geometry.
- **First-paint rule holds.** `splitlayout.New()` is constant-time and does
  no I/O; adding the tree to `newModel` does not touch startup latency.
- **Recursive pump is unaffected.** Split panes add no streaming source.
  The search and terminal pumps target the active buffer, which is the
  focused pane — they keep working unchanged.

## Slices

Each slice is complete on its own, ships with green tests, leaves no dead
keybinding, and keeps the binary starting instantly.

### Slice 1 — bind the tree, no behavior change — LANDED

Done. `model` now carries `split *splitlayout.Tree` and
`paneBuf map[splitlayout.PaneID]int`; `newModel` seeds one pane bound to
buffer 0 (constant-time, no I/O). `editorSize()` routes the region —
computed by the renamed `editorRegion()` — through `split.Rects()` and
returns the focused pane's rectangle, which for one pane equals the region
exactly. `TestEditorSizeSinglePaneMatchesLegacy` pins that equality against
a frozen copy of the pre-split region math across a width/height/tree/right
matrix, so the single-pane path can never drift. Original plan below.

- Add `split *splitlayout.Tree` and `paneBuf map[splitlayout.PaneID]int`
  (pane → `bufman` index) to `model`. In `newModel`, `New()` yields one
  pane bound to buffer 0.
- Route `renderMainColumn()` and `editorSize()` through
  `split.Rects(w, h)`. With one pane the rect equals the full editor area,
  so output is byte-identical to today.
- No split keybinding yet, so no dead keys.
- Tests: for a matrix of widths, heights, tree-shown, and right-pane
  states, the single-pane rect equals the legacy `editorSize()` result.

This slice is the integration milestone — it reconciles `splitlayout`
geometry with the existing tree/right-pane sizing math, which is the real
risk, while changing nothing the user sees.

### Slice 2 — split, close, render two panes

Slice 2 split in two because the render half and the keybinding half carry
independent risk. The render compositor is keybinding-free and testable by
driving the tree directly; the keys need a conflict resolved first.

#### Slice 2a — render compositor + per-buffer sizing — LANDED

Done. `bufman.SetSizeAt(idx, w, h)` sizes one buffer to its own pane rect
(the blanket `WithSize` could only set them all to one size). `resize()`
branches: when `split.Count() > 1` it walks `paneBuf` and sizes each bound
buffer to `split.Rects(leftW, bodyH)[pid]`; otherwise it keeps the legacy
blanket setter. `renderMainColumn()` calls `renderSplitPanes()` when
`split.Count() == 2`, which orders the two panes by screen position, forces
each `buffer.View()` into a fixed `rect.W × rect.H` lipgloss block, and joins
them with a themed divider whose orientation comes from `split.Dividers()[0]`
(vertical bar for Columns via `JoinHorizontal`, horizontal rule for Rows via
`JoinVertical`). The composite fills the editor region exactly. Tests drive
the split tree directly (no keybinding yet) and assert both orientations fill
the region and show both buffers; a mutation-proof confirms the width guard
goes red on a doubled divider. **v1 caps at two panes (one split).** A general
N-pane compositor is deferred until a real use case needs it.

#### Slice 2b — split / close keybindings — LANDED

Done, with one decision changed from the plan below. **The window leader is
`alt+w`, not a pending-`ctrl+w` prefix.** The pinned ctrl+w-prefix idea was
abandoned because `ctrl+w` is already bound to `closeActiveTab()` and several
existing tests (`TestTabFlow`, `TestTabFlowDirtyBlocksClose`) press a single
`ctrl+w` expecting an immediate tab close — a prefix state would have made
the first `ctrl+w` swallow that close, breaking both the tests and the
committed promise to preserve close-tab muscle memory. `alt+w` is free, reads
as "window," and fits nook's existing alt-leader idiom (alt+v markdown
preview, alt+] / alt+[ tab cycle). `ctrl+w` stays exactly as it was.

- **`alt+w` arms a one-shot `awaitingWindowKey` state.** A handler at the top
  of `routeKey` reads it before the global switch, so the chord's second key
  (a plain rune) is consumed there instead of being typed into the buffer.
  The next key runs a window op or disarms.
- `alt+w v` (Columns / split right) and `alt+w s` (Rows / split down).
  `splitPane` requires ≥2 open buffers — a lone-buffer split would show the
  same content twice, which per-pane sizing cannot honor — and v1 caps at one
  split (two panes). It binds the new pane to the next open buffer and
  `bufs.Switch`es to it, keeping the focus==active invariant.
- `alt+w c` closes the focused pane: `CloseFocused`, drop the `paneBuf`
  binding, re-switch the active buffer to the surviving pane's binding. The
  buffer stays open as a tab; only the pane binding drops, so no bufman
  reindex is needed. `CloseFocused` already refuses the last pane.
- Tests (`main_splitpane_test.go`): `alt+w v` / `alt+w s` raise pane count and
  produce the right divider orientation; `alt+w c` returns to one pane with
  both buffers still open; `alt+w` + an unrelated key disarms without
  splitting; split is refused with a single buffer; existing `TestTabFlow`
  confirms bare `ctrl+w` still closes a tab.

#### Slice 3a — focus routing — LANDED

Done. The split is now navigable from the keyboard. `alt+w h/j/k/l` calls
`FocusDir(Left/Down/Up/Right, w, h)` and `alt+w w` calls `FocusNext`; both
route through `syncFocusedPane`, which restores the focus==active invariant by
`bufs.Switch`ing to the newly focused pane's bound buffer, then reapplies
diagnostics and sizing — so every existing editing path operates on the
focused pane with no per-pane bookkeeping. A `FocusDir` with no neighbour that
way (e.g. `alt+w k` in a side-by-side columns split) is a no-op with a hint;
the focus chords are inert with a single pane. The directional geometry is
covered by `splitlayout`'s own `TestFocusDir`; the host tests cover the wiring
(`TestWindowLeaderFocusNextSyncsBuffer` proves the active buffer follows
focus), the dead-end (`...FocusDirNoNeighborIsNoOp`), and the single-pane
no-op (`...FocusNoSplitIsNoOp`).

Affordance: the live-split divider is tinted with the accent (`theme.Primary`)
rather than the quiet border colour, so a split reads as an active workspace at
a glance. **Which** pane holds focus is shown by the cursor — the focused pane
is the active buffer (focus==active) and only it draws the editing cursor. A
per-pane highlighted border was considered and deferred: borders consume cells
and would fight the "composite fills the region byte for byte" invariant; the
cursor is the honest focus indicator for a two-pane v1.

#### Slice 3b — keyboard resize — LANDED

Done. `alt+w >` / `alt+w <` shift the divider via `resizePane(delta)`, which
calls `split.ResizeFocused(delta)` then `m.resize()` so each bound buffer
re-sizes to its new rect. The sign is folded inside `ResizeFocused` (ratio is
child a's fraction; it adds `delta` when the focused pane is child a and
subtracts it when child b), so a **positive delta always grows the focused
pane** — `>` grows, `<` shrinks, regardless of which side of the split holds
focus. `clampRatio` bounds the ratio to `[minRatio, maxRatio]` so neither pane
can collapse. The chord is inert with a single pane (hint, no-op). The leader
hint now reads `< / > resize`. Tests (`main_splitpane_test.go`): `>` grows and
`<` shrinks the focused pane's rect width; 40 repeated shrinks never drive a
pane to zero width (clamp proof); the chord is a no-op with one pane.

The mouse split was deferred to keep this slice complete and green. Keyboard
resize is the interactive capstone; mouse needs its own screen-coordinate map.

#### Slice 3c — mouse focus — NEXT

- Mouse is already enabled (`tea.WithMouseCellMotion()` at `main.go`).
- Click → map screen coords (tree-width left offset, header rows) to an editor
  region point → `split.PaneAt` → `Focus` → `syncFocusedPane`.
- Tests: a click in each pane's rectangle selects that pane; a click in the
  tree or header is ignored.

### Slice 4 — reconciliation polish

- Opening a file (picker, finder, go-to-def, tree) targets the focused
  pane's binding rather than always the active tab.
- Closing a *buffer* (not a pane) reconciles `paneBuf` indices so no pane
  points at a freed slot.
- Per-empty-pane welcome card when a pane's buffer is closed out from
  under it.

## Out of scope (record so it is not rediscovered)

- Same buffer in two panes with independent viewports — needs detachable
  viewport state on `editor.Pane`. Revisit only after slices 1–4.
- Collab / liveshare — out of scope for nook entirely (see ROADMAP).

## Order of operations

Land 1 first; it is pure de-risking and unlocks the rest. 2 and 3 are
each a single visible feature. 4 is cleanup that can trail. None of the
four needs the deferred independent-viewport work.
