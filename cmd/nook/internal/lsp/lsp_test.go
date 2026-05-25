package lsp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
)

// Reused by the non-blocking send test. A minimal payload that doesn't
// dereference any pointers in the handler path.
var dummyDiagnosticParams = protocol.PublishDiagnosticsParams{}

// TestStartFailsWithMissingBinary verifies the friendly error when the
// language server binary cannot be found on PATH.
func TestStartFailsWithMissingBinary(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := Start(ctx, Options{
		Binary:  "definitely-not-a-real-lsp-binary-xyz",
		RootDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}

// TestStartRejectsMissingRoot makes sure callers don't accidentally pass an
// empty workspace root (the server would otherwise initialize on the wrong
// directory).
func TestStartRejectsMissingRoot(t *testing.T) {
	t.Parallel()
	_, err := Start(context.Background(), Options{Binary: "gopls"})
	if err == nil {
		t.Fatal("expected RootDir error, got nil")
	}
	if !strings.Contains(err.Error(), "RootDir") {
		t.Fatalf("error should mention RootDir, got %q", err.Error())
	}
}

// TestEndToEndDiagnostics drives a real gopls subprocess: open a Go file
// with a known compile error and confirm a publishDiagnostics arrives that
// names the broken identifier. Skipped when gopls is not installed so
// constrained CI runners don't fail.
func TestEndToEndDiagnostics(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not on PATH; skipping end-to-end diagnostics test")
	}

	dir := t.TempDir()
	// Minimal Go module so gopls treats this as a workspace it can analyze.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module lsp_test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Reference an undeclared identifier so gopls publishes a diagnostic.
	path := filepath.Join(dir, "main.go")
	src := "package main\n\nfunc main() {\n\t_ = undefinedSymbol\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cli, err := Start(ctx, Options{RootDir: dir})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		shutCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = cli.Shutdown(shutCtx)
	}()

	if err := cli.Open(ctx, path, "go", src); err != nil {
		t.Fatalf("open: %v", err)
	}

	// gopls publishes diagnostics asynchronously; wait up to 20s for one
	// that names our undeclared symbol.
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case ev := <-cli.Diagnostics():
			for _, d := range ev.Items {
				if strings.Contains(d.Message, "undefinedSymbol") {
					return
				}
			}
		case <-deadline.C:
			t.Fatal("did not receive expected diagnostic for undefinedSymbol within 20s")
		}
	}
}

// TestShutdownIsIdempotent confirms calling Shutdown twice is safe (rapid
// quit paths in the host call it from multiple goroutines).
func TestShutdownIsIdempotent(t *testing.T) {
	t.Parallel()
	c := &Client{}
	if err := c.Shutdown(context.Background()); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}
	if err := c.Shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
}

// TestCompletionHoverDefinitionEndToEnd drives a real gopls subprocess to
// verify the three lookup methods round-trip. Skipped when gopls is not on
// PATH so constrained CI still passes.
func TestCompletionHoverDefinitionEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not on PATH; skipping lookup methods test")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module lsp_test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A package with one exported symbol so:
	//   - Completion after `Add` should surface a function-shaped item.
	//   - Hover on the call-site Add should describe the function.
	//   - Definition on the call-site Add should jump to the declaration.
	src := strings.Join([]string{
		"package main",
		"",
		"// Add returns a+b.",
		"func Add(a, b int) int {",
		"\treturn a + b",
		"}",
		"",
		"func main() {",
		"\t_ = Add(1, 2)",
		"}",
		"",
	}, "\n")
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli, err := Start(ctx, Options{RootDir: dir})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		shutCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = cli.Shutdown(shutCtx)
	}()
	if err := cli.Open(ctx, path, "go", src); err != nil {
		t.Fatalf("open: %v", err)
	}

	// Drain the first publishDiagnostics so gopls has finished initial type-
	// check before we ask for completion/hover/definition. Bound at 10s so we
	// don't hang if gopls publishes nothing (it always does on initial parse).
	select {
	case <-cli.Diagnostics():
	case <-time.After(10 * time.Second):
	}

	// Position cursor right after `Add(` on the call line (row 8 zero-indexed,
	// after the leading tab + `_ = Add`). Use the literal line bytes so the
	// offset stays in sync with the source above.
	callLine := "\t_ = Add(1, 2)"
	callRow := 8 // matches the layout above (0-indexed)
	col := strings.Index(callLine, "Add") + 1

	t.Run("Hover", func(t *testing.T) {
		hov, err := cli.Hover(ctx, path, callRow, col)
		if err != nil {
			t.Fatalf("hover: %v", err)
		}
		if !strings.Contains(hov.Contents, "Add") {
			t.Errorf("hover contents missing Add identifier: %q", hov.Contents)
		}
	})

	t.Run("Definition", func(t *testing.T) {
		locs, err := cli.Definition(ctx, path, callRow, col)
		if err != nil {
			t.Fatalf("definition: %v", err)
		}
		if len(locs) == 0 {
			t.Fatalf("definition: expected at least one location, got 0")
		}
		got := locs[0]
		if filepath.Clean(got.Path) != filepath.Clean(path) {
			t.Errorf("definition path = %q, want %q", got.Path, path)
		}
		if got.Line != 3 { // `func Add(...)` is the 4th line, 0-indexed = 3
			t.Errorf("definition line = %d, want 3", got.Line)
		}
	})

	t.Run("Completion", func(t *testing.T) {
		// Pick a column inside the call so gopls considers the call cursor
		// position; gopls returns parameter/member candidates here.
		items, err := cli.Completion(ctx, path, callRow, col+1)
		if err != nil {
			t.Fatalf("completion: %v", err)
		}
		// gopls always returns something inside an active call expression
		// (function args, dot-completions, etc.). We don't assert a specific
		// label because the result varies across gopls versions, but the call
		// must succeed and the items must each have a non-empty Label.
		for _, it := range items {
			if it.Label == "" {
				t.Errorf("completion item has empty Label: %+v", it)
			}
			if it.InsertText == "" {
				t.Errorf("completion item has empty InsertText: %+v", it)
			}
			if it.Kind == "" {
				t.Errorf("completion item has empty Kind: %+v", it)
			}
		}
	})
}

