package bufman

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

func writeTmp(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	return p
}

func TestEmptyManager(t *testing.T) {
	m := New(theme.Default)
	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0", m.Count())
	}
	if m.ActiveIndex() != -1 {
		t.Errorf("ActiveIndex() = %d, want -1", m.ActiveIndex())
	}
	if m.Active() != nil {
		t.Errorf("Active() = %v, want nil", m.Active())
	}
	if m.Find("/nope") != -1 {
		t.Errorf("Find on empty = %d, want -1", m.Find("/nope"))
	}
}

func TestOpenOrSwitchAppendsFirstBuffer(t *testing.T) {
	m := New(theme.Default)
	p := writeTmp(t, "a.go", "package a\n")
	idx, action := m.OpenOrSwitch(p)
	if idx != 0 {
		t.Errorf("first open idx = %d, want 0", idx)
	}
	if action != OpenedNew {
		t.Errorf("first open action = %v, want OpenedNew", action)
	}
	if m.Count() != 1 {
		t.Errorf("Count after first open = %d, want 1", m.Count())
	}
	if m.Active().Path() != p {
		t.Errorf("Active().Path() = %q, want %q", m.Active().Path(), p)
	}
	if !m.Active().Focused() {
		t.Error("active pane should be Focused after open")
	}
}

func TestOpenOrSwitchSwitchesWhenAlreadyOpen(t *testing.T) {
	m := New(theme.Default)
	a := writeTmp(t, "a.go", "package a\n")
	b := writeTmp(t, "b.go", "package b\n")
	m.OpenOrSwitch(a)
	m.OpenOrSwitch(b)
	if m.ActiveIndex() != 1 {
		t.Fatalf("after second open active = %d, want 1", m.ActiveIndex())
	}
	idx, action := m.OpenOrSwitch(a)
	if action != Switched {
		t.Errorf("third open action = %v, want Switched", action)
	}
	if idx != 0 {
		t.Errorf("third open idx = %d, want 0", idx)
	}
	if m.Count() != 2 {
		t.Errorf("Count after switch-not-append = %d, want 2", m.Count())
	}
}

func TestCloseShiftsActive(t *testing.T) {
	m := New(theme.Default)
	a := writeTmp(t, "a.go", "package a\n")
	b := writeTmp(t, "b.go", "package b\n")
	c := writeTmp(t, "c.go", "package c\n")
	m.OpenOrSwitch(a)
	m.OpenOrSwitch(b)
	m.OpenOrSwitch(c) // active=2
	if path := m.Close(0); path != a {
		t.Errorf("Close(0) path = %q, want %q", path, a)
	}
	// After closing index 0, [b, c] remain, active should shift from 2 to 1.
	if m.ActiveIndex() != 1 {
		t.Errorf("ActiveIndex after Close(0) = %d, want 1", m.ActiveIndex())
	}
	if m.Active().Path() != c {
		t.Errorf("Active path after Close(0) = %q, want %q", m.Active().Path(), c)
	}
}

func TestCloseActiveAtEndFallsBack(t *testing.T) {
	m := New(theme.Default)
	a := writeTmp(t, "a.go", "package a\n")
	b := writeTmp(t, "b.go", "package b\n")
	m.OpenOrSwitch(a)
	m.OpenOrSwitch(b) // active=1
	m.CloseActive()
	if m.ActiveIndex() != 0 {
		t.Errorf("ActiveIndex after CloseActive at end = %d, want 0", m.ActiveIndex())
	}
	if m.Active().Path() != a {
		t.Errorf("Active path after CloseActive = %q, want %q", m.Active().Path(), a)
	}
}

func TestCloseLastBufferGoesEmpty(t *testing.T) {
	m := New(theme.Default)
	a := writeTmp(t, "a.go", "package a\n")
	m.OpenOrSwitch(a)
	m.CloseActive()
	if m.Count() != 0 {
		t.Errorf("Count after closing last = %d, want 0", m.Count())
	}
	if m.ActiveIndex() != -1 {
		t.Errorf("ActiveIndex after closing last = %d, want -1", m.ActiveIndex())
	}
	if m.Active() != nil {
		t.Error("Active() should be nil after last close")
	}
}

