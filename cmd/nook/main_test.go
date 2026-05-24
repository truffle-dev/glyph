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
	"github.com/truffle-dev/glyph/cmd/nook/internal/filetree"
	"github.com/truffle-dev/glyph/cmd/nook/internal/ghost"
	"github.com/truffle-dev/glyph/cmd/nook/internal/git"
	"github.com/truffle-dev/glyph/cmd/nook/internal/lookup"
	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/cmd/nook/internal/picker"
	"github.com/truffle-dev/glyph/cmd/nook/internal/search"
)

func newClientForTest(t *testing.T) *ai.Client {
	t.Helper()
	// Use a stub `claude` binary so the test suite stays hermetic and never
	// shells out to a real CLI. The tests that build ghost.Manager via this
	// helper inject SuggestMsg/AcceptMsg directly; the client is never asked
	// to spawn anything.
	dir := t.TempDir()
	stub := filepath.Join(dir, "claude")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write stub claude: %v", err)
	}
	c, err := ai.NewClientWithBinary(stub)
	if err != nil {
		t.Fatalf("ai.NewClientWithBinary: %v", err)
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
	// Welcome card renders when no buffers are open.
	if m.View() == "" {
		t.Fatal("expected non-empty view")
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
	if mm.activePath() == "" {
		t.Fatal("expected active buffer path to be set after SelectMsg")
	}
	if !strings.HasSuffix(mm.activePath(), "a.go") {
		t.Fatalf("expected a.go suffix, got %q", mm.activePath())
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
	if mm.activePath() != path {
		t.Fatalf("expected editor opened on %q, got %q", path, mm.activePath())
	}
	if mm.bufs.Active().CursorRow() != 2 {
		t.Fatalf("expected row 2 (line 3), got %d", mm.bufs.Active().CursorRow())
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
	m.bufs.OpenOrSwitch(filepath.Join(root, "a.go"))
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
	m.bufs.OpenOrSwitch(filepath.Join(root, "a.go"))
	updated, _ := m.Update(edit.AcceptMsg{Path: filepath.Join(root, "a.go"), Line: 0, NewText: "package replaced"})
	mm := updated.(model)
	if got := mm.bufs.Active().Line(0); got != "package replaced" {
		t.Fatalf("expected line replaced, got %q", got)
	}
	if !mm.bufs.Active().Dirty() {
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
	m.bufs.OpenOrSwitch(site.Path)
	// Seed manager state so the message lands at the right site.
	m.ghost.Tick(site, false, false)

	updated, _ := m.Update(ghost.SuggestMsg{Site: site, Text: "ntln(\"hi\")"})
	mm := updated.(model)
	if got := mm.bufs.Active().GhostText(); got != "ntln(\"hi\")" {
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
	m.bufs.OpenOrSwitch(path)
	// Type "fmt.Pri" so we have a real cursor position to merge from.
	for _, r := range "fmt.Pri" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	// Plant ghost-text directly (skip the AI call) by routing a SuggestMsg.
	pa := m.bufs.Active()
	site := ghost.Site{Path: path, Row: pa.CursorRow(), Col: pa.CursorCol(), Prefix: pa.LinePrefix()}
	m.ghost.Tick(site, false, false)
	updated, _ := m.Update(ghost.SuggestMsg{Site: site, Text: "ntln(\"hi\")"})
	m = updated.(model)
	if m.bufs.Active().GhostText() == "" {
		t.Fatal("expected ghost text planted")
	}
	// Press Tab.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := updated.(model)
	// The editor row should now contain the merged line at cursor.
	pp := mm.bufs.Active()
	got := pp.Line(pp.CursorRow())
	if !strings.HasPrefix(got, "fmt.Println(\"hi\")") {
		t.Fatalf("expected merged line starting with fmt.Println, got %q", got)
	}
	if pp.GhostText() != "" {
		t.Fatalf("expected ghost text cleared after accept, got %q", pp.GhostText())
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
	m.bufs.OpenOrSwitch(path)
	for _, r := range "fmt.Pri" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	pa := m.bufs.Active()
	site := ghost.Site{Path: path, Row: pa.CursorRow(), Col: pa.CursorCol(), Prefix: pa.LinePrefix()}
	m.ghost.Tick(site, false, false)
	updated, _ := m.Update(ghost.SuggestMsg{Site: site, Text: "ntln(\"hi\")"})
	m = updated.(model)
	if m.bufs.Active().GhostText() == "" {
		t.Fatal("expected ghost text planted")
	}
	// Esc should dismiss.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(model)
	if mm.bufs.Active().GhostText() != "" {
		t.Fatalf("expected ghost dismissed, got %q", mm.bufs.Active().GhostText())
	}
}

// forceEnabledManager builds a ghost.Manager that reports Enabled() but never
// actually issues an AI request. The stub `claude` binary makes the client
// non-nil; the tests inject SuggestMsg/AcceptMsg directly.
func forceEnabledManager(t *testing.T) *ghost.Manager {
	t.Helper()
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
	// Round-trip a platform-aware temp path through uri.File. A literal
	// "/tmp/x.go" would get resolved against the current drive on Windows
	// and fail the comparison; t.TempDir gives an absolute path that
	// uri.File and pathFromURI can both reproduce verbatim.
	in := filepath.Join(t.TempDir(), "x.go")
	if got := pathFromURI(uri.File(in)); got != in {
		t.Errorf("pathFromURI roundtrip: got %q want %q", got, in)
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

// TestApplyDiagnosticsToActive confirms that a publishDiagnostics for the open
// file results in a row→severity map flowing into the editor pane, with
// error winning over warning on the same row.
func TestApplyDiagnosticsToActive(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	path := filepath.Join(root, "x.go")
	if err := os.WriteFile(path, []byte("package x\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.bufs.OpenOrSwitch(path)

	m.diagnostics[path] = []protocol.Diagnostic{
		{Range: protocol.Range{Start: protocol.Position{Line: 2}}, Severity: protocol.DiagnosticSeverityWarning, Message: "warn"},
		{Range: protocol.Range{Start: protocol.Position{Line: 2}}, Severity: protocol.DiagnosticSeverityError, Message: "err"},
		{Range: protocol.Range{Start: protocol.Position{Line: 0}}, Severity: protocol.DiagnosticSeverityWarning, Message: "header"},
	}
	m = m.applyDiagnosticsToActive()
	pa := m.bufs.Active()
	if got := pa.DiagnosticAt(2); got != editor.SeverityError {
		t.Errorf("row 2 should be Error (worst-severity wins); got %v", got)
	}
	if got := pa.DiagnosticAt(0); got != editor.SeverityWarning {
		t.Errorf("row 0 should be Warning; got %v", got)
	}
	if got := pa.DiagnosticAt(1); got != editor.SeverityNone {
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
	m.bufs.OpenOrSwitch(path)
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

// TestTabFlowMultipleBuffers exercises the full v0.4.0 tab story end-to-end:
// open three buffers, walk forward/back with Alt+]/Alt+[, close with Ctrl+W,
// and verify the welcome card returns when the last buffer closes.
func TestTabFlowMultipleBuffers(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()

	a := filepath.Join(root, "a.go")
	b := filepath.Join(root, "sub", "b.go")
	c := filepath.Join(root, "c.go")
	if err := os.WriteFile(c, []byte("package c\n\nfunc Baz() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Open three buffers via SelectMsg (the picker path).
	open := func(mm model, path string) model {
		updated, _ := mm.Update(picker.SelectMsg{Item: picker.Item{Title: filepath.Base(path), Value: path}})
		return updated.(model)
	}
	m = open(m, a)
	m = open(m, b)
	m = open(m, c)

	if got := m.bufs.Count(); got != 3 {
		t.Fatalf("Count after 3 opens = %d, want 3", got)
	}
	if m.bufs.ActiveIndex() != 2 {
		t.Fatalf("active after 3 opens = %d, want 2", m.bufs.ActiveIndex())
	}
	if !strings.HasSuffix(m.activePath(), "c.go") {
		t.Fatalf("active path = %q, want c.go suffix", m.activePath())
	}

	// Tab bar shows up in the rendered view.
	if !strings.Contains(m.View(), "a.go") || !strings.Contains(m.View(), "c.go") {
		t.Fatalf("tab bar missing basenames in view")
	}

	// Alt+] wraps from index 2 to 0.
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}, Alt: true})
	m = upd.(model)
	if m.bufs.ActiveIndex() != 0 {
		t.Fatalf("after alt+] from end, active = %d, want 0", m.bufs.ActiveIndex())
	}
	if !strings.HasSuffix(m.activePath(), "a.go") {
		t.Fatalf("after alt+], path = %q, want a.go", m.activePath())
	}

	// Alt+[ wraps from 0 back to 2.
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}, Alt: true})
	m = upd.(model)
	if m.bufs.ActiveIndex() != 2 {
		t.Fatalf("after alt+[ from start, active = %d, want 2", m.bufs.ActiveIndex())
	}

	// Ctrl+W closes c.go. Falls back to b.go (the prior tab).
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = upd.(model)
	if got := m.bufs.Count(); got != 2 {
		t.Fatalf("Count after Ctrl+W = %d, want 2", got)
	}
	if !strings.HasSuffix(m.activePath(), "b.go") {
		t.Fatalf("after closing c.go, active = %q, want b.go", m.activePath())
	}

	// Close remaining two buffers; welcome card should return.
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = upd.(model)
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = upd.(model)
	if m.bufs.Count() != 0 {
		t.Fatalf("Count after closing all = %d, want 0", m.bufs.Count())
	}
	if m.activePath() != "" {
		t.Fatalf("activePath after all closed = %q, want \"\"", m.activePath())
	}
	// Welcome card returns after the last buffer closes. The card renders
	// the name as "n  o  o  k" with double spaces, so check the spaced form.
	if !strings.Contains(m.View(), "n  o  o  k") {
		t.Fatalf("welcome card missing after closing all buffers")
	}
}

// TestTabFlowDirtyBlocksClose verifies Ctrl+W refuses to close a dirty buffer.
func TestTabFlowDirtyBlocksClose(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()

	updated, _ := m.Update(picker.SelectMsg{Item: picker.Item{Title: "a.go", Value: "a.go"}})
	m = updated.(model)

	// Make the buffer dirty by inserting text in-place.
	if p := m.bufs.Active(); p != nil {
		*p = p.InsertText("x")
	}
	if !m.bufs.Active().Dirty() {
		t.Fatal("expected buffer to be dirty after InsertText")
	}

	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = upd.(model)
	if m.bufs.Count() != 1 {
		t.Fatalf("dirty buffer should not close, Count = %d", m.bufs.Count())
	}
	if !strings.Contains(m.status, "dirty") {
		t.Fatalf("status should warn about dirty, got %q", m.status)
	}
}

// TestCtrlSWithoutLSPSavesPlain confirms Ctrl+S falls back to a plain
// SaveCmd when no language server is running — formatting is an
// optimization, not a precondition for saving.
func TestCtrlSWithoutLSPSavesPlain(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()

	upd, _ := m.Update(picker.SelectMsg{Item: picker.Item{Title: "a.go", Value: "a.go"}})
	m = upd.(model)

	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = upd.(model)
	if cmd == nil {
		t.Fatal("expected SaveCmd from Ctrl+S without LSP")
	}
	if msg, ok := cmd().(editor.SavedMsg); !ok {
		t.Fatalf("expected editor.SavedMsg, got %T", cmd())
	} else if msg.Err != nil {
		t.Fatalf("save errored: %v", msg.Err)
	}
	if !strings.Contains(m.status, "saving") {
		t.Errorf("status should mention saving, got %q", m.status)
	}
}

// TestAltSAlwaysSkipsFormat verifies alt+s bypasses formatting even when
// an LSP is connected and formatOnSave is on. This is the per-save
// escape hatch.
func TestAltSAlwaysSkipsFormat(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	// Sanity: default is on so the test is meaningful.
	if !m.formatOnSave {
		t.Fatal("formatOnSave should default to true")
	}

	upd, _ := m.Update(picker.SelectMsg{Item: picker.Item{Title: "a.go", Value: "a.go"}})
	m = upd.(model)

	// alt+s arrives as KeyRunes 's' with Alt=true.
	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: true})
	m = upd.(model)
	if cmd == nil {
		t.Fatal("expected SaveCmd from alt+s")
	}
	// alt+s never goes through FormattingCmd — the cmd it returns is
	// SaveCmd, which fires editor.SavedMsg on success.
	if _, ok := cmd().(editor.SavedMsg); !ok {
		t.Fatalf("expected editor.SavedMsg from alt+s, got %T", cmd())
	}
}

// TestAltShiftSTogglesFormatOnSave covers the persistent toggle: alt+S
// flips m.formatOnSave and updates status. Useful when the user wants
// the global default flipped (e.g. working with a half-broken file).
func TestAltShiftSTogglesFormatOnSave(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	if !m.formatOnSave {
		t.Fatal("formatOnSave should default to true")
	}
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}, Alt: true})
	m = upd.(model)
	if m.formatOnSave {
		t.Error("formatOnSave should be off after first alt+S")
	}
	if !strings.Contains(m.status, "off") {
		t.Errorf("status should reflect off, got %q", m.status)
	}
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}, Alt: true})
	m = upd.(model)
	if !m.formatOnSave {
		t.Error("formatOnSave should be back on after second alt+S")
	}
	if !strings.Contains(m.status, "on") {
		t.Errorf("status should reflect on, got %q", m.status)
	}
}