// TestCompletionKindOfMapping checks every protocol enum maps to a non-empty
// CompletionKind and that the unknown branch returns CompletionKindText.
func TestCompletionKindOfMapping(t *testing.T) {
	t.Parallel()
	// Walk all the integer values gopls might send. We don't need every one
	// to map to a distinct string; just every one to be non-empty.
	for k := protocol.CompletionItemKindText; k <= protocol.CompletionItemKindTypeParameter; k++ {
		if completionKindOf(k) == "" {
			t.Errorf("completionKindOf(%d) returned empty CompletionKind", int(k))
		}
	}
	// Unknown / future-protocol kind falls back to Text.
	if got := completionKindOf(protocol.CompletionItemKind(9999)); got != CompletionKindText {
		t.Errorf("completionKindOf(unknown) = %q, want %q", got, CompletionKindText)
	}
}

// TestLookupMethodsRejectUninitialized confirms calling a lookup before Start
// produces a friendly error instead of panicking on a nil server.
func TestLookupMethodsRejectUninitialized(t *testing.T) {
	t.Parallel()
	c := &Client{}
	if _, err := c.Completion(context.Background(), "x.go", 0, 0); err == nil {
		t.Error("completion on uninitialized client should error")
	}
	if _, err := c.Hover(context.Background(), "x.go", 0, 0); err == nil {
		t.Error("hover on uninitialized client should error")
	}
	if _, err := c.Definition(context.Background(), "x.go", 0, 0); err == nil {
		t.Error("definition on uninitialized client should error")
	}
	if _, err := c.CodeAction(context.Background(), "x.go", 0, 0, 0, 0); err == nil {
		t.Error("codeAction on uninitialized client should error")
	}
	if _, err := c.PrepareRename(context.Background(), "x.go", 0, 0); err == nil {
		t.Error("prepareRename on uninitialized client should error")
	}
	if _, err := c.Rename(context.Background(), "x.go", 0, 0, "Foo"); err == nil {
		t.Error("rename on uninitialized client should error")
	}
	if _, err := c.ResolveCompletion(context.Background(), CompletionItem{Label: "Foo"}); err == nil {
		t.Error("resolveCompletion on uninitialized client should error")
	}
}

// TestApplyEmptyEdits returns the source unchanged when there are no edits.
// gopls's "file is already well-formatted" reply takes this path.
func TestApplyEmptyEdits(t *testing.T) {
	t.Parallel()
	src := "package main\n\nfunc main() {}\n"
	got := Apply(src, nil)
	if got != src {
		t.Errorf("Apply(empty) = %q, want %q", got, src)
	}
	got = Apply(src, []TextEdit{})
	if got != src {
		t.Errorf("Apply([]) = %q, want %q", got, src)
	}
}

// TestApplyWholeFileReplace mirrors gopls's common case: a single TextEdit
// covering the whole file with the formatted contents.
func TestApplyWholeFileReplace(t *testing.T) {
	t.Parallel()
	src := "package main\n\nfunc main(){}\n"
	// gopls returns the full formatted file in one edit spanning [0,0)..
	// (lastLineExclusive, 0).
	edits := []TextEdit{{
		StartLine: 0, StartCol: 0,
		EndLine: 3, EndCol: 0,
		NewText: "package main\n\nfunc main() {}\n",
	}}
	got := Apply(src, edits)
	want := "package main\n\nfunc main() {}\n"
	if got != want {
		t.Errorf("Apply whole-file = %q, want %q", got, want)
	}
}

// TestApplyMultipleEditsDescending verifies edits earlier in the file
// don't drift later ones, which would happen if we applied ascending.
func TestApplyMultipleEditsDescending(t *testing.T) {
	t.Parallel()
	src := "alpha beta gamma"
	edits := []TextEdit{
		// Ascending in source order. Apply must sort descending internally.
		{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 5, NewText: "AAA"},      // alpha → AAA
		{StartLine: 0, StartCol: 6, EndLine: 0, EndCol: 10, NewText: "BBBB"},    // beta → BBBB
		{StartLine: 0, StartCol: 11, EndLine: 0, EndCol: 16, NewText: "GAMMA!"}, // gamma → GAMMA!
	}
	got := Apply(src, edits)
	want := "AAA BBBB GAMMA!"
	if got != want {
		t.Errorf("Apply multi = %q, want %q", got, want)
	}
}

// TestApplyInsertOnly covers an empty-range insert (StartCol == EndCol).
// Inserting at col=6 puts the new text between F (col 5) and ( (col 6).
func TestApplyInsertOnly(t *testing.T) {
	t.Parallel()
	src := "func F() {}\n"
	edits := []TextEdit{{
		StartLine: 0, StartCol: 6, EndLine: 0, EndCol: 6,
		NewText: "ormatted",
	}}
	got := Apply(src, edits)
	want := "func Formatted() {}\n"
	if got != want {
		t.Errorf("Apply insert = %q, want %q", got, want)
	}
}

// TestApplyMultilineEdit covers a range that spans lines (e.g. fixing
// indentation across a block).
func TestApplyMultilineEdit(t *testing.T) {
	t.Parallel()
	src := "func F() {\n\t\tx := 1\n\t\ty := 2\n}\n"
	edits := []TextEdit{{
		StartLine: 1, StartCol: 0,
		EndLine: 3, EndCol: 0,
		NewText: "\tx := 1\n\ty := 2\n",
	}}
	got := Apply(src, edits)
	want := "func F() {\n\tx := 1\n\ty := 2\n}\n"
	if got != want {
		t.Errorf("Apply multiline = %q, want %q", got, want)
	}
}

// TestApplyClampsOutOfRange verifies a server that returns past-EOF
// positions doesn't crash Apply or produce garbage. We clamp to EOF and
// keep going.
func TestApplyClampsOutOfRange(t *testing.T) {
	t.Parallel()
	src := "short\n"
	edits := []TextEdit{{
		StartLine: 999, StartCol: 999,
		EndLine: 9999, EndCol: 9999,
		NewText: "tail",
	}}
	got := Apply(src, edits)
	want := "short\ntail"
	if got != want {
		t.Errorf("Apply clamp = %q, want %q", got, want)
	}
}

