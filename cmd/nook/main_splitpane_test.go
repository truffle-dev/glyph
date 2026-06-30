package main

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/truffle-dev/glyph/cmd/nook/internal/splitlayout"
)

// legacyEditorRegion is the editor-region math exactly as it stood before the
// split layout tree was wired in. The test pins editorSize() against it so the
// single-pane path can never drift: with one pane, routing the region through
// splitlayout must reproduce the old dimensions byte for byte.
func legacyEditorRegion(width, height int, showTree bool, right rightPane, hasBuf bool) (int, int) {
	treeW := 0
	if showTree {
		treeW = width / 5
		if treeW < 22 {
			treeW = 22
		}
		if treeW > 40 {
			treeW = 40
		}
	}
	leftW := width - treeW
	if treeW > 0 {
		leftW--
	}
	if right != rightNone {
		rightW := width / 3
		if rightW < 40 {
			rightW = 40
		}
		if rightW > width-40 {
			rightW = width - 40
		}
		leftW = width - rightW - 1 - treeW
		if treeW > 0 {
			leftW--
		}
	}
	if leftW < 20 {
		leftW = 20
	}
	bodyH := height - 2
	if hasBuf {
		bodyH--
	}
	return leftW, bodyH
}

// TestEditorSizeSinglePaneMatchesLegacy is the slice-1 integration milestone:
// binding splitlayout into the host changes no on-screen geometry while a
// single pane is open. editorSize() now routes the editor region through the
// split tree; for one pane the focused rectangle must equal the legacy region
// across every tree / right-pane / size combination.
func TestEditorSizeSinglePaneMatchesLegacy(t *testing.T) {
	m := newModel(t.TempDir())
	if m.split == nil {
		t.Fatal("newModel left split tree nil")
	}
	if m.split.Count() != 1 {
		t.Fatalf("newModel split tree has %d panes, want 1", m.split.Count())
	}
	if got, ok := m.paneBuf[m.split.Focused()]; !ok || got != 0 {
		t.Fatalf("focused pane binds buffer (%d, ok=%v), want (0, true)", got, ok)
	}

	widths := []int{60, 80, 120, 200}
	heights := []int{12, 24, 50}
	rights := []rightPane{rightNone, rightGit}

	for _, w := range widths {
		for _, h := range heights {
			for _, tree := range []bool{false, true} {
				for _, r := range rights {
					m.width = w
					m.height = h
					m.showTree = tree
					m.right = r

					hasBuf := m.bufs.Count() > 0
					wantW, wantH := legacyEditorRegion(w, h, tree, r, hasBuf)

					gotW, gotH := m.editorSize()
					if gotW != wantW || gotH != wantH {
						t.Errorf("editorSize(w=%d h=%d tree=%v right=%d) = (%d,%d), want (%d,%d)",
							w, h, tree, r, gotW, gotH, wantW, wantH)
					}

					// The single pane must own the entire region: its
					// rectangle is the top-left corner with the full region
					// dimensions, never an offset or a shrunk slice.
					regW, regH := m.editorRegion()
					rect := m.split.Rects(regW, regH)[m.split.Focused()]
					if rect.X != 0 || rect.Y != 0 || rect.W != regW || rect.H != regH {
						t.Errorf("single pane rect = %+v, want {0,0,%d,%d}", rect, regW, regH)
					}
				}
			}
		}
	}
}

// TestEditorSizeNilSplitFallsBack guards the defensive nil branch: a model
// whose split tree was never initialised still reports the raw region rather
// than panicking on a nil-map index.
func TestEditorSizeNilSplitFallsBack(t *testing.T) {
	m := newModel(t.TempDir())
	m.split = nil
	m.width = 100
	m.height = 30

	gotW, gotH := m.editorSize()
	wantW, wantH := m.editorRegion()
	if gotW != wantW || gotH != wantH {
		t.Errorf("nil-split editorSize() = (%d,%d), want region (%d,%d)", gotW, gotH, wantW, wantH)
	}
}