// TestCtrlSWithoutFileSurfacesHint confirms Ctrl+S without an open
// buffer prints a status hint instead of crashing on nil.
func TestCtrlSWithoutFileSurfacesHint(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = upd.(model)
	if cmd != nil {
		t.Errorf("expected nil cmd, got %T", cmd())
	}
	if !strings.Contains(m.status, "no file") {
		t.Errorf("status should mention no file, got %q", m.status)
	}
}

// TestFormattingMsgAppliesEdits verifies the format-msg handler applies
// edits to the active buffer, fires SaveCmd, and updates status. The
// LSP version is in sync so no stale-discard path triggers.
func TestFormattingMsgAppliesEdits(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()

	upd, _ := m.Update(picker.SelectMsg{Item: picker.Item{Title: "a.go", Value: "a.go"}})
	m = upd.(model)
	a := m.activePath()

	// Pretend a didChange at version 1 was published.
	m.lspVersions[a] = 1

	// Simulate gopls returning a single whole-file replace.
	formatted := "package a\n\nfunc Foo() {\n\treturn\n}\n"
	msg := lookup.FormattingMsg{
		Path:    a,
		Version: 1,
		Edits: []nooklsp.TextEdit{{
			StartLine: 0, StartCol: 0,
			EndLine: 3, EndCol: 0,
			NewText: formatted,
		}},
	}
	upd, cmd := m.Update(msg)
	m = upd.(model)

	p := m.bufs.Active()
	if p == nil {
		t.Fatal("expected active buffer")
	}
	if got := p.Contents(); got != strings.TrimRight(formatted, "\n") {
		// editor.ReplaceAllFromString splits on \n and drops the final
		// empty line, so Contents() rejoins without the trailing newline.
		t.Errorf("buffer contents = %q, want %q (modulo trailing newline)", got, strings.TrimRight(formatted, "\n"))
	}
	if cmd == nil {
		t.Fatal("expected save+didChange batch cmd")
	}
	if !strings.Contains(m.status, "formatted") {
		t.Errorf("status should mention formatted, got %q", m.status)
	}
	// Version was bumped for the post-format didChange.
	if got := m.lspVersions[a]; got != 2 {
		t.Errorf("lspVersions after format = %d, want 2", got)
	}
}

