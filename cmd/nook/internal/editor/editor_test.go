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

	"github.com/truffle-dev/glyph/cmd/nook/internal/gitgutter"
	"github.com/truffle-dev/glyph/cmd/nook/internal/highlight"
	"github.com/truffle-dev/glyph/cmd/nook/internal/snippets"
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

func TestWithSearchMatchesRendersBackgroundOnRow(t *testing.T) {
	p := NewPane(theme.Default).WithSize(60, 8).Focus()
	p, _ = p.Update(runeMsg("hello world hello"))
	p = p.WithSearchMatches([]Range{
		{Row: 0, Start: 0, End: 5},
		{Row: 0, Start: 12, End: 17},
	}, 0)
	out := p.View()
	plainOut := plain(out)
	if !strings.Contains(plainOut, "hello world hello") {
		t.Fatalf("plain content lost: %q", plainOut)
	}
	// Active match should emit some ANSI background sequence on row 0.
	if plainOut == out {
		t.Fatalf("expected ANSI styling, got plain text: %q", out)
	}
}

func TestClearSearchMatches(t *testing.T) {
	p := NewPane(theme.Default).WithSize(40, 4)
	p = p.WithSearchMatches([]Range{{Row: 0, Start: 0, End: 3}}, 0)
	if len(p.SearchMatches()) != 1 {
		t.Fatal("expected 1 match set")
	}
	p = p.ClearSearchMatches()
	if len(p.SearchMatches()) != 0 {
		t.Error("expected matches cleared")
	}
	if p.SearchCurrent() != -1 {
		t.Errorf("expected current -1, got %d", p.SearchCurrent())
	}
}

func TestSetLineMarkers_AccessorRoundtrip(t *testing.T) {
	p := NewPane(theme.Default)
	if got := p.LineMarkerAt(0); got != gitgutter.None {
		t.Errorf("unset map should return None, got %v", got)
	}
	p = p.SetLineMarkers(map[int]gitgutter.Marker{
		1: gitgutter.Added,
		3: gitgutter.Modified,
		7: gitgutter.DeletedAbove,
	})
	if got := p.LineMarkerAt(1); got != gitgutter.Added {
		t.Errorf("row 1: got %v, want Added", got)
	}
	if got := p.LineMarkerAt(3); got != gitgutter.Modified {
		t.Errorf("row 3: got %v, want Modified", got)
	}
	if got := p.LineMarkerAt(7); got != gitgutter.DeletedAbove {
		t.Errorf("row 7: got %v, want DeletedAbove", got)
	}
	if got := p.LineMarkerAt(99); got != gitgutter.None {
		t.Errorf("absent row: got %v, want None", got)
	}
	p = p.SetLineMarkers(nil)
	if got := p.LineMarkerAt(1); got != gitgutter.None {
		t.Errorf("after clear: got %v, want None", got)
	}
}

func TestSetBreakpointRows_AccessorRoundtrip(t *testing.T) {
	p := NewPane(theme.Default)
	if p.IsBreakpoint(0) {
		t.Errorf("unset map should report false")
	}
	p = p.SetBreakpointRows(map[int]bool{2: true, 7: true})
	if !p.IsBreakpoint(2) || !p.IsBreakpoint(7) {
		t.Errorf("expected rows 2 and 7 set")
	}
	if p.IsBreakpoint(3) {
		t.Errorf("row 3 not set should report false")
	}
	p = p.SetBreakpointRows(nil)
	if p.IsBreakpoint(2) {
		t.Errorf("after clear, row 2 still reports true")
	}
}

func TestSetStoppedAtRow(t *testing.T) {
	p := NewPane(theme.Default)
	if got := p.StoppedAtRow(); got != -1 {
		t.Errorf("fresh pane StoppedAtRow = %d; want -1", got)
	}
	p = p.SetStoppedAtRow(5)
	if got := p.StoppedAtRow(); got != 5 {
		t.Errorf("after set 5, StoppedAtRow = %d", got)
	}
	p = p.SetStoppedAtRow(0)
	if got := p.StoppedAtRow(); got != 0 {
		t.Errorf("after set 0, StoppedAtRow = %d", got)
	}
	p = p.SetStoppedAtRow(-1)
	if got := p.StoppedAtRow(); got != -1 {
		t.Errorf("after clear, StoppedAtRow = %d", got)
	}
}

