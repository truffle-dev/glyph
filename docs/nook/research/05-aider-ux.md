# Aider UX Deep-Dive: The Canonical Terminal AI Coding Tool

## 1. CLI Invocation: Starting an Aider Session

Aider starts with bare simplicity. A typical session begins:

```bash
aider file1.py file2.py --model claude-3-5-sonnet-20241022
```

The key parameters:
- **Positional arguments**: File paths to edit. Only add files that need direct modifications; aider intelligently incorporates broader codebase context via its repo-map without requiring every file present.
- **`--model`**: Specify the LLM (e.g., `claude-3-5-sonnet`, `gpt-4o`, `deepseek-chat`). Defaults to the user's configured model.
- **`--api-key`**: Format `provider=key` (e.g., `anthropic=sk-...` or `openai=sk-...`).

Upon launch, aider:
1. Detects the git repository (or warns if none exists).
2. Builds a repo-map of the codebase using tree-sitter parsers.
3. Displays the welcome prompt, showing loaded files, model, and context token budget.
4. Waits for the user's first message.

Example output:
```
Added file1.py and file2.py to the chat.
Using Claude 3.5 Sonnet with ~100k context tokens available.
Type your request:
```

## 2. The Chat Loop: One Full Edit-and-Apply Cycle

### User Message
```
$ You type: "Rename the calculate_total function to compute_sum"
```

### Model Response
Aider streams the response token-by-token. For code changes, it outputs:
- Brief explanation of the changes
- Structured edits in the selected format (default: unified diff)
- Confirmation prompt

### The Diff Display (Unified Diff Format)
```
file1.py
<<<<<<< ORIGINAL
def calculate_total(items):
    return sum(items)
=======
def compute_sum(items):
    return sum(items)
>>>>>>> UPDATED
```

### User Acceptance Workflow
```
$ Apply this change? [y/n/a/q] >
```

Options:
- **`y`**: Accept and apply this diff
- **`n`**: Reject, keep the file unchanged
- **`a`** (All): Accept all pending diffs from this response
- **`q`** (Quit): Stop processing diffs
- **`s`** (Skip): Skip this diff, but continue to the next

If `y` is pressed, aider:
1. Patches the file in memory.
2. Writes the change to disk.
3. Runs any enabled linters/tests.
4. If clean, commits with a message: `"aider: rename calculate_total to compute_sum"` (message attribution on by default).

### Return to Chat
```
$ Ready for the next request.
```

The user can now ask for follow-up changes, review diffs with `/diff`, undo with `/undo`, or add/drop files with `/add` and `/drop`.

## 3. Diff/Apply UX: Formats and Variations

### Edit Format Options

Aider supports multiple diff formats, auto-selected by model but manually controllable via `--edit-format`:

**Unified Diff (`udiff`)**: The default for most models.
```
<<<<<<< ORIGINAL
def old_func():
    pass
=======
def new_func():
    return True
>>>>>>> UPDATED
```

Advantages: Compact, LLM-native, reduces "lazy coding" (GPT-4 Turbo tendency to elide code).

**Search/Replace (`diff`)**: Alternative format using search/replace blocks.
```
Search in file1.py:
    def old_func():
        pass

Replace with:
    def new_func():
        return True
```

Advantages: Clearer for small, isolated changes; less context needed.

**Whole File (`whole`)**: LLM returns the entire updated file.

Disadvantages: Slow, expensive, high token cost; rarely used now.

**Diff-Fenced (`diff-fenced`)**: Format variant for models like Gemini that struggle with standard fencing.

### Accept/Reject Mechanics

**Interactive yes/no**:
- After each diff, aider waits for user confirmation.
- User can interrupt with `Ctrl+C`; partial responses remain in the chat.

**Batch acceptance**:
- `/all` or `a` accepts all remaining diffs from the current response without prompting each.
- `/skip` or `s` skips one diff but continues to the next.

**Command-line flags**:
- `--yes`: Always answer yes to all confirmations (non-interactive).
- `--auto-accept-architect`: For architect mode, auto-accept editor proposals without user confirmation.

**Partial application**:
- Not per-line (user cannot cherry-pick within a diff), but the `/diff` command lets users review what was changed and decide to undo specific commits.

## 4. Codebase Awareness: Repo-Map and Context Management

### How Aider Understands Your Codebase

Aider builds a **repo-map** on startup using tree-sitter (supporting 130+ languages). The repo-map extracts:
- Function and class definitions
- Import statements
- Symbol references and dependencies

Using a **graph-based PageRank algorithm**, aider ranks which symbols are most important—i.e., most frequently referenced. This creates a token-budgeted summary (default: 1,000 tokens, configurable via `--map-tokens`).

**Why this matters**: For large codebases, the full repo-map might exceed the context window. Aider sends only the most critical definitions, making it smarter about dependencies without bloating the prompt.

### Adding and Dropping Files

