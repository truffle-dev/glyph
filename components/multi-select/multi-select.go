// Package multiselect renders a bounded multi-choice list with checkbox
// rows, an optional substring filter, and a stable selection that survives
// filtering. It is the picker an agent UI reaches for when the answer is
// "zero or more of these": files to stage, tags to attach, channels to
// post to, scopes to grant.
//
// The package is named multiselect because select is a Go reserved word
// and the sibling single-choice component already owns selectinput. The
// component owns a cursor, the checked set, and the optional filter; the
// parent observes ConfirmMsg on Enter and CancelMsg on Esc.
//
// Selection is keyed by each option's resolved value (Value, or Label when
// Value is empty), not by its row index, so a checked row stays checked
// when the filter hides and later reveals it. Space toggles the row under
// the cursor; "a" toggles every currently visible row as a group.
package multiselect

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Option is one row. Value is what comes back on commit and is the stable
// key for the checked set; it defaults to Label when empty.
type Option struct {
	Label string
	Hint  string
	Value string
}

// resolved returns the identity used for the checked set: Value, or Label
// when Value is empty.
func (o Option) resolved() string {
	if o.Value != "" {
		return o.Value
	}
	return o.Label
}

// ConfirmMsg is emitted when the user commits with Enter. Selected lists the
// checked options in their original (unfiltered) order; Values mirrors it as
// the resolved value of each.
type ConfirmMsg struct {
	Selected []Option
	Values   []string
}

// CancelMsg is emitted when the user presses Esc.
type CancelMsg struct{}

// MultiSelect is a Bubble Tea model for a bounded multi-choice picker.
type MultiSelect struct {
	theme       theme.Theme
	options     []Option
	checked     map[string]bool
	cursor      int
	offset      int
	width       int
	height      int
	title       string
	placeholder string
	filterOn    bool
	filter      string
}

// New constructs a MultiSelect with safe defaults.
func New(t theme.Theme) MultiSelect {
	return MultiSelect{theme: t, checked: map[string]bool{}, width: 40, height: 7}
}

// WithOptions replaces the option set and resets the cursor. Checked values
// that are no longer present are dropped so the committed set never names a
// row the user can't see.
func (m MultiSelect) WithOptions(opts []Option) MultiSelect {
	m.options = append([]Option(nil), opts...)
	present := make(map[string]bool, len(opts))
	for _, o := range m.options {
		present[o.resolved()] = true
	}
	next := make(map[string]bool, len(m.checked))
	for v := range m.checked {
		if present[v] {
			next[v] = true
		}
	}
	m.checked = next
	if m.cursor >= len(m.options) {
		m.cursor = 0
	}
	m.offset = 0
	return m
}

// WithChecked pre-checks the options whose resolved value appears in values.
func (m MultiSelect) WithChecked(values []string) MultiSelect {
	next := make(map[string]bool, len(m.checked)+len(values))
	for v := range m.checked {
		next[v] = true
	}
	for _, v := range values {
		next[v] = true
	}
	m.checked = next
	return m
}

// WithSize sets the rendered width and inner list height.
func (m MultiSelect) WithSize(w, h int) MultiSelect {
	if w < 12 {
		w = 12
	}
	if h < 1 {
		h = 1
	}
	m.width, m.height = w, h
	return m
}

// WithTitle sets the title bar above the list.
func (m MultiSelect) WithTitle(title string) MultiSelect { m.title = title; return m }

// WithPlaceholder sets the empty-filter placeholder. Filter-only.
func (m MultiSelect) WithPlaceholder(p string) MultiSelect { m.placeholder = p; return m }

// WithFilter toggles the typeahead substring filter.
func (m MultiSelect) WithFilter(on bool) MultiSelect {
	m.filterOn = on
	if !on {
		m.filter = ""
	}
	return m
}

// Cursor returns the index into the visible (filtered) option list.
func (m MultiSelect) Cursor() int { return m.cursor }

// Count returns how many options are currently checked.
func (m MultiSelect) Count() int {
	n := 0
	for _, o := range m.options {
		if m.checked[o.resolved()] {
			n++
		}
	}
	return n
}

// SelectedValues returns the resolved values of the checked options in their
// original order.
func (m MultiSelect) SelectedValues() []string {
	out := make([]string, 0, len(m.checked))
	for _, o := range m.options {
		if m.checked[o.resolved()] {
			out = append(out, o.resolved())
		}
	}
	return out
}

