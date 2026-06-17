// Package complete renders nook's LSP completion popup — a small menu
// that lists labeled completion items, lets the user move through them
// with the arrow keys, and surfaces the chosen item on Enter.
//
// The popup is a pure value: the host owns when it appears, what items
// it carries, and where on the screen it lands. complete.Popup just
// knows how to navigate its own list and render a bordered menu at a
// requested width. The host's accept path also calls PrefixLen() so it
// can delete the partial word the user already typed before inserting
// the new InsertText.
package complete

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/components/theme"
)

// minWidth is the smallest width we'll try to render at. Below this the
// popup is too cramped to be useful so the host should skip rendering.
const minWidth = 24

// defaultMaxRows is the soft cap on visible rows when the host doesn't
// supply a tighter one. Eight matches the number of suggestions most
// users can take in at a glance without scrolling.
const defaultMaxRows = 8

// Popup is the immutable state of the completion menu. New() returns an
// empty popup; WithItems re-arms it with a fresh result set and resets
// the selection. The host treats Popup like any other tea-style value:
// derive a new one from each transition.
type Popup struct {
	items     []nooklsp.CompletionItem
	selected  int
	prefixLen int
}

// New returns an empty popup. View() on an empty popup returns "".
func New() Popup { return Popup{} }

// WithItems returns a popup carrying these items and the prefix length
// the host trimmed off the cursor row to filter them. Selection resets
// to the first item. prefixLen is informational for the host's accept
// path; the popup itself doesn't use it for rendering.
//
// Items are ordered by each one's LSP SortText before display. The LSP
// spec says a client must sort completion items by SortText, falling
// back to Label when an item omits it; gopls (and most servers) lean on
// this to surface the best match first. The wire order a server returns
// is not the display order it intends, so sorting here is what makes the
// popup match what Zed and VSCode show. The sort is stable, so items
// that tie on both keys keep their server order.
func (p Popup) WithItems(items []nooklsp.CompletionItem, prefixLen int) Popup {
	if prefixLen < 0 {
		prefixLen = 0
	}
	ordered := make([]nooklsp.CompletionItem, len(items))
	copy(ordered, items)
	sort.SliceStable(ordered, func(i, j int) bool {
		ki, kj := sortKey(ordered[i]), sortKey(ordered[j])
		if ki != kj {
			return ki < kj
		}
		return ordered[i].Label < ordered[j].Label
	})
	p.items = ordered
	p.selected = 0
	p.prefixLen = prefixLen
	return p
}

// sortKey is the primary ordering key for an item: its SortText when the
// server supplied one, otherwise its Label. This mirrors the LSP spec's
// fallback rule so a server that omits SortText still ranks sensibly.
func sortKey(it nooklsp.CompletionItem) string {
	if it.SortText != "" {
		return it.SortText
	}
	return it.Label
}

// Empty reports whether the popup has nothing to show.
func (p Popup) Empty() bool { return len(p.items) == 0 }

// Len returns the number of items in the popup.
func (p Popup) Len() int { return len(p.items) }

// PrefixLen returns the word-prefix length captured when the popup was
// armed. The host deletes this many runes before inserting the chosen
// item's InsertText, so the partial word the user typed is replaced
// cleanly.
func (p Popup) PrefixLen() int { return p.prefixLen }

// Selected returns the currently-highlighted item and true, or the zero
// value and false if the popup is empty.
func (p Popup) Selected() (nooklsp.CompletionItem, bool) {
	if len(p.items) == 0 {
		return nooklsp.CompletionItem{}, false
	}
	return p.items[p.selected], true
}

// MoveUp shifts the highlight up one row, wrapping to the bottom.
func (p Popup) MoveUp() Popup {
	if len(p.items) == 0 {
		return p
	}
	p.selected--
	if p.selected < 0 {
		p.selected = len(p.items) - 1
	}
	return p
}

// MoveDown shifts the highlight down one row, wrapping to the top.
func (p Popup) MoveDown() Popup {
	if len(p.items) == 0 {
		return p
	}
	p.selected++
	if p.selected >= len(p.items) {
		p.selected = 0
	}
	return p
}

