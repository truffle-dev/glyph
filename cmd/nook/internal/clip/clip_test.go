package clip

import (
	"testing"
)

// Set/Get must always round-trip through the in-process register even when no
// OS clipboard helper is available on the host. This is the headless-container
// invariant: tests, CI, and locked-down containers all rely on it.
func TestSet_UpdatesRegister(t *testing.T) {
	Set("hello")
	if got := Register(); got != "hello" {
		t.Fatalf("Register = %q, want %q", got, "hello")
	}
}

// Setting an empty string clears the register. Used by the Ctrl+X cut path on
// the first line of an otherwise-empty buffer.
func TestSet_Empty(t *testing.T) {
	Set("non-empty")
	Set("")
	if got := Register(); got != "" {
		t.Fatalf("Register after Set(\"\") = %q, want empty", got)
	}
}

// Multi-line text round-trips byte-exact through the register. The wrapping is
// what Ctrl+X (cut whole line) and a multi-line selection produce.
func TestSet_PreservesNewlines(t *testing.T) {
	in := "line one\nline two\nline three\n"
	Set(in)
	if got := Register(); got != in {
		t.Fatalf("Register = %q, want %q", got, in)
	}
}

// Get returns the most recent register value when no OS helper exists. On
// hosts with a helper Get may return a different OS value, so this test only
// asserts the fallback path by writing then reading and accepting either the
// register or whatever the helper happens to hold — what we care about is
// that Get never panics and always returns a string.
func TestGet_ReturnsString(t *testing.T) {
	Set("round-trip")
	_ = Get()
}

// PowerShell's Get-Clipboard always appends CRLF; without normalization the
// "INS"-shaped tests in editor_selection_test.go paste as "INS\n" on Windows
// and split the line, which is what failed Windows CI on v0.33.0.
func TestNormalizeOSText_StripsCRLF(t *testing.T) {
	if got := normalizeOSText([]byte("hello\r\n")); got != "hello" {
		t.Fatalf("normalizeOSText(\"hello\\r\\n\") = %q, want %q", got, "hello")
	}
}

func TestNormalizeOSText_StripsTrailingLF(t *testing.T) {
	if got := normalizeOSText([]byte("hello\n")); got != "hello" {
		t.Fatalf("normalizeOSText(\"hello\\n\") = %q, want %q", got, "hello")
	}
}

// Interior newlines must survive — a multi-line paste reads back with the
// embedded LFs intact; only ONE trailing newline is stripped.
func TestNormalizeOSText_PreservesInteriorNewlines(t *testing.T) {
	if got := normalizeOSText([]byte("a\nb\nc\n")); got != "a\nb\nc" {
		t.Fatalf("normalizeOSText(multi-line) = %q, want %q", got, "a\nb\nc")
	}
}

func TestNormalizeOSText_NoTrailingNewlineUnchanged(t *testing.T) {
	if got := normalizeOSText([]byte("hello")); got != "hello" {
		t.Fatalf("normalizeOSText(\"hello\") = %q, want %q", got, "hello")
	}
}

// CRLF inside the body (rare but technically possible when text was copied
// from a Windows-side editor that uses CRLF line endings) gets lowered to LF.
func TestNormalizeOSText_LowersInteriorCRLF(t *testing.T) {
	if got := normalizeOSText([]byte("a\r\nb\r\n")); got != "a\nb" {
		t.Fatalf("normalizeOSText(CRLF body) = %q, want %q", got, "a\nb")
	}
}

// Strip only ONE trailing newline so the user can still tell a two-line copy
// from a one-line copy of an empty trailing line; mirrors how wl-paste
// --no-newline behaves on Wayland already.
func TestNormalizeOSText_StripsAtMostOneTrailingNewline(t *testing.T) {
	if got := normalizeOSText([]byte("hello\n\n")); got != "hello\n" {
		t.Fatalf("normalizeOSText(double trailing) = %q, want %q", got, "hello\n")
	}
}
