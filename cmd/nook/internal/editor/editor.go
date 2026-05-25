// Package editor is nook's minimal file viewer/editor pane.
//
// Buffer is line-oriented (a []string of lines). Coordinates are
// 0-based internally; the gutter renders 1-based line numbers.
//
// Multi-cursor model: the primary cursor lives at (p.row, p.col) and
// additional cursors in p.extras. Movement keys collapse to the primary;
// edit primitives (insertRunes, insertNewline, delBack, delForward) apply
// at every cursor in ascending position order, shifting later positions
// after each edit. See applyAtAllCursors.
package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/cmd/nook/internal/clip"
	"github.com/truffle-dev/glyph/cmd/nook/internal/dochi"
	"github.com/truffle-dev/glyph/cmd/nook/internal/gitgutter"
	"github.com/truffle-dev/glyph/cmd/nook/internal/highlight"
	"github.com/truffle-dev/glyph/cmd/nook/internal/inlayhint"
	"github.com/truffle-dev/glyph/cmd/nook/internal/inlineblame"
	"github.com/truffle-dev/glyph/cmd/nook/internal/semtok"
	"github.com/truffle-dev/glyph/cmd/nook/internal/snippets"
	"github.com/truffle-dev/glyph/components/theme"
)

// nowFunc is the clock source for blame strip rendering. Overridable from
// tests so the relative-time formatting is deterministic.
var nowFunc = func() time.Time { return time.Now() }

// SavedMsg is emitted after a successful save.
type SavedMsg struct {
	Path string
	Err  error
}

// CancelMsg is emitted on Esc when not editing inline.
type CancelMsg struct{}

// Severity tags the kind of diagnostic associated with a buffer row. It mirrors
// the LSP DiagnosticSeverity enum so the editor doesn't need to depend on the
// protocol package.
type Severity int

const (
	SeverityNone Severity = iota
	SeverityError
	SeverityWarning
	SeverityInfo
	SeverityHint
)

// Buffer is a line-oriented text buffer.
type Buffer struct {
	Lines []string
	Dirty bool
}

// NewBuffer constructs an empty buffer.
func NewBuffer() Buffer { return Buffer{Lines: []string{""}} }

// Load reads a file into a new buffer.
func Load(path string) (Buffer, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Buffer{Lines: []string{""}}, nil
		}
		return Buffer{}, err
	}
	lines := strings.Split(string(body), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return Buffer{Lines: lines}, nil
}

// Save writes the buffer to disk and clears the dirty flag.
func (b Buffer) Save(path string) (Buffer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return b, err
	}
	body := strings.Join(b.Lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return b, err
	}
	b.Dirty = false
	return b, nil
}

// Pane is the editor UI.
type Pane struct {
	theme   theme.Theme
	path    string
	buf     Buffer
	row     int // cursor row, 0-based
	col     int // cursor col, 0-based
	offset  int // first visible row
	width   int
	height  int
	focused bool
	err     error

	// ghostText, when non-empty, renders as muted dim text immediately after
	// the cursor — Cursor-style inline completion. The host owns the lifecycle
	// (see internal/ghost); the editor only renders.
	ghostText string

	// diagnostics maps 0-based row to the worst-severity LSP diagnostic on
	// that row. The host updates it via SetDiagnosticRows whenever the LSP
	// publishes for the open file.
	diagnostics map[int]Severity

	// lineMarkers maps 0-based row to a per-line git-diff marker (added,
	// modified, deleted-above). The host updates it via SetLineMarkers
	// whenever the open file's working-tree state shifts (on open and on
	// save). Painted as the leading character of the marker column so it
	// composes with the diagnostic sigil to its right.
	lineMarkers map[int]gitgutter.Marker

	// breakpointRows maps 0-based row to whether a DAP breakpoint is set
	// on that row. The host pushes a fresh map after each toggle so the
	// gutter renders a red dot in place of the git marker. The git marker
	// is hidden while a breakpoint is set on the same row (BPs are
	// transient; git changes remain visible elsewhere via alt+m).
	breakpointRows map[int]bool

	// stoppedAtRow is the 0-based row where the debugger is currently
	// paused in this file. hasStop disambiguates row 0 from "no stop
	// active." When hasStop is true the gutter renders a yellow ▶ in
	// place of the breakpoint and git markers so the user can see where
	// execution is.
	stoppedAtRow int
	hasStop      bool

	// inlayHints maps 0-based row to the inlay hints rendered on that row.
	// The host updates it via SetInlayHints whenever the LSP responds with
	// a fresh batch for the open file. nil disables hint rendering.
	inlayHints map[int][]inlayhint.Hint

	// blame maps 0-based row to the git-blame entry for that row. The host
	// fills this via SetBlame whenever a file is opened or saved while the
	// inline-blame feature is enabled. blameVisible gates whether the cursor
	// row's entry is rendered as dim italic text after the line; the map is
	// always kept so a toggle doesn't have to re-shell out.
	blame        map[int]inlineblame.Line
	blameVisible bool

	// Syntax-highlight cache. chromaSpans is the chroma layer (the floor);
	// spans is what render reads — chromaSpans alone, or chromaSpans overlaid
	// with semTokens when semBufVer matches the current bufVer. bufVer is a
	// monotonically-increasing counter bumped by any mutation; hlVer records
	// the bufVer chromaSpans was computed from. applyHighlight re-runs chroma
	// only when bufVer != hlVer and re-applies the semantic overlay each pass.
	highlighter highlight.Highlighter
	chromaSpans highlight.Result
	spans       highlight.Result
	bufVer      int
	hlVer       int

	// Semantic-token overlay state. semTokens is the latest LSP semanticTokens
	// response; semBufVer is the bufVer the tokens were minted for. The
	// overlay is applied only while semBufVer == bufVer, so a stale response
	// (the user typed during the LSP roundtrip) is harmlessly ignored. Host
	// fetches Pane.BufVer() before firing lookup.SemanticTokensCmd and passes
	// that value back into WithSemanticTokens when the response arrives.
	semTokens []semtok.Token
	semBufVer int

	// searchMatches paints byte ranges with a background highlight; the host
	// updates it from the finder overlay. searchCurrent is the index of the
	// currently active match within searchMatches (or -1) so the active hit
	// gets a stronger style than the rest.
	searchMatches []Range
	searchCurrent int

	// docHighlights is the latest LSP textDocument/documentHighlight response
	// folded into per-row spans. Each entry is rendered with a subtle
	// background (less prominent than searchMatches) so that "every
	// occurrence of the identifier under my cursor" is visible without
	// drowning out an active find/replace. dochiBufVer pins the response
	// to the bufVer it was minted for so a typed-while-pending change
	// dismisses the stale overlay automatically.
	docHighlights map[int][]dochi.Span
	dochiBufVer   int

	// extras are additional editing cursors beyond the primary (p.row, p.col).
	// Editing operations apply at the primary AND at every extra; movement
	// keys collapse extras to the primary. See applyAtAllCursors.
	extras []extraCursor

	// selecting flags whether a selection is active. When true, the selection
	// extends from anchor (anchorRow, anchorCol) to head (row, col). The
	// anchor is the side of the selection the user did NOT extend; the head
	// is the side the cursor moves with. Shift+arrow extends the head;
	// plain arrow collapses to one edge (Left→start, Right→end, Up/Down to
	// the head before moving). Edit primitives delete the selection first
	// when present. Selections and extra cursors are mutually exclusive —
	// any operation that begins a selection clears extras, and any operation
	// that adds an extra cursor clears the selection.
	selecting bool
	anchorRow int
	anchorCol int

	// tabWidth is the column count a hard tab expands to during rendering.
	// Defaults to 4 (NewPane sets it). On-disk bytes always stay tabs.
	tabWidth int
	// lineNumbers controls whether the gutter prints the 1-based row number.
	// Defaults to true. The marker column (git + diagnostic) renders regardless.
	lineNumbers bool

	// Snippet mode: when snippetMode is true, Tab/Shift+Tab cycle the cursor
	// between snippetStops in declaration order, Esc exits. Any edit key
	// (rune insert, backspace, enter) auto-exits. snippetStopIdx is the
	// position into snippetStops (-1 before the first Tab); snippetFinal,
	// when non-nil, is the $0 cursor target visited after the last stop.
	snippetMode    bool
	snippetStops   []SnippetTabstop
	snippetStopIdx int
	snippetFinal   *SnippetTabstop
}

// extraCursor is an additional editing cursor beyond the primary at
// (Pane.row, Pane.col).
type extraCursor struct {
	Row int
	Col int
}

// CursorPos is an exported (Row, Col) pair used by AllCursorPositions.
type CursorPos struct {
	Row int
	Col int
}

// SnippetTabstop is the (row, col) position of one tabstop placed by
// ExpandSnippet, with the byte length of any default text that lived
// at that position at expand time (used by callers that want to
// preselect the default; the editor itself only places the cursor).
type SnippetTabstop struct {
	Row    int
	Col    int
	Length int
	Index  int
}

// Range is a byte-range mark inside a single row (used for search matches).
type Range struct {
	Row   int
	Start int // 0-based byte column, inclusive
	End   int // 0-based byte column, exclusive
}

// NewPane constructs an empty pane.
func NewPane(t theme.Theme) Pane {
	return Pane{theme: t, buf: NewBuffer(), width: 80, height: 24, searchCurrent: -1, tabWidth: 4, lineNumbers: true}
}

// SetTabWidth sets the rendered tab expansion. Values <= 0 clamp to 4.
// Defaults to 4. Affects rendering only; file bytes still hold tabs.
func (p Pane) SetTabWidth(n int) Pane {
	if n <= 0 {
		n = 4
	}
	p.tabWidth = n
	return p
}

// TabWidth reports the rendered tab expansion.
func (p Pane) TabWidth() int {
	if p.tabWidth <= 0 {
		return 4
	}
	return p.tabWidth
}

// SetLineNumbers toggles the row-number gutter. The marker column (git +
// diagnostic) is not affected.
func (p Pane) SetLineNumbers(b bool) Pane {
	p.lineNumbers = b
	return p
}

// LineNumbers reports whether the row-number gutter is currently visible.
func (p Pane) LineNumbers() bool { return p.lineNumbers }

// WithSearchMatches stores the search matches to paint as background
// highlights and the index of the currently active hit (or -1 for none).
// The host calls this whenever the finder pattern or selection changes.
func (p Pane) WithSearchMatches(matches []Range, current int) Pane {
	p.searchMatches = matches
	p.searchCurrent = current
	return p
}

// ClearSearchMatches removes all highlights.
func (p Pane) ClearSearchMatches() Pane {
	p.searchMatches = nil
	p.searchCurrent = -1
	return p
}

// SearchMatches returns the current highlight set.
func (p Pane) SearchMatches() []Range { return p.searchMatches }

// SearchCurrent returns the active match index.
func (p Pane) SearchCurrent() int { return p.searchCurrent }

