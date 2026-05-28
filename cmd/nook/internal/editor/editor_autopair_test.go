package editor

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAutoPair_OpenerInsertsCloserAndParksCursor(t *testing.T) {
	cases := []struct {
		open, want rune
	}{
		{'(', ')'},
		{'[', ']'},
		{'{', '}'},
		{'"', '"'},
		{'\'', '\''},
		{'`', '`'},
	}
	for _, c := range cases {
		p := openGoBuf(t, "\n")
		p.row, p.col = 0, 0
		p, _ = p.Update(runeMsg(string(c.open)))
		got := p.Line(0)
		want := string([]rune{c.open, c.want})
		if got != want {
			t.Errorf("opener %q → line %q, want %q", c.open, got, want)
		}
		if p.col != 1 {
			t.Errorf("opener %q → col %d, want 1", c.open, p.col)
		}
		if !p.Dirty() {
			t.Errorf("opener %q → buffer should be dirty", c.open)
		}
	}
}

func TestAutoPair_OpenerBeforeWordRuneDoesNotPair(t *testing.T) {
	// cursor between f and oo. Typing ( should NOT pair — that would
	// shred the identifier.
	p := openGoBuf(t, "foo\n")
	p.row, p.col = 0, 1
	p, _ = p.Update(runeMsg("("))
	if got := p.Line(0); got != "f(oo" {
		t.Errorf("line = %q, want %q", got, "f(oo")
	}
	if p.col != 2 {
		t.Errorf("col = %d, want 2", p.col)
	}
}

func TestAutoPair_QuoteAfterWordIsSuppressed(t *testing.T) {
	// `it` then ' — must not pair.
	p := openGoBuf(t, "it\n")
	p.row, p.col = 0, 2
	p, _ = p.Update(runeMsg("'"))
	if got := p.Line(0); got != "it'" {
		t.Errorf("line = %q, want %q", got, "it'")
	}
	if p.col != 3 {
		t.Errorf("col = %d, want 3", p.col)
	}
}

func TestAutoPair_QuoteAfterSpacePairs(t *testing.T) {
	p := openGoBuf(t, "say \n")
	p.row, p.col = 0, 4
	p, _ = p.Update(runeMsg(`"`))
	if got := p.Line(0); got != `say ""` {
		t.Errorf("line = %q, want %q", got, `say ""`)
	}
	if p.col != 5 {
		t.Errorf("col = %d, want 5", p.col)
	}
}

func TestAutoPair_CloserSkipsOverMatchingCloser(t *testing.T) {
	// User typed `(`, which auto-paired to `()` with cursor at col 1.
	// Now typing `)` should skip over the parked closer, not insert a
	// duplicate.
	p := openGoBuf(t, "\n")
	p.row, p.col = 0, 0
	p, _ = p.Update(runeMsg("("))
	p, _ = p.Update(runeMsg(")"))
	if got := p.Line(0); got != "()" {
		t.Errorf("line = %q, want %q", got, "()")
	}
	if p.col != 2 {
		t.Errorf("col = %d, want 2", p.col)
	}
}

func TestAutoPair_CloserOnNonMatchingNextChar(t *testing.T) {
	// `(a` cursor after `a`. Typing `)` should insert because the
	// next char is EOL, not a closer (no skip needed).
	p := openGoBuf(t, "(a\n")
	p.row, p.col = 0, 2
	p, _ = p.Update(runeMsg(")"))
	if got := p.Line(0); got != "(a)" {
		t.Errorf("line = %q, want %q", got, "(a)")
	}
	if p.col != 3 {
		t.Errorf("col = %d, want 3", p.col)
	}
}

func TestAutoPair_BackspaceInsideEmptyPairDeletesBoth(t *testing.T) {
	cases := []string{"()", "[]", "{}", `""`, `''`, "``"}
	for _, pair := range cases {
		p := openGoBuf(t, "\n")
		p.row, p.col = 0, 0
		p, _ = p.Update(runeMsg(string(rune(pair[0]))))
		// Sanity-check the auto-pair landed.
		if got := p.Line(0); got != pair {
			t.Fatalf("setup: line = %q, want %q", got, pair)
		}
		if p.col != 1 {
			t.Fatalf("setup: col = %d, want 1", p.col)
		}
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		if got := p.Line(0); got != "" {
			t.Errorf("pair %q backspace → line %q, want empty", pair, got)
		}
		if p.col != 0 {
			t.Errorf("pair %q backspace → col %d, want 0", pair, p.col)
		}
	}
}

