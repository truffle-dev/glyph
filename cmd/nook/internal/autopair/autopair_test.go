package autopair

import "testing"

func TestOpenerFor(t *testing.T) {
	cases := []struct {
		in    rune
		want  rune
		ok    bool
		label string
	}{
		{'(', ')', true, "paren"},
		{'[', ']', true, "bracket"},
		{'{', '}', true, "brace"},
		{'"', '"', true, "double-quote symmetric"},
		{'\'', '\'', true, "single-quote symmetric"},
		{'`', '`', true, "backtick symmetric"},
		{')', 0, false, "closer is not opener"},
		{'a', 0, false, "letter is not opener"},
		{' ', 0, false, "space is not opener"},
		{0, 0, false, "zero rune"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			got, ok := OpenerFor(c.in)
			if ok != c.ok || got != c.want {
				t.Fatalf("OpenerFor(%q) = (%q,%v), want (%q,%v)",
					c.in, got, ok, c.want, c.ok)
			}
		})
	}
}

func TestIsCloserOpenerSymmetric(t *testing.T) {
	if !IsOpener('(') || !IsOpener('[') || !IsOpener('{') {
		t.Fatal("brackets should be openers")
	}
	if !IsOpener('"') || !IsOpener('\'') || !IsOpener('`') {
		t.Fatal("quotes should be openers")
	}
	if IsOpener(')') || IsOpener(']') || IsOpener('}') {
		t.Fatal("closers should not be openers")
	}
	if !IsCloser(')') || !IsCloser(']') || !IsCloser('}') {
		t.Fatal("brackets-close should be closers")
	}
	if !IsCloser('"') || !IsCloser('\'') || !IsCloser('`') {
		t.Fatal("quotes are also closers (symmetric)")
	}
	if !IsSymmetric('"') || !IsSymmetric('\'') || !IsSymmetric('`') {
		t.Fatal("quotes are symmetric")
	}
	if IsSymmetric('(') || IsSymmetric('[') || IsSymmetric('{') {
		t.Fatal("brackets are not symmetric")
	}
}

func TestShouldPair_NonOpener(t *testing.T) {
	if ShouldPair("abc", 0, 'x') {
		t.Fatal("non-opener rune should not pair")
	}
}

func TestShouldPair_AtEnd(t *testing.T) {
	if !ShouldPair("foo", 3, '(') {
		t.Fatal("paren at end of line should pair (next char is EOL)")
	}
}

func TestShouldPair_BeforeWordRuneIsSuppressed(t *testing.T) {
	// cursor between f and oo — typing ( should NOT pair, otherwise
	// `foo` becomes `f()oo`.
	if ShouldPair("foo", 1, '(') {
		t.Fatal("paren before word rune should not pair")
	}
}

func TestShouldPair_BeforeWhitespacePairs(t *testing.T) {
	// `foo cursor` — typing ( pairs because the next char is space.
	if !ShouldPair("foo bar", 3, '(') {
		t.Fatal("paren before whitespace should pair")
	}
}

func TestShouldPair_BeforePunctuationPairs(t *testing.T) {
	// `foo;` cursor before ; — typing ( pairs.
	if !ShouldPair("foo;bar", 3, '(') {
		t.Fatal("paren before punctuation should pair")
	}
}

func TestShouldPair_QuoteAfterWordIsSuppressed(t *testing.T) {
	// `it` then ' — should NOT pair, lest contractions blow up.
	if ShouldPair("it", 2, '\'') {
		t.Fatal("single quote after word rune should not pair")
	}
}

func TestShouldPair_QuoteAfterWhitespacePairs(t *testing.T) {
	if !ShouldPair("say ", 4, '"') {
		t.Fatal("double quote after whitespace should pair")
	}
}

func TestShouldPair_QuoteOnTopOfSameQuoteSuppressed(t *testing.T) {
	// `"a"` and cursor at the closing quote — typing " should not pair
	// (skip-over wins instead).
	if ShouldPair(`"a"`, 2, '"') {
		t.Fatal(`typing " on top of " should not pair`)
	}
}

