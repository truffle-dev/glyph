// Package outline is the single-file document-symbol modal that nook
// opens on Ctrl+\. Workspace symbol search (Ctrl+T, v0.23.0) and the
// outline overlay are complementary: workspace search asks for a query
// and emits multibuffer fragments across the whole project; outline
// pre-loads the current file's symbol tree, pre-positions the cursor
// at the symbol enclosing the user's row, and jumps the editor on Enter.
//
// The wedge is intentionally small. Symbols are loaded once when the
// pane opens (we cache by path in the host) and a tiny filter narrows
// the visible list as the user types. Depth indents draw the tree shape
// so a struct's methods read as nested under their parent. Up / Down
// move the cursor, Enter jumps, Esc cancels.
package outline

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/components/theme"
)

// Symbol is the flat-list view of one entry. The outline pane carries
// these instead of lsp.DocSymbol so depth is rendered without re-walking
// the tree on every keystroke.
type Symbol struct {
	Name    string
	Detail  string
	Kind    lsp.WorkspaceSymbolKind
	Line    int // 0-based source line of the symbol's identifier
	Col     int // 0-based column on Line
	EndLine int // 0-based last line of the enclosing range
	Depth   int // 0 = top-level
}

// JumpMsg is emitted on Enter. The host moves the editor cursor to
// (Row, Col) inside Path.
type JumpMsg struct {
	Path string
	Row  int
	Col  int
}

// CancelMsg is emitted on Esc.
type CancelMsg struct{}

// minWidth and minHeight are the smallest sane outer dimensions. The
// host always passes a workable size from the running window, but the
// minimums guard tests and very-small TTYs.
const (
	minWidth  = 40
	minHeight = 8
)

// Pane is the immutable outline modal model.
type Pane struct {
	theme    theme.Theme
	open     bool
	path     string
	symbols  []Symbol
	filtered []int  // indices into symbols
	filter   []rune // user's narrowing query
	cursor   int    // index into filtered
	offset   int    // first visible row of filtered
	width    int
	height   int
	errMsg   string
}

// New returns a closed pane.
func New(t theme.Theme) Pane {
	return Pane{theme: t, width: 60, height: 18}
}

// WithSize sets the outer width and height. Clamps to the small-screen
// floor so callers can pass the raw workspace size without prefiltering.
func (p Pane) WithSize(w, h int) Pane {
	if w < minWidth {
		w = minWidth
	}
	if h < minHeight {
		h = minHeight
	}
	p.width = w
	p.height = h
	return p
}

// Open arms the pane with the file's symbol tree. atRow is the user's
// current editor row (0-based); the cursor lands on the deepest symbol
// whose range contains atRow. An empty syms slice opens the pane with
// an "no symbols" hint instead of failing — the user still sees the
// modal and Esc still closes.
func (p Pane) Open(path string, syms []lsp.DocSymbol, atRow int) Pane {
	p.open = true
	p.path = path
	p.symbols = Flatten(syms)
	p.filter = nil
	p.errMsg = ""
	p.refilter()
	p.cursor = enclosingIndexFiltered(p.symbols, p.filtered, atRow)
	p.scrollToCursor()
	return p
}

// OpenError opens the pane in error-message mode. Used when the LSP
// request fails or the language server isn't attached. The user still
// sees the modal frame and can dismiss with Esc.
func (p Pane) OpenError(path, msg string) Pane {
	p.open = true
	p.path = path
	p.symbols = nil
	p.filtered = nil
	p.filter = nil
	p.cursor = 0
	p.offset = 0
	p.errMsg = msg
	return p
}

// Close hides the pane. Symbols and filter are dropped so the next
// Open starts clean.
func (p Pane) Close() Pane {
	p.open = false
	p.path = ""
	p.symbols = nil
	p.filtered = nil
	p.filter = nil
	p.cursor = 0
	p.offset = 0
	p.errMsg = ""
	return p
}

// IsOpen reports whether the modal should render.
func (p Pane) IsOpen() bool { return p.open }

// Path is the file the pane is currently outlining.
func (p Pane) Path() string { return p.path }

// Filter returns the current narrowing query.
func (p Pane) Filter() string { return string(p.filter) }

// Count returns the number of symbols matching the current filter.
func (p Pane) Count() int { return len(p.filtered) }

// Total returns the number of symbols in the file, ignoring the filter.
func (p Pane) Total() int { return len(p.symbols) }

// Highlighted returns the currently highlighted symbol, or false when
// the filter rejects everything.
func (p Pane) Highlighted() (Symbol, bool) {
	if p.cursor < 0 || p.cursor >= len(p.filtered) {
		return Symbol{}, false
	}
	return p.symbols[p.filtered[p.cursor]], true
}