- **`/add path/to/file.py`**: Include a file in the chat for editing or reference.
- **`/drop path/to/file.py`**: Remove a file from the chat to preserve context tokens.
- **`/read-only path/to/file.py`**: Add a file as read-only reference (e.g., a config or schema file the LLM should understand but not modify).
- **`/ls`**: List all files in the repo and their inclusion status.

### Context Window Strategy

Aider's philosophy: "Just add the files you think need to be edited." Don't overwhelm the LLM with the entire codebase. The repo-map provides enough structural awareness that the LLM can infer dependencies and make informed changes across multiple files without each being explicitly loaded.

## 5. Git Integration: The Standout Feature

### Auto-Commits with Attribution

After each accepted change, aider commits with a sensible message:
```
commit a3f7k2...
Author: Claude <aider@example.com>
Date:   Wed May 21 10:15:00 2026

    aider: rename calculate_total to compute_sum
```

Attribution defaults to `Claude` (or the active model name). Users can customize author via `--author "Your Name <email@example.com>"` or config.

### Dirty-Tree Handling

**Before aider edits**, if the repository has uncommitted changes:
1. Aider commits them with a message like: `"pre-aider commit: save work before AI edits"`.
2. Then aider makes its changes in a separate commit.

This ensures:
- Your work is never lost.
- AI changes are isolated and easily reverted.
- Clear git history showing what was pre-existing vs. AI-generated.

Controlled by `--dirty-commits` (default: true) and `--auto-commits` (default: true).

### Undo Command

`/undo` reverses the last aider-made commit:
```
$ /undo
Undid aider's last commit: "aider: rename calculate_total to compute_sum"
```

Includes safety checks (dirty-tree detection, confirmation prompts) to prevent accidental data loss.

### Commit Inspection

Users can review changes with:
- `/diff`: Show recent diffs
- `/commit`: Manually commit pending changes with a custom message
- Git commands directly: `git log`, `git show`, `git diff HEAD~3`

## 6. Slash Commands: The Command Palette

Essential commands a TUI editor should mirror:

**File Management**:
- `/add <file>`: Add file to chat
- `/drop <file>`: Remove file from chat
- `/read-only <file>`: Add as reference only
- `/ls`: List files and status

**Chat Control**:
- `/clear`: Clear conversation history
- `/reset`: Drop all files and clear history
- `/help`: Show command help

**Code Modes**:
- `/code`: Switch to (or stay in) code mode
- `/ask`: Switch to discussion-only mode (no edits)
- `/architect`: Use two-model workflow (architect proposes, editor implements)
- `/chat-mode <mode>`: Permanently switch modes

**Model & Reasoning**:
- `/model <name>`: Switch the main LLM
- `/think-tokens <n>`: Set reasoning token budget (for o1-class models)
- `/reasoning-effort <level>`: Adjust thinking intensity

**Execution & Review**:
- `/run <command>`: Execute shell command
- `/test`: Run tests and auto-fix failures
- `/lint`: Run linters and auto-fix violations
- `/diff`: Show recent changes
- `/commit`: Commit changes with a message
- `/undo`: Undo last aider commit
- `/tokens`: Show token usage
- `/copy`: Copy last response to clipboard
- `/exit` or `/quit`: Exit aider

## 7. What Aider Gets Right: Why It's Sticky

### 1. **Terminal-Native, No Context Switching**
Aider lives where developers already work: the shell. No browser tabs, no sidebar bloat. The chat, file editing, git operations, and linting all happen in one place.

### 2. **Git as the Source of Truth**
Changes are commits, not magic file rewrites. Every edit is reviewable via `git diff`, undoable via `git revert`, and attributable. Developers trust git; they trust aider.

### 3. **Explicit Control with Diffs**
Users see every proposed change before applying it. The diff-first approach means:
- No silent failures or mangled code.
- Easy to reject bad suggestions.
- LLM stays honest—it knows changes will be scrutinized.

### 4. **Tight Feedback Loop**
Chat → Diff → Accept → Commit → Chat (repeat). Each cycle is fast. No IDE plugin restart, no model reloading. The REPL-like flow is addictive for terminal-proficient developers.

### 5. **Repository-Wide Awareness Without Bloat**
The repo-map means aider understands how files relate without requiring them all in the prompt. It can refactor across 10 files, touching only the ones that matter, with intelligence about the rest.

### 6. **Model-Agnostic**
Seamlessly switch between Claude, GPT-4o, DeepSeek, o1. Use the best model for the job without re-inventing the workflow.

### 7. **Reproducibility**
Every change is a commit with a message. Bugs can be traced. Teammates can review. Contracts with CI/CD are honored.

## 8. What Aider Doesn't Do (Gaps a TUI Editor Fills)

### **Live File Editing**
Aider proposes changes; you accept them. But if you want to edit a file mid-session without AI, you switch to vim or your editor. No integrated editor buffer.

### **LSP-Style IntelliSense**
No real-time autocomplete, no "jump to definition," no diagnostics sidebar. Aider doesn't replace your editor; it augments git-based workflows.

### **File Tree Navigation**
Aider is modal: chat or command. No sidebar tree view, no point-and-click file explorer. Context is managed via `/add`, `/drop`, `/ls`.

