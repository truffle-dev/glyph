package commandpalette

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func sample() []Command {
	return []Command{
		{ID: "open", Title: "Open file", Group: "File", Keybinding: "ctrl+o"},
		{ID: "save", Title: "Save file", Group: "File", Keybinding: "ctrl+s"},
		{ID: "quit", Title: "Quit", Group: "App", Keybinding: "ctrl+q"},
		{ID: "find", Title: "Find in files", Group: "Search", Keybinding: "ctrl+f"},
	}
}

func TestEmptyFilterShowsAllCommands(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12)
	out := p.View()
	for _, want := range []string{"Open file", "Save file", "Quit", "Find in files"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in view, got %q", want, out)
		}
	}
}

func TestFilterNarrowsResults(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12).WithFilter("find")
	out := p.View()
	if !strings.Contains(out, "Find in files") {
		t.Errorf("expected 'Find in files' to remain after filter, got %q", out)
	}
	if strings.Contains(out, "Quit") {
		t.Errorf("expected 'Quit' to be filtered out, got %q", out)
	}
}

func TestNoMatchesShowsEmptyState(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12).WithFilter("zzz")
	out := p.View()
	if !strings.Contains(out, "No commands match") {
		t.Errorf("expected empty-state message, got %q", out)
	}
}

func TestEnterEmitsSelectMsgForCursor(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12)
	// Cursor starts at 0 (which is "Open file" after sort, highest substring match for empty query).
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a tea.Cmd on enter")
	}
	msg := cmd()
	sel, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", msg)
	}
	if sel.Command.ID == "" {
		t.Errorf("expected a non-empty command ID, got %q", sel.Command.ID)
	}
}

func TestEnterIgnoredWhenNoMatches(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12).WithFilter("zzz")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected no command on enter with empty match list, got %v", cmd())
	}
}

func TestEscEmitsCancelMsg(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12)
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a tea.Cmd on esc")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Errorf("expected CancelMsg, got %T", cmd())
	}
}

func TestDownArrowMovesCursorDown(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12)
	before := p.cursor
	next, _ := p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if next.cursor != before+1 {
		t.Errorf("expected cursor to advance to %d, got %d", before+1, next.cursor)
	}
}

func TestUpArrowClampsAtZero(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12)
	next, _ := p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if next.cursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", next.cursor)
	}
}

func TestDownArrowClampsAtEnd(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12)
	for i := 0; i < 100; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if p.cursor != len(sample())-1 {
		t.Errorf("expected cursor clamped at %d, got %d", len(sample())-1, p.cursor)
	}
}

func TestTypingRunesUpdatesFilter(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("sa")})
	if p.filter != "sa" {
		t.Errorf("expected filter 'sa', got %q", p.filter)
	}
	out := p.View()
	if !strings.Contains(out, "Save file") {
		t.Errorf("expected 'Save file' after typing 'sa', got %q", out)
	}
	if strings.Contains(out, "Quit") {
		t.Errorf("expected 'Quit' filtered out after typing 'sa', got %q", out)
	}
}

func TestBackspaceTrimsFilter(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12).WithFilter("save")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.filter != "sav" {
		t.Errorf("expected filter 'sav', got %q", p.filter)
	}
}

func TestCtrlUClearsFilter(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12).WithFilter("anything")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if p.filter != "" {
		t.Errorf("expected empty filter, got %q", p.filter)
	}
}

func TestCustomMatcherIsUsed(t *testing.T) {
	// Custom matcher: only "quit" passes, scored 1.
	only := func(cmd Command, _ string) int {
		if cmd.ID == "quit" {
			return 1
		}
		return 0
	}
	p := New(theme.Default).WithCommands(sample()).WithSize(50, 12).WithMatcher(only)
	out := p.View()
	if !strings.Contains(out, "Quit") {
		t.Errorf("expected 'Quit' to pass custom matcher, got %q", out)
	}
	if strings.Contains(out, "Open file") {
		t.Errorf("expected 'Open file' filtered by custom matcher, got %q", out)
	}
}

func TestSubstringMatcherEmptyQueryPasses(t *testing.T) {
	if SubstringMatcher(Command{Title: "x"}, "") == 0 {
		t.Error("expected empty query to pass any command")
	}
}

func TestSubstringMatcherCaseInsensitive(t *testing.T) {
	if SubstringMatcher(Command{Title: "Open file"}, "OPEN") == 0 {
		t.Error("expected case-insensitive match")
	}
}

func TestKeybindingRendered(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(60, 12)
	out := p.View()
	if !strings.Contains(out, "ctrl+o") {
		t.Errorf("expected keybinding 'ctrl+o' in view, got %q", out)
	}
}

func TestGroupHeaderRendered(t *testing.T) {
	p := New(theme.Default).WithCommands(sample()).WithSize(60, 12)
	out := p.View()
	if !strings.Contains(out, "FILE") {
		t.Errorf("expected uppercased 'FILE' group header in view, got %q", out)
	}
}
