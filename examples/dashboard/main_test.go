package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	textinput "github.com/truffle-dev/glyph/components/text-input"
	"github.com/truffle-dev/glyph/components/theme"
)

func resize(t *testing.T, m model, w, h int) model {
	t.Helper()
	mi, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return mi.(model)
}

func TestInitialRender(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	out := m.View()
	for _, want := range []string{"Engagements", "ACTIVE ENGAGEMENTS", "acme/widgets", "v0.2.0-dev", "filter"} {
		if !strings.Contains(out, want) {
			t.Fatalf("initial view missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestTabAdvancesAndSwapsCards(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mi.(model)
	if m.activeTab != tabThroughput {
		t.Fatalf("expected tabThroughput after one Tab, got %d", m.activeTab)
	}
	out := m.View()
	if !strings.Contains(out, "MERGED THIS WEEK") {
		t.Fatal("throughput cards should appear after switching tab")
	}
	if !strings.Contains(out, "PRs merged") {
		t.Fatal("throughput table column 'PRs merged' should appear")
	}
}

func TestShiftTabCyclesBackward(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = mi.(model)
	if m.activeTab != tabRevenue {
		t.Fatalf("expected tabRevenue (wrap-around), got %d", m.activeTab)
	}
}

func TestRevenueTabShowsMRR(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mi.(model)
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mi.(model)
	if m.activeTab != tabRevenue {
		t.Fatalf("expected tabRevenue, got %d", m.activeTab)
	}
	out := m.View()
	if !strings.Contains(out, "ARR") {
		t.Fatal("revenue tab should show ARR card")
	}
}

func TestEnterPushesToast(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	if got := len(m.tray.Toasts()); got != 0 {
		t.Fatalf("tray should start empty, got %d", got)
	}
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mi.(model)
	if got := len(m.tray.Toasts()); got != 1 {
		t.Fatalf("expected one toast after Enter, got %d", got)
	}
	if !strings.Contains(m.tray.Toasts()[0].Title, "Opened ") {
		t.Fatalf("toast title should start with 'Opened ', got %q", m.tray.Toasts()[0].Title)
	}
}

func TestSlashOpensFilterPrompt(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = mi.(model)
	if m.mode != modeFilterPrompt {
		t.Fatalf("expected modeFilterPrompt, got %d", m.mode)
	}
	if !m.filter.Focused() {
		t.Fatal("filter input should be focused after /")
	}
}

func TestEscClosesFilterPrompt(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = mi.(model)
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mi.(model)
	if m.mode != modeView {
		t.Fatalf("expected modeView after Esc, got %d", m.mode)
	}
}

func TestFilterSubmissionShrinksTable(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	before := len(m.tbl.Rows())
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = mi.(model)
	// Inject value directly (tea.SubmitMsg path lives inside textinput's Update;
	// in the dashboard we read .Value() on Enter).
	m.filter = m.filter.WithValue("jagdeep")
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mi.(model)
	if m.query != "jagdeep" {
		t.Fatalf("expected query='jagdeep', got %q", m.query)
	}
	after := len(m.tbl.Rows())
	if after >= before {
		t.Fatalf("expected fewer rows after filter (before=%d, after=%d)", before, after)
	}
	if after == 0 {
		t.Fatal("filter should still match at least one row")
	}
}

func TestToastTickDoesNotPanic(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mi.(model)
	mi, _ = m.Update(tickToastMsg(time.Now().Add(10 * time.Second)))
	m = mi.(model)
	if len(m.tray.Toasts()) != 0 {
		t.Fatal("toast should have expired after 10s tick")
	}
}

func TestQuitOnQ(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should issue tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q did not produce QuitMsg")
	}
}

func TestCtrlCQuitsFromAnyMode(t *testing.T) {
	for _, mode := range []mode{modeView, modeFilterPrompt} {
		m := resize(t, newModel(), 140, 36)
		m.mode = mode
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil {
			t.Fatalf("ctrl+c from mode=%d did not issue tea.Quit", mode)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("ctrl+c from mode=%d did not produce QuitMsg", mode)
		}
	}
}

// Confirm we route table keys through to table.Update when not in
// modeFilterPrompt — moving cursor should not throw.
func TestTableCursorRoutes(t *testing.T) {
	m := resize(t, newModel(), 140, 36)
	before := m.tbl.Cursor()
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mi.(model)
	after := m.tbl.Cursor()
	if after == before {
		t.Fatalf("Down should advance cursor (before=%d, after=%d)", before, after)
	}
}

// textinput sanity: ensure WithValue / Value round-trip works the way
// the dashboard relies on (we don't drive the SubmitMsg path).
func TestTextInputValueRoundTrip(t *testing.T) {
	ti := textinput.New(theme.Default).WithWidth(20).WithHeight(1)
	ti = ti.WithValue("hello")
	if ti.Value() != "hello" {
		t.Fatalf("expected 'hello', got %q", ti.Value())
	}
}
