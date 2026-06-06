// Package timeline renders a vertical sequence of events with optional
// timestamps, status dots, and multi-line bodies. The model handles
// cursor, scroll window, and keyboard navigation.
package timeline

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Status colors the dot rendered next to each event title.
type Status int

const (
	StatusNeutral Status = iota
	StatusSuccess
	StatusWarning
	StatusError
	StatusInfo
)

// Event is one row on the timeline. Time is pre-formatted; the caller
// controls the format (so the same component renders absolute clocks,
// relative durations, or git-style "3 days ago" identically). Body may
// contain newlines.
type Event struct {
	Time   string
	Title  string
	Body   string
	Status Status
	Value  any
}

// SelectMsg fires when the user presses Enter on the selected event.
type SelectMsg struct {
	Event Event
	Index int
}

// CursorMsg fires each time the cursor moves to a new event.
type CursorMsg struct {
	Index int
}

const (
	dotGlyph       = "●"
	connectorGlyph = "│"
	minTimeCol     = 1
	maxTimeCol     = 20
)

// Model is the Bubble Tea model.
type Model struct {
	th          theme.Theme
	events      []Event
	cursor      int
	offset      int
	width       int
	height      int
	timeCol     int
	showTime    bool
	hasSize     bool
	highlight   bool
	placeholder string
}

// New constructs a Model with Default theme, width 60, height 12.
func New() Model {
	return Model{
		th:          theme.Default,
		width:       60,
		height:      12,
		timeCol:     8,
		showTime:    true,
		highlight:   true,
		placeholder: "no events",
	}
}

// WithTheme overrides the theme.
func (m Model) WithTheme(t theme.Theme) Model {
	m.th = t
	return m
}

// WithEvents replaces the event list and clamps the cursor.
func (m Model) WithEvents(events ...Event) Model {
	m.events = events
	m.timeCol = computeTimeCol(events, m.showTime)
	m.clampCursor()
	m.clampOffset()
	return m
}

// WithSize sets the rendered width and visible-row height. Height counts
// rendered lines, not events; a tall event with a multi-line body still
// counts every line toward the height budget.
func (m Model) WithSize(w, h int) Model {
	if w < 20 {
		w = 20
	}
	if h < 3 {
		h = 3
	}
	m.width = w
	m.height = h
	m.hasSize = true
	m.clampOffset()
	return m
}

// WithSelectedEvent places the cursor at i. Out-of-range is clamped.
func (m Model) WithSelectedEvent(i int) Model {
	m.cursor = i
	m.clampCursor()
	m.ensureCursorVisible()
	return m
}

// WithTimeColumn toggles the timestamp column. Hidden timestamps
// reclaim the column for title space.
func (m Model) WithTimeColumn(show bool) Model {
	m.showTime = show
	m.timeCol = computeTimeCol(m.events, show)
	return m
}

// WithHighlightCursor toggles the highlighted title row at the cursor.
// On by default.
func (m Model) WithHighlightCursor(on bool) Model {
	m.highlight = on
	return m
}

// Cursor returns the index of the selected event, or 0 when empty.
func (m Model) Cursor() int { return m.cursor }

// SelectedEvent returns the event under the cursor and true when the
// list is non-empty.
func (m Model) SelectedEvent() (Event, bool) {
	if len(m.events) == 0 {
		return Event{}, false
	}
	return m.events[m.cursor], true
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update handles window size and keyboard input.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.WithSize(msg.Width, msg.Height), nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if len(m.events) == 0 {
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		return m.moveCursor(-1), nil
	case "down", "j":
		return m.moveCursor(1), nil
	case "pgup":
		return m.moveCursor(-pageStep(m)), nil
	case "pgdown":
		return m.moveCursor(pageStep(m)), nil
	case "home", "g":
		return m.moveCursorTo(0), nil
	case "end", "G":
		return m.moveCursorTo(len(m.events) - 1), nil
	case "enter":
		ev := m.events[m.cursor]
		i := m.cursor
		return m, func() tea.Msg { return SelectMsg{Event: ev, Index: i} }
	}
	return m, nil
}

func pageStep(m Model) int {
	step := m.height / 2
	if step < 1 {
		step = 1
	}
	return step
}

func (m Model) moveCursor(delta int) Model {
	return m.moveCursorTo(m.cursor + delta)
}

func (m Model) moveCursorTo(i int) Model {
	if i < 0 {
		i = 0
	}
	if i >= len(m.events) {
		i = len(m.events) - 1
	}
	m.cursor = i
	m.ensureCursorVisible()
	return m
}

func (m *Model) clampCursor() {
	if len(m.events) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.events) {
		m.cursor = len(m.events) - 1
	}
}

func (m *Model) clampOffset() {
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset >= len(m.events) {
		if len(m.events) == 0 {
			m.offset = 0
		} else {
			m.offset = len(m.events) - 1
		}
	}
}

