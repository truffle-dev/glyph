package accordion

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func sample() []Section {
	return []Section{
		{Title: "Overview", Body: "first line\nsecond line", Value: "ov"},
		{Title: "Details", Body: "alpha\nbeta\ngamma", Value: "de"},
		{Title: "Notes", Body: "single", Value: "no"},
	}
}

func TestNewIsEmptyAndShowsPlaceholder(t *testing.T) {
	m := New()
	if len(m.Sections()) != 0 {
		t.Fatalf("expected empty section list on New, got %d", len(m.Sections()))
	}
	if m.Focused() != 0 {
		t.Fatalf("expected focus 0 on New, got %d", m.Focused())
	}
	if got := m.View(); !strings.Contains(got, "no sections") {
		t.Fatalf("empty view should include placeholder %q, got %q", "no sections", got)
	}
}

func TestWithSectionsClampsFocusOnShrink(t *testing.T) {
	m := New().WithSections(sample()...).WithFocused(2)
	m = m.WithSections(sample()[0:1]...)
	if m.Focused() != 0 {
		t.Fatalf("focus should clamp to 0 when section count shrinks, got %d", m.Focused())
	}
}

func TestWithSectionsFiltersStaleExpandedIndices(t *testing.T) {
	m := New().WithAllowMultiple(true).WithSections(sample()...).WithExpanded(0, 1, 2)
	m = m.WithSections(sample()[0:2]...)
	got := m.ExpandedIndices()
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("expanded set should drop stale index 2 after shrink, got %v", got)
	}
}

func TestWithFocusedClampsOutOfRange(t *testing.T) {
	m := New().WithSections(sample()...)
	if got := m.WithFocused(-1).Focused(); got != 0 {
		t.Errorf("negative focus should clamp to 0, got %d", got)
	}
	if got := m.WithFocused(99).Focused(); got != 2 {
		t.Errorf("over-range focus should clamp to last, got %d", got)
	}
}

func TestSingleExpandedCollapsesSibling(t *testing.T) {
	m := New().WithSections(sample()...).WithExpanded(0)
	if !m.IsExpanded(0) || m.IsExpanded(1) {
		t.Fatalf("expected only section 0 expanded, got %v", m.ExpandedIndices())
	}
	m, _ = m.WithFocused(1).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.IsExpanded(0) {
		t.Fatalf("opening section 1 should close section 0 in single mode")
	}
	if !m.IsExpanded(1) {
		t.Fatalf("section 1 should be expanded after Enter")
	}
}

func TestAllowMultipleKeepsBothOpen(t *testing.T) {
	m := New().WithAllowMultiple(true).WithSections(sample()...).WithExpanded(0)
	m, _ = m.WithFocused(1).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.IsExpanded(0) || !m.IsExpanded(1) {
		t.Fatalf("allowMultiple should keep both sections open, got %v", m.ExpandedIndices())
	}
}

func TestSwitchToSingleKeepsFocusedOpenWhenExpanded(t *testing.T) {
	m := New().WithAllowMultiple(true).WithSections(sample()...).WithExpanded(0, 1, 2).WithFocused(1)
	m = m.WithAllowMultiple(false)
	got := m.ExpandedIndices()
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("switching to single should keep focused (1) and drop others, got %v", got)
	}
}

func TestSwitchToSingleKeepsFirstOpenWhenFocusedClosed(t *testing.T) {
	m := New().WithAllowMultiple(true).WithSections(sample()...).WithExpanded(2).WithFocused(0)
	m = m.WithAllowMultiple(false)
	got := m.ExpandedIndices()
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("switching to single with focused closed should keep first open, got %v", got)
	}
}

func TestUpdateCursorDown(t *testing.T) {
	m := New().WithSections(sample()...)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Focused() != 1 {
		t.Fatalf("down should move focus to 1, got %d", m.Focused())
	}
}

func TestUpdateCursorUpWraps(t *testing.T) {
	m := New().WithSections(sample()...)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Focused() != 2 {
		t.Fatalf("up from 0 should wrap to last (2), got %d", m.Focused())
	}
}

