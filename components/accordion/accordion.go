// Package accordion renders a vertical stack of titled sections, each
// independently collapsible. One section can be focused at a time (the
// cursor); zero or more can be expanded depending on the AllowMultiple
// setting. The component owns the section list, the expanded set, the
// cursor, and the scroll offset; consumers feed it section data and read
// SelectMsg or ToggleMsg back.
//
// Keys: up/down (or k/j) move the focus between section headers, Enter
// toggles the focused section's expanded state and emits SelectMsg,
// Space toggles silently, Right/l expands the focused section if it's
// collapsed, Left/h collapses the focused section if it's expanded,
// Home/End jump to the first or last section, Tab and Shift+Tab cycle
// the focus with wrap.
//
// In single-expanded mode (the default), expanding a section collapses
// every other section. WithAllowMultiple(true) makes every section's
// expanded state independent.
package accordion

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Section is one collapsible row. Title shows on the header line; Body
// renders below the header when expanded. Body may contain newlines.
// Value is the caller's payload, returned in SelectMsg and ToggleMsg.
type Section struct {
	Title string
	Body  string
	Value any
}

// SelectMsg fires when the user presses Enter on the focused section.
// Expanded reports the section's expanded state AFTER the toggle.
type SelectMsg struct {
	Section  Section
	Index    int
	Expanded bool
}

const (
	expandedGlyph  = "▾"
	collapsedGlyph = "▸"
	bodyIndent     = "  "
)

// Model is the Bubble Tea state of the accordion.
type Model struct {
	th            theme.Theme
	sections      []Section
	expanded      map[int]bool
	focus         int
	offset        int
	width         int
	height        int
	hasSize       bool
	allowMultiple bool
	highlight     bool
	placeholder   string
}

// New constructs a Model with Default theme, single-expanded mode, no
// explicit size (renders at natural width and height), and a placeholder
// for the empty state.
func New() Model {
	return Model{
		th:          theme.Default,
		expanded:    map[int]bool{},
		highlight:   true,
		placeholder: "no sections",
	}
}

// WithTheme overrides the theme palette.
func (m Model) WithTheme(t theme.Theme) Model { m.th = t; return m }

// WithSections replaces the section list. The focus clamps into range;
// the expanded set is filtered to drop entries that no longer point at
// a valid section.
func (m Model) WithSections(sections ...Section) Model {
	m.sections = sections
	if m.focus >= len(sections) {
		m.focus = 0
	}
	next := map[int]bool{}
	for i, on := range m.expanded {
		if on && i >= 0 && i < len(sections) {
			next[i] = true
		}
	}
	m.expanded = next
	m.clampOffset()
	return m
}

// WithAllowMultiple toggles between single-expanded (false, the default)
// and independent (true) section state. Switching from independent to
// single collapses every section except the focused one if any are open.
func (m Model) WithAllowMultiple(on bool) Model {
	m.allowMultiple = on
	if !on {
		kept := -1
		if m.expanded[m.focus] {
			kept = m.focus
		} else {
			for i := range m.sections {
				if m.expanded[i] {
					kept = i
					break
				}
			}
		}
		next := map[int]bool{}
		if kept >= 0 {
			next[kept] = true
		}
		m.expanded = next
	}
	return m
}

// WithFocused selects the focused section by index, clamped into range.
// An empty section list forces 0.
func (m Model) WithFocused(i int) Model {
	if len(m.sections) == 0 {
		m.focus = 0
		return m
	}
	if i < 0 {
		i = 0
	}
	if i >= len(m.sections) {
		i = len(m.sections) - 1
	}
	m.focus = i
	m.scrollToFocus()
	return m
}

// WithExpanded expands the given indices. In single-expanded mode, only
// the last in-range index ends up expanded. Out-of-range indices are
// silently dropped.
func (m Model) WithExpanded(indices ...int) Model {
	next := map[int]bool{}
	if m.allowMultiple {
		for _, i := range indices {
			if i >= 0 && i < len(m.sections) {
				next[i] = true
			}
		}
	} else {
		for _, i := range indices {
			if i >= 0 && i < len(m.sections) {
				next = map[int]bool{i: true}
			}
		}
	}
	m.expanded = next
	return m
}

