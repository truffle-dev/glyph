package navhistory

import "testing"

func TestZeroValueEmpty(t *testing.T) {
	var h History
	if h.Len() != 0 {
		t.Fatalf("zero-value Len = %d, want 0", h.Len())
	}
	if _, ok := h.Back(); ok {
		t.Fatalf("Back on empty returned ok=true")
	}
	if _, ok := h.Forward(); ok {
		t.Fatalf("Forward on empty returned ok=true")
	}
	if _, ok := h.Current(); ok {
		t.Fatalf("Current on empty returned ok=true")
	}
	if h.CanBack() || h.CanForward() {
		t.Fatalf("CanBack/CanForward on empty both true")
	}
}

func TestPushIgnoresEmptyPath(t *testing.T) {
	h := New(10)
	h.Push(Entry{Path: "", Row: 0, Col: 0})
	if h.Len() != 0 {
		t.Fatalf("Push with empty path created entry; Len=%d", h.Len())
	}
}

func TestPushSingleAndBack(t *testing.T) {
	h := New(10)
	e := Entry{Path: "/a.go", Row: 5, Col: 2}
	h.Push(e)
	if h.Len() != 1 {
		t.Fatalf("Len after single push = %d, want 1", h.Len())
	}
	got, ok := h.Back()
	if !ok {
		t.Fatalf("Back returned ok=false")
	}
	if !got.Equal(e) {
		t.Fatalf("Back returned %+v, want %+v", got, e)
	}
	if _, ok := h.Back(); ok {
		t.Fatalf("second Back should fail; got ok=true")
	}
}

func TestPushTwiceBackTwice(t *testing.T) {
	h := New(10)
	a := Entry{Path: "/a.go", Row: 1, Col: 0}
	b := Entry{Path: "/b.go", Row: 2, Col: 0}
	h.Push(a)
	h.Push(b)

	got, ok := h.Back()
	if !ok || !got.Equal(b) {
		t.Fatalf("first Back = %+v ok=%v, want %+v ok=true", got, ok, b)
	}
	got, ok = h.Back()
	if !ok || !got.Equal(a) {
		t.Fatalf("second Back = %+v ok=%v, want %+v ok=true", got, ok, a)
	}
	if _, ok := h.Back(); ok {
		t.Fatalf("third Back should fail")
	}
}

func TestForwardAfterBack(t *testing.T) {
	h := New(10)
	a := Entry{Path: "/a.go", Row: 1}
	b := Entry{Path: "/b.go", Row: 2}
	c := Entry{Path: "/c.go", Row: 3}
	h.Push(a)
	h.Push(b)
	h.Push(c)

	if _, ok := h.Back(); !ok {
		t.Fatal("Back 1 failed")
	}
	if _, ok := h.Back(); !ok {
		t.Fatal("Back 2 failed")
	}
	got, ok := h.Forward()
	if !ok || !got.Equal(c) {
		t.Fatalf("Forward after 2 Backs = %+v ok=%v, want %+v", got, ok, c)
	}
	if _, ok := h.Forward(); ok {
		t.Fatalf("second Forward should fail (already at newest)")
	}
}

func TestPushTruncatesForwardTail(t *testing.T) {
	h := New(10)
	a := Entry{Path: "/a.go", Row: 1}
	b := Entry{Path: "/b.go", Row: 2}
	c := Entry{Path: "/c.go", Row: 3}
	d := Entry{Path: "/d.go", Row: 4}
	h.Push(a)
	h.Push(b)
	h.Push(c)

	if _, ok := h.Back(); !ok {
		t.Fatal("Back 1 failed")
	}
	if _, ok := h.Back(); !ok {
		t.Fatal("Back 2 failed")
	}
	// idx is now at 'a' (position 1 of 3).
	h.Push(d)
	if h.Len() != 2 {
		t.Fatalf("Len after truncating push = %d, want 2 (a + d)", h.Len())
	}
	snap := h.Snapshot()
	if !snap[0].Equal(a) || !snap[1].Equal(d) {
		t.Fatalf("Snapshot after truncate = %+v, want [a, d]", snap)
	}
	if _, ok := h.Forward(); ok {
		t.Fatalf("Forward should fail after truncating push")
	}
}

