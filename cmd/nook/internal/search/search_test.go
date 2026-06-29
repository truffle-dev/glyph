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
		"a.go":          "package a\n\nfunc foo() {}\n",
		"sub/b.go":      "package b\n\nfunc Foo() string { return \"x\" }\n",
		"sub/c.txt":     "no match here\n",
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

// EnterReplace no-ops while no matches are present — the host should
// silently ignore the Alt+R key chord rather than flip into a useless
// "replace with nothing" mode.
func TestPaneEnterReplaceNoOpWhenEmpty(t *testing.T) {
	p := NewPane(theme.Default, "/tmp")
	p = p.EnterReplace()
	if p.Replacing() {
		t.Fatal("expected EnterReplace to be a no-op with no matches")
	}
}

// EnterReplace also no-ops while ripgrep is still streaming results.
// Replacing a partial set is a foot-gun.
func TestPaneEnterReplaceNoOpWhileRunning(t *testing.T) {
	p := NewPane(theme.Default, "/tmp").Reset("foo")
	p = p.AppendMatch(Match{Path: "/tmp/a.go", Line: 1, Col: 1, Len: 3, Snippet: "foo"})
	if !p.running {
		t.Fatal("expected pane to be running after Reset")
	}
	p = p.EnterReplace()
	if p.Replacing() {
		t.Fatal("expected EnterReplace to be a no-op while still running")
	}
}

// With matches and done, EnterReplace flips replacing=true; ExitReplace
// flips it back without throwing away the typed replacement.
func TestPaneEnterReplaceFlipsAndPreservesText(t *testing.T) {
	p := NewPane(theme.Default, "/tmp").WithSize(80, 20).Reset("foo")
	p = p.AppendMatch(Match{Path: "/tmp/a.go", Line: 1, Col: 1, Len: 3, Snippet: "foo"})
	p = p.MarkDone(nil)
	p = p.EnterReplace()
	if !p.Replacing() {
		t.Fatal("expected Replacing()==true after EnterReplace with matches")
	}
	p = p.AppendReplacementRune("b")
	p = p.AppendReplacementRune("a")
	p = p.AppendReplacementRune("r")
	if p.Replacement() != "bar" {
		t.Fatalf("expected replacement 'bar', got %q", p.Replacement())
	}
	p = p.ExitReplace()
	if p.Replacing() {
		t.Fatal("expected Replacing()==false after ExitReplace")
	}
	if p.Replacement() != "bar" {
		t.Fatalf("expected replacement preserved across ExitReplace, got %q", p.Replacement())
	}
}

// BackspaceReplacement drops the last rune and is a no-op on empty input.
func TestPaneBackspaceReplacement(t *testing.T) {
	p := NewPane(theme.Default, "/tmp").Reset("foo")
	p = p.AppendMatch(Match{Path: "/tmp/a.go", Line: 1, Col: 1, Len: 3, Snippet: "foo"})
	p = p.MarkDone(nil).EnterReplace()
	p = p.AppendReplacementRune("a").AppendReplacementRune("b")
	p = p.BackspaceReplacement()
	if p.Replacement() != "a" {
		t.Fatalf("expected 'a' after one backspace, got %q", p.Replacement())
	}
	p = p.BackspaceReplacement().BackspaceReplacement() // second is no-op
	if p.Replacement() != "" {
		t.Fatalf("expected empty after exhaustive backspace, got %q", p.Replacement())
	}
}

