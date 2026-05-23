# Zed: Distinctive Patterns Worth Borrowing for TUI Design

Zed is a non-Electron, GPU-accelerated code editor built in Rust that prioritizes performance and AI-native design. While optimized for a graphical environment, several of Zed's architectural and UX patterns translate valuably to TUI contexts. This report examines Zed's distinctive approach across ten dimensions and evaluates which patterns are worth adapting for a terminal-based editor like Glyph.

## 1. Multibuffer: Composing Multiple Files in One View

**How it works:**  
Zed's multibuffer feature allows users to view and edit excerpts from multiple files in a single scrollable buffer, making it a "superpower" for wide-ranging refactors. When you run a project search (`Cmd+Shift+F`), results appear as a multibuffer containing one excerpt per matching line with surrounding context. File boundaries are shown inline as visual dividers or as discrete excerpt sections.

**UX details:**
- Users can `option-click` (or `alt-click` on Windows/Linux) to place multiple cursors across excerpts
- `editor: select all matches` enables batch edits spanning files
- `editor: open excerpts` navigates back to the source file at each cursor location
- Changes sync instantly to open copies of each file in other tabs
- Users save all modified files with a single `editor: Save` command

**Cross-file editing:**  
Yes—multiple cursors allow simultaneous edits across file boundaries within a single multibuffer. This is transformative for refactorings like renaming symbols, adjusting function signatures, or applying patterns uniformly.

**Worth borrowing in TUI?**  
**Highly valuable.** A TUI multibuffer could use visual dividers (e.g., `─────── file.ts:42 ───────`) and keyboard navigation between excerpts. Multiple cursors in TUI are already feasible with Vim-style marks and parallel editing. The killer benefit—editing many files without context-switching—is as powerful in a terminal as on desktop.

---

## 2. AI Panel + Inline Assistant: Native AI Workflow

**Zed's AI architecture:**  
Zed integrates AI natively via two mechanisms:

1. **Inline Assistant (Cmd+K):** Select code or a terminal command, describe desired changes, and the assistant rewrites the selection in place. Supports multiple cursors for batch transformations. No separate panel; edits happen directly in the buffer.

2. **Agent Panel:** A persistent conversation interface where you send prompts to AI agents. The agent monitors progress in real time, reads and writes code, and runs commands. You review proposed changes before committing them.

**Multi-file context:**  
Unlike Cursor's static context selection, Zed's agent system can ingest entire file trees and symbol indices via semantic search, giving agents access to cross-file relationships without explicit inclusion.

**How it differs from Cursor:**  
- **No modal disruption:** Inline assistant modifies code in place; Cursor opens a side panel with diff preview.
- **Bidirectional context:** Agents can read from the entire workspace and write back selectively.
- **Agent control:** You can set tool permissions and behavioral rules for agents (via MCP servers), constraining what they're allowed to do.

**Worth borrowing in TUI?**  
**Absolutely.** The inline assistant paradigm (Cmd+K, select, describe, edit-in-place) maps perfectly to TUI. No need for panels; the editor becomes the canvas. Agents running in the background with a status indicator in the status bar (e.g., `[Agent: refactoring...]`) and a keybind to review pending changes is highly feasible.

---

## 3. Channels & Real-Time Collaboration

**Architecture:**  
Zed's collaboration uses CRDT (Conflict-free Replicated Data Types) and operational transformation to sync edits across users. Multiple people see each other's cursors and edits in real time. Access is managed via "channels" (persistent team project spaces) and private calls (ad-hoc sessions).

**Security:**  
Sharing a project grants collaborators access to the local file system within that project scope. The mental model is straightforward but requires trust.

**Presence indicators:**  
Collaborators appear in the UI with color-coded cursors and names.

**Worth borrowing in TUI?**  
**Lower priority for TUI.** Real-time co-editing in a terminal is rare and complex (shared cursor state, diff reconciliation, low-latency sync). However, one pattern worth stealing: **presence indicators in the status bar** (e.g., `[Alice:editing foo.rs]`). This provides minimal-overhead awareness without overwhelming a 24-line terminal.

---

## 4. Project Search Opens in Multibuffer

**Why it's better than VS Code's approach:**  
VS Code opens search results in a results panel, forces you to click each match to navigate to source, and leaves you juggling two panels. Zed opens results directly in a multibuffer, so:

- **Single view:** All matches visible at once with context
- **Instant editing:** Apply edits across all matches in one buffer
- **No mode switching:** Stay in editing mode; results are just another buffer to navigate
- **Keyboard-friendly:** Arrow keys move through results; multiple cursors select and edit matches in bulk

**Workflow example:**  
`Cmd+Shift+F` → type search term → `Enter` → multibuffer appears with all matches. Use `Option+Click` to place cursors on the lines you want to change, then type the replacement. Done.

