// Package panel wraps arbitrary content in a bordered container with
// an optional title and footer. It's the workhorse layout primitive:
// almost every other component pairs well inside a Panel.
//
// A Panel doesn't update or animate; it's a pure render wrapper.
// Set width to clamp the inner content area; height 0 means
// "natural height" (the content's line count).
package panel

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Variant tunes border weight. Default uses theme.Border (subtle);
// Strong uses theme.BorderStrong (high contrast).
type Variant int

const (
	VariantDefault Variant = iota
	VariantStrong
)

// Border glyphs.
const (
	tlCorner = "╭"
	trCorner = "╮"
	blCorner = "╰"
	brCorner = "╯"
	hLine    = "─"
	vLine    = "│"
)

// Panel is a bordered container with optional title and footer.
type Panel struct {
	theme   theme.Theme
	title   string
	footer  string
	content string
	width   int
	height  int
	variant Variant
	padX    int
	padY    int
}

// New constructs a Panel using the default variant.
func New(t theme.Theme) Panel {
	return Panel{theme: t, padX: 1, padY: 0}
}

func (p Panel) WithTitle(title string) Panel   { p.title = title; return p }
func (p Panel) WithFooter(footer string) Panel { p.footer = footer; return p }
func (p Panel) WithContent(c string) Panel     { p.content = c; return p }

// WithWidth clamps the outer panel width (including borders).
// Values <= 0 mean natural width.
func (p Panel) WithWidth(w int) Panel {
	if w < 0 {
		w = 0
	}
	p.width = w
	return p
}

// WithHeight clamps the outer panel height (including borders).
// Values <= 0 mean natural height.
func (p Panel) WithHeight(h int) Panel {
	if h < 0 {
		h = 0
	}
	p.height = h
	return p
}

// WithVariant selects the border weight.
func (p Panel) WithVariant(v Variant) Panel { p.variant = v; return p }

// WithPadding sets horizontal and vertical padding inside the borders.
func (p Panel) WithPadding(x, y int) Panel {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	p.padX, p.padY = x, y
	return p
}

// View renders the bordered panel.
func (p Panel) View() string {
	borderColor := p.theme.Border
	if p.variant == VariantStrong {
		borderColor = p.theme.BorderStrong
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	textStyle := lipgloss.NewStyle().Foreground(p.theme.Text)
	titleStyle := lipgloss.NewStyle().Foreground(p.theme.Text)
	footerStyle := lipgloss.NewStyle().Foreground(p.theme.TextMuted)

	bodyLines := strings.Split(p.content, "\n")

	// Figure out the inner width (content area between vertical borders).
	naturalContentWidth := 0
	for _, line := range bodyLines {
		if w := lipgloss.Width(line); w > naturalContentWidth {
			naturalContentWidth = w
		}
	}
	inner := naturalContentWidth + 2*p.padX
	if p.width > 0 {
		// Subtract the two border columns.
		inner = p.width - 2
		if inner < 0 {
			inner = 0
		}
	}
	// Reserve room for title decorations on the top border: ─ title ─
	// at minimum needs len(title)+4. If inner is too small, ignore title.
	if p.title != "" {
		minInner := lipgloss.Width(p.title) + 4
		if inner < minInner {
			inner = minInner
		}
	}
	if p.footer != "" {
		minInner := lipgloss.Width(p.footer) + 4
		if inner < minInner {
			inner = minInner
		}
	}

	// Top border with optional title.
	top := buildBorder(p.title, inner, borderStyle, titleStyle, tlCorner, trCorner)
	bottom := buildBorder(p.footer, inner, borderStyle, footerStyle, blCorner, brCorner)

	// Build content rows.
	rows := make([]string, 0, len(bodyLines)+2*p.padY)
	for i := 0; i < p.padY; i++ {
		rows = append(rows, contentRow("", inner, p.padX, borderStyle, textStyle))
	}
	for _, line := range bodyLines {
		rows = append(rows, contentRow(line, inner, p.padX, borderStyle, textStyle))
	}
	for i := 0; i < p.padY; i++ {
		rows = append(rows, contentRow("", inner, p.padX, borderStyle, textStyle))
	}

	// Height clamp: trim or pad rows to match height - 2 (the two border rows).
	if p.height > 0 {
		targetInner := p.height - 2
		if targetInner < 0 {
			targetInner = 0
		}
		if len(rows) > targetInner {
			rows = rows[:targetInner]
		} else {
			for len(rows) < targetInner {
				rows = append(rows, contentRow("", inner, p.padX, borderStyle, textStyle))
			}
		}
	}

	all := append([]string{top}, rows...)
	all = append(all, bottom)
	return strings.Join(all, "\n")
}

// buildBorder builds a top or bottom border line with an optional label
// inlaid two columns from the left corner.
func buildBorder(label string, inner int, borderStyle, labelStyle lipgloss.Style, left, right string) string {
	if label == "" {
		return borderStyle.Render(left + strings.Repeat(hLine, inner) + right)
	}
	labelStr := " " + label + " "
	labelW := lipgloss.Width(labelStr)
	// Layout: corner + ─ + labelStr + ─...─ + corner
	leftFill := 1
	rightFill := inner - leftFill - labelW
	if rightFill < 1 {
		rightFill = 1
	}
	return borderStyle.Render(left+strings.Repeat(hLine, leftFill)) +
		labelStyle.Render(labelStr) +
		borderStyle.Render(strings.Repeat(hLine, rightFill)+right)
}

// contentRow renders one body row between vertical borders.
func contentRow(line string, inner, padX int, borderStyle, textStyle lipgloss.Style) string {
	content := strings.Repeat(" ", padX) + line
	cw := lipgloss.Width(content)
	if cw < inner {
		content += strings.Repeat(" ", inner-cw)
	} else if cw > inner {
		// Truncate (visible width).
		content = lipgloss.NewStyle().MaxWidth(inner).Render(content)
	}
	return borderStyle.Render(vLine) + textStyle.Render(content) + borderStyle.Render(vLine)
}
