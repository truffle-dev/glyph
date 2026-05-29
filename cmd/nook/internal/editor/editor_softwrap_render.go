package editor

import (
	"github.com/truffle-dev/glyph/cmd/nook/internal/dochi"
	"github.com/truffle-dev/glyph/cmd/nook/internal/highlight"
	"github.com/truffle-dev/glyph/cmd/nook/internal/inlayhint"
)

// Per-sub-row slicing helpers for soft-wrap rendering. Each helper takes the
// full-row decoration channel plus the sub-row byte window [subStart, subEnd)
// and returns the subset, with coordinates shifted into the sub-row's own
// frame. Hints, cursors, and selection have boundary rules described per
// function.

// sliceSpansForSubrow returns the subset of spans that overlap [subStart, subEnd),
// with Start/End shifted into the sub-row's coordinate frame.
func sliceSpansForSubrow(spans []highlight.Span, subStart, subEnd int) []highlight.Span {
	if len(spans) == 0 || subStart >= subEnd {
		return nil
	}
	out := make([]highlight.Span, 0, len(spans))
	for _, s := range spans {
		if s.End <= subStart || s.Start >= subEnd {
			continue
		}
		ns := s.Start - subStart
		ne := s.End - subStart
		if ns < 0 {
			ns = 0
		}
		if ne > subEnd-subStart {
			ne = subEnd - subStart
		}
		out = append(out, highlight.Span{Start: ns, End: ne, Kind: s.Kind})
	}
	return out
}

// sliceMatchesForSubrow filters matches that overlap [subStart, subEnd) and
// remaps activeIdx to its new index in the filtered slice (-1 if the active
// match doesn't intersect this sub-row).
func sliceMatchesForSubrow(marks []Range, activeIdx, subStart, subEnd int) ([]Range, int) {
	if len(marks) == 0 || subStart >= subEnd {
		return nil, -1
	}
	out := make([]Range, 0, len(marks))
	newActive := -1
	for i, m := range marks {
		if m.End <= subStart || m.Start >= subEnd {
			continue
		}
		ns := m.Start - subStart
		ne := m.End - subStart
		if ns < 0 {
			ns = 0
		}
		if ne > subEnd-subStart {
			ne = subEnd - subStart
		}
		if i == activeIdx {
			newActive = len(out)
		}
		out = append(out, Range{Row: m.Row, Start: ns, End: ne})
	}
	return out, newActive
}

// sliceDochiForSubrow filters dochi spans that overlap [subStart, subEnd).
// Span.End == -1 means "extend through end of line"; sliced spans receive a
// concrete End == subEnd-subStart in that case.
func sliceDochiForSubrow(spans []dochi.Span, subStart, subEnd, lineLen int) []dochi.Span {
	if len(spans) == 0 || subStart >= subEnd {
		return nil
	}
	out := make([]dochi.Span, 0, len(spans))
	for _, s := range spans {
		end := s.End
		if end < 0 {
			end = lineLen + 1
		}
		if end <= subStart || s.Start >= subEnd {
			continue
		}
		ns := s.Start - subStart
		ne := end - subStart
		if ns < 0 {
			ns = 0
		}
		if ne > subEnd-subStart {
			ne = subEnd - subStart
		}
		out = append(out, dochi.Span{Start: ns, End: ne})
	}
	return out
}

// sliceHintsForSubrow returns hints whose Col falls in [subStart, subEnd).
// Col == lineLen (end-of-line hints) belongs to the last sub-row only.
// Col == subEnd on a non-last sub-row belongs to the NEXT sub-row.
func sliceHintsForSubrow(hints []inlayhint.Hint, subStart, subEnd, lineLen int, isLastSub bool) []inlayhint.Hint {
	if len(hints) == 0 {
		return nil
	}
	out := make([]inlayhint.Hint, 0, len(hints))
	for _, h := range hints {
		c := h.Col
		if c < 0 {
			c = 0
		}
		switch {
		case c < subStart:
			continue
		case c >= subEnd:
			if isLastSub && c <= lineLen {
				out = append(out, inlayhint.Hint{
					Row:          h.Row,
					Col:          c - subStart,
					Label:        h.Label,
					Kind:         h.Kind,
					PaddingLeft:  h.PaddingLeft,
					PaddingRight: h.PaddingRight,
				})
			}
			continue
		default:
			out = append(out, inlayhint.Hint{
				Row:          h.Row,
				Col:          c - subStart,
				Label:        h.Label,
				Kind:         h.Kind,
				PaddingLeft:  h.PaddingLeft,
				PaddingRight: h.PaddingRight,
			})
		}
	}
	return out
}

// sliceBracketsForSubrow returns the subset of bracket cols that fall in
// [subStart, subEnd), shifted into the sub-row's coordinate frame. Brackets
// are always single-byte runes at valid line positions, so there is no EOL
// case to handle.
func sliceBracketsForSubrow(cols []int, subStart, subEnd int) []int {
	if len(cols) == 0 || subStart >= subEnd {
		return nil
	}
	out := make([]int, 0, len(cols))
	for _, c := range cols {
		if c < subStart || c >= subEnd {
			continue
		}
		out = append(out, c-subStart)
	}
	return out
}

// sliceCursorsForSubrow returns cursor cols that fall in [subStart, subEnd).
// A cursor at col == lineLen (EOL) belongs to the last sub-row only.
func sliceCursorsForSubrow(cols []int, subStart, subEnd, lineLen int, isLastSub bool) []int {
	if len(cols) == 0 {
		return nil
	}
	out := make([]int, 0, len(cols))
	for _, c := range cols {
		if c < 0 {
			c = 0
		}
		switch {
		case c < subStart:
			continue
		case c >= subEnd:
			if isLastSub && c <= lineLen {
				out = append(out, c-subStart)
			}
			continue
		default:
			out = append(out, c-subStart)
		}
	}
	return out
}

// sliceSelectionForSubrow translates a row-level selection range into the
// sub-row's frame. Returns (-1, -1, false) when there is no overlap. selTail
// only propagates onto the LAST sub-row of the row.
func sliceSelectionForSubrow(selStart, selEnd int, selTail bool, subStart, subEnd, lineLen int, isLastSub bool) (int, int, bool) {
	if selStart < 0 || selEnd <= selStart {
		return -1, -1, false
	}
	if selEnd <= subStart || selStart >= subEnd {
		if selTail && isLastSub && selEnd >= subEnd && selStart < subEnd {
			ns := selStart - subStart
			if ns < 0 {
				ns = 0
			}
			return ns, subEnd - subStart, true
		}
		return -1, -1, false
	}
	ns := selStart - subStart
	ne := selEnd - subStart
	if ns < 0 {
		ns = 0
	}
	if ne > subEnd-subStart {
		ne = subEnd - subStart
	}
	tail := selTail && isLastSub
	return ns, ne, tail
}

// subRowPrimaryCol returns the cursor's relative col within the given sub-row,
// or -1 if the cursor falls outside [subStart, subEnd). EOL cursor (col ==
// lineLen) attaches to the last sub-row.
func subRowPrimaryCol(col, subStart, subEnd, lineLen int, isLastSub bool) int {
	if col < 0 {
		return -1
	}
	if col >= subStart && col < subEnd {
		return col - subStart
	}
	if isLastSub && col == lineLen {
		return col - subStart
	}
	return -1
}
