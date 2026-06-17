package themepicker

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

func names() []string { return []string{"alpha", "beta", "gamma"} }

func TestNewParksCursorOnCurrent(t *testing.T) {
	p := New(names(), "beta")
	if got := p.Cursor(); got != 1 {
		t.Fatalf("cursor = %d, want 1", got)
	}
	if got := p.Selected(); got != "beta" {
		t.Fatalf("selected = %q, want beta", got)
	}
}

func TestNewUnknownCurrentDefaultsToTop(t *testing.T) {
	p := New(names(), "does-not-exist")
	if got := p.Cursor(); got != 0 {
		t.Fatalf("cursor = %d, want 0", got)
	}
}

func TestUpDownClamp(t *testing.T) {
	p := New(names(), "alpha")
	p = p.Up() // already at top, clamps
	if p.Cursor() != 0 {
		t.Fatalf("up at top moved cursor to %d", p.Cursor())
	}
	p = p.Down().Down()
	if p.Cursor() != 2 {
		t.Fatalf("two downs = %d, want 2", p.Cursor())
	}
	p = p.Down() // at bottom, clamps
	if p.Cursor() != 2 {
		t.Fatalf("down at bottom moved cursor to %d", p.Cursor())
	}
	if got := p.Selected(); got != "gamma" {
		t.Fatalf("selected = %q, want gamma", got)
	}
}

func TestSelectedEmptyList(t *testing.T) {
	p := New(nil, "")
	if got := p.Selected(); got != "" {
		t.Fatalf("selected on empty = %q, want empty", got)
	}
}

func TestViewListsEveryThemeAndMarksCursor(t *testing.T) {
	p := New(names(), "beta")
	out := View(theme.Default, 80, p)
	for _, n := range names() {
		if !strings.Contains(out, n) {
			t.Fatalf("view missing theme %q:\n%s", n, out)
		}
	}
	if !strings.Contains(out, "session only") {
		t.Fatalf("view missing session-only disclosure:\n%s", out)
	}
	if !strings.Contains(out, "> beta") {
		t.Fatalf("view missing cursor marker on beta:\n%s", out)
	}
}

func TestViewWithRealThemeNames(t *testing.T) {
	all := theme.Names()
	p := New(all, all[0])
	out := View(theme.Default, 80, p)
	for _, n := range all {
		if !strings.Contains(out, n) {
			t.Fatalf("view missing real theme %q:\n%s", n, out)
		}
	}
}
