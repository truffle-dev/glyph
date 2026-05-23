package findbar

import (
	"os"
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

func key(kt tea.KeyType, rs ...rune) tea.KeyMsg {
	return tea.KeyMsg{Type: kt, Runes: rs}
}

func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func drainQuery(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a Cmd, got nil")
	}
	msg := cmd()
	q, ok := msg.(QueryMsg)
	if !ok {
		t.Fatalf("expected QueryMsg, got %T", msg)
	}
	return q.Value
}

func TestNewIsFocusedAndEmpty(t *testing.T) {
	b := New(theme.Default)
	if !b.Focused() {
		t.Fatal("bar should be focused by default")
	}
	if b.Query() != "" {
		t.Fatalf("new bar query should be empty, got %q", b.Query())
	}
	if b.MatchCount() != 0 {
		t.Fatalf("new bar should have 0 matches, got %d", b.MatchCount())
	}
	if b.Current() != -1 {
		t.Fatalf("new bar current should be -1, got %d", b.Current())
	}
}

func TestWithQueryParksCursorAtEnd(t *testing.T) {
	b := New(theme.Default).WithQuery("hello")
	if b.Query() != "hello" {
		t.Fatalf("query mismatch: %q", b.Query())
	}
	// cursor at end means another char appends, not inserts mid-word
	b2, cmd := b.Update(runes("!"))
	if got := drainQuery(t, cmd); got != "hello!" {
		t.Fatalf("expected append at end, got %q", got)
	}
	if b2.Query() != "hello!" {
		t.Fatalf("model query mismatch: %q", b2.Query())
	}
}

func TestInsertRunes(t *testing.T) {
	b := New(theme.Default)
	b, cmd := b.Update(runes("h"))
	if got := drainQuery(t, cmd); got != "h" {
		t.Fatalf("expected h, got %q", got)
	}
	b, cmd = b.Update(runes("ello"))
	if got := drainQuery(t, cmd); got != "hello" {
		t.Fatalf("expected hello, got %q", got)
	}
}

func TestBackspaceDeletesLastRune(t *testing.T) {
	b := New(theme.Default).WithQuery("abc")
	b, cmd := b.Update(key(tea.KeyBackspace))
	if got := drainQuery(t, cmd); got != "ab" {
		t.Fatalf("expected ab, got %q", got)
	}
	if b.Query() != "ab" {
		t.Fatalf("model state mismatch: %q", b.Query())
	}
}

func TestBackspaceAtStartIsNoop(t *testing.T) {
	b := New(theme.Default).WithQuery("ab")
	// move to col 0
	b, _ = b.Update(key(tea.KeyHome))
	b, cmd := b.Update(key(tea.KeyBackspace))
	if cmd != nil {
		t.Fatalf("expected nil cmd, got non-nil")
	}
	if b.Query() != "ab" {
		t.Fatalf("query should be unchanged, got %q", b.Query())
	}
}

func TestDeleteForwardRemovesNextRune(t *testing.T) {
	b := New(theme.Default).WithQuery("abc")
	b, _ = b.Update(key(tea.KeyHome))
	b, cmd := b.Update(key(tea.KeyDelete))
	if got := drainQuery(t, cmd); got != "bc" {
		t.Fatalf("expected bc, got %q", got)
	}
}

func TestCtrlUClearsQuery(t *testing.T) {
	b := New(theme.Default).WithQuery("anything")
	b, cmd := b.Update(key(tea.KeyCtrlU))
	if got := drainQuery(t, cmd); got != "" {
		t.Fatalf("expected empty query, got %q", got)
	}
	if b.Query() != "" {
		t.Fatalf("model query should be empty, got %q", b.Query())
	}
}

func TestEscapeEmitsCloseMsg(t *testing.T) {
	b := New(theme.Default).WithQuery("hello")
	_, cmd := b.Update(key(tea.KeyEsc))
	if cmd == nil {
		t.Fatal("expected Cmd, got nil")
	}
	msg := cmd()
	if _, ok := msg.(CloseMsg); !ok {
		t.Fatalf("expected CloseMsg, got %T", msg)
	}
}

