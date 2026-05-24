package symbolsearch

import (
	"errors"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/cmd/nook/internal/multibuffer"
)

func TestBuildFragmentsEmpty(t *testing.T) {
	got := BuildFragments(nil, 3, nil)
	if got != nil {
		t.Fatalf("nil input: got %v want nil", got)
	}
}

func TestBuildFragmentsSingleHit(t *testing.T) {
	file := strings.Join([]string{
		"package main",     // 0
		"",                 // 1
		"// thing",         // 2
		"func DoThing() {", // 3
		"  return 42",      // 4
		"}",                // 5
		"",                 // 6
		"func Caller() {",  // 7
		"  DoThing()",      // 8
		"}",                // 9
	}, "\n")
	reader := func(string) (string, error) { return file, nil }
	syms := []nooklsp.WorkspaceSymbol{{
		Name: "DoThing",
		Kind: nooklsp.WorkspaceSymbolKindFunction,
		Path: "/repo/main.go",
		Line: 3,
		Col:  5,
	}}
	got := BuildFragments(syms, 2, reader)
	if len(got) != 1 {
		t.Fatalf("got %d fragments want 1", len(got))
	}
	f := got[0]
	if f.Path != "/repo/main.go" {
		t.Fatalf("path %q want /repo/main.go", f.Path)
	}
	if f.StartLine != 2 || f.EndLine != 6 {
		t.Fatalf("range %d-%d want 2-6", f.StartLine, f.EndLine)
	}
	if f.Suffix != "func DoThing" {
		t.Fatalf("suffix %q want %q", f.Suffix, "func DoThing")
	}
	if len(f.Lines) != 5 {
		t.Fatalf("lines len %d want 5", len(f.Lines))
	}
	// The hit row (FileLine == 4 in 1-based) carries marker Added.
	found := false
	for _, l := range f.Lines {
		if l.FileLine == 4 {
			if l.Marker != multibuffer.Added {
				t.Fatalf("hit row marker = %v want Added", l.Marker)
			}
			if !strings.Contains(l.Text, "DoThing") {
				t.Fatalf("hit row text %q missing DoThing", l.Text)
			}
			found = true
		} else if l.Marker != multibuffer.Context {
			t.Fatalf("non-hit row marker = %v want Context (row %d)", l.Marker, l.FileLine)
		}
	}
	if !found {
		t.Fatal("hit row not found in fragment lines")
	}
}

func TestBuildFragmentsContainerSuffix(t *testing.T) {
	file := "package main\ntype User struct { Name string }\nfunc (u User) Hello() {}\n"
	reader := func(string) (string, error) { return file, nil }
	syms := []nooklsp.WorkspaceSymbol{{
		Name:      "Hello",
		Container: "User",
		Kind:      nooklsp.WorkspaceSymbolKindMethod,
		Path:      "/repo/u.go",
		Line:      2,
		Col:       14,
	}}
	got := BuildFragments(syms, 1, reader)
	if len(got) != 1 {
		t.Fatalf("got %d fragments want 1", len(got))
	}
	if got[0].Suffix != "method User.Hello" {
		t.Fatalf("suffix %q want %q", got[0].Suffix, "method User.Hello")
	}
}

func TestBuildFragmentsMergesAdjacentHits(t *testing.T) {
	// Two symbols on lines 5 and 7 with contextLines=2 should produce
	// one fragment 3..9 (windows 3..7 and 5..9 overlap).
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = "// row " + string(rune('0'+i))
	}
	file := strings.Join(lines, "\n")
	reader := func(string) (string, error) { return file, nil }
	syms := []nooklsp.WorkspaceSymbol{
		{Name: "Alpha", Kind: nooklsp.WorkspaceSymbolKindFunction, Path: "/repo/m.go", Line: 5},
		{Name: "Beta", Kind: nooklsp.WorkspaceSymbolKindFunction, Path: "/repo/m.go", Line: 7},
	}
	got := BuildFragments(syms, 2, reader)
	if len(got) != 1 {
		t.Fatalf("got %d fragments want 1 (merged)", len(got))
	}
	if got[0].StartLine != 4 || got[0].EndLine != 10 {
		t.Fatalf("merged range %d-%d want 4-10", got[0].StartLine, got[0].EndLine)
	}
	addedRows := []int{}
	for _, l := range got[0].Lines {
		if l.Marker == multibuffer.Added {
			addedRows = append(addedRows, l.FileLine)
		}
	}
	if len(addedRows) != 2 || addedRows[0] != 6 || addedRows[1] != 8 {
		t.Fatalf("added rows %v want [6 8]", addedRows)
	}
	if got[0].Suffix != "func Alpha" {
		t.Fatalf("suffix %q want %q (first hit in line order)", got[0].Suffix, "func Alpha")
	}
}

