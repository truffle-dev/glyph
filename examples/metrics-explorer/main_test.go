package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelHasFleet(t *testing.T) {
	m := newModel()
	if got := len(m.services); got != totalSvcs {
		t.Fatalf("want %d services, got %d", totalSvcs, got)
	}
	if m.page != 0 {
		t.Fatalf("first page should be 0, got %d", m.page)
	}
	if m.lastPage() != 2 {
		t.Fatalf("47/20 should give last page index 2, got %d", m.lastPage())
	}
}

func TestViewRendersAllSurfaces(t *testing.T) {
	m := newModel()
	out := m.View()
	for _, want := range []string{
		"metrics-explorer",
		"Service",
		"p99",
		"rows 1",
		"page",
		"RECENT ROLLOUTS",
		"CONFIG",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q\n--- view ---\n%s", want, out)
		}
	}
}

func TestRightBracketAdvancesPage(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	next, ok := updated.(model)
	if !ok {
		t.Fatalf("update returned non-model")
	}
	if next.page != 1 {
		t.Fatalf("] should advance to page 1, got %d", next.page)
	}
	if !strings.Contains(next.View(), "rows 21") {
		t.Errorf("page 2 should show rows starting at 21\n--- view ---\n%s", next.View())
	}
}

func TestEndKeyJumpsToLast(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	next := updated.(model)
	if next.page != next.lastPage() {
		t.Fatalf("G should land on last page %d, got %d", next.lastPage(), next.page)
	}
	if got := len(next.pageSlice()); got != totalSvcs-perPage*2 {
		t.Fatalf("last page should show %d rows, got %d", totalSvcs-perPage*2, got)
	}
}

func TestRightPanelChangesWithCursor(t *testing.T) {
	m := newModel()
	first := m.View()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := updated.(model)
	second := next.View()
	if first == second {
		t.Fatalf("moving cursor should change the rendered view (right panel should refresh)")
	}
}

func TestEmptyPageGuardsCursor(t *testing.T) {
	m := newModel()
	m.page = m.lastPage()
	m.cursor = 99
	m = m.refreshTableRows()
	if m.cursor < 0 || m.cursor >= len(m.pageSlice()) {
		t.Fatalf("cursor not clamped to page bounds: cursor=%d len=%d", m.cursor, len(m.pageSlice()))
	}
}
