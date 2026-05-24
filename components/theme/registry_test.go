package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestByNameKnownTheme(t *testing.T) {
	got, ok := ByName("default")
	if !ok {
		t.Fatal("expected default to be registered")
	}
	if got.Bg != Default.Bg {
		t.Errorf("ByName(default).Bg = %q, want %q", got.Bg, Default.Bg)
	}
}

func TestByNameUnknown(t *testing.T) {
	_, ok := ByName("does-not-exist")
	if ok {
		t.Fatal("expected unknown theme name to return ok=false")
	}
}

func TestByNameAllRegistered(t *testing.T) {
	for _, name := range []string{"default", "light", "tokyo-night", "catppuccin-mocha", "rose-pine"} {
		t.Run(name, func(t *testing.T) {
			tt, ok := ByName(name)
			if !ok {
				t.Fatalf("expected %s to be registered", name)
			}
			if tt.Bg == lipgloss.Color("") {
				t.Errorf("%s has empty Bg token", name)
			}
			if tt.Text == lipgloss.Color("") {
				t.Errorf("%s has empty Text token", name)
			}
			if tt.SyntaxKeyword == lipgloss.Color("") {
				t.Errorf("%s has empty SyntaxKeyword token", name)
			}
		})
	}
}

func TestNamesSorted(t *testing.T) {
	names := Names()
	if len(names) < 5 {
		t.Fatalf("Names() = %d entries, expected at least 5", len(names))
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Errorf("Names() not sorted: %q before %q", names[i-1], names[i])
		}
	}
	want := map[string]bool{"default": true, "light": true, "tokyo-night": true, "catppuccin-mocha": true, "rose-pine": true}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected theme %q in registry", n)
		}
		delete(want, n)
	}
	for missing := range want {
		t.Errorf("missing theme %q from registry", missing)
	}
}

func TestThemePalettesDistinct(t *testing.T) {
	// Spot-check: every registered theme's Bg should differ from Default's, except
	// Default itself. Catches a copy-paste regression where a new theme accidentally
	// inherits Default's tokens.
	for _, name := range []string{"light", "tokyo-night", "catppuccin-mocha", "rose-pine"} {
		t.Run(name, func(t *testing.T) {
			tt, _ := ByName(name)
			if tt.Bg == Default.Bg {
				t.Errorf("%s.Bg == Default.Bg (%q); palettes should be distinct", name, tt.Bg)
			}
		})
	}
}