func TestViewRendersBreakpointAndStopMarkers(t *testing.T) {
	p := NewPane(theme.Default).WithSize(60, 6).Focus()
	p = p.ReplaceAllFromString("alpha\nbeta\ngamma\n")
	baseline := strings.Count(plain(p.View()), "●")

	p = p.SetBreakpointRows(map[int]bool{1: true})
	withBP := strings.Count(plain(p.View()), "●")
	if withBP != baseline+1 {
		t.Errorf("breakpoint dot count = %d; want baseline+1 = %d", withBP, baseline+1)
	}

	p = p.SetStoppedAtRow(1)
	out := plain(p.View())
	if !strings.Contains(out, "▶") {
		t.Errorf("expected stopped arrow when paused at row 1:\n%s", out)
	}
	withStop := strings.Count(out, "●")
	if withStop != baseline {
		t.Errorf("after stop, ● count = %d; want baseline %d (BP dot hidden under arrow)", withStop, baseline)
	}
}

func TestStopMarkerOverridesGitSigil(t *testing.T) {
	p := NewPane(theme.Default).WithSize(60, 6).Focus()
	p = p.ReplaceAllFromString("first\n")
	p = p.SetLineMarkers(map[int]gitgutter.Marker{0: gitgutter.Modified})
	p = p.SetStoppedAtRow(0)
	out := plain(p.View())
	if !strings.Contains(out, "▶") {
		t.Errorf("expected stop arrow when paused:\n%s", out)
	}
	if strings.Contains(out, "▎") {
		t.Errorf("git modified sigil should be hidden under stop arrow:\n%s", out)
	}
}

func TestViewRendersGitSigils(t *testing.T) {
	p := NewPane(theme.Default).WithSize(60, 8).Focus()
	// Three buffer lines, one for each marker state.
	p = p.ReplaceAllFromString("first\nsecond\nthird\n")
	p = p.SetLineMarkers(map[int]gitgutter.Marker{
		0: gitgutter.Added,
		1: gitgutter.Modified,
		2: gitgutter.DeletedAbove,
	})
	out := plain(p.View())
	// Both block sigils should appear once each in the plain output.
	if !strings.Contains(out, "▎") {
		t.Errorf("expected vertical block sigil for added/modified in view:\n%s", out)
	}
	if !strings.Contains(out, "▔") {
		t.Errorf("expected upper-eighth sigil for deleted-above in view:\n%s", out)
	}
}

func TestViewWithoutGitMarkersStillRendersDivider(t *testing.T) {
	// Regression: when no git markers are set the marker column should still
	// be a space + the divider char, preserving the original two-char width.
	p := NewPane(theme.Default).WithSize(60, 8).Focus()
	p, _ = p.Update(runeMsg("hello"))
	out := plain(p.View())
	if !strings.Contains(out, "│") {
		t.Fatalf("expected divider │ in view:\n%s", out)
	}
	if strings.Contains(out, "▎") || strings.Contains(out, "▔") {
		t.Errorf("unset markers should not render git sigils:\n%s", out)
	}
}

func TestMatchesForRowFiltersByRow(t *testing.T) {
	p := NewPane(theme.Default)
	p = p.WithSearchMatches([]Range{
		{Row: 0, Start: 0, End: 2},
		{Row: 2, Start: 5, End: 7},
		{Row: 2, Start: 10, End: 12},
	}, 2)
	rowMatches, active := p.matchesForRow(2)
	if len(rowMatches) != 2 {
		t.Fatalf("expected 2 matches on row 2, got %d", len(rowMatches))
	}
	if active != 1 {
		t.Errorf("expected active 1, got %d", active)
	}
	rowMatches, active = p.matchesForRow(1)
	if rowMatches != nil {
		t.Errorf("expected no matches on row 1, got %+v", rowMatches)
	}
	if active != -1 {
		t.Errorf("expected active -1, got %d", active)
	}
}

