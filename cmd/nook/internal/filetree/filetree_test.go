package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	glyphtree "github.com/truffle-dev/glyph/components/file-tree"
	"github.com/truffle-dev/glyph/components/theme"
)

// timeNow / timeSince are thin shims so the startup-budget test reads
// cleanly without forcing the test to know about time.Time semantics.
func timeNow() time.Time          { return time.Now() }
func timeSince(t time.Time) int64 { return time.Since(t).Nanoseconds() }

// newBuiltPane constructs a pane and synchronously walks + binds the
// tree so pane-level tests can drive the underlying glyph file-tree
// model immediately. Mirrors what Init() + BuildTreeCmd + the
// BuildTreeMsg handler do at runtime.
func newBuiltPane(t *testing.T, root string) Pane {
	t.Helper()
	p := New(theme.Default, root)
	p.SetSize(40, 20)
	p.SetNode(BuildTree(root))
	return p
}

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
	p := newBuiltPane(t, root)

	abs := filepath.Join(root, "internal", "sub", "deeper.go")
	p.Reveal(abs)

	if got := p.Selected(); got != abs {
		t.Errorf("Selected = %q; want %q", got, abs)
	}
}

func TestPane_BlurredIgnoresKeys(t *testing.T) {
	root := makeFixture(t)
	p := newBuiltPane(t, root)

	before := p.model.Selected()
	p2, _ := p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p2.model.Selected() != before {
		t.Errorf("blurred pane consumed a KeyDown; cursor moved from %q to %q",
			before, p2.model.Selected())
	}
}

func TestPane_FocusedRoutesKeysToTree(t *testing.T) {
	root := makeFixture(t)
	p := newBuiltPane(t, root)
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
	p := newBuiltPane(t, root)
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
	p := newBuiltPane(t, root)
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

func TestPane_RefreshCmdPreservesCursor(t *testing.T) {
	root := makeFixture(t)
	p := newBuiltPane(t, root)
	p.model.SetCursor("a.go")

	if err := os.WriteFile(filepath.Join(root, "newfile.go"), []byte(""), 0o644); err != nil {
		t.Fatalf("write newfile.go: %v", err)
	}
	cmd := p.RefreshCmd()
	if cmd == nil {
		t.Fatal("RefreshCmd returned nil")
	}
	msg, ok := cmd().(BuildTreeMsg)
	if !ok {
		t.Fatalf("RefreshCmd produced %T; want BuildTreeMsg", cmd())
	}
	p.SetNode(msg.Node)

	if got := p.model.Selected(); got != "a.go" {
		t.Errorf("Refresh moved cursor from %q to %q", "a.go", got)
	}
	found := false
	for _, c := range msg.Node.Children {
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
	p := newBuiltPane(t, root)

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

// TestPane_StartupIsConstantTime asserts that New() returns in well
// under the cost of a recursive file-system walk. This is the
// load-bearing property the whole refactor exists to enforce: the
// first paint must never block on the tree walk. Budget is generous
// (10ms) to avoid false positives on shared-CI machines.
func TestPane_StartupIsConstantTime(t *testing.T) {
	root := makeFixture(t)
	t0 := timeNow()
	p := New(theme.Default, root)
	p.SetSize(40, 20)
	dt := timeSince(t0)
	if dt > 10_000_000 { // 10ms
		t.Errorf("New + SetSize took %dns; expected <10ms (it must not walk the FS)", dt)
	}
	if p.Built() {
		t.Error("pane is reporting Built() before SetNode landed")
	}
	if !strings.Contains(stripANSI(p.View()), "Scanning") {
		t.Error("pre-built pane should render a Scanning… placeholder")
	}
}

// TestPane_PendingRevealReplaysAfterBuild covers the dotfile-edit
// race: caller asks for a Reveal before BuildTreeMsg lands; SetNode
// must replay it once the tree is bound.
func TestPane_PendingRevealReplaysAfterBuild(t *testing.T) {
	root := makeFixture(t)
	p := New(theme.Default, root)
	p.SetSize(40, 20)

	abs := filepath.Join(root, "internal", "sub", "deeper.go")
	p.Reveal(abs)
	if p.Selected() != "" {
		t.Errorf("pre-built Reveal moved the cursor; got Selected=%q", p.Selected())
	}
	p.SetNode(BuildTree(root))
	if got := p.Selected(); got != abs {
		t.Errorf("pending reveal didn't replay; Selected=%q want %q", got, abs)
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

func TestPane_CreateKeyOnFocusedFileEmitsPromptWithParentDir(t *testing.T) {
	root := makeFixture(t)
	p := newBuiltPane(t, root)
	p.Focus()
	p.model.SetCursor("a.go")

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("`a` on focused tree produced no command")
	}
	msg := cmd()
	cm, ok := msg.(CreatePromptMsg)
	if !ok {
		t.Fatalf("`a` produced %T; want CreatePromptMsg", msg)
	}
	if cm.ParentDir != root {
		t.Errorf("ParentDir = %q; want %q (file selected → parent is project root)", cm.ParentDir, root)
	}
}

func TestPane_CreateKeyOnFocusedDirSelectsThatDirAsParent(t *testing.T) {
	root := makeFixture(t)
	p := newBuiltPane(t, root)
	p.Focus()
	p.model.SetCursor("internal")

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("`a` on focused tree produced no command")
	}
	cm, ok := cmd().(CreatePromptMsg)
	if !ok {
		t.Fatalf("`a` produced wrong message type")
	}
	want := filepath.Join(root, "internal")
	if cm.ParentDir != want {
		t.Errorf("ParentDir = %q; want %q (directory selected → parent is that dir)", cm.ParentDir, want)
	}
}

func TestPane_CreateKeyOnBlurredTreeIgnored(t *testing.T) {
	root := makeFixture(t)
	p := newBuiltPane(t, root)
	// Pane starts blurred.
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd != nil {
		t.Errorf("`a` on blurred tree produced a command; should be ignored")
	}
}

func TestPane_CreateKeyBeforeBuildIgnored(t *testing.T) {
	root := makeFixture(t)
	p := New(theme.Default, root)
	p.SetSize(40, 20)
	p.Focus()
	// Tree not built yet.
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd != nil {
		t.Errorf("`a` before tree built produced a command; should be ignored until built")
	}
}

func TestPane_CreateKeyAltAModifierFallsThrough(t *testing.T) {
	root := makeFixture(t)
	p := newBuiltPane(t, root)
	p.Focus()
	// Alt+a should NOT trigger the create prompt — only plain `a` does.
	// Verifies the modifier guard so other host keymaps can claim Alt+a.
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}, Alt: true})
	if cmd != nil {
		// Either nil cmd (glyph tree didn't bind alt+a) or a non-Create cmd
		// is acceptable; firing CreatePromptMsg is not.
		if msg := cmd(); msg != nil {
			if _, ok := msg.(CreatePromptMsg); ok {
				t.Errorf("Alt+a fired CreatePromptMsg; should be plain `a` only")
			}
		}
	}
}
