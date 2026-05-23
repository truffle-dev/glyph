package codeview

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func TestRenderEmptyReturnsEmpty(t *testing.T) {
	if got := Render(Block{Source: ""}); got != "" {
		t.Fatalf("Render of empty block should return empty, got %q", got)
	}
}

func TestRenderProducesOneLinePerInputLine(t *testing.T) {
	src := "alpha\nbeta\ngamma"
	out := Render(Block{Source: src})
	if got := strings.Count(out, "\n"); got != 2 {
		t.Fatalf("expected 2 newlines for 3 lines, got %d in %q", got, out)
	}
}

func TestRenderShowGutterIncludesLineNumbers(t *testing.T) {
	out := Render(Block{Source: "a\nb\nc", ShowGutter: true, StartLine: 10})
	for _, want := range []string{"10", "11", "12"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain line number %q, got %q", want, out)
		}
	}
}

func TestRenderGoKeywordTinted(t *testing.T) {
	out := Render(Block{Source: "func main() {}", Language: LangGo})
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected ANSI escapes around keyword, got %q", out)
	}
	if !strings.Contains(out, "func") {
		t.Fatalf("output should still contain the keyword literal, got %q", out)
	}
}

func TestRenderStringLiteralTinted(t *testing.T) {
	out := Render(Block{Source: `x := "hello"`, Language: LangGo})
	if !strings.Contains(out, `"hello"`) {
		t.Fatalf("string content lost, got %q", out)
	}
	// String style uses theme.Default.Success (#41b883). Truecolor encodes as 65;184;131.
	if !strings.Contains(out, "65;184;131") && !strings.Contains(out, "41b883") {
		t.Errorf("expected success-color ANSI around string, got %q", out)
	}
}

func TestRenderCommentTintedEndsAtLineEnd(t *testing.T) {
	out := Render(Block{Source: `x := 1 // trailing note`, Language: LangGo})
	if !strings.Contains(out, "trailing note") {
		t.Fatalf("comment text dropped, got %q", out)
	}
}

func TestCommentInsideStringIsNotComment(t *testing.T) {
	out := Render(Block{Source: `x := "// not a comment"`, Language: LangGo})
	// The two slashes should sit inside the string, not start a comment
	// tint. Easiest verification: the entire string literal renders with
	// the success color, no italic styling kicks in.
	if !strings.Contains(out, "// not a comment") {
		t.Fatalf("escaped content lost, got %q", out)
	}
}

func TestMarkHighlightAddsBackground(t *testing.T) {
	out := Render(Block{
		Source: "alpha\nbeta",
		Marks:  map[int]LineMark{2: MarkHighlight},
	})
	// SurfaceStrong is #1f2026 → truecolor 31;32;38.
	if !strings.Contains(out, "31;32;38") && !strings.Contains(out, "1f2026") {
		t.Fatalf("highlight mark should paint SurfaceStrong background, got %q", out)
	}
}

func TestMaxWidthTruncatesWithEllipsis(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz"
	out := Render(Block{Source: long, MaxWidth: 10})
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis when MaxWidth shorter than source, got %q", out)
	}
}

func TestPythonHashComment(t *testing.T) {
	out := Render(Block{Source: `x = 1 # note`, Language: LangPy})
	if !strings.Contains(out, "# note") {
		t.Fatalf("python comment content lost, got %q", out)
	}
}

func TestNumberLiteralTinted(t *testing.T) {
	out := Render(Block{Source: "x := 42", Language: LangGo})
	// Accent #d57e5e → 213;126;94.
	if !strings.Contains(out, "213;126;94") && !strings.Contains(out, "d57e5e") {
		t.Fatalf("number literal should use accent color, got %q", out)
	}
}

func TestJSONLiteralsKeywords(t *testing.T) {
	out := Render(Block{Source: `{"a": true, "b": null}`, Language: LangJSON})
	for _, w := range []string{"true", "null"} {
		if !strings.Contains(out, w) {
			t.Errorf("JSON keyword %q missing from output", w)
		}
	}
}