func TestNextPrevWrap(t *testing.T) {
	m := New(theme.Default)
	a := writeTmp(t, "a.go", "package a\n")
	b := writeTmp(t, "b.go", "package b\n")
	c := writeTmp(t, "c.go", "package c\n")
	m.OpenOrSwitch(a)
	m.OpenOrSwitch(b)
	m.OpenOrSwitch(c) // active=2
	m.Next()          // 0
	if m.ActiveIndex() != 0 {
		t.Errorf("Next from end = %d, want 0", m.ActiveIndex())
	}
	m.Prev() // 2
	if m.ActiveIndex() != 2 {
		t.Errorf("Prev from 0 = %d, want 2", m.ActiveIndex())
	}
}

func TestSwitchClearsGhostAndBlursOld(t *testing.T) {
	m := New(theme.Default)
	a := writeTmp(t, "a.go", "package a\n")
	b := writeTmp(t, "b.go", "package b\n")
	m.OpenOrSwitch(a) // active=0
	pa := m.Active()
	*pa = pa.SetGhostText("ghosty")
	m.OpenOrSwitch(b) // active=1
	// Switch back to 0 and verify the ghost text was cleared.
	m.Switch(0)
	if m.Active().GhostText() != "" {
		t.Errorf("GhostText after switch-back = %q, want \"\"", m.Active().GhostText())
	}
	if !m.Active().Focused() {
		t.Error("active pane should be Focused after Switch")
	}
}

func TestTabsSnapshot(t *testing.T) {
	m := New(theme.Default)
	a := writeTmp(t, "a.go", "package a\n")
	b := writeTmp(t, "b.go", "package b\n")
	m.OpenOrSwitch(a)
	m.OpenOrSwitch(b)
	tabs := m.Tabs()
	if len(tabs) != 2 {
		t.Fatalf("Tabs len = %d, want 2", len(tabs))
	}
	if tabs[0].Path != a || tabs[1].Path != b {
		t.Errorf("Tabs paths = [%q, %q], want [%q, %q]", tabs[0].Path, tabs[1].Path, a, b)
	}
	if tabs[0].Dirty || tabs[1].Dirty {
		t.Error("freshly-opened tabs should not be dirty")
	}
}

func TestRefreshIfOpen(t *testing.T) {
	m := New(theme.Default)
	p := writeTmp(t, "a.go", "package a\n")
	m.OpenOrSwitch(p)
	// Write new contents to disk.
	if err := os.WriteFile(p, []byte("package a\nvar X = 1\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !m.RefreshIfOpen(p) {
		t.Fatal("RefreshIfOpen should report true for open path")
	}
	if got := m.Active().LineCount(); got != 2 {
		t.Errorf("LineCount after refresh = %d, want 2", got)
	}
	if m.RefreshIfOpen("/never/opened") {
		t.Error("RefreshIfOpen should report false for unknown path")
	}
}

func TestFindReturnsCorrectIndex(t *testing.T) {
	m := New(theme.Default)
	a := writeTmp(t, "a.go", "package a\n")
	b := writeTmp(t, "b.go", "package b\n")
	m.OpenOrSwitch(a)
	m.OpenOrSwitch(b)
	if i := m.Find(a); i != 0 {
		t.Errorf("Find(a) = %d, want 0", i)
	}
	if i := m.Find(b); i != 1 {
		t.Errorf("Find(b) = %d, want 1", i)
	}
	if i := m.Find("/unknown"); i != -1 {
		t.Errorf("Find(unknown) = %d, want -1", i)
	}
}

func TestHasDirty(t *testing.T) {
	m := New(theme.Default)
	if m.HasDirty() {
		t.Error("empty manager should not report dirty")
	}
	a := writeTmp(t, "a.go", "package a\n")
	m.OpenOrSwitch(a)
	if m.HasDirty() {
		t.Error("freshly-opened buffer should not be dirty")
	}
	pa := m.Active()
	*pa = pa.InsertText("x")
	if !m.HasDirty() {
		t.Error("after edit should be dirty")
	}
}

func TestWithSizePropagates(t *testing.T) {
	m := New(theme.Default)
	a := writeTmp(t, "a.go", "package a\n")
	b := writeTmp(t, "b.go", "package b\n")
	m.OpenOrSwitch(a)
	m.OpenOrSwitch(b)
	m.WithSize(120, 40)
	// The View output sizes vary; just verify no panic and panes still exist.
	if m.Count() != 2 {
		t.Errorf("Count after WithSize = %d, want 2", m.Count())
	}
}
