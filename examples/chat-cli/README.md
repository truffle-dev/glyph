# chat-cli

A realistic agent-style chat REPL composed from **thirteen** glyph components.
Every component on screen earns its place by serving the chat surface — there
is no tabbed showcase, no decorative widgets, no glyph-specific glue beyond
Bubble Tea composition.

```bash
go run ./examples/chat-cli/
```

## What's on screen

| Component | Where it shows up |
| --- | --- |
| `status-bar` | top: mode badge, model name, message count |
| `chat-thread` | middle: scrollable message history |
| `chat-bubble` | each row inside the thread (used via thread) |
| `chat-input` | bottom: focused input with prompt and placeholder |
| `key-hints` | below the input: current-mode key bindings |
| `notification-toast` | floating tray, top-right |
| `spinner` | inline next to the assistant label while a reply is in flight |
| `theme` | every color in the layout |

## Surfaces that overlay the chat on demand

| Key | Overlay | Components used |
| --- | --- | --- |
| `Ctrl-P` | command palette | `command-palette` |
| `Ctrl-S` | save dialog | `modal` + `text-input` |
| `Ctrl-L` | clear-conversation prompt | `modal` + `confirmation` |
| `Ctrl-R` | model picker | `select` |

That covers thirteen registry items. The thread, when active, runs the spinner
component as part of the same view, so the spinner-tick wiring is exercised
even when no overlay is open.

## Keys

```
Enter       send the current input
Ctrl-P      open command palette
Ctrl-S      open save dialog
Ctrl-L      open clear-conversation prompt
Ctrl-R      open model picker
Esc         close any overlay
Ctrl-C      quit (works in every mode)
```

## Why this file is in `examples/`

The pattern for composing glyph in your own app is what this demo
demonstrates. Read the `Update` switch in `main.go` to see how each component's
message type is routed:

- `chatinput.SubmitMsg` lands in the chat-mode branch and triggers a fake
  assistant reply.
- `commandpalette.SelectMsg` runs the named command.
- `selectinput.SelectMsg`, `confirmation.ConfirmMsg`, `textinput.SubmitMsg`
  each close their respective overlay and act on the result.
- `notificationtoast.Tray.Tick` is driven by a global 1 Hz `tickMsg`.

The same shape works for a real model call: replace `fakeReply` with a goroutine
that talks to your backend and posts a `replyMsg` when it returns.

## Tests

```bash
go test ./examples/chat-cli/
```

The test file exercises the model logic headlessly: send a message, receive a
reply, open each overlay, switch models, save, clear, expire toasts, and
verify Ctrl-C quits from every mode.
