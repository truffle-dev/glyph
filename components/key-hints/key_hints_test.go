package keyhints

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestEmptyViewIsEmpty(t *testing.T) {
	b := New(theme.Default)
	if b.View() != "" {
		t.Fatalf("empty hint bar should render empty, got %q", b.View())
	}
}

func TestViewIncludesKeysAndDescs(t *testing.T) {
	b := New(theme.Default).WithHints([]Hint{
		{Key: "Tab", Desc: "next pane"},
		{Key: "q", Desc: "quit"},
	})
	out := b.View()
	for _, want := range []string{"Tab", "next pane", "q", "quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("View should include %q, got %q", want, out)
		}
	}
}

func TestSeparatorRendersBetweenHints(t *testing.T) {
	b := New(theme.Default).WithSeparator(" · ").WithHints([]Hint{
		{Key: "a", Desc: "x"},
		{Key: "b", Desc: "y"},
	})
	out := b.View()
	if !strings.Contains(out, "·") {
		t.Fatalf("custom separator should appear, got %q", out)
	}
}

func TestWidthClampsOutput(t *testing.T) {
	b := New(theme.Default).WithHints([]Hint{
		{Key: "Tab", Desc: "next pane"},
		{Key: "Shift+Tab", Desc: "previous pane"},
		{Key: "q", Desc: "quit"},
		{Key: "l", Desc: "logs"},
		{Key: "t", Desc: "toast"},
	}).WithWidth(20)
	out := b.View()
	if got := lipgloss.Width(out); got > 20 {
		t.Fatalf("width should clamp to 20, got %d (%q)", got, out)
	}
}

func TestSingleHintRendersWithoutSeparator(t *testing.T) {
	b := New(theme.Default).WithHints([]Hint{{Key: "?", Desc: "help"}}).WithSeparator(" · ")
	out := b.View()
	if strings.Contains(out, "·") {
		t.Fatalf("single hint should not include separator, got %q", out)
	}
}

func TestDefaultSeparatorIsTwoSpaces(t *testing.T) {
	if DefaultSeparator != "  " {
		t.Fatalf("DefaultSeparator should be two spaces, got %q", DefaultSeparator)
	}
}
