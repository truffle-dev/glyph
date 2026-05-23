package search

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func requireRg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not on PATH; skipping integration test")
	}
}

// writeTree creates a small fixture directory and returns its root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return root
}

func TestRunFindsMatches(t *testing.T) {
	requireRg(t)
	root := writeTree(t, map[string]string{
		"a.go":         "package a\n\nfunc foo() {}\n",
		"sub/b.go":     "package b\n\nfunc Foo() string { return \"x\" }\n",
		"sub/c.txt":    "no match here\n",
		"sub/deep/d.go": "// foo at module top\nvar foo = 1\n",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, done := Run(ctx, root, "foo")
	var matches []Match
	for m := range out {
		matches = append(matches, m)
	}
	if err := <-done; err != nil {
		t.Fatalf("rg err: %v", err)
	}
	if len(matches) < 3 {
		t.Fatalf("expected >=3 matches, got %d: %+v", len(matches), matches)
	}
	// smart-case: "foo" should match both foo and Foo
	hasUpper, hasLower := false, false
	for _, m := range matches {
		if strings.Contains(m.Snippet, "Foo") {
			hasUpper = true
		}
		if strings.Contains(m.Snippet, "foo") {
			hasLower = true
		}
	}
	if !hasUpper || !hasLower {
		t.Fatalf("expected smart-case to hit Foo and foo, got upper=%v lower=%v", hasUpper, hasLower)
	}
}

func TestRunEmptyQuery(t *testing.T) {
	out, done := Run(context.Background(), "/tmp", "")
	for range out {
		t.Fatal("expected no matches for empty query")
	}
	if err := <-done; err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
}

func TestRunNoMatches(t *testing.T) {
	requireRg(t)
	root := writeTree(t, map[string]string{
		"a.txt": "alpha\n",
		"b.txt": "beta\n",
	})
	out, done := Run(context.Background(), root, "zzzzzznomatch")
	for range out {
		t.Fatal("expected no matches")
	}
	if err := <-done; err != nil {
		t.Fatalf("expected nil err on no matches (rg exit 1 is success), got %v", err)
	}
}

func TestRunCancellation(t *testing.T) {
	requireRg(t)
	root := writeTree(t, map[string]string{
		"big.txt": strings.Repeat("hello world\n", 10000),
	})
	ctx, cancel := context.WithCancel(context.Background())
	out, done := Run(ctx, root, "hello")
	// read one and cancel
	<-out
	cancel()
	for range out {
		// drain
	}
	<-done
}

func TestPaneAppendAndSelect(t *testing.T) {
	p := NewPane(theme.Default, "/tmp")
	p = p.Reset("foo")
	p = p.AppendMatch(Match{Path: "/tmp/a.go", Line: 1, Col: 6, Snippet: "func foo() {}"})
	p = p.AppendMatch(Match{Path: "/tmp/b.go", Line: 3, Col: 6, Snippet: "var foo = 1"})
	if len(p.Matches()) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(p.Matches()))
	}
	sel, ok := p.Selected()
	if !ok || sel.Path != "/tmp/a.go" {
		t.Fatalf("expected /tmp/a.go selected, got ok=%v sel=%+v", ok, sel)
	}
}

func TestPaneEnterEmitsOpen(t *testing.T) {
	p := NewPane(theme.Default, "/tmp").WithSize(80, 20)
	p = p.Reset("foo")
	p = p.AppendMatch(Match{Path: "/tmp/a.go", Line: 5, Col: 2, Snippet: "foo here"})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected Cmd on Enter")
	}
	msg := cmd()
	open, ok := msg.(OpenMsg)
	if !ok {
		t.Fatalf("expected OpenMsg, got %T", msg)
	}
	if open.Path != "/tmp/a.go" || open.Line != 5 || open.Col != 2 {
		t.Fatalf("OpenMsg mismatch: %+v", open)
	}
}

func TestPaneEscEmitsCancel(t *testing.T) {
	p := NewPane(theme.Default, "/tmp")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected Cmd on Esc")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd())
	}
}

func TestPaneCursorMoves(t *testing.T) {
	p := NewPane(theme.Default, "/tmp").WithSize(80, 20)
	p = p.Reset("foo")
	for i := 0; i < 3; i++ {
		p = p.AppendMatch(Match{Path: "/tmp/x.go", Line: i + 1, Col: 1, Snippet: "foo"})
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	sel, _ := p.Selected()
	if sel.Line != 3 {
		t.Fatalf("expected line 3 after two Downs, got %d", sel.Line)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	sel, _ = p.Selected()
	if sel.Line != 2 {
		t.Fatalf("expected line 2 after Up, got %d", sel.Line)
	}
}

func TestPaneViewRenders(t *testing.T) {
	p := NewPane(theme.Default, "/tmp").WithSize(80, 20)
	p = p.Reset("foo")
	p = p.AppendMatch(Match{Path: "/tmp/a.go", Line: 1, Col: 1, Snippet: "func foo()"})
	p = p.MarkDone(nil)
	out := p.View()
	if out == "" {
		t.Fatal("expected non-empty View()")
	}
	if !strings.Contains(out, "search") {
		t.Fatalf("expected 'search' in output:\n%s", out)
	}
	if !strings.Contains(out, "foo") {
		t.Fatalf("expected query 'foo' in header:\n%s", out)
	}
}

func TestRelativize(t *testing.T) {
	if got := relativize("/repo/a/b.go", "/repo"); got != "a/b.go" {
		t.Fatalf("expected 'a/b.go', got %q", got)
	}
	if got := relativize("/elsewhere/x.go", "/repo"); got != "/elsewhere/x.go" {
		t.Fatalf("expected absolute path passthrough, got %q", got)
	}
	if got := relativize("/p/x.go", ""); got != "/p/x.go" {
		t.Fatalf("expected passthrough on empty root, got %q", got)
	}
}

func TestFormatStatus(t *testing.T) {
	p := NewPane(theme.Default, "/tmp")
	p = p.Reset("foo")
	if !strings.Contains(formatStatus(p), "searching") {
		t.Fatalf("expected searching status, got %q", formatStatus(p))
	}
	p = p.MarkDone(nil)
	if !strings.Contains(formatStatus(p), "matches") {
		t.Fatalf("expected matches status, got %q", formatStatus(p))
	}
}
