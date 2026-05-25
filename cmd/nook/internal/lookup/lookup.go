// Package lookup turns LSP textDocument lookups (hover, definition) into
// async tea.Cmd factories. The host model fires a Cmd from a key handler
// and reacts to the matching message a few milliseconds later.
//
// The package owns the request timeout (2s — gopls responses are
// sub-second in the common case) and the nil-client guard, so the host
// doesn't have to branch on "do we have an LSP yet" before binding the
// key.
package lookup

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/cmd/nook/internal/inlayhint"
	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/cmd/nook/internal/semtok"
)

// requestTimeout bounds every lookup. gopls usually answers under 200ms
// for hover/definition on a warm workspace; the cap mostly catches a
// frozen server or a stdio deadlock.
const requestTimeout = 2 * time.Second

// errNoClient is returned in messages when no language server is wired
// up. Hosts treat it as "feature unavailable, surface a status hint."
var errNoClient = errors.New("no language server")

// HoverMsg carries the result of a hover lookup back to the host model.
// Path/Row/Col echo the request inputs so a late response that arrived
// after the user moved on can be discarded by comparing them.
type HoverMsg struct {
	Path string
	Row  int
	Col  int
	Info nooklsp.HoverInfo
	Err  error
}

// DefinitionMsg carries the result of a go-to-definition lookup.
// Locations is empty when the symbol has no known definition (e.g.
// cursor over whitespace or an unresolved import).
type DefinitionMsg struct {
	Path      string
	Row       int
	Col       int
	Locations []nooklsp.Location
	Err       error
}

// CompletionMsg carries the result of a completion lookup. Items is
// empty when the server returned nothing useful. PrefixLen is the
// word-prefix length the host captured at request time so it can be
// fed straight back into complete.Popup.WithItems.
type CompletionMsg struct {
	Path      string
	Row       int
	Col       int
	PrefixLen int
	Items     []nooklsp.CompletionItem
	Err       error
}

// FormattingMsg carries the result of a textDocument/formatting request
// back to the host model. Version echoes the LSP didChange version the
// request was issued against so a stale response (the user typed more
// after Ctrl+S) can be discarded by comparing it. Edits is empty when
// the server thinks the file is already well-formatted.
type FormattingMsg struct {
	Path    string
	Version int32
	Edits   []nooklsp.TextEdit
	Err     error
}

// HoverCmd returns a tea.Cmd that calls client.Hover and wraps the
// result in HoverMsg. A nil client short-circuits to a HoverMsg with
// errNoClient — the host can bind the key unconditionally.
func HoverCmd(client *nooklsp.Client, path string, row, col int) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return HoverMsg{Path: path, Row: row, Col: col, Err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		info, err := client.Hover(ctx, path, row, col)
		return HoverMsg{Path: path, Row: row, Col: col, Info: info, Err: err}
	}
}

// DefinitionCmd returns a tea.Cmd that calls client.Definition and
// wraps the result in DefinitionMsg. nil-client behavior matches
// HoverCmd: the message carries errNoClient instead of panicking.
func DefinitionCmd(client *nooklsp.Client, path string, row, col int) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return DefinitionMsg{Path: path, Row: row, Col: col, Err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		locs, err := client.Definition(ctx, path, row, col)
		return DefinitionMsg{Path: path, Row: row, Col: col, Locations: locs, Err: err}
	}
}

// CompletionCmd returns a tea.Cmd that calls client.Completion and
// wraps the result in CompletionMsg. nil-client returns a message
// carrying errNoClient so the host can bind a trigger key (Ctrl+Space)
// regardless of LSP readiness.
func CompletionCmd(client *nooklsp.Client, path string, row, col, prefixLen int) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return CompletionMsg{Path: path, Row: row, Col: col, PrefixLen: prefixLen, Err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		items, err := client.Completion(ctx, path, row, col)
		return CompletionMsg{Path: path, Row: row, Col: col, PrefixLen: prefixLen, Items: items, Err: err}
	}
}

// formattingTimeout gives the language server a longer leash than the
// other lookups. gofumpt-style formatters can take a second or two on a
// big file the first time around; we cap at 5s so a wedged server
// doesn't freeze the save UX forever.
const formattingTimeout = 5 * time.Second

