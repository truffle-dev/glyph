package paginationbar

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestNewIsSinglePage(t *testing.T) {
	m := New()
	if m.Total() != 1 {
		t.Errorf("New().Total() = %d, want 1", m.Total())
	}
	if m.Page() != 0 {
		t.Errorf("New().Page() = %d, want 0", m.Page())
	}
	if m.PageNumber() != 1 {
		t.Errorf("PageNumber should be 1-indexed, got %d", m.PageNumber())
	}
	if !m.AtStart() || !m.AtEnd() {
		t.Errorf("single-page bar should be both AtStart and AtEnd, got start=%v end=%v", m.AtStart(), m.AtEnd())
	}
}

func TestWithTotalClampsToOne(t *testing.T) {
	m := New().WithTotal(0)
	if m.Total() != 1 {
		t.Errorf("WithTotal(0) should clamp to 1, got %d", m.Total())
	}
	m = New().WithTotal(-5)
	if m.Total() != 1 {
		t.Errorf("WithTotal(-5) should clamp to 1, got %d", m.Total())
	}
}

func TestWithTotalClampsPageInRange(t *testing.T) {
	m := New().WithTotal(10).WithPage(7)
	m = m.WithTotal(3)
	if m.Page() != 2 {
		t.Errorf("Page should clamp to last when total shrinks, got %d", m.Page())
	}
}

func TestWithPageClampsOutOfRange(t *testing.T) {
	m := New().WithTotal(5)
	if got := m.WithPage(-3).Page(); got != 0 {
		t.Errorf("negative page should clamp to 0, got %d", got)
	}
	if got := m.WithPage(99).Page(); got != 4 {
		t.Errorf("over-range page should clamp to last, got %d", got)
	}
}

func TestUpdateRightAdvances(t *testing.T) {
	m := New().WithTotal(5)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Page() != 1 {
		t.Errorf("right should advance to page 1, got %d", m.Page())
	}
	if cmd == nil {
		t.Fatal("page change should emit a tea.Cmd")
	}
	msg, ok := cmd().(PageChangedMsg)
	if !ok {
		t.Fatalf("expected PageChangedMsg, got %T", cmd())
	}
	if msg.Page != 1 || msg.Total != 5 {
		t.Errorf("PageChangedMsg = %+v, want {1, 5}", msg)
	}
}

func TestUpdateLeftAtZeroIsNoOp(t *testing.T) {
	m := New().WithTotal(5)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.Page() != 0 {
		t.Errorf("left at page 0 should stay at 0 (no wrap), got %d", m.Page())
	}
	if cmd != nil {
		t.Fatal("no page change should mean no tea.Cmd")
	}
}

func TestUpdateRightAtEndIsNoOp(t *testing.T) {
	m := New().WithTotal(3).WithPage(2)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Page() != 2 {
		t.Errorf("right at last page should stay (no wrap), got %d", m.Page())
	}
	if cmd != nil {
		t.Fatal("no page change should mean no tea.Cmd")
	}
}