**Worth borrowing in TUI?**  
**Essential.** Project search opening in a multibuffer is a natural fit for TUI. No mouse needed; just search, navigate, and edit. This directly addresses the TUI advantage of keyboard-centricity.

---

## 5. Outline View: Symbol Navigation Without Leaving the Editor

**How it works:**  
`Cmd+Shift+B` (or `Ctrl+Shift+B`) opens a persistent panel showing a tree of symbols in the current file—functions, classes, variables, etc. Keyboard navigation within the outline jumps directly to symbol definitions.

**Benefits:**
- Quick jump to specific functions or class members without searching
- Maintains context while navigating large files
- Can also display errors and warnings from the language server in the same panel

**Integration with multibuffers:**  
The outline is particularly useful when viewing search results in a multibuffer; it provides an overview of where symbols appear across the result set.

**Worth borrowing in TUI?**  
**Very much.** A TUI outline panel is straightforward to implement and high-value. Use Vim-style `:` commands or a sticky split to show symbols. This is a proven pattern in Vim and Neovim ecosystems.

---

## 6. Command Palette: Discoverable Action System

**Design:**  
`Cmd+Shift+P` (or `Ctrl+Shift+P`) opens a fuzzy-searchable command palette. All editor actions are grouped by namespace (`editor:`, `zed:`, `ai:`, etc.), making the system transparent and discoverable.

**Integration:**  
The command palette integrates with Zed's action system—each action is registered and can be searched, invoked directly, or bound to a key. This means documentation and UI are aligned; if the docs say "run `workspace: new file`", users search for it directly.

**Worth borrowing in TUI?**  
**Core pattern.** Most TUI editors already have a command palette. Zed's namespacing strategy is clean and worth copying: prefix commands with the system they affect (e.g., `:buffer`, `:ai`, `:git`). This reduces cognitive load and makes help text more coherent.

---

## 7. GPU Rendering Philosophy: "Every Frame Should Be Cheap"

**Core insight:**  
Zed treats UI rendering like video game development. Instead of a general-purpose graphics library, the team built GPUI with custom shaders for editor-specific primitives (rectangles, text, icons, shadows). This decouples CPU work from display—scrolling a 10,000-line file barely touches the CPU; the GPU translates the viewport.

**Technical approach:**
- Data preparation on CPU
- Heavy lifting delegated to GPU via specialized shaders
- Consistent 120 FPS refresh, synced to monitor refresh rate
- No garbage collection pauses; no expensive layout recalculations

**The philosophy applies broadly:**  
"Every frame should be cheap" means avoiding unnecessary recomputation, caching aggressively, and thinking about rendering as a pipeline, not ad-hoc updates.

**Worth borrowing in TUI?**  
**Philosophically, yes. Technically, limited.** TUI doesn't have a GPU, but the principle holds: minimize redraws, batch updates, and cache aggressively. Use dirty-region tracking to redraw only what changed. Test with large files (10K+ lines) to ensure responsiveness. The takeaway: **render performance is a first-class design constraint, not an afterthought.**

---

## 8. Settings & Keybindings: JSON-Based Configuration

**Approach:**  
Zed uses two files:
- **`settings.json`:** Editor behavior, themes, fonts, extensions, UI layout
- **`keymap.json`:** Key bindings, stored separately for easier customization

Each binding is a JSON object with a key sequence, action name, and optional context (e.g., only when editing, or when a specific mode is active).

**Project-level overrides:**  
Create `.zed/settings.json` in the project root to override global settings per-project.

**Benefit:**  
Clear separation of concerns; keybindings don't clutter settings. Users can share keymaps across machines easily.

**Worth borrowing in TUI?**  
**Yes.** Most TUI editors use TOML or YAML for config, but JSON is more structured and easier to validate. The `.zed/settings.json` pattern for per-project config is especially valuable—it allows different projects to enforce different code styles or keybindings without manual user intervention.

---

## 9. Worktree Abstraction: Multi-Root Projects

**What it is:**  
A "worktree" in Zed is a directory (or file) opened as a standalone project. Zed can manage projects with multiple Git repositories or non-Git folders; each gets its own worktree. The system provides:

- **Worktree environment variables:** `ZED_WORKTREE_ROOT` and `ZED_MAIN_GIT_WORKTREE` available to tasks
- **Linked worktrees:** When creating a new worktree from a multi-repo project, Zed creates linked branches in Git
- **Trust model:** New worktrees are untrusted by default; users must explicitly trust each one

**Setup automation:**  
The `create_worktree` task hook runs after creating a new worktree, allowing setup scripts (e.g., `npm install`, dependency resolution).

