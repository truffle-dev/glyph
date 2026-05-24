package codeaction

import (
	"strings"
	"testing"

	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/components/theme"
)

func sampleItems() []nooklsp.CodeActionItem {
	return []nooklsp.CodeActionItem{
		{Title: "Organize imports", Kind: "source.organizeImports", IsPreferred: true},
		{Title: "Extract function 'foo'", Kind: "refactor.extract"},
		{Title: "Inline variable 'bar'", Kind: "refactor.inline"},
	}
}

func TestEmptyPopupRendersNothing(t *testing.T) {
	t.Parallel()
	p := New()
	if !p.Empty() {
		t.Error("New() should produce an empty popup")
	}
	if p.View(theme.Light, 40, 8) != "" {
		t.Error("empty popup should render as empty string")
	}
	if _, ok := p.Selected(); ok {
		t.Error("empty popup Selected() should report not-ok")
	}
}

func TestWithItemsSelectsFirstEnabled(t *testing.T) {
	t.Parallel()
	items := []nooklsp.CodeActionItem{
		{Title: "disabled action", Disabled: "needs gopls 1.x"},
		{Title: "enabled action"},
		{Title: "another enabled"},
	}
	p := New().WithItems(items)
	got, ok := p.Selected()
	if !ok || got.Title != "enabled action" {
		t.Errorf("Selected() = %v ok=%v, want enabled action ok=true", got, ok)
	}
}

func TestWithItemsAllDisabledStillStableCursor(t *testing.T) {
	t.Parallel()
	items := []nooklsp.CodeActionItem{
		{Title: "disabled 1", Disabled: "x"},
		{Title: "disabled 2", Disabled: "y"},
	}
	p := New().WithItems(items)
	if p.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", p.Len())
	}
	if _, ok := p.Selected(); ok {
		t.Error("Selected() on all-disabled should report not-ok")
	}
}

func TestMoveUpDownWraps(t *testing.T) {
	t.Parallel()
	p := New().WithItems(sampleItems())
	p = p.MoveDown().MoveDown()
	got, _ := p.Selected()
	if got.Title != "Inline variable 'bar'" {
		t.Errorf("after 2 MoveDown, selected = %q", got.Title)
	}
	p = p.MoveDown() // wrap to top
	got, _ = p.Selected()
	if got.Title != "Organize imports" {
		t.Errorf("after wrap-down, selected = %q", got.Title)
	}
	p = p.MoveUp() // wrap to bottom
	got, _ = p.Selected()
	if got.Title != "Inline variable 'bar'" {
		t.Errorf("after MoveUp wrap-up, selected = %q", got.Title)
	}
}

func TestViewRendersAllVisibleItems(t *testing.T) {
	t.Parallel()
	p := New().WithItems(sampleItems())
	out := p.View(theme.Light, 48, 8)
	if !strings.Contains(out, "Organize imports") {
		t.Errorf("view missing first item:\n%s", out)
	}
	if !strings.Contains(out, "Extract function") {
		t.Errorf("view missing second item:\n%s", out)
	}
	if !strings.Contains(out, "Inline variable") {
		t.Errorf("view missing third item:\n%s", out)
	}
}

func TestPreferredItemIsMarked(t *testing.T) {
	t.Parallel()
	p := New().WithItems(sampleItems())
	out := p.View(theme.Light, 48, 8)
	if !strings.Contains(out, "* Organize imports") {
		t.Errorf("preferred prefix '*' missing for first item:\n%s", out)
	}
}

func TestShortKindLabels(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"quickfix":               "fix",
		"source.organizeImports": "imports",
		"source":                 "source",
		"refactor":               "refactor",
		"refactor.extract":       "extract",
		"refactor.inline":        "inline",
		"refactor.rewrite":       "rewrite",
		"":                       "",
	}
	for in, want := range cases {
		if got := shortKind(in); got != want {
			t.Errorf("shortKind(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWindowBoundsScrollsAroundSelection(t *testing.T) {
	t.Parallel()
	start, end := windowBounds(0, 20, 5)
	if start != 0 || end != 5 {
		t.Errorf("windowBounds(0,20,5) = (%d,%d)", start, end)
	}
	start, end = windowBounds(19, 20, 5)
	if start != 15 || end != 20 {
		t.Errorf("windowBounds(19,20,5) = (%d,%d), want (15,20)", start, end)
	}
	start, end = windowBounds(10, 20, 5)
	if start < 0 || end > 20 || end-start != 5 {
		t.Errorf("windowBounds(10,20,5) = (%d,%d)", start, end)
	}
}

func TestDisabledItemRendersFaint(t *testing.T) {
	t.Parallel()
	items := []nooklsp.CodeActionItem{
		{Title: "enabled"},
		{Title: "disabled-item", Disabled: "needs gopls 1.x"},
	}
	p := New().WithItems(items)
	out := p.View(theme.Light, 40, 8)
	// Both rendered, but disabled gets its label included (faint styling
	// is escape-only — just confirm the title appears).
	if !strings.Contains(out, "disabled-item") {
		t.Errorf("disabled item not in view:\n%s", out)
	}
}
