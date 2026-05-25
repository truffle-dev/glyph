// Package navhistory implements a vim-style jump list for nook.
//
// Every jump that moves the cursor across or within buffers pushes a
// position entry onto the list. Alt+- walks back to a prior position;
// Alt+= walks forward. A fresh push while walking back truncates the
// forward tail, mirroring Vim's Ctrl-O / Ctrl-I behavior.
package navhistory

// Entry is a single recorded position. Row and Col are zero-based to
// match the editor's internal coordinates; the host translates at the
// boundary if it needs 1-based values.
type Entry struct {
	Path string
	Row  int
	Col  int
}

// Equal reports whether two entries refer to the same path and exact
// position.
func (e Entry) Equal(other Entry) bool {
	return e.Path == other.Path && e.Row == other.Row && e.Col == other.Col
}

// DefaultCap is the default ring-buffer capacity. 100 entries is enough
// for a long debugging session without bloating the model state.
const DefaultCap = 100

// History is the jump list itself. The zero value is usable.
//
// State invariants:
//   - entries is a chronological list of pushed positions.
//   - idx is in [0, len(entries)]. idx == len(entries) is the "past-end"
//     state that occurs immediately after a Push: there is no current
//     entry positioned within the slice, but the latest push is at
//     entries[len-1].
//   - Back decrements idx (when idx > 0) and returns entries[idx]. The
//     first Back after a sequence of Pushes returns the most-recent
//     entry. Subsequent Backs walk earlier.
//   - Forward increments idx (when idx < len-1) and returns entries[idx].
//     Forward never re-enters the past-end state — that state is reached
//     only via Push.
type History struct {
	entries []Entry
	idx     int
	cap     int
}

// New returns a History with the given capacity. A capacity <= 0
// becomes DefaultCap.
func New(capacity int) *History {
	if capacity <= 0 {
		capacity = DefaultCap
	}
	return &History{cap: capacity}
}

func (h *History) effectiveCap() int {
	if h.cap <= 0 {
		return DefaultCap
	}
	return h.cap
}

// Push records a new entry.
//
// Three cases, evaluated in order:
//  1. If the entry at idx-1 (the position "behind" the cursor) already
//     equals e, the list is unchanged and idx jumps to len(entries) —
//     i.e. the cursor returns to past-end without any structural change.
//     This collapses repeated no-op pushes onto the same position.
//  2. If the entry at idx (the next-forward entry) already equals e,
//     idx is advanced into it and the list is unchanged. This collapses
//     a push that exactly matches a forward-stack consumption.
//  3. Otherwise, the forward tail entries[idx:] is discarded (vim-style
//     truncate), e is appended, idx is set to the new len(entries), and
//     the list is capped to effectiveCap (oldest evicted).
func (h *History) Push(e Entry) {
	if e.Path == "" {
		return
	}
	if h.idx > 0 && h.entries[h.idx-1].Equal(e) {
		h.idx = len(h.entries)
		return
	}
	if h.idx < len(h.entries) && h.entries[h.idx].Equal(e) {
		h.idx++
		return
	}
	if h.idx < len(h.entries) {
		h.entries = h.entries[:h.idx]
	}
	h.entries = append(h.entries, e)
	if cap := h.effectiveCap(); len(h.entries) > cap {
		over := len(h.entries) - cap
		h.entries = h.entries[over:]
	}
	h.idx = len(h.entries)
}

// Back walks one step toward the past. Returns the entry walked to plus
// true on success; the zero Entry plus false when at the oldest entry
// or when the list is empty.
//
// The first Back after a sequence of Pushes returns the most-recent
// entry (idx moves from len(entries) to len(entries)-1).
func (h *History) Back() (Entry, bool) {
	if h.idx == 0 {
		return Entry{}, false
	}
	h.idx--
	return h.entries[h.idx], true
}

// Forward walks one step toward the future. Returns the entry walked to
// plus true on success; the zero Entry plus false when at the newest
// entry, in past-end, or when the list is empty.
//
// Forward never re-enters the past-end state. The cursor lands at most
// at entries[len-1].
func (h *History) Forward() (Entry, bool) {
	if h.idx >= len(h.entries)-1 {
		return Entry{}, false
	}
	h.idx++
	return h.entries[h.idx], true
}

// Current returns the entry at the current position plus true, or the
// zero Entry plus false when the list is empty.
//
// In the past-end state (idx == len(entries) immediately after a Push),
// Current returns the most-recently-pushed entry.
func (h *History) Current() (Entry, bool) {
	n := len(h.entries)
	if n == 0 {
		return Entry{}, false
	}
	if h.idx >= n {
		return h.entries[n-1], true
	}
	if h.idx < 0 {
		return Entry{}, false
	}
	return h.entries[h.idx], true
}

// Len reports the number of entries currently in the list.
func (h *History) Len() int { return len(h.entries) }

// Position returns (1-based index, total) suitable for a status-bar
// readout. Returns (0, 0) when the list is empty. In the past-end
// state, position equals total.
func (h *History) Position() (int, int) {
	n := len(h.entries)
	if n == 0 {
		return 0, 0
	}
	if h.idx >= n {
		return n, n
	}
	return h.idx + 1, n
}

// CanBack reports whether Back would succeed.
func (h *History) CanBack() bool { return h.idx > 0 }

// CanForward reports whether Forward would succeed.
func (h *History) CanForward() bool { return h.idx < len(h.entries)-1 }

// Reset clears the jump list entirely. Capacity is preserved.
func (h *History) Reset() {
	h.entries = nil
	h.idx = 0
}

// Snapshot returns a copy of the current entries, oldest to newest.
func (h *History) Snapshot() []Entry {
	if len(h.entries) == 0 {
		return nil
	}
	out := make([]Entry, len(h.entries))
	copy(out, h.entries)
	return out
}