**Worth borrowing in TUI?**  
**Moderately.** The worktree concept is useful for large monorepos or projects with multiple logical roots. For a TUI editor, a simpler model may suffice: support opening multiple folders at once (like VS Code's multi-root workspaces) and provide environment variables for scripts. The trust model is worth implementing if Glyph runs arbitrary code.

---

## 10. The "Zed AI" Workflow End-to-End: Refactoring a Function

**Scenario:** You want to refactor a function to use async/await instead of callbacks.

**Step-by-step:**

1. **Open the function.** Navigate to the function in your editor.

2. **Select the function body.** Use keyboard selection or marks to highlight the function.

3. **Open the inline assistant.** Press `Cmd+K` to open the inline assistant panel.

4. **Describe the change.** Type: *"Convert this callback-based function to use async/await. Add proper error handling."*

5. **Review the diff.** The assistant generates a rewritten function. A side panel shows the before/after diff.

6. **Accept or iterate.** Press `Return` to accept the change, or type a follow-up prompt to refine it.

7. **Undo or modify.** If you want to further tweak the result, press `Cmd+Z` or place cursors and edit manually.

**Alternative: Agent panel workflow**

1. **Open the Agent Panel.** Press `Cmd+Alt+P` or run `agent: new thread`.

2. **Provide context.** Say: *"Look at the function `parseConfig` in `utils/config.ts`. Refactor it to use async/await. Consider error handling and performance."*

3. **Monitor progress.** The agent opens files, reads symbols, and generates a plan in real time. You see each step in the chat.

4. **Review changes.** The agent proposes edits; you can accept, reject, or request modifications.

5. **Run tests.** The agent can execute your test suite to validate changes.

**Key observations:**
- **Immediate feedback:** No separate AI window; changes appear in the editor
- **Incremental refinement:** Follow-up prompts can narrow or broaden the scope
- **Context is automatic:** The agent can read your entire project without explicit inclusion
- **Multiple cursors leverage:** If refactoring affects multiple sites, use inline assistant on each with parallel edits

**Worth borrowing in TUI?**  
**Absolutely.** The same workflow applies to TUI:
1. Select code (Vim visual mode or marks)
2. Invoke inline assistant (`<leader>ai` or similar)
3. Type prompt inline (no separate panel)
4. Accept diff with one key
5. Status bar shows agent activity

The inline model is actually **better for TUI** because it avoids panel overhead and keeps focus on the code being edited.

---

## Summary: Zed Patterns Worth Stealing for Glyph

### High-Priority (Core to TUI Experience)
1. **Multibuffer for project search** — Eliminates context-switching between results and source
2. **Inline assistant (Cmd+K)** — Direct editing, no panels, keyboard-native
3. **Command palette with namespacing** — Discoverable, searchable action system
4. **Outline view** — Fast symbol navigation; already familiar to Vim users
5. **Project-level settings override** — Enables per-project customization without user friction

### Medium-Priority (Valuable but Not Essential)
6. **Worktree abstraction** — Useful for monorepos; consider simplified version
7. **Status bar presence indicators** — Low-overhead awareness in collaborative contexts
8. **JSON keybindings with context** — More structured than TOML/YAML; easier to extend

### Low-Priority for TUI (Context-Specific)
9. **GPU rendering philosophy** — Principle applies (minimize redraws), but not literally GPU
10. **Real-time channels** — Overkill for TUI; presence indicators are the usable part

---

## Conclusion

Zed's design philosophy—treating the editor as a game engine, prioritizing AI as a first-class citizen, and enabling wide-ranging edits through multibuffers—translates surprisingly well to TUI constraints. The most impactful borrowings are **multibuffer project search, inline AI assist, and keyboard-driven symbol navigation**. These patterns reduce context-switching, keep focus on code, and embrace the TUI's keyboard-first paradigm.

The least relevant borrowing is real-time collaboration, which introduces significant complexity and is rarely expected in TUI editors. The philosophy of rendering efficiency, however, is universally applicable: make every terminal redraw count, cache aggressively, and test with large files to ensure responsiveness.

For Glyph, the roadmap should prioritize multibuffer + inline assistant, then outline view and command palette refinement, before exploring advanced patterns like worktrees or presence indicators.

---

## Sources

- [Zed Multibuffers Documentation](https://zed.dev/docs/multibuffers)
- [Zed AI Overview](https://zed.dev/docs/ai/overview)
- [Zed Agent Panel Documentation](https://zed.dev/docs/ai/agent-panel)
- [Zed Collaboration Overview](https://zed.dev/docs/collaboration/overview)
- [Zed Project Search & Finding Documentation](https://zed.dev/docs/finding-navigating)
- [Zed Outline Panel](https://zed.dev/docs/outline-panel)
- [Zed Command Palette Documentation](https://zed.dev/docs/command-palette)
- [Zed GPU Rendering Philosophy Blog Post](https://zed.dev/blog/videogame)
- [Zed Keybindings Documentation](https://zed.dev/docs/key-bindings)
- [Zed Worktree Trust](https://zed.dev/docs/worktree-trust)
