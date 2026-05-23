package filetree

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func sampleTree() Node {
	return Node{
		Name: "project",
		Children: []Node{
			{Name: "cmd", Children: []Node{
				{Name: "main.go"},
				{Name: "root.go"},
			}},
			{Name: "internal", Children: []Node{
				{Name: "store.go"},
			}},
			{Name: "README.md"},
		},
	}
}

func TestNewRootIsExpanded(t *testing.T) {
	m := New(sampleTree())
	out := m.View()
	for _, want := range []string{"cmd", "internal", "README.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("root children should be visible immediately, missing %q in:\n%s", want, out)
		}
	}
}

func TestDirCollapsedHidesChildren(t *testing.T) {
	m := New(sampleTree())
	out := m.View()
	if strings.Contains(out, "main.go") {
		t.Fatalf("nested dir 'cmd' should start collapsed, but main.go is visible:\n%s", out)
	}
}

func TestDirExpandShowsChildren(t *testing.T) {
	m := New(sampleTree())
	// Move cursor to cmd (first child of root) and press right.
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mi
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mi
	out := m.View()
	if !strings.Contains(out, "main.go") {
		t.Fatalf("expanding cmd should reveal main.go:\n%s", out)
	}
}

func TestEnterTogglesDirExpansion(t *testing.T) {
	m := New(sampleTree())
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mi
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mi
	if !strings.Contains(m.View(), "main.go") {
		t.Fatalf("Enter on collapsed dir should expand it")
	}
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mi
	if strings.Contains(m.View(), "main.go") {
		t.Fatalf("Enter on expanded dir should collapse it")
	}
}

func TestCursorMovesWithJK(t *testing.T) {
	m := New(sampleTree())
	first, _ := m.current()
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = mi
	second, _ := m.current()
	if first.path == second.path {
		t.Fatalf("j should move cursor; before=%v after=%v", first.path, second.path)
	}
}

func TestSelectMsgEmittedOnEnter(t *testing.T) {
	m := New(sampleTree())
	mi, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown}) // move to "cmd"
	m = mi
	mi, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mi
	if cmd == nil {
		t.Fatal("Enter on a dir row should emit a SelectMsg cmd")
	}
	msg := cmd()
	sm, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", msg)
	}
	if sm.Path == "" {
		t.Errorf("SelectMsg.Path should be non-empty, got %+v", sm)
	}
	if !sm.IsDir {
		t.Errorf("SelectMsg.IsDir should be true on a dir row, got %+v", sm)
	}
}

func TestMultiSelectToggleSpace(t *testing.T) {
	m := New(sampleTree()).WithMultiSelect(true)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mi
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m = mi
	if len(m.SelectedPaths()) != 1 {
		t.Fatalf("space should add row to selection; selected=%v", m.SelectedPaths())
	}
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m = mi
	if len(m.SelectedPaths()) != 0 {
		t.Fatalf("space twice should toggle selection off; selected=%v", m.SelectedPaths())
	}
}

func TestCursorClampedAfterCollapse(t *testing.T) {
	m := New(sampleTree())
	// Expand cmd then jump to its last child.
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mi
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mi
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mi
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mi
	rowsBefore := len(m.visible)
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	m = mi
	if m.cursor != len(m.visible)-1 {
		t.Fatalf("G should land on last row; cursor=%d visible=%d", m.cursor, len(m.visible))
	}
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = mi
	if m.cursor != 0 {
		t.Fatalf("g should land on first row; cursor=%d", m.cursor)
	}
	if rowsBefore != len(m.visible) {
		t.Fatalf("cursor moves should not change visible rows")
	}
}

func TestSelectedReportsCurrentPath(t *testing.T) {
	m := New(sampleTree())
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mi
	if m.Selected() != "cmd" {
		t.Fatalf("expected Selected() == cmd, got %q", m.Selected())
	}
}

func TestLeftOnLeafJumpsToParent(t *testing.T) {
	m := New(sampleTree())
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mi
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand cmd
	m = mi
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // into main.go
	m = mi
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft}) // jump back to cmd
	m = mi
	if m.Selected() != "cmd" {
		t.Fatalf("Left on leaf should jump to parent; cursor at %q", m.Selected())
	}
}