func TestPushSkipsDuplicateAtTail(t *testing.T) {
	h := New(10)
	a := Entry{Path: "/a.go", Row: 1}
	h.Push(a)
	h.Push(a)
	if h.Len() != 1 {
		t.Fatalf("duplicate Push collapsed wrong; Len = %d, want 1", h.Len())
	}
}

func TestPushSkipsDuplicateAtCurrentInsideForwardStack(t *testing.T) {
	h := New(10)
	a := Entry{Path: "/a.go", Row: 1}
	b := Entry{Path: "/b.go", Row: 2}
	h.Push(a)
	h.Push(b)
	if _, ok := h.Back(); !ok {
		t.Fatal("Back failed")
	}
	// idx now points at 'a' (idx=0, entries len 2).
	h.Push(a)
	// Should advance idx without truncating.
	if h.Len() != 2 {
		t.Fatalf("Len after duplicate inside forward stack = %d, want 2", h.Len())
	}
	pos, total := h.Position()
	if pos != 2 || total != 2 {
		t.Fatalf("Position after dup-push = (%d,%d), want (2,2)", pos, total)
	}
}

func TestEvictsOldestOverCap(t *testing.T) {
	h := New(3)
	a := Entry{Path: "/a.go", Row: 1}
	b := Entry{Path: "/b.go", Row: 2}
	c := Entry{Path: "/c.go", Row: 3}
	d := Entry{Path: "/d.go", Row: 4}
	h.Push(a)
	h.Push(b)
	h.Push(c)
	h.Push(d)
	if h.Len() != 3 {
		t.Fatalf("Len with cap=3 after 4 pushes = %d, want 3", h.Len())
	}
	snap := h.Snapshot()
	if !snap[0].Equal(b) || !snap[1].Equal(c) || !snap[2].Equal(d) {
		t.Fatalf("oldest not evicted; got %+v, want [b,c,d]", snap)
	}
}

func TestPosition(t *testing.T) {
	h := New(10)
	if pos, total := h.Position(); pos != 0 || total != 0 {
		t.Fatalf("empty Position = (%d,%d), want (0,0)", pos, total)
	}
	h.Push(Entry{Path: "/a.go"})
	h.Push(Entry{Path: "/b.go"})
	h.Push(Entry{Path: "/c.go"})
	if pos, total := h.Position(); pos != 3 || total != 3 {
		t.Fatalf("Position after 3 pushes = (%d,%d), want (3,3)", pos, total)
	}
	h.Back()
	if pos, total := h.Position(); pos != 3 || total != 3 {
		t.Fatalf("Position after 1 Back from tail = (%d,%d), want (3,3)", pos, total)
	}
	h.Back()
	if pos, total := h.Position(); pos != 2 || total != 3 {
		t.Fatalf("Position after 2 Backs = (%d,%d), want (2,3)", pos, total)
	}
	h.Back()
	if pos, total := h.Position(); pos != 1 || total != 3 {
		t.Fatalf("Position after 3 Backs = (%d,%d), want (1,3)", pos, total)
	}
}

func TestCurrent(t *testing.T) {
	h := New(10)
	if _, ok := h.Current(); ok {
		t.Fatal("Current on empty returned ok=true")
	}
	a := Entry{Path: "/a.go", Row: 1}
	b := Entry{Path: "/b.go", Row: 2}
	h.Push(a)
	h.Push(b)
	got, ok := h.Current()
	if !ok || !got.Equal(b) {
		t.Fatalf("Current after 2 pushes = %+v ok=%v, want %+v", got, ok, b)
	}
	h.Back()
	got, ok = h.Current()
	if !ok || !got.Equal(b) {
		t.Fatalf("Current after 1 Back = %+v ok=%v, want %+v", got, ok, b)
	}
	h.Back()
	got, ok = h.Current()
	if !ok || !got.Equal(a) {
		t.Fatalf("Current after 2 Backs = %+v ok=%v, want %+v", got, ok, a)
	}
}

