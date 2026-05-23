package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/truffle-dev/glyph/cmd/nook/internal/editor"
	"github.com/truffle-dev/glyph/cmd/nook/internal/git"
	"github.com/truffle-dev/glyph/cmd/nook/internal/picker"
	"github.com/truffle-dev/glyph/cmd/nook/internal/search"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

// fixtureRepo creates a small repo with two files and a commit.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n\nfunc Foo() {}\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "sub", "b.go"), []byte("package b\n\nfunc Bar() {}\n"), 0o644)

	envAll := append(os.Environ(),
		"GIT_AUTHOR_NAME=nook",
		"GIT_AUTHOR_EMAIL=nook@example.com",
		"GIT_COMMITTER_NAME=nook",
		"GIT_COMMITTER_EMAIL=nook@example.com",
	)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = envAll
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "nook")
	run("config", "user.email", "nook@example.com")
	run("config", "commit.gpgsign", "false")
	run("add", ".")
	run("commit", "-q", "-m", "init")
	return root
}

func TestNewModelAndInit(t *testing.T) {
	m := newModel(t.TempDir())
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected Init Cmd")
	}
}

func TestResizeAdjustsPanes(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 120
	m.height = 30
	m = m.resize()
	// without right pane, editor should take full width
	if m.editor.View() == "" {
		t.Fatal("expected non-empty editor view")
	}
	m.right = rightGit
	m = m.resize()
	if m.gitPane.View() == "" {
		t.Fatal("expected non-empty git pane view")
	}
}

func TestFilesLoadedPopulatesPicker(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	files := walkRepo(root)
	if len(files) == 0 {
		t.Fatal("expected files in fixture")
	}
	updated, _ := m.Update(filesLoadedMsg{files: files})
	mm := updated.(model)
	if mm.picker.TotalCount() != len(files) {
		t.Fatalf("expected %d picker items, got %d", len(files), mm.picker.TotalCount())
	}
}

func TestCtrlPOpensFilePicker(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	mm := updated.(model)
	if mm.overlay != overlayFilePicker {
		t.Fatalf("expected file picker overlay, got %v", mm.overlay)
	}
}

func TestCtrlFOpensSearchOverlay(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	mm := updated.(model)
	if mm.overlay != overlayProjectSearch {
		t.Fatalf("expected search overlay, got %v", mm.overlay)
	}
}

func TestPickerSelectOpensEditor(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 100
	m.height = 24
	m = m.resize()
	updated, _ := m.Update(picker.SelectMsg{Item: picker.Item{Title: "a.go", Value: "a.go"}})
	mm := updated.(model)
	if mm.editor.Path() == "" {
		t.Fatal("expected editor.Path to be set after SelectMsg")
	}
	if !strings.HasSuffix(mm.editor.Path(), "a.go") {
		t.Fatalf("expected a.go suffix, got %q", mm.editor.Path())
	}
	if mm.overlay != overlayNone {
		t.Fatal("expected overlay cleared")
	}
}

func TestSearchOpenMsgJumps(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 100
	m.height = 24
	m = m.resize()
	path := filepath.Join(root, "sub", "b.go")
	updated, _ := m.Update(search.OpenMsg{Path: path, Line: 3, Col: 6})
	mm := updated.(model)
	if mm.editor.Path() != path {
		t.Fatalf("expected editor opened on %q, got %q", path, mm.editor.Path())
	}
	if mm.editor.CursorRow() != 2 {
		t.Fatalf("expected row 2 (line 3), got %d", mm.editor.CursorRow())
	}
}

func TestCtrlGTogglesGitPane(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 100
	m.height = 24
	m = m.resize()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	mm := updated.(model)
	if mm.right != rightGit {
		t.Fatalf("expected rightGit, got %v", mm.right)
	}
	if !mm.gitPane.Focused() {
		t.Fatal("expected git pane focused")
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	mm = updated.(model)
	if mm.right != rightNone {
		t.Fatalf("expected rightNone after second toggle, got %v", mm.right)
	}
}

func TestGitStatusMsgUpdatesPane(t *testing.T) {
	root := fixtureRepo(t)
	os.WriteFile(filepath.Join(root, "new.txt"), []byte("hi"), 0o644)

	m := newModel(root)
	m.width = 100
	m.height = 24
	m = m.resize()

	st, err := git.RunStatus(t.Context(), root)
	if err != nil {
		t.Fatalf("status err: %v", err)
	}
	updated, _ := m.Update(git.StatusMsg{Status: st})
	mm := updated.(model)
	mm.right = rightGit
	mm = mm.resize()
	out := mm.gitPane.View()
	if !strings.Contains(out, "new.txt") {
		t.Fatalf("expected new.txt in git view:\n%s", out)
	}
}

func TestEditorSavedMsgUpdatesStatus(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	updated, _ := m.Update(editor.SavedMsg{Path: "/tmp/x.txt"})
	mm := updated.(model)
	if !strings.Contains(mm.status, "saved") {
		t.Fatalf("expected 'saved' in status, got %q", mm.status)
	}
}

func TestEscClearsSearchOverlay(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	mm := updated.(model)
	updated, _ = mm.Update(search.CancelMsg{})
	mm = updated.(model)
	if mm.overlay != overlayNone {
		t.Fatalf("expected overlay cleared, got %v", mm.overlay)
	}
}

func TestViewRendersWithoutPanic(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	out := m.View()
	if out == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestWalkRepoSkipsHiddenDirs(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref"), 0o644)
	os.MkdirAll(filepath.Join(root, "node_modules"), 0o755)
	os.WriteFile(filepath.Join(root, "node_modules", "x.js"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "real.go"), []byte("real"), 0o644)

	files := walkRepo(root)
	for _, f := range files {
		if strings.HasPrefix(f, ".git/") || strings.HasPrefix(f, "node_modules/") {
			t.Fatalf("walkRepo should skip %q", f)
		}
	}
	if len(files) == 0 || files[0] != "real.go" {
		t.Fatalf("expected real.go in walkRepo, got %+v", files)
	}
}
