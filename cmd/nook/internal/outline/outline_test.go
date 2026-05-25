package outline

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/components/theme"
)

// fixtureFile returns a small mock symbol tree: one top-level function
// (main, lines 5..7) and one struct (Server, lines 10..50) that owns
// two methods (Listen, lines 15..25; Close, lines 27..40). Used by
// multiple tests below.
func fixtureFile() []lsp.DocSymbol {
	return []lsp.DocSymbol{
		{Name: "main", Kind: lsp.WorkspaceSymbolKindFunction, Line: 5, Col: 5, EndLine: 7},
		{
			Name: "Server", Kind: lsp.WorkspaceSymbolKindStruct, Line: 10, Col: 5, EndLine: 50,
			Children: []lsp.DocSymbol{
				{Name: "Listen", Kind: lsp.WorkspaceSymbolKindMethod, Line: 15, Col: 15, EndLine: 25},
				{Name: "Close", Kind: lsp.WorkspaceSymbolKindMethod, Line: 27, Col: 15, EndLine: 40},
			},
		},
	}
}

func TestFlattenDepthDFS(t *testing.T) {
	t.Parallel()
	got := Flatten(fixtureFile())
	want := []struct {
		name  string
		depth int
	}{
		{"main", 0},
		{"Server", 0},
		{"Listen", 1},
		{"Close", 1},
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Name != w.name || got[i].Depth != w.depth {
			t.Errorf("got[%d] = {%s,%d}, want {%s,%d}", i, got[i].Name, got[i].Depth, w.name, w.depth)
		}
	}
}

func TestEnclosingIndex(t *testing.T) {
	t.Parallel()
	flat := Flatten(fixtureFile())
	cases := []struct {
		row  int
		want string // expected enclosing symbol name
	}{
		{0, "main"},    // outside any range falls back to index 0
		{5, "main"},    // start of main
		{6, "main"},    // inside main body
		{8, "main"},    // gap between symbols also returns 0 (best fallback)
		{10, "Server"}, // start of Server
		{20, "Listen"}, // inside method picks deepest
		{30, "Close"},  // inside other method
		{45, "Server"}, // inside struct but outside methods → Server
		{49, "Server"}, // last line of struct
	}
	for _, tc := range cases {
		idx := EnclosingIndex(flat, tc.row)
		if flat[idx].Name != tc.want {
			t.Errorf("row %d: got %q, want %q", tc.row, flat[idx].Name, tc.want)
		}
	}
}

func TestPaneOpenPositionsCursorAtEnclosingSymbol(t *testing.T) {
	t.Parallel()
	p := New(theme.Default).WithSize(60, 18)
	p = p.Open("/x/y/z.go", fixtureFile(), 20) // cursor on Listen's line
	s, ok := p.Highlighted()
	if !ok {
		t.Fatal("Highlighted returned false after Open with non-empty syms")
	}
	if s.Name != "Listen" {
		t.Errorf("highlighted = %q, want Listen", s.Name)
	}
}

func TestPaneFilterNarrowsList(t *testing.T) {
	t.Parallel()
	p := New(theme.Default).WithSize(60, 18)
	p = p.Open("/x.go", fixtureFile(), 0)
	if p.Count() != 4 {
		t.Fatalf("unfiltered count = %d, want 4", p.Count())
	}
	// Type "li" → only "Listen" should match.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("li")})
	if p.Filter() != "li" {
		t.Errorf("filter = %q, want li", p.Filter())
	}
	if p.Count() != 1 {
		t.Fatalf("filtered count = %d, want 1", p.Count())
	}
	s, _ := p.Highlighted()
	if s.Name != "Listen" {
		t.Errorf("highlighted = %q, want Listen", s.Name)
	}
	// Backspace once → "l" → matches Listen + Close (both contain 'l'/'L').
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.Filter() != "l" {
		t.Errorf("after backspace filter = %q, want l", p.Filter())
	}
	if p.Count() != 2 {
		t.Errorf("filtered count after backspace = %d, want 2", p.Count())
	}
}

func TestPaneCtrlUClearsFilter(t *testing.T) {
	t.Parallel()
	p := New(theme.Default).WithSize(60, 18)
	p = p.Open("/x.go", fixtureFile(), 0)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("listen")})
	if p.Filter() != "listen" {
		t.Fatalf("setup: filter = %q", p.Filter())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if p.Filter() != "" {
		t.Errorf("after Ctrl+U filter = %q, want empty", p.Filter())
	}
	if p.Count() != 4 {
		t.Errorf("Count after Ctrl+U = %d, want 4", p.Count())
	}
}