func TestCanBackCanForward(t *testing.T) {
	h := New(10)
	if h.CanBack() || h.CanForward() {
		t.Fatal("empty CanBack/CanForward both true")
	}
	h.Push(Entry{Path: "/a.go"})
	h.Push(Entry{Path: "/b.go"})
	if !h.CanBack() {
		t.Fatal("CanBack false after Push")
	}
	if h.CanForward() {
		t.Fatal("CanForward true after Push (no Back yet)")
	}
	h.Back()
	// idx now at b (newest entry); cannot Forward beyond newest.
	if !h.CanBack() {
		t.Fatal("CanBack false after 1 Back (should still reach a)")
	}
	if h.CanForward() {
		t.Fatal("CanForward true at newest entry")
	}
	h.Back()
	// idx now at a (oldest); cannot Back further; can Forward to b.
	if h.CanBack() {
		t.Fatal("CanBack true at oldest")
	}
	if !h.CanForward() {
		t.Fatal("CanForward false after 2 Backs (should reach b)")
	}
}

func TestReset(t *testing.T) {
	h := New(10)
	h.Push(Entry{Path: "/a.go"})
	h.Push(Entry{Path: "/b.go"})
	h.Reset()
	if h.Len() != 0 {
		t.Fatalf("Len after Reset = %d, want 0", h.Len())
	}
	if h.CanBack() || h.CanForward() {
		t.Fatal("CanBack/CanForward after Reset both true")
	}
}

func TestNewWithZeroCapUsesDefault(t *testing.T) {
	h := New(0)
	if cap := h.effectiveCap(); cap != DefaultCap {
		t.Fatalf("New(0) effective cap = %d, want %d", cap, DefaultCap)
	}
	h2 := New(-5)
	if cap := h2.effectiveCap(); cap != DefaultCap {
		t.Fatalf("New(-5) effective cap = %d, want %d", cap, DefaultCap)
	}
}

func TestSnapshotIsCopy(t *testing.T) {
	h := New(10)
	a := Entry{Path: "/a.go", Row: 1}
	h.Push(a)
	snap := h.Snapshot()
	snap[0].Row = 999
	if got, _ := h.Current(); got.Row != 1 {
		t.Fatalf("mutating snapshot affected history; got Row=%d, want 1", got.Row)
	}
}

func TestRealisticJumpSequence(t *testing.T) {
	h := New(10)
	// Simulate: open main.go, Ctrl+] into pkg/util.go, then F12 into pkg/types.go,
	// then Alt+- back twice, then a fresh jump into pkg/log.go.
	main := Entry{Path: "/main.go", Row: 10, Col: 4}
	util := Entry{Path: "/pkg/util.go", Row: 5, Col: 0}
	types := Entry{Path: "/pkg/types.go", Row: 2, Col: 0}
	logp := Entry{Path: "/pkg/log.go", Row: 8, Col: 0}

	h.Push(main)
	h.Push(util)
	h.Push(types)

	got, _ := h.Back()
	if !got.Equal(types) {
		t.Fatalf("Back 1 = %+v, want types", got)
	}
	got, _ = h.Back()
	if !got.Equal(util) {
		t.Fatalf("Back 2 = %+v, want util", got)
	}

	// Forward tail [types] should still be intact.
	if !h.CanForward() {
		t.Fatal("CanForward false after 2 Backs")
	}

	// Fresh push truncates forward tail.
	h.Push(logp)
	snap := h.Snapshot()
	want := []Entry{main, logp}
	if len(snap) != len(want) {
		t.Fatalf("snap len = %d, want %d", len(snap), len(want))
	}
	for i := range want {
		if !snap[i].Equal(want[i]) {
			t.Fatalf("snap[%d] = %+v, want %+v", i, snap[i], want[i])
		}
	}
	if h.CanForward() {
		t.Fatal("CanForward true after truncating push")
	}
}
