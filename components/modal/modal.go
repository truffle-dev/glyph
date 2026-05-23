// Package modal renders a bordered overlay container with an optional
// inlaid title, framed body, optional footer, and a configurable close key.
//
// The modal owns its own box (title bar, body, footer) and the close-key
// handler. It does not own its position relative to a parent — that's the
// parent's job via lipgloss.Place. Keys the modal does not consume are
// returned untouched so the parent or contained widget can handle them.
package modal

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// CloseMsg is emitted when the user presses the configured close key.
type CloseMsg struct{}

// Border glyphs — match the panel component for visual consistency.
const (
	tlCorner = "╭"
	trCorner = "╮"
	blCorner = "╰"
	brCorner = "╯"
	hLine    = "─"
	vLine    = "│"
)

// Minimum outer dimensions so the box always has room for a border, one
// content row, and (optionally) a footer.
const (
	minWidth  = 10
	minHeight = 4
)

// Modal is a Bubble Tea model for a bordered overlay container.
type Modal struct {
	theme    theme.Theme
	title    string
	body     string
	footer   string
	width    int
	height   int
	closeKey string
}

// New constructs a Modal with sensible defaults: 40x10, Esc to close,
// no title/body/footer.
func New(t theme.Theme) Modal {
	return Modal{
		theme:    t,
		width:    40,
		height:   10,
		closeKey: "esc",
	}
}

// WithTitle sets an optional title rendered inlaid in the top border.
func (m Modal) WithTitle(s string) Modal { m.title = s; return m }

// WithBody sets the pre-rendered body string the modal frames. Multi-line
// bodies are split on "\n". Lines wider than the inner content width are
// truncated by lipgloss.MaxWidth (rune-aware).
func (m Modal) WithBody(s string) Modal { m.body = s; return m }

// WithSize sets the total outer width and height (including border cells).
// Values smaller than the minimum clamp up to (minWidth, minHeight).
func (m Modal) WithSize(w, h int) Modal {
	if w < minWidth {
		w = minWidth
	}
	if h < minHeight {
		h = minHeight
	}
	m.width = w
	m.height = h
	return m
}

// WithCloseKey sets the key that emits CloseMsg. Default is "esc". Pass
// an empty string to disable the close-on-key behavior entirely.
func (m Modal) WithCloseKey(key string) Modal { m.closeKey = key; return m }

// WithFooter sets an optional one-line footer rendered just inside the
// bottom border (e.g. key hints). A non-empty footer subtracts one row
// from the content area.
func (m Modal) WithFooter(s string) Modal { m.footer = s; return m }

// Width returns the total outer width including border cells.
func (m Modal) Width() int { return m.width }

// Height returns the total outer height including border cells.
func (m Modal) Height() int { return m.height }

// ContentWidth returns the inner usable width (outer width minus two
// border columns). Callers can use this to size body content before
// handing it off via WithBody.
func (m Modal) ContentWidth() int {
	c := m.width - 2
	if c < 0 {
		c = 0
	}
	return c
}

// ContentHeight returns the inner usable height (outer height minus two
// border rows, minus one more if a footer is set).
func (m Modal) ContentHeight() int {
	c := m.height - 2
	if m.footer != "" {
		c--
	}
	if c < 0 {
		c = 0
	}
	return c
}

// Init implements tea.Model. The modal has no autonomous behavior.
func (m Modal) Init() tea.Cmd { return nil }

// Update handles the close key only. Every other message — including
// other key events — is returned untouched so the parent or contained
// widget can route it.
func (m Modal) Update(msg tea.Msg) (Modal, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.closeKey == "" {
		return m, nil
	}
	if key.String() == m.closeKey {
		return m, func() tea.Msg { return CloseMsg{} }
	}
	return m, nil
}

// View renders the bordered modal box. Parent overlays it via lipgloss.Place.
func (m Modal) View() string {
	borderStyle := lipgloss.NewStyle().Foreground(m.theme.Primary)
	titleStyle := lipgloss.NewStyle().Foreground(m.theme.Text).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
	footerStyle := lipgloss.NewStyle().Foreground(m.theme.TextMuted)

	inner := m.width - 2
	if inner < 0 {
		inner = 0
	}

	// Top border with optional inlaid title.
	top := buildBorder(m.title, inner, borderStyle, titleStyle, tlCorner, trCorner)
	bottom := buildBorder("", inner, borderStyle, footerStyle, blCorner, brCorner)

	// Body lines fit inside two columns of horizontal padding.
	innerPad := 1
	bodyWidth := inner - 2*innerPad
	if bodyWidth < 0 {
		bodyWidth = 0
	}

	contentRows := m.height - 2 // minus top + bottom borders
	if m.footer != "" {
		contentRows--
	}
	if contentRows < 0 {
		contentRows = 0
	}

	// Build body rows with truncation.
	bodyLines := splitBodyLines(m.body, bodyWidth)
	rows := make([]string, 0, contentRows)
	for i := 0; i < contentRows; i++ {
		var line string
		if i < len(bodyLines) {
			line = bodyLines[i]
			// On the last visible body row, if more lines were dropped,
			// signal truncation with an ellipsis.
			if i == contentRows-1 && len(bodyLines) > contentRows {
				line = truncateRunes(line, bodyWidth-1) + "…"
			}
		}
		rows = append(rows, contentRow(line, inner, innerPad, borderStyle, bodyStyle))
	}

	// Footer row sits just inside the bottom border.
	if m.footer != "" {
		footerLine := truncateRunes(m.footer, bodyWidth)
		rows = append(rows, contentRow(footerLine, inner, innerPad, borderStyle, footerStyle))
	}

	all := append([]string{top}, rows...)
	all = append(all, bottom)
	return strings.Join(all, "\n")
}

// splitBodyLines splits body on "\n" and truncates each line to width.
func splitBodyLines(body string, width int) []string {
	if body == "" {
		return nil
	}
	raw := strings.Split(body, "\n")
	out := make([]string, len(raw))
	for i, ln := range raw {
		out[i] = truncateRunes(ln, width)
	}
	return out
}

// truncateRunes returns s clipped to at most n display cells, using
// lipgloss.MaxWidth so wide runes and styled segments are respected.
// Falls back to a rune-safe slice when MaxWidth's behavior is enough.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(n).Render(s)
}

// buildBorder builds a top or bottom border line with an optional label
// inlaid two columns from the left corner. Mirrors the panel approach.
func buildBorder(label string, inner int, borderStyle, labelStyle lipgloss.Style, left, right string) string {
	if label == "" || inner < lipgloss.Width(label)+4 {
		return borderStyle.Render(left + strings.Repeat(hLine, inner) + right)
	}
	labelStr := " " + label + " "
	labelW := lipgloss.Width(labelStr)
	leftFill := 1
	rightFill := inner - leftFill - labelW
	if rightFill < 1 {
		rightFill = 1
	}
	return borderStyle.Render(left+strings.Repeat(hLine, leftFill)) +
		labelStyle.Render(labelStr) +
		borderStyle.Render(strings.Repeat(hLine, rightFill)+right)
}

// contentRow renders one row between vertical borders, padding to inner.
func contentRow(line string, inner, padX int, borderStyle, textStyle lipgloss.Style) string {
	content := strings.Repeat(" ", padX) + line
	cw := lipgloss.Width(content)
	if cw < inner {
		content += strings.Repeat(" ", inner-cw)
	} else if cw > inner {
		content = lipgloss.NewStyle().MaxWidth(inner).Render(content)
	}
	return borderStyle.Render(vLine) + textStyle.Render(content) + borderStyle.Render(vLine)
}