func TestUpdateWrapsWhenEnabled(t *testing.T) {
	m := New().WithTotal(3).WithWrap(true)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.Page() != 2 {
		t.Errorf("left from 0 should wrap to 2, got %d", m.Page())
	}
	if cmd == nil {
		t.Fatal("wrap-around motion should still emit PageChangedMsg")
	}
	m = m.WithPage(2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Page() != 0 {
		t.Errorf("right from last should wrap to 0, got %d", m.Page())
	}
}

func TestUpdateHomeAndEndKeys(t *testing.T) {
	m := New().WithTotal(7).WithPage(3)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.Page() != 6 {
		t.Errorf("End should jump to last page, got %d", m.Page())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.Page() != 0 {
		t.Errorf("Home should jump to first page, got %d", m.Page())
	}
}

func TestUpdateVimKeys(t *testing.T) {
	m := New().WithTotal(5)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.Page() != 1 {
		t.Errorf("l should advance, got %d", m.Page())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.Page() != 0 {
		t.Errorf("h should retreat, got %d", m.Page())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if m.Page() != 4 {
		t.Errorf("G should jump to last, got %d", m.Page())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if m.Page() != 0 {
		t.Errorf("g should jump to first, got %d", m.Page())
	}
}

func TestUpdateIgnoresUnrelatedKeys(t *testing.T) {
	m := New().WithTotal(3).WithPage(1)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Fatal("unrelated keys should not emit a cmd")
	}
}

func TestUpdateIgnoresNonKeyMessages(t *testing.T) {
	m := New().WithTotal(3)
	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m2.Page() != m.Page() {
		t.Errorf("non-key message should not change page")
	}
	if cmd != nil {
		t.Fatal("non-key message should not emit a cmd")
	}
}

func TestViewIncludesPageOfTotal(t *testing.T) {
	m := New().WithTotal(7).WithPage(2)
	out := m.View()
	if !strings.Contains(out, "3 of 7") {
		t.Errorf("View should include 1-indexed label '3 of 7', got %q", out)
	}
	if !strings.Contains(out, prevGlyph) || !strings.Contains(out, nextGlyph) {
		t.Errorf("View should include both chevrons, got %q", out)
	}
}

func TestViewIncludesItemsSuffix(t *testing.T) {
	m := New().WithTotal(3).WithTotalItems(42)
	out := m.View()
	if !strings.Contains(out, "(42 items)") {
		t.Errorf("View should include item count suffix, got %q", out)
	}
}

func TestViewCustomItemLabel(t *testing.T) {
	m := New().WithTotal(3).WithTotalItems(7).WithItemsLabel("results")
	out := m.View()
	if !strings.Contains(out, "(7 results)") {
		t.Errorf("View should honor custom items label, got %q", out)
	}
}

func TestViewPrefixRenders(t *testing.T) {
	m := New().WithTotal(2).WithPrefix("Results ")
	out := m.View()
	if !strings.HasPrefix(stripANSI(out), "Results ") {
		t.Errorf("View should start with prefix, got %q", out)
	}
}

func TestPerPageDerivesTotal(t *testing.T) {
	m := New().WithTotalItems(50).WithPerPage(7)
	if m.Total() != 8 {
		t.Errorf("50 items / 7 per page should derive 8 pages, got %d", m.Total())
	}
}

func TestPerPageWithExactDivide(t *testing.T) {
	m := New().WithTotalItems(20).WithPerPage(5)
	if m.Total() != 4 {
		t.Errorf("20 items / 5 per page should derive 4 pages, got %d", m.Total())
	}
}

func TestPerPageZeroItemsKeepsOnePage(t *testing.T) {
	m := New().WithTotalItems(0).WithPerPage(10)
	if m.Total() != 1 {
		t.Errorf("0 items should keep total at 1 (empty single page), got %d", m.Total())
	}
}

func TestVisibleRangeWithoutPerPageReturnsZero(t *testing.T) {
	m := New().WithTotal(5).WithTotalItems(100)
	start, end := m.VisibleRange()
	if start != 0 || end != 0 {
		t.Errorf("VisibleRange without per-page should return (0, 0), got (%d, %d)", start, end)
	}
}

func TestVisibleRangeOnMiddlePage(t *testing.T) {
	m := New().WithTotalItems(50).WithPerPage(10).WithPage(2)
	start, end := m.VisibleRange()
	if start != 20 || end != 30 {
		t.Errorf("page 2 of 10-per-page should yield [20, 30), got [%d, %d)", start, end)
	}
}

func TestVisibleRangeOnLastPartialPage(t *testing.T) {
	m := New().WithTotalItems(23).WithPerPage(10).WithPage(2)
	start, end := m.VisibleRange()
	if start != 20 || end != 23 {
		t.Errorf("partial last page should clamp end to total items, got [%d, %d)", start, end)
	}
}

func TestAtStartAndAtEnd(t *testing.T) {
	m := New().WithTotal(5)
	if !m.AtStart() {
		t.Errorf("page 0 should be AtStart")
	}
	if m.AtEnd() {
		t.Errorf("page 0 should not be AtEnd with total 5")
	}
	m = m.WithPage(4)
	if m.AtStart() {
		t.Errorf("page 4 should not be AtStart")
	}
	if !m.AtEnd() {
		t.Errorf("page 4 should be AtEnd with total 5")
	}
}

func TestPageChangedMsgNotEmittedOnNoOp(t *testing.T) {
	m := New().WithTotal(1)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if cmd != nil {
		t.Fatal("single-page right should not emit PageChangedMsg")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if cmd != nil {
		t.Fatal("Home at page 0 should not emit PageChangedMsg")
	}
}

func TestWithThemeAppliesPalette(t *testing.T) {
	m := New().WithTheme(theme.Default).WithTotal(3).WithPage(1)
	if !strings.Contains(m.View(), "2 of 3") {
		t.Errorf("WithTheme should not break rendering")
	}
}

func TestWithWidthDoesNotCorruptOutput(t *testing.T) {
	m := New().WithTotal(3).WithPage(1).WithWidth(40)
	if !strings.Contains(m.View(), "2 of 3") {
		t.Errorf("WithWidth should not drop the page label")
	}
}

// stripANSI removes ANSI escape sequences for prefix-matching tests.
func stripANSI(s string) string {
	out := ""
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		out += string(s[i])
	}
	return out
}
