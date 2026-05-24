// Package editor is nook's minimal file viewer/editor pane.
//
// MVP scope: a read-mostly buffer with insert/delete editing, cursor
// movement, scroll, save. Syntax highlighting is intentionally deferred
// (chroma is heavy and we'd rather earn it). Selections, search, undo,
// and multi-cursor are deferred to Phase 2.
//
// Buffer is line-oriented (a []string of lines). Coordinates are
// 0-based internally; the gutter renders 1-based line numbers.
package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/cmd/nook/internal/gitgutter"
	"github.com/truffle-dev/glyph/cmd/nook/internal/highlight"
	"github.com/truffle-dev/glyph/components/theme"
)

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

	// Syntax-highlight cache. spans is the result of the last highlight pass
	// over the buffer; bufVer is a monotonically-increasing counter bumped by
	// any mutation; hlVer records the bufVer the cache was computed from.
	// applyHighlight re-runs the highlighter only when bufVer != hlVer.
	highlighter highlight.Highlighter
	spans       highlight.Result
	bufVer      int
	hlVer       int

	// searchMatches paints byte ranges with a background highlight; the host
	// updates it from the finder overlay. searchCurrent is the index of the
	// currently active match within searchMatches (or -1) so the active hit
	// gets a stronger style than the rest.
	searchMatches []Range
	searchCurrent int
}

// Range is a byte-range mark inside a single row (used for search matches).
type Range struct {
	Row   int
	Start int // 0-based byte column, inclusive
	End   int // 0-based byte column, exclusive
}

// NewPane constructs an empty pane.
func NewPane(t theme.Theme) Pane {
	return Pane{theme: t, buf: NewBuffer(), width: 80, height: 24, searchCurrent: -1}
}

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
	visible := p.height - 1
	if visible < 1 {
		visible = 1
	}
	if p.row < p.offset {
		p.offset = p.row
	}
	if p.row >= p.offset+visible {
		p.offset = p.row - visible + 1
	}
}

// Update handles keys.
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch km.Type {
	case tea.KeyEsc:
		return p, func() tea.Msg { return CancelMsg{} }
	case tea.KeyCtrlS:
		return p, p.SaveCmd()
	case tea.KeyUp:
		if p.row > 0 {
			p.row--
			p.clampCol()
		}
		p.ensureVisible()
	case tea.KeyDown:
		if p.row < len(p.buf.Lines)-1 {
			p.row++
			p.clampCol()
		}
		p.ensureVisible()
	case tea.KeyLeft:
		if p.col > 0 {
			p.col--
		} else if p.row > 0 {
			p.row--
			p.col = len(p.buf.Lines[p.row])
			p.ensureVisible()
		}
	case tea.KeyRight:
		if p.col < len(p.buf.Lines[p.row]) {
			p.col++
		} else if p.row < len(p.buf.Lines)-1 {
			p.row++
			p.col = 0
			p.ensureVisible()
		}
	case tea.KeyHome, tea.KeyCtrlA:
		p.col = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		p.col = len(p.buf.Lines[p.row])
	case tea.KeyPgUp:
		jump := p.height - 2
		if jump < 1 {
			jump = 1
		}
		p.row -= jump
		if p.row < 0 {
			p.row = 0
		}
		p.clampCol()
		p.ensureVisible()
	case tea.KeyPgDown:
		jump := p.height - 2
		if jump < 1 {
			jump = 1
		}
		p.row += jump
		if p.row >= len(p.buf.Lines) {
			p.row = len(p.buf.Lines) - 1
		}
		p.clampCol()
		p.ensureVisible()
	case tea.KeyBackspace:
		p.delBack()
	case tea.KeyDelete:
		p.delForward()
	case tea.KeyEnter:
		p.insertNewline()
	case tea.KeyTab:
		p.insertRunes([]rune{'\t'})
	case tea.KeySpace:
		p.insertRunes([]rune{' '})
	case tea.KeyRunes:
		p.insertRunes(km.Runes)
	}
	(&p).applyHighlight()
	return p, nil
}

