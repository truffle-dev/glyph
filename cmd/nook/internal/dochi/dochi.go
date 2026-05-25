// Package dochi turns LSP textDocument/documentHighlight responses into a
// shape the editor can paint without re-importing the protocol package.
//
// Document highlights are decorative bands the editor draws over every
// occurrence of the identifier under the cursor (variables, functions,
// fields). They're a comprehension aid, not a feature toggle — when the
// language server resolves nothing, the editor falls back to plain
// rendering with zero status noise.
package dochi

// Kind classifies a highlight as a plain text occurrence, a read, or a
// write. Servers that don't distinguish use Text (the LSP default). The
// editor currently paints all three identically because the visual band
// reads better when it stays uniform; the field is preserved on the way
// in so a future tick can introduce read/write differentiation without
// re-plumbing.
type Kind int

// The three Kind values mirror LSP's DocumentHighlightKind enum.
const (
	KindText  Kind = 1
	KindRead  Kind = 2
	KindWrite Kind = 3
)

// Highlight is one occurrence span. StartLine / EndLine are 0-indexed
// inclusive line numbers; StartCol is the inclusive start column on
// StartLine; EndCol is the exclusive end column on EndLine. A single-
// line highlight (the common case) has StartLine == EndLine.
type Highlight struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	Kind      Kind
}

// LineSpans flattens a slice of multi-line highlights into per-line
// (startCol, endCol) ranges so the editor's row-render loop can ask the
// single question "what columns on row N should I band?" without
// re-walking every highlight. Multi-line highlights are split at line
// boundaries; the runtime in the editor is dominated by the per-rune
// emit loop, so a pre-flattened map keyed on row beats a linear scan.
//
// The runtime guard isn't perfectly defensive: malformed spans (Start
// > End on the same line, negative columns) are silently dropped.
// They're decorative — a wrongly-shaped highlight can't corrupt the
// buffer, so dropping is safe.
func LineSpans(hi []Highlight) map[int][]Span {
	out := make(map[int][]Span, len(hi))
	for _, h := range hi {
		if h.StartLine < 0 || h.EndLine < h.StartLine {
			continue
		}
		if h.StartLine == h.EndLine {
			if h.EndCol <= h.StartCol || h.StartCol < 0 {
				continue
			}
			out[h.StartLine] = append(out[h.StartLine], Span{Start: h.StartCol, End: h.EndCol})
			continue
		}
		// Multi-line: paint StartLine from StartCol to end-of-line (signaled
		// by End=-1 — the editor extends to the row length), then any middle
		// rows fully, then EndLine from 0 to EndCol.
		out[h.StartLine] = append(out[h.StartLine], Span{Start: h.StartCol, End: -1})
		for r := h.StartLine + 1; r < h.EndLine; r++ {
			out[r] = append(out[r], Span{Start: 0, End: -1})
		}
		if h.EndCol > 0 {
			out[h.EndLine] = append(out[h.EndLine], Span{Start: 0, End: h.EndCol})
		}
	}
	return out
}

// Span is a one-row contribution from LineSpans. Start is inclusive,
// End is exclusive; End == -1 means "extend through the rest of the
// row" (used for the inner rows of a multi-line highlight where the
// editor's row length is the only authority).
type Span struct {
	Start int
	End   int
}

// Covers reports whether the given byte column on a row sits inside
// any of the spans. -1 in End is treated as +infinity. The editor
// calls this once per rune per row, so the slice walk stays linear
// (highlights for a single row are at most a few dozen entries even
// on very dense files).
func Covers(spans []Span, col, rowLen int) bool {
	for _, s := range spans {
		end := s.End
		if end < 0 {
			end = rowLen + 1
		}
		if col >= s.Start && col < end {
			return true
		}
	}
	return false
}