// SetDocumentHighlights stores the LSP textDocument/documentHighlight
// response for the pane's current buffer. paneVer should be the
// p.bufVer the request was minted against; if it doesn't match the
// current bufVer the call is a no-op so a typed-while-pending edit
// never lights up stale positions. The host calls this from the
// DocumentHighlightMsg handler after a cursor-settle debounce.
func (p Pane) SetDocumentHighlights(hi []dochi.Highlight, paneVer int) Pane {
	if paneVer != p.bufVer {
		return p
	}
	if len(hi) == 0 {
		p.docHighlights = nil
		p.dochiBufVer = paneVer
		return p
	}
	p.docHighlights = dochi.LineSpans(hi)
	p.dochiBufVer = paneVer
	return p
}

// ClearDocumentHighlights drops the current document-highlight overlay.
// Called when the cursor moves before a refresh fires, when the LSP
// returns an error, or when the buffer mutates.
func (p Pane) ClearDocumentHighlights() Pane {
	p.docHighlights = nil
	return p
}

// DocumentHighlightsVisible reports whether at least one row has a
// painted document-highlight span. Test helper and a fast path for the
// render loop.
func (p Pane) DocumentHighlightsVisible() bool {
	return len(p.docHighlights) > 0
}

// WithHighlighter attaches a syntax Highlighter. Passing nil disables
// highlighting (existing tests rely on the no-highlighter path emitting plain
// text). Highlighting is recomputed lazily when the buffer mutates.
func (p Pane) WithHighlighter(h highlight.Highlighter) Pane {
	p.highlighter = h
	p.hlVer = -1 // force re-run on next applyHighlight
	(&p).applyHighlight()
	return p
}

// WithSize sets pane dimensions.
func (p Pane) WithSize(w, h int) Pane { p.width = w; p.height = h; return p }

// Focused reports whether the pane has keyboard focus.
func (p Pane) Focused() bool { return p.focused }

// Focus sets focused=true.
func (p Pane) Focus() Pane { p.focused = true; return p }

// Blur sets focused=false.
func (p Pane) Blur() Pane { p.focused = false; return p }

// Path returns the file path.
func (p Pane) Path() string { return p.path }

// Dirty reports unsaved changes.
func (p Pane) Dirty() bool { return p.buf.Dirty }

// CursorRow returns the 0-based cursor row.
func (p Pane) CursorRow() int { return p.row }

// CursorCol returns the 0-based cursor column.
func (p Pane) CursorCol() int { return p.col }

// HasSelection reports whether a selection is active. An "active but empty"
// selection (anchor == head) is still reported as a selection so a held
// Shift modifier reads correctly even before the first extension.
func (p Pane) HasSelection() bool { return p.selecting }

// AnchorRow returns the 0-based row of the selection anchor, or the cursor
// row when no selection is active.
func (p Pane) AnchorRow() int {
	if !p.selecting {
		return p.row
	}
	return p.anchorRow
}

// AnchorCol returns the 0-based column of the selection anchor, or the
// cursor column when no selection is active.
func (p Pane) AnchorCol() int {
	if !p.selecting {
		return p.col
	}
	return p.anchorCol
}

// SelectionRange returns the normalized selection bounds (startRow,
// startCol) ≤ (endRow, endCol). Returns the cursor position twice when
// no selection is active. End is exclusive on the row's byte range when
// startRow == endRow; otherwise the end row's first endCol bytes are
// selected.
func (p Pane) SelectionRange() (startRow, startCol, endRow, endCol int) {
	if !p.selecting {
		return p.row, p.col, p.row, p.col
	}
	if p.anchorRow < p.row || (p.anchorRow == p.row && p.anchorCol <= p.col) {
		return p.anchorRow, p.anchorCol, p.row, p.col
	}
	return p.row, p.col, p.anchorRow, p.anchorCol
}

// SelectionText returns the selection's text, joining lines with "\n".
// Returns "" when no selection is active or it is empty.
func (p Pane) SelectionText() string {
	if !p.selecting {
		return ""
	}
	sr, sc, er, ec := p.SelectionRange()
	if sr == er && sc == ec {
		return ""
	}
	if sr == er {
		line := p.buf.Lines[sr]
		if sc < 0 {
			sc = 0
		}
		if ec > len(line) {
			ec = len(line)
		}
		if sc >= ec {
			return ""
		}
		return line[sc:ec]
	}
	parts := make([]string, 0, er-sr+1)
	first := p.buf.Lines[sr]
	if sc < 0 {
		sc = 0
	}
	if sc > len(first) {
		sc = len(first)
	}
	parts = append(parts, first[sc:])
	for r := sr + 1; r < er; r++ {
		parts = append(parts, p.buf.Lines[r])
	}
	last := p.buf.Lines[er]
	if ec > len(last) {
		ec = len(last)
	}
	if ec < 0 {
		ec = 0
	}
	parts = append(parts, last[:ec])
	return strings.Join(parts, "\n")
}

// SelectAll selects the entire buffer: anchor at (0,0), head past the last
// rune of the last line. Clears extra cursors.
func (p Pane) SelectAll() Pane {
	p.extras = nil
	p.anchorRow = 0
	p.anchorCol = 0
	if len(p.buf.Lines) == 0 {
		p.row, p.col = 0, 0
	} else {
		p.row = len(p.buf.Lines) - 1
		p.col = len(p.buf.Lines[p.row])
	}
	p.selecting = true
	(&p).ensureVisible()
	return p
}

// ClearSelection drops any active selection. The cursor stays at the head
// position (the side it was last extended to). Returns the updated Pane.
func (p Pane) ClearSelection() Pane {
	p.selecting = false
	p.anchorRow = p.row
	p.anchorCol = p.col
	return p
}

// CollapseToLeft moves the head to the selection start (the smaller of
// anchor and current cursor) and clears the selection. No-op when no
// selection is active.
func (p Pane) CollapseToLeft() Pane {
	if !p.selecting {
		return p
	}
	sr, sc, _, _ := p.SelectionRange()
	p.row, p.col = sr, sc
	p.selecting = false
	p.anchorRow, p.anchorCol = sr, sc
	(&p).ensureVisible()
	return p
}

// CollapseToRight moves the head to the selection end (the larger of
// anchor and current cursor) and clears the selection. No-op when no
// selection is active.
func (p Pane) CollapseToRight() Pane {
	if !p.selecting {
		return p
	}
	_, _, er, ec := p.SelectionRange()
	p.row, p.col = er, ec
	p.selecting = false
	p.anchorRow, p.anchorCol = er, ec
	(&p).ensureVisible()
	return p
}

// DeleteSelection removes the currently-selected range from the buffer and
// places the cursor at the start of what was selected. Marks the buffer
// dirty and clears the selection. Returns the updated Pane unchanged when
// no selection is active.
func (p Pane) DeleteSelection() Pane {
	if !p.selecting {
		return p
	}
	sr, sc, er, ec := p.SelectionRange()
	if sr == er && sc == ec {
		p.selecting = false
		return p
	}
	if sr == er {
		line := p.buf.Lines[sr]
		if sc < 0 {
			sc = 0
		}
		if ec > len(line) {
			ec = len(line)
		}
		p.buf.Lines[sr] = line[:sc] + line[ec:]
	} else {
		first := p.buf.Lines[sr]
		last := p.buf.Lines[er]
		if sc < 0 {
			sc = 0
		}
		if sc > len(first) {
			sc = len(first)
		}
		if ec < 0 {
			ec = 0
		}
		if ec > len(last) {
			ec = len(last)
		}
		merged := first[:sc] + last[ec:]
		p.buf.Lines[sr] = merged
		p.buf.Lines = append(p.buf.Lines[:sr+1], p.buf.Lines[er+1:]...)
	}
	p.row, p.col = sr, sc
	p.anchorRow, p.anchorCol = sr, sc
	p.selecting = false
	p.extras = nil
	p.buf.Dirty = true
	p.bufVer++
	(&p).applyHighlight()
	(&p).ensureVisible()
	return p
}

// startSelectionIfNeeded marks the current cursor as the anchor when no
// selection is active. Called by every Shift+motion handler before moving
// the head.
func (p *Pane) startSelectionIfNeeded() {
	if !p.selecting {
		p.anchorRow = p.row
		p.anchorCol = p.col
		p.selecting = true
		p.extras = nil
	}
}

// LineCount returns total lines.
func (p Pane) LineCount() int { return len(p.buf.Lines) }

// Line returns the content of a 0-based row.
func (p Pane) Line(row int) string {
	if row < 0 || row >= len(p.buf.Lines) {
		return ""
	}
	return p.buf.Lines[row]
}

// Lines returns the full buffer as a slice of lines. Callers must treat the
// result as read-only — mutations would corrupt the editor's syntax cache
// and dirty flag.
func (p Pane) Lines() []string { return p.buf.Lines }

// Open replaces the buffer with the contents of path.
func (p Pane) Open(path string) Pane {
	b, err := Load(path)
	p.buf = b
	p.path = path
	p.row, p.col, p.offset = 0, 0, 0
	p.anchorRow, p.anchorCol = 0, 0
	p.selecting = false
	p.extras = nil
	p.err = err
	p.bufVer++
	(&p).applyHighlight()
	return p
}

// JumpTo moves the cursor to a 1-based (line, col) location and scrolls.
func (p Pane) JumpTo(line, col int) Pane {
	p.row = line - 1
	p.col = col - 1
	if p.row < 0 {
		p.row = 0
	}
	if p.row >= len(p.buf.Lines) {
		p.row = len(p.buf.Lines) - 1
	}
	max := len(p.buf.Lines[p.row])
	if p.col < 0 {
		p.col = 0
	}
	if p.col > max {
		p.col = max
	}
	p.anchorRow, p.anchorCol = p.row, p.col
	p.selecting = false
	p.ensureVisible()
	return p
}

// SetLine replaces the contents of a 0-based row with newText. The replacement
// preserves the original leading indentation of the row so AI edits don't
// silently flatten indentation when the model forgets to.
func (p Pane) SetLine(row int, newText string) Pane {
	if row < 0 {
		return p
	}
	for row >= len(p.buf.Lines) {
		p.buf.Lines = append(p.buf.Lines, "")
	}
	orig := p.buf.Lines[row]
	indent := leadingIndent(orig)
	// If newText already begins with whitespace, trust the model; otherwise
	// re-attach the original indent.
	if !startsWithWhitespace(newText) && indent != "" {
		newText = indent + newText
	}
	p.buf.Lines[row] = newText
	p.buf.Dirty = true
	if p.col > len(newText) {
		p.col = len(newText)
	}
	p.bufVer++
	(&p).applyHighlight()
	return p
}

// ReplaceAllFromString resets the buffer to the given full file contents.
// Used by the composer to apply a full-file edit to the currently-open file
// without re-reading from disk.
func (p Pane) ReplaceAllFromString(contents string) Pane {
	lines := splitLines(contents)
	if len(lines) == 0 {
		lines = []string{""}
	}
	p.buf.Lines = lines
	p.buf.Dirty = true
	if p.row >= len(lines) {
		p.row = len(lines) - 1
	}
	if p.col > len(lines[p.row]) {
		p.col = len(lines[p.row])
	}
	p.bufVer++
	(&p).applyHighlight()
	return p
}

// Contents returns the full buffer as a single string with \n separators.
// Used by the composer to ground the model on the current file state.
func (p Pane) Contents() string {
	return strings.Join(p.buf.Lines, "\n")
}

