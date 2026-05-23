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

	"github.com/truffle-dev/glyph/components/theme"
)

// SavedMsg is emitted after a successful save.
type SavedMsg struct {
	Path string
	Err  error
}

// CancelMsg is emitted on Esc when not editing inline.
type CancelMsg struct{}

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
}

// NewPane constructs an empty pane.
func NewPane(t theme.Theme) Pane {
	return Pane{theme: t, buf: NewBuffer(), width: 80, height: 24}
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

// Open replaces the buffer with the contents of path.
func (p Pane) Open(path string) Pane {
	b, err := Load(path)
	p.buf = b
	p.path = path
	p.row, p.col, p.offset = 0, 0, 0
	p.err = err
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
	return p, nil
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
	p.ensureVisible()
}

func (p *Pane) delBack() {
	if p.col > 0 {
		line := p.buf.Lines[p.row]
		p.buf.Lines[p.row] = line[:p.col-1] + line[p.col:]
		p.col--
		p.buf.Dirty = true
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
	p.ensureVisible()
}

func (p *Pane) delForward() {
	line := p.buf.Lines[p.row]
	if p.col < len(line) {
		p.buf.Lines[p.row] = line[:p.col] + line[p.col+1:]
		p.buf.Dirty = true
		return
	}
	if p.row >= len(p.buf.Lines)-1 {
		return
	}
	p.buf.Lines[p.row] = line + p.buf.Lines[p.row+1]
	p.buf.Lines = append(p.buf.Lines[:p.row+1], p.buf.Lines[p.row+2:]...)
	p.buf.Dirty = true
}

// View renders the pane.
func (p Pane) View() string {
	t := p.theme
	gutterStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	cursorLineGutterStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
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
		line := p.buf.Lines[i]
		// expand tabs to 4 spaces for display
		line = strings.ReplaceAll(line, "\t", "    ")
		if i == p.row {
			// Budget the visible characters first, then style. We allow the
			// ghost preview to consume the remaining width after the prefix.
			cursorCol := p.col
			ghost := ""
			if p.ghostText != "" && p.focused {
				ghost = p.ghostText
			}
			line = renderCursorRowClipped(line, cursorCol, ghost, contentWidth, t)
		} else {
			line = clip(line, contentWidth)
		}
		rows = append(rows, gut+" │ "+line)
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
