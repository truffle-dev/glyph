package chatbubble

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestRendersTextOfAnyRole(t *testing.T) {
	for _, role := range []Role{RoleUser, RoleAssistant, RoleSystem, RoleTool} {
		out := New(theme.Default).WithRole(role).WithText("hello world").View()
		if !strings.Contains(out, "hello world") {
			t.Errorf("role %q: expected text in output, got %q", role, out)
		}
	}
}

func TestRespectsWidth(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog the quick brown fox jumps over the lazy dog"
	out := New(theme.Default).WithRole(RoleAssistant).WithText(text).WithWidth(30).View()
	for _, line := range strings.Split(stripANSI(out), "\n") {
		if visibleLen(line) > 32 { // width + border + padding margin
			t.Errorf("line %q exceeds width 30 (visible len %d)", line, visibleLen(line))
		}
	}
}

func TestLabelRendersAboveBubble(t *testing.T) {
	out := New(theme.Default).WithRole(RoleAssistant).WithText("hi").WithLabel("assistant").View()
	lines := strings.Split(stripANSI(out), "\n")
	if len(lines) < 2 || !strings.Contains(lines[0], "assistant") {
		t.Errorf("expected label on first line, got %q", out)
	}
}

func TestMinWidthClamped(t *testing.T) {
	out := New(theme.Default).WithText("x").WithWidth(1).View()
	if out == "" {
		t.Errorf("expected non-empty output even with sub-minimum width")
	}
}

// stripANSI removes ANSI escape sequences for plain-text length checks.
func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' || r == 'K' || r == 'J' || r == 'H' {
				inEsc = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func visibleLen(s string) int {
	return len([]rune(s))
}