// LinePrefix returns the text of the cursor row up to (but not including) the
// cursor column. Used by the ghost-text manager to ground completions.
func (p Pane) LinePrefix() string {
	if p.row < 0 || p.row >= len(p.buf.Lines) {
		return ""
	}
	line := p.buf.Lines[p.row]
	if p.col < 0 {
		return ""
	}
	if p.col > len(line) {
		return line
	}
	return line[:p.col]
}

// LineSuffix returns the text of the cursor row after the cursor column.
// Used by the ghost-text manager to detect when a ghost completion would
// produce a duplicate of what's already after the cursor.
func (p Pane) LineSuffix() string {
	if p.row < 0 || p.row >= len(p.buf.Lines) {
		return ""
	}
	line := p.buf.Lines[p.row]
	if p.col < 0 {
		return line
	}
	if p.col > len(line) {
		return ""
	}
	return line[p.col:]
}

// SetGhostText sets the rendered ghost-text proposal. Empty string clears it.
func (p Pane) SetGhostText(s string) Pane {
	p.ghostText = s
	return p
}

// GhostText returns the current ghost-text proposal.
func (p Pane) GhostText() string {
	return p.ghostText
}

// SetDiagnosticRows replaces the diagnostic-row map. nil clears it. The map
// is keyed by 0-based row and values name the worst severity on the row.
func (p Pane) SetDiagnosticRows(rows map[int]Severity) Pane {
	p.diagnostics = rows
	return p
}

// DiagnosticAt returns the severity for a 0-based row, or SeverityNone.
func (p Pane) DiagnosticAt(row int) Severity {
	if p.diagnostics == nil {
		return SeverityNone
	}
	return p.diagnostics[row]
}

// SetLineMarkers replaces the per-line git-diff marker map. nil clears it. The
// map is keyed by 0-based row.
func (p Pane) SetLineMarkers(rows map[int]gitgutter.Marker) Pane {
	p.lineMarkers = rows
	return p
}

// LineMarkerAt returns the git-diff marker for a 0-based row, or gitgutter.None.
func (p Pane) LineMarkerAt(row int) gitgutter.Marker {
	if p.lineMarkers == nil {
		return gitgutter.None
	}
	return p.lineMarkers[row]
}

// SetBreakpointRows replaces the per-line breakpoint map. nil clears it.
// The map is keyed by 0-based row; only rows present with true value
// render a breakpoint marker.
func (p Pane) SetBreakpointRows(rows map[int]bool) Pane {
	p.breakpointRows = rows
	return p
}

// IsBreakpoint reports whether a DAP breakpoint is set on a 0-based row.
func (p Pane) IsBreakpoint(row int) bool {
	if p.breakpointRows == nil {
		return false
	}
	return p.breakpointRows[row]
}

// SetStoppedAtRow marks the 0-based row where the debugger is currently
// paused in this file. Pass -1 to clear the indicator.
func (p Pane) SetStoppedAtRow(row int) Pane {
	if row < 0 {
		p.stoppedAtRow = 0
		p.hasStop = false
	} else {
		p.stoppedAtRow = row
		p.hasStop = true
	}
	return p
}

// StoppedAtRow returns the 0-based row where execution is paused in this
// file, or -1 when no stop is active.
func (p Pane) StoppedAtRow() int {
	if !p.hasStop {
		return -1
	}
	return p.stoppedAtRow
}

// SetInlayHints replaces the per-row inlay hint map. nil clears it. The map
// is keyed by 0-based row; each entry is the slice of hints rendered on
// that row in document order.
func (p Pane) SetInlayHints(rows map[int][]inlayhint.Hint) Pane {
	p.inlayHints = rows
	return p
}

// InlayHintsAt returns the inlay hints rendered on a 0-based row, or nil.
func (p Pane) InlayHintsAt(row int) []inlayhint.Hint {
	if p.inlayHints == nil {
		return nil
	}
	return p.inlayHints[row]
}

// SetBlame replaces the per-row git-blame map. nil clears it. The map is
// keyed by 0-based row. Rendering is gated separately by SetBlameVisible
// so the host can toggle visibility without re-fetching.
func (p Pane) SetBlame(rows map[int]inlineblame.Line) Pane {
	p.blame = rows
	return p
}

// SetBlameVisible controls whether the cursor row's blame entry is painted
// after the line. False renders nothing even when the blame map is populated.
func (p Pane) SetBlameVisible(b bool) Pane {
	p.blameVisible = b
	return p
}

// BlameVisible reports whether inline blame is currently rendered.
func (p Pane) BlameVisible() bool { return p.blameVisible }

// BlameAt returns the blame entry for a 0-based row and whether one exists.
func (p Pane) BlameAt(row int) (inlineblame.Line, bool) {
	if p.blame == nil {
		return inlineblame.Line{}, false
	}
	v, ok := p.blame[row]
	return v, ok
}

// InsertText inserts s at the cursor, advancing the cursor. Newlines split the
// current line; the cursor ends just past the last inserted rune. Marks dirty.
func (p Pane) InsertText(s string) Pane {
	if s == "" {
		return p
	}
	for p.row >= len(p.buf.Lines) {
		p.buf.Lines = append(p.buf.Lines, "")
	}
	parts := strings.Split(s, "\n")
	if len(parts) == 1 {
		(&p).insertRunes([]rune(parts[0]))
		(&p).applyHighlight()
		return p
	}
	// Multi-line: insert head into current row, then splice tail rows.
	(&p).insertRunes([]rune(parts[0]))
	for _, line := range parts[1 : len(parts)-1] {
		(&p).insertNewline()
		(&p).insertRunes([]rune(line))
	}
	last := parts[len(parts)-1]
	(&p).insertNewline()
	(&p).insertRunes([]rune(last))
	(&p).applyHighlight()
	return p
}

// DeleteRange removes the inclusive byte range [startCol, endCol) on the
// current cursor row. Used by snippet expansion to remove the prefix the
// user typed before splicing the body in. Marks dirty.
func (p Pane) DeleteRange(row, startCol, endCol int) Pane {
	if row < 0 || row >= len(p.buf.Lines) {
		return p
	}
	line := p.buf.Lines[row]
	if startCol < 0 {
		startCol = 0
	}
	if endCol > len(line) {
		endCol = len(line)
	}
	if startCol >= endCol {
		return p
	}
	p.buf.Lines[row] = line[:startCol] + line[endCol:]
	p.buf.Dirty = true
	if p.row == row && p.col > startCol {
		if p.col >= endCol {
			p.col -= endCol - startCol
		} else {
			p.col = startCol
		}
	}
	p.bufVer++
	(&p).applyHighlight()
	return p
}

// ExpandSnippet replaces the prefix [prefixStart, p.col) on the cursor row
// with exp.Text and enters snippet mode. Tabstops in exp are converted from
// byte offsets in exp.Text into (row, col) buffer coordinates; the cursor
// lands on the first tabstop (or the final tabstop, or end of insertion).
// If exp has no tabstops and no final, snippet mode is not entered.
func (p Pane) ExpandSnippet(prefixStart int, exp snippets.Expansion) Pane {
	if prefixStart < 0 {
		prefixStart = 0
	}
	if prefixStart > p.col {
		prefixStart = p.col
	}
	// Delete the typed prefix, then insert the body at the cursor.
	p = p.DeleteRange(p.row, prefixStart, p.col)
	startRow, startCol := p.row, p.col
	p = p.InsertText(exp.Text)

	convert := func(off int) (int, int) {
		// Count newlines before off; column is bytes since last newline,
		// or startCol + off when on the start row.
		row := startRow
		col := startCol
		last := 0
		for i := 0; i < off; i++ {
			if exp.Text[i] == '\n' {
				row++
				last = i + 1
			}
		}
		if row == startRow {
			col = startCol + off
		} else {
			col = off - last
		}
		return row, col
	}

	stops := make([]SnippetTabstop, 0, len(exp.Tabstops))
	for _, t := range exp.Tabstops {
		r, c := convert(t.Offset)
		stops = append(stops, SnippetTabstop{Row: r, Col: c, Length: t.Length, Index: t.Index})
	}
	var final *SnippetTabstop
	if exp.Final != nil {
		r, c := convert(exp.Final.Offset)
		final = &SnippetTabstop{Row: r, Col: c, Length: exp.Final.Length, Index: exp.Final.Index}
	}

	p.snippetStops = stops
	p.snippetFinal = final
	p.snippetStopIdx = -1

	switch {
	case len(stops) > 0:
		p.snippetMode = true
		p.snippetStopIdx = 0
		p.row = stops[0].Row
		p.col = stops[0].Col
	case final != nil:
		p.row = final.Row
		p.col = final.Col
	}
	p.extras = nil
	p.ensureVisible()
	return p
}

// SnippetNext advances to the next tabstop, or to the final $0 target after
// the last, or exits snippet mode if there is no final.
func (p Pane) SnippetNext() Pane {
	if !p.snippetMode {
		return p
	}
	next := p.snippetStopIdx + 1
	if next < len(p.snippetStops) {
		p.snippetStopIdx = next
		p.row = p.snippetStops[next].Row
		p.col = p.snippetStops[next].Col
		p.ensureVisible()
		return p
	}
	if p.snippetFinal != nil {
		p.row = p.snippetFinal.Row
		p.col = p.snippetFinal.Col
		p.ensureVisible()
	}
	return p.SnippetExit()
}

// SnippetPrev returns to the previous tabstop. At the first stop it stays.
func (p Pane) SnippetPrev() Pane {
	if !p.snippetMode {
		return p
	}
	prev := p.snippetStopIdx - 1
	if prev < 0 {
		prev = 0
	}
	p.snippetStopIdx = prev
	p.row = p.snippetStops[prev].Row
	p.col = p.snippetStops[prev].Col
	p.ensureVisible()
	return p
}

// SnippetExit leaves snippet mode and clears the recorded stops.
func (p Pane) SnippetExit() Pane {
	p.snippetMode = false
	p.snippetStops = nil
	p.snippetFinal = nil
	p.snippetStopIdx = -1
	return p
}

// InSnippetMode reports whether the pane is currently navigating tabstops.
func (p Pane) InSnippetMode() bool { return p.snippetMode }

// CurrentSnippetTabstop returns the tabstop the cursor is currently on (when
// in snippet mode and at least one stop exists). The bool is false otherwise.
func (p Pane) CurrentSnippetTabstop() (SnippetTabstop, bool) {
	if !p.snippetMode {
		return SnippetTabstop{}, false
	}
	if p.snippetStopIdx < 0 || p.snippetStopIdx >= len(p.snippetStops) {
		return SnippetTabstop{}, false
	}
	return p.snippetStops[p.snippetStopIdx], true
}

func leadingIndent(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
}

func startsWithWhitespace(s string) bool {
	return s != "" && (s[0] == ' ' || s[0] == '\t')
}

