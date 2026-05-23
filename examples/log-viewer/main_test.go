package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	logstream "github.com/truffle-dev/glyph/components/log-stream"
	selectinput "github.com/truffle-dev/glyph/components/select"
	textinput "github.com/truffle-dev/glyph/components/text-input"
)

func resize(t *testing.T, m model, w, h int) model {
	t.Helper()
	mi, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return mi.(model)
}

func TestInitialView(t *testing.T) {
	m := newModel()
	m = resize(t, m, 120, 28)
	out := m.View()
	if !strings.Contains(out, "live") {
		t.Fatal("status bar must show live mode by default")
	}
	if !strings.Contains(out, "All") {
		t.Fatal("tabs row must include All level")
	}
}

func TestTickAppendsEntries(t *testing.T) {
	m := newModel()
	m = resize(t, m, 120, 28)
	for i := 0; i < 10; i++ {
		mi, _ := m.Update(tickMsg(time.Now()))
		m = mi.(model)
	}
	if len(m.all) != 10 {
		t.Fatalf("expected 10 entries in buffer, got %d", len(m.all))
	}
	if m.entryCount == 0 {
		t.Fatal("at least some entries should have passed the default filter")
	}
}

func TestSpaceTogglesPause(t *testing.T) {
	m := newModel()
	m = resize(t, m, 120, 28)
	if m.paused {
		t.Fatal("should start unpaused")
	}
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = mi.(model)
	if !m.paused {
		t.Fatal("Space should pause")
	}

	// Confirm tick during pause does not grow the buffer.
	before := len(m.all)
	mi, _ = m.Update(tickMsg(time.Now()))
	m = mi.(model)
	if len(m.all) != before {
		t.Fatal("ticking while paused must not append entries")
	}
}

func TestTabAdvancesLevelFilter(t *testing.T) {
	m := newModel()
	m = resize(t, m, 120, 28)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mi.(model)
	if m.filter != filterInfo {
		t.Fatalf("expected filterInfo after one Tab, got %d", m.filter)
	}
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mi.(model)
	if m.filter != filterWarn {
		t.Fatalf("expected filterWarn after two Tabs, got %d", m.filter)
	}
}

func TestSourcePickerFlow(t *testing.T) {
	m := newModel()
	m = resize(t, m, 120, 28)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	m = mi.(model)
	if m.mode != modeSourcePicker {
		t.Fatalf("expected modeSourcePicker after ctrl+f, got %d", m.mode)
	}
	mi, _ = m.Update(selectinput.SelectMsg{Option: selectinput.Option{Label: "auth", Value: "auth"}})
	m = mi.(model)
	if m.source != "auth" {
		t.Fatalf("expected source=auth, got %q", m.source)
	}
	if m.mode != modeView {
		t.Fatal("returning from picker should restore view mode")
	}
}

func TestSearchPromptAppliesFilter(t *testing.T) {
	m := newModel()
	m = resize(t, m, 120, 28)
	// Seed two entries that will and will not match.
	m.all = []logstream.Entry{
		{Time: time.Now(), Level: logstream.LevelInfo, Source: "server", Message: "listening on :8080"},
		{Time: time.Now(), Level: logstream.LevelInfo, Source: "db", Message: "pool size 12/20"},
	}
	m = m.rebuildStream()
	if m.entryCount != 2 {
		t.Fatalf("expected 2 entries pre-search, got %d", m.entryCount)
	}

	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = mi.(model)
	if m.mode != modeSearchPrompt {
		t.Fatalf("expected modeSearchPrompt, got %d", m.mode)
	}

	mi, _ = m.Update(textinput.SubmitMsg{Value: "listening"})
	m = mi.(model)
	if m.query != "listening" {
		t.Fatalf("expected query='listening', got %q", m.query)
	}
	if m.entryCount != 1 {
		t.Fatalf("expected 1 matching entry, got %d", m.entryCount)
	}
}

func TestCtrlLClearsBuffer(t *testing.T) {
	m := newModel()
	m = resize(t, m, 120, 28)
	for i := 0; i < 5; i++ {
		mi, _ := m.Update(tickMsg(time.Now()))
		m = mi.(model)
	}
	if len(m.all) == 0 {
		t.Fatal("expected buffer to grow")
	}
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	m = mi.(model)
	if len(m.all) != 0 {
		t.Fatal("ctrl+l should clear the buffer")
	}
	if m.entryCount != 0 {
		t.Fatal("ctrl+l should reset entryCount")
	}
}

func TestQuitOnQ(t *testing.T) {
	m := newModel()
	m = resize(t, m, 120, 28)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should issue tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q did not produce QuitMsg")
	}
}
