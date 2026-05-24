package findrefs

import (
	"errors"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/cmd/nook/internal/multibuffer"
)

func TestSymbolBasic(t *testing.T) {
	src := "package main\nfunc DoThing(x int) {}\n"
	got := Symbol(src, 1, 7)
	if got != "DoThing" {
		t.Fatalf("got %q want DoThing", got)
	}
}

func TestSymbolAtIdentStart(t *testing.T) {
	src := "var Counter int"
	if got := Symbol(src, 0, 4); got != "Counter" {
		t.Fatalf("at start: got %q want Counter", got)
	}
}

func TestSymbolAtIdentEnd(t *testing.T) {
	src := "var Counter int"
	if got := Symbol(src, 0, 11); got != "Counter" {
		t.Fatalf("at end: got %q want Counter", got)
	}
}

func TestSymbolNotOnIdentifier(t *testing.T) {
	src := "x = 1 +  2"
	if got := Symbol(src, 0, 7); got != "" {
		t.Fatalf("inside whitespace: got %q want empty", got)
	}
}

func TestSymbolRejectsAllDigits(t *testing.T) {
	src := "x := 12345"
	if got := Symbol(src, 0, 6); got != "" {
		t.Fatalf("numeric literal: got %q want empty", got)
	}
}

func TestSymbolWithUnderscore(t *testing.T) {
	src := "var _internal_id int"
	if got := Symbol(src, 0, 5); got != "_internal_id" {
		t.Fatalf("got %q want _internal_id", got)
	}
}

func TestSymbolOutOfRange(t *testing.T) {
	src := "abc\n"
	if got := Symbol(src, 5, 0); got != "" {
		t.Fatalf("row out of range: got %q", got)
	}
	if got := Symbol(src, 0, 99); got != "" {
		t.Fatalf("col out of range: got %q", got)
	}
}

