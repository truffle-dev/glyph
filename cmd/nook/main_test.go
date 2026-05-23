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
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/truffle-dev/glyph/cmd/nook/internal/ai"
	"github.com/truffle-dev/glyph/cmd/nook/internal/composer"
	"github.com/truffle-dev/glyph/cmd/nook/internal/edit"
	"github.com/truffle-dev/glyph/cmd/nook/internal/editor"
	"github.com/truffle-dev/glyph/cmd/nook/internal/ghost"
	"github.com/truffle-dev/glyph/cmd/nook/internal/git"
	"github.com/truffle-dev/glyph/cmd/nook/internal/picker"
	"github.com/truffle-dev/glyph/cmd/nook/internal/search"
)

func newClientForTest(t *testing.T) *ai.Client {
	t.Helper()
	c, err := ai.NewClient()
	if err != nil {
		t.Fatalf("expected client (key faked in env): %v", err)
	}
	return c
}

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

func TestCtrlKWithoutOpenFileIsStatusHint(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	mm := updated.(model)
	if mm.overlay != overlayNone {
		t.Fatalf("expected no overlay without open file, got %v", mm.overlay)
	}
	if !strings.Contains(mm.status, "open a file") {
		t.Fatalf("expected hint, got status %q", mm.status)
	}
}

func TestCtrlKOpensInlineEditWhenFileOpen(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.editor = m.editor.Open(filepath.Join(root, "a.go")).Focus()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	mm := updated.(model)
	if mm.overlay != overlayInlineEdit {
		t.Fatalf("expected overlayInlineEdit, got %v", mm.overlay)
	}
}

func TestEditAcceptApplies(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.editor = m.editor.Open(filepath.Join(root, "a.go")).Focus()
	updated, _ := m.Update(edit.AcceptMsg{Path: filepath.Join(root, "a.go"), Line: 0, NewText: "package replaced"})
	mm := updated.(model)
	if got := mm.editor.Line(0); got != "package replaced" {
		t.Fatalf("expected line replaced, got %q", got)
	}
	if !mm.editor.Dirty() {
		t.Fatal("expected dirty after edit accept")
	}
}

func TestCtrlLOpensComposer(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	mm := updated.(model)
	if mm.right != rightComposer {
		t.Fatalf("expected rightComposer, got %v", mm.right)
	}
	if !mm.composer.Focused() {
		t.Fatal("expected composer focused")
	}
}

func TestComposerApplyMsgWritesFile(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	updated, _ := m.Update(composer.ApplyMsg{Edit: composer.Edit{
		Path:     "new/file.go",
		Proposed: "package newp\n",
	}})
	mm := updated.(model)
	data, err := os.ReadFile(filepath.Join(root, "new/file.go"))
	if err != nil {
		t.Fatalf("expected file written: %v", err)
	}
	if !strings.Contains(string(data), "package newp") {
		t.Fatalf("expected proposed content written, got %q", data)
	}
	if !strings.Contains(mm.status, "wrote new/file.go") {
		t.Fatalf("expected status update, got %q", mm.status)
	}
}

func TestComposerCancelMsgClosesPane(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.right = rightComposer
	m.composer = m.composer.Focus()
	updated, _ := m.Update(composer.CancelMsg{})
	mm := updated.(model)
	if mm.right != rightNone {
		t.Fatalf("expected right none, got %v", mm.right)
	}
}

func TestGhostSuggestMsgSetsEditorGhostText(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	// Force-enable ghost so SuggestMsg is honored even without ANTHROPIC_API_KEY.
	m.ghost = forceEnabledManager(t)

	site := ghost.Site{
		Path:   filepath.Join(root, "a.go"),
		Row:    2,
		Col:    7,
		Prefix: "fmt.Pri",
	}
	m.editor = m.editor.Open(site.Path).Focus()
	// Seed manager state so the message lands at the right site.
	m.ghost.Tick(site, false, false)

	updated, _ := m.Update(ghost.SuggestMsg{Site: site, Text: "ntln(\"hi\")"})
	mm := updated.(model)
	if got := mm.editor.GhostText(); got != "ntln(\"hi\")" {
		t.Fatalf("expected editor ghost text set, got %q", got)
	}
}

func TestGhostTabAcceptInsertsAndClears(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.ghost = forceEnabledManager(t)

	path := filepath.Join(root, "a.go")
	m.editor = m.editor.Open(path).Focus()
	// Type "fmt.Pri" so we have a real cursor position to merge from.
	for _, r := range "fmt.Pri" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	// Plant ghost-text directly (skip the AI call) by routing a SuggestMsg.
	site := ghost.Site{Path: path, Row: m.editor.CursorRow(), Col: m.editor.CursorCol(), Prefix: m.editor.LinePrefix()}
	m.ghost.Tick(site, false, false)
	updated, _ := m.Update(ghost.SuggestMsg{Site: site, Text: "ntln(\"hi\")"})
	m = updated.(model)
	if m.editor.GhostText() == "" {
		t.Fatal("expected ghost text planted")
	}
	// Press Tab.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := updated.(model)
	// The editor row should now contain the merged line at cursor.
	got := mm.editor.Line(mm.editor.CursorRow())
	if !strings.HasPrefix(got, "fmt.Println(\"hi\")") {
		t.Fatalf("expected merged line starting with fmt.Println, got %q", got)
	}
	if mm.editor.GhostText() != "" {
		t.Fatalf("expected ghost text cleared after accept, got %q", mm.editor.GhostText())
	}
}

