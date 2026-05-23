package breadcrumb

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
	if got := Render(nil, Options{}); got != "" {
		t.Fatalf("Render(nil) should be empty, got %q", got)
	}
}

func TestRenderContainsAllLabels(t *testing.T) {
	out := Render([]Crumb{{Label: "project"}, {Label: "src"}, {Label: "main.go"}}, Options{})
	for _, want := range []string{"project", "src", "main.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q, got %q", want, out)
		}
	}
}

func TestDefaultSeparatorAppears(t *testing.T) {
	out := Render([]Crumb{{Label: "a"}, {Label: "b"}}, Options{})
	if !strings.Contains(out, DefaultSeparator) {
		t.Fatalf("expected default separator %q in output, got %q", DefaultSeparator, out)
	}
}

func TestCustomSeparator(t *testing.T) {
	out := Render([]Crumb{{Label: "a"}, {Label: "b"}}, Options{Separator: " | "})
	if !strings.Contains(out, " | ") {
		t.Fatalf("custom separator should appear, got %q", out)
	}
}

func TestRenderPathSplitsOnSlashes(t *testing.T) {
	out := RenderPath("project/src/cmd/main.go", Options{})
	for _, want := range []string{"project", "src", "cmd", "main.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderPath should include %q, got %q", want, out)
		}
	}
}

func TestRenderPathStripsLeadingTrailingSlash(t *testing.T) {
	out := RenderPath("/foo/bar/", Options{})
	if strings.Contains(out, "  ") {
		t.Fatalf("trim should not leave empty crumbs, got %q", out)
	}
}

func TestMaxItemsCollapsesMiddleWithEllipsis(t *testing.T) {
	long := []Crumb{
		{Label: "root"}, {Label: "a"}, {Label: "b"}, {Label: "c"},
		{Label: "d"}, {Label: "e"}, {Label: "leaf"},
	}
	out := Render(long, Options{MaxItems: 4})
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis crumb when over MaxItems, got %q", out)
	}
	if !strings.Contains(out, "root") {
		t.Errorf("root crumb should survive collapse, got %q", out)
	}
	if !strings.Contains(out, "leaf") {
		t.Errorf("leaf crumb should survive collapse, got %q", out)
	}
	if strings.Contains(out, " a ") || strings.Contains(out, " b ") {
		t.Errorf("middle crumbs should collapse, got %q", out)
	}
}

func TestMaxItemsNoCollapseWhenWithinLimit(t *testing.T) {
	out := Render([]Crumb{{Label: "a"}, {Label: "b"}}, Options{MaxItems: 4})
	if strings.Contains(out, "…") {
		t.Fatalf("no ellipsis expected for short trails, got %q", out)
	}
}

func TestActiveCrumbBolded(t *testing.T) {
	out := Render([]Crumb{{Label: "a"}, {Label: "active"}}, Options{})
	// lipgloss emits SGR 1 for bold, either standalone ("\x1b[1m") or
	// combined with foreground ("\x1b[1;38;..."). Accept either form.
	if !strings.Contains(out, "\x1b[1m") && !strings.Contains(out, "\x1b[1;") && !strings.Contains(out, ";1m") {
		t.Fatalf("expected bold SGR escape in output, got %q", out)
	}
}

func TestIconPrependedToLabel(t *testing.T) {
	out := Render([]Crumb{{Icon: "📁", Label: "src"}}, Options{})
	if !strings.Contains(out, "📁") || !strings.Contains(out, "src") {
		t.Fatalf("expected icon+label, got %q", out)
	}
}
