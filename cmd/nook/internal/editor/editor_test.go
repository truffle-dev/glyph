package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func runeMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestLoadMissingFileGivesEmptyBuffer(t *testing.T) {
	b, err := Load(filepath.Join(t.TempDir(), "noexist.txt"))
	if err != nil {
		t.Fatalf("expected nil err on missing file, got %v", err)
	}
	if len(b.Lines) != 1 || b.Lines[0] != "" {
		t.Fatalf("expected single empty line, got %+v", b.Lines)
	}
}

func TestLoadStripsTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	os.WriteFile(path, []byte("a\nb\nc\n"), 0o644)
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %+v", len(b.Lines), b.Lines)
	}
}

func TestInsertAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	p := NewPane(theme.Default).WithSize(60, 10).Focus().Open(path)
	p, _ = p.Update(runeMsg("hi"))
	if p.Line(0) != "hi" {
		t.Fatalf("expected 'hi', got %q", p.Line(0))
	}
	if !p.Dirty() {
		t.Fatal("expected dirty after insert")
	}
	cmd := p.SaveCmd()
	if cmd == nil {
		t.Fatal("expected SaveCmd")
	}
	msg := cmd()
	if sm, ok := msg.(SavedMsg); !ok || sm.Err != nil {
		t.Fatalf("expected SavedMsg with no err, got %+v", msg)
	}
	body, _ := os.ReadFile(path)
	if string(body) != "hi\n" {
		t.Fatalf("expected 'hi\\n', got %q", body)
	}
}

func TestNewlineSplitsLine(t *testing.T) {
	p := NewPane(theme.Default).Focus()
	p, _ = p.Update(runeMsg("hello"))
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	p, _ = p.Update(runeMsg("world"))
	if p.LineCount() != 2 {
		t.Fatalf("expected 2 lines, got %d", p.LineCount())
	}
	if p.Line(0) != "hello" || p.Line(1) != "world" {
		t.Fatalf("unexpected lines: %q | %q", p.Line(0), p.Line(1))
	}
}

func TestBackspaceMergesLines(t *testing.T) {
	p := NewPane(theme.Default).Focus()
	p, _ = p.Update(runeMsg("a"))
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	p, _ = p.Update(runeMsg("b"))
	// at end of line 1, backspace once: removes 'b'
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.Line(1) != "" {
		t.Fatalf("expected empty line 1, got %q", p.Line(1))
	}
	// backspace again merges lines
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.LineCount() != 1 {
		t.Fatalf("expected 1 line after merge, got %d", p.LineCount())
	}
	if p.Line(0) != "a" {
		t.Fatalf("expected 'a', got %q", p.Line(0))
	}
}

func TestCursorMovement(t *testing.T) {
	p := NewPane(theme.Default).Focus()
	p, _ = p.Update(runeMsg("abc"))
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	p, _ = p.Update(runeMsg("def"))
	// cursor is at (1, 3); go to (0, 1) via up + left + left
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.CursorRow() != 0 {
		t.Fatalf("expected row 0, got %d", p.CursorRow())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyHome})
	if p.CursorCol() != 0 {
		t.Fatalf("expected col 0 after Home, got %d", p.CursorCol())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if p.CursorCol() != 3 {
		t.Fatalf("expected col 3 after End, got %d", p.CursorCol())
	}
}

func TestJumpToScrolls(t *testing.T) {
	p := NewPane(theme.Default).WithSize(60, 5).Focus()
	for i := 0; i < 50; i++ {
		p, _ = p.Update(runeMsg("line"))
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
	p = p.JumpTo(40, 1)
	if p.CursorRow() != 39 {
		t.Fatalf("expected row 39, got %d", p.CursorRow())
	}
	out := p.View()
	if !strings.Contains(out, "40") {
		t.Fatalf("expected line 40 visible in view")
	}
}

func TestEscEmitsCancel(t *testing.T) {
	p := NewPane(theme.Default)
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected Cmd on Esc")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd())
	}
}

func TestCtrlSEmitsSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	p := NewPane(theme.Default).Focus().Open(path)
	p, _ = p.Update(runeMsg("z"))
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("expected SaveCmd on Ctrl+S")
	}
	msg := cmd()
	if sm, ok := msg.(SavedMsg); !ok || sm.Err != nil {
		t.Fatalf("expected SavedMsg no-err, got %+v", msg)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file written: %v", err)
	}
}

func TestViewRendersGutter(t *testing.T) {
	p := NewPane(theme.Default).WithSize(60, 8).Focus()
	p, _ = p.Update(runeMsg("hello"))
	out := p.View()
	if !strings.Contains(out, "editor") {
		t.Fatalf("expected editor header in view:\n%s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected hello in view:\n%s", out)
	}
	if !strings.Contains(out, "│") {
		t.Fatalf("expected gutter separator:\n%s", out)
	}
}

func TestDirtyIndicator(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	p := NewPane(theme.Default).Focus().Open(path)
	if p.Dirty() {
		t.Fatal("expected clean after open")
	}
	p, _ = p.Update(runeMsg("a"))
	if !p.Dirty() {
		t.Fatal("expected dirty after insert")
	}
	cmd := p.SaveCmd()
	cmd()
	p = p.ApplySave()
	if p.Dirty() {
		t.Fatal("expected clean after ApplySave")
	}
}
