package main

import "testing"

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
