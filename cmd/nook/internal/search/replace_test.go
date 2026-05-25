package search

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writeFile is a small fixture helper that drops a file into a temp dir
// and returns its absolute path. Used by every replace test below.
func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// Empty matches is a no-op and never touches the disk.
func TestApplyAll_EmptyMatches(t *testing.T) {
	res, err := ApplyAll(nil, "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FilesChanged != 0 || res.ReplacementsApplied != 0 || len(res.PathsTouched) != 0 {
		t.Fatalf("expected zero result, got %+v", res)
	}
}

// Single match in a single file rewrites the recorded span exactly.
func TestApplyAll_SingleMatch(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.go", "package foo\n")
	matches := []Match{{Path: p, Line: 1, Col: 9, Len: 3, Snippet: "package foo"}}

	res, err := ApplyAll(matches, "bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FilesChanged != 1 || res.ReplacementsApplied != 1 {
		t.Fatalf("expected 1/1, got %+v", res)
	}
	if got := readFile(t, p); got != "package bar\n" {
		t.Fatalf("file contents = %q, want %q", got, "package bar\n")
	}
}

// Multiple matches on the same line apply right-to-left so the recorded
// byte columns stay valid even when the replacement changes line length.
func TestApplyAll_MultipleSameLine(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.go", "foo bar foo\n")
	matches := []Match{
		{Path: p, Line: 1, Col: 1, Len: 3, Snippet: "foo bar foo"},
		{Path: p, Line: 1, Col: 9, Len: 3, Snippet: "foo bar foo"},
	}

	res, err := ApplyAll(matches, "longer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ReplacementsApplied != 2 {
		t.Fatalf("expected 2 replacements, got %d", res.ReplacementsApplied)
	}
	if got := readFile(t, p); got != "longer bar longer\n" {
		t.Fatalf("file contents = %q, want %q", got, "longer bar longer\n")
	}
}

// A replacement that ALSO contains the search term must not feed back
// into a second pass on the same line. Right-to-left iteration plus
// single-pass guarantees this; the test pins it.
func TestApplyAll_ReplacementContainsQuery(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.go", "foo\n")
	matches := []Match{{Path: p, Line: 1, Col: 1, Len: 3, Snippet: "foo"}}

	res, err := ApplyAll(matches, "foofoo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ReplacementsApplied != 1 {
		t.Fatalf("expected 1 replacement, got %d", res.ReplacementsApplied)
	}
	if got := readFile(t, p); got != "foofoo\n" {
		t.Fatalf("file contents = %q, want %q", got, "foofoo\n")
	}
}

// Matches across multiple files all apply; PathsTouched reflects each
// file once and the count matches the file count.
func TestApplyAll_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.go", "alpha\n")
	b := writeFile(t, dir, "b.go", "alpha alpha\n")
	matches := []Match{
		{Path: a, Line: 1, Col: 1, Len: 5},
		{Path: b, Line: 1, Col: 1, Len: 5},
		{Path: b, Line: 1, Col: 7, Len: 5},
	}

	res, err := ApplyAll(matches, "beta")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FilesChanged != 2 || res.ReplacementsApplied != 3 {
		t.Fatalf("expected 2/3, got %+v", res)
	}
	if readFile(t, a) != "beta\n" {
		t.Fatalf("a.go = %q", readFile(t, a))
	}
	if readFile(t, b) != "beta beta\n" {
		t.Fatalf("b.go = %q", readFile(t, b))
	}

	// PathsTouched ordering follows match insertion order, but the
	// test asserts membership not order for robustness.
	got := append([]string(nil), res.PathsTouched...)
	sort.Strings(got)
	want := []string{a, b}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("PathsTouched = %v, want %v", got, want)
	}
}

