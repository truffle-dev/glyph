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
	"github.com/truffle-dev/glyph/cmd/nook/internal/configwatch"
	"github.com/truffle-dev/glyph/cmd/nook/internal/diagnostics"
	"github.com/truffle-dev/glyph/cmd/nook/internal/edit"
	"github.com/truffle-dev/glyph/cmd/nook/internal/editor"
	"github.com/truffle-dev/glyph/cmd/nook/internal/filetree"
	"github.com/truffle-dev/glyph/cmd/nook/internal/ghost"
	"github.com/truffle-dev/glyph/cmd/nook/internal/git"
	"github.com/truffle-dev/glyph/cmd/nook/internal/inlayhint"
	"github.com/truffle-dev/glyph/cmd/nook/internal/lookup"
	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/cmd/nook/internal/multibuffer"
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

func TestAltDownCyclesSignatureOverloadForward(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()

	path := filepath.Join(root, "a.go")
	m.bufs.OpenOrSwitch(path)

	m.sigPane = m.sigPane.Open(nooklsp.SignatureInfo{
		Signatures: []nooklsp.Signature{
			{Label: "func A(x int)", ActiveParameter: -1},
			{Label: "func B(x int, y int)", ActiveParameter: -1},
			{Label: "func C()", ActiveParameter: -1},
		},
		ActiveSignature: 0,
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown, Alt: true})
	mm := updated.(model)
	if got := mm.sigPane.Info().ActiveSignature; got != 1 {
		t.Fatalf("expected ActiveSignature=1 after alt+down, got %d", got)
	}
	if !mm.sigPane.IsOpen() {
		t.Fatal("expected pane still open after cycle")
	}
}

func TestAltUpCyclesSignatureOverloadBackward(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()

	path := filepath.Join(root, "a.go")
	m.bufs.OpenOrSwitch(path)

	m.sigPane = m.sigPane.Open(nooklsp.SignatureInfo{
		Signatures: []nooklsp.Signature{
			{Label: "func A(x int)", ActiveParameter: -1},
			{Label: "func B(x int, y int)", ActiveParameter: -1},
			{Label: "func C()", ActiveParameter: -1},
		},
		ActiveSignature: 0,
	})

	// Wrap from first back to last.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp, Alt: true})
	mm := updated.(model)
	if got := mm.sigPane.Info().ActiveSignature; got != 2 {
		t.Fatalf("expected ActiveSignature=2 after alt+up wrap, got %d", got)
	}
}

