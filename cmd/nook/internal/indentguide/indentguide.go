// Package indentguide computes the visual column positions where indent-
// guide glyphs paint in the editor. Pure functions, no theme or terminal
// concerns. The editor calls VisualGuideCols(raw, tabWidth) for each
// visible row and ActiveGuideCol(cursorVisualCol, tabWidth) for the cursor
// row to learn which guide column should highlight.
//
// Semantic choices:
//
//  1. Guides paint at columns tabWidth, 2*tabWidth, 3*tabWidth, ... within
//     a row's leading whitespace. Column 0 is NOT a guide site: the gutter
//     already provides a left-edge separator, and depth-1 lines stay
//     visually quiet. This matches Zed's restrained indent layer.
//
//  2. Guides only paint where the row's leading whitespace actually
//     extends. A row of "    foo" (4 spaces, tabWidth=4) has zero guides
//     because the candidate column 4 sits on the 'f', not on whitespace.
//     A row of "        foo" (8 spaces) has one guide at column 4.
//
//  3. Active-guide highlighting marks the indent zone the cursor sits in.
//     The active column is `floor(visualCol / tabWidth) * tabWidth` when
//     visualCol >= tabWidth, else -1. The editor uses this to brighten
//     one guide on the cursor row; vertical-column extension across rows
//     is a future polish layer.
package indentguide

// VisualGuideCols returns the visual column indexes (0-based, in the
// tab-expanded display) where indent-guide glyphs should paint for a row.
//
// raw is the source bytes; tabWidth is the column count a hard tab expands
// to during rendering. The function walks the leading whitespace (a prefix
// of ' ' and '\t' bytes), computes its expanded visual width, then returns
// every multiple of tabWidth strictly less than that width — skipping 0
// because the gutter already separates content from the left margin.
//
// Returns an empty slice when the row has no leading whitespace, when the
// leading whitespace doesn't reach the first guide stop, or when tabWidth
// is non-positive.
func VisualGuideCols(raw string, tabWidth int) []int {
	if tabWidth <= 0 {
		return nil
	}
	width := LeadingWhitespaceVisualWidth(raw, tabWidth)
	if width <= tabWidth {
		return nil
	}
	var cols []int
	for c := tabWidth; c < width; c += tabWidth {
		cols = append(cols, c)
	}
	return cols
}

// LeadingWhitespaceVisualWidth returns the number of display columns the
// leading whitespace of raw occupies after tab expansion. A space counts as
// one column; a tab counts as tabWidth columns. The walk stops at the first
// byte that is neither ' ' nor '\t'. Returns 0 when tabWidth is
// non-positive (no expansion is defined).
//
// A row that is entirely whitespace returns the full expanded width — that
// row is itself an indent context, so its guides paint as if it were
// indented to the same depth as its non-whitespace content would be.
func LeadingWhitespaceVisualWidth(raw string, tabWidth int) int {
	if tabWidth <= 0 {
		return 0
	}
	w := 0
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case ' ':
			w++
		case '\t':
			w += tabWidth
		default:
			return w
		}
	}
	return w
}

// ActiveGuideCol returns the visual column of the indent guide that the
// cursor at visualCol currently sits within, or -1 when no guide should
// highlight.
//
// The rule: `floor(visualCol / tabWidth) * tabWidth` when visualCol >=
// tabWidth, else -1. Cursor at column 0 through tabWidth-1 sits in the
// outermost (file-margin) zone and has no parent guide to highlight; once
// the cursor reaches column tabWidth it sits on or past the first guide
// site, and the floor formula picks the deepest stop at-or-before the
// cursor.
//
// Examples (tabWidth=4): col 0 → -1, col 3 → -1, col 4 → 4, col 7 → 4,
// col 8 → 8, col 11 → 8, col 12 → 12.
func ActiveGuideCol(visualCol, tabWidth int) int {
	if tabWidth <= 0 || visualCol < tabWidth {
		return -1
	}
	return (visualCol / tabWidth) * tabWidth
}
