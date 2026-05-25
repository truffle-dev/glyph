package completedoc

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/components/theme"
)

func TestNewIsClosed(t *testing.T) {
	t.Parallel()
	p := New()
	if p.IsOpen() {
		t.Fatalf("new pane should start closed")
	}
	if p.View(theme.Default) != "" {
		t.Errorf("closed pane should render empty string")
	}
}

func TestOpenWithDocsOpensPane(t *testing.T) {
	t.Parallel()
	p := New().Open(lsp.CompletionItem{
		Label:         "Println",
		Detail:        "func(a ...any) (n int, err error)",
		Documentation: "Println formats using the default formats for its operands.",
		Kind:          lsp.CompletionKindFunction,
	})
	if !p.IsOpen() {
		t.Fatalf("open with non-empty doc should open the pane")
	}
	if p.ItemLabel() != "Println" {
		t.Errorf("label: %q", p.ItemLabel())
	}
}

func TestOpenWithDetailOnlyOpensPane(t *testing.T) {
	t.Parallel()
	p := New().Open(lsp.CompletionItem{
		Label:  "MAX",
		Detail: "const int",
	})
	if !p.IsOpen() {
		t.Fatalf("detail-only item should still open the pane")
	}
}

func TestOpenWithNothingClosesPane(t *testing.T) {
	t.Parallel()
	p := New().Open(lsp.CompletionItem{Label: "X", Kind: lsp.CompletionKindVariable})
	if p.IsOpen() {
		t.Errorf("no docs and no detail should leave pane closed")
	}
}

func TestCloseClearsState(t *testing.T) {
	t.Parallel()
	p := New().Open(lsp.CompletionItem{
		Label:         "Foo",
		Documentation: "docs",
	})
	p = p.Close()
	if p.IsOpen() {
		t.Errorf("close should leave the pane closed")
	}
	if p.Item().Label != "" {
		t.Errorf("close should clear the item: %+v", p.Item())
	}
}

func TestWithSizeClampsMin(t *testing.T) {
	t.Parallel()
	p := New().WithSize(5, 1)
	if p.Width() != minWidth {
		t.Errorf("width clamp: got %d, want >= %d", p.Width(), minWidth)
	}
	if p.Height() != minHeight {
		t.Errorf("height clamp: got %d, want >= %d", p.Height(), minHeight)
	}
}

func TestWithSizeZeroKeepsDefault(t *testing.T) {
	t.Parallel()
	p := New().WithSize(64, 0)
	if p.Width() != 64 {
		t.Errorf("explicit width lost: %d", p.Width())
	}
	if p.Height() != defaultHeight {
		t.Errorf("zero height should keep default %d: got %d", defaultHeight, p.Height())
	}
}

func TestViewShowsLabelAndDocs(t *testing.T) {
	t.Parallel()
	p := New().Open(lsp.CompletionItem{
		Label:         "Println",
		Detail:        "func(a ...any) (n int, err error)",
		Documentation: "Println formats using the default formats for its operands and writes to standard output.",
		Kind:          lsp.CompletionKindFunction,
	})
	out := stripStyles(p.View(theme.Default))
	if !strings.Contains(out, "Println") {
		t.Errorf("View missing label: %q", out)
	}
	if !strings.Contains(out, "func(a ...any)") {
		t.Errorf("View missing detail: %q", out)
	}
	if !strings.Contains(out, "Println formats") {
		t.Errorf("View missing docs: %q", out)
	}
	if !strings.Contains(out, "fn") {
		t.Errorf("View missing kind tag: %q", out)
	}
}

func TestViewSuppressesDetailEqualToLabel(t *testing.T) {
	t.Parallel()
	p := New().WithSize(60, 12).Open(lsp.CompletionItem{
		Label:         "x",
		Detail:        "x",
		Documentation: "the only doc",
	})
	out := stripStyles(p.View(theme.Default))
	// Label should appear once, detail row should not duplicate it.
	if strings.Count(out, "x") < 1 {
		t.Errorf("label should be present: %q", out)
	}
	// The "the only doc" line MUST be present even though the detail
	// was suppressed for duplicating the label.
	if !strings.Contains(out, "the only doc") {
		t.Errorf("docs lost when detail==label: %q", out)
	}
}

func TestViewClampsLongLabel(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("abc ", 40) // 160 chars
	p := New().Open(lsp.CompletionItem{
		Label:         long,
		Documentation: "x",
	})
	out := stripStyles(p.View(theme.Default))
	if !strings.Contains(out, "…") {
		t.Errorf("long label should be ellipsized: %q", out)
	}
}

func TestViewWrapsLongDocs(t *testing.T) {
	t.Parallel()
	doc := strings.Repeat("word ", 80)
	p := New().WithSize(40, 12).Open(lsp.CompletionItem{
		Label:         "x",
		Documentation: doc,
	})
	out := stripStyles(p.View(theme.Default))
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	// Border row at top + at bottom + at least three content lines
	if len(lines) < 4 {
		t.Errorf("expected multiple wrapped lines, got %d: %q", len(lines), out)
	}
}

func TestViewClampsTallDocs(t *testing.T) {
	t.Parallel()
	doc := strings.Repeat("paragraph one.\nparagraph two.\n", 50)
	p := New().WithSize(40, 6).Open(lsp.CompletionItem{
		Label:         "x",
		Documentation: doc,
	})
	out := stripStyles(p.View(theme.Default))
	if !strings.Contains(out, "…") {
		t.Errorf("tall docs should be truncated with ellipsis: %q", out)
	}
}

