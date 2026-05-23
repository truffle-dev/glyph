# Language Server Protocol Client Implementation in Go for Bubble Tea TUI

Research date: May 2026

## 1. Existing Go LSP Client Libraries

### go.lsp.dev/protocol + go.lsp.dev/jsonrpc2

**Status**: De-facto standard for Go LSP clients. Actively maintained and widely used.

**API Structure**:
- `go.lsp.dev/protocol`: Type-safe protocol definitions for LSP 3.17+. Covers all standard capabilities: `ClientCapabilities`, `ServerCapabilities`, notifications, requests.
- `go.lsp.dev/jsonrpc2`: JSON-RPC 2.0 transport implementation with bidirectional `Conn` interface.

**Key APIs**:
```go
// jsonrpc2.Conn
Call(ctx context.Context, method string, params, result interface{}) (ID, error)
Notify(ctx context.Context, method string, params interface{}) error
Go(ctx context.Context, handler Handler)

// protocol dispatcher
ClientDispatcher(conn jsonrpc2.Conn, logger *zap.Logger) Client
ServerDispatcher(conn jsonrpc2.Conn, logger *zap.Logger) Server
```

**Strengths**:
- Type-safe protocol definitions using Go structs
- Zero-copy message handling for performance
- Comprehensive LSP 3.17 feature coverage
- Stream abstraction (raw vs. HTTP Content-Length headers for LSP)
- Used by gopls and other production servers

**Weaknesses**:
- Requires zap logger dependency (good, but opinionated)
- Handler pattern requires understanding goroutine lifecycle

### golang.org/x/tools/internal/lsp

**Status**: Internal package; stable API shape but not a public guarantee.

**Details**:
- Implements LSP server-side logic within gopls.
- `lsprpc` package provides `ClientBinder` and `Handshaker` for connection setup.
- `lsppos` mapper handles UTF-8 to UTF-16 position conversion elegantly.
- Not designed as a reusable client library; meant for gopls internals.

**Recommendation**: Avoid for client-side code; use only for reference on position handling.

### sourcegraph/jsonrpc2

**Status**: Older, less maintained than go.lsp.dev alternative.

**Details**:
- Pure JSON-RPC 2.0 implementation without LSP types.
- Provides `Conn` interface with `Call`, `Notify`, `Go` methods.
- Batch requests/responses not supported.
- Useful if you need bare JSON-RPC without protocol types.

**Recommendation**: Use go.lsp.dev stack instead; more mature for LSP.

### Summary

**Recommended primary choice**: `go.lsp.dev/protocol` + `go.lsp.dev/jsonrpc2`
- Production-grade type safety
- Full LSP 3.17 support
- Active maintenance
- Used by gopls ecosystem

---

## 2. Protocol Lifecycle

### Exact Handshake Sequence

1. **Initialize (client → server request)**
   ```
   Request: initialize
   Params: InitializeParams {
       processId: <int or null>,
       clientInfo: ClientInfo { name: "glyph", version: "0.1.0" },
       rootPath: <string or null>,
       rootUri: <DocumentURI or null>,
       initializationOptions: <arbitrary JSON>,
       capabilities: ClientCapabilities { ... },
       workspaceFolders: <array of WorkspaceFolder or null>,
   }
   Response: InitializeResult {
       capabilities: ServerCapabilities { ... },
       serverInfo: ServerInfo { name: "gopls", version: "..." },
   }
   ```
   - Server must not process requests until after receiving initialize.
   - Server responds with its capabilities.

2. **Initialized (client → server notification)**
   ```
   Notification: initialized
   Params: InitializedParams {} (empty)
   ```
   - Only after this can the server send requests/notifications to the client.

3. **Document Open (client → server notification)**
   ```
   Notification: textDocument/didOpen
   Params: DidOpenTextDocumentParams {
       textDocument: TextDocumentItem {
           uri: "file:///path/to/file.go",
           languageId: "go",
           version: 1,
           text: <file contents>,
       },
   }
   ```

4. **Document Changes (client → server notification)**
   ```
   Notification: textDocument/didChange
   Params: DidChangeTextDocumentParams {
       textDocument: VersionedTextDocumentIdentifier { uri, version },
       contentChanges: [
           { range: Range, text: "new text" },  // incremental
           // OR
           { text: "entire new content" },      // full sync
       ],
   }
   ```
   - **Version tracking is mandatory**: each change must increment the version.
   - Server processes changes in order, creating new document state.

5. **Shutdown (client → server request)**
   ```
   Request: shutdown
   Response: null (or error)
   ```