func TestAltDownDoesNotCycleWhenSigPaneClosed(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()

	path := filepath.Join(root, "a.go")
	m.bufs.OpenOrSwitch(path)
	// Pane starts closed.
	if m.sigPane.IsOpen() {
		t.Fatal("expected sigPane closed at start of test")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown, Alt: true})
	mm := updated.(model)
	// With sigPane closed, alt+down falls through to the editor. The
	// important thing is the sigPane is still closed.
	if mm.sigPane.IsOpen() {
		t.Fatal("expected sigPane to remain closed when alt+down arrives with no pane")
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

// openBufferForTest opens a fixture file in the host model and returns the
// model in the "buffer is active" state. The picker.SelectMsg path is the
// only public way to seed a buffer, so we use it.
func openBufferForTest(t *testing.T, m model, relPath string) model {
	t.Helper()
	updated, _ := m.Update(picker.SelectMsg{Item: picker.Item{Title: relPath, Value: relPath}})
	mm := updated.(model)
	if mm.activePath() == "" {
		t.Fatalf("openBufferForTest: no active buffer after SelectMsg")
	}
	return mm
}

func TestHandleCodeActionMsgArmsPopupOnMatchingRequest(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")
	// Pretend the user just pressed alt+enter.
	m.caReqPath = m.activePath()
	m.caReqRow = 0
	m.caReqCol = 0
	msg := lookup.CodeActionMsg{
		Path: m.caReqPath, Row: 0, Col: 0,
		Items: []nooklsp.CodeActionItem{
			{Title: "Organize imports", Kind: "source.organizeImports", IsPreferred: true},
			{Title: "Extract function", Kind: "refactor.extract"},
		},
	}
	updated, _ := m.Update(msg)
	mm := updated.(model)
	if mm.overlay != overlayCodeAction {
		t.Errorf("expected overlayCodeAction, got %v", mm.overlay)
	}
	if mm.caPopup.Len() != 2 {
		t.Errorf("expected 2 items in popup, got %d", mm.caPopup.Len())
	}
}

func TestHandleCodeActionMsgDiscardsStaleResponse(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")
	// Request was for line 0; pretend the cursor moved to line 5.
	m.caReqPath = m.activePath()
	m.caReqRow = 5
	m.caReqCol = 0
	msg := lookup.CodeActionMsg{
		Path: m.caReqPath, Row: 0, Col: 0,
		Items: []nooklsp.CodeActionItem{{Title: "stale"}},
	}
	updated, _ := m.Update(msg)
	mm := updated.(model)
	if mm.overlay == overlayCodeAction {
		t.Errorf("stale response should be discarded; overlay was opened")
	}
}

func TestHandleCodeActionMsgEmptySurfacesStatus(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")
	m.caReqPath = m.activePath()
	msg := lookup.CodeActionMsg{Path: m.caReqPath, Items: nil}
	updated, _ := m.Update(msg)
	mm := updated.(model)
	if mm.overlay == overlayCodeAction {
		t.Errorf("empty result should NOT open the popup")
	}
	if !strings.Contains(mm.status, "no code actions") {
		t.Errorf("status should explain the empty result, got %q", mm.status)
	}
}

func TestAcceptCodeActionAppliesEditToOpenBuffer(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")
	// Build a code action that rewrites the first line ("package a\n") into
	// "package alpha\n". Edit range covers (0,0)-(0,9).
	abs := m.activePath()
	edit := nooklsp.WorkspaceEditChange{Files: map[string][]nooklsp.TextEdit{
		abs: {{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 9, NewText: "package alpha"}},
	}}
	m.caPopup = m.caPopup.WithItems([]nooklsp.CodeActionItem{
		{Title: "Rename package", Edit: edit},
	})
	m.overlay = overlayCodeAction
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(model)
	if mm.overlay == overlayCodeAction {
		t.Errorf("Enter should dismiss the popup")
	}
	if p := mm.bufs.Active(); p == nil || !strings.HasPrefix(p.Contents(), "package alpha") {
		t.Errorf("buffer not rewritten; contents=%q", p.Contents())
	}
}

func TestAcceptCodeActionRefusesDisabled(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")
	m.caPopup = m.caPopup.WithItems([]nooklsp.CodeActionItem{
		{Title: "blocked", Disabled: "needs gopls 1.x"},
	})
	m.overlay = overlayCodeAction
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(model)
	if mm.overlay == overlayCodeAction {
		t.Errorf("disabled-only popup should still close on Enter")
	}
	if !strings.Contains(mm.status, "disabled") {
		t.Errorf("status should mention the disabled reason; got %q", mm.status)
	}
}

func TestHandlePrepareRenameMsgArmsPromptWithIdentifier(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")
	// a.go: "package a\n\nfunc Foo() {}\n"
	// Position the prepareRename over "Foo" at (2, 5)-(2, 8).
	m.pendingRename = pendingRename{path: m.activePath(), row: 2, col: 5}
	msg := lookup.PrepareRenameMsg{
		Path: m.activePath(),
		Row:  2,
		Col:  5,
		Result: nooklsp.PrepareRenameResult{
			Available: true,
			StartLine: 2, StartCol: 5,
			EndLine: 2, EndCol: 8,
		},
	}
	updated, _ := m.Update(msg)
	mm := updated.(model)
	if mm.overlay != overlayRename {
		t.Errorf("expected overlayRename, got %v", mm.overlay)
	}
	if mm.renamePrompt.Current() != "Foo" {
		t.Errorf("placeholder = %q, want Foo", mm.renamePrompt.Current())
	}
}

func TestHandlePrepareRenameMsgWithZeroRangeFallsBackToCursor(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")
	// Modern gopls returns {defaultBehavior:true} which decodes to a zero
	// Range. The host should walk the source for the identifier itself.
	m.pendingRename = pendingRename{path: m.activePath(), row: 2, col: 6}
	msg := lookup.PrepareRenameMsg{
		Path: m.activePath(),
		Row:  2,
		Col:  6,
		Result: nooklsp.PrepareRenameResult{
			Available: true,
		},
	}
	updated, _ := m.Update(msg)
	mm := updated.(model)
	if mm.renamePrompt.Current() != "Foo" {
		t.Errorf("zero-range placeholder = %q, want Foo", mm.renamePrompt.Current())
	}
}

func TestHandlePrepareRenameMsgUnavailableShowsStatus(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")
	m.pendingRename = pendingRename{path: m.activePath(), row: 0, col: 0}
	msg := lookup.PrepareRenameMsg{
		Path: m.activePath(),
		Row:  0,
		Col:  0,
		Result: nooklsp.PrepareRenameResult{
			Available: false,
		},
	}
	updated, _ := m.Update(msg)
	mm := updated.(model)
	if mm.overlay == overlayRename {
		t.Errorf("unavailable rename should NOT open the prompt")
	}
	if !strings.Contains(mm.status, "not available") {
		t.Errorf("status should explain unavailable; got %q", mm.status)
	}
}

func TestRenamePromptAcceptFiresRenameCmd(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")
	// Arm the rename prompt with placeholder Foo.
	m.pendingRename = pendingRename{path: m.activePath(), row: 2, col: 5}
	m.renamePrompt = m.renamePrompt.WithCurrent("Foo", "a.go")
	m.overlay = overlayRename
	// Backspace away the placeholder, then type "Bar".
	for i := 0; i < 3; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = updated.(model)
	}
	for _, r := range "Bar" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	if m.renamePrompt.Value() != "Bar" {
		t.Errorf("expected prompt value Bar, got %q", m.renamePrompt.Value())
	}
	// Enter should fire a RenameCmd (we can't easily assert it runs without
	// gopls; assert the cmd is non-nil so we know the flow plumbed through).
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(model)
	if cmd == nil {
		t.Errorf("Enter on filled prompt should fire a RenameCmd")
	}
	if !strings.Contains(mm.status, "renaming Foo → Bar") {
		t.Errorf("status should announce the rename; got %q", mm.status)
	}
}

func TestRenamePromptEscCancels(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 120
	m.height = 30
	m.renamePrompt = m.renamePrompt.WithCurrent("Foo", "a.go")
	m.overlay = overlayRename
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(model)
	if mm.overlay == overlayRename {
		t.Errorf("Esc should close the rename prompt")
	}
	if !strings.Contains(mm.status, "cancelled") {
		t.Errorf("status should mention cancellation; got %q", mm.status)
	}
}

func TestHandleRenameMsgAppliesAcrossOpenBufferAndDisk(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	// Open a.go but leave sub/b.go closed; the rename touches both.
	m = openBufferForTest(t, m, "a.go")
	subAbs := filepath.Join(root, "sub", "b.go")
	aAbs := m.activePath()
	// Rewrite "Foo" in a.go (line 2) and "Bar" in sub/b.go (line 2).
	edit := nooklsp.WorkspaceEditChange{Files: map[string][]nooklsp.TextEdit{
		aAbs:   {{StartLine: 2, StartCol: 5, EndLine: 2, EndCol: 8, NewText: "Renamed"}},
		subAbs: {{StartLine: 2, StartCol: 5, EndLine: 2, EndCol: 8, NewText: "Renamed"}},
	}}
	m.renamePrompt = m.renamePrompt.WithCurrent("Foo", "a.go")
	m.overlay = overlayRename
	updated, _ := m.Update(lookup.RenameMsg{NewName: "Renamed", Edit: edit})
	mm := updated.(model)
	if mm.overlay == overlayRename {
		t.Errorf("rename apply should dismiss the prompt")
	}
	if p := mm.bufs.Active(); p == nil || !strings.Contains(p.Contents(), "func Renamed()") {
		t.Errorf("buffer not rewritten; contents=%q", func() string {
			if p := mm.bufs.Active(); p != nil {
				return p.Contents()
			}
			return ""
		}())
	}
	b, err := os.ReadFile(subAbs)
	if err != nil {
		t.Fatalf("read sub/b.go: %v", err)
	}
	if !strings.Contains(string(b), "func Renamed()") {
		t.Errorf("disk file not rewritten; got %q", string(b))
	}
}