// splitTwoBuffers seeds a model with two open buffers and one live split,
// binding pane 0 to buffer 0 and the freshly split pane to buffer 1. It is the
// shared fixture for the render-composition tests: it drives the split tree
// directly (no keybinding exists yet) so the two-pane compositor can be tested
// in isolation from the focus-routing slice.
func splitTwoBuffers(t *testing.T, o splitlayout.Orientation) (model, int, int) {
	t.Helper()
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.bufs.OpenOrSwitch(filepath.Join(root, "a.go"))
	m.bufs.OpenOrSwitch(filepath.Join(root, "sub", "b.go"))
	if m.bufs.Count() != 2 {
		t.Fatalf("want 2 open buffers, got %d", m.bufs.Count())
	}
	old := m.split.Focused()
	pid := m.split.SplitFocused(o)
	if m.split.Count() != 2 {
		t.Fatalf("split count = %d, want 2", m.split.Count())
	}
	m.paneBuf[old] = 0
	m.paneBuf[pid] = 1
	m = m.resize()
	regW, regH := m.editorRegion()
	return m, regW, regH
}

// TestRenderSplitColumnsFillsRegion checks the side-by-side compositor: the
// two panes plus a vertical divider must exactly fill the editor region, and
// both buffers' contents must be visible at once (the headline split win).
func TestRenderSplitColumnsFillsRegion(t *testing.T) {
	m, regW, regH := splitTwoBuffers(t, splitlayout.Columns)
	out := m.renderMainColumn()

	if w := lipgloss.Width(out); w != regW {
		t.Errorf("columns split width = %d, want region %d", w, regW)
	}
	if h := lipgloss.Height(out); h != regH {
		t.Errorf("columns split height = %d, want region %d", h, regH)
	}
	if !strings.Contains(out, "│") {
		t.Error("columns split missing vertical divider")
	}
	if !strings.Contains(out, "Foo") || !strings.Contains(out, "Bar") {
		t.Error("columns split should show both buffers (Foo from a.go, Bar from b.go)")
	}
}

// TestRenderSplitRowsFillsRegion is the stacked analogue: two panes plus a
// horizontal divider fill the region top to bottom, both buffers visible.
func TestRenderSplitRowsFillsRegion(t *testing.T) {
	m, regW, regH := splitTwoBuffers(t, splitlayout.Rows)
	out := m.renderMainColumn()

	if w := lipgloss.Width(out); w != regW {
		t.Errorf("rows split width = %d, want region %d", w, regW)
	}
	if h := lipgloss.Height(out); h != regH {
		t.Errorf("rows split height = %d, want region %d", h, regH)
	}
	if !strings.Contains(out, "─") {
		t.Error("rows split missing horizontal divider")
	}
	if !strings.Contains(out, "Foo") || !strings.Contains(out, "Bar") {
		t.Error("rows split should show both buffers (Foo from a.go, Bar from b.go)")
	}
}

// TestRenderSinglePaneUnchanged guards that with no split live the main column
// is byte-identical to the active buffer's own view: the compositor is dormant
// until a split actually exists.
func TestRenderSinglePaneUnchanged(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.bufs.OpenOrSwitch(filepath.Join(root, "a.go"))
	m = m.resize()

	if m.split.Count() != 1 {
		t.Fatalf("single pane expected, got %d", m.split.Count())
	}
	if got, want := m.renderMainColumn(), m.bufs.Active().View(); got != want {
		t.Error("single-pane renderMainColumn diverged from the active buffer view")
	}
}

// altW presses the window-command leader (alt+w) and returns the updated model.
func altW(m model) model {
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}, Alt: true})
	return u.(model)
}

// runeKey presses a single plain rune (no modifiers) and returns the model.
func runeKey(m model, r rune) model {
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return u.(model)
}

// openTwo seeds a model sized for split work with two real buffers open. The
// keybinding tests start here and then drive the alt+w leader so the split /
// close handlers are exercised through the same key path a user takes.
func openTwo(t *testing.T) model {
	t.Helper()
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.bufs.OpenOrSwitch(filepath.Join(root, "a.go"))
	m.bufs.OpenOrSwitch(filepath.Join(root, "sub", "b.go"))
	if m.bufs.Count() != 2 {
		t.Fatalf("want 2 buffers, got %d", m.bufs.Count())
	}
	return m
}