func TestBuildFragmentsMultipleFiles(t *testing.T) {
	// One symbol per file in two different files. Result must group
	// per-file with two fragments sorted by path.
	files := map[string]string{
		"/repo/a.go": "package a\nfunc One() {}\n",
		"/repo/b.go": "package b\nfunc Two() {}\n",
	}
	reader := func(path string) (string, error) { return files[path], nil }
	syms := []nooklsp.WorkspaceSymbol{
		{Name: "Two", Kind: nooklsp.WorkspaceSymbolKindFunction, Path: "/repo/b.go", Line: 1},
		{Name: "One", Kind: nooklsp.WorkspaceSymbolKindFunction, Path: "/repo/a.go", Line: 1},
	}
	got := BuildFragments(syms, 1, reader)
	if len(got) != 2 {
		t.Fatalf("got %d fragments want 2", len(got))
	}
	if got[0].Path != "/repo/a.go" {
		t.Fatalf("first fragment path %q want /repo/a.go (sorted)", got[0].Path)
	}
	if got[1].Path != "/repo/b.go" {
		t.Fatalf("second fragment path %q want /repo/b.go", got[1].Path)
	}
}

func TestBuildFragmentsReaderErrorProducesPlaceholder(t *testing.T) {
	reader := func(string) (string, error) { return "", os.ErrNotExist }
	syms := []nooklsp.WorkspaceSymbol{{
		Name: "Gone",
		Kind: nooklsp.WorkspaceSymbolKindFunction,
		Path: "/missing/x.go",
		Line: 10,
	}}
	got := BuildFragments(syms, 3, reader)
	if len(got) != 1 {
		t.Fatalf("got %d fragments want 1", len(got))
	}
	if !strings.Contains(got[0].Lines[0].Text, "<unreadable") {
		t.Fatalf("expected unreadable placeholder, got %q", got[0].Lines[0].Text)
	}
}

func TestBuildFragmentsClampsAtFileBounds(t *testing.T) {
	// A hit on the last line with contextLines=5 must clamp to the
	// last valid line and not emit phantom rows.
	file := "alpha\nbeta\ngamma\n"
	reader := func(string) (string, error) { return file, nil }
	syms := []nooklsp.WorkspaceSymbol{{
		Name: "Gamma",
		Path: "/x.go",
		Line: 2, // 0-indexed → "gamma"
	}}
	got := BuildFragments(syms, 5, reader)
	if len(got) != 1 {
		t.Fatalf("got %d fragments want 1", len(got))
	}
	for _, l := range got[0].Lines {
		if l.FileLine < 1 {
			t.Fatalf("line FileLine %d < 1", l.FileLine)
		}
	}
}

func TestSuffixForEmptyKind(t *testing.T) {
	s := nooklsp.WorkspaceSymbol{Name: "Foo"}
	if got := suffixFor(s); got != "Foo" {
		t.Fatalf("suffix %q want Foo", got)
	}
}

func TestSuffixForContainerOnly(t *testing.T) {
	s := nooklsp.WorkspaceSymbol{Name: "Bar", Container: "Foo", Kind: nooklsp.WorkspaceSymbolKindField}
	if got := suffixFor(s); got != "field Foo.Bar" {
		t.Fatalf("suffix %q want %q", got, "field Foo.Bar")
	}
}

func TestFindSymbolsCmdNilClient(t *testing.T) {
	cmd := FindSymbolsCmd(nil, "anything", 3, nil)
	msg := cmd().(multibuffer.FragmentsMsg)
	if msg.Source != "symbols" {
		t.Fatalf("source %q want symbols", msg.Source)
	}
	if !errors.Is(msg.Err, ErrNoClient) {
		t.Fatalf("err = %v want ErrNoClient", msg.Err)
	}
	if msg.Fragments != nil {
		t.Fatalf("fragments = %v want nil", msg.Fragments)
	}
}

func TestFindSymbolsCmdEmptyQuery(t *testing.T) {
	// Construct a client placeholder so the nil-client branch doesn't
	// fire. The empty-query branch should short-circuit before any LSP
	// call, so we can pass a non-nil client without a real server.
	cmd := FindSymbolsCmd(&nooklsp.Client{}, "   ", 3, nil)
	msg := cmd().(multibuffer.FragmentsMsg)
	if !errors.Is(msg.Err, ErrEmptyQuery) {
		t.Fatalf("err = %v want ErrEmptyQuery", msg.Err)
	}
}

func TestFindSymbolsCmdReturnsTeaCmd(t *testing.T) {
	// Type assertion sanity-check — the factory must return a tea.Cmd.
	var cmd tea.Cmd = FindSymbolsCmd(nil, "x", 0, nil)
	if cmd == nil {
		t.Fatal("FindSymbolsCmd returned nil")
	}
}