func TestSymbolEmpty(t *testing.T) {
	if got := Symbol("", 0, 0); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

func TestBuildFragmentsEmpty(t *testing.T) {
	got := BuildFragments(nil, 3, nil)
	if got != nil {
		t.Fatalf("nil input: got %v want nil", got)
	}
}

func TestBuildFragmentsSingleHit(t *testing.T) {
	file := strings.Join([]string{
		"package main",      // 0
		"",                  // 1
		"func DoThing() {}", // 2
		"",                  // 3
		"func Caller() {",   // 4
		"  DoThing()",       // 5
		"}",                 // 6
	}, "\n")
	reader := func(string) (string, error) { return file, nil }
	locs := []nooklsp.Location{{Path: "/repo/main.go", Line: 5, Col: 2}}
	got := BuildFragments(locs, 2, reader)
	if len(got) != 1 {
		t.Fatalf("got %d fragments want 1", len(got))
	}
	f := got[0]
	if f.Path != "/repo/main.go" {
		t.Fatalf("path %q want /repo/main.go", f.Path)
	}
	if f.StartLine != 4 || f.EndLine != 7 {
		t.Fatalf("range %d-%d want 4-7", f.StartLine, f.EndLine)
	}
	if len(f.Lines) != 4 {
		t.Fatalf("lines len %d want 4", len(f.Lines))
	}
	// The hit row (FileLine == 6 in 1-based) carries marker Added.
	found := false
	for _, l := range f.Lines {
		if l.FileLine == 6 {
			if l.Marker != multibuffer.Added {
				t.Fatalf("hit row marker = %v want Added", l.Marker)
			}
			if !strings.Contains(l.Text, "DoThing()") {
				t.Fatalf("hit row text %q missing DoThing", l.Text)
			}
			found = true
		} else if l.Marker != multibuffer.Context {
			t.Fatalf("non-hit row marker = %v want Context (row %d)", l.Marker, l.FileLine)
		}
	}
	if !found {
		t.Fatalf("did not find hit row at FileLine=6 in fragment lines")
	}
}

func TestBuildFragmentsMergesOverlappingWindows(t *testing.T) {
	// Two hits on lines 5 and 9 (0-based), context=3. Windows are
	// [2,8] and [6,12]; they overlap so the result is one fragment 3..13 (1-based).
	file := strings.Repeat("line\n", 20)
	reader := func(string) (string, error) { return file, nil }
	locs := []nooklsp.Location{
		{Path: "/x/a.go", Line: 5, Col: 0},
		{Path: "/x/a.go", Line: 9, Col: 0},
	}
	got := BuildFragments(locs, 3, reader)
	if len(got) != 1 {
		t.Fatalf("got %d fragments want 1 (merged)", len(got))
	}
	if got[0].StartLine != 3 || got[0].EndLine != 13 {
		t.Fatalf("merged range %d-%d want 3-13", got[0].StartLine, got[0].EndLine)
	}
	// Both hit rows should carry Added.
	addedHits := 0
	for _, l := range got[0].Lines {
		if l.Marker == multibuffer.Added {
			addedHits++
		}
	}
	if addedHits != 2 {
		t.Fatalf("Added markers = %d want 2", addedHits)
	}
}

func TestBuildFragmentsSeparateWindowsWhenFarApart(t *testing.T) {
	// Two hits at lines 1 and 50 with context=2 — they don't overlap.
	file := strings.Repeat("line\n", 60)
	reader := func(string) (string, error) { return file, nil }
	locs := []nooklsp.Location{
		{Path: "/x/a.go", Line: 1, Col: 0},
		{Path: "/x/a.go", Line: 50, Col: 0},
	}
	got := BuildFragments(locs, 2, reader)
	if len(got) != 2 {
		t.Fatalf("got %d fragments want 2 (disjoint)", len(got))
	}
}

func TestBuildFragmentsTouchingWindowsMerge(t *testing.T) {
	// Windows [0,4] and [5,9] touch (gap of 0 lines). They should merge.
	file := strings.Repeat("line\n", 30)
	reader := func(string) (string, error) { return file, nil }
	locs := []nooklsp.Location{
		{Path: "/x/a.go", Line: 2, Col: 0},
		{Path: "/x/a.go", Line: 7, Col: 0},
	}
	got := BuildFragments(locs, 2, reader)
	if len(got) != 1 {
		t.Fatalf("got %d fragments want 1 (touching merge)", len(got))
	}
	if got[0].StartLine != 1 || got[0].EndLine != 10 {
		t.Fatalf("merged range %d-%d want 1-10", got[0].StartLine, got[0].EndLine)
	}
}

func TestBuildFragmentsClampsAgainstFileEnd(t *testing.T) {
	// Hit at line 4 (0-based), context=10, file has only 5 lines (0..4).
	// EndLine must be clamped to 5 (1-based).
	file := "a\nb\nc\nd\ne\n"
	reader := func(string) (string, error) { return file, nil }
	locs := []nooklsp.Location{{Path: "/x/a.go", Line: 4, Col: 0}}
	got := BuildFragments(locs, 10, reader)
	if len(got) != 1 {
		t.Fatalf("got %d fragments want 1", len(got))
	}
	if got[0].EndLine < got[0].StartLine {
		t.Fatalf("inverted range %d-%d", got[0].StartLine, got[0].EndLine)
	}
}

func TestBuildFragmentsGroupsByFile(t *testing.T) {
	files := map[string]string{
		"/x/a.go": strings.Repeat("a\n", 20),
		"/x/b.go": strings.Repeat("b\n", 20),
	}
	reader := func(p string) (string, error) { return files[p], nil }
	locs := []nooklsp.Location{
		{Path: "/x/b.go", Line: 3, Col: 0},
		{Path: "/x/a.go", Line: 2, Col: 0},
		{Path: "/x/a.go", Line: 15, Col: 0},
	}
	got := BuildFragments(locs, 1, reader)
	if len(got) != 3 {
		t.Fatalf("got %d fragments want 3 (a×2 + b×1)", len(got))
	}
	// Sorted by path → a.go comes first.
	if got[0].Path != "/x/a.go" || got[1].Path != "/x/a.go" {
		t.Fatalf("first two paths %q,%q want /x/a.go", got[0].Path, got[1].Path)
	}
	if got[2].Path != "/x/b.go" {
		t.Fatalf("third path %q want /x/b.go", got[2].Path)
	}
}

func TestBuildFragmentsReaderErrorYieldsPlaceholder(t *testing.T) {
	reader := func(string) (string, error) { return "", errors.New("permission denied") }
	locs := []nooklsp.Location{{Path: "/x/a.go", Line: 4, Col: 0}}
	got := BuildFragments(locs, 3, reader)
	if len(got) != 1 {
		t.Fatalf("got %d fragments want 1 placeholder", len(got))
	}
	if got[0].StartLine != 5 || got[0].EndLine != 5 {
		t.Fatalf("placeholder range %d-%d want 5-5", got[0].StartLine, got[0].EndLine)
	}
	if len(got[0].Lines) != 1 || !strings.Contains(got[0].Lines[0].Text, "permission denied") {
		t.Fatalf("placeholder text %q missing error", got[0].Lines[0].Text)
	}
}

func TestBuildFragmentsContextLinesZero(t *testing.T) {
	file := strings.Repeat("line\n", 10)
	reader := func(string) (string, error) { return file, nil }
	locs := []nooklsp.Location{{Path: "/x/a.go", Line: 4, Col: 0}}
	got := BuildFragments(locs, 0, reader)
	if len(got) != 1 {
		t.Fatalf("got %d fragments want 1", len(got))
	}
	if got[0].StartLine != 5 || got[0].EndLine != 5 {
		t.Fatalf("range %d-%d want 5-5 (single-line)", got[0].StartLine, got[0].EndLine)
	}
	if got[0].Lines[0].Marker != multibuffer.Added {
		t.Fatalf("marker %v want Added", got[0].Lines[0].Marker)
	}
}

func TestBuildFragmentsNegativeContextIsZero(t *testing.T) {
	file := strings.Repeat("line\n", 10)
	reader := func(string) (string, error) { return file, nil }
	locs := []nooklsp.Location{{Path: "/x/a.go", Line: 4, Col: 0}}
	got := BuildFragments(locs, -5, reader)
	if got[0].StartLine != 5 || got[0].EndLine != 5 {
		t.Fatalf("negative context behaves like 0: got %d-%d", got[0].StartLine, got[0].EndLine)
	}
}

func TestFindReferencesCmdNilClient(t *testing.T) {
	cmd := FindReferencesCmd(nil, "/x/a.go", 0, 0, 3, nil)
	msg := cmd()
	fm, ok := msg.(multibuffer.FragmentsMsg)
	if !ok {
		t.Fatalf("msg type %T want FragmentsMsg", msg)
	}
	if !errors.Is(fm.Err, ErrNoClient) {
		t.Fatalf("err = %v want ErrNoClient", fm.Err)
	}
	if fm.Source != "references" {
		t.Fatalf("source %q want references", fm.Source)
	}
}

// TestFindReferencesCmdReturnsTeaCmd asserts the factory is a tea.Cmd —
// i.e. callable as a function returning tea.Msg without panicking when
// the underlying client is nil.
func TestFindReferencesCmdReturnsTeaCmd(t *testing.T) {
	var cmd tea.Cmd = FindReferencesCmd(nil, "/x", 0, 0, 0, nil)
	if cmd == nil {
		t.Fatalf("nil cmd")
	}
	if cmd() == nil {
		t.Fatalf("nil msg")
	}
}

func TestOSReaderRoundTripsRealFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/sample.txt"
	want := "hello\nworld\n"
	if err := writeFile(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := OSReader(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestOSReaderMissingFile(t *testing.T) {
	if _, err := OSReader("/does/not/exist/anywhere"); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
