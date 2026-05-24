package welcome

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestView_RendersAllSignalsInFullSize(t *testing.T) {
	out := View(theme.Default, Info{
		Root:      "/Users/cheema/mcheema/specter",
		FileCount: 137,
		AI: Status{
			Label:  "claude CLI ready",
			OK:     true,
			Detail: "Haiku 4.5 inline · Sonnet 4.6 composer",
		},
		LSP: Status{
			Label:  "gopls available",
			OK:     true,
			Detail: "diagnostics live on any .go file",
		},
	}, 100, 30)

	stripped := stripANSI(out)
	wants := []string{
		"n  o  o  k",
		"a terminal-native AI IDE",
		"specter",
		"137",
		"claude CLI ready",
		"gopls available",
		"Quick start",
		"ctrl+p",
		"ctrl+f",
		"ctrl+k",
		"ctrl+l",
		"Press ctrl+p to find a file",
	}
	for _, w := range wants {
		if !strings.Contains(stripped, w) {
			t.Errorf("welcome view missing %q\n--- view ---\n%s", w, stripped)
		}
	}
}

func TestView_CompactFallbackUnderSmallTerminal(t *testing.T) {
	out := View(theme.Default, Info{
		Root:      "/tmp/x",
		FileCount: 3,
		AI:        ProbeAI(false),
		LSP:       ProbeLSP(),
	}, 40, 10)
	stripped := stripANSI(out)
	wants := []string{
		"nook",
		"ctrl+p open",
		"3 files",
	}
	for _, w := range wants {
		if !strings.Contains(stripped, w) {
			t.Errorf("compact welcome missing %q\n--- view ---\n%s", w, stripped)
		}
	}
	// And it must NOT include the full Quick start section.
	if strings.Contains(stripped, "Quick start") {
		t.Errorf("compact view should not include 'Quick start' header; got:\n%s", stripped)
	}
}

func TestProbeAI_OK(t *testing.T) {
	s := ProbeAI(true)
	if !s.OK || !strings.Contains(s.Label, "claude") {
		t.Errorf("ProbeAI(true) = %+v; want OK with claude label", s)
	}
}

func TestProbeAI_NotOK(t *testing.T) {
	s := ProbeAI(false)
	if s.OK {
		t.Errorf("ProbeAI(false).OK = true; want false")
	}
	if !strings.Contains(s.Detail, "npm i -g @anthropic-ai/claude-code") {
		t.Errorf("ProbeAI(false).Detail = %q; want install hint", s.Detail)
	}
}

func TestProbeLSP_AlwaysReturnsStatus(t *testing.T) {
	// gopls may or may not be on PATH in the test env; we just verify the
	// shape is sensible — non-empty label, and the install-hint detail is
	// present when OK is false.
	s := ProbeLSP()
	if s.Label == "" {
		t.Errorf("ProbeLSP().Label is empty; want a string")
	}
	if !s.OK && !strings.Contains(s.Detail, "go install") {
		t.Errorf("ProbeLSP() OK=false with no install hint: %+v", s)
	}
}

func TestTruncatePath_ShortPassthrough(t *testing.T) {
	got := truncatePath("/short", 20)
	if got != "/short" {
		t.Errorf("truncatePath kept short path = %q; want passthrough", got)
	}
}

func TestTruncatePath_LongTrimsFromHead(t *testing.T) {
	p := "/very/long/path/that/exceeds/the/max/and/keeps/going/and/going"
	got := truncatePath(p, 20)
	if n := len([]rune(got)); n != 20 {
		t.Errorf("truncatePath rune count = %d, want 20: %q", n, got)
	}
	if !strings.HasPrefix(got, "…") {
		t.Errorf("truncatePath should ellipsize head: %q", got)
	}
}

// stripANSI removes lipgloss color/style escape sequences so substring
// assertions can match plain text without the renderer's bytes in the way.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b { // ESC
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