func splitLines(s string) []string {
	// strings.Split keeps trailing empty on "\n"-terminated files; we want
	// the buffer to drop that empty line for symmetry with Load().
	if s == "" {
		return []string{""}
	}
	lines := strings.Split(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// SaveCmd returns a tea.Cmd that writes the buffer to disk.
func (p Pane) SaveCmd() tea.Cmd {
	path := p.path
	buf := p.buf
	return func() tea.Msg {
		if path == "" {
			return SavedMsg{Err: fmt.Errorf("no path set")}
		}
		_, err := buf.Save(path)
		return SavedMsg{Path: path, Err: err}
	}
}

// ApplySave marks the buffer clean after a successful SavedMsg.
func (p Pane) ApplySave() Pane {
	p.buf.Dirty = false
	return p
}

func (p *Pane) ensureVisible() {
	p.scrollToShow(p.row)
}

// scrollToShow adjusts p.offset so that the given row is within the visible
// window, used by multi-cursor add operations to follow the newest cursor.
func (p *Pane) scrollToShow(row int) {
	visible := p.height - 1
	if visible < 1 {
		visible = 1
	}
	if row < p.offset {
		p.offset = row
	}
	if row >= p.offset+visible {
		p.offset = row - visible + 1
	}
}

// ExtraCursorCount returns the number of editing cursors beyond the primary.
func (p Pane) ExtraCursorCount() int { return len(p.extras) }

// AllCursorPositions returns the primary cursor followed by extras in the
// order they were added. Sorted by position is NOT guaranteed; use this for
// tests and inspection, not for edit logic.
func (p Pane) AllCursorPositions() []CursorPos {
	out := make([]CursorPos, 0, len(p.extras)+1)
	out = append(out, CursorPos{Row: p.row, Col: p.col})
	for _, e := range p.extras {
		out = append(out, CursorPos{Row: e.Row, Col: e.Col})
	}
	return out
}

// ClearExtraCursors removes every cursor beyond the primary.
func (p Pane) ClearExtraCursors() Pane {
	p.extras = nil
	return p
}

// AddCursorBelow appends an extra cursor on the row below the bottommost
// existing cursor at the primary cursor's column. No-op when the bottommost
// cursor is already on the last line.
func (p Pane) AddCursorBelow() Pane {
	maxRow := p.row
	for _, e := range p.extras {
		if e.Row > maxRow {
			maxRow = e.Row
		}
	}
	if maxRow >= len(p.buf.Lines)-1 {
		return p
	}
	target := maxRow + 1
	col := p.col
	if max := len(p.buf.Lines[target]); col > max {
		col = max
	}
	p.extras = append(p.extras, extraCursor{Row: target, Col: col})
	(&p).dedupCursors()
	(&p).scrollToShow(target)
	return p
}

// AddCursorAbove appends an extra cursor on the row above the topmost existing
// cursor at the primary cursor's column. No-op at the top of the file.
func (p Pane) AddCursorAbove() Pane {
	minRow := p.row
	for _, e := range p.extras {
		if e.Row < minRow {
			minRow = e.Row
		}
	}
	if minRow <= 0 {
		return p
	}
	target := minRow - 1
	col := p.col
	if max := len(p.buf.Lines[target]); col > max {
		col = max
	}
	p.extras = append(p.extras, extraCursor{Row: target, Col: col})
	(&p).dedupCursors()
	(&p).scrollToShow(target)
	return p
}

// AddNextMatchCursor finds the next whole-word occurrence of the word under
// the primary cursor (searching forward from the latest existing cursor and
// wrapping to the top of the buffer if needed) and adds an extra cursor at
// the end of that match. No-op when there is no word at the primary cursor
// or no other occurrence in the buffer.
func (p Pane) AddNextMatchCursor() Pane {
	if p.row < 0 || p.row >= len(p.buf.Lines) {
		return p
	}
	word := wordAt(p.buf.Lines[p.row], p.col)
	if word == "" {
		return p
	}
	latestRow, latestCol := p.row, p.col
	for _, e := range p.extras {
		if e.Row > latestRow || (e.Row == latestRow && e.Col > latestCol) {
			latestRow, latestCol = e.Row, e.Col
		}
	}
	add := func(r, c int) Pane {
		p.extras = append(p.extras, extraCursor{Row: r, Col: c})
		(&p).dedupCursors()
		(&p).scrollToShow(r)
		return p
	}
	// Search forward from the latest cursor.
	for r := latestRow; r < len(p.buf.Lines); r++ {
		off := 0
		if r == latestRow {
			off = latestCol
		}
		if idx := indexWholeWord(p.buf.Lines[r], word, off); idx >= 0 {
			return add(r, idx+len(word))
		}
	}
	// Wrap.
	for r := 0; r < latestRow; r++ {
		if idx := indexWholeWord(p.buf.Lines[r], word, 0); idx >= 0 {
			return add(r, idx+len(word))
		}
	}
	// And the head of latestRow up to latestCol.
	if latestCol > 0 {
		if idx := indexWholeWord(p.buf.Lines[latestRow][:latestCol], word, 0); idx >= 0 {
			return add(latestRow, idx+len(word))
		}
	}
	return p
}

// allCursorsSorted returns every cursor position (primary + extras) sorted by
// (row, col) ascending. The parallel idxMap maps back to -1 for the primary
// or extras index for each extra.
func (p *Pane) allCursorsSorted() (positions []CursorPos, idxMap []int) {
	type entry struct {
		pos CursorPos
		idx int
	}
	entries := []entry{{CursorPos{Row: p.row, Col: p.col}, -1}}
	for i, e := range p.extras {
		entries = append(entries, entry{CursorPos{Row: e.Row, Col: e.Col}, i})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].pos.Row != entries[j].pos.Row {
			return entries[i].pos.Row < entries[j].pos.Row
		}
		return entries[i].pos.Col < entries[j].pos.Col
	})
	positions = make([]CursorPos, len(entries))
	idxMap = make([]int, len(entries))
	for i, e := range entries {
		positions[i] = e.pos
		idxMap[i] = e.idx
	}
	return positions, idxMap
}

// setCursorByIdx writes pos back to either the primary cursor (idx==-1) or
// the extra at p.extras[idx].
func (p *Pane) setCursorByIdx(idx int, pos CursorPos) {
	if idx == -1 {
		p.row = pos.Row
		p.col = pos.Col
		return
	}
	p.extras[idx].Row = pos.Row
	p.extras[idx].Col = pos.Col
}

// dedupCursors removes any extra cursor that coincides with the primary or
// with a preceding extra. Preserves insertion order of the survivors.
func (p *Pane) dedupCursors() {
	if len(p.extras) == 0 {
		return
	}
	type key struct{ Row, Col int }
	seen := map[key]bool{{Row: p.row, Col: p.col}: true}
	kept := p.extras[:0]
	for _, e := range p.extras {
		k := key{Row: e.Row, Col: e.Col}
		if seen[k] {
			continue
		}
		seen[k] = true
		kept = append(kept, e)
	}
	p.extras = kept
}

// applyAtAllCursors processes every cursor in ascending (row, col) order. For
// each, edit performs the edit at that cursor's CURRENT position and returns
// the new position of that cursor plus a shift function for any later
// position. After all edits, cursors are deduped and the buffer version is
// bumped. The buffer's Dirty flag is set unconditionally — callers that want
// a no-op (e.g. backspace at start of file) should still return their own
// position and an identity shift.
func (p *Pane) applyAtAllCursors(edit func(row, col int) (newPos CursorPos, shift func(CursorPos) CursorPos)) {
	p.dedupCursors()
	positions, idxMap := p.allCursorsSorted()
	for i := 0; i < len(positions); i++ {
		newPos, shift := edit(positions[i].Row, positions[i].Col)
		positions[i] = newPos
		for j := i + 1; j < len(positions); j++ {
			positions[j] = shift(positions[j])
		}
	}
	for i, pos := range positions {
		p.setCursorByIdx(idxMap[i], pos)
	}
	p.dedupCursors()
	p.buf.Dirty = true
	p.bufVer++
}

func shiftAfterInsertRunes(c CursorPos, atRow, atCol, n int) CursorPos {
	if c.Row != atRow || c.Col < atCol {
		return c
	}
	return CursorPos{Row: c.Row, Col: c.Col + n}
}

func shiftAfterDeleteChar(c CursorPos, atRow, atCol int) CursorPos {
	if c.Row != atRow || c.Col <= atCol {
		return c
	}
	return CursorPos{Row: c.Row, Col: c.Col - 1}
}

func shiftAfterInsertNewline(c CursorPos, atRow, atCol int) CursorPos {
	if c.Row < atRow {
		return c
	}
	if c.Row > atRow {
		return CursorPos{Row: c.Row + 1, Col: c.Col}
	}
	if c.Col < atCol {
		return c
	}
	return CursorPos{Row: c.Row + 1, Col: c.Col - atCol}
}

func shiftAfterMergeWithAbove(c CursorPos, row, oldPrevLen int) CursorPos {
	if c.Row < row {
		return c
	}
	if c.Row == row {
		return CursorPos{Row: row - 1, Col: oldPrevLen + c.Col}
	}
	return CursorPos{Row: c.Row - 1, Col: c.Col}
}

