package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func resize(t *testing.T, m model, w, h int) model {
	t.Helper()
	mi, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return mi.(model)
}

func TestInitialRender(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	out := m.View()
	for _, want := range []string{"files", "project", "cmd", "main.go", "ready", "explorer"} {
		if !strings.Contains(out, want) {
			t.Fatalf("initial view missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestNewModelStartsOnCmdMain(t *testing.T) {
	m := newModel()
	if m.tree.Selected() != "cmd/main.go" {
		t.Fatalf("expected cursor on cmd/main.go, got %q", m.tree.Selected())
	}
	if m.current.Path != "cmd/main.go" {
		t.Fatalf("expected current file cmd/main.go, got %q", m.current.Path)
	}
}

func TestDownKeyMovesCursorAndSyncsPreview(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	before := m.tree.Selected()
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mi.(model)
	after := m.tree.Selected()
	if after == before {
		t.Fatalf("Down should advance cursor (before=%q, after=%q)", before, after)
	}
	if m.current.Path != after {
		t.Fatalf("preview did not sync: current=%q tree.Selected=%q", m.current.Path, after)
	}
}

func TestQuitOnQ(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should issue tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q did not produce QuitMsg")
	}
}

func TestCtrlCQuits(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should issue tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not produce QuitMsg")
	}
}

func TestSeedFilesCoversTreeLeaves(t *testing.T) {
	files := seedFiles()
	wantPaths := []string{
		"cmd/main.go",
		"cmd/root.go",
		"internal/store/sqlite.go",
		"internal/store/memory.go",
		"internal/agent.go",
		"internal/config.go",
		"scripts/deploy.sh",
		"go.mod",
		"README.md",
		"config.json",
	}
	for _, p := range wantPaths {
		entry, ok := files[p]
		if !ok {
			t.Fatalf("seedFiles missing entry for %q", p)
		}
		if entry.Source == "" {
			t.Fatalf("entry %q has empty source", p)
		}
		if entry.Path != p {
			t.Fatalf("entry %q has mismatched Path=%q", p, entry.Path)
		}
	}
}

func TestRightExpandsCollapsedDir(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	out := m.View()
	if strings.Contains(out, "deploy.sh") {
		t.Fatal("scripts/deploy.sh should be hidden under collapsed scripts/")
	}
	// Walk the cursor down to scripts/ (it sits after the internal/ branch
	// which has cmd 2 leaves + internal 4 children + store 2 leaves).
	// Just key Down repeatedly until 'scripts' is current.
	for i := 0; i < 20; i++ {
		if strings.HasPrefix(m.tree.Selected(), "scripts") {
			break
		}
		mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mi.(model)
	}
	if !strings.HasPrefix(m.tree.Selected(), "scripts") {
		t.Fatalf("could not navigate to scripts/, ended at %q", m.tree.Selected())
	}
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mi.(model)
	out = m.View()
	if !strings.Contains(out, "deploy.sh") {
		t.Fatal("after expanding scripts/, deploy.sh should be visible")
	}
}

func TestPrettyCrumbs(t *testing.T) {
	cases := map[string]string{
		"":               "project",
		"main.go":        "project / main.go",
		"cmd/main.go":    "project / cmd / main.go",
		"internal/store": "project / internal / store",
	}
	for in, want := range cases {
		if got := prettyCrumbs(in); got != want {
			t.Fatalf("prettyCrumbs(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 1: "1", 9: "9", 10: "10", 42: "42", -7: "-7", 1234: "1234"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Fatalf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}
