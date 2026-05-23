// Package statcard renders a dashboard metric tile: a single bordered
// card with a small uppercase label, a big bold value, and an optional
// trend row (glyph + delta + italic sublabel). Compose multiple cards
// in a horizontal row with lipgloss.JoinHorizontal to open any agent
// surface — engagements, PRs, revenue, latency.
//
// Stat-cards are stateless from a TUI perspective: every Update is a
// no-op. The Bubble Tea Init/Update/View shape is still exposed so a
// parent model can drop a Model into its struct, re-render on every
// tick, and rebuild cards by chaining the immutable builders.
package statcard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Trend chooses the glyph and color of the delta row.
type Trend int

const (
	// TrendNeutral renders an em-dash glyph in muted text.
	TrendNeutral Trend = iota
	// TrendUp renders an upward triangle in success color.
	TrendUp
	// TrendDown renders a downward triangle in error color.
	TrendDown
)

// Trend glyphs. Up/Down are BMP triangles that render in every modern
// terminal font; neutral is an em-dash, which reads as "no change" without
// stealing visual weight from the value above it.
const (
	glyphUp      = "▲"
	glyphDown    = "▼"
	glyphNeutral = "—"
)

// Layout constants. The card is a fixed shape: 1-cell border, 1-cell
// horizontal padding, 1-cell vertical padding. outer = text + 4, where
// 4 = 2 borders + 2 padding columns.
const (
	padX        = 1
	padY        = 1
	borderCells = 2
	chromeCells = borderCells + 2*padX // 4
)

// Model is the immutable stat-card. Each With… builder returns a new
// Model so callers can compose cards inline in their View().
type Model struct {
	theme    theme.Theme
	label    string
	value    string
	delta    string
	sublabel string
	trend    Trend
	width    int  // outer width in cells; 0 = auto
	emphasis bool // primary tile styling: rounded border, surface-strong bg
}

// New constructs a Model bound to theme.Default. Set fields with the
// chain-builders before calling View().
func New() Model {
	return Model{theme: theme.Default}
}

// NewWithTheme is the same as New but lets the parent inject a non-default
// palette (light theme, alt theme, etc.).
func NewWithTheme(t theme.Theme) Model {
	return Model{theme: t}
}

// WithLabel sets the small uppercase label on line 1. Conventionally a
// noun phrase like "Open issues" or "Revenue".
func (m Model) WithLabel(s string) Model { m.label = s; return m }

// WithValue sets the big bold value on line 2. Free-form string —
// "12", "$499/mo", "1.2k", "00:42". No internal formatting; pass the
// already-formatted display string.
func (m Model) WithValue(s string) Model { m.value = s; return m }

// WithDelta sets the small colored change indicator on line 3, e.g.
// "+3", "-12%", "0". The trend chooses the color.
func (m Model) WithDelta(s string) Model { m.delta = s; return m }

// WithTrend chooses the trend glyph and color: up=success, down=error,
// neutral=muted.
func (m Model) WithTrend(t Trend) Model { m.trend = t; return m }

// WithSublabel sets the italic muted line under the value, e.g.
// "since last week", "across 24 repos".
func (m Model) WithSublabel(s string) Model { m.sublabel = s; return m }

// WithWidth fixes the outer width in cells. 0 means auto-size to the
// widest of label/value/(delta+sublabel) plus padding and borders.
// Values smaller than the minimum viable card (chromeCells+1 = 5)
// clamp up so the card always renders something.
func (m Model) WithWidth(w int) Model {
	if w < 0 {
		w = 0
	}
	if w > 0 && w < chromeCells+1 {
		w = chromeCells + 1
	}
	m.width = w
	return m
}

// WithEmphasis bumps the card to "primary tile" styling: rounded border
// over the default normal border, SurfaceStrong over Surface, and a
// brighter border color. Use sparingly — one or two per row.
func (m Model) WithEmphasis(on bool) Model { m.emphasis = on; return m }

// Width returns the rendered outer width in cells.
func (m Model) Width() int {
	return m.outerWidth()
}

// Height returns the rendered outer height in cells. 6 for a
// label+value card, 7 with a delta or sublabel.
func (m Model) Height() int {
	// 2 border rows + 2 vertical padding rows + 1 label + 1 value + maybe 1 trend.
	rows := 2 + 2*padY + 1 + 1
	if m.hasTrendRow() {
		rows++
	}
	return rows
}

// Init implements tea.Model. Stat-cards do not animate, so no command
// is issued; this exists so a parent embedding a Model can call Init
// in its own Init chain without a special case.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model. Stat-cards do not respond to messages —
// the parent owns the data and rebuilds the card on every tick. Update
// returns the receiver unchanged. WindowSizeMsg in particular is a no-op:
// composing cards into a layout is the parent's responsibility.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	_ = msg
	return m, nil
}

// View renders the card. Safe to call repeatedly; no side effects.
func (m Model) View() string {
	textW := m.textWidth()
	bg := m.cardBg()

	labelLine := m.renderLabel(textW, bg)
	valueLine := m.renderValue(textW, bg)
	rows := []string{labelLine, valueLine}
	if m.hasTrendRow() {
		rows = append(rows, m.renderTrendRow(textW, bg))
	}

	body := strings.Join(rows, "\n")

	border := lipgloss.NormalBorder()
	borderColor := m.theme.Border
	if m.emphasis {
		border = lipgloss.RoundedBorder()
		borderColor = m.theme.BorderStrong
	}

	// lipgloss.Width(W) here is the inter-border width INCLUDING padding,
	// so outer = W + 2 borders. We want outer == m.outerWidth(), so
	// W = outer - 2.
	contentW := m.outerWidth() - borderCells

	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Background(bg).
		Padding(padY, padX).
		Width(contentW).
		Render(body)
}

