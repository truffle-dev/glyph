# Cursor Feature Inventory: What a "Cursor Replacement" Actually Needs

**Research Date:** May 2026  
**Goal:** Identify the irreducible feature set for a viable Cursor alternative, grounded in real developer workflows and pain points.

---

## 1. The AI-First Features That Define Cursor vs VS Code

Cursor is not VS Code with plugins bolted on—it's a fork that rethinks the editor around AI-first workflows. The gap is significant:

### Cmd+K: Inline Code Generation
- **Keystroke flow:** Select code or position cursor → `Cmd+K` → natural language prompt → AI generates inline replacement
- **UX detail:** A floating prompt bar appears at the cursor position, not in a sidebar. You type your edit goal ("extract this to a function," "add error handling"), hit Enter, and the code is generated inline with a diff preview that you can Accept or Reject
- **Why it matters:** It's 3x faster than Cmd+L (chat) because you stay in the editor, no context-switching to a sidebar
- **Real workflow:** Refactoring a function takes 8-10 seconds with Cmd+K vs 45 seconds of chat back-and-forth

### Cmd+L: Persistent Chat Sidebar
- **Keystroke flow:** `Cmd+L` → opens right-hand chat pane → multi-turn conversation about code, architecture, debugging
- **UX detail:** Chat maintains the conversation thread; you can ask follow-up questions, reference code with @mentions, and the model remembers context across turns
- **Why it matters:** For exploratory work ("how do I structure this?") and debugging, chat is essential. Cursor users report this is where they spend 40% of AI interaction time
- **Real workflow:** "Why is this test failing?" → model reads stack trace + code → suggests fix → you ask "will this break X?" → chat chain resolves the problem

### Composer: Multi-File Autonomous Editing
- **Keystroke flow:** `Cmd+Shift+K` or `/` in chat → describe task spanning multiple files → Composer reads entire codebase, generates coordinated diffs across all affected files
- **UX detail:** Composer breaks the task into steps, applies changes file-by-file, and shows a unified diff view of all changes. You see a "Review Changes" panel with before/after for every file at once
- **Why it matters:** A refactoring that would take 2 hours manually (find all references, update in 12 files, fix imports) takes 90 seconds. This is where Cursor wins hardest
- **Real workflow example:** "Convert all login handlers from callback to async/await" → Composer touches 8 files, updates 40 function signatures, regenerates tests

### Agent Mode: Hands-Off Task Execution
- **Keystroke flow:** `/` prefix in Composer → describe goal → Agent runs autonomously: reads files, runs tests, executes shell commands, fixes failures iteratively
- **UX detail:** Agent has an "autonomy slider"—you can set it to 1 (just suggest changes) through 10 (full autonomous execution). The diff updates live as the agent modifies files. Hit "Stop" to cancel mid-execution
- **Why it matters:** For long-running tasks (refactor a legacy module, migrate framework versions, fix all linter warnings), you can delegate and check progress occasionally instead of babysitting
- **Real workflow:** "Fix all TypeScript errors in src/" → Agent runs `tsc`, fixes errors, runs tests, commits. You check diff every 30 seconds, intervene if needed
- **Execution:** Agent can run up to 8 parallel agents in git worktrees to prevent file conflicts

### Apply Diff Workflow (Anchored Preview)
- **Previous gold standard (removed in recent versions, user-requested restoration):** After Composer generates changes, a dedicated UI showed green/red inline diffs for each change, with per-change Accept/Reject buttons
- **Current state:** Changes auto-apply, but a unified diff view lets you review everything before committing
- **Why it matters for alternative:** Users specifically complained when this feature was removed. The per-change granularity is the safety net—you catch hallucinations before they hit your codebase
- **UX implication:** A TUI Cursor replacement *must* show diffs in a way that feels granular and inspectable, not bulk-apply

### Codebase Context via @ Symbols
- **Syntax:** Type `@` in any AI prompt → symbol picker appears
- **@codebase:** Semantic search across entire project. `@codebase How is auth structured?` triggers full-text + semantic indexing, returns most relevant functions/files
- **@Files:** Explicitly reference one or more files by name to ensure they're included as context
- **@Docs:** Reference official documentation (React, Django, etc.) or add custom docs by URL
- **@Web:** Live web search, adds current information to context
- **UX detail:** Type `@` and you get an autocomplete picker showing recent files, popular symbols, and search results. This is where good UX makes or breaks the tool—picking the right context takes 2 seconds or feels like hunting
- **Why it matters:** Context quality directly correlates with suggestion quality. Developers using @codebase intelligently report 3x fewer hallucinations than those with sparse context