func (m *Model) ensureCursorVisible() {
	if m.cursor < m.offset {
		m.offset = m.cursor
		return
	}
	visible := m.visibleCount()
	if visible <= 0 {
		visible = 1
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
		if m.offset < 0 {
			m.offset = 0
		}
	}
}

// visibleCount returns how many events fit in the height budget starting
// at the current offset. Each event consumes 1 + bodyLines + 1 connector
// line (the last event drops its connector).
func (m Model) visibleCount() int {
	if m.height <= 0 || len(m.events) == 0 {
		return 0
	}
	budget := m.height
	count := 0
	for i := m.offset; i < len(m.events); i++ {
		lines := eventLineCount(m.events[i])
		if i < len(m.events)-1 {
			lines++ // connector
		}
		if budget-lines < 0 {
			if count == 0 {
				return 1
			}
			break
		}
		budget -= lines
		count++
	}
	return count
}

func eventLineCount(e Event) int {
	if e.Body == "" {
		return 1
	}
	return 1 + strings.Count(e.Body, "\n") + 1
}

// View renders header-less, with timestamps in the gutter and a dot
// column running down the side. Returns the placeholder when empty.
func (m Model) View() string {
	if len(m.events) == 0 {
		return lipgloss.NewStyle().
			Foreground(m.th.TextMuted).
			Italic(true).
			Width(m.width).
			Render(m.placeholder)
	}

	visible := m.visibleCount()
	if visible == 0 {
		return ""
	}

	timeWidth := m.timeCol
	if !m.showTime {
		timeWidth = 0
	}
	gutter := 2 // dot + space
	titleWidth := m.width - timeWidth - gutter
	if !m.showTime {
		titleWidth = m.width - gutter
	}
	if titleWidth < 1 {
		titleWidth = 1
	}

	var b strings.Builder
	for off := 0; off < visible; off++ {
		i := m.offset + off
		ev := m.events[i]
		b.WriteString(m.renderEventHead(ev, timeWidth, titleWidth, i == m.cursor))
		b.WriteString("\n")
		bodyLines := bodyLines(ev.Body)
		for _, line := range bodyLines {
			b.WriteString(m.renderBodyLine(line, timeWidth, titleWidth))
			b.WriteString("\n")
		}
		if i < len(m.events)-1 && off < visible-1 {
			b.WriteString(m.renderConnector(timeWidth))
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderEventHead(ev Event, timeWidth, titleWidth int, selected bool) string {
	var parts []string
	if m.showTime {
		t := truncCell(ev.Time, timeWidth)
		parts = append(parts, lipgloss.NewStyle().
			Foreground(m.th.TextMuted).
			Width(timeWidth).
			Render(t))
	}
	parts = append(parts, m.styleDot(ev.Status).Render(dotGlyph)+" ")
	title := truncCell(ev.Title, titleWidth)
	titleStyle := lipgloss.NewStyle().
		Foreground(m.th.Text).
		Width(titleWidth)
	if selected && m.highlight {
		titleStyle = titleStyle.
			Background(m.th.SurfaceStrong).
			Foreground(m.th.Text).
			Bold(true)
	}
	parts = append(parts, titleStyle.Render(title))
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m Model) renderBodyLine(line string, timeWidth, titleWidth int) string {
	var parts []string
	if m.showTime {
		parts = append(parts, lipgloss.NewStyle().Width(timeWidth).Render(""))
	}
	parts = append(parts,
		lipgloss.NewStyle().
			Foreground(m.th.Border).
			Render(connectorGlyph+" "))
	parts = append(parts,
		lipgloss.NewStyle().
			Foreground(m.th.TextMuted).
			Width(titleWidth).
			Render(truncCell(line, titleWidth)))
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m Model) renderConnector(timeWidth int) string {
	var parts []string
	if m.showTime {
		parts = append(parts, lipgloss.NewStyle().Width(timeWidth).Render(""))
	}
	parts = append(parts,
		lipgloss.NewStyle().
			Foreground(m.th.Border).
			Render(connectorGlyph))
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m Model) styleDot(s Status) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true)
	switch s {
	case StatusSuccess:
		return base.Foreground(m.th.Success)
	case StatusWarning:
		return base.Foreground(m.th.Warning)
	case StatusError:
		return base.Foreground(m.th.Error)
	case StatusInfo:
		return base.Foreground(m.th.Info)
	}
	return base.Foreground(m.th.Primary)
}

func bodyLines(body string) []string {
	if body == "" {
		return nil
	}
	return strings.Split(body, "\n")
}

func computeTimeCol(events []Event, show bool) int {
	if !show {
		return 0
	}
	w := minTimeCol
	for _, e := range events {
		if n := lipgloss.Width(e.Time); n > w {
			w = n
		}
	}
	if w > maxTimeCol {
		w = maxTimeCol
	}
	w++ // trailing space
	return w
}

func truncCell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	out := make([]rune, 0, width)
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > width-1 {
			break
		}
		out = append(out, r)
		used += w
	}
	return string(out) + "…"
}
