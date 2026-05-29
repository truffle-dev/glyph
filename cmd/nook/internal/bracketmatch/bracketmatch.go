// Package bracketmatch finds the matching bracket for a cursor position.
//
// It supports the three universal pairs (), [], and {}. The anchor selection
// prefers the rune at the cursor (the block-cursor "under here" position);
// failing that, the rune immediately before the cursor (the "just typed"
// fallback that catches the cursor sitting past the end of a line).
//
// String- and comment-aware exclusion is intentionally not done at this layer.
// Bracket counting inside string literals is wrong by definition, but the
// caller only paints the highlight when both endpoints exist, which keeps the
// damage to "no highlight" rather than "wrong highlight." Lifting chroma's
// token spans into this layer can wait until a dogfooded case shows it
// matters.
package bracketmatch

// Pair identifies one of the three supported bracket pairs.
type Pair int

const (
	Paren   Pair = iota // ( )
	Bracket             // [ ]
	Brace               // { }
)

// Pos is a 0-based byte position within a logical line.
type Pos struct {
	Row int
	Col int
}

// scanBudget caps how many bytes Match will inspect before giving up. A pane
// holding a megabyte of source between two unmatched braces is allowed to be
// uninformative rather than to stall first paint.
const scanBudget = 1 << 20

// Match looks for a bracket-pair anchor at the cursor and, when found,
// returns both endpoints plus the pair kind. Anchor selection: prefers the
// rune at (row, col) (the block cursor's "under" position); falls back to
// the rune at (row, col-1) when col is past the line end.
//
// Returns ok=false when there is no bracket anchor at either position, when
// row is out of range, or when the matching bracket cannot be found within
// the scan budget.
func Match(lines []string, row, col int) (anchor, match Pos, kind Pair, ok bool) {
	var zero Pos
	if row < 0 || row >= len(lines) {
		return zero, zero, 0, false
	}
	line := lines[row]
	for _, c := range [...]int{col, col - 1} {
		if c < 0 || c >= len(line) {
			continue
		}
		b := line[c]
		if !isBracket(b) {
			continue
		}
		if mr, mc, found := walk(lines, row, c, b); found {
			return Pos{Row: row, Col: c}, Pos{Row: mr, Col: mc}, pairOf(b), true
		}
	}
	return zero, zero, 0, false
}

func isBracket(b byte) bool {
	switch b {
	case '(', ')', '[', ']', '{', '}':
		return true
	}
	return false
}

func isOpener(b byte) bool {
	return b == '(' || b == '[' || b == '{'
}

func closerOf(b byte) byte {
	switch b {
	case '(':
		return ')'
	case '[':
		return ']'
	case '{':
		return '}'
	}
	return 0
}

func openerOf(b byte) byte {
	switch b {
	case ')':
		return '('
	case ']':
		return '['
	case '}':
		return '{'
	}
	return 0
}

func pairOf(b byte) Pair {
	switch b {
	case '(', ')':
		return Paren
	case '[', ']':
		return Bracket
	}
	return Brace
}

// walk searches forward (when anchor is an opener) or backward (when anchor
// is a closer) for the matching bracket, counting nested same-kind pairs.
// Mismatched-kind brackets are ignored; only the matching kind affects depth.
func walk(lines []string, row, col int, anchor byte) (int, int, bool) {
	visited := 0
	if isOpener(anchor) {
		closer := closerOf(anchor)
		depth := 1
		r := row
		c := col + 1
		for r < len(lines) {
			line := lines[r]
			for c < len(line) {
				b := line[c]
				if b == anchor {
					depth++
				} else if b == closer {
					depth--
					if depth == 0 {
						return r, c, true
					}
				}
				visited++
				if visited > scanBudget {
					return 0, 0, false
				}
				c++
			}
			r++
			c = 0
		}
		return 0, 0, false
	}
	opener := openerOf(anchor)
	depth := 1
	r := row
	c := col - 1
	for r >= 0 {
		for c >= 0 {
			b := lines[r][c]
			if b == anchor {
				depth++
			} else if b == opener {
				depth--
				if depth == 0 {
					return r, c, true
				}
			}
			visited++
			if visited > scanBudget {
				return 0, 0, false
			}
			c--
		}
		r--
		if r >= 0 {
			c = len(lines[r]) - 1
		}
	}
	return 0, 0, false
}
