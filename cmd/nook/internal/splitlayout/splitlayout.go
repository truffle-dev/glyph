// Package splitlayout models nook's view splits as a binary tree of panes.
// A leaf is a pane (an opaque PaneID the host maps to a buffer); an internal
// node is a split that divides its rectangle between two children, either as
// Columns (side by side, a vertical divider between them) or Rows (stacked,
// a horizontal divider). The tree owns pane identity and geometry only; what
// content a pane shows is the host's concern.
//
// The package is pure data and geometry: construction is constant-time, there
// is no I/O, and Rects is a single tree walk. That keeps it inside nook's
// first-paint rule — a layout never blocks a frame.
package splitlayout

import "math"

// Orientation is how an internal split arranges its two children.
type Orientation int

const (
	// Columns places children left and right, separated by a vertical
	// divider. This is the "split right" gesture.
	Columns Orientation = iota
	// Rows places children top and bottom, separated by a horizontal
	// divider. This is the "split down" gesture.
	Rows
)

// Direction is a focus-movement request relative to the focused pane.
type Direction int

const (
	Left Direction = iota
	Right
	Up
	Down
)

// PaneID is a stable identifier for a leaf. IDs are assigned by the tree and
// never reused within a tree's lifetime, so the host can keep a PaneID->buffer
// map without worrying about collisions after a close.
type PaneID int

// Rect is a pane's screen rectangle in cells. X,Y is the top-left corner.
type Rect struct {
	X, Y, W, H int
}

const (
	minRatio = 0.05
	maxRatio = 0.95
)

type node struct {
	leaf   bool
	id     PaneID // valid only when leaf
	orient Orientation
	ratio  float64 // fraction of available space given to child a
	a, b   *node
	parent *node
}

// Tree is a pane layout. The zero value is not usable; create one with New.
type Tree struct {
	root   *node
	focus  *node // always points at a leaf
	nextID PaneID
}

// New returns a tree with a single focused pane and reports that pane's ID.
func New() (*Tree, PaneID) {
	t := &Tree{nextID: 1}
	root := &node{leaf: true, id: t.nextID}
	t.nextID++
	t.root = root
	t.focus = root
	return t, root.id
}

// Count reports the number of panes (leaves).
func (t *Tree) Count() int { return countLeaves(t.root) }

func countLeaves(n *node) int {
	if n == nil {
		return 0
	}
	if n.leaf {
		return 1
	}
	return countLeaves(n.a) + countLeaves(n.b)
}

// Focused reports the currently focused pane.
func (t *Tree) Focused() PaneID { return t.focus.id }

// Focus sets focus to id and reports whether id exists.
func (t *Tree) Focus(id PaneID) bool {
	if n := findLeaf(t.root, id); n != nil {
		t.focus = n
		return true
	}
	return false
}

// Panes returns every pane ID in a stable left-to-right, top-to-bottom order
// (an in-order walk of the tree).
func (t *Tree) Panes() []PaneID {
	var out []PaneID
	collect(t.root, &out)
	return out
}

func collect(n *node, out *[]PaneID) {
	if n == nil {
		return
	}
	if n.leaf {
		*out = append(*out, n.id)
		return
	}
	collect(n.a, out)
	collect(n.b, out)
}

// SplitFocused replaces the focused pane with a split of the given orientation,
// keeping the old pane as the first child and adding a fresh pane as the
// second. Focus moves to the new pane, whose ID is returned. The split starts
// at an even ratio.
func (t *Tree) SplitFocused(o Orientation) PaneID {
	old := t.focus
	fresh := &node{leaf: true, id: t.nextID}
	t.nextID++

	split := &node{
		orient: o,
		ratio:  0.5,
		a:      old,
		b:      fresh,
		parent: old.parent,
	}
	old.parent = split
	fresh.parent = split

	if split.parent == nil {
		t.root = split
	} else if split.parent.a == old {
		split.parent.a = split
	} else {
		split.parent.b = split
	}

	t.focus = fresh
	return fresh.id
}

// CloseFocused removes the focused pane and collapses its parent split so the
// sibling takes the freed space. Focus moves to the first leaf of that sibling.
// Closing the last remaining pane is refused; it reports false.
func (t *Tree) CloseFocused() bool {
	leaf := t.focus
	p := leaf.parent
	if p == nil {
		return false // last pane
	}

	sib := p.a
	if sib == leaf {
		sib = p.b
	}

	sib.parent = p.parent
	if p.parent == nil {
		t.root = sib
	} else if p.parent.a == p {
		p.parent.a = sib
	} else {
		p.parent.b = sib
	}

	t.focus = firstLeaf(sib)
	return true
}

func firstLeaf(n *node) *node {
	for !n.leaf {
		n = n.a
	}
	return n
}

func findLeaf(n *node, id PaneID) *node {
	if n == nil {
		return nil
	}
	if n.leaf {
		if n.id == id {
			return n
		}
		return nil
	}
	if r := findLeaf(n.a, id); r != nil {
		return r
	}
	return findLeaf(n.b, id)
}

