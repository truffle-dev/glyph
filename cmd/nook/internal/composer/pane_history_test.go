package composer

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/cmd/nook/internal/aihistory"
	"github.com/truffle-dev/glyph/components/theme"
)

func newHistoryPane(t *testing.T, store *aihistory.Store, path string) Pane {
	t.Helper()
	p := NewPane(theme.Default, nil).WithHistory(store).WithActivePath(path).Focus()
	return p
}

func TestWithHistoryRefreshesCount(t *testing.T) {
	store := aihistory.NewStore()
	store.Append("a.go", aihistory.Turn{Instruction: "ask", Response: "reply", At: time.Now()})

	// WithActivePath first to set path, then WithHistory pulls count.
	p := NewPane(theme.Default, nil).WithActivePath("a.go").WithHistory(store)
	if got := p.HistoryCount(); got != 1 {
		t.Fatalf("HistoryCount = %d, want 1", got)
	}
	// Reverse order also works because both setters re-sync.
	p2 := NewPane(theme.Default, nil).WithHistory(store).WithActivePath("a.go")
	if got := p2.HistoryCount(); got != 1 {
		t.Fatalf("reverse-order HistoryCount = %d, want 1", got)
	}
}

func TestWithoutHistoryHistoryCountIsZero(t *testing.T) {
	p := NewPane(theme.Default, nil).WithActivePath("a.go")
	if got := p.HistoryCount(); got != 0 {
		t.Fatalf("HistoryCount = %d, want 0 (no store bound)", got)
	}
}

func TestBuildUserPromptIncludesPriorTurns(t *testing.T) {
	prior := []aihistory.Turn{
		{Instruction: "add a Run method", Response: "=== a.go ===\n```\npackage a\nfunc Run() {}\n```\n"},
		{Instruction: "also add a Stop method", Response: "=== a.go ===\n```\npackage a\nfunc Stop() {}\n```\n"},
	}
	got := buildUserPrompt(Context{OpenPath: "a.go", OpenContents: "package a\n"}, "now wire them together", prior)

	for _, want := range []string{
		"Prior turns on this file (oldest first):",
		"Turn 1",
		"User: add a Run method",
		"Turn 2",
		"User: also add a Stop method",
		"Instruction: now wire them together",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q\nprompt:\n%s", want, got)
		}
	}
}

func TestBuildUserPromptOmitsHistoryHeaderWhenEmpty(t *testing.T) {
	got := buildUserPrompt(Context{OpenPath: "a.go"}, "do something", nil)
	if strings.Contains(got, "Prior turns") {
		t.Fatalf("prompt contains history header when prior is nil:\n%s", got)
	}
}

func TestBuildUserPromptCapsPriorTurnsAtSix(t *testing.T) {
	var prior []aihistory.Turn
	for i := 0; i < 9; i++ {
		prior = append(prior, aihistory.Turn{
			Instruction: "ask " + string(rune('A'+i)),
			Response:    "reply " + string(rune('A'+i)),
		})
	}
	got := buildUserPrompt(Context{}, "now", prior)
	// First 3 ("A", "B", "C") should be dropped; last 6 ("D".."I") should remain.
	for _, dropped := range []string{"ask A", "ask B", "ask C"} {
		if strings.Contains(got, dropped) {
			t.Fatalf("expected %q to be capped out:\n%s", dropped, got)
		}
	}
	for _, kept := range []string{"ask D", "ask E", "ask F", "ask G", "ask H", "ask I"} {
		if !strings.Contains(got, kept) {
			t.Fatalf("expected %q to be kept:\n%s", kept, got)
		}
	}
}

func TestStreamDoneRecordsTurnInStore(t *testing.T) {
	store := aihistory.NewStore()
	p := newHistoryPane(t, store, "a.go")
	p.prompt = "do the thing"
	p.buffer = "=== a.go ===\n```\nfinished\n```\n"

	pp, _ := p.Update(streamDoneMsg{err: nil})
	if got := store.Count("a.go"); got != 1 {
		t.Fatalf("Count = %d, want 1 (streamDone should record)", got)
	}
	if got := pp.HistoryCount(); got != 1 {
		t.Fatalf("pane HistoryCount = %d, want 1", got)
	}
	turns := store.Turns("a.go")
	if turns[0].Instruction != "do the thing" {
		t.Fatalf("recorded Instruction = %q", turns[0].Instruction)
	}
	if !strings.Contains(turns[0].Response, "finished") {
		t.Fatalf("recorded Response missing body: %q", turns[0].Response)
	}
}

func TestStreamDoneWithErrorDoesNotRecord(t *testing.T) {
	store := aihistory.NewStore()
	p := newHistoryPane(t, store, "a.go")
	p.prompt = "do the thing"
	p.buffer = "partial"

	_, _ = p.Update(streamDoneMsg{err: errSentinel("boom")})
	if got := store.Count("a.go"); got != 0 {
		t.Fatalf("Count = %d, want 0 (error should not record)", got)
	}
}

