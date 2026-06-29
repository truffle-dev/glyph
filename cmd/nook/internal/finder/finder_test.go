package finder

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

func defaultTheme() theme.Theme { return theme.Theme{} }

func TestSearchLiteralCaseInsensitive(t *testing.T) {
	buf := []string{
		"package finder",
		"FINDER is the name",
		"refinder code",
	}
	ms, err := Search(buf, "finder", false, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(ms) != 3 {
		t.Fatalf("want 3 matches, got %d: %+v", len(ms), ms)
	}
	if ms[0].Row != 0 || ms[0].StartCol != 8 || ms[0].EndCol != 14 {
		t.Errorf("row0 wrong: %+v", ms[0])
	}
	if ms[1].Row != 1 {
		t.Errorf("row1 wrong: %+v", ms[1])
	}
	if ms[2].Row != 2 || ms[2].StartCol != 2 {
		t.Errorf("row2 wrong: %+v", ms[2])
	}
}

func TestSearchLiteralCaseSensitive(t *testing.T) {
	buf := []string{"Foo foo FOO"}
	ms, _ := Search(buf, "foo", false, true)
	if len(ms) != 1 {
		t.Fatalf("want 1 match (lowercase only), got %d", len(ms))
	}
	if ms[0].StartCol != 4 {
		t.Errorf("wrong column: %d", ms[0].StartCol)
	}
}

func TestSearchMultipleOnLine(t *testing.T) {
	buf := []string{"ababab"}
	ms, _ := Search(buf, "ab", false, true)
	if len(ms) != 3 {
		t.Fatalf("want 3 matches, got %d", len(ms))
	}
	if ms[0].StartCol != 0 || ms[1].StartCol != 2 || ms[2].StartCol != 4 {
		t.Errorf("wrong start cols: %+v", ms)
	}
}

func TestSearchRegex(t *testing.T) {
	buf := []string{"v1.0 v2.0 v3.0"}
	ms, err := Search(buf, `v\d+\.\d+`, true, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(ms) != 3 {
		t.Fatalf("want 3 matches, got %d", len(ms))
	}
}

func TestSearchRegexCaseInsensitive(t *testing.T) {
	buf := []string{"HELLO hello HeLLo"}
	ms, _ := Search(buf, `hello`, true, false)
	if len(ms) != 3 {
		t.Fatalf("want 3, got %d", len(ms))
	}
}

func TestSearchRegexCompileError(t *testing.T) {
	_, err := Search([]string{"x"}, "[unclosed", true, true)
	if err == nil {
		t.Fatal("expected compile error")
	}
}

func TestSearchEmptyPattern(t *testing.T) {
	ms, err := Search([]string{"hello"}, "", false, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ms != nil {
		t.Errorf("empty pattern should return nil, got %+v", ms)
	}
}

func TestSearchRegexSkipsEmptyMatches(t *testing.T) {
	// `^` matches start-of-line; we don't want it to land in our match list
	// because navigation would freeze on a zero-width hit.
	ms, _ := Search([]string{"one", "two"}, `^`, true, true)
	if len(ms) != 0 {
		t.Errorf("zero-width matches should be skipped, got %d", len(ms))
	}
}

func TestNextPrevWrap(t *testing.T) {
	f := New(defaultTheme()).Open(ModeFind).WithMatches([]Match{
		{Row: 0, StartCol: 0, EndCol: 1},
		{Row: 0, StartCol: 5, EndCol: 6},
		{Row: 1, StartCol: 0, EndCol: 1},
	}, false)
	if f.CurrentIndex() != 0 {
		t.Fatalf("initial index should be 0, got %d", f.CurrentIndex())
	}
	f = f.Next()
	if f.CurrentIndex() != 1 {
		t.Errorf("after Next: %d", f.CurrentIndex())
	}
	f = f.Next().Next()
	if f.CurrentIndex() != 0 {
		t.Errorf("Next wrap to 0 failed: %d", f.CurrentIndex())
	}
	f = f.Prev()
	if f.CurrentIndex() != 2 {
		t.Errorf("Prev wrap to last failed: %d", f.CurrentIndex())
	}
}

func TestUpdateTypePatternEmitsChanged(t *testing.T) {
	f := New(defaultTheme()).Open(ModeFind)
	f, ev := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if ev != EventPatternChanged {
		t.Errorf("expected EventPatternChanged, got %v", ev)
	}
	if f.Pattern() != "a" {
		t.Errorf("pattern: %q", f.Pattern())
	}
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b', 'c'}})
	if f.Pattern() != "abc" {
		t.Errorf("pattern after multi-rune: %q", f.Pattern())
	}
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeySpace})
	if f.Pattern() != "abc " {
		t.Errorf("pattern after space: %q", f.Pattern())
	}
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if f.Pattern() != "abc" {
		t.Errorf("pattern after backspace: %q", f.Pattern())
	}
}

func TestUpdateTabSwitchesFocusInReplaceMode(t *testing.T) {
	f := New(defaultTheme()).Open(ModeReplace)
	if f.Focus() != FocusFind {
		t.Fatalf("expected initial focus on find, got %v", f.Focus())
	}
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyTab})
	if f.Focus() != FocusReplace {
		t.Errorf("expected focus replace after tab, got %v", f.Focus())
	}
	// Typing now targets replacement.
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if f.Replacement() != "X" {
		t.Errorf("replacement: %q", f.Replacement())
	}
	if f.Pattern() != "" {
		t.Errorf("pattern should not change when focus on replace: %q", f.Pattern())
	}
}