### **Persistent Layout**
Aider is a REPL, not an IDE. Close it, lose the scroll history (unless you save it). No persistent session state, no pinned windows or splits.

### **Streaming Diffs in Real-Time**
Aider shows diffs after the full response. A TUI editor could stream diffs incrementally, showing changes as they arrive—tighter feedback.

### **Multi-Cursor / Live Collaboration**
Single-user, single-model-at-a-time. No multi-user editing or concurrent AI proposals.

## 9. Aider Primitives That Translate to a TUI IDE

A terminal-native code editor built around AI should adopt these patterns:

1. **Repo-Map for Codebase Awareness**: Build a persistent symbol graph (tree-sitter based) that LLMs can query. Only ship relevant definitions in prompts, not entire codebases.

2. **/add /drop for Context Management**: Explicit file inclusion/exclusion commands. Users decide what the LLM sees. Reduces confusion, controls costs.

3. **Unified Diff Display & Accept/Reject**: Show diffs by default. Let users see and approve changes before writing to disk. Streaming diffs are a bonus.

4. **Auto-Commit with Attribution**: Every AI change becomes a git commit with a sensible message. Make undo trivial via `git revert` on specific commits.

5. **Multi-Mode Chat**: Code mode (edit), ask mode (discuss), architect mode (two-LLM collaboration). Users pick the right tool for the task.

6. **Slash Commands as First-Class**: Build a command palette. Don't hide power behind UI buttons—let developers type. `/model`, `/clear`, `/undo`, `/add file.rs`.

7. **Linting & Testing Integration**: After edits, auto-run linters/tests. Fix failures iteratively. Bake this into the apply loop, not as an afterthought.

8. **Dirty-Tree Safety**: Never let AI edits overwrite uncommitted work. Commit user changes first, then AI changes. Make recovery obvious.

## 10. Concrete UX Flow: Renaming a Function Across 3 Files

### Scenario
You want to rename `calculate_total()` to `compute_sum()` across three files: `calculator.py`, `main.py`, and `utils.py`.

### Steps

**1. Start aider**
```bash
aider calculator.py main.py utils.py
```

**2. Issue the request**
```
You: Rename calculate_total to compute_sum across all three files.
```

**3. Aider responds with diffs**
```
calculator.py
<<<<<<< ORIGINAL
def calculate_total(items):
    return sum(items)
=======
def compute_sum(items):
    return sum(items)
>>>>>>> UPDATED

main.py
<<<<<<< ORIGINAL
result = calculate_total([1, 2, 3])
=======
result = compute_sum([1, 2, 3])
>>>>>>> UPDATED

utils.py
<<<<<<< ORIGINAL
from calculator import calculate_total
=======
from calculator import compute_sum
>>>>>>> UPDATED
```

**4. User confirmation**
```
Apply this change? [y/n/a/q] > a
```

User presses `a` to accept all.

**5. Aider applies and commits**
```
Applied changes to calculator.py, main.py, utils.py
Committed: "aider: rename calculate_total to compute_sum"
```

**6. User verifies**
```
$ /diff
commit abc123...
Author: Claude <aider@example.com>

Changes in 3 files:
- calculator.py: function definition
- main.py: function call
- utils.py: import statement

$ git log --oneline -1
abc123 aider: rename calculate_total to compute_sum
```

**7. Continue or undo**
- If happy: move on to the next request.
- If not: `/undo` reverts to the pre-rename state.

---

## Summary: Why Aider Matters for Cursor Competitors

Aider proves that a terminal-native, git-aware, diff-first AI editor is **stickier** than browser-based GUI tools for many developers. Its power comes from:

- **Simplicity**: No fancy UI, just a chat REPL with git integration.
- **Control**: Every change is visible, stoppable, and versionable.
- **Speed**: Chat → Diff → Commit happens in seconds without UI overhead.
- **Trust**: Git makes everything auditable and reversible.

A TUI Cursor would need to match or beat these in terminal form: a modular command palette, real-time repo awareness, streaming diffs, and git as the safety net. The "gaps" (LSP, live editing, file trees) are secondary—developers already have editors for that. What they don't have is an AI that respects their git workflow and lets them stay in the shell.

---

## References

- [Aider Homepage](https://aider.chat/)
- [Aider In-Chat Commands](https://aider.chat/docs/usage/commands.html)
- [Aider Chat Modes](https://aider.chat/docs/usage/modes.html)
- [Aider Edit Formats](https://aider.chat/docs/more/edit-formats.html)
- [Building a Better Repository Map with Tree-Sitter](https://aider.chat/2023/10/22/repomap.html)
- [Aider Git Integration](https://deepwiki.com/Aider-AI/aider/4.3-git-integration)
- [Aider Usage Tips](https://aider.chat/docs/usage/tips.html)
- [Why Developers Prefer Aider's Terminal Workflow](https://emergent.sh/learn/cursor-vs-aider)
- [Options Reference](https://aider.chat/docs/config/options.html)
- [Aider Installation & Getting Started](https://aider.chat/docs/install.html)