// Update handles keystrokes. The pane is opaque to non-key messages —
// the host never routes anything else into it.
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	if !p.open {
		return p, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch km.Type {
	case tea.KeyEsc:
		return p.Close(), func() tea.Msg { return CancelMsg{} }
	case tea.KeyEnter:
		s, ok := p.Highlighted()
		if !ok {
			return p, nil
		}
		path := p.path
		row, col := s.Line, s.Col
		return p.Close(), func() tea.Msg { return JumpMsg{Path: path, Row: row, Col: col} }
	case tea.KeyUp, tea.KeyCtrlP:
		return p.moveCursor(-1), nil
	case tea.KeyDown, tea.KeyCtrlN:
		return p.moveCursor(1), nil
	case tea.KeyHome:
		p.cursor = 0
		p.scrollToCursor()
		return p, nil
	case tea.KeyEnd:
		if len(p.filtered) > 0 {
			p.cursor = len(p.filtered) - 1
		}
		p.scrollToCursor()
		return p, nil
	case tea.KeyPgUp:
		return p.moveCursor(-p.listHeight()), nil
	case tea.KeyPgDown:
		return p.moveCursor(p.listHeight()), nil
	case tea.KeyBackspace:
		if len(p.filter) > 0 {
			p.filter = p.filter[:len(p.filter)-1]
			p.refilter()
			p.cursor = 0
			p.scrollToCursor()
		}
		return p, nil
	case tea.KeyCtrlU:
		if len(p.filter) > 0 {
			p.filter = nil
			p.refilter()
			p.cursor = 0
			p.scrollToCursor()
		}
		return p, nil
	case tea.KeyRunes, tea.KeySpace:
		for _, r := range km.Runes {
			if r == 0 {
				continue
			}
			p.filter = append(p.filter, r)
		}
		if km.Type == tea.KeySpace && len(km.Runes) == 0 {
			p.filter = append(p.filter, ' ')
		}
		p.refilter()
		p.cursor = 0
		p.scrollToCursor()
		return p, nil
	}
	return p, nil
}

// moveCursor advances the cursor by delta, clamping to the filtered
// list bounds and re-scrolling so the cursor stays visible.
func (p Pane) moveCursor(delta int) Pane {
	if len(p.filtered) == 0 {
		p.cursor = 0
		return p
	}
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.filtered) {
		p.cursor = len(p.filtered) - 1
	}
	p.scrollToCursor()
	return p
}

// listHeight is the inner row count available for the symbol list. The
// header (file label + filter) and footer (key hints) each take one row;
// borders take two; that leaves height-4.
func (p Pane) listHeight() int {
	h := p.height - 4
	if h < 1 {
		return 1
	}
	return h
}

// scrollToCursor adjusts offset so the cursor sits inside the visible
// window. Called after every move / filter change.
func (p *Pane) scrollToCursor() {
	if len(p.filtered) == 0 {
		p.offset = 0
		return
	}
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+p.listHeight() {
		p.offset = p.cursor - p.listHeight() + 1
	}
	if p.offset < 0 {
		p.offset = 0
	}
}

// refilter rebuilds the filtered index list. With an empty filter every
// symbol passes through; otherwise we keep entries whose Name contains
// the filter as a case-insensitive substring. Document order is
// preserved — this is an outline, not a fuzzy picker.
func (p *Pane) refilter() {
	if len(p.symbols) == 0 {
		p.filtered = nil
		return
	}
	if len(p.filter) == 0 {
		p.filtered = allIndices(len(p.symbols))
		return
	}
	needle := strings.ToLower(string(p.filter))
	out := make([]int, 0, len(p.symbols))
	for i, s := range p.symbols {
		if strings.Contains(strings.ToLower(s.Name), needle) {
			out = append(out, i)
		}
	}
	p.filtered = out
}