// TestFormattingRejectsUninitialized confirms calling Formatting before
// Start returns the friendly error instead of nil-dereferencing.
func TestFormattingRejectsUninitialized(t *testing.T) {
	t.Parallel()
	c := &Client{}
	if _, err := c.Formatting(context.Background(), "x.go", 4, false); err == nil {
		t.Error("formatting on uninitialized client should error")
	}
}

// TestFormattingEndToEnd drives gopls on a deliberately-mis-formatted Go
// source and asserts the returned edits, when applied, produce gofmt
// output. Skipped when gopls isn't on PATH.
func TestFormattingEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not on PATH; skipping format end-to-end test")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module lsp_test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Deliberately mis-formatted: missing space before '{', wrong indent,
	// trailing whitespace. gofmt fixes all three.
	src := "package main\n\nfunc main(){\n\t\tx:=1\n\t_=x   \n}\n"
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cli, err := Start(ctx, Options{RootDir: dir})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		shutCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = cli.Shutdown(shutCtx)
	}()
	if err := cli.Open(ctx, path, "go", src); err != nil {
		t.Fatalf("open: %v", err)
	}
	// Drain the first publishDiagnostics so gopls has finished initial
	// type-check before we ask to format.
	select {
	case <-cli.Diagnostics():
	case <-time.After(10 * time.Second):
	}

	edits, err := cli.Formatting(ctx, path, 4, false)
	if err != nil {
		t.Fatalf("formatting: %v", err)
	}
	if len(edits) == 0 {
		t.Fatal("expected at least one edit from gofmt on misformatted source")
	}
	got := Apply(src, edits)
	want := "package main\n\nfunc main() {\n\tx := 1\n\t_ = x\n}\n"
	if got != want {
		t.Errorf("formatted output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// TestDiagnosticsChannelDoesNotBlockOnFull verifies a slow drainer does not
// stall the handler; overflow events drop rather than block the server pump.
func TestDiagnosticsChannelDoesNotBlockOnFull(t *testing.T) {
	t.Parallel()
	h := &handler{events: make(chan DiagnosticsEvent, 1)}
	h.events <- DiagnosticsEvent{} // fill capacity
	done := make(chan struct{})
	go func() {
		defer close(done)
		// PublishDiagnostics must return immediately even when the channel
		// is full. We send a non-nil params here because the real path
		// dereferences it.
		params := &dummyDiagnosticParams
		_ = h.PublishDiagnostics(context.Background(), params)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler blocked on full channel")
	}
}

// TestWorkspaceEditFromProtocolDocumentChanges asserts the DocumentChanges
// branch maps each file URI to its edits in declaration order. gopls
// returns rename results this way.
func TestWorkspaceEditFromProtocolDocumentChanges(t *testing.T) {
	t.Parallel()
	we := &protocol.WorkspaceEdit{
		DocumentChanges: []protocol.TextDocumentEdit{
			{
				TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: "file:///tmp/a.go"},
				},
				Edits: []protocol.TextEdit{{
					Range:   protocol.Range{Start: protocol.Position{Line: 2, Character: 4}, End: protocol.Position{Line: 2, Character: 9}},
					NewText: "Beta",
				}},
			},
			{
				TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: "file:///tmp/b.go"},
				},
				Edits: []protocol.TextEdit{{
					Range:   protocol.Range{Start: protocol.Position{Line: 0, Character: 0}, End: protocol.Position{Line: 0, Character: 5}},
					NewText: "Beta",
				}},
			},
		},
	}
	got := workspaceEditFromProtocol(we)
	if got.Empty() {
		t.Fatal("expected non-empty change")
	}
	paths := got.Paths()
	if len(paths) != 2 {
		t.Fatalf("paths = %v, want 2 entries", paths)
	}
	for _, p := range paths {
		if len(got.Files[p]) != 1 {
			t.Errorf("file %q has %d edits, want 1", p, len(got.Files[p]))
		}
	}
}

// TestWorkspaceEditFromProtocolChangesFallback covers the `changes` field
// path (older servers, or refactors that don't bother with versioned doc
// identifiers). Each file URI maps to its TextEdit slice.
func TestWorkspaceEditFromProtocolChangesFallback(t *testing.T) {
	t.Parallel()
	we := &protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentURI][]protocol.TextEdit{
			"file:///tmp/only.go": {{
				Range:   protocol.Range{Start: protocol.Position{Line: 0, Character: 0}, End: protocol.Position{Line: 0, Character: 3}},
				NewText: "new",
			}},
		},
	}
	got := workspaceEditFromProtocol(we)
	if got.Empty() {
		t.Fatal("expected non-empty change from Changes map")
	}
	if n := len(got.Files); n != 1 {
		t.Fatalf("files = %d, want 1", n)
	}
}

// TestWorkspaceEditFromProtocolNil returns an empty (non-nil) change so the
// caller can range over Files without nil-checking.
func TestWorkspaceEditFromProtocolNil(t *testing.T) {
	t.Parallel()
	got := workspaceEditFromProtocol(nil)
	if !got.Empty() {
		t.Fatal("nil WorkspaceEdit should produce an empty change")
	}
	if got.Files == nil {
		t.Error("Files map should be non-nil after conversion")
	}
}

// TestApplyWorkspaceEditMultiFile asserts the per-file map is updated and
// untouched files pass through. The fixture mimics a real rename: two files
// reference the same symbol; the change replaces the identifier in both,
// and a third file (not in the edit) is passed through unchanged.
func TestApplyWorkspaceEditMultiFile(t *testing.T) {
	t.Parallel()
	sources := map[string]string{
		"/tmp/a.go": "package x\n\nfunc Alpha() int { return 1 }\n",
		"/tmp/b.go": "package x\n\nfunc Caller() int { return Alpha() }\n",
		"/tmp/c.go": "package x\n\nfunc Unrelated() {}\n",
	}
	edit := WorkspaceEditChange{Files: map[string][]TextEdit{
		"/tmp/a.go": {{StartLine: 2, StartCol: 5, EndLine: 2, EndCol: 10, NewText: "Beta"}},
		"/tmp/b.go": {{StartLine: 2, StartCol: 27, EndLine: 2, EndCol: 32, NewText: "Beta"}},
	}}
	got := ApplyWorkspaceEdit(sources, edit)
	if got["/tmp/a.go"] != "package x\n\nfunc Beta() int { return 1 }\n" {
		t.Errorf("a.go = %q", got["/tmp/a.go"])
	}
	if got["/tmp/b.go"] != "package x\n\nfunc Caller() int { return Beta() }\n" {
		t.Errorf("b.go = %q", got["/tmp/b.go"])
	}
	if got["/tmp/c.go"] != sources["/tmp/c.go"] {
		t.Errorf("c.go was modified despite not appearing in the edit: %q", got["/tmp/c.go"])
	}
}

