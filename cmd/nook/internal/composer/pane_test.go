package composer

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func TestParseEditsSingleBlock(t *testing.T) {
	body := "=== main.go ===\n```\npackage main\n\nfunc main() {}\n```\n"
	edits := parseEdits(body, nil)
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d (%+v)", len(edits), edits)
	}
	if edits[0].Path != "main.go" {
		t.Fatalf("expected path 'main.go', got %q", edits[0].Path)
	}
	got := strings.TrimRight(edits[0].Proposed, "\n")
	want := "package main\n\nfunc main() {}"
	if got != want {
		t.Fatalf("body mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestParseEditsMultipleBlocks(t *testing.T) {
	body := "=== a.go ===\n```\npackage a\n```\n=== sub/b.go ===\n```go\npackage b\n```\n"
	edits := parseEdits(body, nil)
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits, got %d (%+v)", len(edits), edits)
	}
	if edits[0].Path != "a.go" || edits[1].Path != "sub/b.go" {
		t.Fatalf("paths mismatch: %+v", edits)
	}
}

func TestParseEditsIncrementalPartial(t *testing.T) {
	// First fragment: header only.
	partial1 := "=== file.go ===\n```\nstart"
	edits := parseEdits(partial1, nil)
	if len(edits) != 1 {
		t.Fatalf("expected 1 partial edit, got %d", len(edits))
	}
	if !strings.Contains(edits[0].Proposed, "start") {
		t.Fatalf("expected 'start' in partial body, got %q", edits[0].Proposed)
	}

	// Second fragment: now closed.
	partial2 := "=== file.go ===\n```\nstart end\n```\n"
	edits = parseEdits(partial2, edits)
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit after close, got %d", len(edits))
	}
	if got := strings.TrimRight(edits[0].Proposed, "\n"); got != "start end" {
		t.Fatalf("expected 'start end' after close, got %q", got)
	}
}

func TestParseEditsPreservesAppliedFlag(t *testing.T) {
	prev := []Edit{{Path: "a.go", Applied: true}}
	body := "=== a.go ===\n```\npackage a\n```\n"
	edits := parseEdits(body, prev)
	if !edits[0].Applied {
		t.Fatal("expected Applied flag carried over")
	}
}

func TestStartStreamWithoutClient(t *testing.T) {
	p := NewPane(theme.Default, nil).Focus()
	p.prompt = "rewrite"
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.State() != StateError {
		t.Fatalf("expected StateError, got %d", p.State())
	}
}

func TestComposingTypeAndBackspace(t *testing.T) {
	p := NewPane(theme.Default, nil).Focus()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if p.prompt != "hi" {
		t.Fatalf("expected 'hi', got %q", p.prompt)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.prompt != "h" {
		t.Fatalf("expected 'h', got %q", p.prompt)
	}
}

func TestEscEmitsCancel(t *testing.T) {
	p := NewPane(theme.Default, nil).Focus()
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected Cmd")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd())
	}
}

func TestReviewEnterEmitsApply(t *testing.T) {
	p := NewPane(theme.Default, nil).Focus()
	p.state = StateReview
	p.edits = []Edit{
		{Path: "a.go", Proposed: "package a"},
	}
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected Cmd")
	}
	m, ok := cmd().(ApplyMsg)
	if !ok {
		t.Fatalf("expected ApplyMsg, got %T", cmd())
	}
	if m.Edit.Path != "a.go" {
		t.Fatalf("ApplyMsg path mismatch: %+v", m)
	}
}

func TestReviewAEmitsApplyAll(t *testing.T) {
	p := NewPane(theme.Default, nil).Focus()
	p.state = StateReview
	p.edits = []Edit{
		{Path: "a.go", Proposed: "package a"},
		{Path: "b.go", Proposed: "package b"},
	}
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected Cmd")
	}
	m, ok := cmd().(ApplyAllMsg)
	if !ok {
		t.Fatalf("expected ApplyAllMsg, got %T", cmd())
	}
	if len(m.Edits) != 2 {
		t.Fatalf("expected 2 edits in ApplyAll, got %d", len(m.Edits))
	}
}

func TestReviewXMarksRejected(t *testing.T) {
	p := NewPane(theme.Default, nil).Focus()
	p.state = StateReview
	p.edits = []Edit{
		{Path: "a.go"},
		{Path: "b.go"},
	}
	p.cursor = 1
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !p.edits[1].Rejected {
		t.Fatal("expected b.go rejected")
	}
	if p.edits[0].Rejected {
		t.Fatal("expected a.go untouched")
	}
}

func TestReviewArrowsMoveCursor(t *testing.T) {
	p := NewPane(theme.Default, nil).Focus()
	p.state = StateReview
	p.edits = []Edit{
		{Path: "a.go"},
		{Path: "b.go"},
		{Path: "c.go"},
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.cursor != 2 {
		t.Fatalf("expected cursor=2, got %d", p.cursor)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", p.cursor)
	}
}

func TestViewRendersReview(t *testing.T) {
	p := NewPane(theme.Default, nil).Focus().WithSize(40, 30)
	p.state = StateReview
	p.edits = []Edit{{Path: "main.go", Proposed: "package main\nfunc main() {}\n"}}
	out := p.View()
	if !strings.Contains(out, "composer") {
		t.Fatalf("expected 'composer' header: %s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Fatalf("expected file in review: %s", out)
	}
}

func TestStreamDeltaParsesEditsIncrementally(t *testing.T) {
	p := NewPane(theme.Default, nil).Focus()
	p.state = StateStreaming
	p, _ = p.Update(streamDeltaMsg{text: "=== a.go ===\n```\nstart"})
	if len(p.edits) != 1 {
		t.Fatalf("expected 1 partial edit, got %d (%+v)", len(p.edits), p.edits)
	}
	p, _ = p.Update(streamDeltaMsg{text: "\n```\n"})
	if len(p.edits) != 1 {
		t.Fatalf("expected 1 closed edit, got %d", len(p.edits))
	}
}

func TestStreamDoneTransitionsToReview(t *testing.T) {
	p := NewPane(theme.Default, nil).Focus()
	p.state = StateStreaming
	p.buffer = "=== a.go ===\n```\npackage a\n```\n"
	p, _ = p.Update(streamDoneMsg{err: nil})
	if p.State() != StateReview {
		t.Fatalf("expected StateReview, got %d", p.State())
	}
	if len(p.edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(p.edits))
	}
}

func TestResetClearsState(t *testing.T) {
	p := NewPane(theme.Default, nil).Focus()
	p.prompt = "x"
	p.edits = []Edit{{Path: "a.go"}}
	p.state = StateReview
	p = p.Reset()
	if p.State() != StateComposing {
		t.Fatalf("expected StateComposing after Reset")
	}
	if p.prompt != "" || len(p.edits) != 0 {
		t.Fatalf("expected cleared state after Reset, got prompt=%q edits=%d", p.prompt, len(p.edits))
	}
}

func TestComposerWithRulesPersists(t *testing.T) {
	p := NewPane(theme.Default, nil).WithRules("prefer fmt.Errorf wrapping")
	if p.rules != "prefer fmt.Errorf wrapping" {
		t.Fatalf("WithRules did not set field; got %q", p.rules)
	}
	p = p.WithRules("two-line rule\nsecond line")
	if !strings.Contains(p.rules, "two-line") {
		t.Fatalf("WithRules overwrite lost; got %q", p.rules)
	}
}
