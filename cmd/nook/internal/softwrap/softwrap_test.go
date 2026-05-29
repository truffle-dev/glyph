package softwrap

import (
	"reflect"
	"testing"
)

func TestWrapPoints_EmptyLine(t *testing.T) {
	got := WrapPoints([]byte{}, 80, 4)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(empty)=%v want %v", got, want)
	}
}

func TestWrapPoints_SingleChar(t *testing.T) {
	got := WrapPoints([]byte("x"), 80, 4)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(\"x\")=%v want %v", got, want)
	}
}

func TestWrapPoints_ShortLineBelowWidth(t *testing.T) {
	got := WrapPoints([]byte("hello world"), 80, 4)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(short)=%v want %v", got, want)
	}
}

func TestWrapPoints_LineExactlyWidth(t *testing.T) {
	got := WrapPoints([]byte("abcdefghij"), 10, 4)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(exact)=%v want %v", got, want)
	}
}

func TestWrapPoints_WrapAtLastSpace(t *testing.T) {
	got := WrapPoints([]byte("the quick brown fox"), 10, 4)
	want := []int{0, 10}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(quick fox 10)=%v want %v", got, want)
	}
}

func TestWrapPoints_NoSpaceHardBreak(t *testing.T) {
	got := WrapPoints([]byte("abcdefghi"), 5, 4)
	want := []int{0, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(abcdefghi 5)=%v want %v", got, want)
	}
}

func TestWrapPoints_MultipleWrapsAtSpaces(t *testing.T) {
	got := WrapPoints([]byte("abc def ghi jkl"), 7, 4)
	want := []int{0, 4, 8}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(four words 7)=%v want %v", got, want)
	}
}

func TestWrapPoints_TabLeadingExpansion(t *testing.T) {
	got := WrapPoints([]byte("\t\tabcd"), 10, 4)
	want := []int{0, 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(two tabs + abcd, w=10 tw=4)=%v want %v", got, want)
	}
}

func TestWrapPoints_TabMidRowAdvancesToNextStop(t *testing.T) {
	got := WrapPoints([]byte("ab\tcd"), 8, 4)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(ab\\tcd w=8 tw=4)=%v want %v", got, want)
	}
}

func TestWrapPoints_WidthZeroSingleRow(t *testing.T) {
	got := WrapPoints([]byte("anything"), 0, 4)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(width=0)=%v want %v", got, want)
	}
}

func TestWrapPoints_WidthNegativeSingleRow(t *testing.T) {
	got := WrapPoints([]byte("anything"), -1, 4)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(width=-1)=%v want %v", got, want)
	}
}

func TestWrapPoints_MultibyteRuneBoundary(t *testing.T) {
	got := WrapPoints([]byte("αβγδε"), 3, 4)
	want := []int{0, 6}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(αβγδε w=3)=%v want %v", got, want)
	}
}

func TestWrapPoints_MultibyteNeverSplitsRune(t *testing.T) {
	line := []byte("αβ γδ εζ")
	got := WrapPoints(line, 5, 4)
	for _, idx := range got {
		if idx > 0 && idx < len(line) {
			b := line[idx]
			if b&0xC0 == 0x80 {
				t.Fatalf("WrapPoints split rune: idx=%d byte=%x in %q", idx, b, string(line))
			}
		}
	}
}

func TestWrapPoints_SingleTabWiderThanWidthAlone(t *testing.T) {
	got := WrapPoints([]byte("\t"), 1, 4)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(\\t w=1)=%v want %v", got, want)
	}
}

func TestWrapPoints_SingleTabWiderThanWidthWithFollower(t *testing.T) {
	got := WrapPoints([]byte("\ta"), 1, 4)
	want := []int{0, 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(\\ta w=1)=%v want %v", got, want)
	}
}

func TestWrapPoints_TrailingSpaceAtBoundary(t *testing.T) {
	got := WrapPoints([]byte("abc "), 3, 4)
	want := []int{0, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(abc<space> w=3)=%v want %v", got, want)
	}
}

func TestWrapPoints_LeadingTabsForcingWrap(t *testing.T) {
	got := WrapPoints([]byte("\t\t\thello"), 8, 4)
	want := []int{0, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(3 tabs + hello w=8)=%v want %v", got, want)
	}
}