func TestHandleRenameMsgErrorSurfacesInPrompt(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 120
	m.height = 30
	m.renamePrompt = m.renamePrompt.WithCurrent("Foo", "a.go")
	m.overlay = overlayRename
	updated, _ := m.Update(lookup.RenameMsg{NewName: "Bar", Err: errTest})
	mm := updated.(model)
	if mm.overlay != overlayRename {
		t.Errorf("error should leave the prompt open so the user can retry")
	}
	// The prompt itself records the error string; we can't read it directly
	// without a getter, so re-render and check the view contains it.
	view := mm.renamePrompt.View(mm.theme, 60)
	if !strings.Contains(view, errTest.Error()) {
		t.Errorf("prompt view missing error; got %q", view)
	}
}

func TestF2OnlyTriggersRenameWhenBufferOpen(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 120
	m.height = 30
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	mm := updated.(model)
	if cmd != nil {
		t.Errorf("F2 with no buffer should NOT fire a cmd")
	}
	if !strings.Contains(mm.status, "open a file first") {
		t.Errorf("F2 with no buffer should nudge the user to open a file; got %q", mm.status)
	}
}

func TestAltEnterTriggersCodeActions(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	mm := updated.(model)
	if cmd == nil {
		t.Errorf("alt+enter should fire a CodeActionCmd")
	}
	if mm.caReqPath == "" {
		t.Errorf("alt+enter should stash the request path")
	}
}

// errTest is a sentinel for tests that just need a non-nil error value.
var errTest = errSentinel("test error")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

func TestAltMOpensMultibuffer(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}, Alt: true})
	mm := updated.(model)
	if mm.overlay != overlayMultibuffer {
		t.Fatalf("alt+m did not open multibuffer overlay; got overlay=%v", mm.overlay)
	}
	if !mm.multibufPane.Focused() {
		t.Errorf("alt+m did not focus the multibuffer pane")
	}
	if cmd == nil {
		t.Errorf("alt+m should fire LoadDiffCmd")
	}
	if mm.multibufPane.Title() != "uncommitted changes" {
		t.Errorf("multibuffer title = %q, want %q", mm.multibufPane.Title(), "uncommitted changes")
	}
}

func TestMultibufferFragmentsMsgLoadsPane(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 120
	m.height = 30
	m = m.resize()
	m.overlay = overlayMultibuffer
	m.multibufPane = m.multibufPane.Reset("uncommitted changes").Focus()

	frags := []multibuffer.Fragment{{
		Path: "/repo/x.go", StartLine: 5, EndLine: 5,
		Lines: []multibuffer.Line{{Marker: multibuffer.Added, FileLine: 5, Text: "hi"}},
	}}
	updated, _ := m.Update(multibuffer.FragmentsMsg{Fragments: frags, Source: "diff"})
	mm := updated.(model)
	if got := mm.multibufPane.Fragments(); len(got) != 1 {
		t.Errorf("expected 1 fragment loaded into pane, got %d", len(got))
	}
}

func TestMultibufferOpenAtMsgJumpsAndCloses(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m.overlay = overlayMultibuffer

	updated, _ := m.Update(multibuffer.OpenAtMsg{
		Path: filepath.Join(root, "a.go"),
		Line: 3,
	})
	mm := updated.(model)
	if mm.overlay != overlayNone {
		t.Errorf("OpenAtMsg should clear overlay; got %v", mm.overlay)
	}
	if mm.activePath() == "" || !strings.HasSuffix(mm.activePath(), "a.go") {
		t.Errorf("OpenAtMsg should open the target file; activePath = %q", mm.activePath())
	}
	if p := mm.bufs.Active(); p != nil && p.CursorRow() != 2 {
		t.Errorf("OpenAtMsg cursor row = %d, want 2 (1-based line 3 → 0-based row 2)", p.CursorRow())
	}
}

func TestMultibufferCancelMsgClosesOverlay(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	m.overlay = overlayMultibuffer
	m.multibufPane = m.multibufPane.Focus()

	updated, _ := m.Update(multibuffer.CancelMsg{})
	mm := updated.(model)
	if mm.overlay != overlayNone {
		t.Errorf("CancelMsg should clear overlay; got %v", mm.overlay)
	}
	if mm.multibufPane.Focused() {
		t.Errorf("CancelMsg should blur the multibuffer pane")
	}
}

func TestMultibufferOverlayRoutesKeys(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	m.overlay = overlayMultibuffer
	m.multibufPane = m.multibufPane.Focus()

	// Esc on the overlay should produce a CancelMsg cmd.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc on multibuffer overlay returned nil cmd")
	}
	if _, ok := cmd().(multibuffer.CancelMsg); !ok {
		t.Errorf("Esc on multibuffer overlay produced %T, want CancelMsg", cmd())
	}
}

func TestAltPOpensDiagnostics(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 120
	m.height = 30
	m = m.resize()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}, Alt: true})
	mm := updated.(model)
	if mm.overlay != overlayDiagnostics {
		t.Fatalf("alt+p did not open diagnostics overlay; got overlay=%v", mm.overlay)
	}
	if !mm.diagPane.IsFocused() {
		t.Errorf("alt+p did not focus the diagnostics pane")
	}
}

func TestDiagnosticsCollectsAllOpenBuffers(t *testing.T) {
	m := newModel(t.TempDir())
	m.diagnostics["/work/a.go"] = []protocol.Diagnostic{
		{
			Range:    protocol.Range{Start: protocol.Position{Line: 4, Character: 9}},
			Severity: protocol.DiagnosticSeverityError,
			Source:   "gopls",
			Message:  "undefined: foo",
		},
	}
	m.diagnostics["/work/b.go"] = []protocol.Diagnostic{
		{
			Range:    protocol.Range{Start: protocol.Position{Line: 0, Character: 0}},
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   "gopls",
			Message:  "unused variable",
		},
	}
	entries := m.collectDiagnosticEntries()
	if len(entries) != 2 {
		t.Fatalf("collectDiagnosticEntries len = %d, want 2", len(entries))
	}
	paths := map[string]diagnostics.Entry{}
	for _, e := range entries {
		paths[e.Path] = e
	}
	if a, ok := paths["/work/a.go"]; !ok {
		t.Errorf("missing a.go entry")
	} else {
		if a.Row != 4 || a.Col != 9 || a.Severity != diagnostics.SeverityError || a.Source != "gopls" {
			t.Errorf("a.go entry = %+v", a)
		}
	}
	if b, ok := paths["/work/b.go"]; !ok {
		t.Errorf("missing b.go entry")
	} else {
		if b.Severity != diagnostics.SeverityWarning {
			t.Errorf("b.go severity = %v, want Warning", b.Severity)
		}
	}
}

