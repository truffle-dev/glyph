package tablevirtualized

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func cols() []Column {
	return []Column{
		{Key: "id", Title: "ID", Width: 6, Align: AlignRight},
		{Key: "name", Title: "Name", Width: 12},
	}
}

func sliceOf(n int) SliceProvider {
	rows := make([]Row, n)
	for i := range rows {
		rows[i] = Row{Cells: []string{fmt.Sprintf("%d", i), fmt.Sprintf("row-%d", i)}}
	}
	return SliceProvider(rows)
}

func TestEmptyRenders(t *testing.T) {
	m := New().WithColumns(cols()...)
	out := m.View()
	if !strings.Contains(out, "ID") || !strings.Contains(out, "Name") {
		t.Fatalf("header missing: %q", out)
	}
	if !strings.Contains(out, "no rows") {
		t.Fatalf("placeholder missing: %q", out)
	}
}

func TestCursorMovementAndClamp(t *testing.T) {
	m := New().WithColumns(cols()...).WithRows(sliceOf(10)).WithSize(40, 6)
	// height 6 → visible 4 rows.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Fatalf("cursor want 2, got %d", m.Cursor())
	}
	// End jumps to last row.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.Cursor() != 9 {
		t.Fatalf("cursor at end want 9, got %d", m.Cursor())
	}
	// Going further down clamps.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 9 {
		t.Fatalf("cursor past end want clamp to 9, got %d", m.Cursor())
	}
	// Home jumps back to 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.Cursor() != 0 {
		t.Fatalf("cursor at home want 0, got %d", m.Cursor())
	}
}

func TestSelectMsgOnEnter(t *testing.T) {
	m := New().
		WithColumns(cols()...).
		WithRows(sliceOf(3)).
		WithSize(40, 6).
		WithRowSelection(true).
		WithSelectedRow(2)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected SelectMsg cmd")
	}
	msg := cmd()
	sel, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", msg)
	}
	if sel.Index != 2 {
		t.Fatalf("index want 2, got %d", sel.Index)
	}
	if got := sel.Row.Cells[0]; got != "2" {
		t.Fatalf("row cell want 2, got %q", got)
	}
}

func TestScrollIndicatorsAppearOnLargeDataset(t *testing.T) {
	m := New().WithColumns(cols()...).WithRows(sliceOf(100)).WithSize(40, 6)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	out := m.View()
	if !strings.Contains(out, arrowUp) {
		t.Fatalf("expected up arrow when scrolled past top: %q", out)
	}
	// Reset to top, expect down arrow but no up arrow.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	out = m.View()
	if strings.Contains(out, arrowUp) {
		t.Fatalf("did not expect up arrow at top of dataset: %q", out)
	}
	if !strings.Contains(out, arrowDown) {
		t.Fatalf("expected down arrow when more rows below: %q", out)
	}
}

// countingProvider proves that View() touches only the visible window
// of rows, not every row in the provider. The point of virtualization.
type countingProvider struct {
	n    int
	hits int64
}

func (c *countingProvider) Len() int { return c.n }

func (c *countingProvider) At(i int) Row {
	atomic.AddInt64(&c.hits, 1)
	return Row{Cells: []string{fmt.Sprintf("%d", i), fmt.Sprintf("v-%d", i)}}
}

func TestRenderTouchesOnlyVisibleRows(t *testing.T) {
	cp := &countingProvider{n: 10_000_000}
	m := New().WithColumns(cols()...).WithRows(cp).WithSize(40, 12)
	_ = m.View()
	hits := atomic.LoadInt64(&cp.hits)
	// height 12 → visible 10 rows. Allow a small safety margin in case a
	// future maintainer adds a peek-ahead for scroll indicators.
	if hits > 12 {
		t.Fatalf("expected ≤12 At() calls, got %d", hits)
	}
	if hits == 0 {
		t.Fatalf("expected some At() calls, got 0")
	}
}

func TestPageDownAdvancesByVisibleCount(t *testing.T) {
	m := New().WithColumns(cols()...).WithRows(sliceOf(100)).WithSize(40, 7)
	// height 7 → visible 5 rows.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.Cursor() != 5 {
		t.Fatalf("pgdown want 5, got %d", m.Cursor())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.Cursor() != 10 {
		t.Fatalf("pgdown twice want 10, got %d", m.Cursor())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.Cursor() != 5 {
		t.Fatalf("pgup want 5, got %d", m.Cursor())
	}
}

func TestWidthClampOnUnderdeclaredColumn(t *testing.T) {
	m := New().WithColumns(Column{Key: "x", Title: "X", Width: 1}).WithRows(sliceOf(1))
	if got := m.columns[0].Width; got != 4 {
		t.Fatalf("width clamp want 4, got %d", got)
	}
}

func TestSelectedRowAccess(t *testing.T) {
	m := New().WithColumns(cols()...).WithRows(sliceOf(5)).WithSelectedRow(3)
	row, ok := m.SelectedRow()
	if !ok {
		t.Fatalf("expected SelectedRow ok")
	}
	if row.Cells[0] != "3" {
		t.Fatalf("selected row cell want 3, got %q", row.Cells[0])
	}
}

func TestNilProviderSafe(t *testing.T) {
	m := New().WithColumns(cols()...)
	if m.rowsLen() != 0 {
		t.Fatalf("nil provider should report 0 rows, got %d", m.rowsLen())
	}
	if _, ok := m.SelectedRow(); ok {
		t.Fatalf("nil provider should not return a row")
	}
	out := m.View()
	if !strings.Contains(out, "no rows") {
		t.Fatalf("expected placeholder: %q", out)
	}
}
