package lsp

import (
	"context"
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