func TestGhostEscDismissesProposal(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.ghost = forceEnabledManager(t)

	path := filepath.Join(root, "a.go")
	m.editor = m.editor.Open(path).Focus()
	for _, r := range "fmt.Pri" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	site := ghost.Site{Path: path, Row: m.editor.CursorRow(), Col: m.editor.CursorCol(), Prefix: m.editor.LinePrefix()}
	m.ghost.Tick(site, false, false)
	updated, _ := m.Update(ghost.SuggestMsg{Site: site, Text: "ntln(\"hi\")"})
	m = updated.(model)
	if m.editor.GhostText() == "" {
		t.Fatal("expected ghost text planted")
	}
	// Esc should dismiss.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(model)
	if mm.editor.GhostText() != "" {
		t.Fatalf("expected ghost dismissed, got %q", mm.editor.GhostText())
	}
}

// forceEnabledManager builds a ghost.Manager that reports Enabled() but never
// actually issues an AI request. We do this by reaching into the package via a
// helper that the test uses to bypass NewClient's env requirement.
func forceEnabledManager(t *testing.T) *ghost.Manager {
	t.Helper()
	// We can't construct a Client without an API key; instead, set the env
	// for this test only. Since the test never triggers a real Stream (we
	// only inject SuggestMsg directly), the key doesn't need to be real.
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Setenv("ANTHROPIC_API_KEY", "test-key-not-real")
	}
	return mustGhostManager(t)
}

func mustGhostManager(t *testing.T) *ghost.Manager {
	t.Helper()
	c := newClientForTest(t)
	return ghost.NewManager(c)
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

// LSP helper tests ---------------------------------------------------------

func TestIsGoFile(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"main.go":      true,
		"sub/x.go":     true,
		"main.rs":      false,
		"main":         false,
		"go.mod":       false,
		"go.sum":       false,
		"main.go.tmpl": false,
		"":             false,
	}
	for path, want := range cases {
		if got := isGoFile(path); got != want {
			t.Errorf("isGoFile(%q)=%v want %v", path, got, want)
		}
	}
}

func TestPathFromURI(t *testing.T) {
	t.Parallel()
	if got := pathFromURI(uri.File("/tmp/x.go")); got != "/tmp/x.go" {
		t.Errorf("pathFromURI roundtrip: got %q want /tmp/x.go", got)
	}
	if got := pathFromURI(uri.URI("https://example.com/x.go")); got != "" {
		t.Errorf("non-file URI should be empty, got %q", got)
	}
}

func TestMapSeverity(t *testing.T) {
	t.Parallel()
	cases := map[protocol.DiagnosticSeverity]editor.Severity{
		protocol.DiagnosticSeverityError:       editor.SeverityError,
		protocol.DiagnosticSeverityWarning:     editor.SeverityWarning,
		protocol.DiagnosticSeverityInformation: editor.SeverityInfo,
		protocol.DiagnosticSeverityHint:        editor.SeverityHint,
	}
	for in, want := range cases {
		if got := mapSeverity(in); got != want {
			t.Errorf("mapSeverity(%v)=%v want %v", in, got, want)
		}
	}
}

func TestIsMutatingKey(t *testing.T) {
	t.Parallel()
	mutating := []tea.KeyType{tea.KeyRunes, tea.KeyEnter, tea.KeyTab, tea.KeySpace, tea.KeyBackspace, tea.KeyDelete}
	for _, k := range mutating {
		if !isMutatingKey(k) {
			t.Errorf("isMutatingKey(%v) should be true", k)
		}
	}
	nonMutating := []tea.KeyType{tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight, tea.KeyHome, tea.KeyEnd, tea.KeyPgUp, tea.KeyPgDown, tea.KeyEsc}
	for _, k := range nonMutating {
		if isMutatingKey(k) {
			t.Errorf("isMutatingKey(%v) should be false", k)
		}
	}
}

// TestApplyDiagnosticsToEditor confirms that a publishDiagnostics for the open
// file results in a row→severity map flowing into the editor pane, with
// error winning over warning on the same row.
func TestApplyDiagnosticsToEditor(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	path := filepath.Join(root, "x.go")
	if err := os.WriteFile(path, []byte("package x\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.editor = m.editor.Open(path).Focus()

	m.diagnostics[path] = []protocol.Diagnostic{
		{Range: protocol.Range{Start: protocol.Position{Line: 2}}, Severity: protocol.DiagnosticSeverityWarning, Message: "warn"},
		{Range: protocol.Range{Start: protocol.Position{Line: 2}}, Severity: protocol.DiagnosticSeverityError, Message: "err"},
		{Range: protocol.Range{Start: protocol.Position{Line: 0}}, Severity: protocol.DiagnosticSeverityWarning, Message: "header"},
	}
	m = m.applyDiagnosticsToEditor()
	if got := m.editor.DiagnosticAt(2); got != editor.SeverityError {
		t.Errorf("row 2 should be Error (worst-severity wins); got %v", got)
	}
	if got := m.editor.DiagnosticAt(0); got != editor.SeverityWarning {
		t.Errorf("row 0 should be Warning; got %v", got)
	}
	if got := m.editor.DiagnosticAt(1); got != editor.SeverityNone {
		t.Errorf("row 1 should be None; got %v", got)
	}
}

func TestDiagCounts(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	path := filepath.Join(root, "x.go")
	if err := os.WriteFile(path, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.editor = m.editor.Open(path).Focus()
	m.diagnostics[path] = []protocol.Diagnostic{
		{Severity: protocol.DiagnosticSeverityError, Message: "e1"},
		{Severity: protocol.DiagnosticSeverityError, Message: "e2"},
		{Severity: protocol.DiagnosticSeverityWarning, Message: "w1"},
		{Severity: protocol.DiagnosticSeverityHint, Message: "h1"}, // not counted
	}
	errs, warns := m.diagCounts()
	if errs != 2 || warns != 1 {
		t.Errorf("diagCounts=(%d,%d) want (2,1)", errs, warns)
	}
}