// SelectedOptions returns the checked options in their original order.
func (m MultiSelect) SelectedOptions() []Option {
	out := make([]Option, 0, len(m.checked))
	for _, o := range m.options {
		if m.checked[o.resolved()] {
			out = append(out, o)
		}
	}
	return out
}

// Init implements tea.Model.
func (m MultiSelect) Init() tea.Cmd { return nil }

// Update routes key events.
func (m MultiSelect) Update(msg tea.Msg) (MultiSelect, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.Type {
	case tea.KeyEsc:
		return m, func() tea.Msg { return CancelMsg{} }
	case tea.KeyEnter:
		return m.commit()
	case tea.KeySpace, tea.KeyTab:
		return m.toggleCursor(), nil
	case tea.KeyUp:
		return m.moveCursor(-1), nil
	case tea.KeyDown:
		return m.moveCursor(+1), nil
	case tea.KeyPgUp:
		return m.moveCursor(-m.height), nil
	case tea.KeyPgDown:
		return m.moveCursor(+m.height), nil
	case tea.KeyHome:
		return m.jumpCursor(0), nil
	case tea.KeyEnd:
		return m.jumpCursor(len(m.visible()) - 1), nil
	case tea.KeyBackspace:
		if m.filterOn && len(m.filter) > 0 {
			r := []rune(m.filter)
			m.filter = string(r[:len(r)-1])
			m.cursor, m.offset = 0, 0
		}
		return m, nil
	case tea.KeyCtrlU:
		if m.filterOn {
			m.filter, m.cursor, m.offset = "", 0, 0
		}
		return m, nil
	case tea.KeyRunes:
		if m.filterOn {
			m.filter += string(key.Runes)
			m.cursor, m.offset = 0, 0
			return m, nil
		}
		// Filter off: vim-style nav plus "a" for toggle-all-visible.
		switch string(key.Runes) {
		case "k":
			return m.moveCursor(-1), nil
		case "j":
			return m.moveCursor(+1), nil
		case "g":
			return m.jumpCursor(0), nil
		case "G":
			return m.jumpCursor(len(m.visible()) - 1), nil
		case "a":
			return m.toggleAllVisible(), nil
		}
	}
	return m, nil
}

// commit emits a ConfirmMsg with the checked set in original order.
func (m MultiSelect) commit() (MultiSelect, tea.Cmd) {
	sel := m.SelectedOptions()
	vals := m.SelectedValues()
	return m, func() tea.Msg { return ConfirmMsg{Selected: sel, Values: vals} }
}

// toggleCursor flips the checked state of the row under the cursor.
func (m MultiSelect) toggleCursor() MultiSelect {
	vis := m.visible()
	if len(vis) == 0 || m.cursor < 0 || m.cursor >= len(vis) {
		return m
	}
	v := vis[m.cursor].resolved()
	if m.checked[v] {
		delete(m.checked, v)
	} else {
		m.checked[v] = true
	}
	return m
}

// toggleAllVisible checks every visible row, or unchecks them all when they
// are already fully checked. Hidden (filtered-out) rows are untouched.
func (m MultiSelect) toggleAllVisible() MultiSelect {
	vis := m.visible()
	if len(vis) == 0 {
		return m
	}
	allChecked := true
	for _, o := range vis {
		if !m.checked[o.resolved()] {
			allChecked = false
			break
		}
	}
	for _, o := range vis {
		v := o.resolved()
		if allChecked {
			delete(m.checked, v)
		} else {
			m.checked[v] = true
		}
	}
	return m
}

