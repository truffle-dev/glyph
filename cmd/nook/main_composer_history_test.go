package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/cmd/nook/internal/aihistory"
)

// fixtureWithFile creates a temp root with one file, opens it in the model,
// and returns the model with width/height set. The file is a real .go file so
// the editor is willing to switch buffers without complaining.
func fixtureWithFile(t *testing.T, name, contents string) (model, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(root, name)
	if err := os.WriteFile(abs, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m.bufs.OpenOrSwitch(abs)
	return m, abs
}

func TestComposerOpensWithActivePathWired(t *testing.T) {
	m, abs := fixtureWithFile(t, "a.go", "package a\n")
	if m.aiHistory == nil {
		t.Fatal("aiHistory was nil after newModel")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	mm := updated.(model)
	if mm.right != rightComposer {
		t.Fatalf("Ctrl+L did not open composer (right=%v)", mm.right)
	}
	if got := mm.composer.ActivePath(); got != abs {
		t.Fatalf("composer ActivePath = %q, want %q", got, abs)
	}
}

func TestComposerHistoryCountReflectsPriorTurns(t *testing.T) {
	m, abs := fixtureWithFile(t, "a.go", "package a\n")
	m.aiHistory.Append(abs, aihistory.Turn{Instruction: "first", Response: "x", At: time.Now()})
	m.aiHistory.Append(abs, aihistory.Turn{Instruction: "second", Response: "y", At: time.Now()})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	mm := updated.(model)
	if got := mm.composer.HistoryCount(); got != 2 {
		t.Fatalf("HistoryCount = %d, want 2", got)
	}
}

func TestComposerHistoryScopedToActiveFile(t *testing.T) {
	m, absA := fixtureWithFile(t, "a.go", "package a\n")
	// Add a second file so we can switch buffers.
	absB := filepath.Join(filepath.Dir(absA), "b.go")
	if err := os.WriteFile(absB, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.aiHistory.Append(absA, aihistory.Turn{Instruction: "x", Response: "y"})
	m.aiHistory.Append(absB, aihistory.Turn{Instruction: "x", Response: "y"})
	m.aiHistory.Append(absB, aihistory.Turn{Instruction: "x", Response: "y"})

	// Open composer over a.go.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	mm := updated.(model)
	if got := mm.composer.HistoryCount(); got != 1 {
		t.Fatalf("HistoryCount over a.go = %d, want 1", got)
	}

	// Close composer and switch to b.go.
	updated2, _ := mm.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	mm = updated2.(model)
	if mm.right == rightComposer {
		t.Fatal("Ctrl+L second time should close composer")
	}
	mm.bufs.OpenOrSwitch(absB)
	// Re-open composer; should now scope to b.go.
	updated3, _ := mm.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	mm = updated3.(model)
	if got := mm.composer.ActivePath(); got != absB {
		t.Fatalf("composer ActivePath after switch = %q, want %q", got, absB)
	}
	if got := mm.composer.HistoryCount(); got != 2 {
		t.Fatalf("HistoryCount over b.go = %d, want 2", got)
	}
}

func TestComposerViewMentionsPriorTurnsWhenOpenedWithHistory(t *testing.T) {
	m, abs := fixtureWithFile(t, "a.go", "package a\n")
	m.aiHistory.Append(abs, aihistory.Turn{Instruction: "x", Response: "y"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	mm := updated.(model)

	out := mm.composer.View()
	if !strings.Contains(out, "1 prior turn") {
		t.Fatalf("composer View missing prior-turn count:\n%s", out)
	}
}

func TestHelpListsAltHHistoryClearBinding(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 30
	m = m.resize()
	m.overlay = overlayHelp

	out := m.View()
	if !strings.Contains(out, "alt+h") {
		t.Fatalf("help overlay missing alt+h binding:\n%s", out)
	}
	if !strings.Contains(out, "Clear composer history") {
		t.Fatalf("help overlay missing history-clear description:\n%s", out)
	}
}
