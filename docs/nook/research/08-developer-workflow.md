# A Day in the Life: Terminal Developer Workflow

A candid field report of how a terminal-native developer actually spends an 8-hour coding day in 2026 — nvim/helix/emacs user, tmux-bound, ssh'd into a dev box. The goal is to design our TUI editor against the *actual* workflow, not the imagined one.

## 6:30 AM — The Morning Ritual

The developer's laptop boots. They ssh into the remote dev box (`ssh dev.company.com`), or if the project is local, they skip this. Either way, they cd to their main project directory.

**The first move:** Open tmux. Muscle memory. Either `tmux attach` (if a session survived overnight) or `tmux new-session -s main`. Most developers running long-term leave sessions alive — they attach to yesteryear's environment. It has the repo already in the right place, perhaps the test watcher still running in pane 2, the build log in pane 3.

**The second move:** Inside tmux, they navigate to the repo root and run:

```bash
git pull
git status
```

This isn't nostalgic — it's tactical. They need to know:
- Did someone push a breaking change overnight?
- Are there merge conflicts waiting?
- What branch am I on?

Then, depending on project:

```bash
cargo build --tests
pytest --co -q          # or: npm test -- --listTests
```

Not to run the full suite yet. Just to see if the codebase compiles. If it doesn't, that's the morning problem to solve before touching anything else.

**Split 1:** `nvim .` (or `helix .` or `emacs` with `projectile-mode`). Open the editor. Most editors have an integrated file browser or fuzzy finder that launches immediately. They're not opening a specific file; they're opening the *project*. A few seconds later, they're either:
- Looking at last week's open buffers (if the editor persisted them), or
- Running `:Telescope find_files` (nvim) / `:fzf` (emacs) / `<space>f` (helix) to navigate to the file they know they need to edit.

**Split 2 (tmux pane):** While the editor loads, they might run `lazygit` in another pane, or leave it open to a shell for quick `git log --oneline -20`. Some developers keep one pane perpetually showing `git status --short`.

## 8:15 AM — The Edit-Test Loop

The developer opens `/app/services/auth/token.rs`. They know approximately where the bug is (from the 9 PM Slack message about login failures). They press:

**Helix:** `<space>wf` → finds `validate_token` in the current file. Cursor jumps. They read 30 lines of context. Then `<space>/` to open a project-wide search: search for "expiry check" across all Rust files. Results appear in a dropdown. Arrow keys navigate. `<enter>` to jump to the first result (a comment in a test). `n` to go to next match.

**NeoVim:** Similar, but `<leader>ss` or `:Telescope grep_string`. Then they type the search term. Telescope shows results live. Arrow keys. Enter to jump.

**Emacs:** `C-c C-f` (or `projectile-grep`) opens a grep buffer. Type the pattern. Results populate. `RET` to visit.

Once they find the bug (a missing null check), they edit in place — `i`, type the fix, `<esc>`, `:w`.

**Immediately after save:** The test watcher (which has been idling in tmux pane 2) recompiles and re-runs the tests touching that file. 2 seconds later:

```
test token::validate_expiry::test_future ... ok
test token::validate_expiry::test_past  ... ok
test token::validate_expiry::test_null  ... FAIL  <-- still fails
```

The developer is **looking at the error message in the same tmux pane** — not inside the editor. They switch to that pane (`<ctrl-b>p` in tmux), scroll up to see the full stack trace, copy the line number and file path, then switch back to the editor and jump to that line: `:e path/to/file.rs +123`.

This is the 15-minute loop, repeated 5-8 times a morning: edit, save, read error, jump, fix.

## 10:30 AM — Project Navigation & Search

A teammate asks, "Who's calling `process_payment`?" The developer opens tmux, goes to the shell pane, and runs:

```bash
rg "process_payment" --type rs -A 2 -B 2
```

Output: 47 matches across 12 files. Too many to eyeball. They pipe to fzf:

```bash
rg "process_payment" --type rs | fzf --preview 'bat --highlight-line {2}' -m
```

