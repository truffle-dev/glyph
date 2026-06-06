// Package paginationbar renders a single-line page indicator with prev
// and next chevrons and a "page of total" label. It owns the current
// page, the total page count, and the keyboard handling; consumers feed
// it page-state changes via builders and read PageChangedMsg back.
//
// Keys: left/h moves to the previous page, right/l moves to the next
// page, home/g jumps to the first page, end/G jumps to the last page.
// Page numbers in the rendered label are 1-indexed for the reader; the
// API uses 0-indexed Page() for code paths. Moving off either end is a
// no-op when WithWrap(false) (the default); WithWrap(true) wraps. Each
// motion that changes the page emits PageChangedMsg with the new page.
//
// WithTotalItems sets an optional total-items count and renders an
// extra "(N items)" suffix. WithPerPage lets the caller treat the
// component as a window over a sequence: passing a per-page size with
// a total-items count derives the page count automatically.
package paginationbar

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// PageChangedMsg fires when the page actually changes via Update. Page
// is 0-indexed.
type PageChangedMsg struct {
	Page  int
	Total int
}

const (
	prevGlyph = "‹"
	nextGlyph = "›"
	separator = " "
)

// Model is the Bubble Tea state of the pagination bar.
type Model struct {
	th         theme.Theme
	page       int
	total      int
	totalItems int
	perPage    int
	hasItems   bool
	width      int
	wrap       bool
	prefix     string
	itemsLabel string
}

// New constructs a Model with Default theme, one page total, page 0,
// no item count, no wrap, and the default item label "items".
func New() Model {
	return Model{
		th:         theme.Default,
		total:      1,
		itemsLabel: "items",
	}
}

// WithTheme overrides the theme palette.
func (m Model) WithTheme(t theme.Theme) Model { m.th = t; return m }

// WithTotal sets the total page count. Values < 1 clamp to 1. The
// current page clamps into range.
func (m Model) WithTotal(n int) Model {
	if n < 1 {
		n = 1
	}
	m.total = n
	if m.page >= n {
		m.page = n - 1
	}
	if m.page < 0 {
		m.page = 0
	}
	return m
}

// WithPage selects the current page by 0-indexed number. Out-of-range
// values clamp into the valid range.
func (m Model) WithPage(i int) Model {
	if i < 0 {
		i = 0
	}
	if i >= m.total {
		i = m.total - 1
	}
	m.page = i
	return m
}

// WithTotalItems sets an optional total-items count. When set and
// non-negative, the rendered label includes a "(N items)" suffix.
func (m Model) WithTotalItems(n int) Model {
	if n < 0 {
		n = 0
	}
	m.totalItems = n
	m.hasItems = true
	m.recomputeTotalFromPerPage()
	return m
}

// WithPerPage sets an items-per-page size. When combined with
// WithTotalItems the total page count is recomputed automatically so
// the caller doesn't have to do the division. Values < 1 disable the
// recompute (and leave WithTotal in charge).
func (m Model) WithPerPage(n int) Model {
	if n < 1 {
		m.perPage = 0
		return m
	}
	m.perPage = n
	m.recomputeTotalFromPerPage()
	return m
}

// WithWidth sets an optional maximum render width. Values <= 0 mean no
// clamp; the bar renders at its natural width.
func (m Model) WithWidth(w int) Model {
	if w < 0 {
		w = 0
	}
	m.width = w
	return m
}

// WithWrap toggles wrap-around motion. When false (the default), prev
// at page 0 and next at the last page are no-ops; when true, both
// wrap.
func (m Model) WithWrap(on bool) Model { m.wrap = on; return m }

// WithPrefix sets an optional leading label rendered before the
// chevron row, e.g. "Results " so the rendered bar reads
// "Results ‹ 3 of 7 › (42 items)".
func (m Model) WithPrefix(s string) Model { m.prefix = s; return m }