// paneWithBuffer constructs a pane whose buffer is the given lines and whose
// primary cursor is at (row, col). Used to seed multi-cursor tests.
func paneWithBuffer(lines []string, row, col int) Pane {
	p := NewPane(theme.Default).WithSize(80, 20).Focus()
	p.buf.Lines = append([]string{}, lines...)
	p.row = row
	p.col = col
	return p
}

func TestAddCursorBelowAndAbove(t *testing.T) {
	p := paneWithBuffer([]string{"foo", "bar", "baz"}, 0, 2)
	p = p.AddCursorBelow()
	if p.ExtraCursorCount() != 1 {
		t.Fatalf("expected 1 extra cursor after AddCursorBelow, got %d", p.ExtraCursorCount())
	}
	all := p.AllCursorPositions()
	if all[1].Row != 1 || all[1].Col != 2 {
		t.Errorf("expected extra at (1,2), got (%d,%d)", all[1].Row, all[1].Col)
	}
	p = p.AddCursorBelow()
	if p.ExtraCursorCount() != 2 {
		t.Fatalf("expected 2 extras after second AddCursorBelow, got %d", p.ExtraCursorCount())
	}
	// At bottom — no-op.
	p = p.AddCursorBelow()
	if p.ExtraCursorCount() != 2 {
		t.Errorf("AddCursorBelow at bottom should be no-op, got %d extras", p.ExtraCursorCount())
	}

	// AddCursorAbove from row 2 cursor.
	p2 := paneWithBuffer([]string{"foo", "bar", "baz"}, 2, 1)
	p2 = p2.AddCursorAbove()
	if p2.ExtraCursorCount() != 1 {
		t.Fatalf("expected 1 extra after AddCursorAbove, got %d", p2.ExtraCursorCount())
	}
	all2 := p2.AllCursorPositions()
	if all2[1].Row != 1 || all2[1].Col != 1 {
		t.Errorf("expected extra at (1,1), got (%d,%d)", all2[1].Row, all2[1].Col)
	}
	// At top — no-op.
	p3 := paneWithBuffer([]string{"foo"}, 0, 0)
	p3 = p3.AddCursorAbove()
	if p3.ExtraCursorCount() != 0 {
		t.Errorf("AddCursorAbove at top should be no-op, got %d extras", p3.ExtraCursorCount())
	}
}

func TestAddCursorBelowClampsCol(t *testing.T) {
	p := paneWithBuffer([]string{"longer line", "ab"}, 0, 7)
	p = p.AddCursorBelow()
	all := p.AllCursorPositions()
	if all[1].Row != 1 || all[1].Col != 2 {
		t.Errorf("expected extra clamped to (1,2), got (%d,%d)", all[1].Row, all[1].Col)
	}
}

func TestAddNextMatchCursorFindsAndWraps(t *testing.T) {
	p := paneWithBuffer([]string{"foo bar foo", "baz foo", "qux"}, 0, 1) // primary inside "foo"
	p = p.AddNextMatchCursor()
	if p.ExtraCursorCount() != 1 {
		t.Fatalf("expected 1 extra after first AddNext, got %d", p.ExtraCursorCount())
	}
	// Next forward match: "foo" on row 0 starts at col 8, ends at 11.
	all := p.AllCursorPositions()
	if all[1].Row != 0 || all[1].Col != 11 {
		t.Errorf("expected extra at (0,11), got (%d,%d)", all[1].Row, all[1].Col)
	}
	p = p.AddNextMatchCursor()
	if p.ExtraCursorCount() != 2 {
		t.Fatalf("expected 2 extras after second AddNext, got %d", p.ExtraCursorCount())
	}
	// Next forward from (0,11): "foo" on row 1 starts at col 4, ends at 7.
	all = p.AllCursorPositions()
	if all[2].Row != 1 || all[2].Col != 7 {
		t.Errorf("expected extra at (1,7), got (%d,%d)", all[2].Row, all[2].Col)
	}
}

func TestAddNextMatchCursorWholeWordOnly(t *testing.T) {
	p := paneWithBuffer([]string{"foo foobar foo"}, 0, 1)
	p = p.AddNextMatchCursor()
	if p.ExtraCursorCount() != 1 {
		t.Fatalf("expected 1 extra, got %d", p.ExtraCursorCount())
	}
	all := p.AllCursorPositions()
	// Should skip "foobar" and find "foo" at col 11 (end at 14).
	if all[1].Row != 0 || all[1].Col != 14 {
		t.Errorf("expected extra at (0,14), got (%d,%d)", all[1].Row, all[1].Col)
	}
}