func shiftAfterMergeWithBelow(c CursorPos, row, oldRowLen int) CursorPos {
	if c.Row <= row {
		return c
	}
	if c.Row == row+1 {
		return CursorPos{Row: row, Col: oldRowLen + c.Col}
	}
	return CursorPos{Row: c.Row - 1, Col: c.Col}
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// wordAt returns the identifier ([A-Za-z0-9_]+) surrounding col in line, or
// "" if col is not on or adjacent to one.
func wordAt(line string, col int) string {
	if col < 0 {
		col = 0
	}
	if col > len(line) {
		col = len(line)
	}
	l := col
	for l > 0 && isWordChar(line[l-1]) {
		l--
	}
	r := col
	for r < len(line) && isWordChar(line[r]) {
		r++
	}
	if l == r {
		return ""
	}
	return line[l:r]
}

// indexWholeWord searches s for word as a whole identifier (boundaries are
// either string ends or non-word chars), starting at byte offset off.
// Returns the byte index of the match, or -1.
func indexWholeWord(s, word string, off int) int {
	if word == "" || len(word) > len(s)-off {
		// even if len(s)-off is negative; handled by loop bound below
	}
	for i := off; i+len(word) <= len(s); i++ {
		if s[i:i+len(word)] != word {
			continue
		}
		if i > 0 && isWordChar(s[i-1]) {
			continue
		}
		if i+len(word) < len(s) && isWordChar(s[i+len(word)]) {
			continue
		}
		return i
	}
	return -1
}

// Update handles keys.
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	// Snippet mode owns Tab, Shift+Tab, and Esc. Any other key auto-exits
	// the mode and falls through to normal handling so the user can keep
	// typing without an explicit Esc.
	if p.snippetMode {
		switch km.Type {
		case tea.KeyTab:
			return p.SnippetNext(), nil
		case tea.KeyShiftTab:
			return p.SnippetPrev(), nil
		case tea.KeyEsc:
			return p.SnippetExit(), nil
		default:
			p = p.SnippetExit()
		}
	}
	switch km.Type {
	case tea.KeyEsc:
		if p.selecting {
			p.selecting = false
			p.anchorRow, p.anchorCol = p.row, p.col
			return p, nil
		}
		if len(p.extras) > 0 {
			p.extras = nil
			return p, nil
		}
		return p, func() tea.Msg { return CancelMsg{} }
	case tea.KeyCtrlS:
		return p, p.SaveCmd()
	case tea.KeyCtrlA:
		// Select-all replaces the prior Home-binding on Ctrl+A. Home still
		// works on its dedicated key.
		return p.SelectAll(), nil
	case tea.KeyCtrlC:
		// VSCode default: with selection, copy selection text; with no
		// selection, copy the current line including its trailing newline so
		// a paste re-inserts the whole line above wherever the cursor lands.
		if p.selecting {
			clip.Set(p.SelectionText())
		} else {
			clip.Set(p.buf.Lines[p.row] + "\n")
		}
		return p, nil
	case tea.KeyCtrlX:
		// VSCode default: with selection, cut it; with no selection, cut the
		// current line (the line plus newline goes to the clipboard, and the
		// line is removed from the buffer).
		if p.selecting {
			clip.Set(p.SelectionText())
			p = p.DeleteSelection()
		} else {
			clip.Set(p.buf.Lines[p.row] + "\n")
			(&p).deleteCurrentLine()
		}
	case tea.KeyCtrlV:
		// Paste at the head. An active selection is replaced. Multi-cursor
		// extras are cleared first so the splice points stay coherent.
		text := clip.Get()
		if text == "" {
			return p, nil
		}
		p.extras = nil
		if p.selecting {
			p = p.DeleteSelection()
		}
		p = p.InsertText(text)
	case tea.KeyCtrlD:
		// Add a cursor at the end of the next whole-word match of the word
		// under the primary cursor.
		p = p.AddNextMatchCursor()
	case tea.KeyCtrlUp:
		// Stack a cursor on the row above the topmost cursor.
		p = p.AddCursorAbove()
	case tea.KeyCtrlDown:
		// Stack a cursor on the row below the bottommost cursor.
		p = p.AddCursorBelow()
	case tea.KeyShiftUp:
		(&p).startSelectionIfNeeded()
		if p.row > 0 {
			p.row--
			p.clampCol()
		}
		p.ensureVisible()
	case tea.KeyShiftDown:
		(&p).startSelectionIfNeeded()
		if p.row < len(p.buf.Lines)-1 {
			p.row++
			p.clampCol()
		}
		p.ensureVisible()
	case tea.KeyShiftLeft:
		(&p).startSelectionIfNeeded()
		if p.col > 0 {
			p.col--
		} else if p.row > 0 {
			p.row--
			p.col = len(p.buf.Lines[p.row])
			p.ensureVisible()
		}
	case tea.KeyShiftRight:
		(&p).startSelectionIfNeeded()
		if p.col < len(p.buf.Lines[p.row]) {
			p.col++
		} else if p.row < len(p.buf.Lines)-1 {
			p.row++
			p.col = 0
			p.ensureVisible()
		}
	case tea.KeyShiftHome:
		(&p).startSelectionIfNeeded()
		p.col = 0
	case tea.KeyShiftEnd:
		(&p).startSelectionIfNeeded()
		p.col = len(p.buf.Lines[p.row])
	case tea.KeyUp:
		p.extras = nil
		p.selecting = false
		if p.row > 0 {
			p.row--
			p.clampCol()
		}
		p.anchorRow, p.anchorCol = p.row, p.col
		p.ensureVisible()
	case tea.KeyDown:
		p.extras = nil
		p.selecting = false
		if p.row < len(p.buf.Lines)-1 {
			p.row++
			p.clampCol()
		}
		p.anchorRow, p.anchorCol = p.row, p.col
		p.ensureVisible()
	case tea.KeyLeft:
		p.extras = nil
		if p.selecting {
			// Collapse to selection start; no further movement.
			sr, sc, _, _ := p.SelectionRange()
			p.row, p.col = sr, sc
			p.selecting = false
			p.anchorRow, p.anchorCol = p.row, p.col
			p.ensureVisible()
			break
		}
		if p.col > 0 {
			p.col--
		} else if p.row > 0 {
			p.row--
			p.col = len(p.buf.Lines[p.row])
			p.ensureVisible()
		}
		p.anchorRow, p.anchorCol = p.row, p.col
	case tea.KeyRight:
		p.extras = nil
		if p.selecting {
			_, _, er, ec := p.SelectionRange()
			p.row, p.col = er, ec
			p.selecting = false
			p.anchorRow, p.anchorCol = p.row, p.col
			p.ensureVisible()
			break
		}
		if p.col < len(p.buf.Lines[p.row]) {
			p.col++
		} else if p.row < len(p.buf.Lines)-1 {
			p.row++
			p.col = 0
			p.ensureVisible()
		}
		p.anchorRow, p.anchorCol = p.row, p.col
	case tea.KeyHome:
		p.extras = nil
		p.selecting = false
		p.col = 0
		p.anchorRow, p.anchorCol = p.row, p.col
	case tea.KeyEnd, tea.KeyCtrlE:
		p.extras = nil
		p.selecting = false
		p.col = len(p.buf.Lines[p.row])
		p.anchorRow, p.anchorCol = p.row, p.col
	case tea.KeyPgUp:
		p.extras = nil
		p.selecting = false
		jump := p.height - 2
		if jump < 1 {
			jump = 1
		}
		p.row -= jump
		if p.row < 0 {
			p.row = 0
		}
		p.clampCol()
		p.anchorRow, p.anchorCol = p.row, p.col
		p.ensureVisible()
	case tea.KeyPgDown:
		p.extras = nil
		p.selecting = false
		jump := p.height - 2
		if jump < 1 {
			jump = 1
		}
		p.row += jump
		if p.row >= len(p.buf.Lines) {
			p.row = len(p.buf.Lines) - 1
		}
		p.clampCol()
		p.anchorRow, p.anchorCol = p.row, p.col
		p.ensureVisible()
	case tea.KeyBackspace:
		if p.selecting {
			p = p.DeleteSelection()
		} else if len(p.extras) > 0 {
			(&p).backspaceAllCursors()
		} else {
			p.delBack()
		}
	case tea.KeyDelete:
		if p.selecting {
			p = p.DeleteSelection()
		} else if len(p.extras) > 0 {
			(&p).delForwardAllCursors()
		} else {
			p.delForward()
		}
	case tea.KeyEnter:
		if p.selecting {
			p = p.DeleteSelection()
			p.insertNewline()
		} else if len(p.extras) > 0 {
			(&p).newlineAllCursors()
		} else {
			p.insertNewline()
		}
	case tea.KeyTab:
		if p.selecting {
			p = p.DeleteSelection()
			p.insertRunes([]rune{'\t'})
		} else if len(p.extras) > 0 {
			(&p).insertRunesAllCursors([]rune{'\t'})
		} else {
			p.insertRunes([]rune{'\t'})
		}
	case tea.KeySpace:
		if p.selecting {
			p = p.DeleteSelection()
			p.insertRunes([]rune{' '})
		} else if len(p.extras) > 0 {
			(&p).insertRunesAllCursors([]rune{' '})
		} else {
			p.insertRunes([]rune{' '})
		}
	case tea.KeyRunes:
		if p.selecting {
			p = p.DeleteSelection()
			p.insertRunes(km.Runes)
		} else if len(p.extras) > 0 {
			(&p).insertRunesAllCursors(km.Runes)
		} else {
			p.insertRunes(km.Runes)
		}
	}
	(&p).applyHighlight()
	return p, nil
}

// applyHighlight re-runs the highlighter when the buffer version has advanced
// past the last cached version. Called from any path that mutates the buffer.
// Safe to call when highlighter is nil; it just returns. Also re-applies the
// semantic-token overlay so a fresh chroma run plus a still-valid token batch
// merges to the same result.
func (p *Pane) applyHighlight() {
	if p.highlighter == nil {
		p.chromaSpans = highlight.Result{}
		p.spans = highlight.Result{}
		return
	}
	if p.bufVer != p.hlVer {
		src := strings.Join(p.buf.Lines, "\n")
		p.chromaSpans = p.highlighter.Highlight(p.path, src)
		p.hlVer = p.bufVer
	}
	p.applyOverlay()
}

// applyOverlay refreshes p.spans from p.chromaSpans plus the semantic-token
// overlay when the tokens are still relevant to the current buffer version.
func (p *Pane) applyOverlay() {
	if len(p.semTokens) > 0 && p.semBufVer == p.bufVer {
		p.spans = highlight.MergeSemantic(p.chromaSpans, p.semTokens)
		return
	}
	p.spans = p.chromaSpans
}

// BufVer returns the current buffer-revision counter. Hosts pass this into
// lookup.SemanticTokensCmd so they can echo it back via WithSemanticTokens
// and let the editor discard a stale response.
func (p Pane) BufVer() int { return p.bufVer }

// WithSemanticTokens stores an LSP semanticTokens response for later overlay
// on top of the chroma highlight result. paneVer is the value Pane.BufVer()
// returned at request-issue time; if the buffer has moved on, the tokens are
// kept but won't render until a (never-arriving) matching buffer version,
// effectively making the overlay self-clean.
func (p Pane) WithSemanticTokens(tokens []semtok.Token, paneVer int) Pane {
	p.semTokens = tokens
	p.semBufVer = paneVer
	(&p).applyOverlay()
	return p
}

// ClearSemanticTokens drops the semantic-token overlay (e.g. when the file
// closes or the language server detaches). Returns the updated Pane.
func (p Pane) ClearSemanticTokens() Pane {
	p.semTokens = nil
	p.semBufVer = 0
	(&p).applyOverlay()
	return p
}

func (p *Pane) clampCol() {
	max := len(p.buf.Lines[p.row])
	if p.col > max {
		p.col = max
	}
}

func (p *Pane) insertRunes(r []rune) {
	line := p.buf.Lines[p.row]
	if p.col > len(line) {
		p.col = len(line)
	}
	p.buf.Lines[p.row] = line[:p.col] + string(r) + line[p.col:]
	p.col += len(r)
	p.buf.Dirty = true
	p.bufVer++
}

func (p *Pane) insertNewline() {
	line := p.buf.Lines[p.row]
	left, right := line[:p.col], line[p.col:]
	p.buf.Lines[p.row] = left
	tail := append([]string{right}, p.buf.Lines[p.row+1:]...)
	p.buf.Lines = append(p.buf.Lines[:p.row+1], tail...)
	p.row++
	p.col = 0
	p.buf.Dirty = true
	p.bufVer++
	p.ensureVisible()
}

func (p *Pane) delBack() {
	if p.col > 0 {
		line := p.buf.Lines[p.row]
		p.buf.Lines[p.row] = line[:p.col-1] + line[p.col:]
		p.col--
		p.buf.Dirty = true
		p.bufVer++
		return
	}
	if p.row == 0 {
		return
	}
	prev := p.buf.Lines[p.row-1]
	curr := p.buf.Lines[p.row]
	p.col = len(prev)
	p.buf.Lines[p.row-1] = prev + curr
	p.buf.Lines = append(p.buf.Lines[:p.row], p.buf.Lines[p.row+1:]...)
	p.row--
	p.buf.Dirty = true
	p.bufVer++
	p.ensureVisible()
}

func (p *Pane) delForward() {
	line := p.buf.Lines[p.row]
	if p.col < len(line) {
		p.buf.Lines[p.row] = line[:p.col] + line[p.col+1:]
		p.buf.Dirty = true
		p.bufVer++
		return
	}
	if p.row >= len(p.buf.Lines)-1 {
		return
	}
	p.buf.Lines[p.row] = line + p.buf.Lines[p.row+1]
	p.buf.Lines = append(p.buf.Lines[:p.row+1], p.buf.Lines[p.row+2:]...)
	p.buf.Dirty = true
	p.bufVer++
}

