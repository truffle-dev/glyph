// Package list renders a vertical list of items with a single
// selected cursor. It's the navigation primitive most agent UIs
// reach for after tabs: a sidebar of conversations, a queue of
// jobs, a result set, a settings index.
//
// A list owns the cursor and the visible window; it does not own
// the items' detail view. The parent reads Selected() and renders
// the detail panel itself.
package list

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Item is one row in the list.
type Item struct {
	// Label is the visible text.
	Label string
	// Hint is optional secondary text rendered after the label in a muted color.
	Hint string
	// Disabled rows render dimmed and can't be selected.
	Disabled bool
	// Value is opaque payload the parent can pull off Selected().
	Value any
}

// List is a vertical selectable list.
type List struct {
	theme   theme.Theme
	items   []Item
	cursor  int
	offset  int
	height  int
	width   int
	cursorL string
	gutterL string
}

// New constructs an empty List with theme-aware styling.
func New(t theme.Theme) List {
	return List{
		theme:   t,
		height:  10,
		cursorL: "›",
		gutterL: " ",
	}
}

// WithItems replaces the item set. Resets the cursor to the first
// enabled item if the previous cursor falls out of range.
func (l List) WithItems(items []Item) List {
	l.items = items
	if l.cursor >= len(l.items) {
		l.cursor = 0
	}
	l.cursor = l.firstEnabledFrom(l.cursor)
	return l
}

// WithHeight sets the maximum visible rows. The list scrolls internally
// when items exceed the height. Minimum 1.
func (l List) WithHeight(h int) List {
	if h < 1 {
		h = 1
	}
	l.height = h
	return l
}

// WithWidth clamps the render width. Values <= 0 mean natural width.
func (l List) WithWidth(w int) List {
	if w < 0 {
		w = 0
	}
	l.width = w
	return l
}

// WithCursor sets the cursor glyph. Default is ›.
func (l List) WithCursor(glyph string) List {
	l.cursorL = glyph
	return l
}

// Update handles cursor movement keys. Up/down arrows and j/k step by
// one; Home/g goes to the first enabled item, End/G to the last.
func (l List) Update(msg tea.Msg) (List, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return l, nil
	}
	if len(l.items) == 0 {
		return l, nil
	}
	switch key.String() {
	case "up", "k":
		l.cursor = l.prevEnabled(l.cursor)
	case "down", "j":
		l.cursor = l.nextEnabled(l.cursor)
	case "home", "g":
		l.cursor = l.firstEnabledFrom(0)
	case "end", "G":
		l.cursor = l.lastEnabled()
	}
	l.clampOffset()
	return l, nil
}

// View renders the list, including only the window of items visible
// at this scroll offset.
func (l List) View() string {
	if len(l.items) == 0 {
		return lipgloss.NewStyle().Foreground(l.theme.TextMuted).Render("(empty)")
	}
	end := l.offset + l.height
	if end > len(l.items) {
		end = len(l.items)
	}
	rows := make([]string, 0, end-l.offset)
	for i := l.offset; i < end; i++ {
		rows = append(rows, l.renderRow(i))
	}
	return strings.Join(rows, "\n")
}

// Selected returns the currently highlighted item along with its index.
// The bool is false on an empty list.
func (l List) Selected() (Item, int, bool) {
	if len(l.items) == 0 {
		return Item{}, -1, false
	}
	return l.items[l.cursor], l.cursor, true
}

// Cursor returns the current cursor index.
func (l List) Cursor() int { return l.cursor }

// renderRow renders one item, styling for cursor/disabled state.
func (l List) renderRow(i int) string {
	it := l.items[i]
	prefix := l.gutterL + " "
	labelStyle := lipgloss.NewStyle().Foreground(l.theme.Text)
	hintStyle := lipgloss.NewStyle().Foreground(l.theme.TextMuted)

	if it.Disabled {
		labelStyle = labelStyle.Foreground(l.theme.TextMuted)
	}
	if i == l.cursor {
		prefix = lipgloss.NewStyle().Foreground(l.theme.Primary).Render(l.cursorL) + " "
		if !it.Disabled {
			labelStyle = labelStyle.Foreground(l.theme.Primary).Bold(true)
		}
	}
	label := labelStyle.Render(it.Label)
	if it.Hint != "" {
		label += " " + hintStyle.Render(it.Hint)
	}
	row := prefix + label
	if l.width > 0 {
		row = lipgloss.NewStyle().MaxWidth(l.width).Render(row)
	}
	return row
}

// firstEnabledFrom walks forward from idx to find the first enabled
// item, wrapping if necessary. Returns idx if every item is disabled.
func (l List) firstEnabledFrom(idx int) int {
	if len(l.items) == 0 {
		return 0
	}
	for i := 0; i < len(l.items); i++ {
		probe := (idx + i) % len(l.items)
		if !l.items[probe].Disabled {
			return probe
		}
	}
	return idx
}

// lastEnabled returns the highest enabled index.
func (l List) lastEnabled() int {
	for i := len(l.items) - 1; i >= 0; i-- {
		if !l.items[i].Disabled {
			return i
		}
	}
	return l.cursor
}

// nextEnabled returns the next enabled index after cur, or cur itself
// if there is no enabled item below.
func (l List) nextEnabled(cur int) int {
	for i := cur + 1; i < len(l.items); i++ {
		if !l.items[i].Disabled {
			return i
		}
	}
	return cur
}

// prevEnabled returns the prev enabled index before cur, or cur itself
// if there is no enabled item above.
func (l List) prevEnabled(cur int) int {
	for i := cur - 1; i >= 0; i-- {
		if !l.items[i].Disabled {
			return i
		}
	}
	return cur
}

// clampOffset keeps the cursor visible inside the height window.
func (l *List) clampOffset() {
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+l.height {
		l.offset = l.cursor - l.height + 1
	}
	if l.offset < 0 {
		l.offset = 0
	}
}