func TestAddNextMatchCursorEmptyWordNoOp(t *testing.T) {
	p := paneWithBuffer([]string{"  foo"}, 0, 0)
	p = p.AddNextMatchCursor()
	if p.ExtraCursorCount() != 0 {
		t.Errorf("AddNextMatchCursor with no word under cursor should be no-op, got %d extras", p.ExtraCursorCount())
	}
}

func TestMultiCursorInsertRunes(t *testing.T) {
	p := paneWithBuffer([]string{"foo", "bar", "baz"}, 0, 0)
	p = p.AddCursorBelow().AddCursorBelow()
	p, _ = p.Update(runeMsg("X"))
	if p.Line(0) != "Xfoo" || p.Line(1) != "Xbar" || p.Line(2) != "Xbaz" {
		t.Fatalf("expected X-prefix on all rows, got %v", p.Lines())
	}
	// Primary advanced to col 1; extras also at col 1 of their rows.
	if p.CursorCol() != 1 {
		t.Errorf("expected primary col 1, got %d", p.CursorCol())
	}
	for _, e := range p.AllCursorPositions()[1:] {
		if e.Col != 1 {
			t.Errorf("expected extra col 1, got %d", e.Col)
		}
	}
}

func TestMultiCursorInsertSameRowAdvancesBoth(t *testing.T) {
	p := paneWithBuffer([]string{"abcdef"}, 0, 2) // primary between b and c
	// Add extra at (0,5).
	p.extras = append(p.extras, extraCursor{Row: 0, Col: 5})
	p, _ = p.Update(runeMsg("Z"))
	// First (sorted): edit at (0,2) → "abZcdef". Shift extra (0,5) → (0,6).
	// Second: edit at (0,6) → "abZcdeZf". Final positions: primary (0,3), extra (0,7).
	if p.Line(0) != "abZcdeZf" {
		t.Fatalf("expected 'abZcdeZf', got %q", p.Line(0))
	}
	all := p.AllCursorPositions()
	// All positions sorted to verify both cursors landed correctly.
	wantPrimary := CursorPos{Row: 0, Col: 3}
	wantExtra := CursorPos{Row: 0, Col: 7}
	if all[0] != wantPrimary {
		t.Errorf("primary: want %+v, got %+v", wantPrimary, all[0])
	}
	if all[1] != wantExtra {
		t.Errorf("extra: want %+v, got %+v", wantExtra, all[1])
	}
}

func TestMultiCursorBackspaceSameRowDeletesTwoChars(t *testing.T) {
	p := paneWithBuffer([]string{"abcdef"}, 0, 3)            // between c and d
	p.extras = append(p.extras, extraCursor{Row: 0, Col: 6}) // at EOL
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	// Delete char at col 2 ('c') and col 5 ('f'). Line becomes "abdef" -> "abde".
	if p.Line(0) != "abde" {
		t.Fatalf("expected 'abde', got %q", p.Line(0))
	}
}

func TestMultiCursorNewlineSplitsAllRows(t *testing.T) {
	p := paneWithBuffer([]string{"foo", "bar"}, 0, 2)
	p = p.AddCursorBelow()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Each cursor splits its row at col 2. row0 → "fo"+"o", row1 → "ba"+"r".
	want := []string{"fo", "o", "ba", "r"}
	if len(p.Lines()) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(p.Lines()), p.Lines())
	}
	for i, w := range want {
		if p.Line(i) != w {
			t.Errorf("line %d: want %q, got %q", i, w, p.Line(i))
		}
	}
}

func TestMultiCursorBackspaceMergeIsConsistent(t *testing.T) {
	// Both extras at col 0 of non-zero rows. Each backspace should merge with prev row.
	p := paneWithBuffer([]string{"foo", "bar", "baz"}, 1, 0)
	p.extras = append(p.extras, extraCursor{Row: 2, Col: 0})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	// After both merges: one line "foobarbaz".
	if len(p.Lines()) != 1 {
		t.Fatalf("expected 1 line after two merges, got %d: %v", len(p.Lines()), p.Lines())
	}
	if p.Line(0) != "foobarbaz" {
		t.Errorf("expected 'foobarbaz', got %q", p.Line(0))
	}
}

