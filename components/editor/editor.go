// Package editor renders an editable multi-line text buffer with a 2D
// cursor, viewport scrolling, optional line-number gutter, undo/redo,
// and per-line syntax tint via codeview's tokenizer.
//
// The model is a Bubble Tea component. The buffer is stored as a slice
// of rune-slices so every column index is a rune index, never a byte
// offset; multibyte glyphs and combining marks are handled correctly
// at the cost of treating each rune as one cell (no East-Asian wide-
// rune awareness in v0.1 — that lives in a later widening of runeWidth).
//
// The editor only knows about *text*. Saves, syntax-aware indentation,
// language servers, and selection clipboards are intentionally out of
// scope: the consumer owns the file, the disk, and the OS clipboard.
// The model exposes Value() so the consumer can persist whenever it
// wants and emits no Save/Load messages on its own.
package editor

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	codeview "github.com/truffle-dev/glyph/components/code-view"
	"github.com/truffle-dev/glyph/components/theme"
)

// ChangeMsg is emitted on every edit so the consumer can mark a tab
// "dirty" or schedule an autosave. The cursor position is included so
// the consumer can update breadcrumbs / status-bars in lock-step.
type ChangeMsg struct {
	Value string
	Row   int
	Col   int
}

// Model is the Bubble Tea model for an editable text buffer.
type Model struct {
	theme    theme.Theme
	lines    [][]rune
	row, col int

	width, height int
	viewRow       int // 0-indexed top row of viewport
	viewCol       int // 0-indexed left col of viewport (horizontal scroll)

	focused    bool
	showGutter bool
	language   codeview.Language
	tabSize    int

	undo []op
	redo []op
}

// op is a single reversible edit. The kind field selects which fields
// matter; the inverse is applied in undoOp.
type op struct {
	kind opKind
	row  int
	col  int
	// runes carries the inserted-or-deleted payload for opInsert/opDelete.
	runes []rune
	// joined holds the line that was joined onto row (for opJoin), so the
	// inverse (a split) can re-create it exactly.
	joined []rune
	// prevRow/prevCol are the cursor's pre-edit position, restored on undo.
	prevRow, prevCol int
}

type opKind int

const (
	opInsert opKind = iota
	opDelete
	opSplit // newline at (row, col)
	opJoin  // backspace at (row+1, 0) — joins row+1 onto row at col=len(row)
)

// New returns an editor with one empty line, focused, 80 cells wide,
// 20 rows tall, line numbers on, no language tint, 4-space tabs.
func New(t theme.Theme) Model {
	return Model{
		theme:      t,
		lines:      [][]rune{{}},
		width:      80,
		height:     20,
		focused:    true,
		showGutter: true,
		tabSize:    4,
	}
}

// WithContent presets the buffer. The cursor lands at (0, 0).
func (m Model) WithContent(s string) Model {
	parts := strings.Split(s, "\n")
	m.lines = make([][]rune, len(parts))
	for k, p := range parts {
		m.lines[k] = []rune(p)
	}
	m.row, m.col = 0, 0
	m.viewRow, m.viewCol = 0, 0
	m.undo, m.redo = nil, nil
	return m
}

// WithLanguage selects the tokenizer dialect for non-cursor lines.
func (m Model) WithLanguage(lang codeview.Language) Model { m.language = lang; return m }

// WithWidth clamps to >= 16.
func (m Model) WithWidth(w int) Model {
	if w < 16 {
		w = 16
	}
	m.width = w
	return m
}

// WithHeight clamps to >= 3.
func (m Model) WithHeight(h int) Model {
	if h < 3 {
		h = 3
	}
	m.height = h
	return m
}

// WithGutter toggles the line-number gutter.
func (m Model) WithGutter(on bool) Model { m.showGutter = on; return m }

// WithTabSize sets the tab expansion width. Clamped to [1, 16].
func (m Model) WithTabSize(n int) Model {
	if n < 1 {
		n = 1
	}
	if n > 16 {
		n = 16
	}
	m.tabSize = n
	return m
}

