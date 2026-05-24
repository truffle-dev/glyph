package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/cmd/nook/internal/editor"
	"github.com/truffle-dev/glyph/cmd/nook/internal/mdpreview"
)

// mdFixtureRepo creates a tiny repo with one markdown file and one Go file.
// Both live at the repo root so picker.SelectMsg with a basename works.
func mdFixtureRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "README.md"),
		[]byte("# Hello\n\nThis is a *test* document.\n\n- one\n- two\n"),
		0o644,
	); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "main.go"),
		[]byte("package main\n\nfunc main() {}\n"),
		0o644,
	); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return root
}

// altV is the bubbletea key event for `alt+v`, the markdown-preview toggle.
func altV() tea.KeyMsg {
	return tea.KeyMsg{Alt: true, Type: tea.KeyRunes, Runes: []rune{'v'}}
}

func TestAltVWithoutBufferShowsHint(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 120
	m.height = 30
	m = m.resize()
	updated, _ := m.Update(altV())
	mm := updated.(model)
	if mm.right == rightPreview {
		t.Fatal("preview should not open without an active buffer")
	}
	if !strings.Contains(mm.status, "markdown file") {
		t.Fatalf("expected hint about markdown file, got status=%q", mm.status)
	}
}

func TestAltVOnNonMarkdownBufferShowsHint(t *testing.T) {
	root := mdFixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "main.go")
	updated, _ := m.Update(altV())
	mm := updated.(model)
	if mm.right == rightPreview {
		t.Fatal(".go buffer should not open the preview pane")
	}
	if !strings.Contains(mm.status, ".md") {
		t.Fatalf("expected hint mentioning .md, got status=%q", mm.status)
	}
}

func TestAltVOnMarkdownBufferOpensPreview(t *testing.T) {
	root := mdFixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "README.md")
	updated, _ := m.Update(altV())
	mm := updated.(model)
	if mm.right != rightPreview {
		t.Fatalf("expected rightPreview, got %v", mm.right)
	}
	if !mm.mdPane.Focused() {
		t.Fatal("expected mdPane focused after open")
	}
	if !mm.mdPane.HasSource() {
		t.Fatal("expected mdPane to carry source after open")
	}
	if !strings.Contains(mm.mdPane.Path(), "README.md") {
		t.Fatalf("expected pane path to mention README.md, got %q", mm.mdPane.Path())
	}
}

func TestAltVSecondToggleClosesPreview(t *testing.T) {
	root := mdFixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "README.md")
	updated, _ := m.Update(altV())
	mm := updated.(model)
	if mm.right != rightPreview {
		t.Fatalf("setup: expected rightPreview, got %v", mm.right)
	}
	updated, _ = mm.Update(altV())
	mm = updated.(model)
	if mm.right != rightNone {
		t.Fatalf("expected rightNone after second toggle, got %v", mm.right)
	}
	if mm.mdPane.Focused() {
		t.Fatal("expected mdPane blurred after close")
	}
}

func TestAltVOnMarkdownClosesGitPane(t *testing.T) {
	root := mdFixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "README.md")

	// Open git first.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	mm := updated.(model)
	if mm.right != rightGit {
		t.Fatalf("setup: expected rightGit, got %v", mm.right)
	}

	// Switching to the preview pane should blur git and own the right column.
	updated, _ = mm.Update(altV())
	mm = updated.(model)
	if mm.right != rightPreview {
		t.Fatalf("expected rightPreview, got %v", mm.right)
	}
	if mm.gitPane.Focused() {
		t.Fatal("git pane should be blurred after preview takes the right column")
	}
}

func TestPreviewCancelMsgClosesPane(t *testing.T) {
	root := mdFixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "README.md")
	updated, _ := m.Update(altV())
	mm := updated.(model)
	if mm.right != rightPreview {
		t.Fatalf("setup: expected rightPreview, got %v", mm.right)
	}
	updated, _ = mm.Update(mdpreview.CancelMsg{})
	mm = updated.(model)
	if mm.right != rightNone {
		t.Fatalf("CancelMsg should close the preview, got right=%v", mm.right)
	}
	if mm.mdPane.Focused() {
		t.Fatal("mdPane should be blurred after CancelMsg")
	}
}

func TestPreviewScrollRoutedToPaneWhenFocused(t *testing.T) {
	// Build a long markdown buffer so scroll actually moves the offset.
	root := t.TempDir()
	var b strings.Builder
	b.WriteString("# Long doc\n\n")
	for i := 0; i < 200; i++ {
		b.WriteString("paragraph ")
		b.WriteString(strings.Repeat("x", 5))
		b.WriteString("\n\n")
	}
	path := filepath.Join(root, "long.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write long.md: %v", err)
	}
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "long.md")
	updated, _ := m.Update(altV())
	mm := updated.(model)
	if mm.right != rightPreview {
		t.Fatalf("setup: expected rightPreview, got %v", mm.right)
	}

	// PgDn should advance the embedded viewer. We check via the rendered
	// View body — a scrolled viewer drops earlier lines from its window.
	before := mm.mdPane.View()
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	mm = updated.(model)
	after := mm.mdPane.View()
	if before == after {
		t.Fatal("PgDn while preview is focused should mutate the rendered view")
	}
}

func TestSavedMsgRefreshesPreviewContent(t *testing.T) {
	root := mdFixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "README.md")
	updated, _ := m.Update(altV())
	mm := updated.(model)
	if mm.right != rightPreview {
		t.Fatalf("setup: expected rightPreview, got %v", mm.right)
	}

	// Mutate the active buffer's contents in-place via the picker round trip.
	// The cheapest route to a "buffer differs from preview" state is to call
	// ReplaceAllFromString on the active editor pane directly.
	if p := mm.bufs.Active(); p != nil {
		*p = p.ReplaceAllFromString("# After save\n\nfresh body\n")
	} else {
		t.Fatal("no active buffer for refresh test")
	}

	// Sanity: viewer hasn't been told yet, so the rendered view should not
	// mention "After save" until the SavedMsg lands.
	beforeView := mm.mdPane.View()
	if strings.Contains(beforeView, "After save") {
		t.Fatalf("preview should be stale before SavedMsg, got:\n%s", beforeView)
	}

	// Fire the SavedMsg that the host emits after a successful disk write.
	updated, _ = mm.Update(editor.SavedMsg{Path: filepath.Join(root, "README.md")})
	mm = updated.(model)
	afterView := mm.mdPane.View()
	if !strings.Contains(afterView, "After save") {
		t.Fatalf("preview should reflect new buffer after SavedMsg, got:\n%s", afterView)
	}
}

func TestSavedMsgOnOtherFileDoesNotRefreshPreview(t *testing.T) {
	root := mdFixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "README.md")
	updated, _ := m.Update(altV())
	mm := updated.(model)
	if mm.right != rightPreview {
		t.Fatalf("setup: expected rightPreview, got %v", mm.right)
	}

	prev := mm.mdPane.View()
	// SavedMsg on a sibling file should not touch the preview pane.
	updated, _ = mm.Update(editor.SavedMsg{Path: filepath.Join(root, "main.go")})
	mm = updated.(model)
	if mm.mdPane.View() != prev {
		t.Fatal("SavedMsg on a non-preview file should not refresh the preview")
	}
}

func TestPreviewRendersInView(t *testing.T) {
	root := mdFixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "README.md")
	updated, _ := m.Update(altV())
	mm := updated.(model)
	out := mm.View()
	if !strings.Contains(out, "preview") {
		t.Fatalf("full view should include the preview title row, got:\n%s", out)
	}
}