func TestDiagnosticsCollectStripsMessageNewlines(t *testing.T) {
	m := newModel(t.TempDir())
	m.diagnostics["/work/x.go"] = []protocol.Diagnostic{
		{
			Range:    protocol.Range{},
			Severity: protocol.DiagnosticSeverityError,
			Message:  "line one\nline two",
		},
	}
	entries := m.collectDiagnosticEntries()
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	if strings.Contains(entries[0].Message, "\n") {
		t.Errorf("Message should have newlines stripped, got %q", entries[0].Message)
	}
	if !strings.Contains(entries[0].Message, "line one line two") {
		t.Errorf("Message = %q, want collapsed", entries[0].Message)
	}
}

func TestDiagnosticsOpenAtMsgJumpsAndCloses(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m.overlay = overlayDiagnostics
	m.diagPane = m.diagPane.Focus()

	updated, _ := m.Update(diagnostics.OpenAtMsg{
		Path: filepath.Join(root, "a.go"),
		Row:  2, // 0-based — host adds +1 when calling JumpTo
		Col:  5,
	})
	mm := updated.(model)
	if mm.overlay != overlayNone {
		t.Errorf("OpenAtMsg should clear overlay; got %v", mm.overlay)
	}
	if mm.activePath() == "" || !strings.HasSuffix(mm.activePath(), "a.go") {
		t.Errorf("OpenAtMsg should open the target file; activePath = %q", mm.activePath())
	}
	if mm.diagPane.IsFocused() {
		t.Errorf("OpenAtMsg should blur diagnostics pane")
	}
	if p := mm.bufs.Active(); p != nil {
		if p.CursorRow() != 2 {
			t.Errorf("cursor row = %d, want 2", p.CursorRow())
		}
	}
}

func TestDiagnosticsCancelMsgClosesOverlay(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	m.overlay = overlayDiagnostics
	m.diagPane = m.diagPane.Focus()

	updated, _ := m.Update(diagnostics.CancelMsg{})
	mm := updated.(model)
	if mm.overlay != overlayNone {
		t.Errorf("CancelMsg should clear overlay; got %v", mm.overlay)
	}
	if mm.diagPane.IsFocused() {
		t.Errorf("CancelMsg should blur the diagnostics pane")
	}
}

func TestDiagnosticsOverlayRoutesKeys(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	m.overlay = overlayDiagnostics
	m.diagPane = m.diagPane.Focus()

	// Esc on the overlay should produce a CancelMsg cmd.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc on diagnostics overlay returned nil cmd")
	}
	if _, ok := cmd().(diagnostics.CancelMsg); !ok {
		t.Errorf("Esc on diagnostics overlay produced %T, want CancelMsg", cmd())
	}
}

// TestInlayHintsDefaultOn confirms the host boots with hints enabled. Users
// who want to suppress them invoke alt+y; the default is the configured-on
// behavior gopls power-users expect.
func TestInlayHintsDefaultOn(t *testing.T) {
	m := newModel(t.TempDir())
	if !m.inlayHintsOn {
		t.Fatal("inlay hints should default to on")
	}
}

