// Package selectinput renders a bounded single-choice popover with an
// optional substring filter. The picker most agent UIs reach for when
// there are five to fifty options: a model selector, a workspace
// switcher, a destination chooser.
//
// The package is named selectinput because select is a Go reserved word.
// The component owns a cursor and the optional filter; the parent
// observes SelectMsg on Enter (or Tab) and CancelMsg on Esc.
package selectinput

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Option is one row. Value is what comes back on commit; defaults to Label.
type Option struct {
	Label string
	Hint  string
	Value string
}

// SelectMsg is emitted when the user commits with Enter or Tab.
type SelectMsg struct {
	Option Option
	Index  int
}

// CancelMsg is emitted when the user presses Esc.
type CancelMsg struct{}

// Select is a Bubble Tea model for a bounded single-choice picker.
type Select struct {
	theme       theme.Theme
	options     []Option
	cursor      int
	offset      int
	width       int
	height      int
	title       string
	placeholder string
	filterOn    bool
	filter      string
}

// New constructs a Select with safe defaults.
func New(t theme.Theme) Select {
	return Select{theme: t, width: 40, height: 7}
}

// WithOptions replaces the option set and resets the cursor.
func (s Select) WithOptions(opts []Option) Select {
	s.options = append([]Option(nil), opts...)
	if s.cursor >= len(s.options) {
		s.cursor = 0
	}
	s.offset = 0
	return s
}

// WithSelected sets the initial cursor index, clamped.
func (s Select) WithSelected(i int) Select {
	if i < 0 {
		i = 0
	}
	if len(s.options) > 0 && i >= len(s.options) {
		i = len(s.options) - 1
	}
	s.cursor = i
	s.clampOffset()
	return s
}

// WithSize sets the rendered width and inner list height.
func (s Select) WithSize(w, h int) Select {
	if w < 12 {
		w = 12
	}
	if h < 1 {
		h = 1
	}
	s.width, s.height = w, h
	return s
}

// WithTitle sets the title bar above the list.
func (s Select) WithTitle(title string) Select { s.title = title; return s }

// WithPlaceholder sets the empty-filter placeholder. Filter-only.
func (s Select) WithPlaceholder(p string) Select { s.placeholder = p; return s }

// WithFilter toggles the typeahead substring filter.
func (s Select) WithFilter(on bool) Select {
	s.filterOn = on
	if !on {
		s.filter = ""
	}
	return s
}

// Cursor returns the index into the visible (filtered) option list.
func (s Select) Cursor() int { return s.cursor }

// Selected returns the Option under the cursor, or (zero, false) when empty.
func (s Select) Selected() (Option, bool) {
	vis := s.visible()
	if len(vis) == 0 || s.cursor < 0 || s.cursor >= len(vis) {
		return Option{}, false
	}
	return vis[s.cursor], true
}

// Init implements tea.Model.
func (s Select) Init() tea.Cmd { return nil }

// Update routes key events.
func (s Select) Update(msg tea.Msg) (Select, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.Type {
	case tea.KeyEsc:
		return s, func() tea.Msg { return CancelMsg{} }
	case tea.KeyEnter, tea.KeyTab:
		return s.commit()
	case tea.KeyUp:
		return s.moveCursor(-1), nil
	case tea.KeyDown:
		return s.moveCursor(+1), nil
	case tea.KeyPgUp:
		return s.moveCursor(-s.height), nil
	case tea.KeyPgDown:
		return s.moveCursor(+s.height), nil
	case tea.KeyHome:
		return s.jumpCursor(0), nil
	case tea.KeyEnd:
		return s.jumpCursor(len(s.visible()) - 1), nil
	case tea.KeyBackspace:
		if s.filterOn && len(s.filter) > 0 {
			r := []rune(s.filter)
			s.filter = string(r[:len(r)-1])
			s.cursor, s.offset = 0, 0
		}
		return s, nil
	case tea.KeyCtrlU:
		if s.filterOn {
			s.filter, s.cursor, s.offset = "", 0, 0
		}
		return s, nil
	case tea.KeySpace:
		if s.filterOn {
			s.filter += " "
			s.cursor, s.offset = 0, 0
		}
		return s, nil
	case tea.KeyRunes:
		if s.filterOn {
			s.filter += string(key.Runes)
			s.cursor, s.offset = 0, 0
			return s, nil
		}
		// Filter off: vim-style nav shortcuts.
		switch string(key.Runes) {
		case "k":
			return s.moveCursor(-1), nil
		case "j":
			return s.moveCursor(+1), nil
		case "g":
			return s.jumpCursor(0), nil
		case "G":
			return s.jumpCursor(len(s.visible()) - 1), nil
		}
	}
	return s, nil
}