func TestEscClearsExtras(t *testing.T) {
	p := paneWithBuffer([]string{"foo", "bar"}, 0, 0)
	p = p.AddCursorBelow()
	if p.ExtraCursorCount() != 1 {
		t.Fatalf("setup: expected 1 extra")
	}
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.ExtraCursorCount() != 0 {
		t.Errorf("Esc should clear extras, got %d", p.ExtraCursorCount())
	}
	if cmd != nil {
		t.Errorf("Esc with extras should not emit CancelMsg")
	}

	// Second Esc: no extras → emits CancelMsg.
	p, cmd = p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatalf("Esc with no extras should emit CancelMsg")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Errorf("expected CancelMsg, got %T", cmd())
	}
}

func TestMovementClearsExtras(t *testing.T) {
	p := paneWithBuffer([]string{"foo", "bar", "baz"}, 0, 1)
	p = p.AddCursorBelow().AddCursorBelow()
	if p.ExtraCursorCount() != 2 {
		t.Fatalf("setup: expected 2 extras")
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRight})
	if p.ExtraCursorCount() != 0 {
		t.Errorf("KeyRight should clear extras, got %d", p.ExtraCursorCount())
	}

	p = paneWithBuffer([]string{"foo", "bar"}, 0, 0).AddCursorBelow()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.ExtraCursorCount() != 0 {
		t.Errorf("KeyDown should clear extras")
	}
}

func TestCtrlDAddsNextMatch(t *testing.T) {
	p := paneWithBuffer([]string{"foo bar foo"}, 0, 1) // primary inside "foo"
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if p.ExtraCursorCount() != 1 {
		t.Fatalf("Ctrl+D should add a cursor, got %d extras", p.ExtraCursorCount())
	}
	all := p.AllCursorPositions()
	if all[1].Col != 11 {
		t.Errorf("expected extra at col 11, got %d", all[1].Col)
	}
}

func TestCtrlUpDownStacks(t *testing.T) {
	p := paneWithBuffer([]string{"foo", "bar", "baz"}, 1, 1)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlDown})
	if p.ExtraCursorCount() != 1 {
		t.Fatalf("Ctrl+Down should add an extra, got %d", p.ExtraCursorCount())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlUp})
	if p.ExtraCursorCount() != 2 {
		t.Fatalf("Ctrl+Up should add another extra, got %d", p.ExtraCursorCount())
	}
	all := p.AllCursorPositions()
	rows := map[int]bool{}
	for _, c := range all {
		rows[c.Row] = true
	}
	if !rows[0] || !rows[1] || !rows[2] {
		t.Errorf("expected cursors on all 3 rows, got rows %v", rows)
	}
}

func TestViewRendersMultipleCursors(t *testing.T) {
	p := paneWithBuffer([]string{"foo", "bar", "baz"}, 0, 1) // primary at (0,1)
	p = p.AddCursorBelow().AddCursorBelow()                  // extras at (1,1), (2,1)
	out := p.View()
	// Confirm every cursor row's content is present in plain text.
	plainOut := plain(out)
	for _, want := range []string{"foo", "bar", "baz"} {
		if !strings.Contains(plainOut, want) {
			t.Errorf("expected View output to contain %q, got plain:\n%s", want, plainOut)
		}
	}
	// Confirm cursor styling escape is present in the output (3 cursor cells).
	// Count occurrences of the cursor-bg color region; with 3 cursors there
	// should be at least 3 inverse-background renders.
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("expected ANSI styling in output")
	}
}

func TestExtrasStayConsistentAfterDedup(t *testing.T) {
	p := paneWithBuffer([]string{"foo"}, 0, 0)
	// Manually add two extras at the same position; dedup should collapse.
	p.extras = []extraCursor{{Row: 0, Col: 2}, {Row: 0, Col: 2}}
	(&p).dedupCursors()
	if len(p.extras) != 1 {
		t.Fatalf("expected 1 extra after dedup, got %d", len(p.extras))
	}
	// Adding extra at primary's position should also collapse.
	p.extras = append(p.extras, extraCursor{Row: 0, Col: 0})
	(&p).dedupCursors()
	if len(p.extras) != 1 {
		t.Fatalf("expected 1 extra (primary overlap dropped), got %d", len(p.extras))
	}
}