// TestApplyInlayHintsRoutesToPane verifies that an InlayHintMsg for the
// active buffer lands as a row→hint-slice map on the editor pane.
func TestApplyInlayHintsRoutesToPane(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	path := filepath.Join(root, "x.go")
	if err := os.WriteFile(path, []byte("package x\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.bufs.OpenOrSwitch(path)
	m.lspVersions[path] = 3

	hints := map[int][]inlayhint.Hint{
		2: {{Row: 2, Col: 9, Label: ": int", Kind: inlayhint.KindType}},
	}
	updated, _ := m.Update(lookup.InlayHintMsg{Path: path, Version: 3, Hints: hints})
	mm := updated.(model)
	pa := mm.bufs.Active()
	got := pa.InlayHintsAt(2)
	if len(got) != 1 || got[0].Label != ": int" {
		t.Fatalf("InlayHintsAt(2) = %#v, want one hint with label ': int'", got)
	}
}

// TestInlayHintsStaleVersionDiscarded confirms that a response carrying a
// version older than what the host already advanced past gets dropped.
// gopls answering for didChange v3 after the user typed up to v5 should
// leave the pane's hints alone.
func TestInlayHintsStaleVersionDiscarded(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	path := filepath.Join(root, "x.go")
	if err := os.WriteFile(path, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.bufs.OpenOrSwitch(path)
	m.lspVersions[path] = 5

	hints := map[int][]inlayhint.Hint{0: {{Row: 0, Col: 0, Label: "stale"}}}
	updated, _ := m.Update(lookup.InlayHintMsg{Path: path, Version: 3, Hints: hints})
	mm := updated.(model)
	pa := mm.bufs.Active()
	if got := pa.InlayHintsAt(0); len(got) != 0 {
		t.Fatalf("stale-version response should not apply; got %d hints", len(got))
	}
}

// TestAltYTogglesInlayHints walks through the toggle: off clears every
// pane's hints, on flips the bit back. The refresh cmd is nil here because
// no LSP client is wired up in the test harness; that's the intended
// behavior — the toggle survives a missing language server.
func TestAltYTogglesInlayHints(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	path := filepath.Join(root, "x.go")
	if err := os.WriteFile(path, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.bufs.OpenOrSwitch(path)
	pa := m.bufs.Active()
	*pa = pa.SetInlayHints(map[int][]inlayhint.Hint{
		0: {{Row: 0, Col: 0, Label: "before"}},
	})

	// Alt+y off.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}, Alt: true})
	mm := updated.(model)
	if mm.inlayHintsOn {
		t.Fatal("alt+y should toggle inlayHintsOn off")
	}
	if got := mm.bufs.Active().InlayHintsAt(0); len(got) != 0 {
		t.Fatalf("alt+y off should clear hints; got %d", len(got))
	}

	// Alt+y back on.
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}, Alt: true})
	mm = updated.(model)
	if !mm.inlayHintsOn {
		t.Fatal("second alt+y should toggle inlayHintsOn back on")
	}
}

// TestInlayHintCmdNilWhenDisabled confirms the refresh helper is a no-op
// when the toggle is off, even with an active buffer and an LSP version
// recorded. Saves an unnecessary RPC on every save.
func TestInlayHintCmdNilWhenDisabled(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	m.inlayHintsOn = false
	path := filepath.Join(root, "x.go")
	if err := os.WriteFile(path, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.bufs.OpenOrSwitch(path)
	m.lspVersions[path] = 1
	if cmd := m.refreshInlayHintsCmd(); cmd != nil {
		t.Fatal("refreshInlayHintsCmd should be nil when hints are disabled")
	}
}

// writeNookConfig points XDG_CONFIG_HOME at a fresh temp dir, writes the
// given TOML body under nook/config.toml, and returns the resolved path.
// Use this in v0.15.0 host tests to drive newModel + alt+, behavior.
func writeNookConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg := filepath.Join(dir, "nook", "config.toml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestNewModelLoadsConfigOptions(t *testing.T) {
	writeNookConfig(t, `[editor]
tab_width = 2
format_on_save = false
line_numbers = false
indent_guides = false
inlay_hints = false
theme = "tokyo-night"
`)
	m := newModel(t.TempDir())
	if m.formatOnSave {
		t.Error("config format_on_save=false not applied")
	}
	if m.inlayHintsOn {
		t.Error("config inlay_hints=false not applied")
	}
	if m.tabWidth != 2 {
		t.Errorf("tabWidth = %d, want 2", m.tabWidth)
	}
	if m.themeName != "tokyo-night" {
		t.Errorf("themeName = %q, want %q", m.themeName, "tokyo-night")
	}
	// Sanity: a buffer opened after startup picks up the tab width, the
	// line-numbers toggle, and the indent-guides toggle from the manager.
	root := t.TempDir()
	path := filepath.Join(root, "x.go")
	if err := os.WriteFile(path, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.bufs.OpenOrSwitch(path)
	p := m.bufs.Active()
	if p.TabWidth() != 2 {
		t.Errorf("opened buffer TabWidth = %d, want 2", p.TabWidth())
	}
	if p.LineNumbers() {
		t.Error("opened buffer LineNumbers = true, want false")
	}
	if p.IndentGuides() {
		t.Error("opened buffer IndentGuides = true, want false")
	}
}

func TestAltCommaReloadsIndentGuides(t *testing.T) {
	cfg := writeNookConfig(t, `[editor]
indent_guides = true
`)
	root := t.TempDir()
	path := filepath.Join(root, "x.go")
	if err := os.WriteFile(path, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModel(t.TempDir())
	m.bufs.OpenOrSwitch(path)
	if !m.bufs.Active().IndentGuides() {
		t.Fatal("startup state: indent guides should be on")
	}
	// Rewrite the same file with indent_guides=false, then fire alt+,.
	if err := os.WriteFile(cfg, []byte(`[editor]
indent_guides = false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Alt: true})
	mm := updated.(model)
	if mm.bufs.Active().IndentGuides() {
		t.Error("reload did not turn off indent guides on the active buffer")
	}
}

func TestNewModelDefaultsWhenNoConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	m := newModel(t.TempDir())
	if !m.formatOnSave {
		t.Error("formatOnSave should default to true")
	}
	if !m.inlayHintsOn {
		t.Error("inlayHintsOn should default to true")
	}
	if m.tabWidth != 4 {
		t.Errorf("tabWidth = %d, want default 4", m.tabWidth)
	}
	if m.themeName != "default" {
		t.Errorf("themeName = %q, want %q", m.themeName, "default")
	}
}

func TestNewModelUnknownThemeFallsBack(t *testing.T) {
	writeNookConfig(t, `[editor]
theme = "purple-pony"
`)
	m := newModel(t.TempDir())
	// Status hint should mention the unknown theme so the user can fix it.
	if !strings.Contains(m.status, "purple-pony") {
		t.Errorf("status = %q, want it to mention purple-pony", m.status)
	}
	// Theme should be the safe default — same Bg as theme.Default.
	if m.theme.Bg == "" {
		t.Fatal("model theme has empty Bg")
	}
}

func TestNewModelMalformedConfigSurfacesError(t *testing.T) {
	writeNookConfig(t, `[editor
tab_width = 4
`)
	m := newModel(t.TempDir())
	if !strings.HasPrefix(m.status, "config:") {
		t.Errorf("status = %q, want it to start with %q", m.status, "config:")
	}
	// Defaults must still apply so the editor is usable.
	if !m.formatOnSave || !m.inlayHintsOn || m.tabWidth != 4 {
		t.Errorf("malformed config did not fall back to defaults: %+v", m)
	}
}

func TestAltCommaReloadsConfig(t *testing.T) {
	cfg := writeNookConfig(t, `[editor]
inlay_hints = true
tab_width = 4
`)
	m := newModel(t.TempDir())
	if !m.inlayHintsOn || m.tabWidth != 4 {
		t.Fatalf("startup state unexpected: %+v", m)
	}
	// Rewrite the same file with new values, then fire alt+,.
	if err := os.WriteFile(cfg, []byte(`[editor]
inlay_hints = false
tab_width = 8
format_on_save = false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Alt: true})
	mm := updated.(model)
	if mm.inlayHintsOn {
		t.Error("reload did not apply inlay_hints=false")
	}
	if mm.formatOnSave {
		t.Error("reload did not apply format_on_save=false")
	}
	if mm.tabWidth != 8 {
		t.Errorf("reload tabWidth = %d, want 8", mm.tabWidth)
	}
	if !strings.Contains(mm.status, "reloaded") {
		t.Errorf("status = %q, want it to mention reload", mm.status)
	}
}

func TestAltCommaThemeChangeAppliesLive(t *testing.T) {
	cfg := writeNookConfig(t, `[editor]
theme = "default"
`)
	m := newModel(t.TempDir())
	defaultTheme := m.theme
	if err := os.WriteFile(cfg, []byte(`[editor]
theme = "tokyo-night"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Alt: true})
	mm := updated.(model)
	if mm.themeName != "tokyo-night" {
		t.Errorf("themeName after reload = %q, want %q", mm.themeName, "tokyo-night")
	}
	// v0.38.0: theme is swapped on the host immediately; no restart hint.
	if mm.theme == defaultTheme {
		t.Errorf("host theme not swapped on reload")
	}
	if strings.Contains(mm.status, "restart") {
		t.Errorf("status mentions restart but v0.38.0 applies theme live: %q", mm.status)
	}
	if !strings.Contains(mm.status, "tokyo-night") {
		t.Errorf("status = %q, want it to mention the new theme", mm.status)
	}
}

// writeProjectConfig writes the given TOML body to <root>/.nook/config.toml
// and returns the resolved path. Use in v0.42.0 host tests that exercise
// project-config inheritance.
func writeProjectConfig(t *testing.T, root, body string) string {
	t.Helper()
	dir := filepath.Join(root, ".nook")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewModelLayersProjectConfigOnTopOfUser(t *testing.T) {
	// User config sets tab_width=2 + theme=tokyo-night. Project config
	// overrides only tab_width=8 + format_on_save=false. Result must be
	// project's tab_width and format_on_save, user's theme, defaults for
	// everything else.
	writeNookConfig(t, `[editor]
tab_width = 2
theme = "tokyo-night"
`)
	root := t.TempDir()
	writeProjectConfig(t, root, `[editor]
tab_width = 8
format_on_save = false
`)
	m := newModel(root)
	if m.tabWidth != 8 {
		t.Errorf("tabWidth = %d, want 8 (project wins)", m.tabWidth)
	}
	if m.formatOnSave {
		t.Error("formatOnSave = true, want false (project's explicit false must win)")
	}
	if m.themeName != "tokyo-night" {
		t.Errorf("themeName = %q, want %q (user passes through)", m.themeName, "tokyo-night")
	}
	if !m.inlayHintsOn {
		t.Error("inlayHintsOn = false, want true (Default passes through)")
	}
}

func TestNewModelProjectOnlyConfigApplies(t *testing.T) {
	// No user file. Project file sets theme + tab_width. Result must
	// equal Default merged with the project layer.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	root := t.TempDir()
	writeProjectConfig(t, root, `[editor]
tab_width = 8
theme = "rose-pine"
`)
	m := newModel(root)
	if m.tabWidth != 8 {
		t.Errorf("tabWidth = %d, want 8", m.tabWidth)
	}
	if m.themeName != "rose-pine" {
		t.Errorf("themeName = %q, want %q", m.themeName, "rose-pine")
	}
}

func TestNewModelMalformedProjectConfigSurfacesError(t *testing.T) {
	writeNookConfig(t, `[editor]
tab_width = 4
`)
	root := t.TempDir()
	writeProjectConfig(t, root, "this is = = not toml")
	m := newModel(root)
	if !strings.HasPrefix(m.status, "project config:") {
		t.Errorf("status = %q, want it to start with %q", m.status, "project config:")
	}
	// User config still applied — defaults must still be sane.
	if m.tabWidth != 4 {
		t.Errorf("tabWidth = %d, want 4 (user fallback)", m.tabWidth)
	}
}

func TestAltCommaReloadsMergedConfigOnProjectEdit(t *testing.T) {
	writeNookConfig(t, `[editor]
tab_width = 2
inlay_hints = true
`)
	root := t.TempDir()
	prj := writeProjectConfig(t, root, `[editor]
tab_width = 4
`)
	m := newModel(root)
	if m.tabWidth != 4 {
		t.Fatalf("startup tabWidth = %d, want 4 (project wins)", m.tabWidth)
	}
	// Rewrite the project file; alt+, picks up the new value via the
	// merged reload path.
	if err := os.WriteFile(prj, []byte("[editor]\ntab_width = 8\ninlay_hints = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Alt: true})
	mm := updated.(model)
	if mm.tabWidth != 8 {
		t.Errorf("tabWidth = %d, want 8 (project edit picked up)", mm.tabWidth)
	}
	if mm.inlayHintsOn {
		t.Error("inlayHintsOn = true, want false (project's explicit false wins)")
	}
	if !strings.Contains(mm.status, "user + project") {
		t.Errorf("status = %q, want scope hint mentioning user + project", mm.status)
	}
}

func TestProjectTickMsgRoutesAndReloads(t *testing.T) {
	// Simulates the configwatch goroutine landing a TickMsg for the
	// project path. The handler should refetch + reload + re-arm
	// independently of the user-path tick.
	writeNookConfig(t, "[editor]\ntab_width = 2\n")
	root := t.TempDir()
	prj := writeProjectConfig(t, root, "[editor]\ntab_width = 4\n")
	m := newModel(root)
	if m.tabWidth != 4 {
		t.Fatalf("startup tabWidth = %d, want 4", m.tabWidth)
	}
	if err := os.WriteFile(prj, []byte("[editor]\ntab_width = 8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Same-second writes on ext4-relatime can keep mtime + size identical.
	// Forcing Last to a zero fingerprint guarantees Changed() == true so
	// the test asserts the routing + reload contract, not the underlying
	// mtime granularity.
	tick := configwatch.TickMsg{
		Path: m.prjCfgPath,
		Last: configwatch.Fingerprint{},
		Cur:  configwatch.Snapshot(m.prjCfgPath),
	}
	updated, _ := m.Update(tick)
	mm := updated.(model)
	if mm.tabWidth != 8 {
		t.Errorf("after project TickMsg tabWidth = %d, want 8", mm.tabWidth)
	}
	if mm.prjCfgFinger != tick.Cur {
		t.Error("prjCfgFinger not advanced after project TickMsg")
	}
}

func TestAltCommaTurningOffInlayHintsClearsActiveBuffer(t *testing.T) {
	cfg := writeNookConfig(t, `[editor]
inlay_hints = true
`)
	root := t.TempDir()
	m := newModel(root)
	path := filepath.Join(root, "x.go")
	if err := os.WriteFile(path, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.bufs.OpenOrSwitch(path)
	pa := m.bufs.Active()
	*pa = pa.SetInlayHints(map[int][]inlayhint.Hint{
		0: {{Row: 0, Col: 0, Label: "stale"}},
	})
	if err := os.WriteFile(cfg, []byte(`[editor]
inlay_hints = false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Alt: true})
	mm := updated.(model)
	if mm.inlayHintsOn {
		t.Fatal("inlay_hints did not disable on reload")
	}
	if got := mm.bufs.Active().InlayHintsAt(0); len(got) != 0 {
		t.Fatalf("disabled-hint reload did not clear active buffer hints; got %d", len(got))
	}
}

// TestAcceptCompletionSnippetEntersSnippetMode confirms that when the
// server marks a completion item as a snippet (InsertTextFormat == 2),
// acceptCompletion routes the body through the nook snippet engine and
// the editor enters snippet mode at the first tabstop.
func TestAcceptCompletionSnippetEntersSnippetMode(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")

	// Type a prefix the popup is filtering on. Position the cursor at the end
	// of "Pri" so WordPrefix returns "Pri" and prefixStart deletes it.
	p := m.bufs.Active()
	*p = p.JumpTo(2, 13) // "func Foo() {}" — append at end of "Foo" line
	*p = p.InsertText("Pri")
	row := p.CursorRow()
	col := p.CursorCol()
	if col < 3 {
		t.Fatalf("setup: expected col >= 3, got %d at row %d", col, row)
	}

	m.completePopup = m.completePopup.WithItems([]nooklsp.CompletionItem{
		{
			Label:            "Printf",
			InsertText:       "Printf(${1:format}, ${2:args})",
			Kind:             nooklsp.CompletionKindFunction,
			InsertTextFormat: nooklsp.InsertTextFormatSnippet,
		},
	}, 3)
	m.overlay = overlayCompletion

	updated, _ := m.acceptCompletion()
	mm := updated.(model)
	if mm.overlay == overlayCompletion {
		t.Errorf("acceptCompletion should dismiss the popup")
	}
	pp := mm.bufs.Active()
	if pp == nil {
		t.Fatalf("no active buffer after acceptCompletion")
	}
	if !pp.InSnippetMode() {
		t.Fatalf("editor should be in snippet mode after accepting a snippet completion")
	}
	ts, ok := pp.CurrentSnippetTabstop()
	if !ok {
		t.Fatalf("no current tabstop after acceptCompletion")
	}
	if ts.Index != 1 {
		t.Errorf("first tabstop should be index 1, got %d", ts.Index)
	}
	if !strings.Contains(pp.Line(row), "Printf(format, args)") {
		t.Errorf("snippet body not expanded into buffer: %q", pp.Line(row))
	}
	if !strings.Contains(mm.status, "inserted Printf") {
		t.Errorf("status should report inserted, got %q", mm.status)
	}
}

// TestAcceptCompletionPlainTextStillWorks confirms the existing
// plain-insert branch is untouched by the snippet wiring.
func TestAcceptCompletionPlainTextStillWorks(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m = openBufferForTest(t, m, "a.go")

	p := m.bufs.Active()
	*p = p.JumpTo(2, 13)
	*p = p.InsertText("Pri")
	row := p.CursorRow()

	m.completePopup = m.completePopup.WithItems([]nooklsp.CompletionItem{
		{
			Label:            "Println",
			InsertText:       "Println",
			Kind:             nooklsp.CompletionKindFunction,
			InsertTextFormat: nooklsp.InsertTextFormatPlainText,
		},
	}, 3)
	m.overlay = overlayCompletion

	updated, _ := m.acceptCompletion()
	mm := updated.(model)
	pp := mm.bufs.Active()
	if pp.InSnippetMode() {
		t.Errorf("plain-text accept should not enter snippet mode")
	}
	if !strings.Contains(pp.Line(row), "Println") {
		t.Errorf("plain insert not visible: row=%d line=%q contents=%q", row, pp.Line(row), pp.Contents())
	}
	// The "Pri" prefix should have been replaced, not appended.
	if strings.Contains(pp.Line(row), "PriPrintln") {
		t.Errorf("prefix was not consumed: %q", pp.Line(row))
	}
}

func TestF9TogglesBreakpointOnActiveBuffer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {\n\tprintln(1)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModel(dir)
	m.bufs.OpenOrSwitch(path)
	if p := m.bufs.Active(); p != nil {
		*p = p.JumpTo(4, 1) // row 4 (1-based) → cursor row 3
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF9})
	mm := updated.(model)
	if !mm.bpStore.Has(path, 4) {
		t.Fatalf("F9 did not record breakpoint at row 4 in store; rows=%v", mm.bpStore.Rows(path))
	}
	p := mm.bufs.Active()
	if p == nil || !p.IsBreakpoint(3) {
		t.Errorf("F9 did not mark row 3 on pane as breakpoint")
	}
	if !strings.Contains(mm.status, "breakpoint set") {
		t.Errorf("status not updated; got %q", mm.status)
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyF9})
	mm = updated.(model)
	if mm.bpStore.Has(path, 4) {
		t.Errorf("second F9 did not clear breakpoint")
	}
	p = mm.bufs.Active()
	if p != nil && p.IsBreakpoint(3) {
		t.Errorf("pane still shows breakpoint after clear")
	}
	if !strings.Contains(mm.status, "breakpoint cleared") {
		t.Errorf("status not updated; got %q", mm.status)
	}
}

func TestF9NoOpWhenNoBufferOpen(t *testing.T) {
	m := newModel(t.TempDir())
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyF9})
	mm := updated.(model)
	if cmd != nil {
		t.Errorf("F9 with no buffer should not return a cmd")
	}
	if !strings.Contains(mm.status, "no file open") {
		t.Errorf("status: %q", mm.status)
	}
}

func TestDebugStatusBarSegmentAppearsWithBreakpoints(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModel(dir)
	m.width = 120
	m.height = 30
	m = m.resize()
	m.bufs.OpenOrSwitch(path)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF9})
	mm := updated.(model)
	bar := mm.renderStatusBar()
	if !strings.Contains(stripANSI(bar), "dbg") {
		t.Errorf("status bar missing dbg segment: %q", stripANSI(bar))
	}
}