// FormattingCmd returns a tea.Cmd that calls client.Formatting and wraps
// the result in FormattingMsg. version echoes the LSP didChange version
// the caller knew about at request time so a stale response can be
// detected and ignored. nil-client returns a message carrying errNoClient
// so the host can bind Ctrl+S unconditionally and degrade to a plain save.
func FormattingCmd(client *nooklsp.Client, path string, version int32, tabSize int, insertSpaces bool) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return FormattingMsg{Path: path, Version: version, Err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), formattingTimeout)
		defer cancel()
		edits, err := client.Formatting(ctx, path, tabSize, insertSpaces)
		return FormattingMsg{Path: path, Version: version, Edits: edits, Err: err}
	}
}

// CodeActionMsg carries the result of a textDocument/codeAction request
// back to the host model. Path/Row/Col echo the cursor at request time so
// a stale response (the user moved the cursor) can be discarded by
// comparing them. Items is empty when the server returned nothing.
type CodeActionMsg struct {
	Path  string
	Row   int
	Col   int
	Items []nooklsp.CodeActionItem
	Err   error
}

// PrepareRenameMsg carries the result of a textDocument/prepareRename
// request. Path/Row/Col echo the cursor at request time so a stale
// response can be discarded.
type PrepareRenameMsg struct {
	Path   string
	Row    int
	Col    int
	Result nooklsp.PrepareRenameResult
	Err    error
}

// RenameMsg carries the result of a textDocument/rename request. Path is
// the file the rename was initiated from (useful for sourcing the cursor
// when applying); NewName is the name the user typed (echoed so the host
// can surface "renamed X to Y" status); Edit is the workspace edit to
// apply.
type RenameMsg struct {
	Path    string
	NewName string
	Edit    nooklsp.WorkspaceEditChange
	Err     error
}

// renameTimeout gives gopls a longer leash than hover/def: a rename can
// span the workspace and gopls reads every Go file before answering. The
// 10s cap is generous for a small repo and still bounds a wedged server.
const renameTimeout = 10 * time.Second

// CodeActionCmd returns a tea.Cmd that calls client.CodeAction and wraps
// the result in CodeActionMsg. nil-client behavior mirrors HoverCmd: the
// message carries errNoClient so the host can bind Alt+Enter unconditionally.
func CodeActionCmd(client *nooklsp.Client, path string, row, col int) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return CodeActionMsg{Path: path, Row: row, Col: col, Err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		items, err := client.CodeAction(ctx, path, row, col, row, col)
		return CodeActionMsg{Path: path, Row: row, Col: col, Items: items, Err: err}
	}
}

// PrepareRenameCmd returns a tea.Cmd that calls client.PrepareRename and
// wraps the result in PrepareRenameMsg. nil-client returns errNoClient so
// the host can bind F2 unconditionally and surface a status hint.
func PrepareRenameCmd(client *nooklsp.Client, path string, row, col int) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return PrepareRenameMsg{Path: path, Row: row, Col: col, Err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		res, err := client.PrepareRename(ctx, path, row, col)
		return PrepareRenameMsg{Path: path, Row: row, Col: col, Result: res, Err: err}
	}
}

// RenameCmd returns a tea.Cmd that calls client.Rename and wraps the
// result in RenameMsg. nil-client behavior matches the other lookups.
func RenameCmd(client *nooklsp.Client, path string, row, col int, newName string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return RenameMsg{Path: path, NewName: newName, Err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), renameTimeout)
		defer cancel()
		edit, err := client.Rename(ctx, path, row, col, newName)
		return RenameMsg{Path: path, NewName: newName, Edit: edit, Err: err}
	}
}

// InlayHintMsg carries a batch of inlay hints for a single file back to the
// host. Path is the file the request was pinned to; Version echoes the LSP
// didChange version at request time so a stale response (the user typed
// after the request was sent) can be discarded. Hints is keyed by row.
type InlayHintMsg = inlayhint.HintsMsg