// Bytes outside the matched span are preserved verbatim, including UTF-8
// runes adjacent to the replacement window.
func TestApplyAll_PreservesBytesOutsideMatches(t *testing.T) {
	dir := t.TempDir()
	// 日本語 'foo' 日本語 — the multibyte runes are 3 bytes each.
	original := "日本語 foo 日本語\n"
	p := writeFile(t, dir, "a.txt", original)
	// 日本語 = 9 bytes; space = 1; foo starts at byte index 10 (Col 11).
	matches := []Match{{Path: p, Line: 1, Col: 11, Len: 3}}

	res, err := ApplyAll(matches, "bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ReplacementsApplied != 1 {
		t.Fatalf("expected 1 replacement, got %d", res.ReplacementsApplied)
	}
	want := "日本語 bar 日本語\n"
	if got := readFile(t, p); got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

// Multi-line file: a hit on line 3 leaves lines 1, 2, 4 untouched and
// preserves the trailing newline.
func TestApplyAll_MultiLineUntouchedRowsPreserved(t *testing.T) {
	dir := t.TempDir()
	original := "line one\nline two\nline three\nline four\n"
	p := writeFile(t, dir, "a.txt", original)
	// "three" starts at col 6 of line 3 (1-based).
	matches := []Match{{Path: p, Line: 3, Col: 6, Len: 5}}

	res, err := ApplyAll(matches, "tres")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ReplacementsApplied != 1 {
		t.Fatalf("expected 1 replacement, got %d", res.ReplacementsApplied)
	}
	want := "line one\nline two\nline tres\nline four\n"
	if got := readFile(t, p); got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

// File with no trailing newline must come back without one — preserving
// the file's original byte-exact terminator state matters for files like
// JSON that may legitimately have no trailing newline.
func TestApplyAll_NoTrailingNewlinePreserved(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.txt", "foo bar") // no \n
	matches := []Match{{Path: p, Line: 1, Col: 5, Len: 3}}

	if _, err := ApplyAll(matches, "baz"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := readFile(t, p); got != "foo baz" {
		t.Fatalf("file = %q, want %q (no trailing newline)", got, "foo baz")
	}
}

// Out-of-range hits (line 999 in a 3-line file, or negative Len) are
// silently skipped so a stale Match list from before a competing edit
// doesn't corrupt the file or panic.
func TestApplyAll_SkipsCorruptHits(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.txt", "one\ntwo\nthree\n")
	matches := []Match{
		{Path: p, Line: 999, Col: 1, Len: 3},                      // out-of-range line
		{Path: p, Line: 1, Col: 1, Len: 0},                        // zero Len
		{Path: p, Line: 1, Col: 1, Len: -5},                       // negative Len
		{Path: p, Line: 2, Col: 1, Len: 3, Snippet: "two"},        // valid
		{Path: p, Line: 3, Col: 100, Len: 5, Snippet: "OOR span"}, // span past row end
	}

	res, err := ApplyAll(matches, "X")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ReplacementsApplied != 1 {
		t.Fatalf("expected 1 replacement, got %d", res.ReplacementsApplied)
	}
	if got := readFile(t, p); got != "one\nX\nthree\n" {
		t.Fatalf("file = %q, want %q", got, "one\nX\nthree\n")
	}
}

// Replacement to empty string deletes the matched span. Common UX:
// "find foo, replace with nothing" should produce the same file with
// every "foo" gone.
func TestApplyAll_EmptyReplacementDeletesSpan(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.txt", "foo bar foo baz\n")
	matches := []Match{
		{Path: p, Line: 1, Col: 1, Len: 3},
		{Path: p, Line: 1, Col: 9, Len: 3},
	}

	res, err := ApplyAll(matches, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ReplacementsApplied != 2 {
		t.Fatalf("expected 2 replacements, got %d", res.ReplacementsApplied)
	}
	if got := readFile(t, p); got != " bar  baz\n" {
		t.Fatalf("file = %q, want %q", got, " bar  baz\n")
	}
}

// A path that doesn't exist returns the wrapped read error and reports
// zero replacements; subsequent paths are not attempted (partial-apply
// semantics documented on the function).
func TestApplyAll_MissingFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.txt")
	matches := []Match{{Path: missing, Line: 1, Col: 1, Len: 3}}

	res, err := ApplyAll(matches, "X")
	if err == nil {
		t.Fatalf("expected read error, got nil")
	}
	if !strings.Contains(err.Error(), "read") {
		t.Fatalf("error %v does not mention 'read'", err)
	}
	if res.FilesChanged != 0 || res.ReplacementsApplied != 0 {
		t.Fatalf("expected zero result on error, got %+v", res)
	}
}