// hasTrendRow reports whether the third (trend) row should render.
func (m Model) hasTrendRow() bool {
	return m.delta != "" || m.sublabel != ""
}

// outerWidth resolves the effective outer width, applying the auto-size
// rule when none was configured.
func (m Model) outerWidth() int {
	if m.width > 0 {
		return m.width
	}
	return m.autoTextWidth() + chromeCells
}

// textWidth is the visible cell width of each inner row, before the
// outer style adds horizontal padding and borders.
func (m Model) textWidth() int {
	w := m.outerWidth() - chromeCells
	if w < 1 {
		w = 1
	}
	return w
}

// autoTextWidth picks the inner text width by taking the widest of the
// three logical rows — no truncation, no minimum below 1.
func (m Model) autoTextWidth() int {
	w := lipgloss.Width(strings.ToUpper(m.label))
	if vw := lipgloss.Width(m.value); vw > w {
		w = vw
	}
	if m.hasTrendRow() {
		if tw := lipgloss.Width(m.trendRowPlain()); tw > w {
			w = tw
		}
	}
	if w < 1 {
		w = 1
	}
	return w
}

// renderLabel renders line 1 — uppercased, muted, slightly bold. Reads
// as a "small caps" label sitting above the big value.
func (m Model) renderLabel(width int, bg lipgloss.Color) string {
	text := truncate(strings.ToUpper(m.label), width)
	return lipgloss.NewStyle().
		Foreground(m.theme.TextMuted).
		Background(bg).
		Bold(true).
		Width(width).
		Render(text)
}

// renderValue renders line 2 — the big bold value. "Big" in a terminal
// means Bold + full card width + clearly heavier visual weight than the
// muted label above and the small trend row below.
func (m Model) renderValue(width int, bg lipgloss.Color) string {
	val := truncate(m.value, width)
	if val == "" {
		val = " "
	}
	return lipgloss.NewStyle().
		Foreground(m.theme.Text).
		Background(bg).
		Bold(true).
		Width(width).
		Align(lipgloss.Left).
		Render(val)
}

// renderTrendRow renders line 3 — glyph + delta in trend color +
// italic muted sublabel. Truncates to width.
func (m Model) renderTrendRow(width int, bg lipgloss.Color) string {
	g, fg := m.trendGlyphAndColor()

	parts := []string{lipgloss.NewStyle().Foreground(fg).Background(bg).Render(g)}
	if m.delta != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(fg).Background(bg).Bold(true).Render(m.delta))
	}
	if m.sublabel != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(m.theme.TextMuted).Background(bg).Italic(true).Render(m.sublabel))
	}
	row := strings.Join(parts, " ")
	row = truncateANSI(row, width)
	// Pad to exact width so the background fills the row evenly.
	if cw := lipgloss.Width(row); cw < width {
		row += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", width-cw))
	}
	return row
}

// trendRowPlain returns the unstyled width-relevant form of the trend
// row, used for auto-sizing.
func (m Model) trendRowPlain() string {
	g, _ := m.trendGlyphAndColor()
	parts := []string{g}
	if m.delta != "" {
		parts = append(parts, m.delta)
	}
	if m.sublabel != "" {
		parts = append(parts, m.sublabel)
	}
	return strings.Join(parts, " ")
}

// trendGlyphAndColor returns the trend glyph plus its foreground color.
func (m Model) trendGlyphAndColor() (string, lipgloss.Color) {
	switch m.trend {
	case TrendUp:
		return glyphUp, m.theme.Success
	case TrendDown:
		// theme.go has no "Danger" token; Error is the strong red. Per
		// the spec's fallback note, use Error.
		return glyphDown, m.theme.Error
	default:
		return glyphNeutral, m.theme.TextMuted
	}
}

// cardBg returns the background color used for the card surface.
func (m Model) cardBg() lipgloss.Color {
	if m.emphasis {
		return m.theme.SurfaceStrong
	}
	return m.theme.Surface
}

// truncate truncates s to a visible width of n cells, appending an
// ellipsis when the original was wider. Operates on rune-width via
// lipgloss.Width so multi-cell glyphs (CJK, emoji) measure correctly.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	var b strings.Builder
	used := 0
	target := n - 1 // leave room for the ellipsis
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > target {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	b.WriteString("…")
	return b.String()
}

// truncateANSI truncates a possibly-styled string to n visible cells.
// If the string is already short enough, it's returned as-is. The
// truncation walks past ANSI escape sequences without counting them
// against the visible width.
func truncateANSI(s string, n int) string {
	if lipgloss.Width(s) <= n {
		return s
	}
	if n <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	inEsc := false
	target := n - 1
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == 0x1b {
			inEsc = true
			b.WriteRune(r)
			continue
		}
		if inEsc {
			b.WriteRune(r)
			if r == 'm' || r == 'K' || r == 'J' || r == 'H' {
				inEsc = false
			}
			continue
		}
		rw := lipgloss.Width(string(r))
		if used+rw > target {
			b.WriteString("…")
			// Drain any trailing escapes (resets) so the line closes cleanly.
			for j := i; j < len(runes); j++ {
				rj := runes[j]
				if rj == 0x1b {
					inEsc = true
				}
				if inEsc {
					b.WriteRune(rj)
					if rj == 'm' || rj == 'K' || rj == 'J' || rj == 'H' {
						inEsc = false
					}
				}
			}
			return b.String()
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String()
}