// TestApplyWorkspaceEditSkipsPathsNotInSources documents the contract: the
// caller is responsible for reading files from disk before applying. Paths
// in edit.Files that aren't in sources are silently skipped so a partial
// apply can complete what the caller did wire up.
func TestApplyWorkspaceEditSkipsPathsNotInSources(t *testing.T) {
	t.Parallel()
	sources := map[string]string{"/tmp/a.go": "hello"}
	edit := WorkspaceEditChange{Files: map[string][]TextEdit{
		"/tmp/missing.go": {{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 1, NewText: "x"}},
	}}
	got := ApplyWorkspaceEdit(sources, edit)
	if got["/tmp/a.go"] != "hello" {
		t.Errorf("a.go should pass through unchanged, got %q", got["/tmp/a.go"])
	}
	if _, ok := got["/tmp/missing.go"]; ok {
		t.Error("missing.go should not appear in output map")
	}
}

// TestWorkspaceEditPathsAndEmpty covers the small accessor helpers used by
// the host's status/dialog rendering paths.
func TestWorkspaceEditPathsAndEmpty(t *testing.T) {
	t.Parallel()
	we := WorkspaceEditChange{}
	if !we.Empty() {
		t.Error("zero-value WorkspaceEditChange should be Empty()")
	}
	if len(we.Paths()) != 0 {
		t.Error("zero-value Paths() should be empty")
	}
	we = WorkspaceEditChange{Files: map[string][]TextEdit{
		"/z.go":    {{NewText: "x"}},
		"/a.go":    {{NewText: "y"}},
		"/empty":   {},
		"/m.go":    {{NewText: "z"}},
		"/nil.txt": nil,
	}}
	if we.Empty() {
		t.Error("WorkspaceEditChange with edits should not be Empty()")
	}
	paths := we.Paths()
	want := []string{"/a.go", "/m.go", "/z.go"}
	if len(paths) != len(want) {
		t.Fatalf("Paths() = %v, want %v", paths, want)
	}
	for i, p := range paths {
		if p != want[i] {
			t.Errorf("Paths()[%d] = %q, want %q (full slice %v)", i, p, want[i], paths)
		}
	}
}

// TestRenameEndToEnd drives gopls through a real prepareRename + rename
// round-trip. Two files reference Alpha(); rename to Beta should rewrite
// both. Skipped when gopls is not on PATH.
func TestRenameEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not on PATH; skipping rename round-trip test")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module renametest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aPath := filepath.Join(dir, "a.go")
	aSrc := "package renametest\n\nfunc Alpha() int { return 1 }\n"
	if err := os.WriteFile(aPath, []byte(aSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	bPath := filepath.Join(dir, "b.go")
	bSrc := "package renametest\n\nfunc Caller() int { return Alpha() }\n"
	if err := os.WriteFile(bPath, []byte(bSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli, err := Start(ctx, Options{RootDir: dir})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		shutCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = cli.Shutdown(shutCtx)
	}()

	if err := cli.Open(ctx, aPath, "go", aSrc); err != nil {
		t.Fatalf("open a: %v", err)
	}
	if err := cli.Open(ctx, bPath, "go", bSrc); err != nil {
		t.Fatalf("open b: %v", err)
	}

	// Drain any initial diagnostics so gopls has settled on the parse.
	drainCtx, dCancel := context.WithTimeout(ctx, 6*time.Second)
	defer dCancel()
drain:
	for {
		select {
		case <-cli.Diagnostics():
		case <-drainCtx.Done():
			break drain
		}
	}

	// "func Alpha" — Alpha starts at column 5 of line 2 in a.go.
	pre, err := cli.PrepareRename(ctx, aPath, 2, 5)
	if err != nil {
		t.Fatalf("prepareRename: %v", err)
	}
	if !pre.Available {
		t.Fatalf("prepareRename on Alpha returned not-available; range = %+v", pre)
	}
	// Modern gopls answers with `{defaultBehavior: true}`, which the
	// go.lsp.dev decoder collapses to a zero Range. The host falls back
	// to walking the source for the identifier in that case, so an
	// all-zero range is also a valid shape here.
	zeroRange := pre.StartLine == 0 && pre.StartCol == 0 && pre.EndLine == 0 && pre.EndCol == 0
	expectedRange := pre.StartLine == 2 && pre.StartCol == 5 && pre.EndLine == 2 && pre.EndCol == 10
	if !zeroRange && !expectedRange {
		t.Errorf("prepareRename range = (%d,%d)-(%d,%d), want (2,5)-(2,10) or zero-range",
			pre.StartLine, pre.StartCol, pre.EndLine, pre.EndCol)
	}

	edit, err := cli.Rename(ctx, aPath, 2, 5, "Beta")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if edit.Empty() {
		t.Fatal("rename returned empty edit; expected both files to change")
	}
	if len(edit.Files) != 2 {
		t.Errorf("edit touched %d files, want 2 (paths=%v)", len(edit.Files), edit.Paths())
	}

	sources := map[string]string{aPath: aSrc, bPath: bSrc}
	updated := ApplyWorkspaceEdit(sources, edit)
	if !strings.Contains(updated[aPath], "func Beta") {
		t.Errorf("a.go did not get renamed:\n%s", updated[aPath])
	}
	if strings.Contains(updated[aPath], "Alpha") {
		t.Errorf("a.go still mentions Alpha:\n%s", updated[aPath])
	}
	if !strings.Contains(updated[bPath], "Beta()") {
		t.Errorf("b.go did not get renamed:\n%s", updated[bPath])
	}
}

// TestPrepareRenameUnavailable confirms gopls rejects an unrenamable cursor
// (a keyword position) with a not-available result rather than an error.
func TestPrepareRenameUnavailable(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not on PATH; skipping prepareRename unavailable test")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module pr\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.go")
	src := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cli, err := Start(ctx, Options{RootDir: dir})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		shutCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = cli.Shutdown(shutCtx)
	}()
	if err := cli.Open(ctx, path, "go", src); err != nil {
		t.Fatalf("open: %v", err)
	}

	// Cursor over the "package" keyword (line 0, col 0).
	pre, err := cli.PrepareRename(ctx, path, 0, 0)
	if err != nil {
		// gopls in some versions returns an error rather than nil for
		// unrenamable positions. Either shape is acceptable as long as it
		// doesn't crash; we treat both as "unavailable" downstream.
		return
	}
	if pre.Available {
		t.Errorf("prepareRename on `package` keyword reported available; got %+v", pre)
	}
}

