package main

import (
	"os"
	"testing"
	"time"

	"github.com/truffle-dev/glyph/cmd/nook/internal/filetree"
)

// TestStartupNotGatedOnTreeWalk asserts the load-bearing property of
// the async filetree refactor: newModel() + Init() must complete in
// well under a recursive home-directory walk. Pre-refactor this took
// ~1.1s on `nook ~/.zshrc` because filetree.New synchronously walked
// every file in $HOME before returning. Post-refactor the walk runs
// in a goroutine emitting BuildTreeMsg later, so the first frame
// renders immediately.
//
// Budget: 200ms. The dotfile-edit launch (root=$HOME) was the worst
// case, ~1s of sync I/O. Anything under 200ms is "lightning"; this
// test catches regressions where someone adds sync FS work back into
// newModel.
func TestStartupNotGatedOnTreeWalk(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no $HOME on this machine")
	}
	if _, err := os.Stat(home); err != nil {
		t.Skip("$HOME does not exist")
	}

	t0 := time.Now()
	m := newModel(home)
	cmd := m.Init()
	dt := time.Since(t0)

	if cmd == nil {
		t.Fatal("Init returned no cmd")
	}
	if dt > 200*time.Millisecond {
		t.Errorf("newModel(%q) + Init() took %s; want <200ms (file-tree walk leaked back into the sync path?)", home, dt)
	}
	t.Logf("newModel + Init on $HOME: %s", dt)

	// Round-trip the BuildTreeMsg through the host. The walk happens
	// inside cmd(); we don't time the walk here, only that it doesn't
	// block first frame. The host's case-handler matches root and
	// calls SetNode, after which Built() should be true.
	// (We can't easily invoke just the BuildTreeCmd from outside the
	// tea.Batch result, so we instead synthesize a BuildTreeMsg from
	// the public helpers and feed it through Update.)
	msg := filetree.BuildTreeMsg{Root: home, Node: filetree.BuildTree(home)}
	updated, _ := m.Update(msg)
	mm := updated.(model)
	if !mm.treePane.Built() {
		t.Error("treePane should be Built() after BuildTreeMsg lands")
	}
}
