// Package codeaction renders nook's LSP code-action picker — a small
// bordered menu listing the actions the language server proposed at the
// cursor. The user picks one with the arrow keys and accepts with Enter;
// the host then applies the chosen action's workspace edit.
//
// The popup is a pure value: the host owns when it appears and what items
// it carries. codeaction.Popup knows only how to navigate its own list
// and render a bordered menu at a requested width.
package codeaction

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/components/theme"
)

// minWidth is the smallest outer width we'll try to render at.
const minWidth = 28

// defaultMaxRows is the soft cap on visible actions.
const defaultMaxRows = 8

// Popup is the immutable state of the code-action menu. New() returns an
// empty popup; WithItems re-arms it with a fresh result set and resets
// the selection.
type Popup struct {
	items    []nooklsp.CodeActionItem
	selected int
}

// New returns an empty popup. View() on an empty popup returns "".
func New() Popup { return Popup{} }

// WithItems returns a popup carrying these items, with the first non-disabled
// item selected by default. If every item is disabled, the first one is
// selected anyway so the popup has a stable cursor.
func (p Popup) WithItems(items []nooklsp.CodeActionItem) Popup {
	p.items = items
	p.selected = 0
	for i, it := range items {
		if it.Disabled == "" {
			p.selected = i
			break
		}
	}
	return p
}

// Empty reports whether the popup has nothing to show.
func (p Popup) Empty() bool { return len(p.items) == 0 }

// Len returns the number of items in the popup.
func (p Popup) Len() int { return len(p.items) }

// Selected returns the currently-highlighted action and true, or the zero
// value and false if the popup is empty or the highlighted action is
// disabled (the host should refuse to apply a disabled action).
func (p Popup) Selected() (nooklsp.CodeActionItem, bool) {
	if len(p.items) == 0 {
		return nooklsp.CodeActionItem{}, false
	}
	it := p.items[p.selected]
	if it.Disabled != "" {
		return it, false
	}
	return it, true
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

	inner := width - 4 // 1+1 border, 1+1 padding
	if inner < 18 {
		inner = 18
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
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	mutedSelStyle := lipgloss.NewStyle().Foreground(t.Bg).Background(t.Primary)
	disabledStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Surface).
		Faint(true).
		Width(inner)

	start, end := windowBounds(p.selected, len(p.items), maxRows)

	var rows []string
	for i := start; i < end; i++ {
		it := p.items[i]
		line := formatRow(it, inner, i == p.selected, mutedStyle, mutedSelStyle)
		switch {
		case it.Disabled != "":
			rows = append(rows, disabledStyle.Render(line))
		case i == p.selected:
			rows = append(rows, selStyle.Render(line))
		default:
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

// formatRow lays out one item as "<title>   <kind>", clamped to width.
// The kind tag (quickfix / refactor / source / ...) is rendered with the
// muted style when not selected. Items marked IsPreferred are prefixed
// with a "*" so the user can find an auto-fix quickly.
func formatRow(it nooklsp.CodeActionItem, width int, selected bool, muted, mutedSel lipgloss.Style) string {
	title := it.Title
	if it.IsPreferred {
		title = "* " + title
	}
	title = clip(title, width)

	kind := shortKind(it.Kind)

	used := lipgloss.Width(title)
	remaining := width - used - 2
	if kind == "" || remaining < 4 {
		pad := width - used
		if pad < 0 {
			pad = 0
		}
		return title + strings.Repeat(" ", pad)
	}
	kind = clip(kind, remaining)
	var rendered string
	if selected {
		rendered = mutedSel.Render(kind)
	} else {
		rendered = muted.Render(kind)
	}
	pad := width - used - lipgloss.Width(kind)
	if pad < 1 {
		pad = 1
	}
	return title + strings.Repeat(" ", pad) + rendered
}

// shortKind collapses an LSP CodeActionKind into a 1-2 word tag suitable
// for the right edge of the popup row.
func shortKind(kind string) string {
	switch {
	case kind == "":
		return ""
	case kind == "quickfix":
		return "fix"
	case kind == "source.organizeImports":
		return "imports"
	case kind == "source":
		return "source"
	case strings.HasPrefix(kind, "refactor.extract"):
		return "extract"
	case strings.HasPrefix(kind, "refactor.inline"):
		return "inline"
	case strings.HasPrefix(kind, "refactor.rewrite"):
		return "rewrite"
	case strings.HasPrefix(kind, "refactor"):
		return "refactor"
	}
	return kind
}

// clip truncates s to the given display-cell width, appending "…" when cut.
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

// windowBounds returns [start, end) such that selected is visible and the
// window holds at most maxRows entries.
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
