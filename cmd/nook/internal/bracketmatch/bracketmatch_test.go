package bracketmatch

import (
	"bytes"
	"strings"
	"testing"
)

func lines(s string) [][]byte {
	parts := strings.Split(s, "\n")
	out := make([][]byte, len(parts))
	for i, p := range parts {
		out[i] = []byte(p)
	}
	return out
}

func TestMatchParenSameLine(t *testing.T) {
	ls := lines("foo(bar)baz")
	anchor, match, kind, ok := Match(ls, 0, 3)
	if !ok {
		t.Fatal("expected match for cursor on `(`")
	}
	if anchor != (Pos{Row: 0, Col: 3}) {
		t.Errorf("anchor = %+v, want {0, 3}", anchor)
	}
	if match != (Pos{Row: 0, Col: 7}) {
		t.Errorf("match = %+v, want {0, 7}", match)
	}
	if kind != Paren {
		t.Errorf("kind = %v, want Paren", kind)
	}
}

func TestMatchCloserPreferred(t *testing.T) {
	ls := lines("(x)")
	// Cursor at col=3 — past the `)`. col-1 = `)` is anchor.
	anchor, match, _, ok := Match(ls, 0, 3)
	if !ok {
		t.Fatal("expected match for cursor just past `)`")
	}
	if anchor != (Pos{Row: 0, Col: 2}) {
		t.Errorf("anchor = %+v, want {0, 2}", anchor)
	}
	if match != (Pos{Row: 0, Col: 0}) {
		t.Errorf("match = %+v, want {0, 0}", match)
	}
}

func TestMatchAtCursorWinsOverBeforeCursor(t *testing.T) {
	// At col=1 the rune-at is `)` and the rune-before is `(`. Both qualify,
	// so the rune-at (the block cursor's "under" position) should win and
	// the answer should anchor on `)`.
	ls := lines("()")
	anchor, match, _, ok := Match(ls, 0, 1)
	if !ok {
		t.Fatal("expected match")
	}
	if anchor != (Pos{Row: 0, Col: 1}) {
		t.Errorf("anchor = %+v, want {0, 1} (at-cursor)", anchor)
	}
	if match != (Pos{Row: 0, Col: 0}) {
		t.Errorf("match = %+v, want {0, 0}", match)
	}
}

func TestMatchBracketAndBrace(t *testing.T) {
	cases := []struct {
		body string
		kind Pair
	}{
		{"[abc]", Bracket},
		{"{abc}", Brace},
	}
	for _, c := range cases {
		ls := lines(c.body)
		_, _, kind, ok := Match(ls, 0, 0)
		if !ok {
			t.Errorf("%q: expected match", c.body)
			continue
		}
		if kind != c.kind {
			t.Errorf("%q: kind = %v, want %v", c.body, kind, c.kind)
		}
	}
}

func TestMatchNested(t *testing.T) {
	// `((x))` — cursor on outer `(` at 0 matches outer `)` at 4.
	ls := lines("((x))")
	_, match, _, ok := Match(ls, 0, 0)
	if !ok {
		t.Fatal("outer (: expected match")
	}
	if match != (Pos{Row: 0, Col: 4}) {
		t.Errorf("outer match = %+v, want {0, 4}", match)
	}
	// Inner `(` at 1 matches inner `)` at 3.
	_, match, _, ok = Match(ls, 0, 1)
	if !ok {
		t.Fatal("inner (: expected match")
	}
	if match != (Pos{Row: 0, Col: 3}) {
		t.Errorf("inner match = %+v, want {0, 3}", match)
	}
}

func TestMatchAcrossLines(t *testing.T) {
	body := "(\n\tfoo\n\tbar\n)"
	ls := lines(body)
	_, match, _, ok := Match(ls, 0, 0)
	if !ok {
		t.Fatal("expected multi-line match")
	}
	if match != (Pos{Row: 3, Col: 0}) {
		t.Errorf("match = %+v, want {3, 0}", match)
	}
	// Now anchor on the closer; it should walk back across the rows.
	_, match, _, ok = Match(ls, 3, 0)
	if !ok {
		t.Fatal("closer: expected reverse match")
	}
	if match != (Pos{Row: 0, Col: 0}) {
		t.Errorf("reverse match = %+v, want {0, 0}", match)
	}
}

func TestMatchUnmatchedReturnsFalse(t *testing.T) {
	ls := lines("(foo")
	if _, _, _, ok := Match(ls, 0, 0); ok {
		t.Error("expected ok=false for unmatched `(`")
	}
	ls = lines("foo)")
	if _, _, _, ok := Match(ls, 0, 3); ok {
		t.Error("expected ok=false for unmatched `)`")
	}
}

