package picker

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

func mk(titles ...string) []Item {
	out := make([]Item, len(titles))
	for i, t := range titles {
		out[i] = Item{Title: t, Value: i}
	}
	return out
}

func runeMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func sendKeys(p Picker, s string) Picker {
	for _, r := range s {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return p
}

func TestEmptyFilterShowsAll(t *testing.T) {
	p := New(theme.Default).WithItems(mk("alpha", "beta", "gamma"))
	if p.Count() != 3 {
		t.Fatalf("expected 3 matches with empty filter, got %d", p.Count())
	}
}

func TestSubsequenceFilters(t *testing.T) {
	p := New(theme.Default).WithItems(mk("alpha", "beta", "gamma", "alphabet"))
	p = sendKeys(p, "ap")
	matches := p.matches()
	if len(matches) < 2 {
		t.Fatalf("expected 'alpha' and 'alphabet' for 'ap', got %d: %+v", len(matches), matches)
	}
	titles := []string{}
	for _, m := range matches {
		titles = append(titles, m.item.Title)
	}
	joined := strings.Join(titles, ",")
	if !strings.Contains(joined, "alpha") || !strings.Contains(joined, "alphabet") {
		t.Fatalf("expected alpha+alphabet in matches, got %v", titles)
	}
}

func TestNonSubsequenceFiltersOut(t *testing.T) {
	p := New(theme.Default).WithItems(mk("alpha", "beta", "alphabet"))
	p = sendKeys(p, "ab")
	matches := p.matches()
	// alpha has no 'b'; alphabet has 'b' after 'a'
	for _, m := range matches {
		if m.item.Title == "alpha" {
			t.Fatalf("alpha should not match 'ab' (no 'b' in alpha)")
		}
	}
}

func TestStartOfStringBonus(t *testing.T) {
	if s1, s2 := Score("foo", "f"), Score("xfoo", "f"); s1 <= s2 {
		t.Fatalf("expected start-of-string bonus: foo=%d, xfoo=%d", s1, s2)
	}
}

func TestWordBoundaryBonus(t *testing.T) {
	if s1, s2 := Score("snake_case", "c"), Score("snakecase", "c"); s1 <= s2 {
		t.Fatalf("expected word-boundary bonus: snake_case=%d, snakecase=%d", s1, s2)
	}
}

func TestConsecutiveBonus(t *testing.T) {
	if s1, s2 := Score("abxcd", "ab"), Score("axbycd", "ab"); s1 <= s2 {
		t.Fatalf("expected consecutive bonus: abxcd=%d, axbycd=%d", s1, s2)
	}
}

func TestEnterEmitsSelect(t *testing.T) {
	p := New(theme.Default).WithItems(mk("alpha", "beta"))
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected Cmd on Enter")
	}
	msg := cmd()
	sel, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", msg)
	}
	if sel.Item.Title != "alpha" {
		t.Fatalf("expected alpha, got %q", sel.Item.Title)
	}
	_ = p
}

func TestEscEmitsCancel(t *testing.T) {
	p := New(theme.Default).WithItems(mk("alpha"))
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected Cmd on Esc")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd())
	}
}

