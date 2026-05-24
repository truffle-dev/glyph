// Package lsp drives a language-server subprocess (gopls by default) over
// stdio and surfaces textDocument/publishDiagnostics notifications as events
// the host can render. The wedge stays narrow: open/change/close + a
// diagnostics channel, plus synchronous Completion/Hover/Definition lookups
// that return friendly Go-native shapes instead of leaking LSP protocol
// types into the editor.
package lsp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

// DiagnosticsEvent is a single textDocument/publishDiagnostics notification.
// It is the only event surfaced from the server to the host.
type DiagnosticsEvent struct {
	URI   uri.URI
	Items []protocol.Diagnostic
}

// Client is a thin wrapper over a language server subprocess. It owns the
// process, the jsonrpc2 connection, and a buffered events channel.
type Client struct {
	server protocol.Server
	conn   jsonrpc2.Conn
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	cancel context.CancelFunc

	handler *handler
}

// Options configures the language-server invocation. Zero values are safe.
type Options struct {
	// Binary names the executable to spawn. Defaults to "gopls".
	Binary string
	// Args are extra args to pass to the binary.
	Args []string
	// RootDir is the workspace root used as the LSP root URI.
	RootDir string
	// EventBuffer sets the diagnostics channel capacity. Defaults to 64.
	EventBuffer int
}

// Start spawns the language server, sends initialize+initialized, and returns
// a Client. The server lives until Shutdown is called.
func Start(ctx context.Context, opts Options) (*Client, error) {
	if opts.Binary == "" {
		opts.Binary = "gopls"
	}
	if opts.EventBuffer <= 0 {
		opts.EventBuffer = 64
	}
	if opts.RootDir == "" {
		return nil, errors.New("lsp: RootDir is required")
	}

	cmdCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(cmdCtx, opts.Binary, opts.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("lsp: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("lsp: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("lsp: start %s: %w", opts.Binary, err)
	}

	stream := jsonrpc2.NewStream(stdio{stdin: stdin, stdout: stdout})
	h := &handler{events: make(chan DiagnosticsEvent, opts.EventBuffer)}
	// zap.NewNop avoids leaking server logs into our terminal UI. Use cmdCtx
	// (background-derived, lifetime-of-the-Client) so the connection survives
	// the caller's ctx going out of scope after Start returns.
	_, conn, server := protocol.NewClient(cmdCtx, h, stream, zap.NewNop())

	c := &Client{
		server:  server,
		conn:    conn,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		cancel:  cancel,
		handler: h,
	}

	if err := c.initialize(ctx, opts.RootDir); err != nil {
		_ = c.Shutdown(ctx)
		return nil, err
	}
	return c, nil
}

func (c *Client) initialize(ctx context.Context, rootDir string) error {
	rootURI := uri.File(rootDir)
	_, err := c.server.Initialize(ctx, &protocol.InitializeParams{
		ProcessID: 0,
		RootURI:   rootURI,
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				Synchronization: &protocol.TextDocumentSyncClientCapabilities{
					DynamicRegistration: false,
					WillSave:            false,
					WillSaveWaitUntil:   false,
					DidSave:             true,
				},
				PublishDiagnostics: &protocol.PublishDiagnosticsClientCapabilities{
					RelatedInformation: true,
				},
				Completion: &protocol.CompletionTextDocumentClientCapabilities{
					CompletionItem: &protocol.CompletionTextDocumentClientCapabilitiesItem{
						SnippetSupport:          false,
						CommitCharactersSupport: false,
						DocumentationFormat:     []protocol.MarkupKind{protocol.PlainText, protocol.Markdown},
					},
					ContextSupport: true,
				},
				Hover: &protocol.HoverTextDocumentClientCapabilities{
					ContentFormat: []protocol.MarkupKind{protocol.PlainText, protocol.Markdown},
				},
				Definition: &protocol.DefinitionTextDocumentClientCapabilities{
					LinkSupport: false,
				},
			},
		},
		WorkspaceFolders: []protocol.WorkspaceFolder{
			{URI: string(rootURI), Name: "root"},
		},
	})
	if err != nil {
		return fmt.Errorf("lsp: initialize: %w", err)
	}
	if err := c.server.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		return fmt.Errorf("lsp: initialized: %w", err)
	}
	return nil
}

// Open notifies the server that the host is now editing a document. text is
// the buffer's current contents. languageID is e.g. "go" or "rust".
func (c *Client) Open(ctx context.Context, path, languageID, text string) error {
	return c.server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri.File(path),
			LanguageID: protocol.LanguageIdentifier(languageID),
			Version:    1,
			Text:       text,
		},
	})
}

// Change sends a full-text didChange. version monotonically increments per
// document and starts at 2 (1 is reserved for Open).
func (c *Client) Change(ctx context.Context, path string, version int32, text string) error {
	return c.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri.File(path)},
			Version:                version,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{Text: text},
		},
	})
}

// Close notifies the server that the host is no longer editing the document.
func (c *Client) Close(ctx context.Context, path string) error {
	return c.server.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
	})
}