6. **Exit (client → server notification)**
   ```
   Notification: exit
   ```
   - Server terminates.

### Key Capabilities a TUI Client Should Advertise

In `ClientCapabilities`, enable:
- `textDocument.completion`: Code completion
- `textDocument.hover`: Hover tooltips
- `textDocument.definition`: Go-to-definition
- `textDocument.references`: Find references
- `textDocument.diagnostic`: Inline diagnostics (LSP 3.17+)
- `textDocument.documentSymbol`: Outline / symbol list
- `workspace.symbol`: Workspace-wide symbol search
- `textDocument.codeAction`: Quick fixes / refactoring
- `window.showMessage`: Display notifications
- `workspace.didChangeConfiguration`: Settings updates

### Server-Initiated Notifications Handled by Client

- `textDocument/publishDiagnostics`: Errors, warnings, hints
- `$/progress`: Long-running operation progress
- `window/logMessage`: Debug/info messages
- `window/showMessage`: User-facing notifications
- `workspace/applyEdit`: Request to apply a workspace edit

---

## 3. Concurrency Model for Bubble Tea

LSP is **fully bidirectional**: the server can push notifications at any time, not just in response to client requests. A Bubble Tea TUI must handle this without blocking the update loop.

### Recommended Pattern: Notification Channel + tea.Cmd

```
┌─────────────────────────────────────┐
│   Bubble Tea Event Loop (main)      │
│  (sequential Update calls)          │
└──────────────────────────────────────┘
           ▲
           │ tea.Msg
           │
    ┌──────┴────────┐
    │               │
  User Input   Async Cmd
                    │
        ┌───────────┘
        │
┌───────▼────────────────────────────┐
│  Notification Goroutine             │
│  (reads from server, buffers msgs)  │
│                                     │
│  for msg := range rpcConn:          │
│    msgChan <- LSPNotificationMsg{}  │
└─────────────────────────────────────┘
```

### Implementation Pattern

1. **Spawn goroutine reading from LSP connection** (in an init Cmd):
   ```go
   func (m Model) listenForNotifications() tea.Cmd {
       return func() tea.Msg {
           for {
               // jsonrpc2.Conn provides notifications via its Handler
               // or via explicit polling
               
               // Example: create a channel-based handler
               select {
               case notify := <-m.lspNotifications:
                   return LSPNotificationMsg{
                       method: notify.Method,
                       params: notify.Params,
                   }
               case <-m.ctx.Done():
                   return tea.Quit()
               }
           }
       }
   }
   ```

2. **Route notifications in Update**:
   ```go
   case LSPNotificationMsg:
       switch msg.method {
       case "textDocument/publishDiagnostics":
           return m.handleDiagnostics(msg.params)
       case "$/progress":
           return m.handleProgress(msg.params)
       case "window/logMessage":
           return m.handleLogMessage(msg.params)
       }
   ```

3. **Send requests and didChange from Update/Cmd**:
   ```go
   func (m Model) sendDidChange(newText string) tea.Cmd {
       return func() tea.Msg {
           _, err := m.lspClient.DidChange(context.Background(), &protocol.DidChangeTextDocumentParams{
               TextDocument: protocol.VersionedTextDocumentIdentifier{
                   URI:     protocol.URI(m.currentFileURI),
                   Version: int32(m.version),
               },
               ContentChanges: []protocol.TextDocumentContentChangeEvent{
                   {Text: newText},
               },
           })
           if err != nil {
               return ErrorMsg{err}
           }
           m.version++
           return nil
       }
   }
   ```

### Why This Works

- **No blocking**: LSP reads happen in a background goroutine.
- **No mutexes**: Notifications are queued via a channel; Update processes them sequentially.
- **Lagless UI**: The main event loop stays responsive.
- **Clean shutdown**: Context cancellation stops the listener goroutine.

---

## 4. Per-Language-Server Lifecycle Management

### Spawn-on-Demand Pattern