// TestCodeActionEndToEnd asks gopls for code actions on a Go file with an
// unused import — gopls should propose at least one quickfix action that
// removes it. Skipped when gopls is not on PATH.
func TestCodeActionEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not on PATH; skipping codeAction round-trip test")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module catest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.go")
	// Unused "strings" import — gopls's organize-imports quickfix removes it.
	src := "package main\n\nimport \"strings\"\n\nfunc main() {}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cli, err := Start(ctx, Options{RootDir: dir})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		shutCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = cli.Shutdown(shutCtx)
	}()
	if err := cli.Open(ctx, path, "go", src); err != nil {
		t.Fatalf("open: %v", err)
	}

	// Wait briefly for gopls to settle.
	drainCtx, dCancel := context.WithTimeout(ctx, 6*time.Second)
	defer dCancel()
drain:
	for {
		select {
		case <-cli.Diagnostics():
		case <-drainCtx.Done():
			break drain
		}
	}

	actions, err := cli.CodeAction(ctx, path, 2, 0, 2, 16)
	if err != nil {
		t.Fatalf("codeAction: %v", err)
	}
	if len(actions) == 0 {
		t.Fatal("codeAction returned no actions on an unused-import file")
	}
	// At least one action should produce a non-empty workspace edit.
	hasEdit := false
	for _, a := range actions {
		if !a.Edit.Empty() {
			hasEdit = true
			break
		}
	}
	if !hasEdit {
		t.Errorf("none of the %d returned actions carried an edit", len(actions))
	}
}

// TestDecodeDocumentSymbolsHierarchical exercises the modern wire shape gopls
// emits. A function and a type live at the top level; the type owns one method
// as a child. The decoder must preserve the tree, distill Kind through
// mapSymbolKind, and read positions from SelectionRange (not Range).
func TestDecodeDocumentSymbolsHierarchical(t *testing.T) {
	t.Parallel()
	raw := []byte(`[
	  {
	    "name": "main",
	    "detail": "func()",
	    "kind": 12,
	    "range":          {"start":{"line":3,"character":0}, "end":{"line":5,"character":1}},
	    "selectionRange": {"start":{"line":3,"character":5}, "end":{"line":3,"character":9}}
	  },
	  {
	    "name": "Server",
	    "detail": "struct{...}",
	    "kind": 23,
	    "range":          {"start":{"line":7,"character":0}, "end":{"line":18,"character":1}},
	    "selectionRange": {"start":{"line":7,"character":5}, "end":{"line":7,"character":11}},
	    "children": [
	      {
	        "name": "Listen",
	        "detail": "func(ctx context.Context) error",
	        "kind": 6,
	        "range":          {"start":{"line":12,"character":0}, "end":{"line":16,"character":1}},
	        "selectionRange": {"start":{"line":12,"character":15},"end":{"line":12,"character":21}}
	      }
	    ]
	  }
	]`)
	out := decodeDocumentSymbols(raw)
	if len(out) != 2 {
		t.Fatalf("got %d top-level symbols, want 2", len(out))
	}
	if out[0].Name != "main" || out[0].Kind != WorkspaceSymbolKindFunction {
		t.Errorf("top[0] = %+v, want main/func", out[0])
	}
	if out[0].Line != 3 || out[0].Col != 5 {
		t.Errorf("top[0] pos = (%d,%d), want (3,5) from selectionRange", out[0].Line, out[0].Col)
	}
	if out[0].EndLine != 5 {
		t.Errorf("top[0] endLine = %d, want 5 from range.end", out[0].EndLine)
	}
	if out[1].Name != "Server" || out[1].Kind != WorkspaceSymbolKindStruct {
		t.Errorf("top[1] = %+v, want Server/struct", out[1])
	}
	if len(out[1].Children) != 1 {
		t.Fatalf("Server children = %d, want 1", len(out[1].Children))
	}
	c := out[1].Children[0]
	if c.Name != "Listen" || c.Kind != WorkspaceSymbolKindMethod {
		t.Errorf("Server.Listen = %+v, want Listen/method", c)
	}
	if c.Line != 12 || c.Col != 15 {
		t.Errorf("Listen pos = (%d,%d), want (12,15)", c.Line, c.Col)
	}
}

// TestDecodeDocumentSymbolsFlatFallback verifies the legacy SymbolInformation[]
// shape (no selectionRange field) collapses into a one-level tree using
// ContainerName for parenting.
func TestDecodeDocumentSymbolsFlatFallback(t *testing.T) {
	t.Parallel()
	raw := []byte(`[
	  {
	    "name": "Server",
	    "kind": 23,
	    "location": {"uri": "file:///x.go",
	      "range": {"start":{"line":7,"character":5}, "end":{"line":18,"character":1}}}
	  },
	  {
	    "name": "Listen",
	    "kind": 6,
	    "containerName": "Server",
	    "location": {"uri": "file:///x.go",
	      "range": {"start":{"line":12,"character":15}, "end":{"line":16,"character":1}}}
	  },
	  {
	    "name": "main",
	    "kind": 12,
	    "location": {"uri": "file:///x.go",
	      "range": {"start":{"line":3,"character":5}, "end":{"line":5,"character":1}}}
	  }
	]`)
	out := decodeDocumentSymbols(raw)
	if len(out) != 2 {
		t.Fatalf("got %d roots, want 2 (Server, main)", len(out))
	}
	var server, mainFn *DocSymbol
	for i := range out {
		switch out[i].Name {
		case "Server":
			server = &out[i]
		case "main":
			mainFn = &out[i]
		}
	}
	if server == nil || mainFn == nil {
		t.Fatalf("missing root symbol: server=%v main=%v", server, mainFn)
	}
	if len(server.Children) != 1 || server.Children[0].Name != "Listen" {
		t.Errorf("Server children = %+v, want [Listen]", server.Children)
	}
	if server.Children[0].Kind != WorkspaceSymbolKindMethod {
		t.Errorf("Listen kind = %v, want method", server.Children[0].Kind)
	}
}