func allIndices(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// View renders the modal. Centers itself horizontally; the host is
// expected to slot the rendered string into the workspace via
// lipgloss.Place. We render the bordered box directly so the host can
// reuse the same composition path used by edit / composer / rename.
func (p Pane) View() string {
	if !p.open {
		return ""
	}
	t := p.theme
	headerStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	cursorRowStyle := lipgloss.NewStyle().Background(t.SurfaceStrong).Foreground(t.Text).Bold(true)
	kindStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true)
	errStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1).
		Width(p.width)

	innerW := p.width - 4 // padding 0,1 + border 1+1
	if innerW < 8 {
		innerW = 8
	}

	var body strings.Builder
	header := "outline " + relPathHint(p.path)
	body.WriteString(headerStyle.Render(header))
	body.WriteString("\n")

	if p.errMsg != "" {
		body.WriteString(errStyle.Render("error"))
		body.WriteString("\n")
		body.WriteString(p.errMsg)
		body.WriteString("\n")
		// pad to listHeight so the modal doesn't pop in size when the
		// error path renders versus the loaded path.
		for i := 0; i < p.listHeight()-2; i++ {
			body.WriteString("\n")
		}
		body.WriteString(mutedStyle.Render("esc cancel"))
		return borderStyle.Render(body.String())
	}

	if len(p.symbols) == 0 {
		body.WriteString(mutedStyle.Render("no symbols in this file"))
		body.WriteString("\n")
		for i := 0; i < p.listHeight()-1; i++ {
			body.WriteString("\n")
		}
		body.WriteString(mutedStyle.Render("esc cancel"))
		return borderStyle.Render(body.String())
	}

	// Filter / counter line.
	counter := ""
	if len(p.filter) > 0 {
		counter = " " + mutedStyle.Render("("+itoa(len(p.filtered))+"/"+itoa(len(p.symbols))+")")
	} else {
		counter = " " + mutedStyle.Render("("+itoa(len(p.symbols))+")")
	}
	filterRow := "filter: " + string(p.filter) + cursorBlock(t) + counter
	body.WriteString(filterRow)
	body.WriteString("\n")

	// Symbol list.
	end := p.offset + p.listHeight()
	if end > len(p.filtered) {
		end = len(p.filtered)
	}
	for i := p.offset; i < end; i++ {
		s := p.symbols[p.filtered[i]]
		indent := strings.Repeat("  ", s.Depth)
		kind := s.Kind.Short()
		var line string
		if kind != "" {
			line = indent + s.Name + " " + kindStyle.Render(kind)
		} else {
			line = indent + s.Name
		}
		line = clipTo(line, innerW)
		if i == p.cursor {
			line = padRight(line, innerW)
			line = cursorRowStyle.Render(line)
		}
		body.WriteString(line)
		body.WriteString("\n")
	}
	// Pad to listHeight so the footer always sits at the same row.
	for i := end - p.offset; i < p.listHeight(); i++ {
		body.WriteString("\n")
	}

	body.WriteString(mutedStyle.Render("enter jump • esc cancel • ↑↓ move • type to filter"))
	return borderStyle.Render(body.String())
}

// Flatten depth-first walks the DocSymbol tree and returns a list with
// Depth annotated on each entry. The original document order from the
// server is preserved.
func Flatten(syms []lsp.DocSymbol) []Symbol {
	if len(syms) == 0 {
		return nil
	}
	var out []Symbol
	var walk func(s lsp.DocSymbol, depth int)
	walk = func(s lsp.DocSymbol, depth int) {
		out = append(out, Symbol{
			Name:    s.Name,
			Detail:  s.Detail,
			Kind:    s.Kind,
			Line:    s.Line,
			Col:     s.Col,
			EndLine: s.EndLine,
			Depth:   depth,
		})
		for _, c := range s.Children {
			walk(c, depth+1)
		}
	}
	for _, s := range syms {
		walk(s, 0)
	}
	return out
}

// EnclosingIndex returns the index into syms of the deepest symbol whose
// [Line, EndLine] range contains row, or 0 if no symbol contains it.
// Used by Open to pre-position the cursor on the symbol the user is
// currently editing inside.
func EnclosingIndex(syms []Symbol, row int) int {
	best := 0
	bestDepth := -1
	for i, s := range syms {
		if row < s.Line || row > s.EndLine {
			continue
		}
		if s.Depth > bestDepth {
			best = i
			bestDepth = s.Depth
		}
	}
	return best
}

// enclosingIndexFiltered maps the EnclosingIndex over the filtered list,
// returning the cursor position in the filtered slice. If the chosen
// symbol is filtered out (rare on Open since the filter is empty),
// returns 0.
func enclosingIndexFiltered(syms []Symbol, filtered []int, row int) int {
	target := EnclosingIndex(syms, row)
	for i, idx := range filtered {
		if idx == target {
			return i
		}
	}
	return 0
}

// cursorBlock draws the input caret block.
func cursorBlock(t theme.Theme) string {
	return lipgloss.NewStyle().Background(t.Text).Foreground(t.Bg).Render(" ")
}

// relPathHint returns the basename so the header doesn't carry noise.
func relPathHint(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx == -1 {
		return p
	}
	return p[idx+1:]
}

// clipTo truncates a string to max display columns. Lipgloss renders
// run-length-encoded ANSI; we treat the raw string as if every rune
// were one column, which is fine for the symbol names gopls emits.
// Adds an ellipsis when truncation happens.
func clipTo(s string, max int) string {
	if max < 4 {
		max = 4
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// padRight right-pads s with spaces to width w so a row-wide highlight
// looks like one continuous block. ANSI codes inside s are not stripped
// — lipgloss handles their width when it composes the final frame.
func padRight(s string, w int) string {
	rl := runeLen(s)
	if rl >= w {
		return s
	}
	return s + strings.Repeat(" ", w-rl)
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// itoa is a tiny no-alloc integer-to-string for the counter line.
// Avoids pulling strconv just for the modal's footer.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	n := len(buf)
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		buf[n] = '-'
	}
	return string(buf[n:])
}
