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

	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
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