// TestDecodeDocumentSymbolsEmpty handles both null and empty-array responses
// the same way: nil out. gopls returns null for files with no symbols (e.g.
// an empty .go file or a non-Go file in the workspace), and empty array for
// some other servers.
func TestDecodeDocumentSymbolsEmpty(t *testing.T) {
	t.Parallel()
	for _, raw := range [][]byte{nil, []byte("null"), []byte("[]")} {
		out := decodeDocumentSymbols(raw)
		if len(out) != 0 {
			t.Errorf("decode(%q) = %d symbols, want 0", string(raw), len(out))
		}
	}
}

// TestDocumentSymbolRejectsUninitialized covers the nil-client guard. The
// outline wedge binds the keystroke unconditionally; the guard lets it
// surface "no language server attached" without panicking.
func TestDocumentSymbolRejectsUninitialized(t *testing.T) {
	t.Parallel()
	c := &Client{}
	if _, err := c.DocumentSymbol(context.Background(), "x.go"); err == nil {
		t.Error("documentSymbol on uninitialized client should error")
	}
}

// TestSignatureHelpRejectsUninitialized covers the nil-client guard. The
// signature help wedge fires on '(' keypress; the guard lets the host call
// it freely while the LSP is still warming up.
func TestSignatureHelpRejectsUninitialized(t *testing.T) {
	t.Parallel()
	c := &Client{}
	if _, err := c.SignatureHelp(context.Background(), "x.go", 0, 0); err == nil {
		t.Error("signatureHelp on uninitialized client should error")
	}
}

// TestDecodeSignatureHelpEmpty handles the "server returned null" path —
// we expect an empty Signatures slice and ActiveSignature == -1 so the
// renderer can treat both "no help" cases identically.
func TestDecodeSignatureHelpEmpty(t *testing.T) {
	t.Parallel()
	got := decodeSignatureHelp(rawSignatureHelp{})
	if got.ActiveSignature != -1 {
		t.Errorf("ActiveSignature: got %d, want -1", got.ActiveSignature)
	}
	if len(got.Signatures) != 0 {
		t.Errorf("Signatures: got %d, want 0", len(got.Signatures))
	}
	if got.Signatures == nil {
		t.Error("Signatures should be non-nil empty slice, got nil")
	}
}

// TestDecodeSignatureHelpStringLabel covers the legacy form where the
// server emits parameter labels as substrings of the parent signature.
// We resolve their rune offsets via strings.Index.
func TestDecodeSignatureHelpStringLabel(t *testing.T) {
	t.Parallel()
	raw := rawSignatureHelp{
		Signatures: []rawSignature{
			{
				Label: "func Hello(name string, age int) error",
				Parameters: []rawSignatureParam{
					{Label: []byte(`"name string"`)},
					{Label: []byte(`"age int"`)},
				},
			},
		},
	}
	got := decodeSignatureHelp(raw)
	if len(got.Signatures) != 1 {
		t.Fatalf("Signatures: got %d, want 1", len(got.Signatures))
	}
	sig := got.Signatures[0]
	if sig.Label != "func Hello(name string, age int) error" {
		t.Errorf("Label mismatch: %q", sig.Label)
	}
	if len(sig.Parameters) != 2 {
		t.Fatalf("Parameters: got %d, want 2", len(sig.Parameters))
	}
	if sig.Parameters[0].Label != "name string" || sig.Parameters[0].Start != 11 || sig.Parameters[0].End != 22 {
		t.Errorf("param0: %+v", sig.Parameters[0])
	}
	if sig.Parameters[1].Label != "age int" || sig.Parameters[1].Start != 24 || sig.Parameters[1].End != 31 {
		t.Errorf("param1: %+v", sig.Parameters[1])
	}
	if got.ActiveSignature != 0 {
		t.Errorf("ActiveSignature: got %d, want 0 (default)", got.ActiveSignature)
	}
}

// TestDecodeSignatureHelpOffsetLabel covers the modern form where the
// server emits [start, end] pairs after the client declares
// LabelOffsetSupport. rust-analyzer and typescript-language-server use
// this shape.
func TestDecodeSignatureHelpOffsetLabel(t *testing.T) {
	t.Parallel()
	raw := rawSignatureHelp{
		Signatures: []rawSignature{
			{
				Label: "fn add(a: i32, b: i32) -> i32",
				Parameters: []rawSignatureParam{
					{Label: []byte(`[7, 13]`)},  // "a: i32"
					{Label: []byte(`[15, 21]`)}, // "b: i32"
				},
			},
		},
		ActiveSignature: intPtr(0),
		ActiveParameter: intPtr(1),
	}
	got := decodeSignatureHelp(raw)
	if got.ActiveSignature != 0 {
		t.Errorf("ActiveSignature: %d", got.ActiveSignature)
	}
	if got.Signatures[0].ActiveParameter != 1 {
		t.Errorf("ActiveParameter: got %d, want 1", got.Signatures[0].ActiveParameter)
	}
	p0 := got.Signatures[0].Parameters[0]
	if p0.Label != "a: i32" || p0.Start != 7 || p0.End != 13 {
		t.Errorf("param0: %+v", p0)
	}
	p1 := got.Signatures[0].Parameters[1]
	if p1.Label != "b: i32" || p1.Start != 15 || p1.End != 21 {
		t.Errorf("param1: %+v", p1)
	}
}

// TestDecodeSignatureHelpDocAsMarkup covers the MarkupContent doc shape
// servers may emit (`{"kind": "markdown", "value": "..."}`).
func TestDecodeSignatureHelpDocAsMarkup(t *testing.T) {
	t.Parallel()
	raw := rawSignatureHelp{
		Signatures: []rawSignature{
			{
				Label:         "func F()",
				Documentation: []byte(`{"kind":"markdown","value":"Hello docs"}`),
			},
		},
	}
	got := decodeSignatureHelp(raw)
	if got.Signatures[0].Doc != "Hello docs" {
		t.Errorf("Doc: %q", got.Signatures[0].Doc)
	}
}

