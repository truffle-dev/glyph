package bufman

import (
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestSetThemeUpdatesManagerAndAllPanes(t *testing.T) {
	m := New(theme.Default)
	m.WithSize(40, 6)
	a := writeTmp(t, "a.go", "package a\nvar x = 1\n")
	b := writeTmp(t, "b.go", "package b\nfunc F() {}\n")
	m.OpenOrSwitch(a)
	m.OpenOrSwitch(b)

	tn, ok := theme.ByName("tokyo-night")
	if !ok {
		t.Fatal("tokyo-night theme should be registered")
	}
	m.SetTheme(tn)

	if m.theme != tn {
		t.Errorf("manager.theme not swapped")
	}
	for i := 0; i < m.Count(); i++ {
		if got := m.At(i).Theme(); got != tn {
			t.Errorf("pane[%d] theme not propagated", i)
		}
	}
}

func TestSetThemeIsIdempotent(t *testing.T) {
	m := New(theme.Default)
	m.WithSize(40, 4)
	p := writeTmp(t, "a.go", "package a\n")
	m.OpenOrSwitch(p)

	tn, _ := theme.ByName("tokyo-night")
	m.SetTheme(tn)
	first := m.Active().Theme()
	m.SetTheme(tn) // same theme again
	second := m.Active().Theme()
	if first != second {
		t.Errorf("repeated SetTheme should produce identical pane theme")
	}
}
