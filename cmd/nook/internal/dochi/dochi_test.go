package dochi

import "testing"

func TestLineSpansSingleLine(t *testing.T) {
	hi := []Highlight{{StartLine: 5, StartCol: 2, EndLine: 5, EndCol: 7, Kind: KindText}}
	got := LineSpans(hi)
	if len(got) != 1 {
		t.Fatalf("want 1 row, got %d (%v)", len(got), got)
	}
	if len(got[5]) != 1 || got[5][0] != (Span{Start: 2, End: 7}) {
		t.Fatalf("want row 5 span {2,7}, got %v", got[5])
	}
}

func TestLineSpansMultiLine(t *testing.T) {
	hi := []Highlight{{StartLine: 1, StartCol: 4, EndLine: 4, EndCol: 3}}
	got := LineSpans(hi)
	if got[1][0] != (Span{Start: 4, End: -1}) {
		t.Fatalf("row 1: want {4,-1}, got %v", got[1])
	}
	if got[2][0] != (Span{Start: 0, End: -1}) {
		t.Fatalf("row 2: want {0,-1}, got %v", got[2])
	}
	if got[3][0] != (Span{Start: 0, End: -1}) {
		t.Fatalf("row 3: want {0,-1}, got %v", got[3])
	}
	if got[4][0] != (Span{Start: 0, End: 3}) {
		t.Fatalf("row 4: want {0,3}, got %v", got[4])
	}
}

func TestLineSpansMultipleSameRow(t *testing.T) {
	hi := []Highlight{
		{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 3, Kind: KindWrite},
		{StartLine: 0, StartCol: 10, EndLine: 0, EndCol: 13, Kind: KindRead},
	}
	got := LineSpans(hi)
	if len(got[0]) != 2 {
		t.Fatalf("want 2 spans on row 0, got %d (%v)", len(got[0]), got[0])
	}
}

func TestLineSpansDropsMalformed(t *testing.T) {
	hi := []Highlight{
		{StartLine: -1, EndLine: 2},                                          // negative start
		{StartLine: 5, EndLine: 4},                                           // end before start
		{StartLine: 3, StartCol: 5, EndLine: 3, EndCol: 5},                   // empty same-line span
		{StartLine: 7, StartCol: 4, EndLine: 7, EndCol: 2},                   // backwards same-line
		{StartLine: 9, StartCol: -1, EndLine: 9, EndCol: 4},                  // negative start col
		{StartLine: 11, StartCol: 0, EndLine: 11, EndCol: 5, Kind: KindText}, // valid
	}
	got := LineSpans(hi)
	if len(got) != 1 {
		t.Fatalf("want only the valid span to survive, got %v", got)
	}
	if _, ok := got[11]; !ok {
		t.Fatalf("want row 11 surviving, got rows %v", got)
	}
}

func TestLineSpansMultiLineNoEndCol(t *testing.T) {
	// Multi-line highlight ending at column 0 of EndLine: nothing to paint on EndLine.
	hi := []Highlight{{StartLine: 0, StartCol: 0, EndLine: 2, EndCol: 0}}
	got := LineSpans(hi)
	if _, ok := got[2]; ok {
		t.Fatalf("row 2 should be empty when EndCol==0, got %v", got[2])
	}
	if got[0][0] != (Span{Start: 0, End: -1}) {
		t.Fatalf("row 0: want {0,-1}, got %v", got[0])
	}
	if got[1][0] != (Span{Start: 0, End: -1}) {
		t.Fatalf("row 1: want {0,-1}, got %v", got[1])
	}
}

func TestCoversInsideRange(t *testing.T) {
	spans := []Span{{Start: 2, End: 7}}
	if !Covers(spans, 2, 20) {
		t.Errorf("col 2 should be covered (inclusive start)")
	}
	if !Covers(spans, 6, 20) {
		t.Errorf("col 6 should be covered")
	}
	if Covers(spans, 7, 20) {
		t.Errorf("col 7 should NOT be covered (exclusive end)")
	}
	if Covers(spans, 1, 20) {
		t.Errorf("col 1 should NOT be covered")
	}
}

func TestCoversNegativeEndExtendsToRowLen(t *testing.T) {
	spans := []Span{{Start: 4, End: -1}}
	if !Covers(spans, 5, 10) {
		t.Errorf("col 5 should be covered when end is -1 and row is 10")
	}
	if !Covers(spans, 10, 10) {
		t.Errorf("col 10 should be covered up to row length")
	}
	if Covers(spans, 11, 10) {
		t.Errorf("col 11 (past row length) should NOT be covered")
	}
	if Covers(spans, 3, 10) {
		t.Errorf("col 3 (before start) should NOT be covered")
	}
}

func TestCoversMultipleSpans(t *testing.T) {
	spans := []Span{{Start: 0, End: 3}, {Start: 10, End: 13}}
	if !Covers(spans, 1, 20) {
		t.Errorf("col 1 should be in first span")
	}
	if !Covers(spans, 11, 20) {
		t.Errorf("col 11 should be in second span")
	}
	if Covers(spans, 5, 20) {
		t.Errorf("col 5 between spans should NOT be covered")
	}
}

func TestCoversEmpty(t *testing.T) {
	if Covers(nil, 5, 20) {
		t.Errorf("nil spans should never cover")
	}
	if Covers([]Span{}, 0, 0) {
		t.Errorf("empty spans should never cover")
	}
}
