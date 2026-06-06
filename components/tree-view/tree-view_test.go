package treeview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sample() Node {
	return Node{
		Label: "root",
		Children: []Node{
			{
				Label: "src",
				Children: []Node{
					{Label: "main.go"},
					{Label: "util.go"},
				},
			},
			{
				Label: "docs",
				Children: []Node{
					{Label: "README.md"},
				},
			},
			{Label: "go.mod"},
		},
	}
}

func TestEmptyRendersPlaceholder(t *testing.T) {
	m := New().WithPlaceholder("nothing here").WithSize(40, 5)
	out := m.View()
	if !strings.Contains(out, "nothing here") {
		t.Fatalf("expected placeholder in output, got %q", out)
	}
}

func TestRendersRootAndDirectChildren(t *testing.T) {
	m := New().WithRoot(sample()).WithSize(40, 10)
	out := m.View()
	for _, want := range []string{"root", "src", "docs", "go.mod"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
	// Grandchildren should NOT be visible (only root is expanded).
	for _, hidden := range []string{"main.go", "util.go", "README.md"} {
		if strings.Contains(out, hidden) {
			t.Fatalf("did not expect %q in output (collapsed):\n%s", hidden, out)
		}
	}
}

func TestExpandAllShowsEveryDescendant(t *testing.T) {
	m := New().WithRoot(sample()).WithExpandAll().WithSize(40, 20)
	out := m.View()
	for _, want := range []string{"root", "src", "docs", "main.go", "util.go", "README.md", "go.mod"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in expand-all output:\n%s", want, out)
		}
	}
}

func TestCollapseAllHidesEverythingExceptRoot(t *testing.T) {
	m := New().WithRoot(sample()).WithCollapseAll().WithSize(40, 10)
	out := m.View()
	if !strings.Contains(out, "root") {
		t.Fatalf("root should still be visible: %q", out)
	}
	for _, hidden := range []string{"src", "docs", "go.mod", "main.go"} {
		if strings.Contains(out, hidden) {
			t.Fatalf("did not expect %q after collapse-all:\n%s", hidden, out)
		}
	}
}

func TestRightExpandsBranchUnderCursor(t *testing.T) {
	m := New().WithRoot(sample()).WithCollapseAll().WithSize(40, 20)
	// Cursor is at root. Press right to expand root.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated
	out := m.View()
	if !strings.Contains(out, "src") || !strings.Contains(out, "docs") {
		t.Fatalf("expected root expanded after right key:\n%s", out)
	}
	// Grandchildren still hidden.
	if strings.Contains(out, "main.go") {
		t.Fatalf("did not expect grandchildren yet:\n%s", out)
	}
}

func TestLeftCollapsesBranchUnderCursor(t *testing.T) {
	m := New().WithRoot(sample()).WithExpandAll().WithSize(40, 20)
	// Cursor at root. Press left to collapse root.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated
	out := m.View()
	for _, hidden := range []string{"src", "docs", "go.mod"} {
		if strings.Contains(out, hidden) {
			t.Fatalf("expected %q hidden after collapse:\n%s", hidden, out)
		}
	}
}

func TestLeftOnLeafJumpsToParent(t *testing.T) {
	m := New().WithRoot(sample()).WithExpandAll().WithSize(40, 20)
	// Visible order: root, src, main.go, util.go, docs, README.md, go.mod
	// Move cursor to main.go (index 2).
	for i := 0; i < 2; i++ {
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = u
	}
	if cur, ok := m.SelectedNode(); !ok || cur.Label != "main.go" {
		t.Fatalf("expected cursor on main.go, got %+v", cur)
	}
	// Left should jump cursor to parent "src".
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = u
	if cur, ok := m.SelectedNode(); !ok || cur.Label != "src" {
		t.Fatalf("expected cursor jumped to src, got %+v", cur)
	}
}