// Diagnostics returns the channel that receives publishDiagnostics events.
// The channel never closes until Shutdown.
func (c *Client) Diagnostics() <-chan DiagnosticsEvent {
	return c.handler.events
}

// CompletionKind names what a completion item represents (function, variable,
// type, etc.). The string form mirrors LSP's CompletionItemKind enum but stays
// friendly so consumers don't import the protocol package just to switch on it.
type CompletionKind string

// Friendly aliases for the LSP CompletionItemKind enum. Anything unknown maps
// to CompletionKindText so consumers never see an empty string.
const (
	CompletionKindText          CompletionKind = "text"
	CompletionKindMethod        CompletionKind = "method"
	CompletionKindFunction      CompletionKind = "function"
	CompletionKindConstructor   CompletionKind = "constructor"
	CompletionKindField         CompletionKind = "field"
	CompletionKindVariable      CompletionKind = "variable"
	CompletionKindClass         CompletionKind = "class"
	CompletionKindInterface     CompletionKind = "interface"
	CompletionKindModule        CompletionKind = "module"
	CompletionKindProperty      CompletionKind = "property"
	CompletionKindUnit          CompletionKind = "unit"
	CompletionKindValue         CompletionKind = "value"
	CompletionKindEnum          CompletionKind = "enum"
	CompletionKindKeyword       CompletionKind = "keyword"
	CompletionKindSnippet       CompletionKind = "snippet"
	CompletionKindColor         CompletionKind = "color"
	CompletionKindFile          CompletionKind = "file"
	CompletionKindReference     CompletionKind = "reference"
	CompletionKindFolder        CompletionKind = "folder"
	CompletionKindEnumMember    CompletionKind = "enum-member"
	CompletionKindConstant      CompletionKind = "constant"
	CompletionKindStruct        CompletionKind = "struct"
	CompletionKindEvent         CompletionKind = "event"
	CompletionKindOperator      CompletionKind = "operator"
	CompletionKindTypeParameter CompletionKind = "type-parameter"
)

// CompletionItem is the host-side view of one server completion entry. Label
// is what the popup shows; InsertText is what to type into the buffer when
// the user accepts (falls back to Label if the server didn't send one). Detail
// is a short type-or-source string (e.g. "func(s string) int").
type CompletionItem struct {
	Label      string
	InsertText string
	Detail     string
	Kind       CompletionKind
}

// HoverInfo carries the textual content of a hover response. Empty Contents
// means the server returned nothing (e.g. cursor on a comment).
type HoverInfo struct {
	Contents string
}

// Location names a file path plus a 0-indexed line/column. Used as the result
// of Definition.
type Location struct {
	Path string
	Line int
	Col  int
}

// Completion requests completion items at the given 0-indexed line and column.
// Returns an empty slice (not nil) when the server has nothing to suggest, so
// callers can `for _, item := range items` without nil-checking. Errors only
// when the underlying RPC fails.
func (c *Client) Completion(ctx context.Context, path string, line, col int) ([]CompletionItem, error) {
	if c == nil || c.server == nil {
		return nil, errors.New("lsp: client not initialized")
	}
	res, err := c.server.Completion(ctx, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
			Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("lsp: completion: %w", err)
	}
	if res == nil {
		return []CompletionItem{}, nil
	}
	out := make([]CompletionItem, 0, len(res.Items))
	for _, it := range res.Items {
		ci := CompletionItem{
			Label:      it.Label,
			InsertText: it.InsertText,
			Detail:     it.Detail,
			Kind:       completionKindOf(it.Kind),
		}
		if ci.InsertText == "" {
			ci.InsertText = it.Label
		}
		out = append(out, ci)
	}
	return out, nil
}

// Hover returns the textual hover content at the given 0-indexed line and
// column. Empty HoverInfo (no error) when the server has nothing.
func (c *Client) Hover(ctx context.Context, path string, line, col int) (HoverInfo, error) {
	if c == nil || c.server == nil {
		return HoverInfo{}, errors.New("lsp: client not initialized")
	}
	res, err := c.server.Hover(ctx, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
			Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
		},
	})
	if err != nil {
		return HoverInfo{}, fmt.Errorf("lsp: hover: %w", err)
	}
	if res == nil {
		return HoverInfo{}, nil
	}
	return HoverInfo{Contents: res.Contents.Value}, nil
}

// Definition returns target locations for the symbol at the given 0-indexed
// line and column. Empty slice (not nil) when the server resolves nothing.
func (c *Client) Definition(ctx context.Context, path string, line, col int) ([]Location, error) {
	if c == nil || c.server == nil {
		return nil, errors.New("lsp: client not initialized")
	}
	locs, err := c.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
			Position:     protocol.Position{Line: uint32(line), Character: uint32(col)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("lsp: definition: %w", err)
	}
	out := make([]Location, 0, len(locs))
	for _, l := range locs {
		out = append(out, Location{
			Path: uri.URI(l.URI).Filename(),
			Line: int(l.Range.Start.Line),
			Col:  int(l.Range.Start.Character),
		})
	}
	return out, nil
}

