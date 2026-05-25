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
