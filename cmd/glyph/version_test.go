package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestVersionString(t *testing.T) {
	got := versionString()
	if !strings.HasPrefix(got, "glyph ") {
		t.Fatalf("expected versionString to start with %q, got %q", "glyph ", got)
	}
	if !strings.Contains(got, version) {
		t.Errorf("expected versionString to contain version %q, got %q", version, got)
	}
	if !strings.Contains(got, runtime.Version()) {
		t.Errorf("expected versionString to contain Go version %q, got %q", runtime.Version(), got)
	}
}