func TestEnterEmitsNextMsg(t *testing.T) {
	b := New(theme.Default).WithQuery("hello")
	_, cmd := b.Update(key(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected Cmd, got nil")
	}
	if _, ok := cmd().(NextMsg); !ok {
		t.Fatal("expected NextMsg")
	}
}

func TestAltEnterEmitsPrevMsg(t *testing.T) {
	b := New(theme.Default).WithQuery("hello")
	_, cmd := b.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	if cmd == nil {
		t.Fatal("expected Cmd")
	}
	if _, ok := cmd().(PrevMsg); !ok {
		t.Fatal("expected PrevMsg")
	}
}

func TestF3EmitsNextMsg(t *testing.T) {
	b := New(theme.Default).WithQuery("x")
	_, cmd := b.Update(key(tea.KeyF3))
	if _, ok := cmd().(NextMsg); !ok {
		t.Fatal("expected NextMsg from F3")
	}
}

func TestShiftF3EmitsPrevMsg(t *testing.T) {
	b := New(theme.Default).WithQuery("x")
	// Alt is what the Bar checks; treat the modifier as the same channel.
	_, cmd := b.Update(tea.KeyMsg{Type: tea.KeyF3, Alt: true})
	if _, ok := cmd().(PrevMsg); !ok {
		t.Fatal("expected PrevMsg from Alt+F3")
	}
}

func TestCursorMotion(t *testing.T) {
	b := New(theme.Default).WithQuery("abc")
	// at end; move left, insert -> "abXc"
	b, _ = b.Update(key(tea.KeyLeft))
	b, cmd := b.Update(runes("X"))
	if got := drainQuery(t, cmd); got != "abXc" {
		t.Fatalf("expected abXc, got %q", got)
	}
}

func TestHomeAndEndJumps(t *testing.T) {
	b := New(theme.Default).WithQuery("abc")
	b, _ = b.Update(key(tea.KeyHome))
	b, cmd := b.Update(runes("Z"))
	if got := drainQuery(t, cmd); got != "Zabc" {
		t.Fatalf("expected Zabc, got %q", got)
	}
	b, _ = b.Update(key(tea.KeyEnd))
	b, cmd = b.Update(runes("!"))
	if got := drainQuery(t, cmd); got != "Zabc!" {
		t.Fatalf("expected Zabc!, got %q", got)
	}
}

func TestWithMatchesClampsCurrent(t *testing.T) {
	matches := []Match{
		{Row: 0, ColStart: 0, ColEnd: 3},
		{Row: 1, ColStart: 5, ColEnd: 8},
		{Row: 2, ColStart: 10, ColEnd: 13},
	}
	b := New(theme.Default).WithMatches(matches, 99)
	if b.Current() != 2 {
		t.Fatalf("expected current clamped to 2, got %d", b.Current())
	}
	b = New(theme.Default).WithMatches(matches, -5)
	if b.Current() != 0 {
		t.Fatalf("expected current clamped to 0, got %d", b.Current())
	}
	b = New(theme.Default).WithMatches(nil, 0)
	if b.Current() != -1 {
		t.Fatalf("expected -1 with no matches, got %d", b.Current())
	}
}

func TestCurrentMatchReturnsMatch(t *testing.T) {
	matches := []Match{{Row: 5, ColStart: 2, ColEnd: 4}}
	b := New(theme.Default).WithMatches(matches, 0)
	m, ok := b.CurrentMatch()
	if !ok {
		t.Fatal("expected CurrentMatch to return ok")
	}
	if m.Row != 5 || m.ColStart != 2 || m.ColEnd != 4 {
		t.Fatalf("unexpected match: %+v", m)
	}
}

func TestBlurIgnoresInput(t *testing.T) {
	b := New(theme.Default).WithQuery("hi").Blur()
	b2, cmd := b.Update(runes("x"))
	if cmd != nil {
		t.Fatal("blurred bar should not emit Cmd")
	}
	if b2.Query() != "hi" {
		t.Fatalf("blurred bar query mutated: %q", b2.Query())
	}
}

func TestPureMotionDoesNotEmitQuery(t *testing.T) {
	b := New(theme.Default).WithQuery("abc")
	_, cmd := b.Update(key(tea.KeyLeft))
	if cmd != nil {
		t.Fatal("Left should not emit a Cmd")
	}
}

func TestFindMatchesCaseInsensitive(t *testing.T) {
	lines := []string{
		"Hello world",
		"world Hello",
		"WORLD",
	}
	got := FindMatches(lines, "hello", false)
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	if got[0].Row != 0 || got[0].ColStart != 0 || got[0].ColEnd != 5 {
		t.Fatalf("first match wrong: %+v", got[0])
	}
	if got[1].Row != 1 || got[1].ColStart != 6 || got[1].ColEnd != 11 {
		t.Fatalf("second match wrong: %+v", got[1])
	}
}

func TestFindMatchesCaseSensitive(t *testing.T) {
	lines := []string{"Hello world", "hello there"}
	got := FindMatches(lines, "Hello", true)
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].Row != 0 || got[0].ColStart != 0 || got[0].ColEnd != 5 {
		t.Fatalf("unexpected match: %+v", got[0])
	}
}

func TestFindMatchesEmptyQueryReturnsNil(t *testing.T) {
	got := FindMatches([]string{"abc"}, "", false)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestFindMatchesMultipleOnOneLine(t *testing.T) {
	got := FindMatches([]string{"aaaa"}, "a", false)
	if len(got) != 4 {
		t.Fatalf("expected 4 matches, got %d", len(got))
	}
}

func TestFindMatchesNonOverlapping(t *testing.T) {
	// 'aa' in 'aaaa' should match twice (0..2 and 2..4), not three times.
	got := FindMatches([]string{"aaaa"}, "aa", false)
	if len(got) != 2 {
		t.Fatalf("expected 2 non-overlapping matches, got %d", len(got))
	}
}

func TestViewIncludesCounterAndQuery(t *testing.T) {
	matches := []Match{
		{Row: 0, ColStart: 0, ColEnd: 5},
		{Row: 1, ColStart: 0, ColEnd: 5},
		{Row: 2, ColStart: 0, ColEnd: 5},
	}
	b := New(theme.Default).WithQuery("hello").WithMatches(matches, 1).WithWidth(60)
	v := b.View()
	if !strings.Contains(v, "2 / 3") {
		t.Fatalf("expected counter '2 / 3' in view:\n%s", v)
	}
	if !strings.Contains(v, "find ") {
		t.Fatalf("expected 'find ' prefix in view:\n%s", v)
	}
}

func TestViewShowsNoMatchesWhenQueryPresent(t *testing.T) {
	b := New(theme.Default).WithQuery("notfound").WithWidth(60)
	v := b.View()
	if !strings.Contains(v, "no matches") {
		t.Fatalf("expected 'no matches' in view:\n%s", v)
	}
}
