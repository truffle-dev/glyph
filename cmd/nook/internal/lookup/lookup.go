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