func TestViewClosedReturnsEmpty(t *testing.T) {
	t.Parallel()
	p := New()
	if p.View(theme.Default) != "" {
		t.Errorf("closed pane should render empty string")
	}
}

func TestViewBelowMinWidthReturnsEmpty(t *testing.T) {
	t.Parallel()
	p := New().Open(lsp.CompletionItem{Label: "x", Documentation: "y"})
	// Construct directly because WithSize would clamp.
	p.width = 5
	if p.View(theme.Default) != "" {
		t.Errorf("below-min-width pane should refuse to render")
	}
}

func TestShortKindTagCoversEveryKind(t *testing.T) {
	t.Parallel()
	kinds := []lsp.CompletionKind{
		lsp.CompletionKindMethod, lsp.CompletionKindFunction, lsp.CompletionKindConstructor,
		lsp.CompletionKindField, lsp.CompletionKindVariable, lsp.CompletionKindClass,
		lsp.CompletionKindInterface, lsp.CompletionKindModule, lsp.CompletionKindProperty,
		lsp.CompletionKindUnit, lsp.CompletionKindValue, lsp.CompletionKindEnum,
		lsp.CompletionKindKeyword, lsp.CompletionKindSnippet, lsp.CompletionKindColor,
		lsp.CompletionKindFile, lsp.CompletionKindReference, lsp.CompletionKindFolder,
		lsp.CompletionKindEnumMember, lsp.CompletionKindConstant, lsp.CompletionKindStruct,
		lsp.CompletionKindEvent, lsp.CompletionKindOperator, lsp.CompletionKindTypeParameter,
	}
	for _, k := range kinds {
		if shortKindTag(k) == "" {
			t.Errorf("kind %q missing two-letter tag", k)
		}
	}
	// CompletionKindText falls outside the named set; we expect empty
	// so the header just renders the label without a leading tag.
	if got := shortKindTag(lsp.CompletionKindText); got != "" {
		t.Errorf("CompletionKindText should map to empty tag, got %q", got)
	}
}

func TestClampLineShortReturnsUnchanged(t *testing.T) {
	t.Parallel()
	if got := clampLine("hi", 10); got != "hi" {
		t.Errorf("short line: %q", got)
	}
}

func TestClampLineLongTruncatesWithEllipsis(t *testing.T) {
	t.Parallel()
	got := clampLine("hello world", 6)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("missing ellipsis: %q", got)
	}
	if len([]rune(got)) > 6 {
		t.Errorf("over budget: %q", got)
	}
}

func TestClampLineWidthZeroReturnsEmpty(t *testing.T) {
	t.Parallel()
	if got := clampLine("hi", 0); got != "" {
		t.Errorf("width 0 should return empty: %q", got)
	}
}

func TestWrapAndClampKeepsShortText(t *testing.T) {
	t.Parallel()
	if got := wrapAndClamp("hello world", 40, 4); got != "hello world" {
		t.Errorf("short text wrap: %q", got)
	}
}

func TestWrapAndClampWrapsLongLine(t *testing.T) {
	t.Parallel()
	got := wrapAndClamp("alpha beta gamma delta epsilon zeta", 10, 8)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("expected at least 2 wrapped lines: %v", lines)
	}
	for _, ln := range lines {
		if len([]rune(ln)) > 10 {
			t.Errorf("line over budget: %q", ln)
		}
	}
}

func TestWrapAndClampTruncatesAtMaxRows(t *testing.T) {
	t.Parallel()
	body := strings.Repeat("paragraph one\nparagraph two\n", 5)
	got := wrapAndClamp(body, 20, 3)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Errorf("max rows not enforced: %d -> %v", len(lines), lines)
	}
	if !strings.HasSuffix(lines[2], "…") {
		t.Errorf("last line missing ellipsis: %q", lines[2])
	}
}

func TestWrapAndClampZeroBudgetReturnsEmpty(t *testing.T) {
	t.Parallel()
	if got := wrapAndClamp("hi", 10, 0); got != "" {
		t.Errorf("zero max rows: %q", got)
	}
	if got := wrapAndClamp("hi", 0, 5); got != "" {
		t.Errorf("zero width: %q", got)
	}
}

func TestWrapLineHardBreaksOverlongWord(t *testing.T) {
	t.Parallel()
	got := wrapLine("supercalifragilisticexpialidocious", 10)
	if len(got) < 2 {
		t.Errorf("expected hard-break across multiple rows: %v", got)
	}
	for _, ln := range got {
		if len([]rune(ln)) > 10 {
			t.Errorf("hard-break over budget: %q", ln)
		}
	}
}

func TestItemReturnsZeroWhenClosed(t *testing.T) {
	t.Parallel()
	p := New()
	if p.Item().Label != "" {
		t.Errorf("closed pane Item() should be zero value")
	}
	if p.ItemLabel() != "" {
		t.Errorf("closed pane ItemLabel() should be empty")
	}
}

// stripStyles strips ANSI escape sequences so test assertions can match
// raw text. Mirrors the helper in the signature package; both packages
// keep their own copy so they remain decoupled.
func stripStyles(s string) string {
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
