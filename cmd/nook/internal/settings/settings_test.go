package settings

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/cmd/nook/internal/config"
	"github.com/truffle-dev/glyph/components/theme"
)

// stripStyle removes ANSI escape sequences so assertions match on the
// rendered text rather than the lipgloss color codes wrapping it.
func stripStyle(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEsc = true
		case inEsc && r == 'm':
			inEsc = false
		case inEsc:
			// drop
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestViewRendersEveryEffectiveValue(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Editor: config.EditorConfig{
		Theme:        "rosepine",
		TabWidth:     2,
		FormatOnSave: false,
		LineNumbers:  true,
		IndentGuides: false,
		InlayHints:   true,
		SoftWrap:     true,
	}}
	out := stripStyle(View(theme.Default, 100, cfg, "/u/config.toml", "/p/.nook/config.toml", true, true))

	wants := []string{
		"nook settings",
		"theme", "rosepine",
		"tab_width", "2",
		"format_on_save", "false",
		"line_numbers", "true",
		"indent_guides", "false",
		"inlay_hints", "true",
		"soft_wrap", "true",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("settings view missing %q\n---\n%s", w, out)
		}
	}
}

func TestViewShowsDismissHint(t *testing.T) {
	t.Parallel()
	out := stripStyle(View(theme.Default, 100, config.Default(), "/u", "/p", false, false))
	if !strings.Contains(out, "alt+.") || !strings.Contains(out, "esc") {
		t.Errorf("expected dismiss hint naming alt+. and esc, got:\n%s", out)
	}
}

func TestViewScopeFooterReflectsPresence(t *testing.T) {
	t.Parallel()
	// User file present, project file absent.
	out := stripStyle(View(theme.Default, 100, config.Default(), "/home/u/config.toml", "/repo/.nook/config.toml", true, false))
	if !strings.Contains(out, "user config: /home/u/config.toml (loaded)") {
		t.Errorf("expected user scope shown as loaded, got:\n%s", out)
	}
	if !strings.Contains(out, "project config: /repo/.nook/config.toml (not present)") {
		t.Errorf("expected project scope shown as not present, got:\n%s", out)
	}
}

func TestViewUnsetPathRendersUnset(t *testing.T) {
	t.Parallel()
	out := stripStyle(View(theme.Default, 100, config.Default(), "", "/p", false, false))
	if !strings.Contains(out, "user config: (unset)") {
		t.Errorf("expected unset user path, got:\n%s", out)
	}
}

func TestViewNarrowWidthDoesNotPanic(t *testing.T) {
	t.Parallel()
	// A pathologically small terminal must still render a clamped card.
	out := View(theme.Default, 10, config.Default(), "/u", "/p", true, true)
	if out == "" {
		t.Fatal("expected a rendered card even at width 10")
	}
}

func TestOnOffMatchesTomlVocabulary(t *testing.T) {
	t.Parallel()
	if onOff(true) != "true" || onOff(false) != "false" {
		t.Errorf("onOff should mirror TOML bool literals, got %q/%q", onOff(true), onOff(false))
	}
}