// Focus enables key input.
func (m Model) Focus() Model { m.focused = true; return m }

// Blur disables key input.
func (m Model) Blur() Model { m.focused = false; return m }

// Focused reports whether the editor is currently accepting keys.
func (m Model) Focused() bool { return m.focused }

// Value returns the buffer as "\n"-joined source.
func (m Model) Value() string {
	parts := make([]string, len(m.lines))
	for k, l := range m.lines {
		parts[k] = string(l)
	}
	return strings.Join(parts, "\n")
}

// Cursor returns the 0-based (row, col) cursor position.
func (m Model) Cursor() (int, int) { return m.row, m.col }

// LineCount returns the number of lines in the buffer.
func (m Model) LineCount() int { return len(m.lines) }

// Dirty reports whether the undo stack has at least one entry — useful
// for tab "modified" indicators.
func (m Model) Dirty() bool { return len(m.undo) > 0 }

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	mutated := false
	switch key.Type {
	case tea.KeyLeft:
		m = m.moveLeft()
	case tea.KeyRight:
		m = m.moveRight()
	case tea.KeyUp:
		m = m.moveUp()
	case tea.KeyDown:
		m = m.moveDown()
	case tea.KeyHome:
		m.col = 0
	case tea.KeyEnd:
		m.col = len(m.lines[m.row])
	case tea.KeyPgUp:
		m = m.pageUp()
	case tea.KeyPgDown:
		m = m.pageDown()
	case tea.KeyCtrlHome:
		m.row, m.col = 0, 0
	case tea.KeyCtrlEnd:
		m.row = len(m.lines) - 1
		m.col = len(m.lines[m.row])
	case tea.KeyEnter:
		m = m.newline()
		mutated = true
	case tea.KeyBackspace:
		m, mutated = m.backspace()
	case tea.KeyDelete:
		m, mutated = m.deleteForward()
	case tea.KeyTab:
		m = m.insertRunes([]rune(strings.Repeat(" ", m.tabSize)))
		mutated = true
	case tea.KeyCtrlZ:
		m = m.undoOne()
	case tea.KeyCtrlY:
		m = m.redoOne()
	case tea.KeySpace:
		m = m.insertRunes([]rune{' '})
		mutated = true
	case tea.KeyRunes:
		if len(key.Runes) > 0 {
			m = m.insertRunes(key.Runes)
			mutated = true
		}
	}

	m = m.ensureCursorVisible()

	if mutated {
		val := m.Value()
		r, c := m.row, m.col
		return m, func() tea.Msg { return ChangeMsg{Value: val, Row: r, Col: c} }
	}
	return m, nil
}

// --- edits -------------------------------------------------------------

func (m Model) insertRunes(rs []rune) Model {
	prevRow, prevCol := m.row, m.col
	cur := m.lines[m.row]
	out := make([]rune, 0, len(cur)+len(rs))
	out = append(out, cur[:m.col]...)
	out = append(out, rs...)
	out = append(out, cur[m.col:]...)
	m.lines[m.row] = out
	m.col += len(rs)
	m.redo = nil
	m.undo = append(m.undo, op{
		kind: opInsert, row: prevRow, col: prevCol,
		runes:   append([]rune{}, rs...),
		prevRow: prevRow, prevCol: prevCol,
	})
	return m
}

func (m Model) backspace() (Model, bool) {
	prevRow, prevCol := m.row, m.col
	if m.col > 0 {
		cur := m.lines[m.row]
		removed := cur[m.col-1]
		m.lines[m.row] = append(append([]rune{}, cur[:m.col-1]...), cur[m.col:]...)
		m.col--
		m.redo = nil
		m.undo = append(m.undo, op{
			kind: opDelete, row: m.row, col: m.col,
			runes:   []rune{removed},
			prevRow: prevRow, prevCol: prevCol,
		})
		return m, true
	}
	if m.row > 0 {
		// Join current line onto the previous one at its end.
		prevLen := len(m.lines[m.row-1])
		joined := append([]rune{}, m.lines[m.row]...)
		m.lines[m.row-1] = append(m.lines[m.row-1], m.lines[m.row]...)
		m.lines = append(m.lines[:m.row], m.lines[m.row+1:]...)
		m.row--
		m.col = prevLen
		m.redo = nil
		m.undo = append(m.undo, op{
			kind: opJoin, row: m.row, col: prevLen,
			joined:  joined,
			prevRow: prevRow, prevCol: prevCol,
		})
		return m, true
	}
	return m, false
}

