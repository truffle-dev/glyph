package editor

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestSetThemeSwapsPalette(t *testing.T) {
	p := NewPane(theme.Default).WithSize(40, 4)
	p = p.InsertText("hello")

	// Default theme: capture rendered output.
	before := p.View()

	// Switch to TokyoNight: theme palette differs across nearly every
	// token (background, text, cursor, gutter), so the rendered ANSI
	// output should change even with the same buffer contents.
	tn, ok := theme.ByName("tokyo-night")
	if !ok {
		t.Fatal("tokyo-night theme should be registered")
	}
	p = p.SetTheme(tn)
	after := p.View()

	if before == after {
		t.Errorf("SetTheme should change rendered output\nbefore=%q\nafter=%q", before, after)
	}
	// Both renders should still contain the literal text under their styles.
	if !strings.Contains(plain(after), "hello") {
		t.Errorf("after SetTheme the literal text should still be visible, got %q", plain(after))
	}
}

func TestSetThemeIsValueReturning(t *testing.T) {
	// Documents the value-receiver idiom: SetTheme returns a new Pane and
	// does not mutate the receiver in place. Sanity-check by running it on
	// a temporary and confirming the original is untouched.
	p := NewPane(theme.Default).WithSize(20, 3)
	tn, _ := theme.ByName("catppuccin-mocha")
	_ = p.SetTheme(tn) // discard result
	// p.theme should still be Default. Best proxy: rendered output should
	// match a freshly-constructed pane.
	fresh := NewPane(theme.Default).WithSize(20, 3)
	if p.View() != fresh.View() {
		t.Errorf("SetTheme should not mutate receiver; got %q want %q", p.View(), fresh.View())
	}
}