// TestDecodeSignatureHelpDocAsString covers the legacy string doc shape.
func TestDecodeSignatureHelpDocAsString(t *testing.T) {
	t.Parallel()
	raw := rawSignatureHelp{
		Signatures: []rawSignature{
			{
				Label:         "func F()",
				Documentation: []byte(`"plain text doc"`),
			},
		},
	}
	got := decodeSignatureHelp(raw)
	if got.Signatures[0].Doc != "plain text doc" {
		t.Errorf("Doc: %q", got.Signatures[0].Doc)
	}
}

// TestDecodeSignatureHelpActiveParameterPerSignature covers per-signature
// activeParameter (each signature carries its own active index when no
// top-level value is set).
func TestDecodeSignatureHelpActiveParameterPerSignature(t *testing.T) {
	t.Parallel()
	raw := rawSignatureHelp{
		Signatures: []rawSignature{
			{Label: "func F(a, b int)", ActiveParameter: intPtr(1)},
		},
	}
	got := decodeSignatureHelp(raw)
	if got.Signatures[0].ActiveParameter != 1 {
		t.Errorf("per-sig ActiveParameter: %d", got.Signatures[0].ActiveParameter)
	}
}

// TestDecodeSignatureHelpUnresolvedSubstring covers the corner case where
// the parameter label is a string that does not appear in the parent
// label. We keep the label text but emit zero offsets so the renderer
// shows the raw text without an inverted highlight band.
func TestDecodeSignatureHelpUnresolvedSubstring(t *testing.T) {
	t.Parallel()
	raw := rawSignatureHelp{
		Signatures: []rawSignature{
			{
				Label: "func F()",
				Parameters: []rawSignatureParam{
					{Label: []byte(`"missing"`)},
				},
			},
		},
	}
	got := decodeSignatureHelp(raw)
	p := got.Signatures[0].Parameters[0]
	if p.Label != "missing" {
		t.Errorf("Label: %q", p.Label)
	}
	if p.Start != 0 || p.End != 0 {
		t.Errorf("offsets should be zero when substring not found: start=%d end=%d", p.Start, p.End)
	}
}

// TestDecodeSignatureHelpOffsetClampedToLabelEnd guards against a
// malformed server response where the end offset overshoots the rune
// length of the parent label.
func TestDecodeSignatureHelpOffsetClampedToLabelEnd(t *testing.T) {
	t.Parallel()
	raw := rawSignatureHelp{
		Signatures: []rawSignature{
			{
				Label: "func F()",
				Parameters: []rawSignatureParam{
					{Label: []byte(`[2, 99]`)},
				},
			},
		},
	}
	got := decodeSignatureHelp(raw)
	p := got.Signatures[0].Parameters[0]
	if p.End != 8 { // len("func F()")
		t.Errorf("End should clamp to label length: got %d", p.End)
	}
}

// TestDecodeSignatureHelpUtf8Identifier covers UTF-8 identifiers where
// rune indexing diverges from byte indexing. The string-substring path
// must convert byte offsets to rune offsets so renderers that count
// runes line up the highlight correctly.
func TestDecodeSignatureHelpUtf8Identifier(t *testing.T) {
	t.Parallel()
	raw := rawSignatureHelp{
		Signatures: []rawSignature{
			{
				Label: "fn (πfile, βvalue)",
				Parameters: []rawSignatureParam{
					{Label: []byte(`"βvalue"`)},
				},
			},
		},
	}
	got := decodeSignatureHelp(raw)
	p := got.Signatures[0].Parameters[0]
	if p.Label != "βvalue" {
		t.Errorf("Label: %q", p.Label)
	}
	if p.Start != 11 || p.End != 17 {
		t.Errorf("rune offsets: start=%d end=%d, want 11..17", p.Start, p.End)
	}
}

func intPtr(v int) *int { return &v }

// TestDecodeResolvedCompletionMergesDocumentation: the server responded
// with a fully-populated item; documentation now lands on the merged
// result even though the original was empty.
func TestDecodeResolvedCompletionMergesDocumentation(t *testing.T) {
	t.Parallel()
	orig := CompletionItem{
		Label:      "Println",
		InsertText: "Println",
		Detail:     "func(a ...any) (n int, err error)",
		Kind:       CompletionKindFunction,
	}
	raw := json.RawMessage(`{
		"label":"Println",
		"documentation":{"kind":"markdown","value":"Println formats using the default formats for its operands and writes to standard output."},
		"detail":"func(a ...any) (n int, err error)"
	}`)
	got := decodeResolvedCompletion(raw, orig)
	if got.Documentation == "" {
		t.Fatalf("documentation not merged in: %+v", got)
	}
	if !strings.Contains(got.Documentation, "Println formats") {
		t.Errorf("documentation text wrong: %q", got.Documentation)
	}
	if got.Label != "Println" {
		t.Errorf("label clobbered: %q", got.Label)
	}
	if got.Detail != "func(a ...any) (n int, err error)" {
		t.Errorf("detail clobbered: %q", got.Detail)
	}
	if got.Kind != CompletionKindFunction {
		t.Errorf("kind clobbered: %v", got.Kind)
	}
}

// TestDecodeResolvedCompletionPreservesOriginal: the server returned a
// minimal response (no fields beyond label echo) — merged result should
// keep the original's detail/kind/insertText untouched.
func TestDecodeResolvedCompletionPreservesOriginal(t *testing.T) {
	t.Parallel()
	orig := CompletionItem{
		Label:         "Foo",
		InsertText:    "Foo()",
		Detail:        "func() int",
		Documentation: "old docs",
		Kind:          CompletionKindFunction,
		Data:          json.RawMessage(`{"token":42}`),
	}
	raw := json.RawMessage(`{"label":"Foo"}`)
	got := decodeResolvedCompletion(raw, orig)
	if got.InsertText != "Foo()" {
		t.Errorf("insertText: %q", got.InsertText)
	}
	if got.Detail != "func() int" {
		t.Errorf("detail: %q", got.Detail)
	}
	if got.Documentation != "old docs" {
		t.Errorf("documentation lost: %q", got.Documentation)
	}
	if got.Kind != CompletionKindFunction {
		t.Errorf("kind lost: %v", got.Kind)
	}
	if string(got.Data) != `{"token":42}` {
		t.Errorf("data lost: %s", string(got.Data))
	}
}