func (m Model) deleteForward() (Model, bool) {
	prevRow, prevCol := m.row, m.col
	cur := m.lines[m.row]
	if m.col < len(cur) {
		removed := cur[m.col]
		m.lines[m.row] = append(append([]rune{}, cur[:m.col]...), cur[m.col+1:]...)
		m.redo = nil
		m.undo = append(m.undo, op{
			kind: opDelete, row: m.row, col: m.col,
			runes:   []rune{removed},
			prevRow: prevRow, prevCol: prevCol,
		})
		return m, true
	}
	if m.row < len(m.lines)-1 {
		joined := append([]rune{}, m.lines[m.row+1]...)
		m.lines[m.row] = append(m.lines[m.row], m.lines[m.row+1]...)
		m.lines = append(m.lines[:m.row+1], m.lines[m.row+2:]...)
		m.redo = nil
		m.undo = append(m.undo, op{
			kind: opJoin, row: m.row, col: prevCol,
			joined:  joined,
			prevRow: prevRow, prevCol: prevCol,
		})
		return m, true
	}
	return m, false
}

func (m Model) newline() Model {
	prevRow, prevCol := m.row, m.col
	cur := m.lines[m.row]
	before := append([]rune{}, cur[:m.col]...)
	after := append([]rune{}, cur[m.col:]...)
	m.lines[m.row] = before
	tail := append([][]rune{after}, m.lines[m.row+1:]...)
	m.lines = append(m.lines[:m.row+1], tail...)
	m.row++
	m.col = 0
	m.redo = nil
	m.undo = append(m.undo, op{
		kind: opSplit, row: prevRow, col: prevCol,
		prevRow: prevRow, prevCol: prevCol,
	})
	return m
}

// --- undo / redo --------------------------------------------------------

func (m Model) undoOne() Model {
	if len(m.undo) == 0 {
		return m
	}
	o := m.undo[len(m.undo)-1]
	m.undo = m.undo[:len(m.undo)-1]
	m = m.applyInverse(o)
	m.redo = append(m.redo, o)
	return m
}

func (m Model) redoOne() Model {
	if len(m.redo) == 0 {
		return m
	}
	o := m.redo[len(m.redo)-1]
	m.redo = m.redo[:len(m.redo)-1]
	m = m.applyForward(o)
	m.undo = append(m.undo, o)
	return m
}

func (m Model) applyInverse(o op) Model {
	switch o.kind {
	case opInsert:
		// Delete len(runes) at (row, col).
		cur := m.lines[o.row]
		m.lines[o.row] = append(append([]rune{}, cur[:o.col]...), cur[o.col+len(o.runes):]...)
	case opDelete:
		// Re-insert runes at (row, col).
		cur := m.lines[o.row]
		out := make([]rune, 0, len(cur)+len(o.runes))
		out = append(out, cur[:o.col]...)
		out = append(out, o.runes...)
		out = append(out, cur[o.col:]...)
		m.lines[o.row] = out
	case opSplit:
		// Undo a newline: join row+1 back onto row.
		m.lines[o.row] = append(m.lines[o.row], m.lines[o.row+1]...)
		m.lines = append(m.lines[:o.row+1], m.lines[o.row+2:]...)
	case opJoin:
		// Undo a join: re-split row at col, then insert joined as row+1.
		cur := m.lines[o.row]
		head := append([]rune{}, cur[:o.col]...)
		m.lines[o.row] = head
		tail := append([][]rune{append([]rune{}, o.joined...)}, m.lines[o.row+1:]...)
		m.lines = append(m.lines[:o.row+1], tail...)
	}
	m.row, m.col = o.prevRow, o.prevCol
	return m
}

