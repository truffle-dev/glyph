// Package table renders a column-aligned data grid with sortable
// headers, keyboard navigation, internal scrolling, and optional row
// selection. It's the primitive every operator surface reaches for once
// a list outgrows a single column: a queue of jobs with status and
// owner, a roster of repos with engagement counts, a sweep of PRs with
// CI state.
//
// A table owns the cursor, the scroll window, the active sort column,
// and an optional "active" header used to steer the next sort. It does
// not own the detail view. The parent reads SelectedRow() to drive the
// panel beside the table, and listens for SelectMsg on Enter when row
// selection is enabled.
package table

import (
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Align controls horizontal alignment of a column's cells. AlignLeft is
// the default and is what string-flavored columns want; AlignRight is
// what numeric columns want; AlignCenter is for narrow status badges.
type Align int

const (
	AlignLeft Align = iota
	AlignRight
	AlignCenter
)

// Column describes one vertical column of the table. Width == 0 means
// auto-fit to the widest content seen across rows (capped at 40 cells)
// and the title. Otherwise Width is treated as the fixed render width
// in cells.
type Column struct {
	Key      string
	Title    string
	Width    int
	Align    Align
	Sortable bool
}

// Row is one record. Cells must line up with Columns by index; missing
// trailing cells render as blank. Value is opaque payload the parent
// can read off SelectedRow() or pull off SelectMsg.
type Row struct {
	Cells []string
	Value any
}

// SelectMsg is emitted on Enter when row selection is enabled.
type SelectMsg struct {
	Row   Row
	Index int
}

// SortMsg is emitted when the active sort changes (either via WithSortBy
// or pressing 's' over a sortable column).
type SortMsg struct {
	ColumnKey  string
	Descending bool
}

// CursorMsg is emitted on cursor movement. The parent can ignore it; it
// exists for surfaces that mirror the cursor in a separate detail pane.
type CursorMsg struct {
	Index int
}

// Model is the table's Bubble Tea model. Construct with New and chain
// builder calls; every method returns a new Model.
type Model struct {
	theme       theme.Theme
	columns     []Column
	rows        []Row
	cursor      int
	offset      int
	activeCol   int
	width       int
	height      int
	sortKey     string
	sortDesc    bool
	rowSelect   bool
	highlight   bool
	hasSize     bool
	placeholder string
}

// minAutoWidth and maxAutoWidth bound the auto-fit algorithm. Below the
// floor headers truncate awkwardly; above the ceiling one fat column
// crushes the rest of the layout.
const (
	minAutoWidth = 4
	maxAutoWidth = 40
)

// New constructs an empty table with safe defaults: cursor highlight
// on, no row selection, a generous 80x12 render box.
func New() Model {
	return Model{
		theme:       theme.Default,
		width:       80,
		height:      12,
		highlight:   true,
		placeholder: "no rows",
	}
}

// WithTheme overrides the palette. Not part of the required API surface
// but useful for stories that swap Light/Dark; kept unexported-of-spec
// would force a fork, so it's exported.
func (m Model) WithTheme(t theme.Theme) Model {
	m.theme = t
	return m
}

// WithColumns replaces the column set. The active sort column is cleared
// if the previous sort key no longer matches any column.
func (m Model) WithColumns(cols ...Column) Model {
	m.columns = append([]Column(nil), cols...)
	if m.activeCol >= len(m.columns) {
		m.activeCol = 0
	}
	if m.sortKey != "" && m.columnIndex(m.sortKey) < 0 {
		m.sortKey = ""
	}
	m.rows = m.applySort(m.rows)
	m.clampCursor()
	return m
}

// WithRows replaces the row set. Cursor clamps to the new length; the
// active sort (if any) is re-applied so callers can hand in unsorted
// data and trust the view.
func (m Model) WithRows(rows ...Row) Model {
	m.rows = append([]Row(nil), rows...)
	m.rows = m.applySort(m.rows)
	m.clampCursor()
	m.clampOffset()
	return m
}

// WithSize sets the render width and total height. Height includes the
// header row plus one separator line, so visible-row count is height-2.
func (m Model) WithSize(width, height int) Model {
	if width < 8 {
		width = 8
	}
	if height < 3 {
		height = 3
	}
	m.width, m.height = width, height
	m.hasSize = true
	m.clampOffset()
	return m
}

// WithSelectedRow positions the cursor. Clamped to [0, len(rows)).
func (m Model) WithSelectedRow(index int) Model {
	if index < 0 {
		index = 0
	}
	if len(m.rows) > 0 && index >= len(m.rows) {
		index = len(m.rows) - 1
	}
	m.cursor = index
	m.clampOffset()
	return m
}

// WithSortBy sets the active sort column by key and direction. Pass
// columnKey == "" to clear the sort. Applies a stable sort immediately.
func (m Model) WithSortBy(columnKey string, descending bool) Model {
	m.sortKey = columnKey
	m.sortDesc = descending
	if columnKey != "" {
		if idx := m.columnIndex(columnKey); idx >= 0 {
			m.activeCol = idx
		}
	}
	m.rows = m.applySort(m.rows)
	return m
}

// WithRowSelection toggles Enter-emits-SelectMsg behavior.
func (m Model) WithRowSelection(enabled bool) Model {
	m.rowSelect = enabled
	return m
}

// WithHighlightCursor toggles the surface highlight applied to the row
// under the cursor. Default true.
func (m Model) WithHighlightCursor(enabled bool) Model {
	m.highlight = enabled
	return m
}

// Cursor returns the current cursor index, or 0 if there are no rows.
func (m Model) Cursor() int { return m.cursor }

// SelectedRow returns the row under the cursor. Bool is false on empty.
func (m Model) SelectedRow() (Row, bool) {
	if len(m.rows) == 0 {
		return Row{}, false
	}
	return m.rows[m.cursor], true
}

// SortBy returns the current sort key and direction. Empty key means no
// active sort.
func (m Model) SortBy() (string, bool) { return m.sortKey, m.sortDesc }

// Rows returns the row set in current sorted order (copy).
func (m Model) Rows() []Row {
	out := make([]Row, len(m.rows))
	copy(out, m.rows)
	return out
}

// Columns returns the column set (copy).
func (m Model) Columns() []Column {
	out := make([]Column, len(m.columns))
	copy(out, m.columns)
	return out
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update routes key and resize messages. Returns (Model, tea.Cmd) to
// match the convention of every other glyph component; the table value
// is also a tea.Model so callers can pass it through tea.Program.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w, h := msg.Width, msg.Height
		if w < 8 {
			w = 8
		}
		if h < 3 {
			h = 3
		}
		m.width, m.height = w, h
		m.hasSize = true
		m.clampOffset()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(key tea.KeyMsg) (Model, tea.Cmd) {
	prevCursor := m.cursor
	visible := m.visibleRowCount()
	switch key.String() {
	case "up", "k":
		m.cursor--
	case "down", "j":
		m.cursor++
	case "pgup":
		m.cursor -= visible
	case "pgdown":
		m.cursor += visible
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(m.rows) - 1
	case "left", "h":
		m.activeCol = m.shiftActiveCol(-1)
	case "right", "l":
		m.activeCol = m.shiftActiveCol(+1)
	case "enter":
		if m.rowSelect && len(m.rows) > 0 {
			row := m.rows[m.cursor]
			idx := m.cursor
			return m, func() tea.Msg { return SelectMsg{Row: row, Index: idx} }
		}
		return m, nil
	case "s":
		return m.toggleSort()
	case "?":
		// Reserved for caller-rendered help. No-op here.
		return m, nil
	default:
		return m, nil
	}
	m.clampCursor()
	m.clampOffset()
	if m.cursor != prevCursor && len(m.rows) > 0 {
		idx := m.cursor
		return m, func() tea.Msg { return CursorMsg{Index: idx} }
	}
	return m, nil
}

// toggleSort flips the sort direction on the active column if the active
// column is already the sort column; otherwise sets ascending sort on
// the active column. No-op when the active column isn't Sortable.
func (m Model) toggleSort() (Model, tea.Cmd) {
	if len(m.columns) == 0 {
		return m, nil
	}
	col := m.columns[m.activeCol]
	if !col.Sortable {
		return m, nil
	}
	if m.sortKey == col.Key {
		m.sortDesc = !m.sortDesc
	} else {
		m.sortKey = col.Key
		m.sortDesc = false
	}
	m.rows = m.applySort(m.rows)
	key := m.sortKey
	desc := m.sortDesc
	return m, func() tea.Msg { return SortMsg{ColumnKey: key, Descending: desc} }
}

// shiftActiveCol moves the active column index, clamping at the ends.
// Wrapping would let the active column drift past the visible header
// edge with no signal; clamping is the conservative choice.
func (m Model) shiftActiveCol(delta int) int {
	next := m.activeCol + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.columns) {
		next = len(m.columns) - 1
	}
	if next < 0 {
		next = 0
	}
	return next
}

// columnIndex returns the index of a column by key, or -1.
func (m Model) columnIndex(key string) int {
	for i, c := range m.columns {
		if c.Key == key {
			return i
		}
	}
	return -1
}

// applySort returns a stably-sorted copy. Empty sort key returns input
// untouched (still copied so callers don't alias the internal slice).
func (m Model) applySort(rows []Row) []Row {
	out := make([]Row, len(rows))
	copy(out, rows)
	if m.sortKey == "" {
		return out
	}
	idx := m.columnIndex(m.sortKey)
	if idx < 0 {
		return out
	}
	desc := m.sortDesc
	sort.SliceStable(out, func(i, j int) bool {
		ai := cellAt(out[i], idx)
		bj := cellAt(out[j], idx)
		return less(ai, bj, desc)
	})
	return out
}

// cellAt safely fetches a row's cell, returning "" if the cell is missing.
func cellAt(r Row, idx int) string {
	if idx < 0 || idx >= len(r.Cells) {
		return ""
	}
	return r.Cells[idx]
}

// less is the comparison used during sort. Both cells parse as numbers
// → numeric compare; otherwise byte-wise lex. Empty strings sort last
// in ascending and first in descending (so the "missing" rows congregate
// out of the way of the data the operator actually scanned for).
func less(a, b string, desc bool) bool {
	aEmpty := a == ""
	bEmpty := b == ""
	if aEmpty && bEmpty {
		return false
	}
	if aEmpty {
		return desc
	}
	if bEmpty {
		return !desc
	}
	af, aok := parseNum(a)
	bf, bok := parseNum(b)
	if aok && bok {
		if desc {
			return af > bf
		}
		return af < bf
	}
	if desc {
		return a > b
	}
	return a < b
}

// parseNum tries to parse a cell as a float. Handles integers, decimals,
// and a leading sign. Anything else (commas, units, dates) falls back to
// lex compare upstream.
func parseNum(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// View renders the header, separator, and visible window of rows. An
// empty row set produces header + separator + a placeholder line.
func (m Model) View() string {
	if len(m.columns) == 0 {
		return lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Italic(true).
			Render("(no columns)")
	}
	widths := m.computeWidths()
	header := m.renderHeader(widths)
	sep := m.renderSeparator(widths)

	if len(m.rows) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Italic(true).
			Render(m.placeholder)
		return strings.Join([]string{header, sep, empty}, "\n")
	}

	visible := m.visibleRowCount()
	end := m.offset + visible
	if end > len(m.rows) {
		end = len(m.rows)
	}
	body := make([]string, 0, end-m.offset)
	for i := m.offset; i < end; i++ {
		body = append(body, m.renderRow(i, widths))
	}
	out := make([]string, 0, len(body)+2)
	out = append(out, header, sep)
	out = append(out, body...)
	return strings.Join(out, "\n")
}

// visibleRowCount is height minus the header and the separator. Always
// at least 1 so a single row is reachable even on a 3-line table.
func (m Model) visibleRowCount() int {
	n := m.height - 2
	if n < 1 {
		n = 1
	}
	return n
}

// renderHeader draws the column titles in bold Primary, with sort and
// active-column adornments.
func (m Model) renderHeader(widths []int) string {
	cells := make([]string, len(m.columns))
	for i, col := range m.columns {
		title := col.Title
		if m.sortKey == col.Key {
			if m.sortDesc {
				title += " v"
			} else {
				title += " ^"
			}
			// Prefer triangles when terminal width is healthy.
			title = strings.ReplaceAll(title, " ^", " "+arrowUp)
			title = strings.ReplaceAll(title, " v", " "+arrowDown)
		}
		style := lipgloss.NewStyle().
			Bold(true).
			Foreground(m.theme.Primary).
			Width(widths[i]).
			Align(alignFor(col.Align))
		rendered := fitCell(title, widths[i])
		if i == m.activeCol {
			style = style.Underline(true)
		}
		cells[i] = style.Render(rendered)
	}
	return strings.Join(cells, " ")
}

// renderSeparator draws a single horizontal rule under the header,
// matching the total content width of the header row.
func (m Model) renderSeparator(widths []int) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strings.Repeat("-", w)
	}
	line := strings.Join(parts, " ")
	return lipgloss.NewStyle().Foreground(m.theme.Border).Render(line)
}