// WithExpandAll expands every section. In single-expanded mode this is
// equivalent to expanding only the last section.
func (m Model) WithExpandAll() Model {
	if len(m.sections) == 0 {
		m.expanded = map[int]bool{}
		return m
	}
	if !m.allowMultiple {
		m.expanded = map[int]bool{len(m.sections) - 1: true}
		return m
	}
	next := map[int]bool{}
	for i := range m.sections {
		next[i] = true
	}
	m.expanded = next
	return m
}

// WithCollapseAll collapses every section.
func (m Model) WithCollapseAll() Model {
	m.expanded = map[int]bool{}
	return m
}

// WithSize sets the rendered width and visible-row height. Height counts
// rendered lines, not sections; a wide section body that wraps to four
// lines counts as four rows. Values <= 0 disable the clamp on that axis.
func (m Model) WithSize(w, h int) Model {
	m.width = w
	m.height = h
	m.hasSize = w > 0 || h > 0
	m.clampOffset()
	return m
}

// WithHighlightCursor toggles the cursor highlight on the focused
// section header (default true). The cursor still moves either way.
func (m Model) WithHighlightCursor(on bool) Model { m.highlight = on; return m }

// WithPlaceholder overrides the empty-state text rendered when the
// section list is empty.
func (m Model) WithPlaceholder(s string) Model { m.placeholder = s; return m }

// Focused returns the index of the section the cursor is on.
func (m Model) Focused() int { return m.focus }

// FocusedSection returns the section the cursor is on. The boolean
// reports whether the section list is non-empty.
func (m Model) FocusedSection() (Section, bool) {
	if len(m.sections) == 0 {
		return Section{}, false
	}
	return m.sections[m.focus], true
}

// IsExpanded reports whether the section at index i is currently
// expanded. Out-of-range indices return false.
func (m Model) IsExpanded(i int) bool {
	if i < 0 || i >= len(m.sections) {
		return false
	}
	return m.expanded[i]
}

// ExpandedIndices returns the sorted list of currently-expanded section
// indices.
func (m Model) ExpandedIndices() []int {
	out := []int{}
	for i := range m.sections {
		if m.expanded[i] {
			out = append(out, i)
		}
	}
	return out
}

// Sections returns a copy of the section list.
func (m Model) Sections() []Section {
	out := make([]Section, len(m.sections))
	copy(out, m.sections)
	return out
}

// Init satisfies tea.Model. No initial command.
func (m Model) Init() tea.Cmd { return nil }

// Update handles cursor and toggle keys. All other messages pass through
// unchanged.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if len(m.sections) == 0 {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		m.focus = (m.focus - 1 + len(m.sections)) % len(m.sections)
		m.scrollToFocus()
	case "down", "j":
		m.focus = (m.focus + 1) % len(m.sections)
		m.scrollToFocus()
	case "tab":
		m.focus = (m.focus + 1) % len(m.sections)
		m.scrollToFocus()
	case "shift+tab":
		m.focus = (m.focus - 1 + len(m.sections)) % len(m.sections)
		m.scrollToFocus()
	case "home", "g":
		m.focus = 0
		m.scrollToFocus()
	case "end", "G":
		m.focus = len(m.sections) - 1
		m.scrollToFocus()
	case "enter":
		newState := !m.expanded[m.focus]
		m.toggle(m.focus, newState)
		section := m.sections[m.focus]
		idx := m.focus
		m.scrollToFocus()
		return m, func() tea.Msg {
			return SelectMsg{Section: section, Index: idx, Expanded: newState}
		}
	case " ":
		newState := !m.expanded[m.focus]
		m.toggle(m.focus, newState)
		m.scrollToFocus()
	case "right", "l":
		if !m.expanded[m.focus] {
			m.toggle(m.focus, true)
			m.scrollToFocus()
		}
	case "left", "h":
		if m.expanded[m.focus] {
			m.toggle(m.focus, false)
			m.scrollToFocus()
		}
	}
	return m, nil
}

