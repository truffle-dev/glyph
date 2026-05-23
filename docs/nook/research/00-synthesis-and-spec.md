# nook — terminal-native AI IDE, built from glyph

Synthesis of eight research reports into a buildable product spec. This is the
foundation doc for the end-to-end Cursor-replacement-in-the-terminal project.

> **Status:** spec only. Commit this, then build.
> **Date:** 2026-05-23.
> **Name:** `nook`. Four letters, easy to type, friendly, evokes "your own little
> corner of the terminal." Fits the glyph lineage (short, simple noun).

---

## 1. The wedge in one sentence

A terminal-native AI IDE that opens any project instantly, runs over SSH,
ships as a single binary, and is built from glyph primitives that anyone can
copy-paste into their own editor.

## 2. Why this, why now

The four categories of competitor each leave a clean lane:

| Competitor | What it does well | What it leaves on the table |
|---|---|---|
| **Cursor** | Best-in-class AI UX (Cmd-K, Composer, @-mention), tight model loop | Electron memory (8-16 GB on monorepos), opaque pricing (June 2025 credit-system blowup), no SSH/server use, vendor lock-in |
| **Helix** | Selection-first model, native LSP, picker UX, single binary | No AI integration, no plugin system, no terminal pane, no git workflow built in |
| **Neovim + plugins (Avante, CodeCompanion, copilot.lua)** | Mature plugin ecosystem, configurable to anything | Hours of `init.lua` tuning, fragile when plugins update, requires Lua/vimscript fluency |
| **aider** | Terminal-native, git-native commits, repo-map context, transparent | It's a REPL, not an IDE. No file tree, no live editing, no LSP, no persistent layout |
| **Zed** | Multibuffer, inline assistant, GPU rendering | Not a TUI; can't SSH into it; high resource floor |

The unfilled space: an editor that combines **Cursor's AI UX**, **Helix's
picker-driven navigation**, **aider's terminal+git ergonomics**, and **Zed's
multibuffer pattern** — in a TUI single binary. That's nook.

## 3. Architecture: host model + primitives

The composition pattern proven in `examples/code-editor` scales. Each pane is
a glyph component (`tea.Model` with its own state, update, view). The host
model is a thin orchestrator that:

1. Owns workspace state (files, focus, layout, LSP servers, git status)
2. Routes input messages to the focused panel
3. Wires cross-cutting events (LSP diagnostics, search results, git changes)
4. Manages async work (LSP, ripgrep, git, AI streams) via `tea.Cmd` channels

Primitives don't know about each other. Wiring lives in the host. This is the
same architecture that lets shadcn primitives compose into any layout.

