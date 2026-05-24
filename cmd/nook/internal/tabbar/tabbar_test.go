package tabbar

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/cmd/nook/internal/bufman"
	"github.com/truffle-dev/glyph/components/theme"
)

func plain(s string) string {
	// Render with no styles by stripping ANSI via lipgloss's Width-known plain
	// text accessor isn't available; use a simple heuristic: lipgloss escapes
	// are ESC[...m. Strip them by scanning.
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func TestViewEmptyReturnsEmpty(t *testing.T) {
	if got := View(theme.Default, nil, -1, 80); got != "" {
		t.Errorf("empty tabs → %q, want \"\"", got)
	}
	if got := View(theme.Default, []bufman.TabInfo{{Path: "a.go"}}, 0, 0); got != "" {
		t.Errorf("zero width → %q, want \"\"", got)
	}
}

func TestSingleTabRenders(t *testing.T) {
	tabs := []bufman.TabInfo{{Path: "/repo/main.go"}}
	out := View(theme.Default, tabs, 0, 80)
	if !strings.Contains(plain(out), "main.go") {
		t.Errorf("single-tab output missing basename: %q", plain(out))
	}
}

func TestDirtyMarkerShown(t *testing.T) {
	tabs := []bufman.TabInfo{{Path: "/repo/a.go", Dirty: true}}
	out := plain(View(theme.Default, tabs, 0, 80))
	if !strings.Contains(out, "●") {
		t.Errorf("dirty buffer should show ●, got %q", out)
	}
}

func TestBasenameDedup(t *testing.T) {
	// Two files named "main.go" in different dirs should show parent.
	tabs := []bufman.TabInfo{
		{Path: "/repo/cmd/a/main.go"},
		{Path: "/repo/cmd/b/main.go"},
	}
	out := plain(View(theme.Default, tabs, 0, 200))
	if !strings.Contains(out, "a/main.go") {
		t.Errorf("dedup should show a/main.go, got %q", out)
	}
	if !strings.Contains(out, "b/main.go") {
		t.Errorf("dedup should show b/main.go, got %q", out)
	}
}

func TestNoDedupWhenUnique(t *testing.T) {
	tabs := []bufman.TabInfo{
		{Path: "/repo/cmd/a/main.go"},
		{Path: "/repo/lib/helper.go"},
	}
	out := plain(View(theme.Default, tabs, 0, 200))
	// Should just be basenames.
	if strings.Contains(out, "a/main.go") {
		t.Errorf("unique basenames should not dedup, got %q", out)
	}
	if !strings.Contains(out, "main.go") || !strings.Contains(out, "helper.go") {
		t.Errorf("missing one or both basenames in %q", out)
	}
}

func TestOverflowKeepsActiveVisible(t *testing.T) {
	tabs := make([]bufman.TabInfo, 10)
	for i := range tabs {
		// Distinct basenames so dedup doesn't kick in.
		tabs[i].Path = "/repo/file" + string(rune('a'+i)) + ".go"
	}
	active := 7
	// Narrow width forces overflow.
	out := plain(View(theme.Default, tabs, active, 40))
	if !strings.Contains(out, "fileh.go") {
		t.Errorf("active basename (fileh.go) missing in %q", out)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("overflow indicator missing in %q", out)
	}
}

func TestOverflowFitsWithinWidth(t *testing.T) {
	tabs := make([]bufman.TabInfo, 20)
	for i := range tabs {
		tabs[i].Path = "/repo/longername" + string(rune('a'+i)) + ".go"
	}
	width := 50
	out := View(theme.Default, tabs, 5, width)
	if w := lipgloss.Width(out); w != width {
		t.Errorf("rendered width = %d, want %d", w, width)
	}
}