// View renders the section stack honoring focus, expanded state, and
// the optional size clamp.
func (m Model) View() string {
	if len(m.sections) == 0 {
		muted := lipgloss.NewStyle().Foreground(m.th.TextMuted)
		out := muted.Render(m.placeholder)
		if m.hasSize && m.width > 0 {
			return lipgloss.NewStyle().Width(m.width).Render(out)
		}
		return out
	}

	headerActive := lipgloss.NewStyle().Foreground(m.th.Primary).Bold(true)
	headerIdle := lipgloss.NewStyle().Foreground(m.th.Text)
	glyphActive := lipgloss.NewStyle().Foreground(m.th.Primary)
	glyphIdle := lipgloss.NewStyle().Foreground(m.th.TextMuted)
	bodyStyle := lipgloss.NewStyle().Foreground(m.th.TextMuted)

	var rows []string
	for i, sec := range m.sections {
		open := m.expanded[i]
		focused := i == m.focus
		glyph := collapsedGlyph
		if open {
			glyph = expandedGlyph
		}
		var glyphRendered, titleRendered string
		if focused && m.highlight {
			glyphRendered = glyphActive.Render(glyph)
			titleRendered = headerActive.Render(sec.Title)
		} else {
			glyphRendered = glyphIdle.Render(glyph)
			titleRendered = headerIdle.Render(sec.Title)
		}
		rows = append(rows, glyphRendered+" "+titleRendered)
		if open && sec.Body != "" {
			for _, line := range strings.Split(sec.Body, "\n") {
				rows = append(rows, bodyIndent+bodyStyle.Render(line))
			}
		}
	}

	if m.hasSize && m.height > 0 && len(rows) > m.height {
		start := m.offset
		if start < 0 {
			start = 0
		}
		if start > len(rows)-m.height {
			start = len(rows) - m.height
		}
		rows = rows[start : start+m.height]
	}

	out := strings.Join(rows, "\n")
	if m.hasSize && m.width > 0 {
		return lipgloss.NewStyle().MaxWidth(m.width).Render(out)
	}
	return out
}

// toggle sets the expanded state of section i. In single-expanded mode,
// opening one section closes any other open section.
func (m *Model) toggle(i int, on bool) {
	if i < 0 || i >= len(m.sections) {
		return
	}
	if on {
		if !m.allowMultiple {
			m.expanded = map[int]bool{i: true}
			return
		}
		m.expanded[i] = true
		return
	}
	delete(m.expanded, i)
}

// scrollToFocus moves the offset so the focused section's header is
// inside the visible window when a size is set.
func (m *Model) scrollToFocus() {
	if !m.hasSize || m.height <= 0 || len(m.sections) == 0 {
		m.offset = 0
		return
	}
	headerRow := 0
	for i := 0; i < m.focus; i++ {
		headerRow++
		if m.expanded[i] && m.sections[i].Body != "" {
			headerRow += strings.Count(m.sections[i].Body, "\n") + 1
		}
	}
	total := 0
	for i, sec := range m.sections {
		total++
		if m.expanded[i] && sec.Body != "" {
			total += strings.Count(sec.Body, "\n") + 1
		}
	}
	if total <= m.height {
		m.offset = 0
		return
	}
	if headerRow < m.offset {
		m.offset = headerRow
	}
	if headerRow >= m.offset+m.height {
		m.offset = headerRow - m.height + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > total-m.height {
		m.offset = total - m.height
	}
}

// clampOffset re-applies the visible-window clamp after section or size
// changes.
func (m *Model) clampOffset() {
	if !m.hasSize || m.height <= 0 || len(m.sections) == 0 {
		m.offset = 0
		return
	}
	total := 0
	for i, sec := range m.sections {
		total++
		if m.expanded[i] && sec.Body != "" {
			total += strings.Count(sec.Body, "\n") + 1
		}
	}
	if total <= m.height {
		m.offset = 0
		return
	}
	if m.offset > total-m.height {
		m.offset = total - m.height
	}
	if m.offset < 0 {
		m.offset = 0
	}
}
