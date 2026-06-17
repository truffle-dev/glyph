package multiselect

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func opts(labels ...string) []Option {
	out := make([]Option, len(labels))
	for i, l := range labels {
		out[i] = Option{Label: l}
	}
	return out
}

func sample() []Option {
	return []Option{
		{Label: "src/main.go", Hint: "go", Value: "main"},
		{Label: "src/util.go", Hint: "go", Value: "util"},
		{Label: "README.md", Hint: "docs", Value: "readme"},
		{Label: "go.mod", Hint: "mod", Value: "mod"},
		{Label: "go.sum", Hint: "mod", Value: "sum"},
	}
}

func space() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeySpace} }
func down() tea.KeyMsg  { return tea.KeyMsg{Type: tea.KeyDown} }

func TestNew_DefaultsAreSafe(t *testing.T) {
	m := New(theme.Default)
	if m.Count() != 0 {
		t.Fatalf("fresh MultiSelect should have zero checked, got %d", m.Count())
	}
	if out := m.View(); out == "" {
		t.Fatal("View should produce output even when empty")
	}
	// Tiny sizes clamp rather than honour verbatim.
	m2 := New(theme.Default).WithSize(0, 0)
	_ = m2.View()
	if m2.height < 1 {
		t.Fatalf("height should clamp to at least 1, got %d", m2.height)
	}
}

func TestSpace_TogglesRowUnderCursor(t *testing.T) {
	m := New(theme.Default).WithOptions(sample())
	m = m.toggleViaKey(space())
	if m.Count() != 1 {
		t.Fatalf("space should check the first row, got count %d", m.Count())
	}
	if got := m.SelectedValues(); len(got) != 1 || got[0] != "main" {
		t.Fatalf("expected [main] checked, got %v", got)
	}
	// Toggling the same row again unchecks it.
	m = m.toggleViaKey(space())
	if m.Count() != 0 {
		t.Fatalf("second space should uncheck, got count %d", m.Count())
	}
}

func TestTab_AlsoToggles(t *testing.T) {
	m := New(theme.Default).WithOptions(sample())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.Count() != 1 {
		t.Fatalf("Tab should toggle like Space, got count %d", m.Count())
	}
}

func TestConfirm_EmitsCheckedInOriginalOrder(t *testing.T) {
	m := New(theme.Default).WithOptions(sample())
	// Check rows 3 (go.mod) then 0 (main) — out of order on purpose.
	m, _ = m.Update(down())
	m, _ = m.Update(down())
	m, _ = m.Update(down())
	m, _ = m.Update(space()) // go.mod
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m, _ = m.Update(space()) // main

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should emit a ConfirmMsg")
	}
	msg, ok := cmd().(ConfirmMsg)
	if !ok {
		t.Fatalf("expected ConfirmMsg, got %T", cmd())
	}
	// Original order: main (idx 0) comes before mod (idx 3).
	want := []string{"main", "mod"}
	if len(msg.Values) != len(want) {
		t.Fatalf("expected %v, got %v", want, msg.Values)
	}
	for i := range want {
		if msg.Values[i] != want[i] {
			t.Fatalf("order wrong: expected %v, got %v", want, msg.Values)
		}
	}
	if len(msg.Selected) != 2 || msg.Selected[0].Label != "src/main.go" {
		t.Fatalf("Selected should mirror Values in order, got %+v", msg.Selected)
	}
}

func TestConfirm_EmptyIsAllowed(t *testing.T) {
	m := New(theme.Default).WithOptions(sample())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should emit a ConfirmMsg even with nothing checked")
	}
	msg := cmd().(ConfirmMsg)
	if len(msg.Values) != 0 {
		t.Fatalf("empty selection should confirm with no values, got %v", msg.Values)
	}
}

func TestEsc_EmitsCancelMsg(t *testing.T) {
	m := New(theme.Default).WithOptions(sample())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should produce a cmd")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Fatalf("Esc should emit CancelMsg, got %T", cmd())
	}
}

func TestSelectionSurvivesFiltering(t *testing.T) {
	m := New(theme.Default).WithOptions(sample()).WithFilter(true)
	// Filter to the go.* files, check go.mod, then change the filter so the
	// row is hidden, then clear it — the check must persist.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("go.")})
	// Visible now: src/main.go, src/util.go, go.mod, go.sum (all contain "go.").
	// Walk to go.mod and check it.
	for {
		o, ok := visAt(m)
		if !ok {
			t.Fatal("expected a visible row while filtered")
		}
		if o.Value == "mod" {
			break
		}
		m, _ = m.Update(down())
	}
	m, _ = m.Update(space())
	if m.Count() != 1 {
		t.Fatalf("expected go.mod checked, got count %d", m.Count())
	}
	// Now narrow the filter so go.mod is hidden.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("sum")})
	if m.Count() != 1 {
		t.Fatalf("hiding the row must not drop its check, got %d", m.Count())
	}
	// Clear the filter; the check should still be there.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	got := m.SelectedValues()
	if len(got) != 1 || got[0] != "mod" {
		t.Fatalf("check should survive filter cycling, got %v", got)
	}
}

