package indentguide

import (
	"reflect"
	"testing"
)

func TestVisualGuideColsLeadingTabs(t *testing.T) {
	got := VisualGuideCols("\t\t\t\tfoo", 4)
	want := []int{4, 8, 12}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("guides = %v, want %v", got, want)
	}
}

func TestVisualGuideColsLeadingSpaces(t *testing.T) {
	got := VisualGuideCols("        foo", 4)
	want := []int{4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("guides = %v, want %v", got, want)
	}
}

func TestVisualGuideColsTwelveSpaces(t *testing.T) {
	got := VisualGuideCols("            foo", 4)
	want := []int{4, 8}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("guides = %v, want %v", got, want)
	}
}

func TestVisualGuideColsMixedTabAndSpaces(t *testing.T) {
	// One tab (expands to 4 cols) followed by three spaces = 7 cols of
	// leading whitespace. tabWidth=4 → candidate {4}; 4 < 7 keeps the
	// guide at col 4.
	got := VisualGuideCols("\t   foo", 4)
	want := []int{4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("guides = %v, want %v", got, want)
	}
}

func TestVisualGuideColsNoIndent(t *testing.T) {
	if got := VisualGuideCols("foo", 4); got != nil {
		t.Fatalf("guides = %v, want nil", got)
	}
}

func TestVisualGuideColsEmpty(t *testing.T) {
	if got := VisualGuideCols("", 4); got != nil {
		t.Fatalf("guides = %v, want nil", got)
	}
}

func TestVisualGuideColsExactlyOneIndentDoesNotPaint(t *testing.T) {
	// 4 spaces of leading ws with tabWidth=4 means leading width == tabWidth.
	// The candidate stop at column 4 is NOT strictly less than 4, so it
	// is filtered out. A depth-1 row paints zero guides on purpose: the
	// gutter already separates content from the file edge.
	if got := VisualGuideCols("    foo", 4); got != nil {
		t.Fatalf("guides = %v, want nil (depth-1 paints no guides)", got)
	}
}

func TestVisualGuideColsAllWhitespaceRow(t *testing.T) {
	// A row that's entirely whitespace gets guides as if it were a
	// content row at that indent depth — the guide layer treats it as
	// part of the surrounding indent context.
	got := VisualGuideCols("            ", 4)
	want := []int{4, 8}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("guides = %v, want %v", got, want)
	}
}

func TestVisualGuideColsShallowLeadingWhitespace(t *testing.T) {
	// 3 spaces (less than one indent step) paints no guide because the
	// first candidate stop sits past the leading ws end.
	if got := VisualGuideCols("   foo", 4); got != nil {
		t.Fatalf("guides = %v, want nil", got)
	}
}

func TestVisualGuideColsZeroTabWidth(t *testing.T) {
	if got := VisualGuideCols("\t\tfoo", 0); got != nil {
		t.Fatalf("guides = %v, want nil when tabWidth=0", got)
	}
}

func TestVisualGuideColsNegativeTabWidth(t *testing.T) {
	if got := VisualGuideCols("\t\tfoo", -1); got != nil {
		t.Fatalf("guides = %v, want nil when tabWidth<0", got)
	}
}

func TestVisualGuideColsTabWidthTwo(t *testing.T) {
	// Same row, tabWidth=2. Six spaces leading → candidates {2, 4} both
	// less than 6.
	got := VisualGuideCols("      foo", 2)
	want := []int{2, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("guides = %v, want %v", got, want)
	}
}

func TestVisualGuideColsTabWidthEight(t *testing.T) {
	// 16 cols of leading ws, tabWidth=8 → guide at col 8 only (col 16
	// is filtered).
	got := VisualGuideCols("                foo", 8)
	want := []int{8}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("guides = %v, want %v", got, want)
	}
}

func TestLeadingWhitespaceVisualWidthMixedTabsSpaces(t *testing.T) {
	if got := LeadingWhitespaceVisualWidth("\t  foo", 4); got != 6 {
		t.Fatalf("width = %d, want 6", got)
	}
}

func TestLeadingWhitespaceVisualWidthNoIndent(t *testing.T) {
	if got := LeadingWhitespaceVisualWidth("foo", 4); got != 0 {
		t.Fatalf("width = %d, want 0", got)
	}
}

func TestLeadingWhitespaceVisualWidthEmpty(t *testing.T) {
	if got := LeadingWhitespaceVisualWidth("", 4); got != 0 {
		t.Fatalf("width = %d, want 0", got)
	}
}

func TestLeadingWhitespaceVisualWidthAllTabs(t *testing.T) {
	if got := LeadingWhitespaceVisualWidth("\t\t\t", 4); got != 12 {
		t.Fatalf("width = %d, want 12", got)
	}
}

func TestLeadingWhitespaceVisualWidthTabWidthTwo(t *testing.T) {
	if got := LeadingWhitespaceVisualWidth("\t\t", 2); got != 4 {
		t.Fatalf("width = %d, want 4", got)
	}
}

func TestLeadingWhitespaceVisualWidthZeroTabWidth(t *testing.T) {
	if got := LeadingWhitespaceVisualWidth("\t\t", 0); got != 0 {
		t.Fatalf("width = %d, want 0 (tabWidth<=0)", got)
	}
}

func TestActiveGuideColZero(t *testing.T) {
	if got := ActiveGuideCol(0, 4); got != -1 {
		t.Fatalf("active = %d, want -1", got)
	}
}

func TestActiveGuideColBelowFirstStop(t *testing.T) {
	if got := ActiveGuideCol(3, 4); got != -1 {
		t.Fatalf("active = %d, want -1 (col<tabWidth)", got)
	}
}

func TestActiveGuideColAtFirstStop(t *testing.T) {
	if got := ActiveGuideCol(4, 4); got != 4 {
		t.Fatalf("active = %d, want 4", got)
	}
}

func TestActiveGuideColInsideZone(t *testing.T) {
	if got := ActiveGuideCol(7, 4); got != 4 {
		t.Fatalf("active = %d, want 4", got)
	}
}

func TestActiveGuideColAtSecondStop(t *testing.T) {
	if got := ActiveGuideCol(8, 4); got != 8 {
		t.Fatalf("active = %d, want 8", got)
	}
}

func TestActiveGuideColDeepZone(t *testing.T) {
	if got := ActiveGuideCol(11, 4); got != 8 {
		t.Fatalf("active = %d, want 8", got)
	}
}

func TestActiveGuideColZeroTabWidth(t *testing.T) {
	if got := ActiveGuideCol(5, 0); got != -1 {
		t.Fatalf("active = %d, want -1 (tabWidth=0)", got)
	}
}

func TestActiveGuideColTabWidthTwo(t *testing.T) {
	// tabWidth=2 → col 0→-1, col 1→-1, col 2→2, col 3→2, col 4→4.
	cases := []struct{ col, want int }{
		{0, -1},
		{1, -1},
		{2, 2},
		{3, 2},
		{4, 4},
		{5, 4},
	}
	for _, c := range cases {
		if got := ActiveGuideCol(c.col, 2); got != c.want {
			t.Errorf("ActiveGuideCol(%d, 2) = %d, want %d", c.col, got, c.want)
		}
	}
}
