// Package inlayhint defines the on-the-wire representation of LSP inlay
// hints and the message types the host uses to ferry hint results to the
// editor pane.
//
// Inlay hints are short labels that an LSP server (e.g. gopls) wants
// rendered inline at specific positions in a source file. The two
// dominant categories are type hints (": int" after a `:=`) and
// parameter-name hints ("name: " before a call argument). The editor
// paints them in a dim italic style; they are visual-only and do not
// affect the underlying buffer or cursor coordinates.
//
// This package keeps the protocol decoded into Go-native types so the
// editor never imports go.lsp.dev/protocol (which in any case does not
// yet expose inlay hint types at v0.12.0).
package inlayhint

// Kind tags a hint as a type annotation or a parameter name. Matches the
// numeric values in the LSP 3.17 spec; servers may also return zero
// (server didn't categorize), which we treat as KindUnknown.
type Kind int

const (
	KindUnknown   Kind = 0
	KindType      Kind = 1
	KindParameter Kind = 2
)

// Hint is one rendered annotation. Row and Col are 0-based; Col is a byte
// column in the underlying buffer, and the hint renders immediately
// before the rune at that column (LSP semantics: "the character before
// which the hint is inserted").
//
// When Col is at or past end-of-line, the hint renders after the last
// rune on the row.
type Hint struct {
	Row          int
	Col          int
	Label        string
	Kind         Kind
	PaddingLeft  bool
	PaddingRight bool
}

// HintsMsg is the bubbletea message a host receives in response to an
// inlay-hint request. Path is the absolute file path the request was
// pinned to so the host can drop a late response that arrived after the
// buffer was closed or switched.
//
// Hints is keyed by row to keep the editor's render path one map lookup
// per visible row instead of a linear scan of every hint on the page.
// Multiple hints on the same row are stored in document order.
type HintsMsg struct {
	Path    string
	Version int32
	Hints   map[int][]Hint
	Err     error
}

// ByRow groups a flat hint slice by Row. Hints inside each row keep their
// input order (matching the LSP response, which is already document
// order). The returned map is nil iff hints is empty.
func ByRow(hints []Hint) map[int][]Hint {
	if len(hints) == 0 {
		return nil
	}
	m := make(map[int][]Hint, len(hints))
	for _, h := range hints {
		m[h.Row] = append(m[h.Row], h)
	}
	return m
}
