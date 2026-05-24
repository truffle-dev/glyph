package editor

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/truffle-dev/glyph/cmd/nook/internal/highlight"
	"github.com/truffle-dev/glyph/components/theme"
)

// ansiRE strips lipgloss/termenv escape sequences so substring asserts work
// on the rendered output regardless of how many style chunks the row splits
// into.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return ansiRE.ReplaceAllString(s, "") }

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

func TestSetLineReplacesAndPreservesIndent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	os.WriteFile(path, []byte("package main\n\n    fmt.Println(\"old\")\n"), 0o644)
	p := NewPane(theme.Default).Focus().Open(path)
	if p.Dirty() {
		t.Fatal("expected clean after open")
	}
	// Replace the 3rd line (index 2). Model omits indentation; we should re-attach.
	p = p.SetLine(2, "fmt.Println(\"new\")")
	if !p.Dirty() {
		t.Fatal("expected dirty after SetLine")
	}
	got := p.Line(2)
	want := "    fmt.Println(\"new\")"
	if got != want {
		t.Fatalf("expected indent re-attached, got %q want %q", got, want)
	}
}

func TestSetLineRespectsExplicitIndent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	os.WriteFile(path, []byte("    x := 1\n"), 0o644)
	p := NewPane(theme.Default).Focus().Open(path)
	p = p.SetLine(0, "\tx := 2")
	if got := p.Line(0); got != "\tx := 2" {
		t.Fatalf("expected explicit indent honored, got %q", got)
	}
}

func TestReplaceAllFromStringResetsBuffer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	os.WriteFile(path, []byte("a\nb\nc\n"), 0o644)
	p := NewPane(theme.Default).Focus().Open(path)
	p = p.ReplaceAllFromString("x\ny\n")
	if p.LineCount() != 2 {
		t.Fatalf("expected 2 lines, got %d", p.LineCount())
	}
	if p.Line(0) != "x" || p.Line(1) != "y" {
		t.Fatalf("ReplaceAllFromString contents wrong: %q %q", p.Line(0), p.Line(1))
	}
	if !p.Dirty() {
		t.Fatal("expected dirty after ReplaceAllFromString")
	}
}

func TestContentsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644)
	p := NewPane(theme.Default).Focus().Open(path)
	got := p.Contents()
	want := "package main\n\nfunc main() {}"
	if got != want {
		t.Fatalf("Contents mismatch:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestLinePrefixAndSuffix(t *testing.T) {
	p := NewPane(theme.Default).Focus()
	p, _ = p.Update(runeMsg("fmt.Println"))
	// cursor is at col 11 (end of "fmt.Println")
	if got := p.LinePrefix(); got != "fmt.Println" {
		t.Fatalf("expected prefix 'fmt.Println', got %q", got)
	}
	if got := p.LineSuffix(); got != "" {
		t.Fatalf("expected empty suffix, got %q", got)
	}
	// move cursor back 4 columns to land at col 7 (mid-word)
	for i := 0; i < 4; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyLeft})
	}
	if got := p.LinePrefix(); got != "fmt.Pri" {
		t.Fatalf("expected prefix 'fmt.Pri', got %q", got)
	}
	if got := p.LineSuffix(); got != "ntln" {
		t.Fatalf("expected suffix 'ntln', got %q", got)
	}
}

func TestSetAndGhostText(t *testing.T) {
	p := NewPane(theme.Default).Focus()
	p = p.SetGhostText("ntln(\"hi\")")
	if got := p.GhostText(); got != "ntln(\"hi\")" {
		t.Fatalf("expected ghost text round-trip, got %q", got)
	}
	p = p.SetGhostText("")
	if got := p.GhostText(); got != "" {
		t.Fatalf("expected empty ghost after clear, got %q", got)
	}
}

func TestInsertTextSingleLine(t *testing.T) {
	p := NewPane(theme.Default).Focus()
	p, _ = p.Update(runeMsg("fmt.Pri"))
	p = p.InsertText("ntln(\"hi\")")
	if got := p.Line(0); got != "fmt.Println(\"hi\")" {
		t.Fatalf("expected merged line, got %q", got)
	}
	if p.CursorCol() != len("fmt.Println(\"hi\")") {
		t.Fatalf("expected cursor at end, got col %d", p.CursorCol())
	}
}

