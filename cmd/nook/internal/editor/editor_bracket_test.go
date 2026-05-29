package editor

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// bracketSig renders a bracket char under the same style View() applies, so
// the tests can grep the rendered output for the painted bracket without
// duplicating the precedence logic.
func bracketSig(t theme.Theme, b rune) string {
	st := lipgloss.NewStyle().Foreground(t.Primary).Background(t.SurfaceStrong).Bold(true)
	if string(t.SurfaceStrong) == "" {
		st = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	}
	return st.Render(string(b))
}

func TestBracketPairHighlightsMatchWhenCursorOnAnchor(t *testing.T) {
	// When the cursor sits ON a bracket, the cursor cell paints that bracket
	// (so the cursor stays visible) and the OTHER end of the pair gets the
	// bracket band. This is the "show me where the match is" mode.
	tm := theme.Default
	p := NewPane(tm).WithSize(80, 5).Focus()
	p.buf.Lines = []string{"foo(bar)"}
	p.row, p.col = 0, 3 // cursor on `(`

	out := p.View()
	if !strings.Contains(out, bracketSig(tm, ')')) {
		t.Errorf("expected styled `)` (the match) in output:\n%s", out)
	}
}

func TestBracketPairHighlightsBothWhenCursorPastBracket(t *testing.T) {
	// Cursor at col=4 sits on `b`. Rune-before is `(` at col 3 — the anchor
	// fallback path. Now neither bracket is under the cursor cell, so both
	// `(` and `)` should be in the bracket band.
	tm := theme.Default
	p := NewPane(tm).WithSize(80, 5).Focus()
	p.buf.Lines = []string{"foo(bar)"}
	p.row, p.col = 0, 4 // cursor on `b`, rune-before is `(`

	out := p.View()
	if !strings.Contains(out, bracketSig(tm, '(')) {
		t.Errorf("expected styled `(` (rune-before-cursor) in output:\n%s", out)
	}
	if !strings.Contains(out, bracketSig(tm, ')')) {
		t.Errorf("expected styled `)` (match) in output:\n%s", out)
	}
}

func TestBracketPairSuppressedWhenCursorOffBracket(t *testing.T) {
	tm := theme.Default
	p := NewPane(tm).WithSize(80, 5).Focus()
	p.buf.Lines = []string{"foo(bar)"}
	p.row, p.col = 0, 1 // cursor on `o`, not on a bracket

	out := p.View()
	if strings.Contains(out, bracketSig(tm, '(')) || strings.Contains(out, bracketSig(tm, ')')) {
		t.Errorf("expected no bracket highlight off-bracket, got:\n%s", out)
	}
}

func TestBracketPairSuppressedDuringSelection(t *testing.T) {
	tm := theme.Default
	p := NewPane(tm).WithSize(80, 5).Focus()
	p.buf.Lines = []string{"foo(bar)"}
	p.row, p.col = 0, 3
	p.selecting = true
	p.anchorRow, p.anchorCol = 0, 2 // selection covers `o(` at least

	out := p.View()
	if strings.Contains(out, bracketSig(tm, '(')) || strings.Contains(out, bracketSig(tm, ')')) {
		t.Errorf("expected no bracket highlight during selection, got:\n%s", out)
	}
}

func TestBracketPairSuppressedWithExtraCursors(t *testing.T) {
	tm := theme.Default
	p := NewPane(tm).WithSize(80, 5).Focus()
	p.buf.Lines = []string{"foo(bar)"}
	p.row, p.col = 0, 3
	p.extras = []extraCursor{{Row: 0, Col: 7}}

	out := p.View()
	if strings.Contains(out, bracketSig(tm, '(')) || strings.Contains(out, bracketSig(tm, ')')) {
		t.Errorf("expected no bracket highlight with extra cursors, got:\n%s", out)
	}
}

func TestBracketPairHighlightsAllThreeKinds(t *testing.T) {
	tm := theme.Default
	// Cursor on `b` (col=4), rune-before is the opener at col=3 — fallback
	// path so both ends of the pair sit outside the cursor cell and get the
	// bracket band.
	cases := []struct {
		body string
		open rune
		clos rune
	}{
		{"foo(bar)", '(', ')'},
		{"foo[bar]", '[', ']'},
		{"foo{bar}", '{', '}'},
	}
	for _, c := range cases {
		p := NewPane(tm).WithSize(80, 5).Focus()
		p.buf.Lines = []string{c.body}
		p.row, p.col = 0, 4

		out := p.View()
		if !strings.Contains(out, bracketSig(tm, c.open)) {
			t.Errorf("%q: missing styled %q in:\n%s", c.body, c.open, out)
		}
		if !strings.Contains(out, bracketSig(tm, c.clos)) {
			t.Errorf("%q: missing styled %q in:\n%s", c.body, c.clos, out)
		}
	}
}

func TestBracketPairHighlightsAcrossLines(t *testing.T) {
	tm := theme.Default
	p := NewPane(tm).WithSize(80, 10).Focus()
	p.buf.Lines = []string{"func() {", "  body", "}"}
	// Cursor at end of line 0 (col=8): rune-before is `{` at col 7. Both
	// `{` (row 0) and `}` (row 2) should be painted.
	p.row, p.col = 0, 8

	out := p.View()
	if !strings.Contains(out, bracketSig(tm, '{')) {
		t.Errorf("expected styled `{` in:\n%s", out)
	}
	if !strings.Contains(out, bracketSig(tm, '}')) {
		t.Errorf("expected styled `}` in:\n%s", out)
	}
}

func TestBracketPairHighlightUnderSoftWrap(t *testing.T) {
	tm := theme.Default
	// Single long line that wraps. Cursor at col=4 (`b`); `(` at col 3 and
	// `)` at the very end. Wrap puts `(` on row 0's first sub-row and `)` on
	// a later sub-row. Both should be band-painted (neither under cursor).
	body := "foo(" + strings.Repeat("bar", 14) + ")"
	p := NewPane(tm).WithSize(30, 10).Focus().SetSoftWrap(true)
	p.buf.Lines = []string{body}
	p.row, p.col = 0, 4

	out := p.View()
	if !strings.Contains(out, bracketSig(tm, '(')) {
		t.Errorf("expected styled `(` under soft wrap in:\n%s", out)
	}
	if !strings.Contains(out, bracketSig(tm, ')')) {
		t.Errorf("expected styled `)` (on later sub-row) under soft wrap in:\n%s", out)
	}
}

func TestBracketPairUnmatchedDoesNotPaint(t *testing.T) {
	tm := theme.Default
	p := NewPane(tm).WithSize(80, 5).Focus()
	p.buf.Lines = []string{"foo(bar"}
	p.row, p.col = 0, 3

	out := p.View()
	if strings.Contains(out, bracketSig(tm, '(')) {
		t.Errorf("expected no bracket highlight when unmatched, got:\n%s", out)
	}
}