```
┌─────────────────────────────────────────────────────────────────────┐
│ nook · main.go · go · 1 of 4 · main↑2 · gopls✓                       │ ← status-bar
├─ tree ──────┬─ tabs ────────────────────────────────────────────────┤
│ ▾ cmd       │ main.go · root.go · agent.go                          │ ← tabs
│   main.go ● │ ──────────────────────────────────────────────────────│
│   root.go   │  1 package main                                       │
│ ▾ internal  │  2                                                    │
│   agent.go  │  3 import (                                           │ ← editor
│ go.mod      │  4   "context"                                        │   (focus)
├─────────────┤  ...                                                  │
│ ░ search    │                                                       │
│  Agent (12) │                                                       │
│ ░ chat      ├───────────────────────────────────────────────────────┤
│  > refactor │  $ go test ./... ─────────────────────────────────────│ ← terminal
│            │  ok  ./cmd  0.041s                                     │   pane
│            │  FAIL ./internal/agent (token.go:42)                   │
├─────────────┴───────────────────────────────────────────────────────┤
│ ● editing · cmd/main.go        Ln 12, Col 5      diagnostics: 0 / 1 │ ← status
│ Ctrl-L tree  Ctrl-E edit  Ctrl-F find  Ctrl-Space chat  Ctrl-` term │ ← key-hints
└─────────────────────────────────────────────────────────────────────┘
```

### State shape

```go
type Model struct {
    // workspace
    root      string                 // project root (cwd or arg)
    files     map[string]*FileEntry  // tracked files (path → entry)
    git       *GitState              // branch, ahead/behind, dirty, status
    lsp       map[string]*LSPClient  // language → running server
    diagnostics map[string][]Diagnostic  // path → diagnostics

    // layout
    focus     focusID                // which pane has keyboard
    layout    LayoutKind             // tree+editor, tree+editor+chat, etc.
    width     int
    height    int

    // panes (each is a glyph component)
    tree      filetree.Model
    tabs      tabs.Model
    bufs      []*openBuffer          // open buffers (path + editor.Model)
    active    int                    // index into bufs
    bar       findbar.Model          // in-file find
    search    *searchPane            // project search (multibuffer)
    chat      *chatPane              // AI chat sidebar
    term      *terminalPane          // pty-backed shell
    status    statusbar.Model
    hints     keyhints.Model
    palette   palette.Model          // command palette
}
```

### Async work pattern

LSP, ripgrep, git, and AI all run async. The pattern (validated in the
`go.lsp.dev` research):

1. Spawn worker goroutine (LSP server, ripgrep subprocess, git command, HTTP stream)
2. Worker emits typed messages to a channel
3. Host model has a single `tea.Cmd` that reads from the channel and returns
   the message to the tea runtime
4. `model.Update(msg)` handles the message and updates state

This keeps the UI lock-free. The proof points in research:

- LSP server: long-lived goroutine reads server stdout, posts
  `LSPDiagnosticMsg{Path, Diagnostics}` to the model
- ripgrep: subprocess streams matches; goroutine parses and posts
  `SearchMatchMsg{Path, Line, Text}` per row
- AI: HTTP stream; goroutine posts `ChatTokenMsg{Token}` per chunk
- Git: short-lived; one-shot Cmd that returns `GitStatusMsg` when complete

## 4. The MVP (Phase 1: the day-1 binary)

What does "end-to-end" actually mean? A developer can run `nook` in any
project and complete the 5 must-have verbs from the dev-workflow research:

1. **Open file / search by name** — fuzzy file picker, project-wide
2. **Search code (ripgrep) and navigate matches** — multibuffer-style results
3. **Edit + save + auto-test** — type, save, run command, see output inline
4. **Jump to error** — click in search results / terminal output → file:line
5. **Commit + push** — git status, stage, diff, commit, push

These five give a real day's work. Everything else is Phase 2+.

### MVP component manifest

| Component | Status | Source |
|---|---|---|
| theme | shipped | `components/theme` |
| panel | shipped | `components/panel` |
| status-bar | shipped | `components/status-bar` |
| key-hints | shipped | `components/key-hints` |
| file-tree | shipped | `components/file-tree` |
| tabs | shipped | `components/tabs` |
| editor | shipped | `components/editor` |
| find-bar | shipped | `components/find-bar` |
| code-view | shipped | `components/code-view` |
| diff-view | shipped | `components/diff-view` |
| command-palette | shipped | `components/command-palette` |
| text-input | shipped | `components/text-input` |
| list | shipped | `components/list` |
| **picker** | **NEW** | overlay over `text-input` + `list` with fuzzy match + preview pane |
| **search-pane** | **NEW** | ripgrep streaming results, multibuffer-style with file boundaries |
| **git-pane** | **NEW** | `git status` + staging + diff view + commit input |
| **terminal-pane** | **NEW** | pty-backed shell (creack/pty), capture output, parse file:line for jump |
| **lsp-status** | **NEW (later)** | tiny widget for status-bar showing connected servers |

5 new glyph components to ship for MVP. All inherit the same shape (component
JSON manifest, README, story+snap, tests).

### MVP host (the `nook` binary)

Lives at `cmd/nook/` in the glyph repo. ~800-1500 lines of orchestration:

- workspace discovery (walk `root`, gather files, detect git, detect language)
- ripgrep subprocess runner with streaming
- git subprocess runner (status, diff, add, commit, push)
- pty manager for terminal pane
- layout manager (resize, split, focus shift)
- keymap (the 5 must-have verbs bound to chords)
- pane orchestration (open file in tabs from picker/search/tree)

## 5. The Phase 2 wedge — what makes nook better than vim+plugins

Once Phase 1 is shipped (the day-1 IDE), Phase 2 is what makes it
*irreplaceable*. From the research, these are the differentiators:

### LSP integration (own primitive, not bolted on)

- `go.lsp.dev/protocol` + `jsonrpc2` to talk to gopls, rust-analyzer,
  pyright, typescript-language-server
- Per-workspace lifecycle (spawn on first relevant file open, kill after idle)
- Diagnostics rendered in: gutter (●), inline squiggle, status-bar summary
- Goto-definition + find-references → opens a multibuffer pane with all hits
- Hover docs → status-line preview or modal popup
- Completion → ghost-text first, picker fallback (Helix's pattern)

### AI panel (Cursor-grade, terminal-shaped)

- **Inline edit (Ctrl-K)**: select text → prompt → diff preview → accept/reject.
  Uses `diff-view` component for the preview.
- **Chat sidebar (Ctrl-Space)**: glyph `chat-thread` + `chat-input` panel,
  `@file` and `@symbol` mentions, streaming output
- **Apply across files (Composer-equivalent)**: AI proposes edits across N
  files → all shown in a *multibuffer* (Zed pattern) → accept all / per file
- **Streaming** via `tea.Cmd` channel of `ChatTokenMsg` (proven pattern)
- **Provider abstraction**: Claude (default), OpenAI, Ollama. One `Provider`
  interface, three implementations.
- **Auto-commit**: aider's pattern — each AI-applied edit is its own commit
  with attribution (`Co-Authored-By: Claude <noreply@anthropic.com>`)

### Multibuffer (Zed's standout)

A scrollable buffer that contains slices of multiple files with visible file
boundaries. Used for:
- Project search results
- Find-references results
- AI multi-file proposals
- Git unstaged-changes review

The user edits across files in *one* view. Save propagates to underlying
buffers. This is the pattern no current TUI editor implements well.

### Picker UX (Helix's standout)

`<Space>` opens a context palette showing what verbs are bound. Each picker
has:
- Fuzzy-matched results
- Live preview pane (file content, symbol context, diagnostics)
- Pagination + scrolling
- Multiselect (`<Tab>`) for batch ops

Implemented once as a `picker.Model` component, then specialized: file
picker, buffer picker, symbol picker, command picker, branch picker.

## 6. Phase 3 — the magnet

If Phase 2 lands, nook is the best terminal IDE in the world. Phase 3 is what
makes it the editor people choose *over* Cursor:

- **Worktree-style multi-repo**: open a sibling repo without closing the
  current session. Tabs/buffers across roots.
- **Agent mode**: a session-bound agent (claude-code-style) that can run in
  the background and report into a chat panel
- **Plugin system**: a stable extension point (HTTP-mode or Go-plugin) for
  custom commands, snippets, themes
- **Persistent sessions**: restore tabs, layout, search-state on restart
- **Devcontainer / remote dev**: nook is already SSH-friendly; phase 3 adds
  one-command spawn-on-remote

## 7. Build order — what we do this session

The instruction was: build end-to-end, multi-hour, email when done. So the
priority is *getting one binary you can actually use*, not feature parity.

```
1. Bootstrap cmd/nook/        ← workspace + layout + minimal model
2. Picker component            ← fuzzy file + buffer picker; reusable
3. Search pane (ripgrep)       ← project search with file-grouped results
4. Git pane (status + diff)    ← status list, stage/unstage, commit input
5. Terminal pane (pty)         ← creack/pty for shell-out
6. Wire it all into nook      ← keymaps, focus routing, open-from-X glue
7. Test end-to-end on a real repo
8. Polish + commit + push
9. Email Cheema
```

LSP and AI are Phase 2 — they're real lift, and a working "vim with picker +
search + git + terminal" is already a usable editor. Ship that first; layer
the AI on top.

## 8. The bar

From the previous session's learning: "each component should be best-in-class
quality; demos should be increasingly complex and show real-world usage
patterns." The nook binary is the most complex composition demo yet. Quality
gates stay the same:

- Each new component ships with `<name>.json` manifest, README, story+snap
  with `glyph_story` / `glyph_snap` tags, unit tests, GIF preview
- The `nook` binary itself has integration tests for the host model
- README at `cmd/nook/` explains what it is and how to run it
- All commits go in via the standard glyph PR voice (no "Generated with"
  trailers; truffle/truffleagent@gmail.com signature is the disclosure)

## 9. Open questions tabled to Phase 2+

- Modal vs modeless? (Phase 1 is modeless+chord; Phase 2 can opt-in modal)
- Tree-sitter for richer syntax? (currently regex-tinted via `code-view`;
  good enough for Phase 1)
- Theme switching from inside the editor? (Phase 2)
- Settings file? (Phase 2 — `.nook/settings.json` at project root)
- Plugin system? (Phase 3)
- Web/browser nook (over WebSocket TUI)? (Phase 4 if at all)