func TestInsertTextMultiLine(t *testing.T) {
	p := NewPane(theme.Default).Focus()
	p = p.InsertText("a\nb\nc")
	if p.LineCount() != 3 {
		t.Fatalf("expected 3 lines, got %d", p.LineCount())
	}
	if p.Line(0) != "a" || p.Line(1) != "b" || p.Line(2) != "c" {
		t.Fatalf("multi-line insert wrong: %q %q %q", p.Line(0), p.Line(1), p.Line(2))
	}
}

func TestGhostTextRendersWhenFocused(t *testing.T) {
	p := NewPane(theme.Default).WithSize(60, 6).Focus()
	p, _ = p.Update(runeMsg("fmt.Pri"))
	p = p.SetGhostText("ntln")
	out := p.View()
	// First ghost rune is cursor-styled, the rest is muted-faint. The "tln"
	// tail renders contiguously after the styled cursor escape; check both
	// the prefix and that ghost tail.
	if !strings.Contains(out, "fmt.Pri") {
		t.Fatalf("expected prefix in view:\n%s", out)
	}
	if !strings.Contains(out, "tln") {
		t.Fatalf("expected ghost tail rendered in view:\n%s", out)
	}
}

func TestGhostTextHiddenWhenBlurred(t *testing.T) {
	p := NewPane(theme.Default).WithSize(60, 6).Focus()
	p, _ = p.Update(runeMsg("fmt.Pri"))
	p = p.SetGhostText("ntln")
	p = p.Blur()
	out := p.View()
	if strings.Contains(out, "tln") {
		t.Fatalf("expected ghost tail hidden when blurred:\n%s", out)
	}
}

func TestHighlightingWiringPreservesPlainText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.go")
	os.WriteFile(path, []byte("package main\n\nfunc Foo() {}\n"), 0o644)
	p := NewPane(theme.Default).WithSize(80, 8).WithHighlighter(highlight.New()).Focus().Open(path)
	out := plain(p.View())
	for _, want := range []string{"package", "main", "func", "Foo"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in stripped highlighted view:\n%s", want, out)
		}
	}
}

func TestHighlightingEmitsAnsiForKnownLanguages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.go")
	os.WriteFile(path, []byte("package main\n"), 0o644)
	p := NewPane(theme.Default).WithSize(80, 6).WithHighlighter(highlight.New()).Focus().Open(path)
	out := p.View()
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected ANSI escape in highlighted view, got plain:\n%s", out)
	}
}

func TestHighlightingDisabledWhenNoHighlighter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.txt")
	os.WriteFile(path, []byte("hello world\n"), 0o644)
	p := NewPane(theme.Default).WithSize(80, 6).Focus().Open(path)
	out := plain(p.View())
	if !strings.Contains(out, "hello world") {
		t.Fatalf("expected plain content with no highlighter:\n%s", out)
	}
}

func TestHighlightingRefreshesOnInsert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.go")
	os.WriteFile(path, []byte("package main\n"), 0o644)
	p := NewPane(theme.Default).WithSize(80, 6).WithHighlighter(highlight.New()).Focus().Open(path)
	// Move cursor past end of "package main", add a newline, then insert.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	p, _ = p.Update(runeMsg("var x = 42"))
	out := plain(p.View())
	if !strings.Contains(out, "var x = 42") {
		t.Fatalf("expected inserted text in stripped view:\n%s", out)
	}
	if !strings.Contains(p.View(), "\x1b[") {
		t.Fatalf("expected ANSI escape after insert")
	}
}

func TestHighlightingUnknownExtensionStaysPlain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.xyzq")
	os.WriteFile(path, []byte("not a real lang\n"), 0o644)
	p := NewPane(theme.Default).WithSize(80, 6).WithHighlighter(highlight.New()).Focus().Open(path)
	out := plain(p.View())
	if !strings.Contains(out, "not a real lang") {
		t.Fatalf("expected source content in stripped view:\n%s", out)
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
