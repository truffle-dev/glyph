package editor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

// guidesInContent counts vertical-line glyphs that fall in a row's content
// region (past the gutter+marker prefix). The marker column always paints
// one '│' as the diagnostic separator, so we subtract that one occurrence
// per non-empty row to leave only the indent-guide glyphs.
func guidesInContent(rendered string) int {
	// strip ANSI so the count is on raw runes
	stripped := plain(rendered)
	rows := strings.Split(stripped, "\n")
	if len(rows) <= 1 {
		return 0
	}
	total := 0
	for _, r := range rows[1:] { // rows[0] is the header
		// every visible row carries one '│' in the marker column. Empty
		// (~ tilde) rows beyond EOF still carry a marker.
		c := strings.Count(r, "│")
		if c > 0 {
			c--
		}
		total += c
	}
	return total
}

func TestIndentGuidesDefaultsOn(t *testing.T) {
	p := NewPane(theme.Default)
	if !p.IndentGuides() {
		t.Fatalf("expected IndentGuides()=true after NewPane, got false")
	}
}

func TestSetIndentGuidesToggles(t *testing.T) {
	p := NewPane(theme.Default).SetIndentGuides(false)
	if p.IndentGuides() {
		t.Fatalf("expected IndentGuides()=false after SetIndentGuides(false)")
	}
	p = p.SetIndentGuides(true)
	if !p.IndentGuides() {
		t.Fatalf("expected IndentGuides()=true after SetIndentGuides(true)")
	}
}

func TestIndentGuidePaintsAtIndentStop(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 10).Focus().SetTabWidth(4)
	// 8 spaces + content. Depth 2 → one guide at visual col 4.
	p, _ = p.Update(runeMsg("        foo"))
	// Move cursor off the row so the cursor cell doesn't claim col 4.
	p.row, p.col = 0, 0
	p.buf.Lines = []string{"", "        foo"}
	if got := guidesInContent(p.View()); got < 1 {
		t.Fatalf("expected at least 1 indent guide, got %d in:\n%s", got, plain(p.View()))
	}
}

func TestIndentGuideTogglesOffNoGlyph(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 10).Focus().SetTabWidth(4).SetIndentGuides(false)
	p.buf.Lines = []string{"", "        foo"}
	p.row, p.col = 0, 0
	if got := guidesInContent(p.View()); got != 0 {
		t.Fatalf("expected 0 indent guides when toggled off, got %d in:\n%s", got, plain(p.View()))
	}
}

func TestIndentGuideOneStopDepthThree(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 10).Focus().SetTabWidth(4)
	// 12 spaces + content. Depth 3 → guides at cols 4 and 8.
	p.buf.Lines = []string{"", "            x"}
	p.row, p.col = 0, 0
	if got := guidesInContent(p.View()); got != 2 {
		t.Fatalf("expected 2 indent guides on depth-3 row, got %d in:\n%s", got, plain(p.View()))
	}
}

func TestIndentGuideDepthOnePaintsZero(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 10).Focus().SetTabWidth(4)
	// 4 spaces + content. Leading-ws width == tabWidth, so the candidate
	// stop at col 4 is not strictly less than 4: depth-1 paints no guides.
	p.buf.Lines = []string{"    foo"}
	p.row, p.col = 0, 0
	if got := guidesInContent(p.View()); got != 0 {
		t.Fatalf("expected 0 guides on depth-1 row, got %d in:\n%s", got, plain(p.View()))
	}
}

func TestIndentGuideShallowLeadingWhitespacePaintsZero(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 10).Focus().SetTabWidth(4)
	// 3 spaces (below first stop) + content → no guide.
	p.buf.Lines = []string{"   foo"}
	p.row, p.col = 0, 0
	if got := guidesInContent(p.View()); got != 0 {
		t.Fatalf("expected 0 guides on shallow row, got %d", got)
	}
}

func TestIndentGuideNotPaintedUnderCursor(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 10).Focus().SetTabWidth(4)
	// Depth-2 row with cursor parked on visual col 4 — the cursor cell
	// owns col 4 and should suppress the guide there, leaving 0 guide
	// glyphs in the content region (cursor-cell rendering uses a space,
	// not '│').
	p.buf.Lines = []string{"        foo"}
	p.row, p.col = 0, 4
	if got := guidesInContent(p.View()); got != 0 {
		t.Fatalf("expected guide suppressed under cursor, got %d in:\n%s", got, plain(p.View()))
	}
}

func TestIndentGuideTabRowPaintsAtTabStops(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 10).Focus().SetTabWidth(4)
	// Two tabs (each expands to 4 spaces) = 8 visual cols → one guide at
	// col 4 because the tab boundary at col 4 falls inside the leading
	// whitespace and the second stop at col 8 sits on 'f'.
	p.buf.Lines = []string{"", "\t\tfoo"}
	p.row, p.col = 0, 0
	if got := guidesInContent(p.View()); got != 1 {
		t.Fatalf("expected 1 guide on \\t\\tfoo, got %d in:\n%s", got, plain(p.View()))
	}
}

func TestIndentGuideAllWhitespaceRow(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 10).Focus().SetTabWidth(4)
	// 12 spaces, no content. Should paint two guides at 4 and 8 since
	// the row is itself an indent context.
	p.buf.Lines = []string{"", "            "}
	p.row, p.col = 0, 0
	if got := guidesInContent(p.View()); got != 2 {
		t.Fatalf("expected 2 guides on all-whitespace row, got %d in:\n%s", got, plain(p.View()))
	}
}

func TestIndentGuideTabWidthTwo(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 10).Focus().SetTabWidth(2)
	// 6 spaces + content. With tabWidth=2 → guides at cols 2 and 4.
	p.buf.Lines = []string{"", "      foo"}
	p.row, p.col = 0, 0
	if got := guidesInContent(p.View()); got != 2 {
		t.Fatalf("expected 2 guides at tabWidth=2 depth-3, got %d in:\n%s", got, plain(p.View()))
	}
}

// ensure motion + indent guides interplay doesn't crash and respects the
// cursor row.
func TestIndentGuideCursorMoveStillRenders(t *testing.T) {
	p := NewPane(theme.Default).WithSize(80, 10).Focus().SetTabWidth(4)
	p.buf.Lines = []string{"        foo", "            bar"}
	p.row, p.col = 1, 0
	// move cursor onto the second row, col 0 — guides on row 1 should
	// still paint normally (2 stops: 4, 8). row 0 should paint 1 stop.
	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	view := p.View()
	if got := guidesInContent(view); got < 3 {
		t.Fatalf("expected >=3 total guides across rows, got %d in:\n%s", got, plain(view))
	}
}