func TestShouldPair_QuoteWithDifferentNextPairs(t *testing.T) {
	// after `(` cursor — typing " pairs.
	if !ShouldPair("(", 1, '"') {
		t.Fatal(`typing " after ( should pair`)
	}
}

func TestShouldPair_QuoteAtStartOfLine(t *testing.T) {
	if !ShouldPair("", 0, '"') {
		t.Fatal(`typing " at empty line should pair`)
	}
}

func TestShouldPair_BoundsGuard(t *testing.T) {
	if ShouldPair("ab", -1, '(') {
		t.Fatal("negative col should not pair")
	}
	if ShouldPair("ab", 5, '(') {
		t.Fatal("out-of-range col should not pair")
	}
}

func TestShouldPair_ParenAfterIdentifierPairs(t *testing.T) {
	// `foo` cursor at end — typing ( pairs because next char is EOL,
	// even though prev char is word. Word-prev only suppresses for
	// symmetric quotes.
	if !ShouldPair("foo", 3, '(') {
		t.Fatal("paren after identifier at EOL should pair")
	}
}

func TestShouldSkip_CloserOnTopOfMatch(t *testing.T) {
	// `(|)` cursor at byte 1 — typing ) should skip.
	if !ShouldSkip("()", 1, ')') {
		t.Fatal(`typing ) on top of ) should skip`)
	}
}

func TestShouldSkip_QuoteOnTopOfMatch(t *testing.T) {
	if !ShouldSkip(`""`, 1, '"') {
		t.Fatal(`typing " on top of " should skip`)
	}
}

func TestShouldSkip_NonCloser(t *testing.T) {
	if ShouldSkip("abc", 0, 'a') {
		t.Fatal("non-closer should not skip")
	}
}

func TestShouldSkip_NoMatch(t *testing.T) {
	// `(a` cursor at byte 1 — typing ) should not skip because next is `a`.
	if ShouldSkip("(a", 1, ')') {
		t.Fatal("closer with non-matching next char should not skip")
	}
}

func TestShouldSkip_AtEnd(t *testing.T) {
	if ShouldSkip("(", 1, ')') {
		t.Fatal("closer at EOL should not skip (nothing to skip over)")
	}
}

func TestIsEmptyPair_Match(t *testing.T) {
	cases := []struct {
		line string
		col  int
		want bool
		lab  string
	}{
		{"()", 1, true, "empty paren"},
		{"[]", 1, true, "empty bracket"},
		{"{}", 1, true, "empty brace"},
		{`""`, 1, true, "empty double-quote"},
		{`''`, 1, true, "empty single-quote"},
		{"``", 1, true, "empty backtick"},
		{"(a)", 1, false, "non-empty paren"},
		{"()", 0, false, "cursor before opener"},
		{"()", 2, false, "cursor after closer"},
		{"", 0, false, "empty line"},
		{"(", 1, false, "lonely opener at EOL"},
		{")", 0, false, "cursor before closer no opener"},
		{"a)", 1, false, "letter before closer"},
		{"(]", 1, false, "mismatched pair"},
		{"[)", 1, false, "mismatched pair"},
		{`('`, 1, false, "paren next to symmetric mismatch"},
	}
	for _, c := range cases {
		t.Run(c.lab, func(t *testing.T) {
			got := IsEmptyPair(c.line, c.col)
			if got != c.want {
				t.Fatalf("IsEmptyPair(%q, %d) = %v, want %v",
					c.line, c.col, got, c.want)
			}
		})
	}
}

func TestUnicodePrevRuneRespected(t *testing.T) {
	// Greek letter alpha (α, 2 bytes) followed by cursor at byte 2.
	// Typing ' should not pair because alpha is a letter.
	line := "α"
	if len(line) != 2 {
		t.Fatalf("expected 2-byte alpha, got %d", len(line))
	}
	if ShouldPair(line, 2, '\'') {
		t.Fatal("quote after greek letter should not pair (word rune prev)")
	}
}

func TestUnicodeNextRuneRespected(t *testing.T) {
	line := "α"
	if ShouldPair(line, 0, '(') {
		t.Fatal("paren before greek letter should not pair (word rune next)")
	}
}