// commit emits a SelectMsg for the current cursor. No-op when no visible row.
func (s Select) commit() (Select, tea.Cmd) {
	vis := s.visible()
	if len(vis) == 0 || s.cursor < 0 || s.cursor >= len(vis) {
		return s, nil
	}
	opt := vis[s.cursor]
	if opt.Value == "" {
		opt.Value = opt.Label
	}
	idx := s.cursor
	return s, func() tea.Msg { return SelectMsg{Option: opt, Index: idx} }
}

// moveCursor steps the cursor by delta, clamping at both ends (no wrap).
func (s Select) moveCursor(delta int) Select {
	vis := s.visible()
	if len(vis) == 0 {
		s.cursor = 0
		return s
	}
	s.cursor += delta
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor >= len(vis) {
		s.cursor = len(vis) - 1
	}
	s.clampOffset()
	return s
}

// jumpCursor moves the cursor to an absolute index, clamping.
func (s Select) jumpCursor(idx int) Select {
	vis := s.visible()
	if len(vis) == 0 {
		s.cursor = 0
		return s
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(vis) {
		idx = len(vis) - 1
	}
	s.cursor = idx
	s.clampOffset()
	return s
}

// visible returns options that survive the current filter.
func (s Select) visible() []Option {
	if !s.filterOn || s.filter == "" {
		return s.options
	}
	q := strings.ToLower(s.filter)
	out := make([]Option, 0, len(s.options))
	for _, o := range s.options {
		if strings.Contains(strings.ToLower(o.Label+" "+o.Hint), q) {
			out = append(out, o)
		}
	}
	return out
}

// clampOffset keeps the cursor inside the height window.
func (s *Select) clampOffset() {
	if s.height < 1 {
		s.height = 1
	}
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+s.height {
		s.offset = s.cursor - s.height + 1
	}
	if s.offset < 0 {
		s.offset = 0
	}
}

// View renders the popover: optional title, optional filter input, list body.
func (s Select) View() string {
	parts := []string{}
	if s.title != "" {
		parts = append(parts, s.underlined(lipgloss.NewStyle().
			Foreground(s.theme.TextMuted).
			Bold(true).
			Render(s.title)))
	}
	if s.filterOn {
		parts = append(parts, s.underlined(s.renderFilterInput()))
	}
	parts = append(parts, s.renderBody())

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.theme.BorderStrong).
		Background(s.theme.Surface).
		Padding(0, 1).
		Width(s.width).
		Render(strings.Join(parts, "\n"))
}

// underlined wraps content in a width-clamped row with a subtle bottom rule.
func (s Select) underlined(content string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.theme.Border).
		Width(s.width - 4).
		Render(content)
}

// renderFilterInput draws "> filter␣" with placeholder fallback.
func (s Select) renderFilterInput() string {
	prompt := lipgloss.NewStyle().Foreground(s.theme.Primary).Bold(true).Render("> ")
	var body string
	if s.filter == "" {
		body = lipgloss.NewStyle().Foreground(s.theme.TextMuted).Italic(true).Render(s.placeholder)
	} else {
		body = lipgloss.NewStyle().Foreground(s.theme.Text).Render(s.filter)
	}
	cursor := lipgloss.NewStyle().Background(s.theme.Text).Foreground(s.theme.Bg).Render(" ")
	return prompt + body + cursor
}

// renderBody draws the scrolled list, or a muted empty-state line.
func (s Select) renderBody() string {
	vis := s.visible()
	if len(vis) == 0 {
		empty := "(empty)"
		if s.filterOn {
			empty = "(no matches)"
		}
		return lipgloss.NewStyle().Foreground(s.theme.TextMuted).Italic(true).Render(empty)
	}
	end := s.offset + s.height
	if end > len(vis) {
		end = len(vis)
	}
	rows := make([]string, 0, end-s.offset)
	for i := s.offset; i < end; i++ {
		rows = append(rows, s.renderRow(vis[i], i == s.cursor))
	}
	return strings.Join(rows, "\n")
}

// renderRow draws one option: cursor mark + label + right-aligned hint.
func (s Select) renderRow(o Option, active bool) string {
	rowWidth := s.width - 4
	labelStyle := lipgloss.NewStyle().Foreground(s.theme.Text)
	hintStyle := lipgloss.NewStyle().Foreground(s.theme.TextMuted)
	mark := "  "
	if active {
		mark = lipgloss.NewStyle().Foreground(s.theme.Primary).Render("› ")
		labelStyle = labelStyle.Foreground(s.theme.PrimaryStrong).Bold(true)
	}
	label := labelStyle.Render(o.Label)
	hint := ""
	if o.Hint != "" {
		hint = hintStyle.Render(o.Hint)
	}
	gap := rowWidth - lipgloss.Width(mark) - lipgloss.Width(label) - lipgloss.Width(hint)
	if gap < 1 {
		gap = 1
	}
	row := mark + label + strings.Repeat(" ", gap) + hint
	if active {
		row = lipgloss.NewStyle().Background(s.theme.SurfaceStrong).Width(rowWidth).Render(row)
	}
	return row
}