func TestAutoPair_BackspaceInsideNonEmptyPairOnlyDeletesOne(t *testing.T) {
	// `(a)` cursor at col 1, after the opener. Backspace must NOT eat
	// the closer — only the opener.
	p := openGoBuf(t, "(a)\n")
	p.row, p.col = 0, 1
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if got := p.Line(0); got != "a)" {
		t.Errorf("line = %q, want %q", got, "a)")
	}
	if p.col != 0 {
		t.Errorf("col = %d, want 0", p.col)
	}
}

func TestAutoPair_BackspaceMismatchedPairNoDouble(t *testing.T) {
	// `(]` is not a recognized pair — backspace at col 1 deletes only
	// the opener.
	p := openGoBuf(t, "(]\n")
	p.row, p.col = 0, 1
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if got := p.Line(0); got != "]" {
		t.Errorf("line = %q, want %q", got, "]")
	}
	if p.col != 0 {
		t.Errorf("col = %d, want 0", p.col)
	}
}

func TestAutoPair_WithSelectionUsesPlainInsert(t *testing.T) {
	// Select `foo` and type `(`. Behavior: selection replaced with `(`,
	// then auto-pair runs? Or selection replaced with `(` plain? Zed/
	// VSCode default is "surround": opener wraps selection. But our
	// editor's selection path currently goes p.DeleteSelection() +
	// insertRunes(km.Runes) — no auto-pair. Test asserts the existing
	// behavior: selection deleted, just the opener inserted.
	p := openGoBuf(t, "foo bar\n")
	p.row, p.col = 0, 0
	p.selecting = true
	p.anchorRow, p.anchorCol = 0, 0
	// Move to col 3 with selection extending.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	// Type '(' — selection branch should fire (not auto-pair).
	p, _ = p.Update(runeMsg("("))
	if got := p.Line(0); got != "( bar" {
		t.Errorf("line = %q, want %q", got, "( bar")
	}
}

func TestAutoPair_MultiCursorBypassesAutoPair(t *testing.T) {
	// With an extra cursor stacked, typing `(` should insert literal
	// `(` at every cursor — no closer, no parking.
	p := openGoBuf(t, "a\nb\n")
	p.row, p.col = 0, 0
	p.extras = []extraCursor{{Row: 1, Col: 0}}
	p, _ = p.Update(runeMsg("("))
	if got := p.Line(0); got != "(a" {
		t.Errorf("line 0 = %q, want %q", got, "(a")
	}
	if got := p.Line(1); got != "(b" {
		t.Errorf("line 1 = %q, want %q", got, "(b")
	}
}

func TestAutoPair_BufVerAdvancesOnPair(t *testing.T) {
	p := openGoBuf(t, "\n")
	p.row, p.col = 0, 0
	before := p.bufVer
	p, _ = p.Update(runeMsg("("))
	if p.bufVer == before {
		t.Errorf("bufVer should advance after auto-pair insert; got %d", p.bufVer)
	}
}

func TestAutoPair_SkipDoesNotAdvanceBufVer(t *testing.T) {
	p := openGoBuf(t, "()\n")
	p.row, p.col = 0, 1
	before := p.bufVer
	p, _ = p.Update(runeMsg(")"))
	if p.bufVer != before {
		t.Errorf("bufVer should stay at %d on skip-over; got %d", before, p.bufVer)
	}
	if p.col != 2 {
		t.Errorf("col should advance to 2 on skip; got %d", p.col)
	}
}

func TestAutoPair_BackspaceAtEmptyPairAdvancesBufVer(t *testing.T) {
	p := openGoBuf(t, "()\n")
	p.row, p.col = 0, 1
	before := p.bufVer
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.bufVer == before {
		t.Errorf("bufVer should advance after empty-pair backspace; got %d", p.bufVer)
	}
}

func TestAutoPair_RoundTrip(t *testing.T) {
	// Type `(`, immediately backspace, end up with empty buffer.
	p := openGoBuf(t, "\n")
	p.row, p.col = 0, 0
	p, _ = p.Update(runeMsg("("))
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if got := p.Line(0); got != "" {
		t.Errorf("round-trip line = %q, want empty", got)
	}
	if p.col != 0 {
		t.Errorf("round-trip col = %d, want 0", p.col)
	}
}