// TestWindowLeaderSplitColumns drives alt+w v and asserts a second column pane
// appears bound to the other buffer, leaving the leader state disarmed.
func TestWindowLeaderSplitColumns(t *testing.T) {
	m := openTwo(t)
	m = altW(m)
	if !m.awaitingWindowKey {
		t.Fatal("alt+w did not arm the window leader")
	}
	m = runeKey(m, 'v')
	if m.awaitingWindowKey {
		t.Error("window leader still armed after running a window op")
	}
	if m.split.Count() != 2 {
		t.Fatalf("split count = %d, want 2", m.split.Count())
	}
	divs := m.split.Dividers(m.editorRegion())
	if len(divs) != 1 || divs[0].Orient != splitlayout.Columns {
		t.Errorf("want a single columns divider, got %+v", divs)
	}
}

// TestWindowLeaderSplitRows drives alt+w s and asserts a stacked-row split.
func TestWindowLeaderSplitRows(t *testing.T) {
	m := openTwo(t)
	m = altW(m)
	m = runeKey(m, 's')
	if m.split.Count() != 2 {
		t.Fatalf("split count = %d, want 2", m.split.Count())
	}
	divs := m.split.Dividers(m.editorRegion())
	if len(divs) != 1 || divs[0].Orient != splitlayout.Rows {
		t.Errorf("want a single rows divider, got %+v", divs)
	}
}

// TestWindowLeaderClosePane splits then closes via alt+w c, returning to one
// pane while both buffers stay open as tabs.
func TestWindowLeaderClosePane(t *testing.T) {
	m := openTwo(t)
	m = altW(m)
	m = runeKey(m, 'v')
	if m.split.Count() != 2 {
		t.Fatalf("split count = %d after split, want 2", m.split.Count())
	}
	m = altW(m)
	m = runeKey(m, 'c')
	if m.split.Count() != 1 {
		t.Fatalf("split count = %d after close, want 1", m.split.Count())
	}
	if m.bufs.Count() != 2 {
		t.Errorf("closing a pane dropped a buffer: count = %d, want 2", m.bufs.Count())
	}
}

// TestWindowLeaderCancels guards the disarm path: alt+w then an unrelated key
// runs no window op and clears the leader, never splitting.
func TestWindowLeaderCancels(t *testing.T) {
	m := openTwo(t)
	m = altW(m)
	m = runeKey(m, 'z')
	if m.awaitingWindowKey {
		t.Error("unrelated key left the window leader armed")
	}
	if m.split.Count() != 1 {
		t.Errorf("unrelated key after alt+w changed the split: count = %d, want 1", m.split.Count())
	}
}

// TestWindowLeaderRefusesSingleBuffer guards the v1 rule that a split needs a
// second buffer to show; with one buffer open alt+w v is a no-op with a hint.
func TestWindowLeaderRefusesSingleBuffer(t *testing.T) {
	root := fixtureRepo(t)
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.bufs.OpenOrSwitch(filepath.Join(root, "a.go"))
	if m.bufs.Count() != 1 {
		t.Fatalf("want 1 buffer, got %d", m.bufs.Count())
	}
	m = altW(m)
	m = runeKey(m, 'v')
	if m.split.Count() != 1 {
		t.Errorf("split with one buffer should be refused, count = %d", m.split.Count())
	}
}

// TestWindowLeaderFocusNextSyncsBuffer is the headline of slice 3: moving split
// focus must carry the active buffer with it, because every editing path keys
// off the active buffer (the focus==active invariant). After a split, alt+w w
// cycles to the other pane and the active buffer becomes that pane's binding.
func TestWindowLeaderFocusNextSyncsBuffer(t *testing.T) {
	m := openTwo(t)
	m = altW(m)
	m = runeKey(m, 'v')
	first := m.split.Focused()
	if m.bufs.ActiveIndex() != m.paneBuf[first] {
		t.Fatalf("post-split invariant broken: active=%d, focused pane binds %d",
			m.bufs.ActiveIndex(), m.paneBuf[first])
	}

	m = altW(m)
	m = runeKey(m, 'w')
	if m.split.Focused() == first {
		t.Fatal("alt+w w did not move focus to the other pane")
	}
	if m.bufs.ActiveIndex() != m.paneBuf[m.split.Focused()] {
		t.Errorf("focus==active broken after move: active=%d, focused pane binds %d",
			m.bufs.ActiveIndex(), m.paneBuf[m.split.Focused()])
	}
	if m.bufs.ActiveIndex() == m.paneBuf[first] {
		t.Error("active buffer did not change when focus moved to the other pane")
	}
}

