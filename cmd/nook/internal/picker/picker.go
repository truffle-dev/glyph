// Package picker is a fuzzy-filtered selection overlay for nook. It is the
// underlying primitive behind file picker, buffer picker, symbol picker, and
// branch picker.
//
// An Item carries a Title (rendered), an optional Subtitle, optional Group
// label, and an opaque Value. Consumers feed items in, route keys through
// Update, and observe SelectMsg / CancelMsg.
//
// The default matcher is a subsequence-with-bonuses scorer (start-of-string,
// word-boundary, consecutive). Replace it with WithMatcher when needed.
package picker

import (
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Item is one row.
type Item struct {
	Title    string // primary label
	Subtitle string // optional secondary label, dimmed
	Group    string // optional group label, used by matchers
	Value    any    // opaque payload returned in SelectMsg
}

// SelectMsg is emitted when the user presses Enter on a non-empty result list.
type SelectMsg struct {
	Item Item
}

// CancelMsg is emitted when the user presses Esc.
type CancelMsg struct{}

// Matcher returns a score for it against query. Zero score drops the item;
// higher scores rank earlier.
type Matcher func(it Item, query string) int

// PreviewFunc renders preview content for the highlighted item. Return "" to
// skip the preview pane for that item.
type PreviewFunc func(it Item) string

// Picker is the model.
type Picker struct {
	theme       theme.Theme
	items       []Item
	filter      string
	cursor      int // cursor index into filtered results
	width       int
	height      int
	title       string
	placeholder string
	matcher     Matcher
	preview     PreviewFunc
}

// New constructs a picker with the default fuzzy matcher.
func New(t theme.Theme) Picker {
	return Picker{
		theme:       t,
		width:       72,
		height:      14,
		title:       "Pick",
		placeholder: "Type to filter…",
		matcher:     SubsequenceMatcher,
	}
}

// WithItems replaces the item set.
func (p Picker) WithItems(items []Item) Picker {
	p.items = append([]Item(nil), items...)
	p.cursor = 0
	return p
}

// AppendItems streams new items into the picker, preserving the cursor.
func (p Picker) AppendItems(items []Item) Picker {
	p.items = append(p.items, items...)
	return p
}

// WithFilter presets the filter text.
func (p Picker) WithFilter(s string) Picker {
	p.filter = s
	p.cursor = 0
	return p
}

// WithSize sets the rendered width and height.
func (p Picker) WithSize(w, h int) Picker {
	if w < 24 {
		w = 24
	}
	if h < 6 {
		h = 6
	}
	p.width = w
	p.height = h
	return p
}

// WithTitle sets the title rendered above the input.
func (p Picker) WithTitle(s string) Picker { p.title = s; return p }

// WithPlaceholder sets the input placeholder.
func (p Picker) WithPlaceholder(s string) Picker { p.placeholder = s; return p }

// WithMatcher replaces the matching function.
func (p Picker) WithMatcher(m Matcher) Picker {
	if m == nil {
		m = SubsequenceMatcher
	}
	p.matcher = m
	return p
}

// WithPreview wires a preview renderer. If set and the picker is wide enough,
// the right half shows preview content for the highlighted item.
func (p Picker) WithPreview(f PreviewFunc) Picker { p.preview = f; return p }

// Filter returns the current filter string.
func (p Picker) Filter() string { return p.filter }

// Count returns the number of items currently matching the filter.
func (p Picker) Count() int { return len(p.matches()) }

// TotalCount returns the number of items in the picker, before filtering.
func (p Picker) TotalCount() int { return len(p.items) }

// Highlighted returns the currently highlighted item (if any).
func (p Picker) Highlighted() (Item, bool) {
	m := p.matches()
	if len(m) == 0 || p.cursor < 0 || p.cursor >= len(m) {
		return Item{}, false
	}
	return m[p.cursor].item, true
}

type scored struct {
	item  Item
	score int
}

// matches returns the filtered+sorted item set.
func (p Picker) matches() []scored {
	if p.filter == "" {
		out := make([]scored, len(p.items))
		for i, it := range p.items {
			out[i] = scored{item: it, score: 1}
		}
		return out
	}
	out := make([]scored, 0, len(p.items))
	for _, it := range p.items {
		if s := p.matcher(it, p.filter); s > 0 {
			out = append(out, scored{item: it, score: s})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

// Update routes key events. It returns a tea.Cmd carrying SelectMsg or
// CancelMsg when those events occur.
func (p Picker) Update(msg tea.Msg) (Picker, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	matches := p.matches()
	switch km.Type {
	case tea.KeyEsc:
		return p, func() tea.Msg { return CancelMsg{} }
	case tea.KeyEnter:
		if len(matches) == 0 {
			return p, nil
		}
		if p.cursor >= len(matches) {
			p.cursor = len(matches) - 1
		}
		it := matches[p.cursor].item
		return p, func() tea.Msg { return SelectMsg{Item: it} }
	case tea.KeyUp, tea.KeyCtrlP:
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case tea.KeyDown, tea.KeyCtrlN:
		if p.cursor < len(matches)-1 {
			p.cursor++
		}
		return p, nil
	case tea.KeyBackspace:
		if len(p.filter) > 0 {
			r := []rune(p.filter)
			p.filter = string(r[:len(r)-1])
			p.cursor = 0
		}
		return p, nil
	case tea.KeyCtrlU:
		p.filter = ""
		p.cursor = 0
		return p, nil
	case tea.KeySpace:
		p.filter += " "
		p.cursor = 0
		return p, nil
	case tea.KeyRunes:
		p.filter += string(km.Runes)
		p.cursor = 0
		return p, nil
	}
	return p, nil
}

// View renders the picker.
func (p Picker) View() string {
	t := p.theme
	border := lipgloss.NewStyle().
		Foreground(t.Border).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Bold(true)
	filterStyle := lipgloss.NewStyle().Foreground(t.Text)
	placeStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true)
	cursorRow := lipgloss.NewStyle().Foreground(t.TextInverse).Background(t.Primary).Bold(true)
	dim := lipgloss.NewStyle().Foreground(t.TextMuted)
	subtitleStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true)
	countStyle := lipgloss.NewStyle().Foreground(t.TextMuted)

	listWidth := p.width
	previewWidth := 0
	if p.preview != nil && p.width >= 90 {
		previewWidth = p.width / 2
		listWidth = p.width - previewWidth - 2
	}

	// header
	filterLine := p.filter
	if filterLine == "" {
		filterLine = placeStyle.Render(p.placeholder)
	} else {
		filterLine = filterStyle.Render(filterLine)
	}
	header := titleStyle.Render(p.title) + "  " + dim.Render("›") + " " + filterLine

	// body rows
	matches := p.matches()
	bodyH := p.height - 4 // header + count line + 2 borders
	if bodyH < 1 {
		bodyH = 1
	}
	start := 0
	if p.cursor >= bodyH {
		start = p.cursor - bodyH + 1
	}
	end := start + bodyH
	if end > len(matches) {
		end = len(matches)
	}

	var rows []string
	for i := start; i < end; i++ {
		it := matches[i].item
		line := it.Title
		if it.Subtitle != "" {
			line = it.Title + "  " + subtitleStyle.Render(it.Subtitle)
		}
		if i == p.cursor {
			line = cursorRow.Render(padRight(stripStyle(line), listWidth-2))
		} else {
			line = padRight(line, listWidth-2)
		}
		rows = append(rows, line)
	}
	for len(rows) < bodyH {
		rows = append(rows, padRight("", listWidth-2))
	}
	body := strings.Join(rows, "\n")

	// count line
	count := countStyle.Render(formatCount(len(matches), len(p.items)))

	listPanel := border.Width(listWidth).Render(strings.Join([]string{header, body, count}, "\n"))

	if previewWidth == 0 {
		return listPanel
	}

	// preview pane
	prevContent := ""
	if it, ok := p.Highlighted(); ok {
		prevContent = p.preview(it)
	}
	prevHeader := titleStyle.Render("Preview")
	prevBody := truncateLines(prevContent, p.height-4, previewWidth-2)
	prevPanel := border.Width(previewWidth).Render(prevHeader + "\n" + prevBody)

	return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, " ", prevPanel)
}

// --- matchers ---

// SubsequenceMatcher scores items by subsequence match with bonuses for
// start-of-string, word-boundary, and consecutive runs. Returns 0 if query
// chars don't all appear in order.
func SubsequenceMatcher(it Item, query string) int {
	target := it.Title
	if it.Group != "" {
		target = it.Group + "/" + it.Title
	}
	return Score(target, query)
}

// Score evaluates the fuzzy match of query against target. Returns 0 if query
// is not a subsequence of target. Higher = better match.
func Score(target, query string) int {
	if query == "" {
		return 1
	}
	tl := strings.ToLower(target)
	ql := strings.ToLower(query)
	tr := []rune(tl)
	qr := []rune(ql)

	pos := 0
	score := 0
	prevMatch := -2
	for _, q := range qr {
		found := -1
		for i := pos; i < len(tr); i++ {
			if tr[i] == q {
				found = i
				break
			}
		}
		if found < 0 {
			return 0
		}
		// bonuses
		bonus := 1
		if found == 0 {
			bonus += 8
		} else if isBoundary(tr[found-1]) {
			bonus += 4
		}
		if found == prevMatch+1 {
			bonus += 3 // consecutive run
		}
		score += bonus
		prevMatch = found
		pos = found + 1
	}
	// shorter targets score slightly higher (favors exact matches)
	score += max(0, 16-len(tr)/4)
	return score
}

func isBoundary(r rune) bool {
	switch r {
	case '/', '\\', '_', '-', '.', ' ':
		return true
	}
	return unicode.IsUpper(r)
}

// --- helpers ---

func padRight(s string, w int) string {
	if w <= 0 {
		return ""
	}
	n := lipgloss.Width(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

func stripStyle(s string) string {
	// best-effort plain-text length for highlighted row width calc
	return s
}

func formatCount(matched, total int) string {
	if matched == total {
		return formatNum(total) + " items"
	}
	return formatNum(matched) + " / " + formatNum(total)
}

func formatNum(n int) string {
	if n < 1000 {
		return itoa(n)
	}
	s := itoa(n)
	// insert thousand separators
	var out []byte
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(r))
	}
	return string(out)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		n = -n
		neg = true
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func truncateLines(s string, maxLines, maxWidth int) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	for i, ln := range lines {
		if lipgloss.Width(ln) > maxWidth {
			rs := []rune(ln)
			if maxWidth > 0 && maxWidth < len(rs) {
				rs = rs[:maxWidth]
			}
			lines[i] = string(rs)
		}
	}
	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