```go
type LanguageServerCache struct {
    servers map[string]*ServerInstance // key: workspace root
    mu      sync.RWMutex
}

type ServerInstance struct {
    cmd    *exec.Cmd
    conn   jsonrpc2.Conn
    client protocol.Client
    uri    protocol.DocumentURI
}

func (c *LanguageServerCache) GetOrSpawn(
    ctx context.Context,
    workspaceRoot string,
    language string,
) (*ServerInstance, error) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if srv, ok := c.servers[workspaceRoot]; ok {
        return srv, nil
    }

    // Spawn server (e.g., gopls, pyright, rust-analyzer)
    srv := &ServerInstance{}
    
    // 1. Start subprocess
    cmd := exec.CommandContext(ctx, "gopls", "serve", "-listen=stdio")
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    srv.cmd = cmd
    
    // 2. Create JSON-RPC connection
    stream := jsonrpc2.NewStream(struct {
        io.Reader
        io.WriteCloser
    }{stdout, stdin})
    conn := jsonrpc2.NewConn(stream)
    srv.conn = conn
    
    // 3. Create LSP client dispatcher
    srv.client = protocol.ClientDispatcher(conn, logger)
    
    // 4. Initialize
    initResp, err := srv.client.Initialize(ctx, &protocol.InitializeParams{
        RootURI: protocol.URI("file://" + workspaceRoot),
        Capabilities: protocol.ClientCapabilities{
            TextDocument: &protocol.TextDocumentClientCapabilities{
                Completion: &protocol.CompletionOptions{},
                Hover: &protocol.HoverOptions{},
                Definition: &protocol.DefinitionOptions{},
            },
        },
    })
    if err != nil {
        return nil, err
    }
    
    // 5. Send initialized notification
    srv.client.Initialized(ctx, &protocol.InitializedParams{})
    
    c.servers[workspaceRoot] = srv
    return srv, nil
}

func (c *LanguageServerCache) Shutdown(ctx context.Context, workspaceRoot string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if srv, ok := c.servers[workspaceRoot]; ok {
        srv.client.Shutdown(ctx)
        srv.client.Exit(ctx)
        srv.cmd.Wait()
        delete(c.servers, workspaceRoot)
    }
}
```

### Idle Timeout (Optional)

Use a timer to kill servers after N minutes of inactivity:
```go
type ServerInstance struct {
    // ... fields above
    lastActivityTime time.Time
    idleTimeout      time.Duration
}

func (c *LanguageServerCache) CleanupIdle(now time.Time) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    for root, srv := range c.servers {
        if now.Sub(srv.lastActivityTime) > srv.idleTimeout {
            c.Shutdown(context.Background(), root)
        }
    }
}
```

---

## 5. Concrete Code Sketch: ~50 Lines

```go
package lsp

import (
    "context"
    "io"
    "os/exec"
    
    "go.lsp.dev/jsonrpc2"
    "go.lsp.dev/protocol"
)

// Spawn gopls and initialize
func NewGoplsClient(ctx context.Context, workspaceRoot string) (protocol.Client, error) {
    // 1. Start gopls subprocess
    cmd := exec.CommandContext(ctx, "gopls", "serve", "-listen=stdio")
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    
    // 2. Connect via JSON-RPC
    stream := jsonrpc2.NewStream(struct {
        io.Reader
        io.WriteCloser
    }{stdout, stdin})
    conn := jsonrpc2.NewConn(stream)
    client := protocol.ClientDispatcher(conn, nil) // nil logger = noop
    
    // 3. Initialize
    _, err := client.Initialize(ctx, &protocol.InitializeParams{
        RootURI: protocol.URI("file://" + workspaceRoot),
        Capabilities: protocol.ClientCapabilities{
            TextDocument: &protocol.TextDocumentClientCapabilities{
                Completion: &protocol.CompletionOptions{},
                Hover:      &protocol.HoverOptions{},
                Definition: &protocol.DefinitionOptions{},
            },
        },
    })
    if err != nil {
        return nil, err
    }
    
    // 4. Send initialized
    client.Initialized(ctx, &protocol.InitializedParams{})
    
    // 5. Open a file
    err = client.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
        TextDocument: protocol.TextDocumentItem{
            URI:        protocol.URI("file://path/to/main.go"),
            LanguageID: "go",
            Version:    1,
            Text:       "package main\n",
        },
    })
    if err != nil {
        return nil, err
    }
    
    // 6. Handle incoming notifications in a goroutine
    go func() {
        // Connect a handler to receive notifications
        conn.Go(ctx, jsonrpc2.HandlerFunc(func(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
            method := r.Method()
            switch method {
            case "textDocument/publishDiagnostics":
                // Extract diagnostics and send to UI
                return nil
            case "window/logMessage":
                // Log to status line
                return nil
            }
            return nil
        }))
    }()
    
    return client, nil
}

// Usage in tea.Cmd
func (m Model) requestCompletion(line, col int) tea.Cmd {
    return func() tea.Msg {
        pos := protocol.Position{Line: uint32(line), Character: uint32(col)}
        items, err := m.lspClient.Completion(context.Background(), &protocol.CompletionParams{
            TextDocumentPositionParams: protocol.TextDocumentPositionParams{
                TextDocument: protocol.TextDocumentIdentifier{
                    URI: protocol.URI(m.currentFileURI),
                },
                Position: pos,
            },
        })
        if err != nil {
            return CompletionErrorMsg{err}
        }
        return CompletionResultsMsg{items}
    }
}
```