// deleteCurrentLine removes the row the primary cursor sits on. Used by Ctrl+X
// when no selection is active (VSCode default: cut entire line). The cursor
// stays on the same row index when possible, landing at column 0; on the last
// row it drops one row. A single-line buffer becomes one empty line.
func (p *Pane) deleteCurrentLine() {
	if len(p.buf.Lines) <= 1 {
		p.buf.Lines = []string{""}
		p.row, p.col = 0, 0
		p.anchorRow, p.anchorCol = 0, 0
		p.buf.Dirty = true
		p.bufVer++
		p.applyHighlight()
		p.ensureVisible()
		return
	}
	p.buf.Lines = append(p.buf.Lines[:p.row], p.buf.Lines[p.row+1:]...)
	if p.row >= len(p.buf.Lines) {
		p.row = len(p.buf.Lines) - 1
	}
	p.col = 0
	p.anchorRow, p.anchorCol = p.row, p.col
	p.buf.Dirty = true
	p.bufVer++
	p.applyHighlight()
	p.ensureVisible()
}

// insertRunesAllCursors inserts r at every cursor in ascending order, advancing
// each. Wraps insertRunes' core via applyAtAllCursors so later cursors stay
// consistent with the in-place buffer mutation.
func (p *Pane) insertRunesAllCursors(r []rune) {
	if len(r) == 0 {
		return
	}
	n := len(r)
	s := string(r)
	p.applyAtAllCursors(func(row, col int) (CursorPos, func(CursorPos) CursorPos) {
		for row >= len(p.buf.Lines) {
			p.buf.Lines = append(p.buf.Lines, "")
		}
		line := p.buf.Lines[row]
		if col > len(line) {
			col = len(line)
		}
		p.buf.Lines[row] = line[:col] + s + line[col:]
		return CursorPos{Row: row, Col: col + n}, func(other CursorPos) CursorPos {
			return shiftAfterInsertRunes(other, row, col, n)
		}
	})
	p.ensureVisible()
}

// newlineAllCursors splits the line at every cursor in ascending order. Each
// cursor moves to column 0 of the new line below.
func (p *Pane) newlineAllCursors() {
	p.applyAtAllCursors(func(row, col int) (CursorPos, func(CursorPos) CursorPos) {
		line := p.buf.Lines[row]
		if col > len(line) {
			col = len(line)
		}
		left, right := line[:col], line[col:]
		p.buf.Lines[row] = left
		tail := append([]string{right}, p.buf.Lines[row+1:]...)
		p.buf.Lines = append(p.buf.Lines[:row+1], tail...)
		return CursorPos{Row: row + 1, Col: 0}, func(other CursorPos) CursorPos {
			return shiftAfterInsertNewline(other, row, col)
		}
	})
	p.ensureVisible()
}

// backspaceAllCursors deletes the char before every cursor (or merges with the
// previous row when at col 0). Processes ascending so later positions stay
// consistent after each delete or merge.
func (p *Pane) backspaceAllCursors() {
	p.applyAtAllCursors(func(row, col int) (CursorPos, func(CursorPos) CursorPos) {
		if col > 0 {
			line := p.buf.Lines[row]
			if col > len(line) {
				col = len(line)
			}
			p.buf.Lines[row] = line[:col-1] + line[col:]
			return CursorPos{Row: row, Col: col - 1}, func(other CursorPos) CursorPos {
				return shiftAfterDeleteChar(other, row, col-1)
			}
		}
		if row == 0 {
			return CursorPos{Row: 0, Col: 0}, func(other CursorPos) CursorPos { return other }
		}
		prev := p.buf.Lines[row-1]
		curr := p.buf.Lines[row]
		oldPrevLen := len(prev)
		p.buf.Lines[row-1] = prev + curr
		p.buf.Lines = append(p.buf.Lines[:row], p.buf.Lines[row+1:]...)
		return CursorPos{Row: row - 1, Col: oldPrevLen}, func(other CursorPos) CursorPos {
			return shiftAfterMergeWithAbove(other, row, oldPrevLen)
		}
	})
	p.ensureVisible()
}

// delForwardAllCursors deletes the char at every cursor (or merges row+1 when
// at EOL). Cursors stay in place; later positions shift as needed.
func (p *Pane) delForwardAllCursors() {
	p.applyAtAllCursors(func(row, col int) (CursorPos, func(CursorPos) CursorPos) {
		line := p.buf.Lines[row]
		if col < len(line) {
			p.buf.Lines[row] = line[:col] + line[col+1:]
			return CursorPos{Row: row, Col: col}, func(other CursorPos) CursorPos {
				return shiftAfterDeleteChar(other, row, col)
			}
		}
		if row >= len(p.buf.Lines)-1 {
			return CursorPos{Row: row, Col: col}, func(other CursorPos) CursorPos { return other }
		}
		oldRowLen := len(line)
		p.buf.Lines[row] = line + p.buf.Lines[row+1]
		p.buf.Lines = append(p.buf.Lines[:row+1], p.buf.Lines[row+2:]...)
		return CursorPos{Row: row, Col: col}, func(other CursorPos) CursorPos {
			return shiftAfterMergeWithBelow(other, row, oldRowLen)
		}
	})
	p.ensureVisible()
}

// View renders the pane.
func (p Pane) View() string {
	t := p.theme
	gutterStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	cursorLineGutterStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	infoStyle := lipgloss.NewStyle().Foreground(t.Info)
	gitAddStyle := lipgloss.NewStyle().Foreground(t.Success)
	gitModStyle := lipgloss.NewStyle().Foreground(t.Warning)
	gitDelStyle := lipgloss.NewStyle().Foreground(t.Error)
	bpStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	stopStyle := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	header := lipgloss.NewStyle().Foreground(t.TextMuted).Bold(true)
	dirty := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	muted := lipgloss.NewStyle().Foreground(t.TextMuted)

	visible := p.height - 1
	if visible < 1 {
		visible = 1
	}
	end := p.offset + visible
	if end > len(p.buf.Lines) {
		end = len(p.buf.Lines)
	}

	gw := numWidth(len(p.buf.Lines))
	if !p.lineNumbers {
		gw = 0
	}
	contentWidth := p.width - gw - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	tabW := p.tabWidth
	if tabW <= 0 {
		tabW = 4
	}

	// Group every cursor (primary + extras) by row so renderHighlightedRow can
	// paint one cell per cursor. Cursor cols are kept sorted ascending; the
	// primary cursor's col is tracked separately so the ghost-text preview is
	// anchored to the primary.
	cursorsByRow := map[int][]int{p.row: {p.col}}
	for _, e := range p.extras {
		cursorsByRow[e.Row] = append(cursorsByRow[e.Row], e.Col)
	}
	for r := range cursorsByRow {
		sort.Ints(cursorsByRow[r])
	}

	// Compute per-row selection bounds once. selStartRow/selEndRow bracket
	// the visible band of selection; on rows in the middle of a multi-line
	// selection, the byte range covers the full row + the trailing newline
	// indicator (rendered as a single trailing space).
	selActive := p.selecting
	var selStartRow, selStartCol, selEndRow, selEndCol int
	if selActive {
		selStartRow, selStartCol, selEndRow, selEndCol = p.SelectionRange()
		if selStartRow == selEndRow && selStartCol == selEndCol {
			selActive = false
		}
	}

	var rows []string
	for i := p.offset; i < end; i++ {
		var gut string
		if p.lineNumbers {
			num := fmt.Sprintf("%*d", gw, i+1)
			if i == p.row {
				gut = cursorLineGutterStyle.Render(num)
			} else {
				gut = gutterStyle.Render(num)
			}
		}
		// Two-character marker column. First char carries the debugger
		// stop arrow (▶) when execution paused here, the DAP breakpoint
		// dot (●), or the git-diff sigil (▎/▔). Precedence is
		// stop > breakpoint > git. Second char carries the worst LSP
		// diagnostic. Either can be empty (space + │) so the column
		// width stays fixed regardless of which signals are active.
		gitChar := " "
		switch p.LineMarkerAt(i) {
		case gitgutter.Added:
			gitChar = gitAddStyle.Render("▎")
		case gitgutter.Modified:
			gitChar = gitModStyle.Render("▎")
		case gitgutter.DeletedAbove:
			gitChar = gitDelStyle.Render("▔")
		}
		if p.IsBreakpoint(i) {
			gitChar = bpStyle.Render("●")
		}
		if p.hasStop && p.stoppedAtRow == i {
			gitChar = stopStyle.Render("▶")
		}
		diagChar := "│"
		switch p.DiagnosticAt(i) {
		case SeverityError:
			diagChar = errStyle.Render("●")
		case SeverityWarning:
			diagChar = warnStyle.Render("●")
		case SeverityInfo, SeverityHint:
			diagChar = infoStyle.Render("●")
		}
		marker := gitChar + diagChar
		raw := p.buf.Lines[i]
		spans := p.spans.Spans(i)
		marks, activeIdx := p.matchesForRow(i)
		hints := p.InlayHintsAt(i)
		// Selection bounds for this row: -1/-1 when this row is outside
		// the selection. Mid-rows of a multi-line selection get a full
		// row coverage plus a one-cell trailing tail (rendered after the
		// last rune) to indicate the newline is included.
		selStart, selEnd, selTail := -1, -1, false
		if selActive && i >= selStartRow && i <= selEndRow {
			switch {
			case selStartRow == selEndRow:
				selStart, selEnd = selStartCol, selEndCol
			case i == selStartRow:
				selStart = selStartCol
				selEnd = len(raw)
				selTail = true
			case i == selEndRow:
				selStart = 0
				selEnd = selEndCol
			default:
				selStart = 0
				selEnd = len(raw)
				selTail = true
			}
		}
		var line string
		var dochiSpans []dochi.Span
		if p.docHighlights != nil {
			dochiSpans = p.docHighlights[i]
		}
		cols, hasCursors := cursorsByRow[i]
		if hasCursors {
			primaryCol := -1
			ghost := ""
			if i == p.row {
				primaryCol = p.col
				if p.ghostText != "" && p.focused {
					ghost = p.ghostText
				}
			}
			line = renderHighlightedRow(raw, spans, marks, activeIdx, dochiSpans, cols, primaryCol, ghost, hints, contentWidth, t, true, tabW, selStart, selEnd, selTail)
		} else {
			line = renderHighlightedRow(raw, spans, marks, activeIdx, dochiSpans, nil, -1, "", hints, contentWidth, t, false, tabW, selStart, selEnd, selTail)
		}
		if p.blameVisible && i == p.row {
			if entry, ok := p.BlameAt(i); ok {
				line = appendBlameStrip(line, entry, contentWidth, t)
			}
		}
		rows = append(rows, gut+marker+" "+line)
	}
	for len(rows) < visible {
		rows = append(rows, gutterStyle.Render(strings.Repeat(" ", gw))+" │ "+muted.Render("~"))
	}

	title := header.Render("editor")
	if p.path != "" {
		title += "  " + muted.Render(p.path)
	}
	if p.buf.Dirty {
		title += "  " + dirty.Render("●")
	}
	pos := fmt.Sprintf("  %s", muted.Render(fmt.Sprintf("%d:%d", p.row+1, p.col+1)))
	title += pos

	return strings.Join(append([]string{title}, rows...), "\n")
}