// moveCursor steps the cursor by delta, clamping at both ends (no wrap).
func (m MultiSelect) moveCursor(delta int) MultiSelect {
	vis := m.visible()
	if len(vis) == 0 {
		m.cursor = 0
		return m
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(vis) {
		m.cursor = len(vis) - 1
	}
	m.clampOffset()
	return m
}

// jumpCursor moves the cursor to an absolute index, clamping.
func (m MultiSelect) jumpCursor(idx int) MultiSelect {
	vis := m.visible()
	if len(vis) == 0 {
		m.cursor = 0
		return m
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(vis) {
		idx = len(vis) - 1
	}
	m.cursor = idx
	m.clampOffset()
	return m
}

// visible returns options that survive the current filter.
func (m MultiSelect) visible() []Option {
	if !m.filterOn || m.filter == "" {
		return m.options
	}
	q := strings.ToLower(m.filter)
	out := make([]Option, 0, len(m.options))
	for _, o := range m.options {
		if strings.Contains(strings.ToLower(o.Label+" "+o.Hint), q) {
			out = append(out, o)
		}
	}
	return out
}

// clampOffset keeps the cursor inside the height window.
func (m *MultiSelect) clampOffset() {
	if m.height < 1 {
		m.height = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// View renders the popover: optional title with a live count, optional filter
// input, list body.
func (m MultiSelect) View() string {
	parts := []string{}
	if m.title != "" {
		title := lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Bold(true).
			Render(m.title)
		count := lipgloss.NewStyle().
			Foreground(m.theme.Primary).
			Render(m.countLabel())
		gap := (m.width - 4) - lipgloss.Width(title) - lipgloss.Width(count)
		if gap < 1 {
			gap = 1
		}
		parts = append(parts, m.underlined(title+strings.Repeat(" ", gap)+count))
	}
	if m.filterOn {
		parts = append(parts, m.underlined(m.renderFilterInput()))
	}
	parts = append(parts, m.renderBody())

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.BorderStrong).
		Background(m.theme.Surface).
		Padding(0, 1).
		Width(m.width).
		Render(strings.Join(parts, "\n"))
}

// countLabel renders the "n selected" badge for the title row.
func (m MultiSelect) countLabel() string {
	n := m.Count()
	if n == 1 {
		return "1 selected"
	}
	return strconv.Itoa(n) + " selected"
}

// underlined wraps content in a width-clamped row with a subtle bottom rule.
func (m MultiSelect) underlined(content string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.theme.Border).
		Width(m.width - 4).
		Render(content)
}

// renderFilterInput draws "> filter␣" with placeholder fallback.
func (m MultiSelect) renderFilterInput() string {
	prompt := lipgloss.NewStyle().Foreground(m.theme.Primary).Bold(true).Render("> ")
	var body string
	if m.filter == "" {
		body = lipgloss.NewStyle().Foreground(m.theme.TextMuted).Italic(true).Render(m.placeholder)
	} else {
		body = lipgloss.NewStyle().Foreground(m.theme.Text).Render(m.filter)
	}
	cursor := lipgloss.NewStyle().Background(m.theme.Text).Foreground(m.theme.Bg).Render(" ")
	return prompt + body + cursor
}

// renderBody draws the scrolled list, or a muted empty-state line.
func (m MultiSelect) renderBody() string {
	vis := m.visible()
	if len(vis) == 0 {
		empty := "(empty)"
		if m.filterOn {
			empty = "(no matches)"
		}
		return lipgloss.NewStyle().Foreground(m.theme.TextMuted).Italic(true).Render(empty)
	}
	end := m.offset + m.height
	if end > len(vis) {
		end = len(vis)
	}
	rows := make([]string, 0, end-m.offset)
	for i := m.offset; i < end; i++ {
		rows = append(rows, m.renderRow(vis[i], i == m.cursor))
	}
	return strings.Join(rows, "\n")
}

// renderRow draws one option: cursor mark + checkbox + label + hint.
func (m MultiSelect) renderRow(o Option, active bool) string {
	rowWidth := m.width - 4
	labelStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
	hintStyle := lipgloss.NewStyle().Foreground(m.theme.TextMuted)
	mark := "  "
	if active {
		mark = lipgloss.NewStyle().Foreground(m.theme.Primary).Render("› ")
		labelStyle = labelStyle.Foreground(m.theme.PrimaryStrong).Bold(true)
	}
	var box string
	if m.checked[o.resolved()] {
		box = lipgloss.NewStyle().Foreground(m.theme.Primary).Bold(true).Render("[x] ")
	} else {
		box = lipgloss.NewStyle().Foreground(m.theme.TextMuted).Render("[ ] ")
	}
	label := labelStyle.Render(o.Label)
	hint := ""
	if o.Hint != "" {
		hint = hintStyle.Render(o.Hint)
	}
	gap := rowWidth - lipgloss.Width(mark) - lipgloss.Width(box) - lipgloss.Width(label) - lipgloss.Width(hint)
	if gap < 1 {
		gap = 1
	}
	row := mark + box + label + strings.Repeat(" ", gap) + hint
	if active {
		row = lipgloss.NewStyle().Background(m.theme.SurfaceStrong).Width(rowWidth).Render(row)
	}
	return row
}