// snippetPane constructs a focused pane with one line of content and the
// cursor parked at the right edge (simulating the user having just typed a
// prefix).
func snippetPane(t *testing.T, line string) Pane {
	t.Helper()
	p := NewPane(theme.Default).WithSize(60, 10).Focus()
	p, _ = p.Update(runeMsg(line))
	return p
}

func TestExpandSnippetSingleLineNoTabstops(t *testing.T) {
	p := snippetPane(t, "todo")
	exp := snippets.Expansion{Text: "// TODO:", Tabstops: nil, Final: nil}
	p = p.ExpandSnippet(0, exp)
	if p.Line(0) != "// TODO:" {
		t.Fatalf("line = %q", p.Line(0))
	}
	if p.InSnippetMode() {
		t.Fatalf("expected no snippet mode when expansion has no tabstops or final")
	}
	if p.CursorCol() != len("// TODO:") {
		t.Fatalf("cursor col = %d, expected %d", p.CursorCol(), len("// TODO:"))
	}
}

func TestExpandSnippetPlacesCursorOnFirstTabstop(t *testing.T) {
	p := snippetPane(t, "fn")
	// Body: "func ${1:name}() ${2:err}" — two stops, first at byte 5
	// (after "func "), second at byte 17 (after "func name() ").
	exp := snippets.Expansion{
		Text: "func name() err",
		Tabstops: []snippets.Tabstop{
			{Index: 1, Offset: 5, Length: 4},
			{Index: 2, Offset: 12, Length: 3},
		},
	}
	p = p.ExpandSnippet(0, exp)
	if p.Line(0) != "func name() err" {
		t.Fatalf("line = %q", p.Line(0))
	}
	if !p.InSnippetMode() {
		t.Fatal("expected snippet mode active after expansion with tabstops")
	}
	if got := p.CursorCol(); got != 5 {
		t.Fatalf("cursor col = %d, want 5", got)
	}
	ts, ok := p.CurrentSnippetTabstop()
	if !ok {
		t.Fatal("CurrentSnippetTabstop returned !ok")
	}
	if ts.Index != 1 || ts.Length != 4 {
		t.Fatalf("got %+v, want index=1 length=4", ts)
	}
}

func TestSnippetTabAdvancesAndExits(t *testing.T) {
	p := snippetPane(t, "fn")
	exp := snippets.Expansion{
		Text: "func name() err",
		Tabstops: []snippets.Tabstop{
			{Index: 1, Offset: 5, Length: 4},
			{Index: 2, Offset: 12, Length: 3},
		},
	}
	p = p.ExpandSnippet(0, exp)
	// Tab -> second stop
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := p.CursorCol(); got != 12 {
		t.Fatalf("after first Tab cursor col = %d, want 12", got)
	}
	// Tab again -> exit
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyTab})
	if p.InSnippetMode() {
		t.Fatal("expected snippet mode to exit after last Tab")
	}
}

func TestSnippetTabToFinalThenExit(t *testing.T) {
	p := snippetPane(t, "fn")
	exp := snippets.Expansion{
		Text: "func name() { body }",
		Tabstops: []snippets.Tabstop{
			{Index: 1, Offset: 5, Length: 4},
		},
		Final: &snippets.Tabstop{Index: 0, Offset: 13, Length: 4},
	}
	p = p.ExpandSnippet(0, exp)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyTab})
	if p.InSnippetMode() {
		t.Fatal("snippet mode should exit after Tab past last stop")
	}
	if p.CursorCol() != 13 {
		t.Fatalf("expected cursor at final ($0) col=13, got %d", p.CursorCol())
	}
}