// TestCtrlSlashTogglesLineComment confirms ctrl+/ runs the comment
// transform on the active buffer. Ctrl+/ on every xterm-style emulator
// folds into 0x1F, which bubbletea surfaces as KeyCtrlUnderscore.
func TestCtrlSlashTogglesLineComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("x := 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModel(dir)
	m.width = 120
	m.height = 30
	m = m.resize()
	m.bufs.OpenOrSwitch(path)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlUnderscore})
	mm := updated.(model)
	p := mm.bufs.Active()
	if p == nil {
		t.Fatal("expected active buffer after Ctrl+/")
	}
	if got := p.Line(0); got != "// x := 1" {
		t.Errorf("Line 0 = %q, want %q", got, "// x := 1")
	}
	// Toggle again to confirm round-trip.
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyCtrlUnderscore})
	mm = updated.(model)
	p = mm.bufs.Active()
	if got := p.Line(0); got != "x := 1" {
		t.Errorf("Line 0 after round-trip = %q, want %q", got, "x := 1")
	}
}

// TestCtrlSlashWithoutBufferIsNoop confirms Ctrl+/ without an active
// buffer doesn't panic and doesn't change state. The model only ships a
// status hint when there's a real thing to act on; comment toggle on
// the welcome card silently does nothing, matching alt+enter's shape.
func TestCtrlSlashWithoutBufferIsNoop(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	before := m.status
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlUnderscore})
	mm := updated.(model)
	if cmd != nil {
		t.Errorf("expected nil cmd, got %T", cmd())
	}
	if mm.status != before {
		t.Errorf("status changed for Ctrl+/ with no buffer: %q -> %q", before, mm.status)
	}
}

