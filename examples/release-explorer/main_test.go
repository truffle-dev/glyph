package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelHasReleases(t *testing.T) {
	m := newModel()
	if got := len(m.releases); got != 7 {
		t.Fatalf("want 7 releases, got %d", got)
	}
	if m.releaseList.Cursor() != 0 {
		t.Fatalf("cursor should start at 0, got %d", m.releaseList.Cursor())
	}
	if m.rightTabs.Active() != 0 {
		t.Fatalf("first tab should be active, got %d", m.rightTabs.Active())
	}
}

func TestViewRendersAllSurfaces(t *testing.T) {
	m := newModel()
	out := m.View()
	for _, want := range []string{
		"release-explorer",
		"truffle-dev/glyph",
		"v0.47.0",
		"Body",
		"Assets",
		"Meta",
		"release",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q\n--- view ---\n%s", want, out)
		}
	}
}

func TestDownArrowMovesCursorAndRefreshesBody(t *testing.T) {
	m := newModel()
	first := m.View()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	next, ok := updated.(model)
	if !ok {
		t.Fatalf("update returned non-model")
	}
	if next.releaseList.Cursor() != 1 {
		t.Fatalf("down should advance cursor to 1, got %d", next.releaseList.Cursor())
	}
	if first == next.View() {
		t.Fatalf("moving cursor should change the rendered view (right panel should refresh)")
	}
	if !strings.Contains(next.View(), "v0.46.0") {
		t.Errorf("after down arrow, view should show v0.46.0 in status bar\n--- view ---\n%s", next.View())
	}
}

func TestRightArrowSwitchesToAssetsTab(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	next, ok := updated.(model)
	if !ok {
		t.Fatalf("update returned non-model")
	}
	if next.rightTabs.Active() != 1 {
		t.Fatalf("right should switch to tab 1, got %d", next.rightTabs.Active())
	}
	out := next.View()
	if !strings.Contains(out, "glyph_0.47.0_linux_amd64.tar.gz") {
		t.Errorf("assets tab should list the first release's asset\n--- view ---\n%s", out)
	}
}

func TestGJumpsToFirstAndLast(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	next := updated.(model)
	if next.releaseList.Cursor() != len(m.releases)-1 {
		t.Fatalf("G should jump to last release, got cursor=%d", next.releaseList.Cursor())
	}
	if !strings.Contains(next.View(), "v0.41.0-beta.1") {
		t.Errorf("after G, view should show v0.41.0-beta.1 in status bar\n--- view ---\n%s", next.View())
	}
	updated, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	back := updated.(model)
	if back.releaseList.Cursor() != 0 {
		t.Fatalf("g should jump back to first release, got cursor=%d", back.releaseList.Cursor())
	}
}

func TestMetaTabShowsReleaseMetadata(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mid := updated.(model)
	updated, _ = mid.Update(tea.KeyMsg{Type: tea.KeyRight})
	last := updated.(model)
	if last.rightTabs.Active() != 2 {
		t.Fatalf("two right-arrows should land on tab 2, got %d", last.rightTabs.Active())
	}
	out := last.View()
	for _, want := range []string{"tag", "published", "kind", "stable", "assets"} {
		if !strings.Contains(out, want) {
			t.Errorf("meta tab missing %q\n--- view ---\n%s", want, out)
		}
	}
}