func TestPaneArrowMovesCursor(t *testing.T) {
	t.Parallel()
	p := New(theme.Default).WithSize(60, 18)
	p = p.Open("/x.go", fixtureFile(), 0) // cursor on main
	// Down → Server, Listen, Close
	for i, want := range []string{"Server", "Listen", "Close"} {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
		got, ok := p.Highlighted()
		if !ok || got.Name != want {
			t.Errorf("after %d downs: got %v, want %s", i+1, got, want)
		}
	}
	// One more Down — should clamp.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	got, _ := p.Highlighted()
	if got.Name != "Close" {
		t.Errorf("Down past end: got %s, want Close (clamped)", got.Name)
	}
	// Up back to main.
	for i := 0; i < 4; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	got, _ = p.Highlighted()
	if got.Name != "main" {
		t.Errorf("after 4 ups: got %s, want main", got.Name)
	}
}

func TestPaneEnterEmitsJumpMsg(t *testing.T) {
	t.Parallel()
	p := New(theme.Default).WithSize(60, 18)
	p = p.Open("/x.go", fixtureFile(), 20) // cursor on Listen
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.IsOpen() {
		t.Error("Pane should close after Enter")
	}
	if cmd == nil {
		t.Fatal("Enter should return a tea.Cmd")
	}
	msg := cmd()
	jm, ok := msg.(JumpMsg)
	if !ok {
		t.Fatalf("got %T, want JumpMsg", msg)
	}
	if jm.Path != "/x.go" || jm.Row != 15 || jm.Col != 15 {
		t.Errorf("JumpMsg = %+v, want {/x.go,15,15}", jm)
	}
}

func TestPaneEscEmitsCancelMsg(t *testing.T) {
	t.Parallel()
	p := New(theme.Default).WithSize(60, 18)
	p = p.Open("/x.go", fixtureFile(), 0)
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.IsOpen() {
		t.Error("Pane should close after Esc")
	}
	if cmd == nil {
		t.Fatal("Esc should return a tea.Cmd")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Errorf("got %T, want CancelMsg", cmd())
	}
}

func TestPaneOpenErrorPath(t *testing.T) {
	t.Parallel()
	p := New(theme.Default).WithSize(60, 18)
	p = p.OpenError("/x.go", "no language server attached")
	if !p.IsOpen() {
		t.Fatal("OpenError should leave pane open")
	}
	view := p.View()
	if !strings.Contains(view, "no language server attached") {
		t.Errorf("View missing error text:\n%s", view)
	}
	// Esc still closes from the error path.
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.IsOpen() {
		t.Error("Esc should close error-state pane")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Error("Esc from error state should still emit CancelMsg")
	}
}

func TestPaneEmptySymbolsRenders(t *testing.T) {
	t.Parallel()
	p := New(theme.Default).WithSize(60, 18)
	p = p.Open("/empty.go", nil, 0)
	view := p.View()
	if !strings.Contains(view, "no symbols") {
		t.Errorf("View missing 'no symbols' hint:\n%s", view)
	}
	// Enter is a no-op when nothing is highlighted.
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !p.IsOpen() {
		t.Error("Enter on empty symbols list should not close pane")
	}
	if cmd != nil {
		t.Errorf("Enter on empty list should return nil cmd, got %T", cmd())
	}
}

func TestPaneViewClosedReturnsEmpty(t *testing.T) {
	t.Parallel()
	p := New(theme.Default)
	if v := p.View(); v != "" {
		t.Errorf("closed pane View = %q, want empty", v)
	}
}

func TestPanePageDownAdvancesByVisibleRows(t *testing.T) {
	t.Parallel()
	// Build a synthetic 30-symbol tree so PgDown can move further than
	// the visible window.
	syms := make([]lsp.DocSymbol, 30)
	for i := range syms {
		syms[i] = lsp.DocSymbol{
			Name:    "sym" + itoa(i),
			Kind:    lsp.WorkspaceSymbolKindFunction,
			Line:    i * 2,
			EndLine: i*2 + 1,
		}
	}
	p := New(theme.Default).WithSize(60, 12).Open("/big.go", syms, 0)
	if h, _ := p.Highlighted(); h.Name != "sym0" {
		t.Fatalf("start: got %s, want sym0", h.Name)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	got, _ := p.Highlighted()
	// height=12 means listHeight=8, so PgDown moves cursor by 8.
	if got.Name != "sym8" {
		t.Errorf("after PgDown: got %s, want sym8", got.Name)
	}
}
