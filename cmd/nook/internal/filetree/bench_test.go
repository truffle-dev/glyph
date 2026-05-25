package filetree

import (
	"fmt"
	"testing"
	"time"
)

// TestBenchBuildTree is a one-shot measurement, not a regression gate.
// Run with: go test ./cmd/nook/internal/filetree/ -run TestBenchBuildTree -v -count=1.
func TestBenchBuildTree(t *testing.T) {
	if testing.Short() {
		t.Skip("manual benchmark; -short skips")
	}
	roots := []string{
		"/app/projects/glyph/repo",
		"/home/phantom",
		"/home/phantom/repos",
	}
	for _, r := range roots {
		t0 := time.Now()
		node := BuildTree(r)
		dt := time.Since(t0)
		fmt.Printf("[bench] BuildTree(%s) = %s, %d top-level children\n", r, dt, len(node.Children))
	}
}
