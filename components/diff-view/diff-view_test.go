package diffview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

const sample = `--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
-import "fmt"
+import "os"
+import "log"
`

func TestEmptyDiffShowsPlaceholder(t *testing.T) {
	v := New(theme.Default).WithSize(40, 5)
	out := v.View()
	if !strings.Contains(out, "No diff.") {
		t.Fatalf("empty diff should show placeholder, got %q", out)
	}
}

func TestParseUnifiedClassifiesKinds(t *testing.T) {
	lines := ParseUnified(sample)
	want := []Kind{KindFile, KindFile, KindHunk, KindContext, KindRemoved, KindAdded, KindAdded}
	if len(lines) != len(want) {
		t.Fatalf("want %d lines, got %d: %#v", len(want), len(lines), lines)
	}
	for i, ln := range lines {
		if ln.Kind != want[i] {
			t.Fatalf("line %d kind = %d, want %d (content=%q)", i, ln.Kind, want[i], ln.Content)
		}
	}
}

func TestParseUnifiedAssignsLineNumbers(t *testing.T) {
	lines := ParseUnified(sample)
	// Hunk starts at -1,3 +1,4 → context line ("package main") is old=1, new=1
	ctx := lines[3]
	if ctx.Old != 1 || ctx.New != 1 {
		t.Fatalf("first context line: want Old=1 New=1, got Old=%d New=%d", ctx.Old, ctx.New)
	}
	// Removed line ("import \"fmt\"") is old=2
	removed := lines[4]
	if removed.Kind != KindRemoved {
		t.Fatalf("expected removed at index 4, got kind %d", removed.Kind)
	}
	if removed.Old != 2 {
		t.Fatalf("removed line Old: want 2, got %d", removed.Old)
	}
}

func TestRendersAdditionsRemovalsContext(t *testing.T) {
	v := New(theme.Default).WithSize(60, 20).WithLines(ParseUnified(sample))
	out := v.View()
	if !strings.Contains(out, `import "os"`) {
		t.Fatalf("addition must render, got %q", out)
	}
	if !strings.Contains(out, `import "fmt"`) {
		t.Fatalf("removal must render, got %q", out)
	}
	if !strings.Contains(out, "package main") {
		t.Fatalf("context must render, got %q", out)
	}
}

func TestScrollDownClampsAtMaxOffset(t *testing.T) {
	lines := make([]Line, 50)
	for i := range lines {
		lines[i] = Line{Kind: KindContext, Old: i + 1, New: i + 1, Content: "ctx"}
	}
	v := New(theme.Default).WithSize(60, 5).WithLines(lines)
	for i := 0; i < 200; i++ {
		v, _ = v.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if v.Offset() != 50-5 {
		t.Fatalf("offset should clamp to total-height (45), got %d", v.Offset())
	}
}

func TestLineNumbersCanBeHidden(t *testing.T) {
	v := New(theme.Default).WithSize(60, 5).WithLineNumbers(false).WithLines([]Line{
		{Kind: KindContext, Old: 99, New: 99, Content: "x"},
	})
	out := v.View()
	if strings.Contains(out, "99") {
		t.Fatalf("line numbers should be hidden, got %q", out)
	}
}

func TestTruncatesLongContent(t *testing.T) {
	long := strings.Repeat("a", 200)
	v := New(theme.Default).WithSize(40, 3).WithLines([]Line{{Kind: KindContext, Content: long}})
	out := v.View()
	if !strings.Contains(out, "…") {
		t.Fatalf("long line should truncate with ellipsis, got %q", out)
	}
}

func TestParseUnifiedHandlesEmpty(t *testing.T) {
	if got := ParseUnified(""); got != nil {
		t.Fatalf("empty diff should return nil, got %#v", got)
	}
}

func TestParseUnifiedHandlesMalformedHunk(t *testing.T) {
	lines := ParseUnified("@@ garbage @@\n hello\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	if lines[0].Kind != KindHunk {
		t.Fatalf("malformed hunk should still classify as KindHunk, got %d", lines[0].Kind)
	}
}
