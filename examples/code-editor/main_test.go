package main

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

// pump runs m.Update on the given msg and then drains the resulting
// Cmd (if any) into a follow-up Update so internal Cmd-routed messages
// (QueryMsg / CloseMsg from the find-bar) reach the outer model the
// same way they would inside a real tea.Program.
func pump(m model, msg tea.Msg) model {
	updated, cmd := m.Update(msg)
	m = updated.(model)
	if cmd == nil {
		return m
	}
	next := cmd()
	if next == nil {
		return m
	}
	updated, _ = m.Update(next)
	return updated.(model)
}

func resize(m model, w, h int) model {
	return pump(m, tea.WindowSizeMsg{Width: w, Height: h})
}

func sendKey(t *testing.T, m model, s string) model {
	t.Helper()
	return pump(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func sendCtrl(t *testing.T, m model, kt tea.KeyType) model {
	t.Helper()
	return pump(m, tea.KeyMsg{Type: kt})
}

func TestInitialFocusOnEditor(t *testing.T) {
	m := newModel()
	if m.focus != focusEditor {
		t.Fatalf("expected initial focus on editor, got %v", m.focus)
	}
	if len(m.bufs) != 1 {
		t.Fatalf("expected 1 open buffer at start, got %d", len(m.bufs))
	}
	if m.bufs[0].entry.Path != "cmd/main.go" {
		t.Fatalf("expected cmd/main.go preopened, got %q", m.bufs[0].entry.Path)
	}
}

func TestCtrlLFocusesTree(t *testing.T) {
	m := resize(newModel(), 120, 36)
	m = sendCtrl(t, m, tea.KeyCtrlL)
	if m.focus != focusTree {
		t.Fatalf("expected focusTree, got %v", m.focus)
	}
}

func TestCtrlEFocusesEditor(t *testing.T) {
	m := resize(newModel(), 120, 36)
	m = sendCtrl(t, m, tea.KeyCtrlL)
	m = sendCtrl(t, m, tea.KeyCtrlE)
	if m.focus != focusEditor {
		t.Fatalf("expected focusEditor, got %v", m.focus)
	}
}

func TestCtrlFOpensFindBar(t *testing.T) {
	m := resize(newModel(), 120, 36)
	m = sendCtrl(t, m, tea.KeyCtrlF)
	if !m.barOpen {
		t.Fatal("expected barOpen=true after Ctrl-F")
	}
	if m.focus != focusFindBar {
		t.Fatalf("expected focusFindBar, got %v", m.focus)
	}
	if !m.bar.Focused() {
		t.Fatal("expected find-bar to be focused")
	}
}

func TestEscClosesFindBar(t *testing.T) {
	m := resize(newModel(), 120, 36)
	m = sendCtrl(t, m, tea.KeyCtrlF)
	m = sendCtrl(t, m, tea.KeyEsc)
	if m.barOpen {
		t.Fatal("expected barOpen=false after Esc")
	}
	if m.focus != focusEditor {
		t.Fatalf("expected focus to return to editor, got %v", m.focus)
	}
}

func TestFindBarQueryFindsMatches(t *testing.T) {
	m := resize(newModel(), 120, 36)
	m = sendCtrl(t, m, tea.KeyCtrlF)
	for _, r := range []rune("ctx") {
		m = pump(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.bar.MatchCount() == 0 {
		t.Fatal("expected at least one match for 'ctx' in cmd/main.go")
	}
}

func TestEnterOpensFileFromTree(t *testing.T) {
	m := resize(newModel(), 120, 36)
	m = sendCtrl(t, m, tea.KeyCtrlL)
	// Move cursor down to root.go (next leaf after main.go).
	tm, _ := m.routeToTree(tea.KeyMsg{Type: tea.KeyDown})
	m = tm.(model)
	tm, _ = m.routeToTree(tea.KeyMsg{Type: tea.KeyEnter})
	m = tm.(model)
	if len(m.bufs) < 2 {
		t.Fatalf("expected at least 2 open buffers, got %d", len(m.bufs))
	}
}

func TestTabCyclesOpenTabs(t *testing.T) {
	m := resize(newModel(), 120, 36)
	// Open a second buffer manually so we have something to cycle to.
	m = m.openFile(m.files["cmd/root.go"])
	if m.active != 1 {
		t.Fatalf("expected active=1 after open, got %d", m.active)
	}
	tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = tm.(model)
	if m.active != 0 {
		t.Fatalf("expected active=0 after Tab wrap, got %d", m.active)
	}
}

func TestCtrlWClosesActiveTab(t *testing.T) {
	m := resize(newModel(), 120, 36)
	m = m.openFile(m.files["cmd/root.go"])
	before := len(m.bufs)
	m = sendCtrl(t, m, tea.KeyCtrlW)
	if len(m.bufs) != before-1 {
		t.Fatalf("expected %d buffers after close, got %d", before-1, len(m.bufs))
	}
}

func TestCtrlWWithLastTabIsNoop(t *testing.T) {
	m := resize(newModel(), 120, 36)
	if len(m.bufs) != 1 {
		t.Fatalf("setup expects 1 buffer, got %d", len(m.bufs))
	}
	m = sendCtrl(t, m, tea.KeyCtrlW)
	if len(m.bufs) != 1 {
		t.Fatalf("last tab should not close, got %d buffers", len(m.bufs))
	}
}

func TestEditingTogglesDirtyAndTabMark(t *testing.T) {
	m := resize(newModel(), 120, 36)
	// Type a single character into the editor and confirm the tab label
	// gains the dirty mark.
	m = sendKey(t, m, "x")
	b := m.currentBuffer()
	if !b.ed.Dirty() {
		t.Fatal("expected editor to be dirty after a keystroke")
	}
	labels := m.tabLabels()
	if !strings.HasPrefix(labels[0], "• ") {
		t.Fatalf("expected dirty mark on tab label, got %q", labels[0])
	}
}

func TestOpeningSameFileTwiceSwitchesTab(t *testing.T) {
	m := resize(newModel(), 120, 36)
	m = m.openFile(m.files["cmd/root.go"])
	bufsBefore := len(m.bufs)
	m = m.openFile(m.files["cmd/main.go"])
	if len(m.bufs) != bufsBefore {
		t.Fatalf("expected no new tab when re-opening; got %d -> %d", bufsBefore, len(m.bufs))
	}
	if m.bufs[m.active].entry.Path != "cmd/main.go" {
		t.Fatalf("expected active tab to be cmd/main.go, got %q", m.bufs[m.active].entry.Path)
	}
}

func TestStatusBarReflectsCurrentFile(t *testing.T) {
	m := resize(newModel(), 120, 36)
	m = m.openFile(m.files["go.mod"])
	m = m.syncStatus()
	v := m.status.View()
	if !strings.Contains(v, "go.mod") {
		t.Fatalf("expected status bar to mention go.mod, got:\n%s", v)
	}
}