// TestFormattingMsgStaleVersionFallsBackToSave verifies that a response
// arriving after the buffer has moved on (version drift) doesn't apply
// the stale edits — it just saves the current buffer.
func TestFormattingMsgStaleVersionFallsBackToSave(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()

	upd, _ := m.Update(picker.SelectMsg{Item: picker.Item{Title: "a.go", Value: "a.go"}})
	m = upd.(model)
	a := m.activePath()

	originalContents := m.bufs.Active().Contents()
	// Buffer moved to version 5 while a format was in flight for version 2.
	m.lspVersions[a] = 5
	msg := lookup.FormattingMsg{
		Path:    a,
		Version: 2,
		Edits: []nooklsp.TextEdit{{
			StartLine: 0, StartCol: 0,
			EndLine: 99, EndCol: 0,
			NewText: "// garbage from stale format\n",
		}},
	}
	upd, cmd := m.Update(msg)
	m = upd.(model)

	p := m.bufs.Active()
	if got := p.Contents(); got != originalContents {
		t.Errorf("stale edits should not apply: got %q, want %q", got, originalContents)
	}
	if cmd == nil {
		t.Fatal("expected plain SaveCmd as fallback")
	}
	if !strings.Contains(m.status, "changed during format") {
		t.Errorf("status should explain the skip, got %q", m.status)
	}
}

