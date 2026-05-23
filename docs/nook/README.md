# nook

A terminal-native AI IDE built from glyph components. Single binary, opens
any project, runs over SSH, ships with picker-driven navigation, project
search, git integration, embedded terminal, LSP, and an AI panel.

This directory holds the design docs. The binary itself lives at
`cmd/nook/` once we start shipping it.

## Files

- [`spec.md`](spec.md) — the product spec and architecture (read this first)
- [`research/00-synthesis-and-spec.md`](research/00-synthesis-and-spec.md) — same content, kept as the research-foundation copy
- [`research/01-cursor-features.md`](research/01-cursor-features.md) — Cursor feature inventory + MVP shortlist
- [`research/02-helix-architecture.md`](research/02-helix-architecture.md) — Helix architecture deep-dive (compositor, picker, LSP, selection model)
- [`research/03-neovim-ai-plugins.md`](research/03-neovim-ai-plugins.md) — Avante / CodeCompanion / copilot.lua / aider patterns
- [`research/04-lsp-go-client.md`](research/04-lsp-go-client.md) — `go.lsp.dev` + concurrency model + lifecycle
- [`research/05-aider-ux.md`](research/05-aider-ux.md) — aider end-to-end UX, repo-map, slash commands, git-native auto-commit
- [`research/06-zed-distinctive.md`](research/06-zed-distinctive.md) — Zed's multibuffer + inline assistant + outline pattern
- [`research/07-tui-ide-landscape.md`](research/07-tui-ide-landscape.md) — micro / amp / kakoune / Edit / nvim distros / convergence patterns
- [`research/08-developer-workflow.md`](research/08-developer-workflow.md) — what a terminal-native dev's day actually looks like

## Why nook

Cursor is electron and expensive. Helix has no AI. Neovim+plugins is fragile.
aider is a REPL, not an IDE. Zed is GPU, not TUI. The unfilled space: a TUI
that has Cursor's AI UX, Helix's picker UX, aider's git ergonomics, and Zed's
multibuffer — in a single binary. Built from glyph primitives. That's nook.
