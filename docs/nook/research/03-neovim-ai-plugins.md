# Neovim AI Plugin Landscape (2026)

In 2026, Neovim users have access to a mature ecosystem of AI coding tools, with two plugins — **Avante.nvim** and **CodeCompanion.nvim** — dominating the space. This report surveys the leading plugins, their UX patterns, workflows, and the emerging sidecar approach with tools like Aider.

## 1. Avante.nvim: "Cursor in Neovim"

**Architecture & UX Philosophy.** Avante.nvim positions itself as a direct Cursor IDE port. The core interaction model is: select code → invoke keybind → describe the change → review diff in sidebar → accept or reject. This explicit, review-first pattern mirrors Cursor's philosophy: AI suggestions are never applied without human verification.

**Sidebar Layout & Diff Preview.**
- Sidebar opens on the right (configurable) with an input window for prompts
- Diff shown side-by-side with conflict highlighting
- Width adjustable via percentage configuration
- Navigation: `]x`/`[x` to move between conflicts; `co`/`ct` to choose ours/theirs

**Keystroke Flow for Inline Edit.**
1. Visual select code: `v` + motion or `V` for line
2. Invoke: `:AvanteEdit` (or mapped keybind, e.g., `<leader>ae`)
3. Type prompt (e.g., "refactor for readability")
4. LLM processes; sidebar opens with diff
5. Review changes in split view
6. Accept all: `A` in sidebar; Accept at cursor: `a`
7. Tab to switch windows between original and diff

**Multi-Model Support.** Supports Claude, OpenAI, Gemini, Copilot, and Ollama via configuration. Project-level `.avante.md` file allows behavioral instructions scoped per codebase (similar to system prompts).

**Distinction from Competitors.** The diff-first workflow is Avante's signature. Bigger refactors benefit from seeing visual changes rather than prose suggestions.

## 2. CodeCompanion.nvim: "Buffer-Integrated AI"

**Architecture & UX Philosophy.** CodeCompanion rejects the sidebar paradigm. Instead, it integrates AI into Neovim's native paradigm: chat buffers in splits, inline transformations in your current buffer, action palette for quick ops. It respects Vim's philosophy of unobtrusive utilities.

**Three Core Interaction Modes.**
1. **Chat Buffer.** Dedicated split where you converse with an LLM. Supports `@file`, `@lsp`, `#{buffer}` context variables to pull diagnostics and file content without manual copying.
2. **Inline Interaction.** Run `:CodeCompanion <prompt>` with a visual selection. The LLM writes directly into your buffer. Auto-triggers diff mode by default.
3. **Action Palette.** Quick-access prompt library for common tasks (explain LSP error, suggest tests, refactor).

**Keystroke Flow for Inline Edit.**
1. Visual select: `v` + motion
2. Type: `:CodeCompanion refactor this for clarity`
3. Accept: `gda` (CodeCompanion diff accept)
4. Reject: `gdr` (CodeCompanion diff reject)
5. Streaming output appears in buffer; diff updates in real-time

**Distinction from Avante.** No dedicated sidebar; changes appear in your working buffer with inline diff. Faster for incremental edits; less suitable for massive refactors where you want to review before touching the original.

## 3. Copilot.lua: Ghost-Text Autocomplete

**Rendering & Acceptance.** Ghost text (gray inline suggestions) appears as you type — visually distinct from LSP completions. Community prefers this over the cmp menu integration to avoid cognitive overload.

