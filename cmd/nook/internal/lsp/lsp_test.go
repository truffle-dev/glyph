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