func TestEnterEmitsSelectMsgWithCorrectNode(t *testing.T) {
	m := New().WithRoot(sample()).WithExpandAll().WithSize(40, 20)
	// Move down twice to land on main.go.
	for i := 0; i < 2; i++ {
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = u
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected select cmd on enter")
	}
	msg := cmd()
	sel, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", msg)
	}
	if sel.Node.Label != "main.go" {
		t.Fatalf("expected main.go selected, got %q", sel.Node.Label)
	}
	if sel.Index != 2 {
		t.Fatalf("expected index 2, got %d", sel.Index)
	}
}

func TestEnterOnBranchTogglesExpansion(t *testing.T) {
	m := New().WithRoot(sample()).WithCollapseAll().WithSize(40, 20)
	// Cursor at root (collapsed). Enter expands AND emits select.
	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u
	if !m.IsExpanded("") {
		t.Fatalf("expected root expanded after enter")
	}
	if cmd == nil {
		t.Fatal("expected select cmd on enter even when toggling")
	}
	// Enter again on root collapses.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u
	if m.IsExpanded("") {
		t.Fatalf("expected root collapsed after second enter")
	}
}

func TestSpaceTogglesBranchWithoutSelect(t *testing.T) {
	m := New().WithRoot(sample()).WithCollapseAll().WithSize(40, 20)
	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = u
	if !m.IsExpanded("") {
		t.Fatalf("expected root expanded after space")
	}
	if cmd != nil {
		// Space should not emit a select cmd.
		msg := cmd()
		if _, isSel := msg.(SelectMsg); isSel {
			t.Fatal("space should not emit SelectMsg")
		}
	}
}

func TestCursorClampsAtEnds(t *testing.T) {
	m := New().WithRoot(sample()).WithExpandAll().WithSize(40, 20)
	// Press End: cursor at last row.
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = u
	if cur, ok := m.SelectedNode(); !ok || cur.Label != "go.mod" {
		t.Fatalf("expected go.mod at end, got %+v", cur)
	}
	// Down at end is a no-op.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = u
	if cur, _ := m.SelectedNode(); cur.Label != "go.mod" {
		t.Fatalf("expected cursor stayed at end, got %q", cur.Label)
	}
	// Home jumps to root.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = u
	if cur, _ := m.SelectedNode(); cur.Label != "root" {
		t.Fatalf("expected root at home, got %q", cur.Label)
	}
}

func TestRootVisibleFalseShowsChildrenAtDepthZero(t *testing.T) {
	m := New().WithRoot(sample()).WithRootVisible(false).WithSize(40, 20)
	out := m.View()
	if strings.Contains(out, "root") {
		t.Fatalf("did not expect root row when WithRootVisible(false):\n%s", out)
	}
	for _, want := range []string{"src", "docs", "go.mod"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q at depth 0:\n%s", want, out)
		}
	}
	// First selectable row should be the first child.
	if cur, ok := m.SelectedNode(); !ok || cur.Label != "src" {
		t.Fatalf("expected first row src, got %+v", cur)
	}
}

func TestPgDownAdvancesCursor(t *testing.T) {
	m := New().WithRoot(sample()).WithExpandAll().WithSize(40, 6)
	pre := m.Cursor()
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = u
	if m.Cursor() <= pre {
		t.Fatalf("pgdown should advance cursor, was %d -> %d", pre, m.Cursor())
	}
}

func TestExpandedDepthLimitsRevealedLevels(t *testing.T) {
	m := New().WithRoot(sample()).WithExpandedDepth(1).WithSize(40, 20)
	out := m.View()
	// Root expanded -> direct children visible.
	for _, want := range []string{"src", "docs", "go.mod"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q at depth 1: %s", want, out)
		}
	}
	// Direct children NOT expanded -> grandchildren hidden.
	for _, hidden := range []string{"main.go", "util.go", "README.md"} {
		if strings.Contains(out, hidden) {
			t.Fatalf("did not expect %q at depth 1: %s", hidden, out)
		}
	}
}

func TestTruncCell(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 6, "hello…"},
		{"abc", 1, "…"},
		{"", 5, ""},
		{"x", 0, ""},
	}
	for _, c := range cases {
		got := truncCell(c.in, c.max)
		if got != c.want {
			t.Errorf("truncCell(%q,%d)=%q want %q", c.in, c.max, got, c.want)
		}
	}
}
