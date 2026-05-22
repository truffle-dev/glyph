package markdownviewer

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestEmptySourceRendersBlank(t *testing.T) {
	v := New(theme.Default).WithSize(40, 5)
	out := v.View()
	if out == "" {
		t.Fatal("View() should return space-padded height even on empty source")
	}
	if got := strings.Count(out, "\n"); got != 4 {
		t.Fatalf("want 4 newlines (5 lines), got %d", got)
	}
}

func TestRendersHeading(t *testing.T) {
	v := New(theme.Default).WithSize(60, 5).WithSource("# Hello")
	out := v.View()
	if !strings.Contains(out, "Hello") {
		t.Fatalf("expected heading text in output, got %q", out)
	}
	if strings.Contains(out, "# Hello") {
		t.Fatalf("heading marker should be stripped, got %q", out)
	}
}

func TestRendersBulletList(t *testing.T) {
	src := "- first\n- second"
	v := New(theme.Default).WithSize(60, 5).WithSource(src)
	out := v.View()
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Fatalf("both items must render, got %q", out)
	}
	if !strings.Contains(out, "•") {
		t.Fatalf("bullet marker must render, got %q", out)
	}
}

func TestRendersCodeBlock(t *testing.T) {
	src := "```go\nfunc main() {}\n```"
	v := New(theme.Default).WithSize(60, 5).WithSource(src)
	out := v.View()
	if !strings.Contains(out, "func main() {}") {
		t.Fatalf("code block body must render, got %q", out)
	}
	if !strings.Contains(out, "go") {
		t.Fatalf("language label should render, got %q", out)
	}
	if strings.Contains(out, "```") {
		t.Fatalf("fence markers must not appear in rendered output, got %q", out)
	}
}

func TestRendersHorizontalRule(t *testing.T) {
	v := New(theme.Default).WithSize(20, 5).WithSource("Above\n\n---\n\nBelow")
	out := v.View()
	if !strings.Contains(out, "─") {
		t.Fatalf("hr should render box-drawing dashes, got %q", out)
	}
}

func TestRendersBlockquote(t *testing.T) {
	v := New(theme.Default).WithSize(40, 5).WithSource("> quoted line")
	out := v.View()
	if !strings.Contains(out, "quoted line") {
		t.Fatalf("blockquote text must render, got %q", out)
	}
}

func TestRendersInlineBoldItalicCode(t *testing.T) {
	v := New(theme.Default).WithSize(60, 5).WithSource("a **bold** b *it* c `code` d")
	out := v.View()
	for _, want := range []string{"bold", "it", "code", "a ", " b ", " d"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output %q", want, out)
		}
	}
	if strings.Contains(out, "**") {
		t.Fatalf("bold markers must be stripped, got %q", out)
	}
}

func TestRendersLink(t *testing.T) {
	v := New(theme.Default).WithSize(80, 5).WithSource("see [docs](https://example.com)")
	out := v.View()
	if !strings.Contains(out, "docs") {
		t.Fatalf("link text must render, got %q", out)
	}
	if !strings.Contains(out, "example.com") {
		t.Fatalf("link url must render, got %q", out)
	}
}

func TestScrollDownClampsAtMaxOffset(t *testing.T) {
	src := strings.Repeat("paragraph line\n\n", 30)
	v := New(theme.Default).WithSize(40, 5).WithSource(src)
	for i := 0; i < 1000; i++ {
		v, _ = v.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	max := v.TotalLines() - 5
	if v.Offset() != max {
		t.Fatalf("offset should clamp to TotalLines-height (%d), got %d", max, v.Offset())
	}
}

func TestScrollUpClampsAtZero(t *testing.T) {
	v := New(theme.Default).WithSize(40, 5).WithSource("a\nb\nc")
	for i := 0; i < 50; i++ {
		v, _ = v.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	if v.Offset() != 0 {
		t.Fatalf("scroll up below zero must clamp to 0, got %d", v.Offset())
	}
}

func TestEndJumpsToBottom(t *testing.T) {
	src := strings.Repeat("line\n", 30)
	v := New(theme.Default).WithSize(40, 5).WithSource(src)
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyEnd})
	max := v.TotalLines() - 5
	if v.Offset() != max {
		t.Fatalf("End must jump to bottom (%d), got %d", max, v.Offset())
	}
}

func TestHomeJumpsToTop(t *testing.T) {
	src := strings.Repeat("line\n", 30)
	v := New(theme.Default).WithSize(40, 5).WithSource(src)
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyEnd})
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyHome})
	if v.Offset() != 0 {
		t.Fatalf("Home must return to 0, got %d", v.Offset())
	}
}

func TestSourceShorterThanHeightHasZeroMaxOffset(t *testing.T) {
	v := New(theme.Default).WithSize(40, 20).WithSource("one\ntwo\nthree")
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyDown})
	if v.Offset() != 0 {
		t.Fatalf("scroll on short source must stay at 0, got %d", v.Offset())
	}
}

func TestWithSizeClampsMinimums(t *testing.T) {
	v := New(theme.Default).WithSize(2, 0)
	if got := v.View(); got == "" {
		t.Fatal("clamped size should still render")
	}
}

func TestInlineMarkersNotRequiredToBalance(t *testing.T) {
	v := New(theme.Default).WithSize(40, 3).WithSource("a *unterminated italic")
	out := v.View()
	if !strings.Contains(out, "unterminated italic") {
		t.Fatalf("unterminated inline marker should not eat following text, got %q", out)
	}
}
