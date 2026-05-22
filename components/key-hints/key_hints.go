// Package keyhints renders a compact footer of "<key> <description>"
// pairs separated by a thin divider. It's the bottom-row cheatsheet
// almost every TUI grows into; building it as a primitive saves the
// same render code from being rewritten in every parent model.
//
// Hints are pure render: there is no Update, no message. Build the
// hint set from your model and let key-hints lay it out.
package keyhints

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Default separator placed between adjacent hints.
const DefaultSeparator = "  "

// Hint is one key/description pair.
type Hint struct {
	Key  string
	Desc string
}

// Bar is a row of key hints.
type Bar struct {
	theme     theme.Theme
	hints     []Hint
	width     int
	separator string
}

// New constructs an empty hint bar.
func New(t theme.Theme) Bar {
	return Bar{theme: t, separator: DefaultSeparator}
}

// WithHints replaces the hint set.
func (b Bar) WithHints(hints []Hint) Bar { b.hints = hints; return b }

// WithWidth clamps the render width. The bar truncates from the right
// when the natural width exceeds the clamp. Values <= 0 mean natural.
func (b Bar) WithWidth(w int) Bar {
	if w < 0 {
		w = 0
	}
	b.width = w
	return b
}

// WithSeparator overrides the inter-hint separator.
func (b Bar) WithSeparator(s string) Bar { b.separator = s; return b }

// View renders the bar.
func (b Bar) View() string {
	if len(b.hints) == 0 {
		return ""
	}
	keyStyle := lipgloss.NewStyle().Foreground(b.theme.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(b.theme.TextMuted)

	pieces := make([]string, 0, len(b.hints))
	for _, h := range b.hints {
		piece := keyStyle.Render(h.Key) + " " + descStyle.Render(h.Desc)
		pieces = append(pieces, piece)
	}
	out := strings.Join(pieces, b.separator)
	if b.width > 0 {
		out = lipgloss.NewStyle().MaxWidth(b.width).Render(out)
	}
	return out
}