func TestCtrlBTogglesTreePane(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()

	if m.showTree {
		t.Fatal("tree should default to hidden")
	}

	// Open.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	mm := updated.(model)
	if !mm.showTree {
		t.Fatal("ctrl+b should open the tree")
	}
	if !mm.treePane.Focused() {
		t.Fatal("opening the tree should focus it")
	}

	// Close.
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	mm = updated.(model)
	if mm.showTree {
		t.Fatal("second ctrl+b should hide the tree")
	}
	if mm.treePane.Focused() {
		t.Fatal("closing the tree should blur it")
	}
}

func TestTreePaneEscapeBlursButKeepsVisible(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	mm := updated.(model)
	if !mm.showTree || !mm.treePane.Focused() {
		t.Fatal("precondition: tree must be open and focused")
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm = updated.(model)
	if !mm.showTree {
		t.Errorf("esc should not hide the tree; got showTree=%v", mm.showTree)
	}
	if mm.treePane.Focused() {
		t.Errorf("esc should blur the tree")
	}
}

func TestTreeOpenMsgOpensBuffer(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()

	abs := filepath.Join(root, "a.go")
	updated, _ := m.Update(filetree.OpenMsg{Path: abs})
	mm := updated.(model)
	if mm.activePath() != abs {
		t.Errorf("expected active buffer %q, got %q", abs, mm.activePath())
	}
	if mm.treePane.Focused() {
		t.Errorf("opening a file should blur the tree so the editor can take keys")
	}
}

func TestTreeViewRenderedWhenShown(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()

	hiddenView := m.View()
	if strings.Contains(hiddenView, filepath.Base(root)+"\n") {
		// The status bar mentions the project name too; we use the
		// header-then-newline shape to disambiguate. Either way, the
		// hidden case shouldn't include the tree's bordered surface.
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	mm := updated.(model)
	shownView := mm.View()

	if len(shownView) <= len(hiddenView) {
		t.Errorf("View should grow when the tree is open; hidden=%d shown=%d",
			len(hiddenView), len(shownView))
	}
}

func TestTreeShrinksWhenEditorWouldStarve(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	// 60 cols is the smallest size resize() will paint at. With the
	// tree open AND the right pane active, the editor still needs at
	// least 20 cols, so the tree must shrink.
	m.width = 60
	m.height = 16
	m.right = rightGit
	m.showTree = true
	m = m.resize()
	if m.View() == "" {
		t.Fatal("expected non-empty view at minimum supported size")
	}
}

func TestTreeRoutesKeysOnlyWhenFocused(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()

	// Open and immediately blur (escape).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	mm := updated.(model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm = updated.(model)

	// Open a file so the editor exists; then press 'x' which the
	// blurred tree must NOT eat. The editor takes it as a no-op for
	// a no-file state, but the test asserts the tree didn't claim it.
	updated, _ = mm.Update(picker.SelectMsg{Item: picker.Item{Title: "a.go", Value: "a.go"}})
	mm = updated.(model)
	before := mm.treePane.Selected()
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm = updated.(model)
	if mm.treePane.Selected() != before {
		t.Errorf("blurred tree consumed KeyDown; cursor moved from %q to %q",
			before, mm.treePane.Selected())
	}
}
