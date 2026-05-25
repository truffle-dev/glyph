package breakpoints

import (
	"reflect"
	"sync"
	"testing"
)

func TestToggleFlipsState(t *testing.T) {
	s := New()
	if s.Has("a.go", 10) {
		t.Fatal("fresh store reports breakpoint set")
	}
	if got := s.Toggle("a.go", 10); !got {
		t.Errorf("first toggle = false; want true")
	}
	if !s.Has("a.go", 10) {
		t.Errorf("after toggle Has returns false")
	}
	if got := s.Toggle("a.go", 10); got {
		t.Errorf("second toggle = true; want false (cleared)")
	}
	if s.Has("a.go", 10) {
		t.Errorf("after second toggle Has still true")
	}
}

func TestToggleRejectsBadInput(t *testing.T) {
	s := New()
	if s.Toggle("", 5) {
		t.Error("empty path accepted")
	}
	if s.Toggle("a.go", -1) {
		t.Error("negative row accepted")
	}
}

func TestRowsSorted(t *testing.T) {
	s := New()
	s.Toggle("a.go", 7)
	s.Toggle("a.go", 1)
	s.Toggle("a.go", 13)
	got := s.Rows("a.go")
	want := []int{1, 7, 13}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Rows = %v; want %v", got, want)
	}
	if got := s.Rows("b.go"); got != nil {
		t.Errorf("Rows for unknown path = %v; want nil", got)
	}
}

func TestPathsSorted(t *testing.T) {
	s := New()
	s.Toggle("zeta.go", 1)
	s.Toggle("alpha.go", 1)
	s.Toggle("middle.go", 1)
	got := s.Paths()
	want := []string{"alpha.go", "middle.go", "zeta.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Paths = %v; want %v", got, want)
	}
}

func TestCountAndClear(t *testing.T) {
	s := New()
	s.Toggle("a.go", 1)
	s.Toggle("a.go", 2)
	s.Toggle("b.go", 5)
	if got := s.Count(); got != 3 {
		t.Errorf("Count = %d; want 3", got)
	}
	if got := s.Clear("a.go"); got != 2 {
		t.Errorf("Clear a.go = %d; want 2", got)
	}
	if got := s.Count(); got != 1 {
		t.Errorf("Count after clear = %d; want 1", got)
	}
	if got := s.Clear("missing.go"); got != 0 {
		t.Errorf("Clear of unknown = %d; want 0", got)
	}
	s.Toggle("a.go", 3)
	if got := s.ClearAll(); got != 2 {
		t.Errorf("ClearAll = %d; want 2", got)
	}
	if got := s.Count(); got != 0 {
		t.Errorf("Count after ClearAll = %d; want 0", got)
	}
}

func TestEmptyPathSetRemovedAfterToggleOff(t *testing.T) {
	s := New()
	s.Toggle("a.go", 1)
	s.Toggle("a.go", 1)
	if got := s.Paths(); len(got) != 0 {
		t.Errorf("Paths after toggle-off = %v; want []", got)
	}
}

func TestSnapshotIsCopy(t *testing.T) {
	s := New()
	s.Toggle("a.go", 1)
	s.Toggle("a.go", 4)
	snap := s.Snapshot()
	if got, want := snap["a.go"], []int{1, 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("snap[a.go] = %v; want %v", got, want)
	}
	// mutate snapshot — store must be unaffected
	snap["a.go"][0] = 99
	if got := s.Rows("a.go"); !reflect.DeepEqual(got, []int{1, 4}) {
		t.Errorf("store Rows mutated by snapshot edit: %v", got)
	}
}

func TestConcurrentTogglesDontPanic(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.Toggle("x.go", i)
			_ = s.Has("x.go", i)
			_ = s.Rows("x.go")
		}(i)
	}
	wg.Wait()
	if got := s.Count(); got != 20 {
		t.Errorf("Count after parallel toggles = %d; want 20", got)
	}
}