Now they can fuzzy-search the matches. They select 3 files (multiselect with `<tab>`). In true fzf + vim fashion, some developers then pipe the result to `xargs vim`:

```bash
rg "process_payment" --type rs | cut -d: -f1 | sort -u | xargs nvim
```

This opens all 3 files in nvim buffers. They navigate between them with `:bnext` / `:bprev` or with leader+j/leader+k if mapped.

**But here's what we're NOT seeing yet:** No editor (including our TUI) gracefully shows you the *exact lines* of all callers in one browsable list, then lets you toggle between editing and viewing the context around each caller. Most developers accept that they'll grep, filter with fzf, then open files one by one. **Zed's multibuffer is the closest fit — a TUI equivalent is a real gap.**

## 12:00 PM — Running Things

The developer's been editing for 3 hours. They now need to run the full test suite before committing. They don't do this in-editor with a `:test` command. Instead:

- **Option A (60% of devs):** They switch to the tmux pane running the test watcher, or a new pane, and run `cargo test --release`. The editor is still open; they watch the terminal. When tests pass, they switch back to edit another file or prepare a commit.
- **Option B (30%):** Some editors support `:make` or `:!cargo test`, which runs the command and captures output in a quickfix list. This is convenient, but if the test run takes 90 seconds, they've lost focus. So they still tend to split the screen.
- **Option C (10%):** They use an editor plugin (vim-fugitive, helix's `:spawn`, emacs' compile mode) to run and capture. Rare, because most developers find tmux splits less intrusive for long-running builds.

**Build failure?** It's in the test output. They scroll up in the terminal pane, find the failing test, copy the line number, switch to the editor, jump to that test, read the assertion, fix the code or the test. Loop.

## 1:45 PM — Git Workflow

By now, it's time to commit. The developer runs `lazygit`, or if they prefer pure CLI:

```bash
git diff --cached       # if they've staged things
git add -p              # interactive add (show me hunks one by one)
```

**With lazygit:** They see the unstaged changes as a list. They navigate (arrow keys) through modified files. `<space>` to stage. `c` to commit. A vim buffer opens for the commit message. They write it, save, exit. `P` to push. Done in 2 minutes.

**With `git add -p`:** They're in an interactive prompt:

```
Stage this hunk? (y,n,s,e,?) y
Stage this hunk? (y,n,s,e,?) n
```

Then:

```bash
git commit -m "fix(auth): handle null expiry in token validation"
git push origin my-branch
```

Then they open the browser *or* use `gh`:

```bash
gh pr create --title "Fix null expiry bug" --body "Fixes issue #442"
```

Some developers never leave the terminal for PR creation. Others open GitHub in a browser to review their own PR before marking it ready.

## 3:00 PM — AI in the Workflow

The developer is now refactoring a large payment-processing module. They select a messy 40-line function and think, "This should be simpler."

They use one of:
- **aider.chat** (in a tmux pane, or integrated into some editors): `aider --model claude-opus-4-7 payment.rs`. They type, "Refactor this function to split payment validation from processing. Keep the same API." Aider edits the file in place. They review the changes in the editor.
- **Claude Code (or VSCode + Copilot):** They select the function, hit a hotkey, and a suggestion panel appears showing the refactored version. They accept or tweak it.
- **Manual AI query:** They copy the function to a browser, paste it into claude.ai or ChatGPT, ask "refactor this", get back code, copy it back. Slow, but happens.

AI happens 5-15 times a day in a typical developer's workflow. Not for every edit — mostly for: refactoring, boilerplate generation, test writing, explaining errors, and (increasingly) debugging stack traces.

## 4:30 PM — Code Review

A teammate posted a PR. The developer opens it:

**Via browser:** They go to GitHub, read the diff, leave comments. Not ideal for long code reviews. Takes 20 minutes, and they're out of context.

**Via terminal:** They run `gh pr checkout 342`. Git checks out the branch. They're now looking at the actual code in their editor. They can:
- Run tests on the code (`cargo test`).
- Open files and read full context (not just the diff).
- Leave comments inline.

