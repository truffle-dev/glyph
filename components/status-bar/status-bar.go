// Package statusbar renders a single-line, three-segment status bar.
// Items in the left segment fill from the left, items in the right
// fill from the right, and an optional center segment is placed at
// the visual center. When width is tight the left segment truncates
// first so the right segment stays visible.
package statusbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Style controls an item's color treatment.
type Style int

const (
	StyleDefault Style = iota
	StylePrimary
	StyleSuccess
	StyleWarning
	StyleError
	StyleMuted
)

// Item is one chip in the bar.
type Item struct {
	Text  string
	Style Style
}

// Bar is a Bubble Tea-compatible status bar. It implements no Update;
// composition with a parent model is the expected pattern.
type Bar struct {
	theme  theme.Theme
	left   []Item
	center []Item
	right  []Item
	width  int
	sep    string
}

// New constructs an empty Bar with width 80 and " · " between items.
func New(t theme.Theme) Bar {
	return Bar{theme: t, width: 80, sep: " · "}
}

// WithWidth sets the total rendered width. Minimum 20.
func (b Bar) WithWidth(w int) Bar {
	if w < 20 {
		w = 20
	}
	b.width = w
	return b
}

// WithSeparator replaces the default " · " between items within a segment.
func (b Bar) WithSeparator(s string) Bar { b.sep = s; return b }

// WithLeft sets the left segment. Items appear left-to-right.
func (b Bar) WithLeft(items ...Item) Bar { b.left = items; return b }

// WithCenter sets the center segment.
func (b Bar) WithCenter(items ...Item) Bar { b.center = items; return b }

// WithRight sets the right segment.
func (b Bar) WithRight(items ...Item) Bar { b.right = items; return b }

// View renders the bar as a single line padded to width. The bar has
// no border; consumers can wrap it with lipgloss themselves.
func (b Bar) View() string {
	left := b.renderSegment(b.left)
	center := b.renderSegment(b.center)
	right := b.renderSegment(b.right)

	bg := lipgloss.NewStyle().Background(b.theme.Surface)

	leftW := lipgloss.Width(left)
	centerW := lipgloss.Width(center)
	rightW := lipgloss.Width(right)

	// If everything fits cleanly, place each segment.
	total := leftW + centerW + rightW
	if total <= b.width {
		// center placement: split remaining space around the center segment
		// then anchor right to the far edge.
		var line strings.Builder
		line.WriteString(left)
		if centerW > 0 {
			leftPad := (b.width-centerW)/2 - leftW
			if leftPad < 1 {
				leftPad = 1
			}
			line.WriteString(strings.Repeat(" ", leftPad))
			line.WriteString(center)
			rightStart := leftW + leftPad + centerW
			gap := b.width - rightStart - rightW
			if gap < 1 {
				gap = 1
			}
			line.WriteString(strings.Repeat(" ", gap))
		} else {
			gap := b.width - leftW - rightW
			if gap < 1 {
				gap = 1
			}
			line.WriteString(strings.Repeat(" ", gap))
		}
		line.WriteString(right)
		return bg.Render(line.String())
	}

	// Tight budget: drop center, truncate left.
	leftBudget := b.width - rightW - 1
	if leftBudget < 1 {
		// even right doesn't fit cleanly; truncate right too.
		right = truncate(right, b.width)
		return bg.Render(right)
	}
	left = truncate(left, leftBudget)
	leftW = lipgloss.Width(left)
	gap := b.width - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	return bg.Render(left + strings.Repeat(" ", gap) + right)
}

func (b Bar) renderSegment(items []Item) string {
	if len(items) == 0 {
		return ""
	}
	rendered := make([]string, 0, len(items))
	for _, it := range items {
		if it.Text == "" {
			continue
		}
		rendered = append(rendered, b.styleFor(it.Style).Render(it.Text))
	}
	if len(rendered) == 0 {
		return ""
	}
	sepStyle := lipgloss.NewStyle().Foreground(b.theme.Border)
	return strings.Join(rendered, sepStyle.Render(b.sep))
}

func (b Bar) styleFor(s Style) lipgloss.Style {
	base := lipgloss.NewStyle().Background(b.theme.Surface)
	switch s {
	case StylePrimary:
		return base.Foreground(b.theme.PrimaryStrong).Bold(true)
	case StyleSuccess:
		return base.Foreground(b.theme.Success).Bold(true)
	case StyleWarning:
		return base.Foreground(b.theme.Warning).Bold(true)
	case StyleError:
		return base.Foreground(b.theme.Error).Bold(true)
	case StyleMuted:
		return base.Foreground(b.theme.TextMuted)
	default:
		return base.Foreground(b.theme.Text)
	}
}

// truncate returns s shortened to width including a trailing ellipsis
// when truncation actually occurs. ANSI styling is preserved by
// lipgloss.Width but stripping mid-escape is unsafe; callers should
// only truncate already-rendered output of known plain content.
func truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	// best-effort: trim runes from the end. lipgloss escape sequences
	// will be preserved as long as we don't cut into them; since we
	// only style at the item level, the join sites are between styles,
	// not inside.
	runes := []rune(s)
	for lipgloss.Width(string(runes)) > width-1 && len(runes) > 0 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
