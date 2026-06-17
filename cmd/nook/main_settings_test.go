package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// altPeriod is the alt+. key event that toggles the settings overlay.
func altPeriod() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}, Alt: true}
}

func TestAltPeriodOpensSettingsOverlay(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()

	updated, _ := m.Update(altPeriod())
	mm := updated.(model)
	if mm.overlay != overlaySettings {
		t.Fatalf("expected settings overlay, got %v", mm.overlay)
	}
	if !strings.Contains(mm.View(), "nook settings") {
		t.Errorf("settings overlay view should render the card title")
	}
}

func TestAltPeriodTogglesSettingsClosed(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()

	updated, _ := m.Update(altPeriod())
	updated, _ = updated.(model).Update(altPeriod())
	mm := updated.(model)
	if mm.overlay != overlayNone {
		t.Fatalf("a second alt+. should dismiss settings, got %v", mm.overlay)
	}
}

func TestEscDismissesSettingsOverlay(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()

	updated, _ := m.Update(altPeriod())
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(model)
	if mm.overlay != overlayNone {
		t.Fatalf("esc should dismiss settings, got %v", mm.overlay)
	}
}

func TestSettingsOverlaySwallowsStrayKeys(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()

	updated, _ := m.Update(altPeriod())
	// A stray ctrl+g must not open the git pane underneath the read-only card.
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	mm := updated.(model)
	if mm.overlay != overlaySettings {
		t.Fatalf("stray key should leave settings overlay open, got %v", mm.overlay)
	}
}
