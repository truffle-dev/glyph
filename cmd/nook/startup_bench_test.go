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

// bestNewModelCost times newModel(root)+Init() best-of-N, so scheduler
// and GC outliers are stripped from the construction timing.
func bestNewModelCost(t *testing.T, root string) time.Duration {
	t.Helper()
	best := time.Duration(1 << 62)
	for i := 0; i < 5; i++ {
		t0 := time.Now()
		m := newModel(root)
		cmd := m.Init()
		if cmd == nil {
			t.Fatal("Init returned no cmd")
		}
		if c := time.Since(t0); c < best {
			best = c
		}
	}
	return best
}

// TestStartupNotGatedOnTreeWalk asserts the load-bearing property of the
// async filetree refactor: newModel() + Init() must not walk the project
// tree. Pre-refactor a dotfile-edit launch (root=$HOME) took ~1.1s because
// filetree.New synchronously walked every file in $HOME before returning;
// post-refactor the walk runs in a goroutine emitting BuildTreeMsg later,
// so the first frame renders immediately.
//
// The guard isolates tree-size scaling rather than timing against an
// absolute budget. newModel has a fixed cost of its own (config path
// resolution + load) that is several milliseconds on a slow filesystem and
// has nothing to do with the project tree, so neither an absolute budget
// nor a ratio against a single walk is reliable across machines. Instead
// construct on a large tree and a tiny tree: both pay the same fixed
// baseline, so the difference between them is the part of newModel that
// scales with tree size. Constant-time construction makes that difference
// noise; a reintroduced sync walk makes it approach the walk cost. The
// difference must stay well under a real walk of the large tree.
func TestStartupNotGatedOnTreeWalk(t *testing.T) {
	big := makeStartupFixture(t, 120, 20) // ~2,400 files
	small := makeStartupFixture(t, 1, 2)  // a handful of files

	// Real recursive walk cost of the large tree, the scale a leaked walk
	// would add to construction. Warm the cache so walkCost is steady-state.
	_ = filetree.BuildTree(big)
	tw := time.Now()
	walk := filetree.BuildTree(big)
	walkCost := time.Since(tw)
	if len(walk.Children) < 100 {
		t.Fatalf("fixture too small to time a walk: %d top-level entries", len(walk.Children))
	}

	bigCost := bestNewModelCost(t, big)
	smallCost := bestNewModelCost(t, small)
	scaleWithTree := bigCost - smallCost

	// A leaked walk adds ~walkCost to the large-tree construction; honest
	// constant-time construction leaves only timing noise. Half the walk is
	// a wide margin between the two.
	if scaleWithTree*2 >= walkCost {
		t.Errorf("newModel grew %s going from a tiny tree (%s) to a ~2,400-file tree (%s) "+
			"against a %s walk of that tree; construction scales with tree size, "+
			"so sync FS work leaked into newModel", scaleWithTree, smallCost, bigCost, walkCost)
	}
	t.Logf("newModel tree-size delta %s (small %s, big %s) vs walk %s",
		scaleWithTree, smallCost, bigCost, walkCost)

	// The async walk must still land as Built() once BuildTreeMsg arrives.
	m := newModel(big)
	msg := filetree.BuildTreeMsg{Root: big, Node: filetree.BuildTree(big)}
	updated, _ := m.Update(msg)
	mm := updated.(model)
	if !mm.treePane.Built() {
		t.Error("treePane should be Built() after BuildTreeMsg lands")
	}
}
