// Package tablevirtualized renders a column-aligned data table over a
// row source that may be much larger than memory. Where table reaches
// for a []Row, table-virtualized reaches for a RowProvider — an
// interface the caller satisfies with a slice wrapper, a database
// cursor, a memory-mapped file, or anything else with Len + At
// semantics. Render cost is O(visible rows), independent of Len, which
// is the point: an operator can scroll through ten million log lines
// without the table ever materializing more than a screen at a time.
//
// The trade for the size is that the caller owns more invariants.
// Columns must declare explicit widths (no auto-fit scan over the
// source). Sort is the caller's job (no in-memory shuffle of a row set
// you may not be able to materialize). The component does what only it
// can do — manage the cursor, the offset window, the scroll affordance,
// and the render of the visible band.
package tablevirtualized

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Align controls horizontal alignment of a column's cells.
type Align int

const (
	AlignLeft Align = iota
	AlignRight
	AlignCenter
)

// Column describes one vertical column. Width is required and must be
// positive; auto-fit is intentionally absent because scanning Len rows
// to compute it defeats the virtualization. AlignLeft is the default
// for string-flavored columns; AlignRight for numeric.
type Column struct {
	Key   string
	Title string
	Width int
	Align Align
}

// Row is one record. Cells line up with Columns by index; missing
// trailing cells render as blank. Value is opaque payload the caller
// can pull off SelectedRow().
type Row struct {
	Cells []string
	Value any
}

// RowProvider is the source of rows. Implementations need only know how
// many rows exist (Len) and how to surface one by index (At). The model
// calls At only for rows in the visible window, so a streaming or
// disk-backed implementation never materializes more than ~height rows.
type RowProvider interface {
	Len() int
	At(i int) Row
}

// SliceProvider adapts a plain []Row to RowProvider for callers whose
// data already fits in memory but still want the virtualized cursor +
// scroll semantics. Zero-cost wrapper.
type SliceProvider []Row

// Len returns the number of rows.
func (s SliceProvider) Len() int { return len(s) }

// At returns the row at index i.
func (s SliceProvider) At(i int) Row { return s[i] }

// SelectMsg is emitted on Enter when row selection is enabled.
type SelectMsg struct {
	Row   Row
	Index int
}

// CursorMsg is emitted on cursor movement. Surfaces with a detail pane
// beside the table can subscribe to this without polling Cursor().
type CursorMsg struct {
	Index int
}

// Model is the table's Bubble Tea model.
type Model struct {
	theme       theme.Theme
	columns     []Column
	rows        RowProvider
	cursor      int
	offset      int
	width       int
	height      int
	rowSelect   bool
	highlight   bool
	hasSize     bool
	placeholder string
}

// New constructs an empty virtualized table with safe defaults: cursor
// highlight on, no row selection, a generous 80x12 render box.
func New() Model {
	return Model{
		theme:       theme.Default,
		width:       80,
		height:      12,
		highlight:   true,
		placeholder: "no rows",
	}
}

// WithTheme overrides the palette.
func (m Model) WithTheme(t theme.Theme) Model {
	m.theme = t
	return m
}

// WithColumns replaces the column set. Columns with non-positive Width
// are clamped to 4 cells so a misconfigured column doesn't collapse to
// zero width during render.
func (m Model) WithColumns(cols ...Column) Model {
	out := make([]Column, len(cols))
	for i, c := range cols {
		if c.Width < 4 {
			c.Width = 4
		}
		out[i] = c
	}
	m.columns = out
	return m
}

// WithRows binds a RowProvider. Cursor clamps to the new length.
func (m Model) WithRows(rows RowProvider) Model {
	m.rows = rows
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

// WithSelectedRow positions the cursor. Clamped to [0, Len).
func (m Model) WithSelectedRow(index int) Model {
	if index < 0 {
		index = 0
	}
	if n := m.rowsLen(); n > 0 && index >= n {
		index = n - 1
	}
	m.cursor = index
	m.clampOffset()
	return m
}

// WithRowSelection toggles Enter-emits-SelectMsg behavior.
func (m Model) WithRowSelection(enabled bool) Model {
	m.rowSelect = enabled
	return m
}

// WithHighlightCursor toggles the surface highlight applied to the row
// under the cursor.
func (m Model) WithHighlightCursor(enabled bool) Model {
	m.highlight = enabled
	return m
}

// Cursor returns the current cursor index, or 0 if there are no rows.
func (m Model) Cursor() int { return m.cursor }

// SelectedRow returns the row under the cursor. Bool is false on empty.
func (m Model) SelectedRow() (Row, bool) {
	if m.rowsLen() == 0 {
		return Row{}, false
	}
	return m.rows.At(m.cursor), true
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update routes key and resize messages.
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
		m.cursor = m.rowsLen() - 1
	case "enter":
		if m.rowSelect && m.rowsLen() > 0 {
			row := m.rows.At(m.cursor)
			idx := m.cursor
			return m, func() tea.Msg { return SelectMsg{Row: row, Index: idx} }
		}
		return m, nil
	default:
		return m, nil
	}
	m.clampCursor()
	m.clampOffset()
	if m.cursor != prevCursor && m.rowsLen() > 0 {
		idx := m.cursor
		return m, func() tea.Msg { return CursorMsg{Index: idx} }
	}
	return m, nil
}

