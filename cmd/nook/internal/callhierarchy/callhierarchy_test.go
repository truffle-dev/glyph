package callhierarchy

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/cmd/nook/internal/multibuffer"
)

func TestDirectionLabels(t *testing.T) {
	if got := Incoming.Label(); got != "incoming calls" {
		t.Fatalf("Incoming.Label = %q want incoming calls", got)
	}
	if got := Outgoing.Label(); got != "outgoing calls" {
		t.Fatalf("Outgoing.Label = %q want outgoing calls", got)
	}
	if got := Incoming.Source(); got != "callhierarchy.incoming" {
		t.Fatalf("Incoming.Source = %q", got)
	}
	if got := Outgoing.Source(); got != "callhierarchy.outgoing" {
		t.Fatalf("Outgoing.Source = %q", got)
	}
}

func TestSymbolBasic(t *testing.T) {
	src := "package main\nfunc DoThing(x int) {}\n"
	if got := Symbol(src, 1, 7); got != "DoThing" {
		t.Fatalf("got %q want DoThing", got)
	}
}

func TestSymbolAtIdentEnd(t *testing.T) {
	src := "var Counter int"
	if got := Symbol(src, 0, 11); got != "Counter" {
		t.Fatalf("at end: got %q", got)
	}
}

func TestSymbolNotOnIdentifier(t *testing.T) {
	src := "x = 1 +  2"
	if got := Symbol(src, 0, 7); got != "" {
		t.Fatalf("whitespace: got %q", got)
	}
}

func TestSymbolRejectsAllDigits(t *testing.T) {
	src := "x := 12345"
	if got := Symbol(src, 0, 6); got != "" {
		t.Fatalf("numeric: got %q", got)
	}
}

func TestSymbolOutOfRange(t *testing.T) {
	if got := Symbol("abc\n", 5, 0); got != "" {
		t.Fatalf("row out of range: got %q", got)
	}
	if got := Symbol("abc\n", 0, 99); got != "" {
		t.Fatalf("col out of range: got %q", got)
	}
}

func TestBuildFragmentsEmpty(t *testing.T) {
	if got := BuildFragments(nil, Incoming, "x.go", 3, nil); got != nil {
		t.Fatalf("nil input: got %v", got)
	}
}

func TestBuildFragmentsIncomingSingleCall(t *testing.T) {
	src := strings.Join([]string{
		"package main",       // 0
		"",                   // 1
		"func Caller() {",    // 2
		"\tprintln(\"hi\")",  // 3
		"\tTarget()",         // 4 hit
		"\tprintln(\"bye\")", // 5
		"}",                  // 6
	}, "\n")
	calls := []nooklsp.CallHierarchyCall{{
		Item: nooklsp.CallHierarchyItem{
			Name:         "Caller",
			Detail:       "func()",
			Path:         "/repo/caller.go",
			SelStartLine: 2,
		},
		FromRanges: []nooklsp.Range{{StartLine: 4, StartCol: 1, EndLine: 4, EndCol: 7}},
	}}
	reader := mapReader(map[string]string{"/repo/caller.go": src})
	frags := BuildFragments(calls, Incoming, "/repo/target.go", 1, reader)
	if len(frags) != 1 {
		t.Fatalf("len=%d want 1", len(frags))
	}
	f := frags[0]
	if f.Path != "/repo/caller.go" {
		t.Fatalf("Path=%q", f.Path)
	}
	if f.StartLine != 4 || f.EndLine != 6 {
		t.Fatalf("range %d..%d want 4..6", f.StartLine, f.EndLine)
	}
	if f.Suffix != "Caller — func()" {
		t.Fatalf("Suffix=%q", f.Suffix)
	}
	if len(f.Lines) != 3 {
		t.Fatalf("lines=%d want 3", len(f.Lines))
	}
	if f.Lines[1].Marker != multibuffer.Added {
		t.Fatalf("hit row marker=%v want Added", f.Lines[1].Marker)
	}
	if f.Lines[1].FileLine != 5 {
		t.Fatalf("hit FileLine=%d want 5", f.Lines[1].FileLine)
	}
	if f.Lines[0].Marker != multibuffer.Context || f.Lines[2].Marker != multibuffer.Context {
		t.Fatalf("context rows wrong markers")
	}
}

