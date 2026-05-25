// Package completedoc renders nook's LSP completion documentation
// side-panel — a small bordered box that sits beside the completion
// popup and shows the resolved documentation for whichever item is
// highlighted. As the user moves through the popup the host fires
// completionItem/resolve and pumps the resolved item back in via
// Open(), so the docs always match the highlighted entry.
//
// The pane is a pure value type — every mutator returns an updated
// copy — mirroring outline / signature / lookup so the host can pump
// it as `m.docPane = m.docPane.Open(item)` without aliasing surprises.
package completedoc

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/components/theme"
)

const (
	// defaultWidth is the column count we paint at when the caller passes
	// zero. Wide enough for a sentence-ish line of prose; the popup runs
	// to its left so total floor for the screen is popup+gap+doc.
	defaultWidth = 48
	// minWidth is the smallest width worth painting at. Below this the
	// pane stops being readable and the host should skip rendering.
	minWidth = 28
	// defaultHeight caps the visible row count when the caller passes
	// zero. Eight matches the completion popup's default row count so
	// the two boxes feel like one widget.
	defaultHeight = 12
	// minHeight floors the row count so the pane always shows at least
	// the label header plus a couple of doc lines.
	minHeight = 5
)

// Pane carries the open/closed state and the most recent resolved
// completion item from the server.
type Pane struct {
	open   bool
	width  int
	height int
	item   lsp.CompletionItem
}

// New returns a closed pane with default dimensions.
func New() Pane {
	return Pane{width: defaultWidth, height: defaultHeight}
}

// WithSize returns a copy of the pane sized to width/height (clamped to
// minWidth/minHeight). Either value may be zero to keep the default.
func (p Pane) WithSize(width, height int) Pane {
	if width != 0 {
		if width < minWidth {
			width = minWidth
		}
		p.width = width
	}
	if height != 0 {
		if height < minHeight {
			height = minHeight
		}
		p.height = height
	}
	return p
}

// Width returns the pane's render width.
func (p Pane) Width() int { return p.width }

// Height returns the pane's render height.
func (p Pane) Height() int { return p.height }

// Open returns a copy of the pane displaying the given item. An item
// with no Documentation AND no Detail returns a closed pane so the
// host doesn't paint an empty box for items the server can't enrich.
func (p Pane) Open(item lsp.CompletionItem) Pane {
	if item.Documentation == "" && item.Detail == "" {
		p.open = false
		p.item = lsp.CompletionItem{}
		return p
	}
	p.open = true
	p.item = item
	return p
}

// Close returns a copy of the pane in the closed state.
func (p Pane) Close() Pane {
	p.open = false
	p.item = lsp.CompletionItem{}
	return p
}

// IsOpen reports whether the pane should currently be rendered.
func (p Pane) IsOpen() bool { return p.open }

// Item returns the item currently displayed (zero value when closed).
func (p Pane) Item() lsp.CompletionItem { return p.item }

// ItemLabel is the staleness key the host uses to drop late resolve
// responses for items the user has already scrolled past.
func (p Pane) ItemLabel() string { return p.item.Label }

// View renders the pane as a bordered side-panel. Returns "" when
// closed or when the width is below the minimum (the host should
// skip rendering in that case, but a defensive empty return is
// cheaper than a caller guard everywhere).
func (p Pane) View(t theme.Theme) string {
	if !p.open || p.width < minWidth {
		return ""
	}

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Background(t.Surface).
		Padding(0, 1).
		Width(p.width - 2) // -2 for border columns

	innerWidth := p.width - 4 // -2 border, -2 padding
	if innerWidth < 4 {
		innerWidth = 4
	}

	var lines []string

	// Header row: <kind-tag> <label> — gives the user instant context
	// for which item the docs belong to.
	header := p.item.Label
	kind := shortKindTag(p.item.Kind)
	if kind != "" {
		tag := lipgloss.NewStyle().Foreground(t.TextMuted).Render(kind)
		header = tag + " " + header
	}
	lines = append(lines, clampLine(header, innerWidth))

	// Detail line (short type / signature). Render in muted unless it
	// duplicates the label.
	if p.item.Detail != "" && p.item.Detail != p.item.Label {
		detail := lipgloss.NewStyle().Foreground(t.TextMuted).Render(
			clampLine(p.item.Detail, innerWidth),
		)
		lines = append(lines, detail)
	}

	// Documentation block. Wrap then clamp to fit within the height
	// budget; the last visible row gets " …" when content was cut.
	if p.item.Documentation != "" {
		// Two lines of header (label + detail at most) plus one
		// implicit padding row leave height-3 doc rows available.
		docBudget := p.height - 2 - 2 // -2 border rows, -2 header rows worst case
		if docBudget < 2 {
			docBudget = 2
		}
		// Blank row between header/detail and docs for visual rhythm.
		lines = append(lines, "")
		doc := wrapAndClamp(p.item.Documentation, innerWidth, docBudget)
		if doc != "" {
			lines = append(lines, doc)
		}
	}

	body := strings.Join(lines, "\n")
	return border.Render(body)
}