func TestUpdateHomeAndEnd(t *testing.T) {
	m := New().WithSections(sample()...).WithFocused(1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.Focused() != 2 {
		t.Fatalf("End should jump to last, got %d", m.Focused())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.Focused() != 0 {
		t.Fatalf("Home should jump to 0, got %d", m.Focused())
	}
}

func TestUpdateTabAndShiftTab(t *testing.T) {
	m := New().WithSections(sample()...)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.Focused() != 1 {
		t.Fatalf("tab should advance to 1, got %d", m.Focused())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.Focused() != 0 {
		t.Fatalf("shift+tab should return to 0, got %d", m.Focused())
	}
}

func TestUpdateRightExpandsLeftCollapses(t *testing.T) {
	m := New().WithSections(sample()...)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if !m.IsExpanded(0) {
		t.Fatalf("right should expand focused section")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.IsExpanded(0) {
		t.Fatalf("left should collapse focused section")
	}
}

func TestUpdateSpaceTogglesSilently(t *testing.T) {
	m := New().WithSections(sample()...)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.IsExpanded(0) {
		t.Fatalf("space should expand focused section")
	}
	if cmd != nil {
		t.Fatalf("space toggle should not emit a tea.Cmd")
	}
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.IsExpanded(0) {
		t.Fatalf("space again should collapse")
	}
	if cmd != nil {
		t.Fatalf("space toggle should not emit a tea.Cmd")
	}
}

func TestUpdateEnterEmitsSelectMsg(t *testing.T) {
	m := New().WithSections(sample()...).WithFocused(1)
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("enter must return a tea.Cmd")
	}
	msg := cmd()
	sel, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", msg)
	}
	if sel.Index != 1 {
		t.Errorf("SelectMsg.Index = %d, want 1", sel.Index)
	}
	if !sel.Expanded {
		t.Errorf("SelectMsg.Expanded should be true on first Enter")
	}
	if sel.Section.Value != "de" {
		t.Errorf("SelectMsg.Section.Value = %v, want %q", sel.Section.Value, "de")
	}
	if !m2.IsExpanded(1) {
		t.Errorf("section should be expanded in returned model")
	}
}

func TestUpdateEnterAgainEmitsCollapsed(t *testing.T) {
	m := New().WithSections(sample()...).WithExpanded(0)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := cmd().(SelectMsg)
	if sel.Expanded {
		t.Fatalf("Enter on an expanded section should emit Expanded=false")
	}
}

func TestUpdateIgnoresOtherKeysAndMessages(t *testing.T) {
	m := New().WithSections(sample()...)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.Focused() != 0 {
		t.Fatalf("unrelated key should not move focus")
	}
	if cmd != nil {
		t.Fatalf("unrelated key should not emit a cmd")
	}
	m, cmd = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Fatalf("non-key message should not emit a cmd")
	}
}

func TestViewIncludesEveryTitle(t *testing.T) {
	m := New().WithSections(sample()...)
	out := m.View()
	for _, want := range []string{"Overview", "Details", "Notes"} {
		if !strings.Contains(out, want) {
			t.Errorf("View should include title %q, got %q", want, out)
		}
	}
	if !strings.Contains(out, collapsedGlyph) {
		t.Errorf("View should show collapsed glyph %q when all sections collapsed", collapsedGlyph)
	}
}

func TestViewExpandedBodyRendersIndented(t *testing.T) {
	m := New().WithSections(sample()...).WithExpanded(0)
	out := m.View()
	if !strings.Contains(out, expandedGlyph) {
		t.Errorf("View should show expanded glyph %q for open section", expandedGlyph)
	}
	if !strings.Contains(out, bodyIndent+"first line") {
		t.Errorf("body line should be indented under header, got %q", out)
	}
	if !strings.Contains(out, bodyIndent+"second line") {
		t.Errorf("each body line should be indented, got %q", out)
	}
}

func TestViewHeightClampsRows(t *testing.T) {
	m := New().WithSections(sample()...).WithAllowMultiple(true).WithExpandAll().WithSize(40, 4)
	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != 4 {
		t.Fatalf("view should clamp to height 4 rows, got %d lines: %q", len(lines), out)
	}
}

func TestViewScrollsToFocusedHeader(t *testing.T) {
	m := New().WithSections(sample()...).WithAllowMultiple(true).WithExpandAll().WithSize(40, 4)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	out := m.View()
	if !strings.Contains(out, "Notes") {
		t.Fatalf("scroll should keep focused header (Notes) visible, got %q", out)
	}
}

func TestFocusedSectionEmptyReturnsFalse(t *testing.T) {
	m := New()
	if _, ok := m.FocusedSection(); ok {
		t.Fatalf("FocusedSection should return ok=false on empty section list")
	}
	m = m.WithSections(sample()...)
	sec, ok := m.FocusedSection()
	if !ok || sec.Title != "Overview" {
		t.Fatalf("FocusedSection should return Overview on populated list, got %+v ok=%v", sec, ok)
	}
}

func TestWithExpandAllSingleModeKeepsLast(t *testing.T) {
	m := New().WithSections(sample()...).WithExpandAll()
	got := m.ExpandedIndices()
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("ExpandAll in single mode should keep only the last index, got %v", got)
	}
}

func TestWithExpandAllMultipleOpensAll(t *testing.T) {
	m := New().WithAllowMultiple(true).WithSections(sample()...).WithExpandAll()
	got := m.ExpandedIndices()
	if len(got) != 3 {
		t.Fatalf("ExpandAll in multiple mode should open every section, got %v", got)
	}
}

func TestWithCollapseAllEmptiesExpanded(t *testing.T) {
	m := New().WithAllowMultiple(true).WithSections(sample()...).WithExpandAll().WithCollapseAll()
	if len(m.ExpandedIndices()) != 0 {
		t.Fatalf("CollapseAll should leave expanded set empty, got %v", m.ExpandedIndices())
	}
}

func TestWithExpandedOutOfRangeIgnored(t *testing.T) {
	m := New().WithSections(sample()...).WithExpanded(-1, 5, 99)
	if len(m.ExpandedIndices()) != 0 {
		t.Fatalf("out-of-range expanded indices should be silently dropped, got %v", m.ExpandedIndices())
	}
}

func TestPlaceholderHonorsCustomText(t *testing.T) {
	m := New().WithPlaceholder("nothing yet")
	if !strings.Contains(m.View(), "nothing yet") {
		t.Fatalf("custom placeholder should appear in empty view, got %q", m.View())
	}
}

func TestWithThemeAppliesPalette(t *testing.T) {
	custom := theme.Default
	m := New().WithTheme(custom).WithSections(sample()...)
	out := m.View()
	if !strings.Contains(out, "Overview") {
		t.Fatalf("WithTheme must not break rendering, got %q", out)
	}
}