// In replace mode, Enter emits ApplyMsg with the current replacement
// (not OpenMsg as the search-mode default would).
func TestPaneEnterInReplaceModeEmitsApplyMsg(t *testing.T) {
	p := NewPane(theme.Default, "/tmp").WithSize(80, 20).Reset("foo")
	p = p.AppendMatch(Match{Path: "/tmp/a.go", Line: 1, Col: 1, Len: 3, Snippet: "foo"})
	p = p.MarkDone(nil).EnterReplace()
	p = p.AppendReplacementRune("b").AppendReplacementRune("a").AppendReplacementRune("r")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected Cmd on Enter in replace mode")
	}
	msg, ok := cmd().(ApplyMsg)
	if !ok {
		t.Fatalf("expected ApplyMsg, got %T", cmd())
	}
	if msg.Replacement != "bar" {
		t.Fatalf("expected replacement 'bar' on ApplyMsg, got %q", msg.Replacement)
	}
}

// In replace mode, Esc collapses back to search-result navigation
// without emitting CancelMsg. A single Esc never throws away the
// user's typed replacement; a second Esc (now in search mode) does
// cancel the whole pane.
func TestPaneEscInReplaceModeCollapsesWithoutCancel(t *testing.T) {
	p := NewPane(theme.Default, "/tmp").WithSize(80, 20).Reset("foo")
	p = p.AppendMatch(Match{Path: "/tmp/a.go", Line: 1, Col: 1, Len: 3, Snippet: "foo"})
	p = p.MarkDone(nil).EnterReplace().AppendReplacementRune("x")
	got, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatalf("expected nil Cmd on Esc in replace mode, got %T", cmd())
	}
	if got.Replacing() {
		t.Fatal("expected Replacing()==false after Esc")
	}
	if got.Replacement() != "x" {
		t.Fatalf("expected replacement preserved on Esc, got %q", got.Replacement())
	}
	// Second Esc (now in search mode) emits CancelMsg.
	_, cmd2 := got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd2 == nil {
		t.Fatal("expected CancelMsg on Esc in search mode")
	}
	if _, ok := cmd2().(CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd2())
	}
}

// In replace mode the cursor still moves so the user can see what
// they're about to replace while typing.
func TestPaneCursorMovesInReplaceMode(t *testing.T) {
	p := NewPane(theme.Default, "/tmp").WithSize(80, 20).Reset("foo")
	for i := 0; i < 3; i++ {
		p = p.AppendMatch(Match{Path: "/tmp/x.go", Line: i + 1, Col: 1, Len: 3, Snippet: "foo"})
	}
	p = p.MarkDone(nil).EnterReplace()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	sel, _ := p.Selected()
	if sel.Line != 3 {
		t.Fatalf("expected cursor on line 3 after two Downs in replace mode, got %d", sel.Line)
	}
}

// View() renders a replace-prompt row between header and rows when
// Replacing()==true.
func TestPaneViewRendersReplaceRow(t *testing.T) {
	p := NewPane(theme.Default, "/tmp").WithSize(80, 20).Reset("foo")
	p = p.AppendMatch(Match{Path: "/tmp/a.go", Line: 1, Col: 1, Len: 3, Snippet: "func foo()"})
	p = p.MarkDone(nil).EnterReplace()
	p = p.AppendReplacementRune("b").AppendReplacementRune("a").AppendReplacementRune("r")
	out := p.View()
	if !strings.Contains(out, "replace") {
		t.Fatalf("expected 'replace' label in View() with Replacing()==true:\n%s", out)
	}
	if !strings.Contains(out, "bar") {
		t.Fatalf("expected 'bar' replacement value in View():\n%s", out)
	}
}

// TestTruncateRespectsDisplayWidth confirms truncate budgets display cells,
// not runes: a wide character (CJK) is one rune but two columns, so a
// rune-counting truncate would overshoot the column budget.
func TestTruncateRespectsDisplayWidth(t *testing.T) {
	t.Parallel()
	out := truncate("日本語コード done", 6)
	if w := lipgloss.Width(out); w > 6 {
		t.Errorf("truncate exceeded 6 display cells (got %d): %q", w, out)
	}
	if !strings.HasSuffix(out, "…") {
		t.Errorf("expected ellipsis tail on truncated input: %q", out)
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate fit-input changed content: %q", got)
	}
}
