package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// question is the '?' key event that toggles the help overlay (only when no
// file is actively absorbing keystrokes).
func question() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
}

// typeRune feeds a single printable character into the model.
func typeRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func newHelpModel(t *testing.T) model {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	return m.resize()
}

func openHelp(t *testing.T, m model) model {
	updated, _ := m.Update(question())
	mm := updated.(model)
	if mm.overlay != overlayHelp {
		t.Fatalf("? should open the help overlay, got %v", mm.overlay)
	}
	if mm.helpQuery != "" {
		t.Fatalf("help should open with an empty query, got %q", mm.helpQuery)
	}
	return mm
}

func typeQuery(m model, s string) model {
	for _, r := range s {
		updated, _ := m.Update(typeRune(r))
		m = updated.(model)
	}
	return m
}

func TestHelpSearch_TypingFiltersTheCard(t *testing.T) {
	m := openHelp(t, newHelpModel(t))
	m = typeQuery(m, "rename")

	if m.helpQuery != "rename" {
		t.Fatalf("typed query not captured, got %q", m.helpQuery)
	}
	view := m.View()
	if !strings.Contains(view, "search: rename") {
		t.Errorf("filtered help view should show the search line")
	}
	if !strings.Contains(view, "Rename symbol under cursor (workspace-wide)") {
		t.Errorf("filtered help view should still contain the rename binding")
	}
	if strings.Contains(view, "Fuzzy file picker") {
		t.Errorf("filtered help view should drop bindings that do not match the query")
	}
}

func TestHelpSearch_BackspaceEditsQuery(t *testing.T) {
	m := openHelp(t, newHelpModel(t))
	m = typeQuery(m, "rename")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(model)

	if m.helpQuery != "rena" {
		t.Fatalf("backspace should trim the query to %q, got %q", "rena", m.helpQuery)
	}
	if m.overlay != overlayHelp {
		t.Fatalf("backspace must not close the overlay, got %v", m.overlay)
	}
}

func TestHelpSearch_EscIsTwoStage(t *testing.T) {
	m := openHelp(t, newHelpModel(t))
	m = typeQuery(m, "git")

	// First esc clears the query but keeps the overlay open.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if m.overlay != overlayHelp {
		t.Fatalf("first esc should keep help open while a query is active, got %v", m.overlay)
	}
	if m.helpQuery != "" {
		t.Fatalf("first esc should clear the query, got %q", m.helpQuery)
	}

	// Second esc, now that the query is empty, closes the overlay.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if m.overlay != overlayNone {
		t.Fatalf("second esc should close help, got %v", m.overlay)
	}
}

func TestHelpSearch_QuestionMarkClosesMidSearch(t *testing.T) {
	m := openHelp(t, newHelpModel(t))
	m = typeQuery(m, "git")

	updated, _ := m.Update(question())
	m = updated.(model)
	if m.overlay != overlayNone {
		t.Fatalf("? should close help even mid-search, got %v", m.overlay)
	}
	if m.helpQuery != "" {
		t.Fatalf("closing help should reset the query, got %q", m.helpQuery)
	}
}

func TestHelpSearch_SpaceBuildsMultiTokenQuery(t *testing.T) {
	m := openHelp(t, newHelpModel(t))
	m = typeQuery(m, "git")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(model)
	m = typeQuery(m, "toggle")

	if m.helpQuery != "git toggle" {
		t.Fatalf("space should extend the query, got %q", m.helpQuery)
	}
	if !strings.Contains(m.View(), "Toggle git pane") {
		t.Errorf("multi-token query should still match \"Toggle git pane\"")
	}
}