// WithItemsLabel overrides the default item label "items".
func (m Model) WithItemsLabel(s string) Model {
	if s == "" {
		s = "items"
	}
	m.itemsLabel = s
	return m
}

// Page returns the 0-indexed current page.
func (m Model) Page() int { return m.page }

// Total returns the page count.
func (m Model) Total() int { return m.total }

// PageNumber returns the 1-indexed current page for display.
func (m Model) PageNumber() int { return m.page + 1 }

// AtStart reports whether the current page is the first page.
func (m Model) AtStart() bool { return m.page <= 0 }

// AtEnd reports whether the current page is the last page.
func (m Model) AtEnd() bool { return m.page >= m.total-1 }

// VisibleRange returns the [start, end) item indices visible on the
// current page when WithPerPage and WithTotalItems are both set; both
// return zero otherwise. End is clamped at totalItems.
func (m Model) VisibleRange() (int, int) {
	if m.perPage <= 0 || !m.hasItems {
		return 0, 0
	}
	start := m.page * m.perPage
	end := start + m.perPage
	if end > m.totalItems {
		end = m.totalItems
	}
	if start > m.totalItems {
		start = m.totalItems
	}
	return start, end
}

// Init satisfies tea.Model. No initial command.
func (m Model) Init() tea.Cmd { return nil }

// Update handles motion keys. Returns a PageChangedMsg command only
// when the page actually changes.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	prev := m.page
	switch key.String() {
	case "left", "h":
		m.page = m.stepBackward()
	case "right", "l":
		m.page = m.stepForward()
	case "home", "g":
		m.page = 0
	case "end", "G":
		m.page = m.total - 1
	default:
		return m, nil
	}
	if m.page == prev {
		return m, nil
	}
	page, total := m.page, m.total
	return m, func() tea.Msg {
		return PageChangedMsg{Page: page, Total: total}
	}
}

// View renders the pagination bar.
func (m Model) View() string {
	chevronOn := lipgloss.NewStyle().Foreground(m.th.Primary).Bold(true)
	chevronOff := lipgloss.NewStyle().Foreground(m.th.TextMuted)
	labelStyle := lipgloss.NewStyle().Foreground(m.th.Text)
	itemsStyle := lipgloss.NewStyle().Foreground(m.th.TextMuted)
	prefixStyle := lipgloss.NewStyle().Foreground(m.th.TextMuted)

	prev := chevronOff.Render(prevGlyph)
	if !m.AtStart() || m.wrap {
		prev = chevronOn.Render(prevGlyph)
	}
	next := chevronOff.Render(nextGlyph)
	if !m.AtEnd() || m.wrap {
		next = chevronOn.Render(nextGlyph)
	}
	label := labelStyle.Render(fmt.Sprintf("%d of %d", m.PageNumber(), m.total))

	out := prev + separator + label + separator + next
	if m.prefix != "" {
		out = prefixStyle.Render(m.prefix) + out
	}
	if m.hasItems {
		out += separator + itemsStyle.Render(fmt.Sprintf("(%d %s)", m.totalItems, m.itemsLabel))
	}
	if m.width > 0 {
		return lipgloss.NewStyle().MaxWidth(m.width).Render(out)
	}
	return out
}

func (m Model) stepBackward() int {
	if m.page > 0 {
		return m.page - 1
	}
	if m.wrap {
		return m.total - 1
	}
	return m.page
}

func (m Model) stepForward() int {
	if m.page < m.total-1 {
		return m.page + 1
	}
	if m.wrap {
		return 0
	}
	return m.page
}

func (m *Model) recomputeTotalFromPerPage() {
	if m.perPage <= 0 || !m.hasItems {
		return
	}
	if m.totalItems == 0 {
		m.total = 1
		m.page = 0
		return
	}
	total := m.totalItems / m.perPage
	if m.totalItems%m.perPage != 0 {
		total++
	}
	if total < 1 {
		total = 1
	}
	m.total = total
	if m.page >= total {
		m.page = total - 1
	}
}