// completionKindOf maps the LSP enum to the friendly string. Unknown values
// (e.g. extensions) fall back to "text" so callers don't have to handle
// empty strings.
func completionKindOf(k protocol.CompletionItemKind) CompletionKind {
	switch k {
	case protocol.CompletionItemKindMethod:
		return CompletionKindMethod
	case protocol.CompletionItemKindFunction:
		return CompletionKindFunction
	case protocol.CompletionItemKindConstructor:
		return CompletionKindConstructor
	case protocol.CompletionItemKindField:
		return CompletionKindField
	case protocol.CompletionItemKindVariable:
		return CompletionKindVariable
	case protocol.CompletionItemKindClass:
		return CompletionKindClass
	case protocol.CompletionItemKindInterface:
		return CompletionKindInterface
	case protocol.CompletionItemKindModule:
		return CompletionKindModule
	case protocol.CompletionItemKindProperty:
		return CompletionKindProperty
	case protocol.CompletionItemKindUnit:
		return CompletionKindUnit
	case protocol.CompletionItemKindValue:
		return CompletionKindValue
	case protocol.CompletionItemKindEnum:
		return CompletionKindEnum
	case protocol.CompletionItemKindKeyword:
		return CompletionKindKeyword
	case protocol.CompletionItemKindSnippet:
		return CompletionKindSnippet
	case protocol.CompletionItemKindColor:
		return CompletionKindColor
	case protocol.CompletionItemKindFile:
		return CompletionKindFile
	case protocol.CompletionItemKindReference:
		return CompletionKindReference
	case protocol.CompletionItemKindFolder:
		return CompletionKindFolder
	case protocol.CompletionItemKindEnumMember:
		return CompletionKindEnumMember
	case protocol.CompletionItemKindConstant:
		return CompletionKindConstant
	case protocol.CompletionItemKindStruct:
		return CompletionKindStruct
	case protocol.CompletionItemKindEvent:
		return CompletionKindEvent
	case protocol.CompletionItemKindOperator:
		return CompletionKindOperator
	case protocol.CompletionItemKindTypeParameter:
		return CompletionKindTypeParameter
	default:
		return CompletionKindText
	}
}

// Shutdown sends shutdown+exit, waits briefly, then kills the process. Safe
// to call multiple times and safe on a zero-value Client (used by the
// failure-paths in Start).
func (c *Client) Shutdown(ctx context.Context) error {
	if c == nil {
		return nil
	}
	// Best-effort graceful shutdown. Ignore individual errors so we still
	// cancel the context and reap the process.
	if c.server != nil {
		_ = c.server.Shutdown(ctx)
		_ = c.server.Exit(ctx)
	}
	if c.handler != nil {
		c.handler.markClosed()
	}
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
		c.cmd = nil
	}
	return nil
}

// handler implements protocol.Client. We only care about publishDiagnostics;
// everything else is a no-op that returns a nil error.
type handler struct {
	events chan DiagnosticsEvent

	mu     sync.Mutex
	closed bool
}

func (h *handler) PublishDiagnostics(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
	if params == nil {
		return nil
	}
	h.mu.Lock()
	closed := h.closed
	h.mu.Unlock()
	if closed {
		return nil
	}
	select {
	case h.events <- DiagnosticsEvent{URI: params.URI, Items: params.Diagnostics}:
	default:
		// drop if the host hasn't drained; diagnostics are idempotent.
	}
	return nil
}

func (h *handler) markClosed() {
	h.mu.Lock()
	h.closed = true
	h.mu.Unlock()
}

func (h *handler) Progress(context.Context, *protocol.ProgressParams) error {
	return nil
}
func (h *handler) WorkDoneProgressCreate(context.Context, *protocol.WorkDoneProgressCreateParams) error {
	return nil
}
func (h *handler) LogMessage(context.Context, *protocol.LogMessageParams) error { return nil }
func (h *handler) ShowMessage(context.Context, *protocol.ShowMessageParams) error {
	return nil
}
func (h *handler) ShowMessageRequest(context.Context, *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	return nil, nil
}
func (h *handler) Telemetry(context.Context, interface{}) error { return nil }
func (h *handler) RegisterCapability(context.Context, *protocol.RegistrationParams) error {
	return nil
}
func (h *handler) UnregisterCapability(context.Context, *protocol.UnregistrationParams) error {
	return nil
}
func (h *handler) ApplyEdit(context.Context, *protocol.ApplyWorkspaceEditParams) (bool, error) {
	return false, nil
}
func (h *handler) Configuration(context.Context, *protocol.ConfigurationParams) ([]interface{}, error) {
	return nil, nil
}
func (h *handler) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	return nil, nil
}

// stdio adapts a process's stdin+stdout into an io.ReadWriteCloser for
// jsonrpc2.NewStream. Reads come from stdout; writes go to stdin.
type stdio struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (s stdio) Read(p []byte) (int, error)  { return s.stdout.Read(p) }
func (s stdio) Write(p []byte) (int, error) { return s.stdin.Write(p) }
func (s stdio) Close() error {
	err1 := s.stdin.Close()
	err2 := s.stdout.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
