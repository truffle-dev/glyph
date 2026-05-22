package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestShowcaseRendersEachTab exercises the headless model path on every
// tab and confirms View() produces non-empty output. The binary needs a
// TTY to run as a TUI, but the model logic is reachable in tests.
func TestShowcaseRendersEachTab(t *testing.T) {
	m := newModel()

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init must return the tick command")
	}

	mi, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	m = mi.(model)

	for tab := tabKey(0); tab < tabNumTabs; tab++ {
		if m.active != tab {
			mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
			m = mi.(model)
		}
		if m.active != tab {
			t.Fatalf("expected to land on tab %d, got %d", tab, m.active)
		}
		out := m.View()
		if out == "" {
			t.Fatalf("View on tab %d returned empty string", tab)
		}
		if !strings.Contains(out, tabNames[tab]) {
			t.Fatalf("View on tab %d missing its own name %q", tab, tabNames[tab])
		}
	}
}

func TestShowcaseChatEcho(t *testing.T) {
	m := newModel()
	mi, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	m = mi.(model)

	// type "hi"
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h', 'i'}})
	m = mi.(model)

	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mi.(model)

	out := m.View()
	if !strings.Contains(out, "echo: hi") {
		t.Fatalf("expected echo reply in chat view; got:\n%s", out)
	}
}

func TestShowcaseToastAndLogShortcuts(t *testing.T) {
	m := newModel()
	mi, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	m = mi.(model)

	// Tab to non-chat (Commands).
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mi.(model)
	if m.active == tabChat {
		t.Fatal("tab key did not advance from chat")
	}

	// Push a toast with 't'.
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = mi.(model)
	if got := len(m.toasts.Toasts()); got != 1 {
		t.Fatalf("expected one toast after pressing t, got %d", got)
	}

	// Push a log entry with 'l'.
	before := len(m.logs.Entries())
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = mi.(model)
	after := len(m.logs.Entries())
	if after != before+1 {
		t.Fatalf("expected one log entry added by l, got delta %d", after-before)
	}

	// Tick the toast clock past expiry and confirm cleanup runs.
	future := time.Now().Add(1 * time.Hour)
	mi, _ = m.Update(tickMsg(future))
	m = mi.(model)
	if got := len(m.toasts.Toasts()); got != 0 {
		t.Fatalf("expected toasts cleared after far-future tick, got %d", got)
	}
}
