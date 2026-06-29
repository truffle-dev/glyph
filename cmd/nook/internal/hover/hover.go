// Package hover renders nook's LSP hover overlay — a small rounded-border
// box that displays whatever the language server returned for the symbol
// under the cursor.
//
// The box owns its own border + padding so the host just decides where
// to drop it (above the status bar in nook's case). View("") returns an
// empty string so callers can render unconditionally without a visible/
// invisible flag.
//
// Hover contents from gopls are markdown. The first slice renders them
// verbatim with hard-wrapping; richer markdown rendering (headings,
// code-fence syntax-highlighting) is deferred until the rest of the
// overlay rhythm settles.
package hover

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/truffle-dev/glyph/components/theme"
)

// minWidth is the smallest width we'll try to render at. Below this the
// overlay is too cramped to be useful so callers should skip rendering
// entirely.
const minWidth = 24

// View renders the hover overlay with the given contents at the
// requested outer width. Empty contents return "" so the host can
// render unconditionally and let the empty string collapse out of the
// layout. Contents above maxLines are truncated with a "…" marker so
// large doc blocks don't drown the editor.
func View(t theme.Theme, contents string, width, maxLines int) string {
	if strings.TrimSpace(contents) == "" {
		return ""
	}
	if width < minWidth {
		width = minWidth
	}
	if maxLines < 1 {
		maxLines = 1
	}
	// Reserve 4 cells for border (1+1) and padding (1+1).
	inner := width - 4
	if inner < 10 {
		inner = 10
	}

	body := wrapAndClamp(contents, inner, maxLines)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Foreground(t.Text).
		Background(t.Surface).
		Padding(0, 1).
		Width(inner).
		Render(body)
}

// wrapAndClamp hard-wraps the input at width display columns then keeps
// the first maxLines rows, replacing the final row with "…" if the
// original content was longer. Wrapping is grapheme- and wide-character-
// aware: gopls hover markdown routinely contains multi-byte runes (arrows,
// curly quotes, em dashes), so byte-slicing at the wrap point would both
// split a rune mid-sequence (emitting invalid UTF-8) and mismeasure the
// line width. ansi.Hardwrap counts display cells and never breaks a
// grapheme cluster.
func wrapAndClamp(s string, width, maxLines int) string {
	wrapped := ansi.Hardwrap(s, width, true)
	rows := strings.Split(wrapped, "\n")
	if len(rows) <= maxLines {
		return wrapped
	}
	rows = rows[:maxLines]
	rows[maxLines-1] = "…"
	return strings.Join(rows, "\n")
}