// InlayHintCmd returns a tea.Cmd that calls client.InlayHint over a line
// range and wraps the result in InlayHintMsg. startLine is inclusive,
// endLine is exclusive (LSP semantics; pass the row past the last visible
// line). nil-client returns a message carrying errNoClient so the host can
// trigger fetches unconditionally and degrade gracefully.
func InlayHintCmd(client *nooklsp.Client, path string, version int32, startLine, endLine int) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return InlayHintMsg{Path: path, Version: version, Err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		hints, err := client.InlayHint(ctx, path, startLine, endLine)
		if err != nil {
			return InlayHintMsg{Path: path, Version: version, Err: err}
		}
		return InlayHintMsg{Path: path, Version: version, Hints: inlayhint.ByRow(hints)}
	}
}

// SignatureHelpMsg carries the result of a textDocument/signatureHelp
// request back to the host model. Path/Row/Col echo the request inputs so
// a late response (the user moved on) can be discarded. Info.Signatures is
// empty when the cursor is not inside a call expression.
type SignatureHelpMsg struct {
	Path string
	Row  int
	Col  int
	Info nooklsp.SignatureInfo
	Err  error
}

// SignatureHelpCmd returns a tea.Cmd that calls client.SignatureHelp and
// wraps the result in SignatureHelpMsg. nil-client returns the message
// with errNoClient so the host can bind '(' unconditionally — a buffer
// with no language server attached just silently never opens the overlay.
func SignatureHelpCmd(client *nooklsp.Client, path string, row, col int) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return SignatureHelpMsg{Path: path, Row: row, Col: col, Err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		info, err := client.SignatureHelp(ctx, path, row, col)
		return SignatureHelpMsg{Path: path, Row: row, Col: col, Info: info, Err: err}
	}
}

// ResolveCompletionMsg carries the result of a completionItem/resolve
// request back to the host. The original item is echoed via ReqLabel so
// the host can drop stale responses (the user navigated past the item
// before the server answered).
type ResolveCompletionMsg struct {
	ReqLabel string
	Item     nooklsp.CompletionItem
	Err      error
}

// ResolveCompletionCmd returns a tea.Cmd that calls client.ResolveCompletion
// and wraps the result in ResolveCompletionMsg. nil-client returns errNoClient
// so the host can pump resolve requests on every popup-selection-change
// without branching on LSP readiness.
func ResolveCompletionCmd(client *nooklsp.Client, item nooklsp.CompletionItem) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return ResolveCompletionMsg{ReqLabel: item.Label, Item: item, Err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		resolved, err := client.ResolveCompletion(ctx, item)
		return ResolveCompletionMsg{ReqLabel: item.Label, Item: resolved, Err: err}
	}
}

// SemanticTokensMsg carries a batch of semantic tokens for a single file
// back to the host. Path is the file the request was pinned to; PaneVer
// echoes the editor's buffer-revision counter at request-issue time so the
// editor can drop a stale overlay (the user typed after the request was
// sent). Tokens is nil when the server resolves nothing or doesn't support
// the request.
type SemanticTokensMsg struct {
	Path    string
	PaneVer int
	Tokens  []semtok.Token
	Err     error
}

// SemanticTokensCmd returns a tea.Cmd that calls client.SemanticTokensFull
// and wraps the result in SemanticTokensMsg. paneVer is the value the host
// read from editor.Pane.BufVer() before firing the cmd; the editor uses it
// to discard overlays for already-edited buffer states. nil-client or a
// server that didn't advertise semantic-tokens support returns the message
// with errNoClient so the host can fire the cmd unconditionally and let the
// downstream handler drop the result.
func SemanticTokensCmd(client *nooklsp.Client, path string, paneVer int) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return SemanticTokensMsg{Path: path, PaneVer: paneVer, Err: errNoClient}
		}
		if !client.SemanticTokensSupported() {
			return SemanticTokensMsg{Path: path, PaneVer: paneVer, Err: errNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		tokens, err := client.SemanticTokensFull(ctx, path)
		if err != nil {
			return SemanticTokensMsg{Path: path, PaneVer: paneVer, Err: err}
		}
		return SemanticTokensMsg{Path: path, PaneVer: paneVer, Tokens: tokens}
	}
}