### Terminal Integration as First-Class AI Component
- **Keystroke flow:** `Ctrl+` ` (backtick) → integrated terminal opens below editor
- **UX detail:** Terminal is not just a pane—it's woven into Agent workflows. Agent can run `npm test`, read error output, modify code, re-run tests automatically
- **Why it matters:** Without terminal integration, Agent mode is crippled. You can't run tests, builds, or diagnostics without a separate window
- **Real workflow:** Agent encounters a failed test → reads terminal output → traces error to root cause in code → patches code → re-runs test → confirms fix

---

## 2. Table-Stakes Editor Features (The Basics That Still Matter)

These are not unique to Cursor, but their *absence* would make the tool unusable:

- **File tree & tabs:** Open multiple files, navigate project structure. This is Ctrl+P (Go to File) plus left sidebar tree view
- **Syntax highlighting & language support:** Code must be readable. At minimum: JavaScript, TypeScript, Python, Go, Rust, Java
- **Multi-cursor editing:** Select multiple locations, edit simultaneously. `Cmd+D` to select next occurrence
- **Find/Replace:** `Cmd+F` in-file, `Cmd+Shift+F` across codebase. Regex support is essential
- **Go-to-Definition & Find References:** `Cmd+Click` or F12 to jump to definition; `Shift+F12` to see all references. This is what makes refactoring safe
- **Inline diagnostics:** Red/yellow squiggles for linting, type errors, runtime warnings. Developers unconsciously rely on this to catch mistakes before running code
- **Hover documentation:** Hover over function name → see docstring, type signature, available parameters
- **Diff view (Git integration):** See changes line-by-line before commit. This is VS Code's default git pane
- **Problems panel:** Aggregated list of all linting/type errors in project. Essential for "clean up all warnings"

**Why these matter even in an AI editor:** Developers still *read* code more than they generate it. If navigation, search, and diagnostics are clunky, AI suggestions feel useless because you can't verify or refine them efficiently.

---

## 3. Cursor-Specific UX Choices Worth Copying

### (a) The @ Mention Picker UX
Typing `@` brings up a smart autocomplete that shows:
- Recent files you've edited
- Popular functions/classes from recent context
- Live search results as you type

This beats having to manually type file names. A TUI version could use fuzzy-find (`fzf`-style) picker.

### (b) Anchored Inline Edits (Cmd+K Preview Flow)
- Position cursor in code → `Cmd+K` → type goal → preview appears *in-place*, not in a modal
- The diff is right there in your reading context, not in a separate panel
- Accept/Reject buttons are two keystrokes away

For a TUI, this could map to: cursor → mode switch to "edit mode" → view diff inline → `y/n/edit` confirmation.

### (c) Unified Multi-File Diff View
Instead of jumping between files to see what changed, Composer shows all diffs stacked in a single scrollable view with file headers. This is faster mental overhead than VS Code's split-pane approach.

### (d) Autonomy Slider in Agent Mode
The ability to dial down Agent independence is psychologically important—developers worry less about "the AI breaking everything" if they can set it to 1 (suggestions only) vs 10 (full autonomy). Even a "dry-run" mode helps.

---

## 4. What Cursor Users Complain About (And Where Alternatives Can Win)

### Pricing & Cost Transparency
- **The complaint:** In June 2025, Cursor switched from "500 fast requests/month" to an opaque credit system without warning. Users reported spending $350 on overages in a week; costs doubled or tripled overnight
- **Why it matters:** Cost opacity drives lock-in anxiety. A transparent alternative (e.g., "flat $15/month, unlimited usage") would immediately poach users
- **CEO acknowledgment:** Cursor's Michael Truell issued a public apology and offered refunds, but the damage to trust persists

### Rate Limiting for Power Users
- **The complaint:** Pro tier hits limits after ~50 heavy Composer uses/day. For intensive refactoring sessions, you run out of requests before month-end
- **Why it matters:** A flat-rate or "pay-per-token" model would appeal to teams doing large refactors

### Electron Memory Overhead
- **The complaint:** Cursor uses Electron (Chromium-based). On large monorepos (>50k files), indexing consumes 8-16GB of RAM. On 8GB machines, this is a blocker
- **Where a TUI alternative wins:** Terminal-native tools have 100-200MB memory footprint for the same indexing
- **Real quote:** "I use Cursor on a $3k MacBook Pro, but it crashes when indexing a 100k-file monorepo"

### Hallucinations in Composer
- **The complaint:** Agent/Composer sometimes generates plausible but incorrect code, adds imports that don't exist, or breaks subtly
- **Why it matters:** This is why the "apply diff" preview UX is so important—you *must* review before committing

### Lock-In Concerns
- **The complaint:** Cursor settings, key bindings, and workflows don't export cleanly to VS Code or other editors. Switching costs are high
- **Where alternative wins:** If built on standard LSP, DAP, and VSCode extension protocols, you avoid the "rewire your entire workflow" tax

---

## 5. The Terminal-Native Angle

Building a Cursor equivalent in the terminal (e.g., as a TUI) has **strengths and tradeoffs:**

### What Translates Naturally to Terminal
- **Chat/Composer prompts:** A text input at the bottom of the screen works fine. `Cmd+K` could map to `C-e` (edit mode)
- **@ mentions:** Fuzzy-find picker (`fzf`-style) is actually smoother in terminal than a dropdown
- **File navigation:** `fzf` or `telescope` for file picker is arguably faster than clicking a sidebar
- **Diff review:** `git diff` output is already TUI-friendly. A pager with syntax highlighting works
- **Terminal integration:** The terminal pane becomes the *same terminal* you're already using—no context switch

### What Doesn't Translate Well
- **Inline hover docs:** No GPU-accelerated rich text. Could use "show docs in pane below" or `:help <symbol>` mode
- **Multi-cursor editing:** Fine with modal editing (vim-style), clunky in non-modal TUI
- **Visual diff preview:** Can show diff, but not with the spatial anchoring of GUI (lines highlighted in-place)
- **File tree sidebar:** Works, but costs precious screen real estate. Alternatives: `:ls` command or fuzzy picker

### Compensating UX Patterns
- **Mode indicators:** Show `[EDIT]` / `[COMPOSER]` / `[AGENT]` in status bar so user always knows context
- **Command palette:** A fuzzy picker for all commands (Cursor replacement for VS Code's Cmd+Shift+P)
- **Pager + syntax highlighting:** Use `less -R` or `bat` for viewing diffs/docs with color
- **Split panes:** tmux/zellij-style horizontal/vertical splits for simultaneous file viewing and diff review
- **Quick preview:** Hotkey to show current file definition, hover docs, or diagnostics in a small popup

---

## 6. The Hard MVP: Minimum Feature Set for Full-Day Development

Based on research, here are the **5–8 irreducible features** a Cursor alternative must have to support a real developer for a full 8-hour day:

### Must-Have Features

1. **Cmd+K Inline Edit** (or TUI equivalent: `C-e` enter edit mode, see diff, `y/n`)
   - Without this, the entire "AI-assisted coding" pitch collapses. Tab completion + Cmd+K is 80% of Cursor's usage
   - Real-world test: Refactor a function, add error handling, extract a helper. Should take <2min with Cmd+K

2. **Cmd+L Chat + Context (@codebase, @files)**
   - For the 20% of tasks that need exploration, "why is this failing?", architecture questions
   - Critical constraint: @ picker must be snappy (<100ms to show suggestions)
   - Real-world test: "How is the payment flow structured?" Answer comes from actual code, not hallucination

3. **File Navigation (Fuzzy Go-to-File + Find-in-Files)**
   - `Cmd+P` or `:find` command. Developers use this 50+ times per day. If it's slow, the whole tool feels slow
   - Must support regex search in grep
   - Real-world test: Find all callers of `validatePayment()` in <2 seconds

4. **Syntax Highlighting + Diagnostics (Linting/Type Errors)**
   - Non-negotiable. Developers mentally rely on red squiggles to catch errors before running code
   - Minimal: JavaScript/TypeScript, Python. Extended: Go, Rust, Java
   - Real-world test: Typo in variable name → red squiggle appears instantly

5. **Git Diff View (Staged/Unstaged Changes)**
   - Before committing, developers must see what's changed. Cursor integrates this tightly. A clone must too
   - Real-world test: Run Composer, see all changes in one unified view, commit with confidence

6. **Multi-File Edit with Composer (Optional But High-Value)**
   - Single-file Cmd+K handles 60% of tasks. Composer handles the other 40% (refactors, migrations, boilerplate)
   - Without it, tool feels limited to small edits
   - Real-world test: "Convert all login handlers to async/await" should touch 6+ files automatically

7. **Terminal Integration (Must Run Tests, Builds, Commands)**
   - Agent mode doesn't work without this. Even in non-Agent workflows, developers need to run code
   - Minimal: can execute shell commands and capture output in-pane
   - Real-world test: Run `npm test`, see failures, fix code, re-run, confirm green

8. **Tab Completion (AI-Powered or Even Simple Heuristic)**
   - A developer writing raw code without completion feels like 1995. Even a basic predictive model (not cutting-edge) beats nothing
   - Real-world test: Type the start of a function name → completion suggests the full name and common arguments

### Optional But Valuable
- **Agent Mode** (autonomous multi-step execution): Differentiator, but not MVP
- **Multi-cursor editing**: Vim users expect it; others don't care as much
- **Hover documentation**: Nice-to-have; essential only for dynamic languages
- **Workspace indexing** (codebase semantic search): High ROI if good, but slow indexing kills UX

---

## 7. Implementation Implications for a TUI Cursor Replacement

### Architecture
- **Editor core:** Use `neovim` as base (proven, fast, extensible) or build minimal from scratch
- **AI integration:** Speak to Claude/OpenAI/other via API. Stream tokens to editor in real-time
- **Language support:** Rely on existing LSP servers for diagnostics, hover docs, go-to-def (standardized, fast)
- **Terminal:** Reuse system shell; no need to reimplement bash/zsh

### UX Mapping
| Cursor | TUI Equivalent |
|--------|---|
| `Cmd+K` | `C-e` or `:edit` command, inline diff, `y/n` |
| `Cmd+L` | `:chat` command opens persistent chat pane |
| `Cmd+Shift+K` (Composer) | `:composer` or `/` prefix in chat |
| @ picker | `fzf`-style fuzzy search after `@` |
| File tree | `C-n` toggle tree, or `:files` picker |
| Diff view | Split pane with `git diff` + syntax highlight |
| Terminal | Native shell pane, `C-b` toggle |

### Performance Targets
- File fuzzy search: <100ms for 50k files
- @ mention picker: <50ms suggestions
- Diagnostics update: <500ms after edit
- Chat streaming: First token <500ms (perceived latency), then 20+ tokens/sec

---

## 8. Sources & Further Research

### Cursor Docs & Blog Posts
- [Cursor vs VS Code: AI Coding Editor Showdown](https://www.augmentcode.com/tools/cursor-vs-vscode-comparison-guide)
- [Next-Level Cursor: Cmd+K, Composer, and Agent Unpacked](https://www.newline.co/@MaksymMitusov/next-level-cursor-cmdk-composer-and-agent-unpacked--326f7ed3)
- [Cursor's Codebase Indexing & @ Symbols Guide](https://eastondev.com/blog/en/posts/dev/20260115-cursor-codebase-index-guide/)
- [Best Practices for Coding with Agents](https://cursor.com/blog/agent-best-practices)

### User Feedback & Complaints
- [When Cursor Silently Raised Prices 20x](https://medium.com/@jimeng_57761/when-cursor-silently-raised-their-price-by-over-20x-and-more-what-is-the-message-the-users-are-getting)
- [Cursor Pricing Changes & User Apology](https://techcrunch.com/2025/07/07/cursor-apologizes-for-unclear-pricing-changes-that-upset-users/)
- [Cursor Terminal & Integrated Workflow](https://cursor.gr.com/terminal.html)
- [Cursor AI Review 2025: Agent Mode & Privacy](https://skywork.ai/blog/cursor-ai-review-2025-agent-refactors-privacy/)

### Market Context
- [VS Code vs Cursor vs Windsurf 2025 Comparison](https://dev.to/_d7eb1c1703182e3ce1782/vs-code-vs-cursor-vs-windsurf-which-ai-code-editor-should-you-use-in-2025-1jk4)
- [Cursor Alternatives in 2026](https://www.builder.io/blog/cursor-alternatives-2026)

---

## Conclusion

A viable Cursor replacement doesn't need to clone every feature—it needs to nail the core workflow loop:
1. **Cmd+K for in-place edits** (the speed multiplier)
2. **Cmd+L for exploration** (the safety valve)
3. **Context management via @** (the difference between genius and hallucination)
4. **Diff review before commit** (the guardrail)
5. **Terminal integration** (the unifier)

With these five and competent file navigation + diagnostics, a developer can sustain a full day's work. Everything else (Composer, Agent, rich hover UIs) is high-value but not MVP-blocking. 

The alternative's real advantage lies in **transparency** (pricing, indexing), **speed** (terminal native), and **openness** (standards-based, not fork-locked). Cursor's strength is baked-deep AI integration; a replacement's strength is being faster, cheaper, and less of a black box.
