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

func TestFilter_EmptyQueryReturnsAll(t *testing.T) {
	secs := Default()
	for _, q := range []string{"", "   ", "\t"} {
		got := Filter(secs, q)
		if len(got) != len(secs) {
			t.Errorf("Filter(%q) returned %d sections, want all %d", q, len(got), len(secs))
		}
	}
}

func TestFilter_MatchesDescriptionCaseInsensitive(t *testing.T) {
	got := Filter(Default(), "RENAME")
	total := 0
	for _, sec := range got {
		for _, b := range sec.Bindings {
			total++
			hay := strings.ToLower(b.Key + " " + b.Desc)
			if !strings.Contains(hay, "rename") {
				t.Errorf("Filter kept non-matching binding %q / %q", b.Key, b.Desc)
			}
		}
	}
	if total == 0 {
		t.Fatal("Filter(\"RENAME\") found nothing; expected the f2 rename binding")
	}
}

func TestFilter_MatchesKey(t *testing.T) {
	got := Filter(Default(), "ctrl+g")
	found := false
	for _, sec := range got {
		for _, b := range sec.Bindings {
			if b.Key == "ctrl+g" {
				found = true
			}
		}
	}
	if !found {
		t.Error("Filter(\"ctrl+g\") did not surface the ctrl+g binding")
	}
}

func TestFilter_TokenAndOrderIndependent(t *testing.T) {
	// "git toggle" must find "Toggle git pane" even though the words are
	// in the opposite order in the description.
	got := Filter(Default(), "git toggle")
	found := false
	for _, sec := range got {
		for _, b := range sec.Bindings {
			if strings.Contains(strings.ToLower(b.Desc), "toggle git pane") {
				found = true
			}
		}
	}
	if !found {
		t.Error("token-AND filter failed: \"git toggle\" should match \"Toggle git pane\"")
	}
}

func TestFilter_DropsEmptySections(t *testing.T) {
	got := Filter(Default(), "zzz-no-such-binding")
	if len(got) != 0 {
		t.Errorf("Filter on an impossible query returned %d sections, want 0", len(got))
	}
}

func TestViewQuery_NoMatchStateRendersMessage(t *testing.T) {
	out := stripANSI(ViewQuery(theme.Default, 100, "zzqqxx"))
	if !strings.Contains(out, "no binding matches") {
		t.Errorf("filtered view with no matches missing the empty-state line\n--- view ---\n%s", out)
	}
}

func TestViewQuery_ShowsMatchCountAndQuery(t *testing.T) {
	out := stripANSI(ViewQuery(theme.Default, 100, "rename"))
	if !strings.Contains(out, "search: rename") {
		t.Errorf("filtered view missing the search line for the query\n--- view ---\n%s", out)
	}
	if strings.Contains(out, "press ? or esc to dismiss") {
		t.Errorf("filtered view should swap the dismiss hint for the search line")
	}
}

func TestViewQuery_EmptyQueryMatchesView(t *testing.T) {
	if ViewQuery(theme.Default, 100, "") != View(theme.Default, 100) {
		t.Error("ViewQuery with an empty query should be identical to View")
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