func TestWrapPoints_WordLongerThanWidth(t *testing.T) {
	got := WrapPoints([]byte("hi supercalifragilistic"), 8, 4)
	if got[0] != 0 {
		t.Fatalf("WrapPoints(long word in sentence): first index %d != 0", got[0])
	}
	if len(got) < 2 {
		t.Fatalf("WrapPoints(long word in sentence): expected at least 2 rows, got %v", got)
	}
}

func TestWrapPoints_AllTabs(t *testing.T) {
	got := WrapPoints([]byte("\t\t\t\t"), 8, 4)
	want := []int{0, 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(four tabs w=8 tw=4)=%v want %v", got, want)
	}
}

func TestWrapPoints_NegativeTabWidthDefensive(t *testing.T) {
	got := WrapPoints([]byte("\tabc"), 10, -1)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(tw=-1)=%v want %v", got, want)
	}
}

func TestWrapPoints_ZeroTabWidthDefensive(t *testing.T) {
	got := WrapPoints([]byte("\tabc"), 10, 0)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WrapPoints(tw=0)=%v want %v", got, want)
	}
}

func TestWrapPoints_LongLineMultipleWraps(t *testing.T) {
	line := []byte("the quick brown fox jumps over the lazy dog")
	got := WrapPoints(line, 10, 4)
	if got[0] != 0 {
		t.Fatalf("WrapPoints multi: first %d != 0", got[0])
	}
	for k, idx := range got {
		if idx < 0 || idx > len(line) {
			t.Fatalf("WrapPoints multi: idx[%d]=%d out of range [0,%d]", k, idx, len(line))
		}
		if k > 0 && idx <= got[k-1] {
			t.Fatalf("WrapPoints multi: non-monotonic at k=%d: %v", k, got)
		}
	}
}

func TestLineRowCount_EmptyOne(t *testing.T) {
	if n := LineRowCount([]byte{}, 80, 4); n != 1 {
		t.Fatalf("LineRowCount(empty)=%d want 1", n)
	}
}

func TestLineRowCount_ShortOne(t *testing.T) {
	if n := LineRowCount([]byte("hi"), 80, 4); n != 1 {
		t.Fatalf("LineRowCount(short)=%d want 1", n)
	}
}

func TestLineRowCount_WrappedTwo(t *testing.T) {
	if n := LineRowCount([]byte("the quick brown fox"), 10, 4); n != 2 {
		t.Fatalf("LineRowCount(quick fox 10)=%d want 2", n)
	}
}

func TestLineRowCount_MatchesWrapPoints(t *testing.T) {
	line := []byte("abc def ghi jkl mno pqr")
	for w := 1; w <= 30; w++ {
		wp := WrapPoints(line, w, 4)
		n := LineRowCount(line, w, 4)
		if n != len(wp) {
			t.Fatalf("LineRowCount/WrapPoints mismatch w=%d: count=%d points=%v", w, n, wp)
		}
	}
}

func TestWrapPoints_FuzzNoRuneSplitNoRegression(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"the quick brown fox jumps over the lazy dog",
		"\t\tlet x = 42;",
		"αβγ δεζ ηθι",
		"日本語のテスト",
		"\t",
		"a",
		"a b c d e f g h i j",
		"supercalifragilisticexpialidocious",
	}
	widths := []int{1, 2, 3, 5, 8, 10, 20, 80}
	for _, s := range cases {
		for _, w := range widths {
			line := []byte(s)
			pts := WrapPoints(line, w, 4)
			if len(pts) == 0 || pts[0] != 0 {
				t.Fatalf("WrapPoints(%q,%d) bad header: %v", s, w, pts)
			}
			for k, idx := range pts {
				if idx < 0 || idx > len(line) {
					t.Fatalf("WrapPoints(%q,%d) idx[%d]=%d out of range", s, w, k, idx)
				}
				if idx > 0 && idx < len(line) {
					if line[idx]&0xC0 == 0x80 {
						t.Fatalf("WrapPoints(%q,%d) split rune at %d", s, w, idx)
					}
				}
				if k > 0 && idx <= pts[k-1] {
					t.Fatalf("WrapPoints(%q,%d) non-monotonic at k=%d: %v", s, w, k, pts)
				}
			}
		}
	}
}