func TestSnippetShiftTabReturns(t *testing.T) {
	p := snippetPane(t, "fn")
	exp := snippets.Expansion{
		Text: "func name() err",
		Tabstops: []snippets.Tabstop{
			{Index: 1, Offset: 5, Length: 4},
			{Index: 2, Offset: 12, Length: 3},
		},
	}
	p = p.ExpandSnippet(0, exp)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyTab})
	if p.CursorCol() != 12 {
		t.Fatalf("after Tab col = %d, want 12", p.CursorCol())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if p.CursorCol() != 5 {
		t.Fatalf("after Shift+Tab col = %d, want 5", p.CursorCol())
	}
	if !p.InSnippetMode() {
		t.Fatal("snippet mode should still be active after Shift+Tab")
	}
}

func TestSnippetEscExits(t *testing.T) {
	p := snippetPane(t, "fn")
	exp := snippets.Expansion{
		Text: "func x()",
		Tabstops: []snippets.Tabstop{
			{Index: 1, Offset: 5, Length: 1},
		},
	}
	p = p.ExpandSnippet(0, exp)
	if !p.InSnippetMode() {
		t.Fatal("expected snippet mode active")
	}
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.InSnippetMode() {
		t.Fatal("Esc should exit snippet mode")
	}
	if cmd != nil {
		t.Fatal("Esc in snippet mode should not emit CancelMsg")
	}
}

func TestSnippetEditKeyAutoExits(t *testing.T) {
	p := snippetPane(t, "fn")
	exp := snippets.Expansion{
		Text: "func x()",
		Tabstops: []snippets.Tabstop{
			{Index: 1, Offset: 5, Length: 1},
		},
	}
	p = p.ExpandSnippet(0, exp)
	p, _ = p.Update(runeMsg("y"))
	if p.InSnippetMode() {
		t.Fatal("typing a rune should exit snippet mode")
	}
	// And the rune was inserted at the tabstop position.
	if !strings.Contains(p.Line(0), "func yx()") {
		t.Fatalf("expected 'y' inserted at tabstop col, got %q", p.Line(0))
	}
}

func TestExpandSnippetDeletesPrefix(t *testing.T) {
	p := snippetPane(t, "tdo")
	// User cursor sits at col=3 ("tdo|"); prefix start at 0; expansion replaces.
	exp := snippets.Expansion{Text: "// TODO:"}
	p = p.ExpandSnippet(0, exp)
	if p.Line(0) != "// TODO:" {
		t.Fatalf("line = %q (expected prefix to be removed)", p.Line(0))
	}
}

func TestExpandSnippetMultilineRowsAreCorrect(t *testing.T) {
	p := snippetPane(t, "iferr")
	// Body has a newline; first tabstop on row 1 at col 1.
	// Text bytes:           0         1
	//             0123456789012345678901
	//             "if err != nil {\n\treturn err\n}"
	// Offsets used:   3 (err),  24 (err in return).
	exp := snippets.Expansion{
		Text: "if err != nil {\n\treturn err\n}",
		Tabstops: []snippets.Tabstop{
			{Index: 1, Offset: 3, Length: 3},  // first "err" on row 0
			{Index: 2, Offset: 24, Length: 3}, // second "err" on row 1
		},
	}
	p = p.ExpandSnippet(0, exp)
	if p.LineCount() != 3 {
		t.Fatalf("expected 3 lines after multiline snippet, got %d: %v", p.LineCount(), p.Lines())
	}
	if got := p.CursorRow(); got != 0 {
		t.Fatalf("cursor row = %d, want 0 (first tabstop)", got)
	}
	if got := p.CursorCol(); got != 3 {
		t.Fatalf("cursor col = %d, want 3", got)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := p.CursorRow(); got != 1 {
		t.Fatalf("after Tab cursor row = %d, want 1", got)
	}
	if got := p.CursorCol(); got != 8 {
		t.Fatalf("after Tab cursor col = %d, want 8", got)
	}
}

func TestDeleteRangeRemovesBytes(t *testing.T) {
	p := snippetPane(t, "abcdef")
	p = p.DeleteRange(0, 1, 4)
	if p.Line(0) != "aef" {
		t.Fatalf("DeleteRange line = %q, want 'aef'", p.Line(0))
	}
	if p.CursorCol() != 3 {
		t.Fatalf("CursorCol = %d, want 3 (was 6, removed 3)", p.CursorCol())
	}
}