func renderCursor(line string, col int, t theme.Theme) string {
	return renderCursorRowClipped(line, col, "", -1, t)
}

// appendBlameStrip pads the rendered cursor row to a fixed gap and appends
// the blame entry as dim italic text, truncated so the total displayed width
// stays at or below contentWidth. Returns the rendered line unchanged when
// there isn't room for even a short suffix.
func appendBlameStrip(line string, entry inlineblame.Line, contentWidth int, t theme.Theme) string {
	const gap = 4
	const minBlame = 12
	used := lipgloss.Width(line)
	remaining := contentWidth - used - gap
	if remaining < minBlame {
		return line
	}
	strip := inlineblame.Render(entry, nowFunc(), remaining)
	if strip == "" {
		return line
	}
	style := lipgloss.NewStyle().Foreground(t.TextMuted).Faint(true).Italic(true)
	return line + strings.Repeat(" ", gap) + style.Render(strip)
}

// renderCursorRowClipped budgets visible columns first (treating ghost text as
// consuming columns alongside buffer text), then layers styles. If width < 0
// no clipping is applied. Cursor sits at col; ghost text renders muted-faint
// inserted at col, the cursor highlighting its first rune.
func renderCursorRowClipped(line string, col int, ghost string, width int, t theme.Theme) string {
	cur := lipgloss.NewStyle().Foreground(t.TextInverse).Background(t.Primary)
	muted := lipgloss.NewStyle().Foreground(t.TextMuted).Faint(true)
	r := []rune(line)
	if col < 0 {
		col = 0
	}
	// Compose: prefix + ghost + tail in plain runes first, then split into
	// (prefix, cursorRune, after) for styling.
	prefix := r
	tail := []rune{}
	if col < len(r) {
		prefix = r[:col]
		tail = r[col:]
	}
	gr := []rune(ghost)

	// Total visible columns: len(prefix) + len(gr) + len(tail) + (1 for end-cursor if needed)
	endCursor := col >= len(r) && len(gr) == 0

	// Apply width budget. The trailing ellipsis costs 1.
	if width > 0 {
		total := len(prefix) + len(gr) + len(tail)
		if endCursor {
			total++ // cursor space
		}
		if total > width {
			// Drop from the back: prefer keeping prefix + ghost + as much tail as fits.
			budget := width - 1 // reserve ellipsis
			if budget < 0 {
				budget = 0
			}
			// Reserve cursor cell when at end.
			if endCursor && budget > 0 {
				budget--
			}
			used := len(prefix) + len(gr)
			if used >= budget {
				// Even prefix+ghost overflows; clip them right-to-left from tail end.
				if used > budget {
					if len(gr) >= used-budget {
						gr = gr[:len(gr)-(used-budget)]
					} else {
						// shouldn't happen but safe
						gr = nil
						prefix = prefix[:budget]
					}
				}
				tail = nil
				return styledRow(prefix, gr, tail, col, len(r), cur, muted) + "…"
			}
			remain := budget - used
			if remain < len(tail) {
				tail = tail[:remain]
				return styledRow(prefix, gr, tail, col, len(r), cur, muted) + "…"
			}
		}
	}

	return styledRow(prefix, gr, tail, col, len(r), cur, muted)
}

// styledRow renders the pre-split row segments with cursor + ghost styling.
// origLen is the original len(r); we use it to detect whether the cursor is at
// end-of-line (no character under it) so we paint a space cursor.
func styledRow(prefix, ghost, tail []rune, col, origLen int, cur, muted lipgloss.Style) string {
	if col >= origLen {
		// Cursor at end of line. If we have ghost, cursor lights the first ghost rune.
		if len(ghost) == 0 {
			return string(prefix) + cur.Render(" ")
		}
		first := cur.Render(string(ghost[0]))
		rest := ""
		if len(ghost) > 1 {
			rest = muted.Render(string(ghost[1:]))
		}
		return string(prefix) + first + rest
	}
	// Cursor mid-line.
	if len(ghost) == 0 {
		if len(tail) == 0 {
			return string(prefix) + cur.Render(" ")
		}
		return string(prefix) + cur.Render(string(tail[0])) + string(tail[1:])
	}
	first := cur.Render(string(ghost[0]))
	rest := ""
	if len(ghost) > 1 {
		rest = muted.Render(string(ghost[1:]))
	}
	return string(prefix) + first + rest + string(tail)
}

// renderHighlightedRow renders a single buffer row with syntax spans, optional
// cursor cell, optional ghost-text suffix, tab expansion (\t -> 4 spaces), and
// width clipping. cursorCol is in RAW byte coords (matching p.col); if
// drawCursor is false the cursor cell is not drawn. Span byte offsets are also
// raw and walked alongside the input.
//
// Rendering invariants the existing tests rely on:
//   - The plain text content of the row appears verbatim (substring tests in
//     editor_test.go check for "hello", "tln", etc.).
//   - Ghost text is rendered with the first rune cursor-styled, the rest faint.
//   - Trailing "…" appears only when the row exceeded width.
//
// matchKind tags a rune as outside a match, inside a non-active match, or
// inside the active match. Background styles vary per kind.
type matchKind int

const (
	matchNone matchKind = iota
	matchOther
	matchActive
)

// matchesForRow returns the slice of Range covering row plus the index of the
// active match within that slice (or -1 if no active match falls on row).
func (p Pane) matchesForRow(row int) ([]Range, int) {
	if len(p.searchMatches) == 0 {
		return nil, -1
	}
	var out []Range
	active := -1
	for i, r := range p.searchMatches {
		if r.Row != row {
			continue
		}
		if i == p.searchCurrent {
			active = len(out)
		}
		out = append(out, r)
	}
	return out, active
}