---

## 6. Rendering LSP Data in the Editor

### Diagnostics

- **Gutter markers**: Render ▼ or ✗ on lines with errors/warnings
- **Status line**: Show count of errors/warnings in current file
- **Problems pane** (optional): Scrollable list of all diagnostics with jump-to-file

```go
type Diagnostic struct {
    Line      int
    Character int
    Severity  protocol.DiagnosticSeverity
    Message   string
}

// In View():
for line := 0; line < len(m.lines); line++ {
    if errs := m.diagnostics[line]; len(errs) > 0 {
        severity := errs[0].Severity
        if severity == protocol.DiagnosticSeverityError {
            fmt.Fprint(&b, " ✗ ")
        } else {
            fmt.Fprint(&b, " ◆ ")
        }
    }
    fmt.Fprintln(&b, m.lines[line])
}
```

### Hover

- **Floating popup**: Display server's hover response at cursor position
  - Use a fixed-size box (20-30 chars wide, 5-10 lines tall)
  - Show markdown/plaintext from `MarkupContent`

- **Status line fallback**: Show hover text in status bar (one-liner)

```go
func (m Model) requestHover(line, col int) tea.Cmd {
    return func() tea.Msg {
        hover, err := m.lspClient.Hover(context.Background(), &protocol.HoverParams{
            TextDocumentPositionParams: protocol.TextDocumentPositionParams{
                TextDocument: protocol.TextDocumentIdentifier{URI: protocol.URI(m.uri)},
                Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
            },
        })
        if err != nil {
            return nil
        }
        return HoverMsg{hover}
    }
}
```

### Goto Definition

- **Jump-to-file**: Parse `Location` from response, open file and jump to line/col
- **Link-like behavior**: Cmd+click or Ctrl+click to follow

```go
case KeyMsg:
    if msg.String() == "ctrl+g" {
        return m.gotoDefinition()
    }

func (m Model) gotoDefinition() tea.Cmd {
    return func() tea.Msg {
        locs, err := m.lspClient.Definition(context.Background(), ...)
        if err != nil || len(locs) == 0 {
            return StatusMsg("No definition found"}
        }
        loc := locs[0]
        return OpenFileMsg{
            path: uriToPath(loc.URI),
            line: int(loc.Range.Start.Line),
            col:  int(loc.Range.Start.Character),
        }
    }
}
```

### Completion

Three strategies:

1. **Ghost text** (VSCode-style): Show greyed completion as suffix on current line; Tab to accept
2. **Popup picker**: Small dropdown list; arrow keys to navigate, Enter to select
3. **Search mode**: Type to filter; ↓/↑ to move; Enter to accept

```go
// In Model:
type CompletionState struct {
    Items      []*protocol.CompletionItem
    Filter     string
    Selected   int
}

// Trigger on typing (debounce ~300ms)
func (m Model) startCompletion() tea.Cmd {
    return func() tea.Msg {
        items, err := m.lspClient.Completion(context.Background(), ...)
        if err != nil {
            return nil
        }
        return CompletionMsg{items}
    }
}

// Render picker in View()
func (m Model) renderCompletionPicker() string {
    if m.completion == nil {
        return ""
    }
    // Draw a box with filtered items, highlight selected
}
```

### Signature Help

- **Status line**: Show function signature as you type arguments
- **Auto-trigger** after `(` or `,` in function calls

```go
func (m Model) requestSignature(line, col int) tea.Cmd {
    return func() tea.Msg {
        sig, err := m.lspClient.SignatureHelp(context.Background(), &protocol.SignatureHelpParams{
            TextDocumentPositionParams: ...,
        })
        if err != nil {
            return nil
        }
        return SignatureMsg{sig}
    }
}

// In View() status line:
if m.signature != nil && len(m.signature.Signatures) > 0 {
    return m.signature.Signatures[0].Label
}
```

---

## 7. Pitfalls to Know About

### Position Encoding (UTF-16 vs UTF-8)

**Problem**: LSP uses UTF-16 offsets by default. A file with emoji/non-ASCII breaks naive byte counting.

Example: The string "a𐐀b" (with DESERET CAPITAL LETTER A, a 4-byte UTF-8 sequence):
- UTF-8 byte offsets: a=0, 𐐀=1..4, b=5
- UTF-16 code units: a=0, 𐐀=1..2 (surrogate pair), b=3

