// Package breakpoints stores per-file breakpoint state — a set of 0-based
// line indices keyed by absolute file path. The store is editor-agnostic:
// callers (editor, gutter renderer, DAP session) consult it the same way.
// The DAP session uses Snapshot to push the current set to the debugger
// after each toggle.
package breakpoints

import (
	"sort"
	"sync"
)

// Store is the per-path breakpoint set. Safe for concurrent use.
type Store struct {
	mu  sync.RWMutex
	per map[string]map[int]struct{}
}

// New constructs an empty Store.
func New() *Store {
	return &Store{per: make(map[string]map[int]struct{})}
}

// Toggle flips the breakpoint at row in path. Returns the new state
// (true = breakpoint now set, false = breakpoint cleared).
func (s *Store) Toggle(path string, row int) bool {
	if path == "" || row < 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.per[path]
	if !ok {
		set = make(map[int]struct{})
		s.per[path] = set
	}
	if _, exists := set[row]; exists {
		delete(set, row)
		if len(set) == 0 {
			delete(s.per, path)
		}
		return false
	}
	set[row] = struct{}{}
	return true
}

// Has reports whether path has a breakpoint at row.
func (s *Store) Has(path string, row int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set, ok := s.per[path]
	if !ok {
		return false
	}
	_, ok = set[row]
	return ok
}

// Rows returns a sorted slice of 0-based rows that hold a breakpoint
// in path. Empty slice for paths with no breakpoints.
func (s *Store) Rows(path string) []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set, ok := s.per[path]
	if !ok {
		return nil
	}
	out := make([]int, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	sort.Ints(out)
	return out
}

// Paths returns the absolute paths with at least one breakpoint set,
// sorted lexicographically.
func (s *Store) Paths() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.per))
	for p := range s.per {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Count returns the total number of breakpoints across all paths.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, set := range s.per {
		n += len(set)
	}
	return n
}

// Clear removes every breakpoint in path. Returns the number of breakpoints
// that were cleared. Zero is returned if the path had none.
func (s *Store) Clear(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.per[path])
	delete(s.per, path)
	return n
}

// ClearAll removes every breakpoint in every path. Returns the total
// number cleared.
func (s *Store) ClearAll() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, set := range s.per {
		n += len(set)
	}
	s.per = make(map[string]map[int]struct{})
	return n
}

// Snapshot returns a per-path map of sorted 0-based rows. The result
// is a fresh copy safe for the caller to mutate.
func (s *Store) Snapshot() map[string][]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]int, len(s.per))
	for p, set := range s.per {
		rows := make([]int, 0, len(set))
		for r := range set {
			rows = append(rows, r)
		}
		sort.Ints(rows)
		out[p] = rows
	}
	return out
}