// renderHighlightedRow renders a single buffer row with syntax spans, optional
// cursor cells, optional ghost-text suffix, tab expansion (\t -> 4 spaces), and
// width clipping. cursorCols is a sorted-ascending slice of RAW byte columns;
// each gets a cursor cell. primaryCol is the byte col of the primary cursor on
// this row (or -1 if not on this row) — used to anchor ghost-text rendering.
// drawCursor disables all cursor cells when false (rows without any cursor).
// hints is the inlay-hint slice for the row; each renders as dim italic text
// at its display column, between the rune at that column and the rune to its
// left. Hints anchored at or past end-of-line render after the last rune.
// selStart/selEnd are raw-byte columns (inclusive/exclusive) covered by the
// active selection on this row; -1/-1 means no selection on this row. selTail
// is true when the selection extends past EOL on this row (mid-rows of a
// multi-line selection); a trailing space is painted to make the wrap visible.
// dochiSpans is the slice of document-highlight bands on this row (LSP
// textDocument/documentHighlight). Each rune inside a span gets a subtle
// SurfaceStrong background painted UNDER the syntax color; matches and
// selection still win when both apply (see flush precedence chain).
func renderHighlightedRow(raw string, spans []highlight.Span, matches []Range, activeMatch int, dochiSpans []dochi.Span, cursorCols []int, primaryCol int, ghost string, hints []inlayhint.Hint, width int, t theme.Theme, drawCursor bool, tabWidth int, selStart, selEnd int, selTail bool) string {
	if tabWidth <= 0 {
		tabWidth = 4
	}
	cur := lipgloss.NewStyle().Foreground(t.TextInverse).Background(t.Primary)
	muted := lipgloss.NewStyle().Foreground(t.TextMuted).Faint(true)
	hintStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Faint(true).Italic(true)
	// matchBG is the dim highlight applied to every match on the row;
	// matchActiveBG is the stronger highlight on the currently-selected match.
	matchBG := lipgloss.NewStyle().Background(t.SurfaceStrong)
	if string(t.SurfaceStrong) == "" {
		matchBG = lipgloss.NewStyle().Background(t.Surface)
	}
	matchActiveBG := lipgloss.NewStyle().Foreground(t.TextInverse).Background(t.Warning).Bold(true)
	if string(t.Warning) == "" {
		matchActiveBG = lipgloss.NewStyle().Foreground(t.TextInverse).Background(t.Primary).Bold(true)
	}
	// selBG paints the active text selection. Background is the primary
	// accent (the same band the cursor cell wears) so a held Shift+motion
	// reads as a single contiguous painted region; foreground flips to
	// TextInverse so syntax colors stay legible over the strong band.
	selBG := lipgloss.NewStyle().Foreground(t.TextInverse).Background(t.Primary)
	if string(t.Primary) == "" {
		selBG = lipgloss.NewStyle().Background(t.SurfaceStrong)
	}
	palette := syntaxPalette(t)

	// Pre-expand tabs and build a raw-byte -> display-column index so spans
	// and the cursor map cleanly into expanded coords. expandedRunes holds
	// the runes we'll emit; runeKind[i] is the Kind backing expandedRunes[i];
	// runeMatch[i] tags whether the rune is inside a finder match (and which);
	// runeSel[i] flags whether the rune sits inside the active selection;
	// runeDocHi[i] flags whether the rune sits inside an LSP document-
	// highlight span (the cursor-settle "every occurrence" overlay).
	hasSel := selStart >= 0 && selEnd > selStart
	hasDochi := len(dochiSpans) > 0
	var expanded []rune
	var runeKind []highlight.Kind
	var runeMatch []matchKind
	var runeSel []bool
	var runeDocHi []bool
	rawToExpStart := make([]int, len(raw)+1) // raw byte offset -> first expanded rune idx
	for i := 0; i < len(raw); i++ {
		rawToExpStart[i] = len(expanded)
		c := raw[i]
		// Find which span covers this byte (linear scan; spans are sorted).
		kind := highlight.KindPlain
		for _, s := range spans {
			if i >= s.Start && i < s.End {
				kind = s.Kind
				break
			}
			if s.Start > i {
				break
			}
		}
		// Compute match kind for this byte. Active match wins over Other when
		// they overlap (matches don't normally overlap, but defensively the
		// stronger style is the more useful default).
		mk := matchNone
		for j, m := range matches {
			if i >= m.Start && i < m.End {
				if j == activeMatch {
					mk = matchActive
					break
				}
				mk = matchOther
			}
		}
		sel := hasSel && i >= selStart && i < selEnd
		dh := hasDochi && dochi.Covers(dochiSpans, i, len(raw))
		if c == '\t' {
			for k := 0; k < tabWidth; k++ {
				expanded = append(expanded, ' ')
				runeKind = append(runeKind, highlight.KindPlain)
				runeMatch = append(runeMatch, mk)
				runeSel = append(runeSel, sel)
				runeDocHi = append(runeDocHi, dh)
			}
			continue
		}
		// Standard byte. For multi-byte UTF-8 we still emit per-rune below;
		// here we just count the leading byte. We append the rune later via
		// rune iteration to keep widths sane.
		expanded = append(expanded, rune(c))
		runeKind = append(runeKind, kind)
		runeMatch = append(runeMatch, mk)
		runeSel = append(runeSel, sel)
		runeDocHi = append(runeDocHi, dh)
	}
	rawToExpStart[len(raw)] = len(expanded)

	// Convert each raw cursor col to a display column (rune index in expanded).
	// dispCursorSet allows fast lookup during the rune emit loop; dispPrimary is
	// tracked separately so the ghost-text suffix is anchored to the primary
	// cursor only.
	dispCursorSet := make(map[int]bool)
	dispPrimary := -1
	cursorAtEnd := false
	if drawCursor {
		for _, c := range cursorCols {
			if c < 0 {
				c = 0
			}
			var dc int
			if c >= len(raw) {
				dc = len(expanded)
				cursorAtEnd = true
			} else {
				dc = rawToExpStart[c]
			}
			dispCursorSet[dc] = true
		}
		if primaryCol >= 0 {
			if primaryCol >= len(raw) {
				dispPrimary = len(expanded)
			} else {
				dispPrimary = rawToExpStart[primaryCol]
			}
		}
	}

	ghostRunes := []rune(ghost)

	// Translate each hint to a display column, pad it, pre-render the styled
	// label. Hints are decorative; they're the first thing dropped if width
	// is tight. Sort ascending by display column to keep the iteration order
	// stable and make "drop from the back" mean "drop rightmost first."
	type displayHint struct {
		col      int
		rendered string
		runes    int
	}
	var hintItems []displayHint
	hintCellsTotal := 0
	for _, h := range hints {
		var dc int
		switch {
		case h.Col < 0:
			dc = 0
		case h.Col >= len(raw):
			dc = len(expanded)
		default:
			dc = rawToExpStart[h.Col]
		}
		text := h.Label
		if h.PaddingLeft {
			text = " " + text
		}
		if h.PaddingRight {
			text = text + " "
		}
		rr := []rune(text)
		if len(rr) == 0 {
			continue
		}
		hintItems = append(hintItems, displayHint{
			col:      dc,
			rendered: hintStyle.Render(text),
			runes:    len(rr),
		})
		hintCellsTotal += len(rr)
	}
	sort.SliceStable(hintItems, func(i, j int) bool {
		return hintItems[i].col < hintItems[j].col
	})

	// Width budget: visible content + hint cells + (1 if any cursor at EOL with
	// no ghost) + (1 if ellipsis).
	endCursor := drawCursor && cursorAtEnd && len(ghostRunes) == 0
	totalCells := len(expanded) + len(ghostRunes) + hintCellsTotal
	if endCursor {
		totalCells++
	}
	clipped := false
	if width > 0 && totalCells > width {
		// Hints are decorative — drop them from the back first.
		for len(hintItems) > 0 && totalCells > width {
			last := hintItems[len(hintItems)-1]
			hintCellsTotal -= last.runes
			totalCells -= last.runes
			hintItems = hintItems[:len(hintItems)-1]
		}
	}
	if width > 0 && totalCells > width {
		clipped = true
		// Reserve 1 col for ellipsis (and 1 for endCursor if drawn).
		budget := width - 1
		if budget < 0 {
			budget = 0
		}
		if endCursor && budget > 0 {
			budget--
		}
		// Trim from the back. Prefer keeping expanded then ghost.
		if budget <= len(expanded) {
			expanded = expanded[:budget]
			runeKind = runeKind[:budget]
			runeMatch = runeMatch[:budget]
			runeSel = runeSel[:budget]
			runeDocHi = runeDocHi[:budget]
			ghostRunes = nil
		} else {
			remain := budget - len(expanded)
			if remain < len(ghostRunes) {
				ghostRunes = ghostRunes[:remain]
			}
		}
		// Hints anchored past the clipped expanded boundary are now invisible.
		keep := hintItems[:0]
		for _, hi := range hintItems {
			if hi.col <= len(expanded) {
				keep = append(keep, hi)
			}
		}
		hintItems = keep
	}

	// Group kept hints by display column for O(1) lookup in the emit loop.
	dispHintsByCol := make(map[int][]displayHint, len(hintItems))
	for _, hi := range hintItems {
		dispHintsByCol[hi.col] = append(dispHintsByCol[hi.col], hi)
	}

	// Emit. Batch contiguous runes of the same (kind, matchKind, sel, dochi)
	// tuple into one Style.Render call — per-rune emission inflates ANSI
	// overhead and breaks substring asserts in tests because each rune
	// gets surrounded by escape codes. The cursor cell breaks the run and
	// is emitted separately. Precedence chain (strongest first):
	//   matchActive > selection > matchOther > dochi > plain
	// Selection takes precedence over the inactive match band but yields
	// to the active search match (so a Ctrl+F hit stays salient while the
	// user grows the selection toward it). Document highlights paint a
	// subtle SurfaceStrong band, lower than the search overlays so a
	// hovered identifier never out-shouts the user's active query.
	var b strings.Builder
	flush := func(buf []rune, k highlight.Kind, m matchKind, sel bool, dh bool) {
		if len(buf) == 0 {
			return
		}
		if m == matchActive {
			b.WriteString(matchActiveBG.Render(string(buf)))
			return
		}
		if sel {
			b.WriteString(selBG.Render(string(buf)))
			return
		}
		switch m {
		case matchOther:
			// Layer syntax color over the match background. lipgloss styles
			// merge cleanly when applied via Background + Foreground in one
			// pass; pick the foreground from the syntax palette.
			st, ok := palette[k]
			if !ok || k == highlight.KindPlain {
				b.WriteString(matchBG.Render(string(buf)))
				return
			}
			b.WriteString(st.Background(matchBG.GetBackground()).Render(string(buf)))
			return
		}
		if dh {
			// Document highlight: same SurfaceStrong band the muted match
			// state uses. The two overlays don't visually conflict in
			// practice because search highlights arbitrary text and dochi
			// only lights up identifiers; match wins the precedence above
			// regardless. Layer syntax color over the band so keyword/
			// string colors remain legible on the painted cells.
			st, ok := palette[k]
			if !ok || k == highlight.KindPlain {
				b.WriteString(matchBG.Render(string(buf)))
				return
			}
			b.WriteString(st.Background(matchBG.GetBackground()).Render(string(buf)))
			return
		}
		if k == highlight.KindPlain {
			b.WriteString(string(buf))
			return
		}
		st, ok := palette[k]
		if !ok {
			b.WriteString(string(buf))
			return
		}
		b.WriteString(st.Render(string(buf)))
	}
	emitHintsAt := func(col int) {
		hs := dispHintsByCol[col]
		if len(hs) == 0 {
			return
		}
		for _, hi := range hs {
			b.WriteString(hi.rendered)
		}
		delete(dispHintsByCol, col)
	}
	var run []rune
	var runKind highlight.Kind
	var runMatch matchKind
	var runSel bool
	var runDocHi bool
	for i, r := range expanded {
		if _, hasHint := dispHintsByCol[i]; hasHint {
			flush(run, runKind, runMatch, runSel, runDocHi)
			run = nil
			emitHintsAt(i)
		}
		if drawCursor && dispCursorSet[i] {
			flush(run, runKind, runMatch, runSel, runDocHi)
			run = nil
			b.WriteString(cur.Render(string(r)))
			continue
		}
		k := runeKind[i]
		var m matchKind
		if i < len(runeMatch) {
			m = runeMatch[i]
		}
		var sel bool
		if i < len(runeSel) {
			sel = runeSel[i]
		}
		var dh bool
		if i < len(runeDocHi) {
			dh = runeDocHi[i]
		}
		if len(run) > 0 && (k != runKind || m != runMatch || sel != runSel || dh != runDocHi) {
			flush(run, runKind, runMatch, runSel, runDocHi)
			run = run[:0]
		}
		runKind = k
		runMatch = m
		runSel = sel
		runDocHi = dh
		run = append(run, r)
	}
	flush(run, runKind, runMatch, runSel, runDocHi)
	// EOL hints (anchored at or past the last rune) render before ghost-text
	// and the end-cursor cell so they sit naturally after the line content.
	emitHintsAt(len(expanded))
	// Trailing selection tail. Mid-rows of a multi-line selection extend past
	// EOL; render one painted cell so the wrap to the next line reads as a
	// continuous band. Suppressed when the cursor is at EOL (the cursor cell
	// owns that visual real estate).
	if selTail && !cursorAtEnd && !clipped {
		b.WriteString(selBG.Render(" "))
	}
	// Ghost text rendered next; first rune cursor-styled if the PRIMARY cursor
	// is at EOL (no rune under it), else all muted. Extra cursors do not anchor
	// ghost text.
	if len(ghostRunes) > 0 {
		if drawCursor && dispPrimary >= len(expanded) {
			b.WriteString(cur.Render(string(ghostRunes[0])))
			if len(ghostRunes) > 1 {
				b.WriteString(muted.Render(string(ghostRunes[1:])))
			}
		} else {
			b.WriteString(muted.Render(string(ghostRunes)))
		}
	} else if drawCursor && cursorAtEnd {
		// Any cursor at EOL with no ghost: paint a space cursor cell.
		b.WriteString(cur.Render(" "))
	}
	if clipped {
		b.WriteString("…")
	}
	return b.String()
}

// syntaxPalette materializes per-Kind lipgloss styles once per row render.
// Theme colors map onto the palette; missing tokens (e.g. an older theme that
// hasn't been updated yet) fall back to Text.
func syntaxPalette(t theme.Theme) map[highlight.Kind]lipgloss.Style {
	pick := func(c, fallback lipgloss.Color) lipgloss.Color {
		if string(c) == "" {
			return fallback
		}
		return c
	}
	return map[highlight.Kind]lipgloss.Style{
		highlight.KindKeyword:     lipgloss.NewStyle().Foreground(pick(t.SyntaxKeyword, t.Primary)),
		highlight.KindString:      lipgloss.NewStyle().Foreground(pick(t.SyntaxString, t.Accent)),
		highlight.KindComment:     lipgloss.NewStyle().Foreground(pick(t.SyntaxComment, t.TextMuted)).Italic(true),
		highlight.KindNumber:      lipgloss.NewStyle().Foreground(pick(t.SyntaxNumber, t.Success)),
		highlight.KindFunction:    lipgloss.NewStyle().Foreground(pick(t.SyntaxFunction, t.Warning)),
		highlight.KindType:        lipgloss.NewStyle().Foreground(pick(t.SyntaxType, t.Info)),
		highlight.KindPunctuation: lipgloss.NewStyle().Foreground(pick(t.SyntaxPunctuation, t.TextMuted)),
		highlight.KindParameter:   lipgloss.NewStyle().Foreground(pick(t.SyntaxParameter, t.Text)),
		highlight.KindProperty:    lipgloss.NewStyle().Foreground(pick(t.SyntaxProperty, t.Info)),
		highlight.KindEnumMember:  lipgloss.NewStyle().Foreground(pick(t.SyntaxEnumMember, t.Warning)),
		highlight.KindNamespace:   lipgloss.NewStyle().Foreground(pick(t.SyntaxNamespace, t.Info)),
		highlight.KindReadonly:    lipgloss.NewStyle().Foreground(pick(t.SyntaxReadonly, t.Accent)),
	}
}

func numWidth(n int) int {
	if n < 1 {
		return 1
	}
	w := 0
	for n > 0 {
		w++
		n /= 10
	}
	if w < 3 {
		return 3
	}
	return w
}