// renderRow draws one data row, applying cursor surface highlight when
// the cursor is on this index and highlight is enabled.
func (m Model) renderRow(idx int, widths []int) string {
	row := m.rows[idx]
	cells := make([]string, len(m.columns))
	textStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
	for i, col := range m.columns {
		raw := cellAt(row, i)
		fitted := fitCell(raw, widths[i])
		cells[i] = textStyle.
			Width(widths[i]).
			Align(alignFor(col.Align)).
			Render(fitted)
	}
	line := strings.Join(cells, " ")
	if idx == m.cursor && m.highlight {
		highlight := lipgloss.NewStyle().
			Background(m.theme.SurfaceStrong).
			Foreground(m.theme.Text).
			Bold(true)
		line = highlight.Render(line)
	}
	return line
}

// computeWidths resolves the rendered width of every column. Fixed
// columns keep their declared widths; auto-fit columns scan the rows.
// If the total would overflow m.width, auto-fit columns shrink first
// (proportionally), then fixed columns truncate from the right.
func (m Model) computeWidths() []int {
	widths := make([]int, len(m.columns))
	autoIdx := make([]int, 0, len(m.columns))
	for i, col := range m.columns {
		if col.Width > 0 {
			widths[i] = col.Width
			continue
		}
		autoIdx = append(autoIdx, i)
		widths[i] = m.autoFit(i, col.Title)
	}
	if !m.hasSize {
		return widths
	}
	// Account for single-space gutters between columns.
	gutters := len(m.columns) - 1
	if gutters < 0 {
		gutters = 0
	}
	total := gutters
	for _, w := range widths {
		total += w
	}
	if total <= m.width {
		return widths
	}
	overflow := total - m.width
	// First, shrink auto-fit columns proportionally.
	if len(autoIdx) > 0 {
		autoTotal := 0
		for _, idx := range autoIdx {
			autoTotal += widths[idx]
		}
		for _, idx := range autoIdx {
			if overflow <= 0 {
				break
			}
			share := overflow * widths[idx] / max(autoTotal, 1)
			if share < 1 {
				share = 1
			}
			if share > widths[idx]-minAutoWidth {
				share = widths[idx] - minAutoWidth
			}
			if share < 0 {
				share = 0
			}
			widths[idx] -= share
			overflow -= share
		}
	}
	// Still over? Shrink fixed columns from the right.
	if overflow > 0 {
		for i := len(widths) - 1; i >= 0 && overflow > 0; i-- {
			room := widths[i] - minAutoWidth
			if room <= 0 {
				continue
			}
			cut := room
			if cut > overflow {
				cut = overflow
			}
			widths[i] -= cut
			overflow -= cut
		}
	}
	return widths
}