func TestBuildFragmentsOutgoingUsesSourcePath(t *testing.T) {
	src := strings.Join([]string{
		"package main",   // 0
		"",               // 1
		"func Source(){", // 2
		"\tFoo()",        // 3 hit
		"\tBar()",        // 4 hit (different callee)
		"}",              // 5
	}, "\n")
	calls := []nooklsp.CallHierarchyCall{
		{
			Item:       nooklsp.CallHierarchyItem{Name: "Foo", Path: "/elsewhere/foo.go", SelStartLine: 0},
			FromRanges: []nooklsp.Range{{StartLine: 3, EndLine: 3}},
		},
		{
			Item:       nooklsp.CallHierarchyItem{Name: "Bar", Path: "/elsewhere/bar.go", SelStartLine: 0},
			FromRanges: []nooklsp.Range{{StartLine: 4, EndLine: 4}},
		},
	}
	reader := mapReader(map[string]string{"/repo/src.go": src})
	frags := BuildFragments(calls, Outgoing, "/repo/src.go", 0, reader)
	if len(frags) != 2 {
		t.Fatalf("len=%d want 2", len(frags))
	}
	// Sort is (path, firstLine). Both share path so Foo at line 3 < Bar at line 4.
	if frags[0].Suffix != "Foo" {
		t.Fatalf("first suffix=%q want Foo", frags[0].Suffix)
	}
	if frags[1].Suffix != "Bar" {
		t.Fatalf("second suffix=%q want Bar", frags[1].Suffix)
	}
	for _, f := range frags {
		if f.Path != "/repo/src.go" {
			t.Fatalf("outgoing fragment path=%q want sourcePath", f.Path)
		}
	}
}

func TestBuildFragmentsMergesAdjacentRanges(t *testing.T) {
	src := strings.Join([]string{
		"a", // 0
		"b", // 1
		"c", // 2 hit
		"d", // 3 hit
		"e", // 4
	}, "\n")
	calls := []nooklsp.CallHierarchyCall{{
		Item: nooklsp.CallHierarchyItem{Name: "Caller", Path: "/p.go", SelStartLine: 0},
		FromRanges: []nooklsp.Range{
			{StartLine: 2, EndLine: 2},
			{StartLine: 3, EndLine: 3},
		},
	}}
	reader := mapReader(map[string]string{"/p.go": src})
	frags := BuildFragments(calls, Incoming, "/p.go", 0, reader)
	if len(frags) != 1 {
		t.Fatalf("len=%d want 1 (merged)", len(frags))
	}
	f := frags[0]
	if f.StartLine != 3 || f.EndLine != 4 {
		t.Fatalf("range %d..%d want 3..4", f.StartLine, f.EndLine)
	}
	for _, ln := range f.Lines {
		if ln.Marker != multibuffer.Added {
			t.Fatalf("expected all hit rows; got marker=%v at line %d", ln.Marker, ln.FileLine)
		}
	}
}

func TestBuildFragmentsCollapseDisjointWindows(t *testing.T) {
	// Two hits far apart in the same file collapse into a single fragment
	// with Context for the gap (multibuffer.Fragment carries one span).
	src := ""
	for i := 0; i < 20; i++ {
		src += "line\n"
	}
	src = strings.TrimRight(src, "\n")
	calls := []nooklsp.CallHierarchyCall{{
		Item: nooklsp.CallHierarchyItem{Name: "C", Path: "/p.go", SelStartLine: 0},
		FromRanges: []nooklsp.Range{
			{StartLine: 2, EndLine: 2},
			{StartLine: 15, EndLine: 15},
		},
	}}
	reader := mapReader(map[string]string{"/p.go": src})
	frags := BuildFragments(calls, Incoming, "/p.go", 1, reader)
	if len(frags) != 1 {
		t.Fatalf("len=%d want 1", len(frags))
	}
	f := frags[0]
	if f.StartLine != 2 || f.EndLine != 17 {
		t.Fatalf("collapsed range %d..%d", f.StartLine, f.EndLine)
	}
	// Hits remain Added; in-between lines are Context.
	hits := 0
	for _, ln := range f.Lines {
		if ln.Marker == multibuffer.Added {
			hits++
		}
	}
	if hits != 2 {
		t.Fatalf("hits=%d want 2", hits)
	}
}

func TestBuildFragmentsUnreadableFile(t *testing.T) {
	calls := []nooklsp.CallHierarchyCall{{
		Item:       nooklsp.CallHierarchyItem{Name: "C", Path: "/missing.go", SelStartLine: 0},
		FromRanges: []nooklsp.Range{{StartLine: 1, EndLine: 1}},
	}}
	reader := func(string) (string, error) { return "", errors.New("nope") }
	frags := BuildFragments(calls, Incoming, "/missing.go", 1, reader)
	if len(frags) != 1 {
		t.Fatalf("len=%d want 1", len(frags))
	}
	f := frags[0]
	if !strings.Contains(f.Lines[0].Text, "<unreadable:") {
		t.Fatalf("expected placeholder text, got %q", f.Lines[0].Text)
	}
	if f.Suffix != "C" {
		t.Fatalf("Suffix=%q", f.Suffix)
	}
}

