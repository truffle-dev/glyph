// Package softwrap computes how a single editor line breaks across visual
// rows when the pane's text width is bounded. Pure functions, no editor
// state. The editor calls WrapPoints(line, width, tabWidth) per logical
// line to learn where each visual row begins, and LineRowCount as a
// convenience when only the count matters (scroll math, gutter sizing).
//
// Semantics:
//
//  1. WrapPoints returns the byte indices into line where each visual row
//     begins. Index 0 is always present.
//
//  2. width is the maximum visual columns a row may occupy. A tab advances
//     to the next tabWidth-multiple within the current row; non-tab runes
//     count as one column each. East-asian-wide handling is deferred to a
//     future revision; treating CJK as single-column is the same fallback
//     used by the rest of the editor today.
//
//  3. When a rune would push the row past width, the function prefers to
//     break at the last whitespace seen in the current row, consuming the
//     whitespace into the prior row. When no whitespace is reachable it
//     hard-breaks immediately before the overflowing rune; never mid-rune.
//     A single rune wider than width at the row start is placed anyway,
//     with the wrap occurring after it.
//
//  4. width <= 0 disables wrapping. Empty input returns the single-row
//     answer []int{0}.
package softwrap

import "unicode/utf8"

// WrapPoints returns the byte indices where each visual row of line begins
// when soft-wrapped to width columns with tabWidth-column hard tabs.
func WrapPoints(line []byte, width, tabWidth int) []int {
	if width <= 0 || len(line) == 0 {
		return []int{0}
	}
	if tabWidth <= 0 {
		tabWidth = 1
	}

	result := []int{0}
	rowStart := 0
	rowCol := 0
	lastSpaceEnd := -1

	i := 0
	for i < len(line) {
		r, sz := utf8.DecodeRune(line[i:])
		var advance int
		if r == '\t' {
			advance = tabWidth - (rowCol % tabWidth)
		} else {
			advance = 1
		}

		if rowCol+advance > width {
			var breakAt int
			switch {
			case lastSpaceEnd > rowStart:
				breakAt = lastSpaceEnd
			case i > rowStart:
				breakAt = i
			case i+sz < len(line):
				breakAt = i + sz
			default:
				rowCol += advance
				i += sz
				continue
			}
			result = append(result, breakAt)
			rowStart = breakAt
			rowCol = 0
			lastSpaceEnd = -1
			i = breakAt
			continue
		}

		rowCol += advance
		i += sz
		if r == ' ' || r == '\t' {
			lastSpaceEnd = i
		}
	}

	return result
}

// LineRowCount returns the number of visual rows line occupies when wrapped
// at width columns. Always >= 1.
func LineRowCount(line []byte, width, tabWidth int) int {
	return len(WrapPoints(line, width, tabWidth))
}
