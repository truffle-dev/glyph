// Package aihistory keeps a small per-file conversation buffer for the
// composer wedge. The composer's prompt builder reads recent turns so a
// second Ctrl+L on the same file picks up where the first left off
// instead of re-introducing the codebase from scratch.
//
// Storage is in-memory only. The slice per path is capped, with the
// oldest turn evicted on overflow so a long session doesn't balloon
// memory. Disk persistence is intentionally deferred: composer
// transcripts are large, file paths are absolute and machine-specific,
// and the value of cross-restart continuity for an editor is unclear
// until users ask for it.
package aihistory

import (
	"sync"
	"time"
)

// DefaultMaxPerPath caps the number of turns kept per file. Eight turns
// is enough to ground a follow-up without flooding Sonnet's prompt; the
// oldest turn is dropped when a ninth arrives.
const DefaultMaxPerPath = 8

// Turn is one composer round-trip: the user's instruction plus the raw
// model response. The composer parses the response into Edits at render
// time; this layer stores the original text so a follow-up turn can see
// what was proposed (Applied or otherwise).
type Turn struct {
	Instruction string
	Response    string
	At          time.Time
}

// Store is a concurrency-safe per-path turn buffer.
type Store struct {
	mu     sync.RWMutex
	turns  map[string][]Turn
	maxPer int
}

// NewStore returns a Store with the default per-path cap.
func NewStore() *Store {
	return &Store{turns: map[string][]Turn{}, maxPer: DefaultMaxPerPath}
}

// NewStoreWithMax returns a Store with a custom per-path cap. A non-
// positive cap falls back to DefaultMaxPerPath.
func NewStoreWithMax(n int) *Store {
	if n <= 0 {
		n = DefaultMaxPerPath
	}
	return &Store{turns: map[string][]Turn{}, maxPer: n}
}

// Append records a Turn for the given path. When the per-path cap is
// exceeded, the oldest turn is evicted. An empty path is a no-op so
// callers don't have to gate on "buffer has a path."
func (s *Store) Append(path string, t Turn) {
	if path == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list := append(s.turns[path], t)
	if len(list) > s.maxPer {
		list = list[len(list)-s.maxPer:]
	}
	s.turns[path] = list
}

// Turns returns a defensive copy of the slice for path. Callers can
// iterate freely without holding the lock. An empty path returns nil.
func (s *Store) Turns(path string) []Turn {
	if path == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := s.turns[path]
	if len(list) == 0 {
		return nil
	}
	out := make([]Turn, len(list))
	copy(out, list)
	return out
}

// Count reports how many turns are stored for path.
func (s *Store) Count(path string) int {
	if path == "" {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.turns[path])
}

// Clear drops all turns for path and returns how many were removed.
func (s *Store) Clear(path string) int {
	if path == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.turns[path])
	delete(s.turns, path)
	return n
}

// ClearAll wipes every per-path buffer and returns the total cleared.
func (s *Store) ClearAll() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	for _, v := range s.turns {
		n += len(v)
	}
	s.turns = map[string][]Turn{}
	return n
}

// MaxPerPath reports the configured per-path cap.
func (s *Store) MaxPerPath() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxPer
}
