package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/truffle-dev/glyph/cmd/nook/internal/filetree"
)

// makeStartupFixture lays out nDirs directories (each with a "sub" child)
// holding nFiles files apiece, so a recursive walk of the tree has a cost
// that clearly dwarfs constant-time host construction. The startup guard
// times newModel against an actual walk of this tree rather than against
// whatever happens to live in $HOME, so the guard's sensitivity does not
// depend on the test machine's home directory.
func makeStartupFixture(t *testing.T, nDirs, nFiles int) string {
	t.Helper()
	root := t.TempDir()
	for d := 0; d < nDirs; d++ {
		dir := filepath.Join(root, fmt.Sprintf("pkg%03d", d), "sub")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		for f := 0; f < nFiles; f++ {
			p := filepath.Join(dir, fmt.Sprintf("f%03d.go", f))
			if err := os.WriteFile(p, []byte("package x\n"), 0o644); err != nil {
				t.Fatalf("write %s: %v", p, err)
			}
		}
	}
	return root
}

// TestStartupNotGatedOnTreeWalk asserts the load-bearing property of the
// async filetree refactor: newModel() + Init() must complete in well under
// a recursive walk of the same tree. Pre-refactor a dotfile-edit launch
// (root=$HOME) took ~1.1s because filetree.New synchronously walked every
// file in $HOME before returning; post-refactor the walk runs in a
// goroutine emitting BuildTreeMsg later, so the first frame renders
// immediately.
//
// The guard is relative, not an absolute millisecond budget. An absolute
// budget against $HOME silently weakens to a no-op on a host with a small
// home directory: a reintroduced sync walk of a near-empty $HOME finishes
// under the budget and the regression passes. Here the same controlled
// tree is walked to obtain a real baseline, and construction must be at
// least 10x faster than that walk — a property that holds with wide margin
// for constant-time construction and collapses the moment sync FS work
// leaks back into the startup path.
func TestStartupNotGatedOnTreeWalk(t *testing.T) {
	root := makeStartupFixture(t, 120, 20) // ~2,400 files

	// Real recursive walk cost of the same tree, the baseline the startup
	// path must beat. Warm the cache first so walkCost is steady-state.
	_ = filetree.BuildTree(root)
	tw := time.Now()
	walk := filetree.BuildTree(root)
	walkCost := time.Since(tw)
	if len(walk.Children) < 100 {
		t.Fatalf("fixture too small to time a walk: %d top-level entries", len(walk.Children))
	}

	// Best-of-N strips scheduler/GC outliers from the construction timing.
	var startCost time.Duration = 1 << 62
	for i := 0; i < 5; i++ {
		t0 := time.Now()
		m := newModel(root)
		cmd := m.Init()
		if cmd == nil {
			t.Fatal("Init returned no cmd")
		}
		if c := time.Since(t0); c < startCost {
			startCost = c
		}
	}
	if startCost*10 >= walkCost {
		t.Errorf("newModel+Init took %s vs a %s BuildTree walk on the same tree; "+
			"startup is not <<1/10th the walk, so sync FS work leaked into newModel",
			startCost, walkCost)
	}
	t.Logf("newModel+Init %s vs walk %s", startCost, walkCost)

	// The async walk must still land as Built() once BuildTreeMsg arrives.
	m := newModel(root)
	msg := filetree.BuildTreeMsg{Root: root, Node: filetree.BuildTree(root)}
	updated, _ := m.Update(msg)
	mm := updated.(model)
	if !mm.treePane.Built() {
		t.Error("treePane should be Built() after BuildTreeMsg lands")
	}
}
