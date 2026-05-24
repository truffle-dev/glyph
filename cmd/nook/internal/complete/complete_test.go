package complete

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/components/theme"
)

func testTheme() theme.Theme {
	return theme.Theme{
		Bg:        lipgloss.Color("#000"),
		Surface:   lipgloss.Color("#111"),
		Border:    lipgloss.Color("#444"),
		Text:      lipgloss.Color("#eee"),
		TextMuted: lipgloss.Color("#888"),
		Primary:   lipgloss.Color("#a0f"),
	}
}

// TestEmptyRendersEmpty confirms an unset popup or one with no items
// returns "" so the host can render unconditionally.
func TestEmptyRendersEmpty(t *testing.T) {
	t.Parallel()
	if out := New().View(testTheme(), 40, 8); out != "" {
		t.Errorf("empty popup View = %q, want empty", out)
	}
	if out := New().WithItems(nil, 0).View(testTheme(), 40, 8); out != "" {
		t.Errorf("nil-items popup View = %q, want empty", out)
	}
}

// TestSelectedReturnsHighlighted confirms Selected tracks MoveUp/MoveDown
// and wraps at both ends.
func TestSelectedReturnsHighlighted(t *testing.T) {
	t.Parallel()
	items := []nooklsp.CompletionItem{
		{Label: "alpha", InsertText: "alpha"},
		{Label: "beta", InsertText: "beta"},
		{Label: "gamma", InsertText: "gamma"},
	}
	p := New().WithItems(items, 0)

	got, ok := p.Selected()
	if !ok || got.Label != "alpha" {
		t.Fatalf("initial Selected = %v (ok=%v), want alpha", got, ok)
	}

	p = p.MoveDown()
	if got, _ := p.Selected(); got.Label != "beta" {
		t.Errorf("after MoveDown Selected = %q, want beta", got.Label)
	}

	p = p.MoveDown().MoveDown() // gamma -> wrap to alpha
	if got, _ := p.Selected(); got.Label != "alpha" {
		t.Errorf("after wrap-down Selected = %q, want alpha", got.Label)
	}

	p = p.MoveUp() // alpha -> wrap to gamma
	if got, _ := p.Selected(); got.Label != "gamma" {
		t.Errorf("after wrap-up Selected = %q, want gamma", got.Label)
	}
}

// TestSelectedOnEmptyPopup confirms Selected returns false when there's
// nothing to select.
func TestSelectedOnEmptyPopup(t *testing.T) {
	t.Parallel()
	p := New()
	if _, ok := p.Selected(); ok {
		t.Errorf("Selected on empty popup returned ok=true, want false")
	}
	if p2 := p.MoveDown().MoveUp(); !p2.Empty() {
		t.Errorf("Move* on empty popup should remain empty")
	}
}

// TestPrefixLenStored confirms WithItems retains the prefix length and
// clamps a negative input to 0.
func TestPrefixLenStored(t *testing.T) {
	t.Parallel()
	items := []nooklsp.CompletionItem{{Label: "fmt", InsertText: "fmt"}}
	if got := New().WithItems(items, 3).PrefixLen(); got != 3 {
		t.Errorf("PrefixLen = %d, want 3", got)
	}
	if got := New().WithItems(items, -2).PrefixLen(); got != 0 {
		t.Errorf("PrefixLen for negative input = %d, want 0", got)
	}
}

// TestViewContainsAllItemLabels confirms each item's label survives
// through the rendered popup at a reasonable width.
func TestViewContainsAllItemLabels(t *testing.T) {
	t.Parallel()
	items := []nooklsp.CompletionItem{
		{Label: "Println", InsertText: "Println", Kind: nooklsp.CompletionKindFunction, Detail: "func(...any)"},
		{Label: "Printf", InsertText: "Printf", Kind: nooklsp.CompletionKindFunction, Detail: "func(string, ...any)"},
	}
	out := stripANSI(New().WithItems(items, 0).View(testTheme(), 60, 8))
	for _, it := range items {
		if !strings.Contains(out, it.Label) {
			t.Errorf("popup view missing label %q\n--- view ---\n%s", it.Label, out)
		}
	}
}

// TestViewClampsToWindow confirms a long list with maxRows=4 only shows
// four entries at a time and includes the selected one.
func TestViewClampsToWindow(t *testing.T) {
	t.Parallel()
	var items []nooklsp.CompletionItem
	for i := 0; i < 20; i++ {
		items = append(items, nooklsp.CompletionItem{
			Label:      "item-" + string(rune('A'+i)),
			InsertText: "item",
		})
	}
	p := New().WithItems(items, 0)
	for i := 0; i < 15; i++ { // move selection deep
		p = p.MoveDown()
	}
	out := stripANSI(p.View(testTheme(), 50, 4))

	rendered := 0
	for _, it := range items {
		if strings.Contains(out, it.Label) {
			rendered++
		}
	}
	if rendered == 0 || rendered > 4 {
		t.Errorf("clamped popup rendered %d items, want 1-4", rendered)
	}

	// The selected item (index 15 = item-P) must be visible.
	if !strings.Contains(out, "item-P") {
		t.Errorf("selected item not visible in window:\n%s", out)
	}
}

// TestWordPrefixExtractsTrailingIdent walks the identifier-prefix
// helper through the cases the host actually feeds it: bare identifier,
// dot-led member access, leading whitespace, mixed punctuation.
func TestWordPrefixExtractsTrailingIdent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"fmt", "fmt"},
		{"fmt.Print", "Print"},
		{"fmt.", ""},
		{"foo(bar", "bar"},
		{"my_var_2", "my_var_2"},
		{"package main\nfn", "fn"},
		{"x := 42; y := abc", "abc"},
	}
	for _, c := range cases {
		if got := WordPrefix(c.in); got != c.want {
			t.Errorf("WordPrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNarrowWidthStillRenders confirms an undersized width promotes up
// to minWidth so the popup remains legible.
func TestNarrowWidthStillRenders(t *testing.T) {
	t.Parallel()
	items := []nooklsp.CompletionItem{{Label: "x", InsertText: "x"}}
	out := New().WithItems(items, 0).View(testTheme(), 4, 8)
	if out == "" {
		t.Fatal("narrow-width render should still produce output")
	}
	if !strings.Contains(stripANSI(out), "x") {
		t.Errorf("narrow render dropped label: %q", out)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' || r == 'J' || r == 'H' || r == 'K' {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