It's much better, but still involves switching back to the browser to post comments. Some developers use `gh pr comment 342 -b "..."` from the CLI. But you can't do threaded discussion this way. Most code review still happens in a browser.

## 5:30 PM — Multiple Repos

The developer realizes they need to grab a utility function from a different repo (sibling project). They don't have both open. They:

1. Open a new tmux window or pane.
2. `cd /home/dev/projects/shared-utils && rg "fn calculate_hash" src/`
3. Find the function, `nvim src/crypto.rs +45`.
4. Read it. Copy it mentally (or with a screenshot). Or copy the code to clipboard: `yG` (visual select all, yank).
5. Switch back to the main repo's editor.
6. Paste, adapt, move on.

**What's painful:** No editor lets you keep "two projects open at once" in a single session without splitting the tmux window or opening a second editor instance. Some developers just accept this; others use tmux heavily to manage it. We have a big opportunity here.

## 6:30 PM — Agent Autonomy

The developer's been working on a feature all week. It's mostly done, but there's a 5-file refactor that's tedious: adding a new parameter to 8 functions and updating all callers. They could do it in 45 minutes manually.

Instead, they spin up **claude-code** or **aider** with a prompt:

```
I need to add a 'timeout_ms' parameter to the payment_process function and all its callers.
Here's the function signature: [pastes function]
Here are the callers: [runs rg, pastes output]
Please update all 8 call sites. I'll review the diff when you're done.
```

They hit send. Claude Code spins up in a background tmux session (or runs fully autonomous). They switch to a different repo and start a different task. 30 minutes later, Claude Code says, "Done. Here's the diff." They review, approve, commit.

This is happening maybe 1-3 times a week per developer, for 15-60 minutes each time.

## What's Missing in Today's TUI Editors

Over a year of watching developers, here's what becomes clear:

1. **Cross-repo awareness.** Most editors can't gracefully open a related repo without closing the current one. You want to run a query across your local monorepo or a sibling project without leaving the editor.
2. **Integrated AI with *review*-in-place.** Aider and Claude Code work, but they're separate processes. You get edits back as a patch or file. No editor yet shows you "here are the 4 files AI wants to change, side-by-side, with old vs new, and one button to accept all."
3. **Graceful terminal integration.** Tmux splits work, but they're a workaround, not a feature. A TUI editor that *truly* owns the terminal — with running builds/tests in a sidebar, inline error navigation, and no context-switching — would be radical.
4. **Smart test running.** No editor runs only the tests *relevant to your changes*. They run all tests or none. Developers waste 5-10 minutes a day waiting for full suites when they could run 3 targeted tests in 10 seconds.
5. **Async-safe workflow.** When you spin up an agent (Claude Code, aider), the editor should let you *keep editing* in a different buffer or pane, with a non-blocking status bar showing progress. Right now, most developers context-switch entirely, which kills flow.

## The 5 Must-Have Verbs

To be usable for a real day, the editor must do these well:

1. **Open file / search by name (fuzzy)** — `<space>f` or `ctrl-p`. Fastest in Helix, rock-solid in nvim. Must work across large projects in <200ms.
2. **Search code (ripgrep) + navigate results** — `<space>/` or `:Telescope grep_string`. Results in a list. Jump between matches with `n`. Single-command workflow. *Multibuffer view of results would be a real win.*
3. **Edit + save + auto-test** — Write code, `:w`, immediate feedback in another pane. This is the 80/20 of the day.
4. **Jump to error** — `gg` (go to grep result), `:e file.rs +123`, or `:cc` (quickfix). Fast, single keystroke.
5. **Commit + push** — `lazygit` in-editor or `gh pr create` in a split. Ideally, one tmux pane dedicated to git. Can't force this in the editor; users will split. But the editor shouldn't *get in the way*.

Everything else (AI refactoring, code review, running builds) is nice-to-have or happens in surrounding tools.
