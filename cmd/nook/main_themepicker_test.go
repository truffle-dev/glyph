package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// altShiftT is the alt+shift+T key event that opens the theme picker.
func altShiftT() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}, Alt: true}
}

func newThemeModel(t *testing.T) model {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	return m.resize()
}

func TestAltShiftTOpensThemePicker(t *testing.T) {
	m := newThemeModel(t)

	updated, _ := m.Update(altShiftT())
	mm := updated.(model)
	if mm.overlay != overlayThemePicker {
		t.Fatalf("expected theme picker overlay, got %v", mm.overlay)
	}
	if mm.themePickerOrig != mm.themeName {
		t.Fatalf("orig theme %q should match active %q on open", mm.themePickerOrig, mm.themeName)
	}
	if !strings.Contains(mm.View(), "session only") {
		t.Errorf("theme picker view should disclose session-only behavior")
	}
}

func TestThemePickerPreviewsAndCommits(t *testing.T) {
	m := newThemeModel(t)
	orig := m.themeName

	updated, _ := m.Update(altShiftT())
	// Move the highlight down: the previewed theme should change live.
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyDown})
	mm := updated.(model)
	previewed := mm.themePicker.Selected()
	if previewed == orig {
		t.Fatalf("down should preview a different theme, still on %q", orig)
	}
	if mm.themeName != orig {
		t.Fatalf("preview must not commit themeName yet; got %q", mm.themeName)
	}

	// Enter keeps the previewed theme for the session.
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(model)
	if mm.overlay != overlayNone {
		t.Fatalf("enter should close the picker, got %v", mm.overlay)
	}
	if mm.themeName != previewed {
		t.Fatalf("enter should commit %q, got %q", previewed, mm.themeName)
	}
}

func TestThemePickerEscRestoresOriginal(t *testing.T) {
	m := newThemeModel(t)
	orig := m.themeName

	updated, _ := m.Update(altShiftT())
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyDown})
	// Esc cancels: themeName and the live theme both revert to original.
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(model)
	if mm.overlay != overlayNone {
		t.Fatalf("esc should close the picker, got %v", mm.overlay)
	}
	if mm.themeName != orig {
		t.Fatalf("esc should leave themeName at %q, got %q", orig, mm.themeName)
	}
	if wantT, _ := resolveTheme(orig); mm.theme != wantT {
		t.Fatalf("esc should restore the live theme to the %q palette", orig)
	}
}