// FocusNext moves focus to the next pane in Panes order, wrapping around.
func (t *Tree) FocusNext() { t.step(1) }

// FocusPrev moves focus to the previous pane in Panes order, wrapping around.
func (t *Tree) FocusPrev() { t.step(-1) }

func (t *Tree) step(d int) {
	order := t.Panes()
	if len(order) < 2 {
		return
	}
	for i, id := range order {
		if id == t.focus.id {
			ni := (i + d + len(order)) % len(order)
			t.Focus(order[ni])
			return
		}
	}
}

// FocusDir moves focus to the nearest pane in the given direction, using the
// layout for the supplied width and height. It reports whether focus moved;
// when no pane lies that way, focus is unchanged. A pane qualifies only if it
// sits beyond the focused pane on the requested axis and overlaps it on the
// perpendicular axis. Ties break toward the pane whose perpendicular center is
// closest.
func (t *Tree) FocusDir(d Direction, width, height int) bool {
	rects := t.Rects(width, height)
	cur, ok := rects[t.focus.id]
	if !ok {
		return false
	}
	curCX, curCY := cur.X*2+cur.W, cur.Y*2+cur.H // doubled centers, integer-exact

	best := PaneID(0)
	bestPrimary, bestPerp := math.MaxInt, math.MaxInt
	for id, r := range rects {
		if id == t.focus.id {
			continue
		}
		var primary, perp int
		switch d {
		case Right:
			if r.X < cur.X+cur.W || !overlap(r.Y, r.H, cur.Y, cur.H) {
				continue
			}
			primary = r.X - (cur.X + cur.W)
			perp = abs((r.Y*2 + r.H) - curCY)
		case Left:
			if r.X+r.W > cur.X || !overlap(r.Y, r.H, cur.Y, cur.H) {
				continue
			}
			primary = cur.X - (r.X + r.W)
			perp = abs((r.Y*2 + r.H) - curCY)
		case Down:
			if r.Y < cur.Y+cur.H || !overlap(r.X, r.W, cur.X, cur.W) {
				continue
			}
			primary = r.Y - (cur.Y + cur.H)
			perp = abs((r.X*2 + r.W) - curCX)
		case Up:
			if r.Y+r.H > cur.Y || !overlap(r.X, r.W, cur.X, cur.W) {
				continue
			}
			primary = cur.Y - (r.Y + r.H)
			perp = abs((r.X*2 + r.W) - curCX)
		}
		if primary < bestPrimary || (primary == bestPrimary && perp < bestPerp) {
			best, bestPrimary, bestPerp = id, primary, perp
		}
	}
	if best == 0 {
		return false
	}
	return t.Focus(best)
}

func overlap(aStart, aLen, bStart, bLen int) bool {
	return aStart < bStart+bLen && bStart < aStart+aLen
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ResizeFocused shifts the divider of the focused pane's parent split by delta
// (a fraction, positive growing the focused side when it is child a). The
// ratio is clamped so neither pane vanishes. It reports false when the focused
// pane has no parent (a single-pane tree has nothing to resize).
func (t *Tree) ResizeFocused(delta float64) bool {
	p := t.focus.parent
	if p == nil {
		return false
	}
	r := p.ratio
	if p.b == t.focus {
		r -= delta
	} else {
		r += delta
	}
	p.ratio = clampRatio(r)
	return true
}

func clampRatio(r float64) float64 {
	if r < minRatio {
		return minRatio
	}
	if r > maxRatio {
		return maxRatio
	}
	return r
}

// Rects computes each pane's rectangle within the given total width and height.
// A one-cell gap is reserved between split children for the divider the host
// draws. Panes are never given negative sizes; in a space too small to honor a
// split, children collapse to zero width or height rather than overflow.
func (t *Tree) Rects(width, height int) map[PaneID]Rect {
	out := make(map[PaneID]Rect, t.Count())
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	layout(t.root, Rect{X: 0, Y: 0, W: width, H: height}, out)
	return out
}

func layout(n *node, r Rect, out map[PaneID]Rect) {
	if n.leaf {
		out[n.id] = r
		return
	}
	if n.orient == Columns {
		avail := r.W - 1
		if avail < 0 {
			avail = 0
		}
		aw := int(math.Round(float64(avail) * n.ratio))
		if aw > avail {
			aw = avail
		}
		bw := avail - aw
		layout(n.a, Rect{X: r.X, Y: r.Y, W: aw, H: r.H}, out)
		layout(n.b, Rect{X: r.X + aw + 1, Y: r.Y, W: bw, H: r.H}, out)
		return
	}
	avail := r.H - 1
	if avail < 0 {
		avail = 0
	}
	ah := int(math.Round(float64(avail) * n.ratio))
	if ah > avail {
		ah = avail
	}
	bh := avail - ah
	layout(n.a, Rect{X: r.X, Y: r.Y, W: r.W, H: ah}, out)
	layout(n.b, Rect{X: r.X, Y: r.Y + ah + 1, W: r.W, H: bh}, out)
}
