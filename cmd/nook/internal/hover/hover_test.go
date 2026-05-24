package hover

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

func testTheme() theme.Theme {
	return theme.Theme{
		Bg:      lipgloss.Color("#000"),
		Surface: lipgloss.Color("#111"),
		Border:  lipgloss.Color("#444"),
		Text:    lipgloss.Color("#eee"),
	}
}

// TestEmptyContentsRendersEmpty confirms View returns "" for empty or
// whitespace-only input — the host renders unconditionally, so we
// collapse out instead of drawing an empty bordered box.
func TestEmptyContentsRendersEmpty(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "   ", "\n\n\t"} {
		if got := View(testTheme(), in, 60, 8); got != "" {
			t.Errorf("View(%q) = %q, want empty", in, got)
		}
	}
}

// TestNonEmptyContentsAppear confirms the body shows up inside the
// rendered output. We don't golden-test the exact ANSI escape sequence
// because lipgloss may re-flow it between releases — but the readable
// text must survive.
func TestNonEmptyContentsAppear(t *testing.T) {
	t.Parallel()
	contents := "func Add(a, b int) int\nAdd returns a+b."
	out := View(testTheme(), contents, 60, 8)
	if out == "" {
		t.Fatal("View returned empty for non-empty input")
	}
	if !strings.Contains(out, "Add returns a+b.") {
		t.Errorf("rendered output missing body line: %q", out)
	}
	if !strings.Contains(out, "func Add(a, b int) int") {
		t.Errorf("rendered output missing signature line: %q", out)
	}
}

// TestClampToMaxLines confirms output past maxLines is replaced with a
// trailing "…" row instead of being dropped silently.
func TestClampToMaxLines(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("line\n", 20)
	out := View(testTheme(), long, 60, 4)
	if !strings.Contains(out, "…") {
		t.Errorf("clamp marker missing from output: %q", out)
	}
}

// TestWrapAndClampHardWraps confirms a single long line gets hard-
// wrapped at the inner width.
func TestWrapAndClampHardWraps(t *testing.T) {
	t.Parallel()
	in := strings.Repeat("x", 200)
	out := wrapAndClamp(in, 20, 100)
	for _, line := range strings.Split(out, "\n") {
		if len(line) > 20 {
			t.Errorf("wrap produced a %d-char line, want <=20: %q", len(line), line)
		}
	}
}

// TestNarrowWidthClampsToMinimum confirms width below minWidth gets
// promoted up so the box stays legible. We just check the rendered
// output isn't empty.
func TestNarrowWidthClampsToMinimum(t *testing.T) {
	t.Parallel()
	out := View(testTheme(), "short", 8, 4)
	if out == "" {
		t.Fatal("narrow-width render should still produce output")
	}
}
