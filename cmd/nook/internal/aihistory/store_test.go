package aihistory

import (
	"sync"
	"testing"
	"time"
)

func mkTurn(s string) Turn {
	return Turn{Instruction: "ask " + s, Response: "reply " + s, At: time.Date(2026, 5, 24, 20, 0, 0, 0, time.UTC)}
}

func TestNewStoreUsesDefaultCap(t *testing.T) {
	s := NewStore()
	if got := s.MaxPerPath(); got != DefaultMaxPerPath {
		t.Fatalf("MaxPerPath = %d, want %d", got, DefaultMaxPerPath)
	}
}

func TestNewStoreWithMaxClampsNonPositive(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		s := NewStoreWithMax(n)
		if got := s.MaxPerPath(); got != DefaultMaxPerPath {
			t.Fatalf("NewStoreWithMax(%d).MaxPerPath = %d, want default %d", n, got, DefaultMaxPerPath)
		}
	}
}

func TestNewStoreWithMaxHonorsPositive(t *testing.T) {
	s := NewStoreWithMax(3)
	if got := s.MaxPerPath(); got != 3 {
		t.Fatalf("MaxPerPath = %d, want 3", got)
	}
}

func TestAppendAndTurnsRoundTrip(t *testing.T) {
	s := NewStore()
	s.Append("a.go", mkTurn("one"))
	s.Append("a.go", mkTurn("two"))
	got := s.Turns("a.go")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Instruction != "ask one" || got[1].Instruction != "ask two" {
		t.Fatalf("turns out of order: %+v", got)
	}
}

func TestAppendRespectsCapEvictingOldest(t *testing.T) {
	s := NewStoreWithMax(3)
	for _, name := range []string{"one", "two", "three", "four", "five"} {
		s.Append("a.go", mkTurn(name))
	}
	got := s.Turns("a.go")
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (cap)", len(got))
	}
	if got[0].Instruction != "ask three" || got[1].Instruction != "ask four" || got[2].Instruction != "ask five" {
		t.Fatalf("oldest not evicted: %+v", got)
	}
}

func TestAppendEmptyPathIsNoOp(t *testing.T) {
	s := NewStore()
	s.Append("", mkTurn("x"))
	if c := s.Count(""); c != 0 {
		t.Fatalf("Count(empty) = %d, want 0", c)
	}
	if c := s.Count("a.go"); c != 0 {
		t.Fatalf("Count(a.go) = %d, want 0 — empty append leaked", c)
	}
}

func TestTurnsEmptyPathReturnsNil(t *testing.T) {
	s := NewStore()
	s.Append("a.go", mkTurn("one"))
	if got := s.Turns(""); got != nil {
		t.Fatalf("Turns(empty) = %v, want nil", got)
	}
}

func TestTurnsIsDefensiveCopy(t *testing.T) {
	s := NewStore()
	s.Append("a.go", mkTurn("one"))
	s.Append("a.go", mkTurn("two"))
	got := s.Turns("a.go")
	// Mutating the returned slice must not affect the store's record.
	got[0].Instruction = "tampered"
	extended := append(got, mkTurn("appended"))
	_ = extended
	again := s.Turns("a.go")
	if again[0].Instruction != "ask one" {
		t.Fatalf("first turn mutated: %q", again[0].Instruction)
	}
	if len(again) != 2 {
		t.Fatalf("len = %d, want 2 (caller append leaked)", len(again))
	}
}

func TestCountReportsLength(t *testing.T) {
	s := NewStore()
	if got := s.Count("a.go"); got != 0 {
		t.Fatalf("empty Count = %d, want 0", got)
	}
	s.Append("a.go", mkTurn("one"))
	s.Append("a.go", mkTurn("two"))
	if got := s.Count("a.go"); got != 2 {
		t.Fatalf("Count = %d, want 2", got)
	}
}

func TestClearRemovesPathReturnsCount(t *testing.T) {
	s := NewStore()
	s.Append("a.go", mkTurn("one"))
	s.Append("a.go", mkTurn("two"))
	s.Append("b.go", mkTurn("three"))
	if n := s.Clear("a.go"); n != 2 {
		t.Fatalf("Clear = %d, want 2", n)
	}
	if got := s.Count("a.go"); got != 0 {
		t.Fatalf("after Clear, Count = %d, want 0", got)
	}
	if got := s.Count("b.go"); got != 1 {
		t.Fatalf("Clear leaked into sibling path: Count(b.go) = %d", got)
	}
}

func TestClearEmptyPathIsNoOp(t *testing.T) {
	s := NewStore()
	s.Append("a.go", mkTurn("one"))
	if n := s.Clear(""); n != 0 {
		t.Fatalf("Clear(empty) = %d, want 0", n)
	}
	if got := s.Count("a.go"); got != 1 {
		t.Fatalf("Clear(empty) wiped real path: Count = %d", got)
	}
}

func TestClearAllRemovesEveryPath(t *testing.T) {
	s := NewStore()
	s.Append("a.go", mkTurn("one"))
	s.Append("a.go", mkTurn("two"))
	s.Append("b.go", mkTurn("three"))
	if n := s.ClearAll(); n != 3 {
		t.Fatalf("ClearAll = %d, want 3", n)
	}
	if got := s.Count("a.go"); got != 0 {
		t.Fatalf("a.go survived ClearAll: %d", got)
	}
	if got := s.Count("b.go"); got != 0 {
		t.Fatalf("b.go survived ClearAll: %d", got)
	}
}

func TestStoreIsConcurrencySafe(t *testing.T) {
	// Hammer Append + Turns + Count + Clear from many goroutines. The
	// race detector (`go test -race`) is the actual assertion; the
	// "no panic" outcome is the no-data-race signal.
	s := NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				s.Append("a.go", mkTurn("x"))
				_ = s.Turns("a.go")
				_ = s.Count("a.go")
				if j%10 == 0 {
					_ = s.Clear("a.go")
				}
			}
		}(i)
	}
	wg.Wait()
}
