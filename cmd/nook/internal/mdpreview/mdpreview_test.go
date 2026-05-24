package mdpreview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestIsMarkdownPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"README.md", true},
		{"docs/intro.md", true},
		{"NOTES.MD", true},
		{"changelog.markdown", true},
		{"foo.MARKDOWN", true},
		{"main.go", false},
		{"index.html", false},
		{"Makefile", false},
		{"", false},
		{".mdx", false},
	}
	for _, c := range cases {
		if got := IsMarkdownPath(c.path); got != c.want {
			t.Errorf("IsMarkdownPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestPaneDefaults(t *testing.T) {
	p := NewPane(theme.Default)
	if p.HasSource() {
		t.Fatal("new pane should not have source")
	}
	if p.Focused() {
		t.Fatal("new pane should not be focused")
	}
	if p.Path() != "" {
		t.Fatalf("new pane path = %q, want empty", p.Path())
	}
}

func TestPaneWithSourceSetsState(t *testing.T) {
	p := NewPane(theme.Default).
		WithSize(60, 20).
		WithSource("docs/foo.md", "# Title\n\nbody")

	if !p.HasSource() {
		t.Fatal("pane should report source after WithSource")
	}
	if p.Path() != "docs/foo.md" {
		t.Fatalf("Path() = %q, want docs/foo.md", p.Path())
	}

	view := p.View()
	if !strings.Contains(view, "foo.md") {
		t.Fatalf("View should mention basename, got:\n%s", view)
	}
}

func TestPaneViewShowsNoSourceMessage(t *testing.T) {
	p := NewPane(theme.Default).WithSize(40, 10)
	view := p.View()
	if !strings.Contains(view, "no markdown buffer") {
		t.Fatalf("empty pane view should mention no markdown buffer, got:\n%s", view)
	}
}

func TestPaneFocusBlur(t *testing.T) {
	p := NewPane(theme.Default).Focus()
	if !p.Focused() {
		t.Fatal("expected focused after Focus()")
	}
	p = p.Blur()
	if p.Focused() {
		t.Fatal("expected not focused after Blur()")
	}
}

func TestPaneEscEmitsCancel(t *testing.T) {
	p := NewPane(theme.Default).WithSize(40, 10).Focus()
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc on focused pane should emit a tea.Cmd")
	}
	msg := cmd()
	if _, ok := msg.(CancelMsg); !ok {
		t.Fatalf("Esc cmd should produce CancelMsg, got %T", msg)
	}
}

func TestPaneEscIgnoredWhenBlurred(t *testing.T) {
	p := NewPane(theme.Default).WithSize(40, 10)
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("Esc on blurred pane should not emit a cmd")
	}
}

func TestScrollKeysAdvanceViewer(t *testing.T) {
	// Build a long enough document that scroll is meaningful.
	var b strings.Builder
	for i := 0; i < 80; i++ {
		b.WriteString("line ")
		b.WriteString(strings.Repeat("x", 5))
		b.WriteString("\n")
	}
	p := NewPane(theme.Default).
		WithSize(40, 8).
		WithSource("notes.md", b.String()).
		Focus()

	before := p.viewer.Offset()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.viewer.Offset() != before+1 {
		t.Fatalf("KeyDown should advance offset by 1, before=%d after=%d", before, p.viewer.Offset())
	}

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if p.viewer.Offset() <= before+1 {
		t.Fatalf("KeyEnd should jump near bottom, offset=%d", p.viewer.Offset())
	}

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyHome})
	if p.viewer.Offset() != 0 {
		t.Fatalf("KeyHome should return to 0, got %d", p.viewer.Offset())
	}
}

func TestScrollKeysIgnoredWhenBlurred(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 80; i++ {
		b.WriteString("line\n")
	}
	p := NewPane(theme.Default).
		WithSize(40, 8).
		WithSource("notes.md", b.String())

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.viewer.Offset() != 0 {
		t.Fatalf("blurred pane should not scroll, offset=%d", p.viewer.Offset())
	}
}

func TestWithSourceResetsOffset(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 80; i++ {
		b.WriteString("line\n")
	}
	p := NewPane(theme.Default).
		WithSize(40, 8).
		WithSource("notes.md", b.String()).
		Focus()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if p.viewer.Offset() == 0 {
		t.Fatal("setup: expected non-zero offset after End")
	}
	p = p.WithSource("notes.md", b.String())
	if p.viewer.Offset() != 0 {
		t.Fatalf("WithSource should reset offset, got %d", p.viewer.Offset())
	}
}

func TestViewHasTitleRowAndBody(t *testing.T) {
	p := NewPane(theme.Default).
		WithSize(40, 8).
		WithSource("notes.md", "# Hello\n\nworld")
	rows := strings.Split(p.View(), "\n")
	if len(rows) < 2 {
		t.Fatalf("View should produce at least a title and body row, got %d rows", len(rows))
	}
	if !strings.Contains(rows[0], "preview") {
		t.Fatalf("first row should be the title, got %q", rows[0])
	}
}

func TestFocusedViewHintsScroll(t *testing.T) {
	p := NewPane(theme.Default).
		WithSize(40, 8).
		WithSource("notes.md", "# Hello").
		Focus()
	if !strings.Contains(p.View(), "PgUp/PgDn") {
		t.Fatalf("focused view should hint at scroll keys, got:\n%s", p.View())
	}
}