**Keystroke Flow (Default Keymaps).**
- Accept suggestion: `<M-l>` (Alt+L)
- Dismiss: `<C-]>` (Ctrl+])
- Next: `<M-]>` (Alt+])
- Previous: `<M-[>` (Alt+[)

Tab and Esc can be remapped via `suggestion.keymap` config.

**Latency Considerations.** Cloud backends (GitHub Copilot) show minimal latency. Local Ollama on CPU-only hardware is noticeably slower. On slow hardware, disable `auto_trigger = true` and use manual keybind instead to avoid sluggish editor feel.

**Rendering Implementation.** Uses highlight groups `CopilotSuggestion` and `CopilotAnnotation` (default: Comment color). Automatically hides when popupmenu-completion is open to avoid visual clutter.

## 4. Codeium.vim & Supermaven-nvim: Local-Friendly Alternatives

**Positioning.** Both offer cloud-based ghost-text completions similar to Copilot but with different LLM backends. Supermaven uses a 1-million-token context window; Codeium performed poorly in community testing (0.5% acceptance rate in one comparison).

**UX Pattern.** Identical to Copilot — gray inline suggestions with Tab to accept. No sidebar, no chat, purely autocomplete-focused. Supermaven edges out Codeium in volume of useful suggestions.

## 5. Aider as Terminal Sidecar: Why It Matters

**Workflow Model.** Instead of embedding AI in Neovim, developers run Aider in a tmux pane beside Nvim. User edits happen in Nvim; AI work happens in Aider's CLI. Git diffs are the integration point.

**Advantages Over In-Editor Plugins.**
1. **Context Persistence.** IDE extensions lose context on crash or task switch. Aider's terminal session survives; state persists.
2. **Git-Native.** Every AI change becomes a reviewable commit with descriptive messages. No diff-preview UI needed; `git diff` is the review tool.
3. **Transparency.** Minimal UI means no confirmation dialogs or sidebar decisions — just prompts and git log.
4. **Multi-Agent.** Run Aider, Claude Code, and other agents in parallel on different branches. Each has its own shell history.
5. **Cost Control & Local Support.** Aider supports 100+ languages and any LLM (including local Ollama). Open source and free.

**When to Choose Aider:** Long-running autonomous tasks, git-workflow preference, cost control, or support for exotic languages/local models.

## 6. Common UI Patterns Across Plugins

**Sidebar (Chat).**
- Avante: Right (default), left configurable
- CodeCompanion: No sidebar; uses buffer splits instead
- Copilot/Codeium/Supermaven: No sidebar

**Inline Edit (Cursor Over Selection).**
- Avante: Select → `:AvanteEdit` → diff in sidebar → accept/reject
- CodeCompanion: Select → `:CodeCompanion <prompt>` → inline diff in buffer → `gda`/`gdr`
- Aider: Manual refactoring; git diff as review

**Ghost Text.** Copilot.lua, Codeium, Supermaven: Gray inline suggestions; Tab to accept, Esc to dismiss.

**Diff Preview.**
- Avante: Full-buffer side-by-side diff in sidebar
- CodeCompanion: Inline diff overlay in current buffer (hunk-by-hunk by default)
- Aider: `git diff` output in terminal

**Context Variables & Mentions.**
- CodeCompanion: `@file`, `@lsp`, `#{buffer}` to reference diagnostics and files
- Avante: Project-level `.avante.md` for system prompts
- Aider: File arguments in CLI command

**Streaming Output.**
- Avante: Streaming response in sidebar while you wait
- CodeCompanion: Streaming directly into buffer (non-blocking; you can keep editing)
- Copilot: Streaming suggestions (silent; appears as ghost text)

## 7. The Minimum Set of AI Primitives (80% of Value)

A glyph-based editor needs four core capabilities to deliver most of the value:

1. **Inline Edit with Diff Preview.** Select text → invoke prompt → see diff → accept/reject. (Covers refactoring, bug fixes, style changes.)
2. **Ghost-Text Autocomplete.** Passive suggestions as you type, accept with one key. (Covers boilerplate, function names, obvious continuations.)
3. **Chat Sidebar or Buffer.** Ask questions about code without selecting it. `@mention` syntax for context. (Covers learning, debugging, design feedback.)
4. **Streaming Output.** LLM response arrives incrementally so user sees progress and doesn't feel blocked. (UX: feedback loop; no "thinking..." spinner.)

These four give you ~80% of what Cursor or Windsurf offer. Slash commands, agent mode, and multi-file orchestration are nice-to-haves but not essential for individual editing tasks.

## 8. Concrete Keystroke Flow: "Rename This Variable Everywhere in the File with a Prompt"

**Scenario:** A JavaScript function has a variable `tmpData` used in 7 places. You want to rename it to `preprocessedInput` based on how it's actually used.

### Using Avante.nvim:

```
1. % (select all in buffer)
2. <leader>ae (AvanteEdit keybind)
3. Type: "Rename variable tmpData to preprocessedInput throughout.
   It holds user input that's been validated."
4. [Wait for sidebar to open with diff]
5. Review the diff; changes are highlighted
6. Press A (accept all changes)
7. Buffer updated; press <leader>at to close sidebar
```
Time: ~5 seconds (after LLM latency).

### Using CodeCompanion.nvim:

```
1. % (select all in buffer)
2. :CodeCompanion /refactor - rename tmpData to preprocessedInput
3. [Output streams into inline diff]
4. Review changes (hunk-by-hunk if needed)
5. gda (accept all changes)
6. Buffer updated; continue editing
```
Time: ~5 seconds (same, but no sidebar switch).

### Using Aider CLI (in tmux pane):

```
1. (In Aider pane) /add filename.js
2. (Type) Rename variable tmpData to preprocessedInput throughout.
   Context: this variable holds validated user input.
3. [Aider applies changes; shows git diff]
4. Review: git diff
5. (Type) y (commit the changes)
6. (Switch to Nvim) git pull (or already synced)
7. Nvim buffer auto-refreshes; continue editing
```
Time: ~8 seconds (slower, but changes are versioned commits).

## Summary

**Avante.nvim** is for developers who want a Cursor lookalike: sidebar-centric, diff-first, ideal for larger refactors. **CodeCompanion.nvim** is for Vim purists who want AI that stays out of the way: buffer-integrated, no sidebar, chat in splits. **Copilot.lua** dominates for passive autocomplete. **Aider + sidecar** dominates for autonomous long-running tasks and git-workflow zealots.

The ecosystem has matured enough that the "best" choice depends entirely on your workflow philosophy — not on capability gaps.

## Sources

- github.com/yetone/avante.nvim
- github.com/olimorris/codecompanion.nvim
- github.com/zbirenbaum/copilot.lua
- aider.chat
- composio.dev/content/how-to-transform-your-neovim-to-cursor-in-minutes
- samuellawrentz.com/blog/neovim-ai-plugins-avante-codecompanion
- pristren.com/blog/neovim-for-modern-developers