**Solution**:
1. Use `golang.org/x/tools/internal/lsp/lsppos.Mapper` for conversion:
   ```go
   mapper := lsppos.NewMapper([]byte(fileContent))
   pos := mapper.Position(byteOffset) // returns protocol.Position with UTF-16 aware line/col
   ```

2. Or negotiate UTF-8 in init:
   ```go
   capabilities.general.positionEncodings = []string{"utf-8", "utf-16"}
   // Server responds with chosen encoding in `capabilities.positionEncoding`
   ```

### Workspace/Configuration vs InitializationOptions

**Problem**: Some servers use `initializationOptions`, others use `workspace/didChangeConfiguration`.

**Solution**:
1. Advertise both capabilities:
   ```go
   capabilities.workspace.didChangeConfiguration = true
   ```

2. Send initial config in `initializationOptions`:
   ```go
   InitializeParams.InitializationOptions = json.RawMessage(`{"usePlaceholders": true}`)
   ```

3. Listen for config requests:
   ```go
   case "workspace/configuration":
       // Client responds with current settings
       return m.getWorkspaceConfig()
   ```

### File Versioning (didChange Must Include Monotonic Version)

**Problem**: Each `didChange` notification requires an incremented `version`. Servers use version to detect missing changes.

**Solution**:
```go
type Document struct {
    URI     string
    Content string
    Version int32
}

func (d *Document) ApplyChange(newText string) {
    d.Content = newText
    d.Version++  // Always increment
    
    m.lspClient.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
        TextDocument: protocol.VersionedTextDocumentIdentifier{
            URI:     protocol.URI(d.URI),
            Version: d.Version,  // Send new version
        },
        ContentChanges: []protocol.TextDocumentContentChangeEvent{
            {Text: newText},
        },
    })
}
```

### Cancellation Tokens

**Problem**: Long-running operations (completion, formatting) may be cancelled if user moves cursor or presses Escape.

**Solution**:
```go
// Track tokens per operation type
type RequestTokens struct {
    completion context.CancelFunc
    formatting context.CancelFunc
}

// On completion request:
ctx, cancel := context.WithCancel(parentCtx)
m.tokens.completion = cancel

go func() {
    items, err := m.lspClient.Completion(ctx, ...)
    // If ctx was cancelled, this returns early
}()

// On Escape key:
if m.tokens.completion != nil {
    m.tokens.completion()
    m.tokens.completion = nil
}
```

---

## 8. Recommended Approach for Glyph Editor

**Start with `go.lsp.dev/protocol` + `go.lsp.dev/jsonrpc2`.** Spawn the language server (gopls, rust-analyzer, pyright, typescript-language-server) on demand when opening the first file of that language. Maintain a workspace-keyed cache of server processes. Route server notifications via a channel-based handler to a background goroutine, which posts LSP messages to the Bubble Tea model via a notification channel—this keeps the UI event loop lock-free and responsive. Render diagnostics as gutter markers and status-line summary; display hover hints in a floating overlay or status-line fallback. Implement goto-definition as a file-jump action; provide completion as a ghost-text suffix with Tab-to-accept, or optionally a picker dropdown. Defer references and workspace symbols to a dedicated pane/picker for batch results. Always use `golang.org/x/tools/internal/lsp/lsppos` for position mapping to handle UTF-8 ↔ UTF-16 conversion correctly and avoid emoji/non-ASCII bugs. Version every `didChange` and respect server-initiated configuration requests. Start simple (diagnostics + hover + completion) and incrementally add goto/references/actions as UX matures.

---

## References

- [Language Server Protocol 3.17 Specification](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/)
- [go.lsp.dev/protocol Package](https://pkg.go.dev/go.lsp.dev/protocol)
- [go.lsp.dev/jsonrpc2 Package](https://pkg.go.dev/go.lsp.dev/jsonrpc2)
- [golang.org/x/tools/internal/lsp/lsppos](https://pkg.go.dev/golang.org/x/tools/internal/lsp/lsppos)
- [Gopls: The language server for Go](https://go.dev/gopls/)
- [sourcegraph/jsonrpc2 GitHub](https://github.com/sourcegraph/jsonrpc2)
- [Building a Terminal IRC Client with Bubble Tea](https://sngeth.com/go/terminal/ui/bubble-tea/2025/08/17/building-terminal-ui-with-bubble-tea/)
- [Bubble Tea Concurrency Guide](https://deepwiki.com/charmbracelet/bubbletea/5.1-concurrency-and-goroutines)