// autoFit returns the column width that fits the longest cell in this
// column, the title, plus 2 cells of padding, clamped to [min, max].
func (m Model) autoFit(colIdx int, title string) int {
	maxContent := lipgloss.Width(title)
	for _, r := range m.rows {
		w := lipgloss.Width(cellAt(r, colIdx))
		if w > maxContent {
			maxContent = w
		}
	}
	w := maxContent + 2
	if w > maxAutoWidth {
		w = maxAutoWidth
	}
	if w < minAutoWidth {
		w = minAutoWidth
	}
	return w
}

// clampCursor keeps the cursor inside [0, len(rows)).
func (m *Model) clampCursor() {
	if len(m.rows) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
}

// clampOffset keeps the cursor inside the visible window.
func (m *Model) clampOffset() {
	visible := m.visibleRowCount()
	if visible < 1 {
		visible = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	// Don't scroll past the last row.
	maxOffset := len(m.rows) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

// fitCell truncates with an ellipsis when raw is wider than width.
// Cells equal to or narrower than width pass through; the lipgloss style
// pads them to the rendered width.
func fitCell(raw string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(raw) <= width {
		return raw
	}
	if width == 1 {
		return ellipsis
	}
	// Truncate by runes so multi-byte characters don't shred.
	runes := []rune(raw)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

// alignFor maps the table Align enum to lipgloss positions.
func alignFor(a Align) lipgloss.Position {
	switch a {
	case AlignRight:
		return lipgloss.Right
	case AlignCenter:
		return lipgloss.Center
	default:
		return lipgloss.Left
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

const (
	arrowUp   = "▲" // ▲
	arrowDown = "▼" // ▼
	ellipsis  = "…" // …
)
