package help

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestDefault_CoversAllRoutedKeys(t *testing.T) {
	secs := Default()
	flat := map[string]bool{}
	for _, sec := range secs {
		for _, b := range sec.Bindings {
			flat[b.Key] = true
		}
	}
	mustHave := []string{
		"ctrl+p", "ctrl+f", "ctrl+h", "alt+f", "ctrl+s", "alt+s", "alt+shift+s", "ctrl+w",
		"alt+]", "alt+[",
		"ctrl+k", "ctrl+l",
		"alt+i", "ctrl+]", "ctrl+space", "alt+enter", "f2", "alt+u", "alt+k", "alt+K", "ctrl+t", "ctrl+\\", "(",
		"ctrl+b", "ctrl+g", "ctrl+`",
		"ctrl+/",
		"ctrl+q", "?",
	}
	for _, k := range mustHave {
		if !flat[k] {
			t.Errorf("help keymap missing %q (Default() must cover every routed key)", k)
		}
	}
}

func TestView_ContainsAllSectionHeaders(t *testing.T) {
	out := View(theme.Default, 100)
	stripped := stripANSI(out)
	for _, sec := range Default() {
		if !strings.Contains(stripped, sec.Name) {
			t.Errorf("help view missing section header %q\n--- view ---\n%s", sec.Name, stripped)
		}
	}
	if !strings.Contains(stripped, "press ? or esc to dismiss") {
		t.Errorf("help view missing dismiss hint\n--- view ---\n%s", stripped)
	}
}

func TestView_ClampsToNarrowTerminal(t *testing.T) {
	// Should render at a usable size even when the terminal is narrow.
	out := View(theme.Default, 40)
	if !strings.Contains(stripANSI(out), "nook keymap") {
		t.Errorf("narrow help view dropped title")
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' || r == 'J' || r == 'H' || r == 'K' {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