func TestStreamDoneNoPathDoesNotRecord(t *testing.T) {
	store := aihistory.NewStore()
	p := NewPane(theme.Default, nil).WithHistory(store).Focus()
	p.prompt = "do the thing"
	p.buffer = "=== a.go ===\n```\nx\n```\n"

	_, _ = p.Update(streamDoneMsg{err: nil})
	if got := store.Count(""); got != 0 {
		t.Fatalf("empty-path Count = %d, want 0", got)
	}
}

func TestStreamDoneEmptyPromptDoesNotRecord(t *testing.T) {
	store := aihistory.NewStore()
	p := newHistoryPane(t, store, "a.go")
	p.prompt = "   "
	p.buffer = "=== a.go ===\n```\nx\n```\n"

	_, _ = p.Update(streamDoneMsg{err: nil})
	if got := store.Count("a.go"); got != 0 {
		t.Fatalf("Count = %d, want 0 (empty prompt should not record)", got)
	}
}

func TestAltHClearsHistoryAndSetsStatus(t *testing.T) {
	store := aihistory.NewStore()
	store.Append("a.go", aihistory.Turn{Instruction: "x", Response: "y"})
	store.Append("a.go", aihistory.Turn{Instruction: "x2", Response: "y2"})
	p := newHistoryPane(t, store, "a.go")

	pp, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}, Alt: true})
	if got := store.Count("a.go"); got != 0 {
		t.Fatalf("after alt+h, Count = %d, want 0", got)
	}
	if got := pp.HistoryCount(); got != 0 {
		t.Fatalf("pane HistoryCount = %d, want 0", got)
	}
	if !strings.Contains(pp.statusOn, "cleared 2") {
		t.Fatalf("statusOn = %q, want contains 'cleared 2'", pp.statusOn)
	}
}

func TestAltHWhenNoHistorySetsStatus(t *testing.T) {
	store := aihistory.NewStore()
	p := newHistoryPane(t, store, "a.go")

	pp, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}, Alt: true})
	if !strings.Contains(pp.statusOn, "no per-file history") {
		t.Fatalf("statusOn = %q, want 'no per-file history'", pp.statusOn)
	}
}

func TestAltHWithoutHistoryStoreShowsStatus(t *testing.T) {
	p := NewPane(theme.Default, nil).WithActivePath("a.go").Focus()
	pp, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}, Alt: true})
	if !strings.Contains(pp.statusOn, "no per-file history") {
		t.Fatalf("statusOn = %q, want 'no per-file history'", pp.statusOn)
	}
}

func TestViewShowsHistoryCountWhenNonZero(t *testing.T) {
	store := aihistory.NewStore()
	store.Append("a.go", aihistory.Turn{Instruction: "x", Response: "y"})
	store.Append("a.go", aihistory.Turn{Instruction: "x2", Response: "y2"})
	p := newHistoryPane(t, store, "a.go")

	out := p.View()
	if !strings.Contains(out, "2 prior turn") {
		t.Fatalf("View missing prior-turn count line:\n%s", out)
	}
	if !strings.Contains(out, "alt+h clear") {
		t.Fatalf("View missing clear hint:\n%s", out)
	}
}

func TestViewHidesHistoryLineWhenZero(t *testing.T) {
	store := aihistory.NewStore()
	p := newHistoryPane(t, store, "a.go")

	out := p.View()
	if strings.Contains(out, "prior turn") {
		t.Fatalf("View should not show history line when empty:\n%s", out)
	}
}

func TestWithActivePathSwitchesScope(t *testing.T) {
	store := aihistory.NewStore()
	store.Append("a.go", aihistory.Turn{Instruction: "for a", Response: "x"})
	store.Append("b.go", aihistory.Turn{Instruction: "for b1", Response: "y"})
	store.Append("b.go", aihistory.Turn{Instruction: "for b2", Response: "z"})

	p := NewPane(theme.Default, nil).WithHistory(store).WithActivePath("a.go")
	if got := p.HistoryCount(); got != 1 {
		t.Fatalf("a.go HistoryCount = %d, want 1", got)
	}
	p = p.WithActivePath("b.go")
	if got := p.HistoryCount(); got != 2 {
		t.Fatalf("b.go HistoryCount = %d, want 2", got)
	}
	p = p.WithActivePath("c.go")
	if got := p.HistoryCount(); got != 0 {
		t.Fatalf("c.go HistoryCount = %d, want 0", got)
	}
}

// errSentinel is a tiny error type so we don't need to import errors.New into
// the test body inline.
type errSentinel string

func (e errSentinel) Error() string { return string(e) }
