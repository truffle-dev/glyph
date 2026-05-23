// Package findbar renders an in-buffer search bar — the narrow query
// input that overlays an editor when the user hits Ctrl-F. It owns
// nothing but the query, the active match index, and the navigation
// buttons; the actual buffer-side search is performed by the consumer
// (FindMatches walks any "lines" string for plain or case-insensitive
// matches; if the consumer wants fuzzy or regex they wire their own).
//
// Composition shape:
//
//	bar := findbar.New(theme.Default)
//	bar = bar.WithQuery(q).WithMatches(matches, current)
//	// In Update: pass tea.KeyMsg to bar.Update; act on its messages.
//
// The bar emits four messages: QueryMsg{Value} when the input changes,
// NextMsg / PrevMsg when the user hits Enter/F3/Shift-F3, CloseMsg
// when the user hits Esc. The consumer is the one that scrolls the
// editor to the next match — the bar is purely an input.
package findbar

import (
	"fmt"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Match is a single hit inside the buffer. Row is 0-based; ColStart
// and ColEnd are rune indices (start-inclusive, end-exclusive).
type Match struct {
	Row      int
	ColStart int
	ColEnd   int
}

// QueryMsg is emitted whenever the query value changes. The consumer
// should re-run FindMatches and reseat the bar via WithMatches.
type QueryMsg struct{ Value string }

// NextMsg / PrevMsg are emitted when the user requests cursor motion.
type (
	NextMsg struct{}
	PrevMsg struct{}
)

// CloseMsg is emitted when the user hits Esc.
type CloseMsg struct{}

// Bar is a Bubble Tea model for the find bar.
type Bar struct {
	theme        theme.Theme
	query        []rune
	col          int // cursor in the query, rune-indexed
	caseSensitive bool
	width        int
	focused      bool

	matches []Match
	current int // 0-based; -1 when no matches
}

// New constructs a Bar focused, 48 cells wide, with no matches yet.
func New(t theme.Theme) Bar {
	return Bar{
		theme:   t,
		width:   48,
		focused: true,
		current: -1,
	}
}

// WithWidth sets the bar width in cells. Clamped to >= 24.
func (b Bar) WithWidth(w int) Bar {
	if w < 24 {
		w = 24
	}
	b.width = w
	return b
}

// WithQuery presets the query string and parks the cursor at end.
func (b Bar) WithQuery(s string) Bar {
	b.query = []rune(s)
	b.col = len(b.query)
	return b
}

// WithCaseSensitive toggles case-sensitive matching for FindMatches.
func (b Bar) WithCaseSensitive(on bool) Bar { b.caseSensitive = on; return b }

// WithMatches replaces the match set; current is clamped to a valid
// index or -1 when empty.
func (b Bar) WithMatches(matches []Match, current int) Bar {
	b.matches = matches
	if len(matches) == 0 {
		b.current = -1
		return b
	}
	if current < 0 {
		current = 0
	}
	if current >= len(matches) {
		current = len(matches) - 1
	}
	b.current = current
	return b
}

// Focus / Blur control whether the bar consumes key events.
func (b Bar) Focus() Bar { b.focused = true; return b }
func (b Bar) Blur() Bar  { b.focused = false; return b }

// Focused reports whether the bar is accepting input.
func (b Bar) Focused() bool { return b.focused }

// Query returns the current query string.
func (b Bar) Query() string { return string(b.query) }

// CaseSensitive reports the bar's current match mode.
func (b Bar) CaseSensitive() bool { return b.caseSensitive }

// Current returns the active match index, or -1 if none.
func (b Bar) Current() int { return b.current }

// MatchCount returns the size of the bar's match set.
func (b Bar) MatchCount() int { return len(b.matches) }

// CurrentMatch returns the active match and true when one exists.
func (b Bar) CurrentMatch() (Match, bool) {
	if b.current < 0 || b.current >= len(b.matches) {
		return Match{}, false
	}
	return b.matches[b.current], true
}

// Init implements tea.Model.
func (b Bar) Init() tea.Cmd { return nil }

// Update implements tea.Model. It returns its own messages via Cmd
// rather than mutating any parent state; the consumer is expected to
// re-issue queries and navigate the editor on each message.
func (b Bar) Update(msg tea.Msg) (Bar, tea.Cmd) {
	if !b.focused {
		return b, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return b, nil
	}
	switch key.Type {
	case tea.KeyEsc:
		return b, func() tea.Msg { return CloseMsg{} }
	case tea.KeyEnter:
		if key.Alt {
			return b, func() tea.Msg { return PrevMsg{} }
		}
		return b, func() tea.Msg { return NextMsg{} }
	case tea.KeyF3:
		if key.Alt {
			return b, func() tea.Msg { return PrevMsg{} }
		}
		return b, func() tea.Msg { return NextMsg{} }
	case tea.KeyLeft:
		if b.col > 0 {
			b.col--
		}
		return b, nil
	case tea.KeyRight:
		if b.col < len(b.query) {
			b.col++
		}
		return b, nil
	case tea.KeyHome:
		b.col = 0
		return b, nil
	case tea.KeyEnd:
		b.col = len(b.query)
		return b, nil
	case tea.KeyBackspace:
		if b.col == 0 {
			return b, nil
		}
		b.query = append(append([]rune{}, b.query[:b.col-1]...), b.query[b.col:]...)
		b.col--
		v := string(b.query)
		return b, func() tea.Msg { return QueryMsg{Value: v} }
	case tea.KeyDelete:
		if b.col == len(b.query) {
			return b, nil
		}
		b.query = append(append([]rune{}, b.query[:b.col]...), b.query[b.col+1:]...)
		v := string(b.query)
		return b, func() tea.Msg { return QueryMsg{Value: v} }
	case tea.KeyCtrlU:
		b.query = b.query[:0]
		b.col = 0
		return b, func() tea.Msg { return QueryMsg{Value: ""} }
	case tea.KeySpace:
		return b.insertRunes([]rune{' '})
	case tea.KeyRunes:
		return b.insertRunes(key.Runes)
	}
	return b, nil
}

func (b Bar) insertRunes(rs []rune) (Bar, tea.Cmd) {
	out := make([]rune, 0, len(b.query)+len(rs))
	out = append(out, b.query[:b.col]...)
	out = append(out, rs...)
	out = append(out, b.query[b.col:]...)
	b.query = out
	b.col += len(rs)
	v := string(b.query)
	return b, func() tea.Msg { return QueryMsg{Value: v} }
}

// View renders the bar: query input on the left, match counter on the
// right ("3 / 12" or "no matches"), wrapped in a single-line bordered
// frame the same height as a status-bar row.
func (b Bar) View() string {
	// Counter chip
	chip := "no matches"
	chipStyle := lipgloss.NewStyle().
		Foreground(b.theme.TextMuted).
		Padding(0, 1)
	if len(b.matches) > 0 {
		chip = fmt.Sprintf("%d / %d", b.current+1, len(b.matches))
		chipStyle = chipStyle.Foreground(b.theme.Text).Background(b.theme.SurfaceStrong)
	} else if len(b.query) == 0 {
		chip = ""
	}
	chipText := chipStyle.Render(chip)
	chipW := lipgloss.Width(chipText)

	// Query input field (1 line)
	inputW := b.width - 4 - chipW - 1 // border 2 + pad 2 + chip + gap
	if inputW < 8 {
		inputW = 8
	}
	input := b.renderInput(inputW)

	row := input + " " + chipText

	border := b.theme.Border
	if b.focused {
		border = b.theme.PrimaryStrong
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(b.width).
		Render(row)
}

func (b Bar) renderInput(width int) string {
	prefix := lipgloss.NewStyle().Foreground(b.theme.Primary).Render("find ")
	prefixW := lipgloss.Width(prefix)
	contentW := width - prefixW
	if contentW < 4 {
		contentW = 4
	}

	if len(b.query) == 0 && !b.focused {
		placeholder := lipgloss.NewStyle().
			Foreground(b.theme.TextMuted).
			Render(truncate("type to search", contentW))
		return prefix + placeholder
	}

	textStyle := lipgloss.NewStyle().Foreground(b.theme.Text)
	cursorStyle := lipgloss.NewStyle().
		Background(b.theme.Text).
		Foreground(b.theme.Bg)

	visible := append([]rune{}, b.query...)
	col := b.col
	if len(visible) > contentW-1 {
		half := contentW / 2
		start := col - half
		if start < 0 {
			start = 0
		}
		end := start + contentW - 1
		if end > len(visible) {
			end = len(visible)
			start = end - (contentW - 1)
			if start < 0 {
				start = 0
			}
		}
		visible = visible[start:end]
		col = b.col - start
	}
	if col < 0 {
		col = 0
	}
	if col > len(visible) {
		col = len(visible)
	}

	if !b.focused {
		return prefix + textStyle.Render(string(visible))
	}

	before := textStyle.Render(string(visible[:col]))
	cursorCh := " "
	afterStart := col
	if col < len(visible) {
		cursorCh = string(visible[col])
		afterStart = col + 1
	}
	cursor := cursorStyle.Render(cursorCh)
	after := textStyle.Render(string(visible[afterStart:]))
	return prefix + before + cursor + after
}

// FindMatches is a stateless helper: walk lines for every occurrence
// of query and return the match set in document order. Empty queries
// return nil. The bar passes b.CaseSensitive() through; consumers can
// call FindMatches directly with any boolean.
func FindMatches(lines []string, query string, caseSensitive bool) []Match {
	if query == "" {
		return nil
	}
	if !caseSensitive {
		query = strings.ToLower(query)
	}
	queryRunes := []rune(query)
	out := []Match{}
	for r, line := range lines {
		hay := []rune(line)
		if !caseSensitive {
			hay = []rune(strings.ToLower(line))
		}
		i := 0
		for i <= len(hay)-len(queryRunes) {
			if runeSliceEqual(hay[i:i+len(queryRunes)], queryRunes) {
				out = append(out, Match{Row: r, ColStart: i, ColEnd: i + len(queryRunes)})
				i += len(queryRunes)
				continue
			}
			i++
		}
	}
	return out
}

func runeSliceEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// truncate clips s to max visible cells, appending "…" when cut.
func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max < 2 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

// isPrintable reports whether r is printable (used to defend against
// pasting control characters into the query). Currently unused but
// kept for v0.2 when paste support lands.
func isPrintable(r rune) bool {
	return unicode.IsPrint(r)
}

var _ = isPrintable