func TestBuildFragmentsPlaceholderWhenNoRanges(t *testing.T) {
	calls := []nooklsp.CallHierarchyCall{{
		Item: nooklsp.CallHierarchyItem{Name: "Empty", Path: "/p.go", SelStartLine: 7},
	}}
	frags := BuildFragments(calls, Incoming, "/p.go", 1, nil)
	if len(frags) != 1 {
		t.Fatalf("len=%d want 1", len(frags))
	}
	f := frags[0]
	if f.StartLine != 8 || f.EndLine != 8 {
		t.Fatalf("placeholder range %d..%d want 8..8", f.StartLine, f.EndLine)
	}
	if f.Lines[0].Text != "(no ranges reported)" {
		t.Fatalf("placeholder text=%q", f.Lines[0].Text)
	}
}

func TestBuildFragmentsStableSortByPathThenLine(t *testing.T) {
	src := "a\nb\nc\nd\ne\nf\n"
	calls := []nooklsp.CallHierarchyCall{
		{
			Item:       nooklsp.CallHierarchyItem{Name: "Beta", Path: "/b.go"},
			FromRanges: []nooklsp.Range{{StartLine: 1, EndLine: 1}},
		},
		{
			Item:       nooklsp.CallHierarchyItem{Name: "Alpha2", Path: "/a.go"},
			FromRanges: []nooklsp.Range{{StartLine: 4, EndLine: 4}},
		},
		{
			Item:       nooklsp.CallHierarchyItem{Name: "Alpha1", Path: "/a.go"},
			FromRanges: []nooklsp.Range{{StartLine: 0, EndLine: 0}},
		},
	}
	reader := mapReader(map[string]string{"/a.go": src, "/b.go": src})
	frags := BuildFragments(calls, Incoming, "/src.go", 0, reader)
	if len(frags) != 3 {
		t.Fatalf("len=%d", len(frags))
	}
	if frags[0].Suffix != "Alpha1" || frags[1].Suffix != "Alpha2" || frags[2].Suffix != "Beta" {
		t.Fatalf("order: %q %q %q", frags[0].Suffix, frags[1].Suffix, frags[2].Suffix)
	}
}

func TestCallHierarchyCmdNilClient(t *testing.T) {
	cmd := CallHierarchyCmd(nil, "/x.go", 0, 0, Incoming, 3, nil)
	msg := cmd().(multibuffer.FragmentsMsg)
	if !errors.Is(msg.Err, ErrNoClient) {
		t.Fatalf("err=%v want ErrNoClient", msg.Err)
	}
	if msg.Source != "callhierarchy.incoming" {
		t.Fatalf("Source=%q", msg.Source)
	}
}

func TestCallHierarchyCmdNilClientOutgoing(t *testing.T) {
	cmd := CallHierarchyCmd(nil, "/x.go", 0, 0, Outgoing, 3, nil)
	msg := cmd().(multibuffer.FragmentsMsg)
	if !errors.Is(msg.Err, ErrNoClient) {
		t.Fatalf("err=%v want ErrNoClient", msg.Err)
	}
	if msg.Source != "callhierarchy.outgoing" {
		t.Fatalf("Source=%q", msg.Source)
	}
}

func TestSuffixIncludesDetail(t *testing.T) {
	call := nooklsp.CallHierarchyCall{
		Item: nooklsp.CallHierarchyItem{Name: "Foo", Detail: "func() error"},
	}
	if got := callSuffix(call); got != "Foo — func() error" {
		t.Fatalf("suffix=%q", got)
	}
}

func TestSuffixWithoutDetail(t *testing.T) {
	call := nooklsp.CallHierarchyCall{
		Item: nooklsp.CallHierarchyItem{Name: "Foo"},
	}
	if got := callSuffix(call); got != "Foo" {
		t.Fatalf("suffix=%q", got)
	}
}

// mapReader returns a Reader that resolves a small in-memory file map; an
// unknown path returns os.ErrNotExist-shaped error.
func mapReader(m map[string]string) Reader {
	return func(p string) (string, error) {
		if v, ok := m[p]; ok {
			return v, nil
		}
		return "", errors.New("no such file in map: " + p)
	}
}

// Compile-time assertion that the package builds in a tea.Cmd context.
var _ tea.Cmd = CallHierarchyCmd(nil, "", 0, 0, Incoming, 0, nil)