func TestUpdateTabIsNoOpInFindMode(t *testing.T) {
	f := New(defaultTheme()).Open(ModeFind)
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyTab})
	if f.Focus() != FocusFind {
		t.Errorf("tab in find mode should not switch focus")
	}
}

func TestUpdateEscClosesAndEmitsEvent(t *testing.T) {
	f := New(defaultTheme()).Open(ModeFind)
	f, ev := f.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if ev != EventClose {
		t.Errorf("expected EventClose, got %v", ev)
	}
	if f.IsOpen() {
		t.Errorf("finder should be closed")
	}
}

func TestUpdateClosedFinderIgnoresKeys(t *testing.T) {
	f := New(defaultTheme())
	f2, ev := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if ev != EventNone {
		t.Errorf("closed finder should ignore keys, got %v", ev)
	}
	if f2.Pattern() != "" {
		t.Errorf("closed finder should not store input")
	}
}

func TestUpdateEnterAndArrowsEmitNavigation(t *testing.T) {
	f := New(defaultTheme()).Open(ModeFind)
	_, ev := f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if ev != EventJumpNext {
		t.Errorf("enter: %v", ev)
	}
	_, ev = f.Update(tea.KeyMsg{Type: tea.KeyDown})
	if ev != EventJumpNext {
		t.Errorf("down: %v", ev)
	}
	_, ev = f.Update(tea.KeyMsg{Type: tea.KeyUp})
	if ev != EventJumpPrev {
		t.Errorf("up: %v", ev)
	}
}

func TestUpdateAltXTogglesRegex(t *testing.T) {
	f := New(defaultTheme()).Open(ModeFind)
	if f.UseRegex() {
		t.Fatal("regex should start off")
	}
	f, ev := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}, Alt: true})
	if ev != EventPatternChanged {
		t.Errorf("expected pattern-changed after toggle, got %v", ev)
	}
	if !f.UseRegex() {
		t.Errorf("regex should be on after alt+x")
	}
}

func TestUpdateAltCTogglesCase(t *testing.T) {
	f := New(defaultTheme()).Open(ModeFind)
	f, ev := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}, Alt: true})
	if ev != EventPatternChanged {
		t.Errorf("expected pattern-changed, got %v", ev)
	}
	if !f.CaseSensitive() {
		t.Error("case-sensitive should be on")
	}
}

func TestUpdateCtrlRReplacesInReplaceMode(t *testing.T) {
	f := New(defaultTheme()).Open(ModeReplace)
	_, ev := f.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if ev != EventReplaceCurrent {
		t.Errorf("expected EventReplaceCurrent, got %v", ev)
	}
}

func TestUpdateCtrlRIsNoOpInFindMode(t *testing.T) {
	f := New(defaultTheme()).Open(ModeFind)
	_, ev := f.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if ev != EventNone {
		t.Errorf("ctrl+r in find mode should be no-op, got %v", ev)
	}
}

func TestUpdateAltRReplacesAllInReplaceMode(t *testing.T) {
	f := New(defaultTheme()).Open(ModeReplace)
	_, ev := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}, Alt: true})
	if ev != EventReplaceAll {
		t.Errorf("expected EventReplaceAll, got %v", ev)
	}
}