func TestToggleAllVisible(t *testing.T) {
	// 'a' toggles every visible row when the filter is off.
	m := New(theme.Default).WithOptions(sample())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if m.Count() != len(sample()) {
		t.Fatalf("'a' should check all %d rows, got %d", len(sample()), m.Count())
	}
	// Pressing 'a' again clears them all.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if m.Count() != 0 {
		t.Fatalf("'a' again should clear all, got %d", m.Count())
	}
}

func TestToggleAllVisible_OnlyTouchesVisibleRows(t *testing.T) {
	// Check one row, then filter to a disjoint set and toggle-all-visible
	// off: the hidden check must remain. Uses a programmatic toggle since
	// 'a' feeds the filter when filtering is on.
	m := New(theme.Default).WithOptions(sample()).WithChecked([]string{"main"})
	m = m.WithFilter(true)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("readme")})
	m = m.toggleAllVisible() // checks README.md, leaves main untouched
	got := m.SelectedValues()
	if len(got) != 2 || got[0] != "main" || got[1] != "readme" {
		t.Fatalf("toggle-all-visible should add readme and keep main, got %v", got)
	}
}

func TestWithChecked_PreChecks(t *testing.T) {
	m := New(theme.Default).WithOptions(sample()).WithChecked([]string{"util", "sum"})
	if m.Count() != 2 {
		t.Fatalf("WithChecked should pre-check 2, got %d", m.Count())
	}
	got := m.SelectedValues()
	if len(got) != 2 || got[0] != "util" || got[1] != "sum" {
		t.Fatalf("expected [util sum] in order, got %v", got)
	}
}

func TestWithOptions_DropsStaleChecks(t *testing.T) {
	m := New(theme.Default).WithOptions(sample()).WithChecked([]string{"util", "sum"})
	// Replace options with a set that no longer contains "sum".
	m = m.WithOptions([]Option{{Label: "src/util.go", Value: "util"}, {Label: "new.go", Value: "new"}})
	if m.Count() != 1 {
		t.Fatalf("stale check 'sum' should be dropped, got count %d", m.Count())
	}
	if got := m.SelectedValues(); len(got) != 1 || got[0] != "util" {
		t.Fatalf("only the surviving check should remain, got %v", got)
	}
}

func TestValueFallsBackToLabel(t *testing.T) {
	m := New(theme.Default).WithOptions([]Option{{Label: "no-value-here"}})
	m, _ = m.Update(space())
	if got := m.SelectedValues(); len(got) != 1 || got[0] != "no-value-here" {
		t.Fatalf("value should fall back to label, got %v", got)
	}
}

func TestView_RendersCheckboxesTitleAndCount(t *testing.T) {
	m := New(theme.Default).
		WithOptions(sample()).
		WithSize(44, 6).
		WithTitle("Stage files")
	m, _ = m.Update(space()) // check first
	out := m.View()
	if !strings.Contains(out, "Stage files") {
		t.Errorf("View should contain the title, got %q", out)
	}
	if !strings.Contains(out, "1 selected") {
		t.Errorf("View should show the live count, got %q", out)
	}
	if !strings.Contains(out, "[x]") {
		t.Errorf("View should render a checked box, got %q", out)
	}
	if !strings.Contains(out, "[ ]") {
		t.Errorf("View should render unchecked boxes, got %q", out)
	}
	if !strings.Contains(out, "›") {
		t.Errorf("View should include a cursor marker, got %q", out)
	}
}

func TestUpdate_IgnoresNonKeyMessages(t *testing.T) {
	m := New(theme.Default).WithOptions(sample())
	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m2.Count() != m.Count() || m2.Cursor() != m.Cursor() {
		t.Fatal("non-key msg should not change state")
	}
	if cmd != nil {
		t.Fatal("non-key msg should not produce a cmd")
	}
}

func TestUpdate_FilterOnIgnoresVimAndAll(t *testing.T) {
	m := New(theme.Default).WithOptions(sample()).WithFilter(true)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	// 'a' fed the filter rather than toggling-all.
	if m.Count() != 0 {
		t.Fatalf("with filter on, 'a' should type, not toggle-all, got %d", m.Count())
	}
	if m.filter != "a" {
		t.Fatalf("expected filter 'a', got %q", m.filter)
	}
}

// toggleViaKey applies a key and returns the model, dropping the cmd.
func (m MultiSelect) toggleViaKey(k tea.KeyMsg) MultiSelect {
	out, _ := m.Update(k)
	return out
}

// visAt returns the option under the cursor in the visible list.
func visAt(m MultiSelect) (Option, bool) {
	vis := m.visible()
	if len(vis) == 0 || m.cursor < 0 || m.cursor >= len(vis) {
		return Option{}, false
	}
	return vis[m.cursor], true
}