func stripANSI(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

// --- create-prompt (filetree 'a' → new file/dir) -----------------------

func TestFiletreeCreatePromptMsgOpensOverlaySeededWithParent(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	parentDir := filepath.Join(root, "subdir")
	if err := os.Mkdir(parentDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	updated, _ := m.Update(filetree.CreatePromptMsg{ParentDir: parentDir})
	mm := updated.(model)
	if mm.overlay != overlayCreate {
		t.Fatalf("expected overlayCreate, got %v", mm.overlay)
	}
	if !mm.createPrompt.Open() {
		t.Errorf("expected create prompt to be open")
	}
	if mm.createParentDir != parentDir {
		t.Errorf("expected parentDir %q, got %q", parentDir, mm.createParentDir)
	}
	if mm.createPrompt.ParentRel() != "subdir" {
		t.Errorf("expected parent label %q, got %q", "subdir", mm.createPrompt.ParentRel())
	}
}

func TestCreatePromptParentLabelAtRootIsDot(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	updated, _ := m.Update(filetree.CreatePromptMsg{ParentDir: root})
	mm := updated.(model)
	if mm.createPrompt.ParentRel() != "." {
		t.Errorf("expected '.' at root, got %q", mm.createPrompt.ParentRel())
	}
}

func TestCreatePromptEnterFiresCreatePathCmd(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m.overlay = overlayCreate
	m.createParentDir = root
	m.createPrompt = m.createPrompt.WithParent(".")
	// .txt picks a non-Go file so gopls auto-start doesn't clobber the
	// "created X" status; the gopls-bootstrap path is exercised by the
	// regular open-file tests upstream.
	for _, r := range "hello.txt" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(model)
	if cmd == nil {
		t.Fatalf("Enter should fire a CreatePathCmd")
	}
	if !mm.createPrompt.Open() {
		t.Errorf("prompt should still be open until createPathMsg arrives")
	}
	resultMsg := cmd()
	if _, ok := resultMsg.(createPathMsg); !ok {
		t.Fatalf("expected createPathMsg, got %T", resultMsg)
	}
	updated, _ = mm.Update(resultMsg)
	mm = updated.(model)
	if mm.overlay == overlayCreate {
		t.Errorf("overlay should close on success, still %v", mm.overlay)
	}
	if _, err := os.Stat(filepath.Join(root, "hello.txt")); err != nil {
		t.Errorf("expected hello.txt to be created: %v", err)
	}
	if !strings.Contains(mm.status, "created hello.txt") {
		t.Errorf("status should announce create, got %q", mm.status)
	}
	if mm.bufs.Active() == nil {
		t.Errorf("expected the new file to be opened in a buffer")
	}
}

func TestCreatePromptTrailingSlashCreatesDir(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m.overlay = overlayCreate
	m.createParentDir = root
	m.createPrompt = m.createPrompt.WithParent(".")
	for _, r := range "newdir/" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(model)
	if cmd == nil {
		t.Fatalf("Enter should fire a CreatePathCmd for dir")
	}
	updated, _ = mm.Update(cmd())
	mm = updated.(model)
	info, err := os.Stat(filepath.Join(root, "newdir"))
	if err != nil {
		t.Fatalf("expected newdir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected newdir to be a directory")
	}
	if !strings.Contains(mm.status, "newdir/") {
		t.Errorf("status should announce dir, got %q", mm.status)
	}
	if mm.overlay == overlayCreate {
		t.Errorf("overlay should close on success, still %v", mm.overlay)
	}
}

func TestCreatePromptEscCancelsWithoutSideEffect(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m.overlay = overlayCreate
	m.createParentDir = root
	m.createPrompt = m.createPrompt.WithParent(".")
	for _, r := range "uncommitted.txt" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(model)
	if mm.overlay == overlayCreate {
		t.Errorf("Esc should close the create prompt")
	}
	if _, err := os.Stat(filepath.Join(root, "uncommitted.txt")); err == nil {
		t.Errorf("Esc should NOT have created uncommitted.txt")
	}
	if !strings.Contains(mm.status, "cancelled") {
		t.Errorf("status should mention cancellation, got %q", mm.status)
	}
}

func TestCreatePromptEmptyValueKeepsPromptOpenWithError(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m.overlay = overlayCreate
	m.createParentDir = root
	m.createPrompt = m.createPrompt.WithParent(".")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(model)
	if cmd != nil {
		t.Errorf("empty value should not fire a Cmd")
	}
	if mm.overlay != overlayCreate {
		t.Errorf("prompt should stay open on empty Enter")
	}
}

func TestCreatePromptCollisionKeepsPromptOpenWithError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "exists.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m.overlay = overlayCreate
	m.createParentDir = root
	m.createPrompt = m.createPrompt.WithParent(".")
	for _, r := range "exists.txt" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(model)
	if cmd == nil {
		t.Fatalf("Enter on collision should still fire the cmd")
	}
	updated, _ = mm.Update(cmd())
	mm = updated.(model)
	if mm.overlay != overlayCreate {
		t.Errorf("collision should keep prompt open, overlay=%v", mm.overlay)
	}
	if !mm.createPrompt.Open() {
		t.Errorf("prompt should remain open on collision")
	}
	// Confirm the existing file wasn't overwritten.
	body, _ := os.ReadFile(filepath.Join(root, "exists.txt"))
	if string(body) != "x" {
		t.Errorf("collision should not overwrite existing file; got %q", body)
	}
}

func TestCreatePromptNestedFileMaterializesParents(t *testing.T) {
	root := t.TempDir()
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()
	m.overlay = overlayCreate
	m.createParentDir = root
	m.createPrompt = m.createPrompt.WithParent(".")
	for _, r := range "deep/nest/leaf.go" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(model)
	if cmd == nil {
		t.Fatalf("Enter should fire CreatePathCmd")
	}
	updated, _ = mm.Update(cmd())
	mm = updated.(model)
	if _, err := os.Stat(filepath.Join(root, "deep", "nest")); err != nil {
		t.Errorf("expected parent dirs materialized: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "deep", "nest", "leaf.go")); err != nil {
		t.Errorf("expected leaf file created: %v", err)
	}
	_ = mm
}