func TestSelectMatchAt(t *testing.T) {
	ms := []Match{
		{Row: 0, StartCol: 0, EndCol: 1},
		{Row: 2, StartCol: 3, EndCol: 4},
		{Row: 5, StartCol: 0, EndCol: 1},
	}
	f := New(defaultTheme()).Open(ModeFind).WithMatches(ms, false)

	f = f.SelectMatchAt(3, 0)
	if f.CurrentIndex() != 2 {
		t.Errorf("want index 2 for row=3, got %d", f.CurrentIndex())
	}
	f = f.SelectMatchAt(10, 0)
	if f.CurrentIndex() != 0 {
		t.Errorf("past-end should wrap to first, got %d", f.CurrentIndex())
	}
}

func TestApplyReplacementLiteral(t *testing.T) {
	got := ApplyReplacement("hello world", Match{Row: 0, StartCol: 6, EndCol: 11}, "Phantom", nil)
	if got != "hello Phantom" {
		t.Errorf("got %q", got)
	}
}

func TestApplyReplacementRegexCaptureExpansion(t *testing.T) {
	re, err := CompileRegex(`(\w+)@(\w+)`, true)
	if err != nil {
		t.Fatal(err)
	}
	line := "contact: hi@example send to bob@org"
	// First match is "hi@example" at byte 9..19.
	got := ApplyReplacement(line, Match{Row: 0, StartCol: 9, EndCol: 19}, "$2-$1", re)
	if got != "contact: example-hi send to bob@org" {
		t.Errorf("got %q", got)
	}
}

func TestApplyReplacementInvalidRange(t *testing.T) {
	got := ApplyReplacement("abc", Match{Row: 0, StartCol: -1, EndCol: 99}, "X", nil)
	if got != "abc" {
		t.Errorf("invalid range should return line unchanged, got %q", got)
	}
}

func TestViewRendersFindBar(t *testing.T) {
	f := New(defaultTheme()).Open(ModeFind).WithSize(80)
	out := f.View()
	if !strings.Contains(out, "find") {
		t.Errorf("View should contain 'find' label: %q", out)
	}
}

func TestViewRendersReplaceBarTwoLines(t *testing.T) {
	f := New(defaultTheme()).Open(ModeReplace).WithSize(80)
	out := f.View()
	if strings.Count(out, "\n") != 1 {
		t.Errorf("replace bar should have 2 rows (1 newline), got %d", strings.Count(out, "\n"))
	}
	if !strings.Contains(out, "find") || !strings.Contains(out, "replace") {
		t.Errorf("replace bar should contain both labels: %q", out)
	}
}

func TestHeight(t *testing.T) {
	f := New(defaultTheme())
	if f.Height() != 0 {
		t.Errorf("closed finder height should be 0, got %d", f.Height())
	}
	f = f.Open(ModeFind)
	if f.Height() != 1 {
		t.Errorf("find mode height should be 1, got %d", f.Height())
	}
	f = f.Open(ModeReplace)
	if f.Height() != 2 {
		t.Errorf("replace mode height should be 2, got %d", f.Height())
	}
}

func TestCompileRegexCaseFlag(t *testing.T) {
	re, _ := CompileRegex("hello", false)
	if !re.MatchString("HELLO") {
		t.Error("case-insensitive regex should match HELLO")
	}
	re2, _ := CompileRegex("hello", true)
	if re2.MatchString("HELLO") {
		t.Error("case-sensitive regex should not match HELLO")
	}
}

func TestApplyReplacementWithExpandUsesRegexp(t *testing.T) {
	re := regexp.MustCompile(`(\d+)`)
	got := ApplyReplacement("v42 final", Match{Row: 0, StartCol: 1, EndCol: 3}, "<$1>", re)
	if got != "v<42> final" {
		t.Errorf("got %q", got)
	}
}

// TestClipRespectsDisplayWidth confirms clip budgets display cells, not
// runes: a wide character (CJK) is one rune but two columns, so a rune-
// counting clip would overshoot the column budget.
func TestClipRespectsDisplayWidth(t *testing.T) {
	t.Parallel()
	out := clip("日本語コード done", 6)
	if w := lipgloss.Width(out); w > 6 {
		t.Errorf("clip exceeded 6 display cells (got %d): %q", w, out)
	}
	if !strings.HasSuffix(out, "…") {
		t.Errorf("expected ellipsis tail on truncated input: %q", out)
	}
	if got := clip("hi", 10); got != "hi" {
		t.Errorf("clip fit-input changed content: %q", got)
	}
}