// View renders the popup at the requested outer width and row cap.
// Returns "" for an empty popup so the host can render unconditionally.
// maxRows clamps the visible window; if the selected index falls below
// the window it scrolls down to keep it visible.
func (p Popup) View(t theme.Theme, width, maxRows int) string {
	if len(p.items) == 0 {
		return ""
	}
	if width < minWidth {
		width = minWidth
	}
	if maxRows < 1 {
		maxRows = defaultMaxRows
	}

	// Reserve 4 cells for border (1+1) and padding (1+1).
	inner := width - 4
	if inner < 14 {
		inner = 14
	}

	rowStyle := lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Surface).
		Width(inner)
	selStyle := lipgloss.NewStyle().
		Foreground(t.Bg).
		Background(t.Primary).
		Bold(true).
		Width(inner)
	mutedStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)
	mutedSelStyle := lipgloss.NewStyle().
		Foreground(t.Bg).
		Background(t.Primary)

	start, end := windowBounds(p.selected, len(p.items), maxRows)

	var rows []string
	for i := start; i < end; i++ {
		it := p.items[i]
		line := formatRow(it, inner, i == p.selected, mutedStyle, mutedSelStyle)
		if i == p.selected {
			rows = append(rows, selStyle.Render(line))
		} else {
			rows = append(rows, rowStyle.Render(line))
		}
	}

	body := strings.Join(rows, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Background(t.Surface).
		Padding(0, 1).
		Width(inner).
		Render(body)
}

// formatRow lays out one item as "<label>  <kind>  <detail>" clamped to
// the given inner width. Kind and detail are rendered with the muted
// style when the row is not selected, and inverted to keep contrast
// against the selection background when it is.
func formatRow(it nooklsp.CompletionItem, width int, selected bool, muted, mutedSel lipgloss.Style) string {
	label := it.Label
	if label == "" {
		label = it.InsertText
	}
	label = clip(label, width)

	kind := string(it.Kind)
	detail := it.Detail

	// label takes priority; kind+detail compete for the remainder.
	used := lipgloss.Width(label)
	remaining := width - used - 2
	if remaining < 4 {
		return label
	}

	var suffix string
	if kind != "" && remaining > 0 {
		k := clip(kind, remaining)
		if selected {
			suffix = mutedSel.Render(k)
		} else {
			suffix = muted.Render(k)
		}
		remaining -= lipgloss.Width(k) + 2
	}
	if detail != "" && remaining > 4 {
		d := clip(detail, remaining)
		var rendered string
		if selected {
			rendered = mutedSel.Render(d)
		} else {
			rendered = muted.Render(d)
		}
		if suffix != "" {
			suffix = suffix + "  " + rendered
		} else {
			suffix = rendered
		}
	}

	pad := width - used - lipgloss.Width(suffix)
	if pad < 1 {
		pad = 1
	}
	return fmt.Sprintf("%s%s%s", label, strings.Repeat(" ", pad), suffix)
}

// clip truncates s to width display cells, appending "…" if cut. width
// of 0 or less returns "".
func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	// Walk runes until adding the next would exceed width-1; reserve 1 for "…".
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > width-1 {
			break
		}
		b.WriteRune(r)
		used += w
	}
	b.WriteRune('…')
	return b.String()
}

// windowBounds returns [start, end) such that selected is visible and
// the window holds at most maxRows entries.
func windowBounds(selected, total, maxRows int) (int, int) {
	if total <= maxRows {
		return 0, total
	}
	start := selected - maxRows/2
	if start < 0 {
		start = 0
	}
	end := start + maxRows
	if end > total {
		end = total
		start = end - maxRows
	}
	return start, end
}

// WordPrefix extracts the trailing identifier prefix from s — the
// longest tail of runes that match an identifier (letter, digit, or
// underscore). Used by the host to compute how many chars to delete
// before inserting the popup's chosen InsertText.
func WordPrefix(s string) string {
	runes := []rune(s)
	i := len(runes)
	for i > 0 {
		r := runes[i-1]
		if !isIdentRune(r) {
			break
		}
		i--
	}
	return string(runes[i:])
}

func isIdentRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	case r == '_':
		return true
	}
	return false
}