func (m Model) applyForward(o op) Model {
	switch o.kind {
	case opInsert:
		cur := m.lines[o.row]
		out := make([]rune, 0, len(cur)+len(o.runes))
		out = append(out, cur[:o.col]...)
		out = append(out, o.runes...)
		out = append(out, cur[o.col:]...)
		m.lines[o.row] = out
		m.row = o.row
		m.col = o.col + len(o.runes)
	case opDelete:
		cur := m.lines[o.row]
		m.lines[o.row] = append(append([]rune{}, cur[:o.col]...), cur[o.col+len(o.runes):]...)
		m.row, m.col = o.row, o.col
	case opSplit:
		cur := m.lines[o.row]
		before := append([]rune{}, cur[:o.col]...)
		after := append([]rune{}, cur[o.col:]...)
		m.lines[o.row] = before
		tail := append([][]rune{after}, m.lines[o.row+1:]...)
		m.lines = append(m.lines[:o.row+1], tail...)
		m.row = o.row + 1
		m.col = 0
	case opJoin:
		m.lines[o.row] = append(m.lines[o.row], m.lines[o.row+1]...)
		m.lines = append(m.lines[:o.row+1], m.lines[o.row+2:]...)
		m.row, m.col = o.row, o.col
	}
	return m
}

// --- cursor motion -----------------------------------------------------

func (m Model) moveLeft() Model {
	if m.col > 0 {
		m.col--
		return m
	}
	if m.row > 0 {
		m.row--
		m.col = len(m.lines[m.row])
	}
	return m
}

func (m Model) moveRight() Model {
	if m.col < len(m.lines[m.row]) {
		m.col++
		return m
	}
	if m.row < len(m.lines)-1 {
		m.row++
		m.col = 0
	}
	return m
}

func (m Model) moveUp() Model {
	if m.row > 0 {
		m.row--
		if m.col > len(m.lines[m.row]) {
			m.col = len(m.lines[m.row])
		}
	}
	return m
}

func (m Model) moveDown() Model {
	if m.row < len(m.lines)-1 {
		m.row++
		if m.col > len(m.lines[m.row]) {
			m.col = len(m.lines[m.row])
		}
	}
	return m
}

func (m Model) pageUp() Model {
	jump := m.height - 1
	if jump < 1 {
		jump = 1
	}
	m.row -= jump
	if m.row < 0 {
		m.row = 0
	}
	if m.col > len(m.lines[m.row]) {
		m.col = len(m.lines[m.row])
	}
	return m
}

func (m Model) pageDown() Model {
	jump := m.height - 1
	if jump < 1 {
		jump = 1
	}
	m.row += jump
	if m.row > len(m.lines)-1 {
		m.row = len(m.lines) - 1
	}
	if m.col > len(m.lines[m.row]) {
		m.col = len(m.lines[m.row])
	}
	return m
}

// ensureCursorVisible scrolls viewRow / viewCol to keep the cursor in
// the visible area.
func (m Model) ensureCursorVisible() Model {
	// Vertical
	if m.row < m.viewRow {
		m.viewRow = m.row
	}
	if m.row >= m.viewRow+m.height {
		m.viewRow = m.row - m.height + 1
	}
	if m.viewRow < 0 {
		m.viewRow = 0
	}

	// Horizontal
	bodyW := m.bodyWidth()
	if bodyW < 1 {
		bodyW = 1
	}
	if m.col < m.viewCol {
		m.viewCol = m.col
	}
	if m.col >= m.viewCol+bodyW {
		m.viewCol = m.col - bodyW + 1
	}
	if m.viewCol < 0 {
		m.viewCol = 0
	}
	return m
}

func (m Model) gutterWidth() int {
	if !m.showGutter {
		return 0
	}
	// gutter renders " NNN " — log10(lineCount) + 1 digits, plus one space
	// pad on each side.
	d := digits(len(m.lines))
	if d < 2 {
		d = 2
	}
	return d + 2
}