// shortKindTag returns the two-letter tag for a completion kind,
// matching the outline pane's tag scheme so the IDE reads consistently.
func shortKindTag(k lsp.CompletionKind) string {
	switch k {
	case lsp.CompletionKindMethod:
		return "mt"
	case lsp.CompletionKindFunction:
		return "fn"
	case lsp.CompletionKindConstructor:
		return "ct"
	case lsp.CompletionKindField:
		return "fl"
	case lsp.CompletionKindVariable:
		return "vr"
	case lsp.CompletionKindClass:
		return "cl"
	case lsp.CompletionKindInterface:
		return "if"
	case lsp.CompletionKindModule:
		return "md"
	case lsp.CompletionKindProperty:
		return "pr"
	case lsp.CompletionKindUnit:
		return "un"
	case lsp.CompletionKindValue:
		return "vl"
	case lsp.CompletionKindEnum:
		return "en"
	case lsp.CompletionKindKeyword:
		return "kw"
	case lsp.CompletionKindSnippet:
		return "sn"
	case lsp.CompletionKindColor:
		return "co"
	case lsp.CompletionKindFile:
		return "fi"
	case lsp.CompletionKindReference:
		return "rf"
	case lsp.CompletionKindFolder:
		return "fd"
	case lsp.CompletionKindEnumMember:
		return "em"
	case lsp.CompletionKindConstant:
		return "cn"
	case lsp.CompletionKindStruct:
		return "st"
	case lsp.CompletionKindEvent:
		return "ev"
	case lsp.CompletionKindOperator:
		return "op"
	case lsp.CompletionKindTypeParameter:
		return "tp"
	}
	return ""
}

// clampLine truncates a single-line string to fit within width display
// cells, appending an ellipsis when cut. Multi-byte runes are handled
// via lipgloss.Width.
func clampLine(s string, width int) string {
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
	b.WriteString("…")
	return b.String()
}

// wrapAndClamp wraps long input lines at width, then keeps the first
// maxRows lines, replacing the final visible row with " …" when content
// was longer. Existing newlines are honored.
func wrapAndClamp(s string, width, maxRows int) string {
	if maxRows <= 0 || width <= 0 {
		return ""
	}
	var rows []string
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			rows = append(rows, "")
			continue
		}
		rows = append(rows, wrapLine(line, width)...)
	}
	if len(rows) <= maxRows {
		return strings.Join(rows, "\n")
	}
	kept := rows[:maxRows]
	// Replace the last row with an ellipsis row to signal truncation.
	kept[len(kept)-1] = " …"
	return strings.Join(kept, "\n")
}

// wrapLine word-wraps a single input line at width display cells. Words
// longer than width are hard-broken so we never overflow the pane.
func wrapLine(line string, width int) []string {
	if width <= 0 {
		return nil
	}
	var out []string
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}
	var cur strings.Builder
	curW := 0
	for _, w := range words {
		wcw := lipgloss.Width(w)
		if wcw > width {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
				curW = 0
			}
			// Hard-break the long word.
			out = append(out, hardBreak(w, width)...)
			continue
		}
		add := wcw
		if cur.Len() > 0 {
			add++ // space
		}
		if curW+add > width {
			out = append(out, cur.String())
			cur.Reset()
			cur.WriteString(w)
			curW = wcw
			continue
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
			curW++
		}
		cur.WriteString(w)
		curW += wcw
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// hardBreak splits an over-long token into width-sized chunks.
func hardBreak(s string, width int) []string {
	var out []string
	var cur strings.Builder
	curW := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if curW+w > width {
			out = append(out, cur.String())
			cur.Reset()
			curW = 0
		}
		cur.WriteRune(r)
		curW += w
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