// View renders the header, separator, scroll affordance, and visible
// window of rows.
func (m Model) View() string {
	if len(m.columns) == 0 {
		return lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Italic(true).
			Render("(no columns)")
	}
	widths := m.columnWidths()
	header := m.renderHeader(widths)
	sep := m.renderSeparator(widths)

	if m.rowsLen() == 0 {
		empty := lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Italic(true).
			Render(m.placeholder)
		return strings.Join([]string{header, sep, empty}, "\n")
	}

	visible := m.visibleRowCount()
	end := m.offset + visible
	total := m.rowsLen()
	if end > total {
		end = total
	}
	body := make([]string, 0, end-m.offset)
	for i := m.offset; i < end; i++ {
		body = append(body, m.renderRow(i, widths))
	}
	// Scroll affordance: replace the first body line with an up-indicator
	// when more rows exist above the window, and the last with a
	// down-indicator when more rows exist below. The affordance overwrites
	// content rather than adding a line so render height is stable.
	if m.offset > 0 && len(body) > 0 {
		body[0] = m.renderScrollIndicator(arrowUp, widths)
	}
	if end < total && len(body) > 0 {
		body[len(body)-1] = m.renderScrollIndicator(arrowDown, widths)
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

// renderHeader draws the column titles in bold Primary.
func (m Model) renderHeader(widths []int) string {
	cells := make([]string, len(m.columns))
	for i, col := range m.columns {
		style := lipgloss.NewStyle().
			Bold(true).
			Foreground(m.theme.Primary).
			Width(widths[i]).
			Align(alignFor(col.Align))
		cells[i] = style.Render(fitCell(col.Title, widths[i]))
	}
	return strings.Join(cells, " ")
}

// renderSeparator draws a single horizontal rule under the header.
func (m Model) renderSeparator(widths []int) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strings.Repeat("-", w)
	}
	line := strings.Join(parts, " ")
	return lipgloss.NewStyle().Foreground(m.theme.Border).Render(line)
}

// renderRow draws one data row at the given index.
func (m Model) renderRow(idx int, widths []int) string {
	row := m.rows.At(idx)
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

// renderScrollIndicator draws a centered arrow across the row width.
// Replaces the first or last visible row when there's content outside
// the window.
func (m Model) renderScrollIndicator(glyph string, widths []int) string {
	total := 0
	for _, w := range widths {
		total += w
	}
	total += len(widths) - 1 // gutters
	if total < 1 {
		total = 1
	}
	style := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted).
		Width(total).
		Align(lipgloss.Center)
	return style.Render(glyph)
}

// columnWidths returns the slice of declared widths. No auto-fit pass;
// virtualization requires caller-declared widths.
func (m Model) columnWidths() []int {
	widths := make([]int, len(m.columns))
	for i, col := range m.columns {
		widths[i] = col.Width
	}
	return widths
}

// rowsLen safely reads the provider length, treating a nil provider as 0.
func (m Model) rowsLen() int {
	if m.rows == nil {
		return 0
	}
	return m.rows.Len()
}

// clampCursor keeps the cursor inside [0, Len).
func (m *Model) clampCursor() {
	n := m.rowsLen()
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= n {
		m.cursor = n - 1
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
	n := m.rowsLen()
	maxOffset := n - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

// cellAt safely fetches a row's cell, returning "" if the cell is missing.
func cellAt(r Row, idx int) string {
	if idx < 0 || idx >= len(r.Cells) {
		return ""
	}
	return r.Cells[idx]
}

// fitCell truncates with an ellipsis when raw is wider than width.
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
	runes := []rune(raw)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

// alignFor maps the Align enum to lipgloss positions.
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

const (
	arrowUp   = "▲"
	arrowDown = "▼"
	ellipsis  = "…"
)
