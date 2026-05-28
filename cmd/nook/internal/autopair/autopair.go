// Package autopair implements bracket and quote auto-pairing for the
// nook editor. It is pure: no editor state, no I/O. The editor pane
// asks it four questions and wires the answers into its rune-insert
// and backspace paths.
//
//   - OpenerFor(r) reports whether r is an opener and what its closer
//     is. Six pairs ship by default: () [] {} "" ” “.
//
//   - ShouldPair(line, col, r) decides whether typing the opener r at
//     byte position col of line should also insert the matching closer.
//     Suppressed when the next char is a word rune (so `foo(` plus `(`
//     doesn't break `foo` apart) and, for quote openers, when the prev
//     char is a word rune (so `it's` stays single-quoted).
//
//   - ShouldSkip(line, col, r) decides whether typing the closer r at
//     col should skip over the existing matching closer at line[col]
//     instead of inserting a duplicate. This is what makes typing `()`
//     feel natural: the ( inserts both halves, the ) skips over the
//     parked closer.
//
//   - IsEmptyPair(line, col) reports whether the cursor sits between
//     an opener at col-1 and its matching closer at col. The editor's
//     backspace path consults this to delete both halves of an empty
//     auto-paired pair.
//
// Slice 1 of v0.40.0 is just this package and its tests. Slice 2 wires
// it into editor.Pane via the rune-insert and backspace hot paths.
// Slice 3 ships v0.40.0.
package autopair

import (
	"unicode"
	"unicode/utf8"
)

// Pair is a single auto-pair entry. Symmetric pairs (quotes) have
// Open == Close.
type Pair struct {
	Open  rune
	Close rune
}

// Pairs is the default list of auto-pairs. Order is significant only
// for documentation; OpenerFor / IsCloser use map lookups.
var Pairs = []Pair{
	{'(', ')'},
	{'[', ']'},
	{'{', '}'},
	{'"', '"'},
	{'\'', '\''},
	{'`', '`'},
}

var (
	openerToCloser map[rune]rune
	closers        map[rune]struct{}
	symmetric      map[rune]struct{}
)

func init() {
	openerToCloser = make(map[rune]rune, len(Pairs))
	closers = make(map[rune]struct{}, len(Pairs))
	symmetric = make(map[rune]struct{}, len(Pairs))
	for _, p := range Pairs {
		openerToCloser[p.Open] = p.Close
		closers[p.Close] = struct{}{}
		if p.Open == p.Close {
			symmetric[p.Open] = struct{}{}
		}
	}
}

// OpenerFor returns the matching closer rune when r is a known opener.
// Returns (0, false) when r is not an opener.
func OpenerFor(r rune) (rune, bool) {
	c, ok := openerToCloser[r]
	return c, ok
}

// IsOpener reports whether r is one of the known openers.
func IsOpener(r rune) bool {
	_, ok := openerToCloser[r]
	return ok
}

// IsCloser reports whether r is one of the known closers.
func IsCloser(r rune) bool {
	_, ok := closers[r]
	return ok
}

// IsSymmetric reports whether r is an opener whose Close == Open
// (a quote-style pair).
func IsSymmetric(r rune) bool {
	_, ok := symmetric[r]
	return ok
}

// ShouldPair decides whether typing the opener r at byte position col
// of line should also insert the matching closer.
//
// The rules:
//
//  1. r must be a known opener.
//  2. The next char (the rune decoded at line[col]) must not be a word
//     rune. A word rune means we're inside an identifier; auto-pairing
//     would break it apart.
//  3. For symmetric openers (", ', `), the previous char must not be a
//     word rune. This prevents typing ' after `it` (as in `it's`) from
//     spawning an unwanted closing quote.
//  4. For symmetric openers, if the next char is the same rune as r,
//     the user is "closing" an already-paired quote — ShouldSkip will
//     handle that.
//
// End-of-line, whitespace, and ordinary punctuation are all valid
// next-chars. Only word runes suppress pairing.
func ShouldPair(line string, col int, r rune) bool {
	if !IsOpener(r) {
		return false
	}
	if col < 0 || col > len(line) {
		return false
	}
	next, _ := nextRune(line, col)
	if isWord(next) {
		return false
	}
	if IsSymmetric(r) {
		if next == r {
			return false
		}
		prev, _ := prevRune(line, col)
		if isWord(prev) {
			return false
		}
	}
	return true
}

// ShouldSkip decides whether typing the closer r at byte position col
// should skip over an existing matching closer at line[col] rather
// than insert a duplicate.
//
// Returns true when r is a known closer and the next char equals r.
func ShouldSkip(line string, col int, r rune) bool {
	if !IsCloser(r) {
		return false
	}
	if col < 0 || col >= len(line) {
		return false
	}
	next, _ := nextRune(line, col)
	return next == r
}

// IsEmptyPair reports whether the cursor at col sits between an
// opener at col-1 and its matching closer at col.
//
// Returns false at line boundaries or when the surrounding pair
// doesn't match a known opener/closer pairing.
func IsEmptyPair(line string, col int) bool {
	if col <= 0 || col >= len(line) {
		return false
	}
	prev, _ := prevRune(line, col)
	next, _ := nextRune(line, col)
	want, ok := OpenerFor(prev)
	if !ok {
		return false
	}
	return next == want
}

// nextRune decodes the rune at line[col] and reports its byte width.
// At end-of-string returns (0, 0).
func nextRune(line string, col int) (rune, int) {
	if col >= len(line) {
		return 0, 0
	}
	return utf8.DecodeRuneInString(line[col:])
}

// prevRune decodes the rune ending at line[col] and reports its byte
// width. At start-of-string returns (0, 0).
func prevRune(line string, col int) (rune, int) {
	if col <= 0 {
		return 0, 0
	}
	return utf8.DecodeLastRuneInString(line[:col])
}

// isWord reports whether r is a word rune for auto-pair suppression
// purposes. Letters, digits, and underscore are word runes; everything
// else (including 0/EOL) is not.
func isWord(r rune) bool {
	if r == 0 {
		return false
	}
	if r == '_' {
		return true
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