func (m Model) bodyWidth() int {
	// Frame: 1 left border + 1 left pad + body + 1 right pad + 1 right border.
	w := m.width - 4 - m.gutterWidth()
	if w < 1 {
		return 1
	}
	return w
}

// View renders the editor as a bordered frame containing the gutter and
// the visible slice of the buffer with the cursor injected on its line.
func (m Model) View() string {
	bodyW := m.bodyWidth()
	gutterW := m.gutterWidth()

	rows := make([]string, m.height)
	for i := 0; i < m.height; i++ {
		bufRow := m.viewRow + i
		if bufRow >= len(m.lines) {
			rows[i] = m.emptyRow(gutterW, bodyW)
			continue
		}
		isCursor := bufRow == m.row && m.focused
		rows[i] = m.renderRow(bufRow, gutterW, bodyW, isCursor)
	}

	body := strings.Join(rows, "\n")
	border := m.theme.Border
	if m.focused {
		border = m.theme.PrimaryStrong
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(m.width).
		Render(body)
}

func (m Model) emptyRow(gutterW, bodyW int) string {
	gutter := ""
	if gutterW > 0 {
		gutter = lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Render(strings.Repeat(" ", gutterW))
	}
	pad := strings.Repeat(" ", bodyW)
	return gutter + pad
}

func (m Model) renderRow(bufRow, gutterW, bodyW int, isCursor bool) string {
	raw := m.lines[bufRow]

	// Slice the horizontal viewport window in rune-space.
	start := m.viewCol
	if start > len(raw) {
		start = len(raw)
	}
	end := start + bodyW
	if end > len(raw) {
		end = len(raw)
	}
	visible := raw[start:end]
	pad := bodyW - (end - start)
	if pad < 0 {
		pad = 0
	}

	// Gutter
	gutter := ""
	if gutterW > 0 {
		num := strconv.Itoa(bufRow + 1)
		gutterText := strings.Repeat(" ", gutterW-1-len(num)) + num + " "
		style := lipgloss.NewStyle().Foreground(m.theme.TextMuted)
		if isCursor {
			style = style.Foreground(m.theme.Text).Background(m.theme.SurfaceStrong)
		}
		gutter = style.Render(gutterText)
	}

	// Body
	var body string
	if isCursor {
		body = m.renderCursorLine(visible, start, pad)
	} else if m.language != codeview.LangPlain {
		body = codeview.Tokenize(string(visible), m.language) + strings.Repeat(" ", pad)
	} else {
		body = lipgloss.NewStyle().
			Foreground(m.theme.Text).
			Render(string(visible)) + strings.Repeat(" ", pad)
	}

	return gutter + body
}

// renderCursorLine renders the cursor row with the cursor cell inverted
// at the right column. visible is the rune-slice clipped to the
// horizontal viewport; start is the rune offset into the source line.
func (m Model) renderCursorLine(visible []rune, start, pad int) string {
	textStyle := lipgloss.NewStyle().
		Foreground(m.theme.Text).
		Background(m.theme.SurfaceStrong)
	cursorStyle := lipgloss.NewStyle().
		Foreground(m.theme.Bg).
		Background(m.theme.Text)

	relCol := m.col - start
	if relCol < 0 {
		relCol = 0
	}
	if relCol > len(visible) {
		relCol = len(visible)
	}

	before := textStyle.Render(string(visible[:relCol]))
	cursorCh := " "
	afterStart := relCol
	if relCol < len(visible) {
		cursorCh = string(visible[relCol])
		afterStart = relCol + 1
	}
	cursor := cursorStyle.Render(cursorCh)
	after := textStyle.Render(string(visible[afterStart:]))
	tail := textStyle.Render(strings.Repeat(" ", pad))
	return before + cursor + after + tail
}

// digits returns the count of decimal digits in n (n >= 0); 0 returns 1.
func digits(n int) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for n > 0 {
		n /= 10
		d++
	}
	return d
}