// TestWindowLeaderFocusDirNoNeighborIsNoOp guards the dead-end path: in a
// side-by-side columns split there is no pane above, so alt+w k must leave
// focus exactly where it was rather than wrapping or panicking.
func TestWindowLeaderFocusDirNoNeighborIsNoOp(t *testing.T) {
	m := openTwo(t)
	m = altW(m)
	m = runeKey(m, 'v') // columns: the two panes sit side by side
	before := m.split.Focused()

	m = altW(m)
	m = runeKey(m, 'k') // Up has no neighbour in a columns split
	if m.split.Focused() != before {
		t.Error("alt+w k moved focus in a columns split with no vertical neighbour")
	}
}

// TestWindowLeaderFocusNoSplitIsNoOp confirms the focus chords are inert with a
// single pane: alt+w w / alt+w l must not panic or change the active buffer.
func TestWindowLeaderFocusNoSplitIsNoOp(t *testing.T) {
	m := openTwo(t) // two buffers, but no split yet
	before := m.bufs.ActiveIndex()
	if m.split.Count() != 1 {
		t.Fatalf("expected a single pane before splitting, got %d", m.split.Count())
	}

	m = altW(m)
	m = runeKey(m, 'w')
	m = altW(m)
	m = runeKey(m, 'l')
	if m.split.Count() != 1 {
		t.Errorf("focus chord created a split: count = %d", m.split.Count())
	}
	if m.bufs.ActiveIndex() != before {
		t.Errorf("focus chord with no split changed the active buffer: %d -> %d",
			before, m.bufs.ActiveIndex())
	}
}

// focusedRectW returns the width of the currently focused pane's rectangle,
// the observable quantity that resize moves.
func focusedRectW(m model) int {
	w, h := m.editorRegion()
	return m.split.Rects(w, h)[m.split.Focused()].W
}

// TestWindowLeaderResizeGrowsFocused drives alt+w > and asserts the focused
// pane gets wider while its sibling shrinks; a positive delta always grows the
// focused side (ResizeFocused folds the child-a/child-b sign for us).
func TestWindowLeaderResizeGrowsFocused(t *testing.T) {
	m := openTwo(t)
	m = altW(m)
	m = runeKey(m, 'v') // columns split, focus on the new (right) pane
	before := focusedRectW(m)

	m = altW(m)
	m = runeKey(m, '>')
	after := focusedRectW(m)
	if after <= before {
		t.Errorf("alt+w > did not grow the focused pane: %d -> %d", before, after)
	}
}

// TestWindowLeaderResizeShrinksFocused is the mirror: alt+w < narrows the
// focused pane.
func TestWindowLeaderResizeShrinksFocused(t *testing.T) {
	m := openTwo(t)
	m = altW(m)
	m = runeKey(m, 'v')
	before := focusedRectW(m)

	m = altW(m)
	m = runeKey(m, '<')
	after := focusedRectW(m)
	if after >= before {
		t.Errorf("alt+w < did not shrink the focused pane: %d -> %d", before, after)
	}
}

// TestWindowLeaderResizeClampsBeforeCollapse hammers alt+w < far past the
// floor and confirms the sibling pane never vanishes: the split tree clamps
// the ratio so both panes keep a positive width.
func TestWindowLeaderResizeClampsBeforeCollapse(t *testing.T) {
	m := openTwo(t)
	m = altW(m)
	m = runeKey(m, 'v')

	for i := 0; i < 40; i++ {
		m = altW(m)
		m = runeKey(m, '<')
	}
	w, h := m.editorRegion()
	rects := m.split.Rects(w, h)
	for pid, r := range rects {
		if r.W <= 0 {
			t.Errorf("pane %v collapsed to width %d after repeated shrink", pid, r.W)
		}
	}
}

// TestWindowLeaderResizeNoSplitIsNoOp confirms the resize chord is inert with a
// single pane: alt+w > / alt+w < must not panic or create a split.
func TestWindowLeaderResizeNoSplitIsNoOp(t *testing.T) {
	m := openTwo(t) // two buffers, no split yet
	m = altW(m)
	m = runeKey(m, '>')
	m = altW(m)
	m = runeKey(m, '<')
	if m.split.Count() != 1 {
		t.Errorf("resize chord created a split: count = %d", m.split.Count())
	}
}