func TestMatchMismatchedKindIgnoresInner(t *testing.T) {
	// `[ (x) ]` — bracket pair surrounds a paren pair. Cursor on `[`
	// must skip the `(` and `)` and match the `]` at col 6.
	ls := lines("[ (x) ]")
	anchor, match, kind, ok := Match(ls, 0, 0)
	if !ok {
		t.Fatal("expected match")
	}
	if anchor != (Pos{Row: 0, Col: 0}) || match != (Pos{Row: 0, Col: 6}) {
		t.Errorf("anchor=%+v match=%+v, want {0,0} and {0,6}", anchor, match)
	}
	if kind != Bracket {
		t.Errorf("kind = %v, want Bracket", kind)
	}
}

func TestMatchNoBracketAtCursorReturnsFalse(t *testing.T) {
	ls := lines("foobar")
	if _, _, _, ok := Match(ls, 0, 3); ok {
		t.Error("expected ok=false on plain identifier")
	}
}

func TestMatchOutOfRangeRowReturnsFalse(t *testing.T) {
	ls := lines("(x)")
	if _, _, _, ok := Match(ls, -1, 0); ok {
		t.Error("row=-1: expected ok=false")
	}
	if _, _, _, ok := Match(ls, 9, 0); ok {
		t.Error("row=9: expected ok=false")
	}
}

func TestMatchEmptyLinesSafe(t *testing.T) {
	ls := lines("")
	if _, _, _, ok := Match(ls, 0, 0); ok {
		t.Error("empty buffer: expected ok=false")
	}
}

func TestMatchColAtLineEndChecksBeforeOnly(t *testing.T) {
	// `(abc)` length 5; col=5 means "after the closer." rune-at is out of
	// range; rune-before is `)` and the walk should find `(` at col 0.
	ls := lines("(abc)")
	anchor, match, _, ok := Match(ls, 0, 5)
	if !ok {
		t.Fatal("expected match at line-end position")
	}
	if anchor != (Pos{Row: 0, Col: 4}) || match != (Pos{Row: 0, Col: 0}) {
		t.Errorf("anchor=%+v match=%+v, want {0,4}/{0,0}", anchor, match)
	}
}

func TestMatchAllThreeKinds(t *testing.T) {
	body := "({[x]})"
	ls := lines(body)
	// Outer paren at col 0 → col 6.
	_, m, _, ok := Match(ls, 0, 0)
	if !ok || m != (Pos{Row: 0, Col: 6}) {
		t.Errorf("paren: match=%+v ok=%v, want col=6 ok=true", m, ok)
	}
	// Brace at col 1 → col 5.
	_, m, _, ok = Match(ls, 0, 1)
	if !ok || m != (Pos{Row: 0, Col: 5}) {
		t.Errorf("brace: match=%+v ok=%v, want col=5 ok=true", m, ok)
	}
	// Bracket at col 2 → col 4.
	_, m, _, ok = Match(ls, 0, 2)
	if !ok || m != (Pos{Row: 0, Col: 4}) {
		t.Errorf("bracket: match=%+v ok=%v, want col=4 ok=true", m, ok)
	}
}

func TestMatchScanBudgetReturnsFalseGracefully(t *testing.T) {
	// Build a buffer with `(` at the start and the matching `)` way past
	// the budget. Match should bail rather than scan forever. Use 200 KiB
	// of non-bracket bytes, which is comfortably under the budget but
	// proves the loop terminates.
	body := "(" + strings.Repeat("x", 200_000) + ")"
	ls := lines(body)
	_, m, _, ok := Match(ls, 0, 0)
	if !ok {
		t.Fatal("expected match within budget")
	}
	if m.Col != 200_001 {
		t.Errorf("match.Col = %d, want 200001", m.Col)
	}
	// Now force budget overrun: 2 MiB of x.
	big := "(" + strings.Repeat("x", scanBudget+1) + ")"
	bls := lines(big)
	if _, _, _, ok := Match(bls, 0, 0); ok {
		t.Error("expected ok=false when scan exceeds budget")
	}
}

func TestMatchAnchorPosByteAccurate(t *testing.T) {
	// Make sure anchor.Col indexes the actual bracket rune even when prior
	// cells held multibyte UTF-8. Use the rune ñ (2 bytes) as a prefix.
	body := "ñ(x)"
	ls := lines(body)
	// `(` lives at byte col 2 (ñ takes bytes 0-1).
	cursor := bytes.IndexByte(ls[0], '(')
	if cursor != 2 {
		t.Fatalf("expected `(` at byte col 2, got %d", cursor)
	}
	anchor, match, _, ok := Match(ls, 0, cursor)
	if !ok {
		t.Fatal("expected match across multibyte prefix")
	}
	if anchor.Col != 2 || match.Col != 4 {
		t.Errorf("got anchor=%+v match=%+v, want cols 2 and 4", anchor, match)
	}
}