func TestArrowKeysMoveCursor(t *testing.T) {
	p := New(theme.Default).WithItems(mk("a", "b", "c"))
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if it, _ := p.Highlighted(); it.Title != "c" {
		t.Fatalf("expected c, got %q", it.Title)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if it, _ := p.Highlighted(); it.Title != "b" {
		t.Fatalf("expected b, got %q", it.Title)
	}
}

func TestArrowKeyStaysInRange(t *testing.T) {
	p := New(theme.Default).WithItems(mk("only"))
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if it, _ := p.Highlighted(); it.Title != "only" {
		t.Fatalf("expected to stay on the one item, got %q", it.Title)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if it, _ := p.Highlighted(); it.Title != "only" {
		t.Fatalf("expected to stay on the one item, got %q", it.Title)
	}
}

func TestFilterClearWithCtrlU(t *testing.T) {
	p := New(theme.Default).WithItems(mk("alpha", "beta"))
	p = sendKeys(p, "be")
	if p.Filter() != "be" {
		t.Fatalf("setup expected filter 'be', got %q", p.Filter())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if p.Filter() != "" {
		t.Fatalf("expected filter cleared, got %q", p.Filter())
	}
}

func TestBackspaceRemovesChar(t *testing.T) {
	p := New(theme.Default).WithItems(mk("alpha"))
	p = sendKeys(p, "abc")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.Filter() != "ab" {
		t.Fatalf("expected 'ab', got %q", p.Filter())
	}
}

func TestAppendItemsStreaming(t *testing.T) {
	p := New(theme.Default).WithItems(mk("alpha"))
	if p.TotalCount() != 1 {
		t.Fatalf("expected 1 item, got %d", p.TotalCount())
	}
	p = p.AppendItems(mk("beta", "gamma"))
	if p.TotalCount() != 3 {
		t.Fatalf("expected 3 items after append, got %d", p.TotalCount())
	}
}

func TestEnterOnEmptyDoesNothing(t *testing.T) {
	p := New(theme.Default).WithItems(mk("alpha"))
	p = sendKeys(p, "zzzz") // no match
	if p.Count() != 0 {
		t.Fatalf("expected 0 matches, got %d", p.Count())
	}
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no Cmd on empty Enter")
	}
}

func TestViewRendersWithoutPanic(t *testing.T) {
	p := New(theme.Default).WithItems(mk("alpha", "beta", "gamma")).WithSize(60, 10)
	out := p.View()
	if out == "" {
		t.Fatal("View() returned empty string")
	}
	if !strings.Contains(out, "Pick") {
		t.Fatalf("expected title 'Pick' in output:\n%s", out)
	}
}

func TestPreviewPaneWhenWide(t *testing.T) {
	p := New(theme.Default).
		WithItems(mk("file.go")).
		WithSize(120, 20).
		WithPreview(func(it Item) string { return "preview of " + it.Title })
	out := p.View()
	if !strings.Contains(out, "preview of file.go") {
		t.Fatalf("expected preview content, got:\n%s", out)
	}
}

func TestNarrowHidesPreview(t *testing.T) {
	p := New(theme.Default).
		WithItems(mk("file.go")).
		WithSize(60, 10).
		WithPreview(func(it Item) string { return "PREVIEW" })
	out := p.View()
	if strings.Contains(out, "PREVIEW") {
		t.Fatalf("expected no preview at narrow width, got:\n%s", out)
	}
}

func TestNoSubsequenceIsZero(t *testing.T) {
	if s := Score("hello", "world"); s != 0 {
		t.Fatalf("expected 0 score for non-subsequence, got %d", s)
	}
}

func TestUnrelatedFilterMatchesNothing(t *testing.T) {
	p := New(theme.Default).WithItems(mk("alpha", "beta"))
	p = sendKeys(p, "zzz")
	if p.Count() != 0 {
		t.Fatalf("expected 0 matches for 'zzz', got %d", p.Count())
	}
}

func TestSpaceIsAcceptedInFilter(t *testing.T) {
	p := New(theme.Default).WithItems(mk("hello world"))
	p, _ = p.Update(runeMsg("h"))
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeySpace})
	p, _ = p.Update(runeMsg("w"))
	if p.Filter() != "h w" {
		t.Fatalf("expected 'h w', got %q", p.Filter())
	}
}

// "café" written two ways: NFC has the precomposed é (U+00E9); NFD has a
// bare e (U+0065) followed by a combining acute (U+0301). They render
// identically and name the same file, but a rune-by-rune match treats them
// as different sequences. macOS stores filenames in NFD, while most input
// methods emit NFC, so the file on disk and the query you type routinely
// disagree on form.
const (
	cafeNFC = "caf\u00e9"          // café, precomposed
	cafeNFD = "cafe\u0301"         // café, decomposed
	resNFD  = "re\u0301sume\u0301" // résumé, decomposed
)

func TestNFDTargetMatchesNFCQuery(t *testing.T) {
	// File on disk is NFD (macOS), the typed query is NFC.
	if s := Score(cafeNFD, cafeNFC); s == 0 {
		t.Fatalf("NFD filename %q should match NFC query %q, scored 0", cafeNFD, cafeNFC)
	}
}

func TestNFCTargetMatchesNFDQuery(t *testing.T) {
	// The reverse: NFC filename, NFD query (paste from an NFD source).
	if s := Score(cafeNFC, cafeNFD); s == 0 {
		t.Fatalf("NFC filename %q should match NFD query %q, scored 0", cafeNFC, cafeNFD)
	}
}

func TestNFDSubsequenceMatchesNFCPrefix(t *testing.T) {
	// Typing a precomposed prefix should still match an NFD filename.
	if s := Score(resNFD, "r\u00e9s"); s == 0 {
		t.Fatalf("NFD filename %q should match NFC prefix %q, scored 0", resNFD, "r\u00e9s")
	}
}

func TestUnrelatedQueryStillMissesAfterNormalization(t *testing.T) {
	// Normalization must not turn a genuine non-match into a match.
	if s := Score(cafeNFD, "xyz"); s != 0 {
		t.Fatalf("unrelated query should still miss, got %d", s)
	}
}