// applyHighlight re-runs the highlighter when the buffer version has advanced
// past the last cached version. Called from any path that mutates the buffer.
// Safe to call when highlighter is nil; it just returns.
func (p *Pane) applyHighlight() {
	if p.highlighter == nil {
		p.spans = highlight.Result{}
		return
	}
	if p.bufVer == p.hlVer {
		return
	}
	src := strings.Join(p.buf.Lines, "\n")
	p.spans = p.highlighter.Highlight(p.path, src)
	p.hlVer = p.bufVer
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
	contentWidth := p.width - gw - 2
	if contentWidth < 1 {
		contentWidth = 1
	}

	var rows []string
	for i := p.offset; i < end; i++ {
		num := fmt.Sprintf("%*d", gw, i+1)
		var gut string
		if i == p.row {
			gut = cursorLineGutterStyle.Render(num)
		} else {
			gut = gutterStyle.Render(num)
		}
		// Two-character marker column. First char carries the git-diff state
		// (added / modified / deleted-above); second char carries the worst
		// LSP diagnostic. Either can be empty (space + │) so the column width
		// stays fixed regardless of which signals are active.
		gitChar := " "
		switch p.LineMarkerAt(i) {
		case gitgutter.Added:
			gitChar = gitAddStyle.Render("▎")
		case gitgutter.Modified:
			gitChar = gitModStyle.Render("▎")
		case gitgutter.DeletedAbove:
			gitChar = gitDelStyle.Render("▔")
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
		var line string
		if i == p.row {
			// Budget the visible characters first, then style. We allow the
			// ghost preview to consume the remaining width after the prefix.
			cursorCol := p.col
			ghost := ""
			if p.ghostText != "" && p.focused {
				ghost = p.ghostText
			}
			line = renderHighlightedRow(raw, spans, marks, activeIdx, cursorCol, ghost, contentWidth, t, true)
		} else {
			line = renderHighlightedRow(raw, spans, marks, activeIdx, -1, "", contentWidth, t, false)
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

func renderHighlightedRow(raw string, spans []highlight.Span, matches []Range, activeMatch int, cursorCol int, ghost string, width int, t theme.Theme, drawCursor bool) string {
	cur := lipgloss.NewStyle().Foreground(t.TextInverse).Background(t.Primary)
	muted := lipgloss.NewStyle().Foreground(t.TextMuted).Faint(true)
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
	palette := syntaxPalette(t)

	// Pre-expand tabs and build a raw-byte -> display-column index so spans
	// and the cursor map cleanly into expanded coords. expandedRunes holds
	// the runes we'll emit; runeKind[i] is the Kind backing expandedRunes[i];
	// runeMatch[i] tags whether the rune is inside a finder match (and which).
	var expanded []rune
	var runeKind []highlight.Kind
	var runeMatch []matchKind
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
		if c == '\t' {
			for k := 0; k < 4; k++ {
				expanded = append(expanded, ' ')
				runeKind = append(runeKind, highlight.KindPlain)
				runeMatch = append(runeMatch, mk)
			}
			continue
		}
		// Standard byte. For multi-byte UTF-8 we still emit per-rune below;
		// here we just count the leading byte. We append the rune later via
		// rune iteration to keep widths sane.
		expanded = append(expanded, rune(c))
		runeKind = append(runeKind, kind)
		runeMatch = append(runeMatch, mk)
	}
	rawToExpStart[len(raw)] = len(expanded)

	// Convert raw cursor col to a display column (rune index in expanded).
	dispCursor := -1
	if drawCursor {
		if cursorCol < 0 {
			cursorCol = 0
		}
		if cursorCol >= len(raw) {
			dispCursor = len(expanded)
		} else {
			dispCursor = rawToExpStart[cursorCol]
		}
	}

	ghostRunes := []rune(ghost)
	// Width budget: visible content + (1 if endCursor) + (1 if ellipsis).
	endCursor := drawCursor && dispCursor >= len(expanded) && len(ghostRunes) == 0
	totalCells := len(expanded) + len(ghostRunes)
	if endCursor {
		totalCells++
	}
	clipped := false
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
			ghostRunes = nil
		} else {
			remain := budget - len(expanded)
			if remain < len(ghostRunes) {
				ghostRunes = ghostRunes[:remain]
			}
		}
	}

	// Emit. Batch contiguous runes of the same (kind, matchKind) pair into one
	// Style.Render call — per-rune emission inflates ANSI overhead and breaks
	// substring asserts in tests because each rune gets surrounded by escape
	// codes. The cursor cell breaks the run and is emitted separately.
	var b strings.Builder
	flush := func(buf []rune, k highlight.Kind, m matchKind) {
		if len(buf) == 0 {
			return
		}
		switch m {
		case matchActive:
			b.WriteString(matchActiveBG.Render(string(buf)))
			return
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
	var run []rune
	var runKind highlight.Kind
	var runMatch matchKind
	for i, r := range expanded {
		if drawCursor && i == dispCursor {
			flush(run, runKind, runMatch)
			run = nil
			b.WriteString(cur.Render(string(r)))
			continue
		}
		k := runeKind[i]
		var m matchKind
		if i < len(runeMatch) {
			m = runeMatch[i]
		}
		if len(run) > 0 && (k != runKind || m != runMatch) {
			flush(run, runKind, runMatch)
			run = run[:0]
		}
		runKind = k
		runMatch = m
		run = append(run, r)
	}
	flush(run, runKind, runMatch)
	// Ghost text rendered next; first rune cursor-styled if the cursor is at
	// EOL (no rune under it), else all muted.
	if len(ghostRunes) > 0 {
		if drawCursor && dispCursor >= len(expanded) {
			b.WriteString(cur.Render(string(ghostRunes[0])))
			if len(ghostRunes) > 1 {
				b.WriteString(muted.Render(string(ghostRunes[1:])))
			}
		} else {
			b.WriteString(muted.Render(string(ghostRunes)))
		}
	} else if drawCursor && dispCursor >= len(expanded) {
		// Cursor at EOL with no ghost: paint a space cursor.
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
	}
}

func clip(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return string(r[:w])
	}
	return string(r[:w-1]) + "…"
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
