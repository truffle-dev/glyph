package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	glyphtree "github.com/truffle-dev/glyph/components/file-tree"
	"github.com/truffle-dev/glyph/components/theme"
)

// makeFixture lays out a deterministic project layout under t.TempDir
// so the builder tests have a known shape to assert against.
func makeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mk := func(rel string, isDir bool) {
		full := filepath.Join(root, rel)
		if isDir {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", rel, err)
			}
			return
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir parent of %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(""), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	mk("a.go", false)
	mk("b.go", false)
	mk("internal/util.go", false)
	mk("internal/sub/deeper.go", false)
	mk("node_modules/junk.js", false)
	mk(".git/HEAD", false)
	mk(".hidden/x.go", false)
	mk("vendor/v.go", false)
	mk("dist/out.js", false)
	mk("target/release/bin", false)
	return root
}

func TestBuildTree_IgnoresVcsAndBuildDirs(t *testing.T) {
	root := makeFixture(t)
	tree := BuildTree(root)

	names := map[string]bool{}
	for _, c := range tree.Children {
		names[c.Name] = true
	}
	for _, banned := range []string{".git", ".hidden", "node_modules", "vendor", "dist", "target"} {
		if names[banned] {
			t.Errorf("BuildTree included %q; expected it to be skipped", banned)
		}
	}
	for _, expected := range []string{"a.go", "b.go", "internal"} {
		if !names[expected] {
			t.Errorf("BuildTree missing %q", expected)
		}
	}
}

func TestBuildTree_DirsBeforeFiles(t *testing.T) {
	root := makeFixture(t)
	tree := BuildTree(root)

	// First child should be "internal" (dir), then files alphabetically.
	if len(tree.Children) < 2 {
		t.Fatalf("expected >= 2 children, got %d", len(tree.Children))
	}
	if !tree.Children[0].IsDir() {
		t.Errorf("first child is %q; expected a directory", tree.Children[0].Name)
	}
	if tree.Children[1].IsDir() {
		t.Errorf("second child is %q (dir); expected a file", tree.Children[1].Name)
	}
}

func TestBuildTree_RecursesIntoSubdirs(t *testing.T) {
	root := makeFixture(t)
	tree := BuildTree(root)

	var internal *glyphtree.Node
	for i, c := range tree.Children {
		if c.Name == "internal" {
			internal = &tree.Children[i]
			break
		}
	}
	if internal == nil {
		t.Fatal("no 'internal' directory in tree")
	}
	names := map[string]bool{}
	for _, c := range internal.Children {
		names[c.Name] = true
	}
	if !names["sub"] || !names["util.go"] {
		t.Errorf("internal/ missing expected entries; got %v", names)
	}
}

func TestPane_RevealExpandsAncestorsAndCursors(t *testing.T) {
	root := makeFixture(t)
	p := New(theme.Default, root)
	p.SetSize(40, 20)

	abs := filepath.Join(root, "internal", "sub", "deeper.go")
	p.Reveal(abs)

	if got := p.Selected(); got != abs {
		t.Errorf("Selected = %q; want %q", got, abs)
	}
}

func TestPane_BlurredIgnoresKeys(t *testing.T) {
	root := makeFixture(t)
	p := New(theme.Default, root)
	p.SetSize(40, 20)

	before := p.model.Selected()
	p2, _ := p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p2.model.Selected() != before {
		t.Errorf("blurred pane consumed a KeyDown; cursor moved from %q to %q",
			before, p2.model.Selected())
	}
}

func TestPane_FocusedRoutesKeysToTree(t *testing.T) {
	root := makeFixture(t)
	p := New(theme.Default, root)
	p.SetSize(40, 20)
	p.Focus()

	// Down should advance the cursor at least one row (the fixture has
	// several top-level entries).
	before := p.model.Selected()
	p2, _ := p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p2.model.Selected() == before {
		t.Errorf("focused pane did not move cursor on KeyDown")
	}
}

func TestPane_EnterOnFileLifesIntoOpenMsg(t *testing.T) {
	root := makeFixture(t)
	p := New(theme.Default, root)
	p.SetSize(40, 20)
	p.Focus()
	p.model.SetCursor("a.go")

	p2, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on a file produced no command")
	}
	msg := cmd()
	open, ok := msg.(OpenMsg)
	if !ok {
		t.Fatalf("enter on file produced %T; want OpenMsg", msg)
	}
	want := filepath.Join(root, "a.go")
	if open.Path != want {
		t.Errorf("OpenMsg.Path = %q; want %q", open.Path, want)
	}
	if p2.model.Selected() != "a.go" {
		t.Errorf("cursor moved unexpectedly: %q", p2.model.Selected())
	}
}

func TestPane_EnterOnDirectoryStaysInPane(t *testing.T) {
	root := makeFixture(t)
	p := New(theme.Default, root)
	p.SetSize(40, 20)
	p.Focus()
	p.model.SetCursor("internal")

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		return
	}
	msg := cmd()
	if _, ok := msg.(OpenMsg); ok {
		t.Errorf("directory expand emitted OpenMsg; should stay inside the pane")
	}
}

func TestPane_RefreshPreservesCursor(t *testing.T) {
	root := makeFixture(t)
	p := New(theme.Default, root)
	p.SetSize(40, 20)
	p.model.SetCursor("a.go")

	if err := os.WriteFile(filepath.Join(root, "newfile.go"), []byte(""), 0o644); err != nil {
		t.Fatalf("write newfile.go: %v", err)
	}
	p.Refresh()

	if got := p.model.Selected(); got != "a.go" {
		t.Errorf("Refresh moved cursor from %q to %q", "a.go", got)
	}
	tree := BuildTree(root)
	found := false
	for _, c := range tree.Children {
		if c.Name == "newfile.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Refresh did not surface the new file")
	}
}

func TestPane_ViewIncludesProjectName(t *testing.T) {
	root := makeFixture(t)
	p := New(theme.Default, root)
	p.SetSize(30, 12)

	out := stripANSI(p.View())
	if !strings.Contains(out, filepath.Base(root)) {
		t.Errorf("View missing project name %q in:\n%s", filepath.Base(root), out)
	}
}

func TestPane_ViewEmptyWhenTooSmall(t *testing.T) {
	root := makeFixture(t)
	p := New(theme.Default, root)
	p.SetSize(8, 2)

	if p.View() != "" {
		t.Errorf("View should be empty when pane is too small; got %q", p.View())
	}
}

func stripANSI(s string) string {
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