// TestDecodeResolvedCompletionEmptyRaw: server returned a null response
// (some implementations do this for items they can't resolve). The
// original item must round-trip unchanged.
func TestDecodeResolvedCompletionEmptyRaw(t *testing.T) {
	t.Parallel()
	orig := CompletionItem{Label: "X", InsertText: "X"}
	got := decodeResolvedCompletion(nil, orig)
	if got.Label != "X" || got.InsertText != "X" {
		t.Errorf("empty raw should return original verbatim: %+v", got)
	}
}

// TestDecodeResolvedCompletionStringDoc: the server returned a plain
// string for documentation (older spec shape). Decoded text should match.
func TestDecodeResolvedCompletionStringDoc(t *testing.T) {
	t.Parallel()
	orig := CompletionItem{Label: "Hello"}
	raw := json.RawMessage(`{"label":"Hello","documentation":"Greets the user."}`)
	got := decodeResolvedCompletion(raw, orig)
	if got.Documentation != "Greets the user." {
		t.Errorf("string-form doc: %q", got.Documentation)
	}
}

// TestDecodeResolvedCompletionUpdatesKindAndData: when the server
// includes a new Kind value AND a new opaque Data token, both should be
// preserved on the merged item so a follow-up resolve call (if a server
// allows chained resolves) can use them.
func TestDecodeResolvedCompletionUpdatesKindAndData(t *testing.T) {
	t.Parallel()
	orig := CompletionItem{Label: "X", Kind: CompletionKindText, Data: json.RawMessage(`{"a":1}`)}
	raw := json.RawMessage(`{"label":"X","kind":3,"data":{"a":2}}`)
	got := decodeResolvedCompletion(raw, orig)
	if got.Kind != CompletionKindFunction {
		t.Errorf("kind should update to function (3): %v", got.Kind)
	}
	if string(got.Data) != `{"a":2}` {
		t.Errorf("data should update to new token: %s", string(got.Data))
	}
}

// TestMarshalAndDecodeDocStringForm covers the interface{} path through
// when the typed protocol package gives us a plain Go string for the
// Documentation field. The marshal/re-decode round-trip should land
// the same string.
func TestMarshalAndDecodeDocStringForm(t *testing.T) {
	t.Parallel()
	got := marshalAndDecodeDoc("plain prose")
	if got != "plain prose" {
		t.Errorf("string round-trip: %q", got)
	}
}

// TestMarshalAndDecodeDocMarkupForm covers the same path for a
// MarkupContent shape passed through map[string]interface{} (the
// concrete type the protocol package surfaces).
func TestMarshalAndDecodeDocMarkupForm(t *testing.T) {
	t.Parallel()
	got := marshalAndDecodeDoc(map[string]interface{}{
		"kind":  "markdown",
		"value": "**bold** docs",
	})
	if got != "**bold** docs" {
		t.Errorf("markup round-trip: %q", got)
	}
}

// TestMarshalAndDecodeDocNil returns the empty string for a nil
// interface (the common case when servers omit documentation in the
// initial completion list and only attach it on resolve).
func TestMarshalAndDecodeDocNil(t *testing.T) {
	t.Parallel()
	if got := marshalAndDecodeDoc(nil); got != "" {
		t.Errorf("nil doc should be empty: %q", got)
	}
}

// TestMarshalRawPreservesShape verifies that opaque server tokens
// survive a round trip through the marshalRaw helper.
func TestMarshalRawPreservesShape(t *testing.T) {
	t.Parallel()
	got := marshalRaw(map[string]interface{}{
		"resolveToken": "abc123",
		"version":      2,
	})
	if len(got) == 0 {
		t.Fatalf("nil result for non-nil input")
	}
	var roundtrip map[string]interface{}
	if err := json.Unmarshal(got, &roundtrip); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}
	if roundtrip["resolveToken"] != "abc123" {
		t.Errorf("token lost: %v", roundtrip["resolveToken"])
	}
	if v, _ := roundtrip["version"].(float64); v != 2 {
		t.Errorf("version lost: %v", roundtrip["version"])
	}
}

// TestMarshalRawNilInterface returns the nil RawMessage rather than
// the JSON literal "null" so omitempty on the resolve request skips
// the field entirely.
func TestMarshalRawNilInterface(t *testing.T) {
	t.Parallel()
	if got := marshalRaw(nil); got != nil {
		t.Errorf("nil input should produce nil RawMessage, got %q", string(got))
	}
}

// TestCompletionKindToProtocolRoundTrip exhaustively walks each nook
// kind through completionKindToProtocol -> completionKindOf and asserts
// the round trip is lossless for every named kind (the implicit
// CompletionKindText falls outside this set since it's our fallback
// bucket, not a wire-level enum).
func TestCompletionKindToProtocolRoundTrip(t *testing.T) {
	t.Parallel()
	kinds := []CompletionKind{
		CompletionKindMethod, CompletionKindFunction, CompletionKindConstructor,
		CompletionKindField, CompletionKindVariable, CompletionKindClass,
		CompletionKindInterface, CompletionKindModule, CompletionKindProperty,
		CompletionKindUnit, CompletionKindValue, CompletionKindEnum,
		CompletionKindKeyword, CompletionKindSnippet, CompletionKindColor,
		CompletionKindFile, CompletionKindReference, CompletionKindFolder,
		CompletionKindEnumMember, CompletionKindConstant, CompletionKindStruct,
		CompletionKindEvent, CompletionKindOperator, CompletionKindTypeParameter,
	}
	for _, k := range kinds {
		p := completionKindToProtocol(k)
		if p == 0 {
			t.Errorf("kind %q lost in to-protocol mapping", k)
			continue
		}
		if back := completionKindOf(p); back != k {
			t.Errorf("round trip: %q -> %v -> %q", k, p, back)
		}
	}
	if got := completionKindToProtocol(CompletionKindText); got != 0 {
		t.Errorf("CompletionKindText should map to 0 (omitempty), got %v", got)
	}
}
