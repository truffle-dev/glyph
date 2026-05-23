package kbd

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	// Force TrueColor so lipgloss emits ANSI escapes deterministically
	// regardless of whether the test process is attached to a TTY.
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func TestRenderSingleKeyContainsLabel(t *testing.T) {
	out := Render("K")
	if out == "" {
		t.Fatal("Render(\"K\") returned empty")
	}
	if !strings.Contains(out, "K") {
		t.Fatalf("Render(\"K\") output missing label, got %q", out)
	}
}

func TestRenderSubstitutesCtrlGlyph(t *testing.T) {
	out := Render("ctrl")
	if !strings.Contains(out, "⌃") {
		t.Fatalf("Render(\"ctrl\") should contain ⌃, got %q", out)
	}
	if strings.Contains(out, "ctrl") || strings.Contains(out, "Ctrl") {
		t.Fatalf("Render(\"ctrl\") should not contain the raw word, got %q", out)
	}
}

func TestChordContainsBothCapsAndSeparator(t *testing.T) {
	out := Chord("Ctrl", "K")
	if !strings.Contains(out, "⌃") {
		t.Errorf("Chord should contain ⌃ for Ctrl, got %q", out)
	}
	if !strings.Contains(out, "K") {
		t.Errorf("Chord should contain K, got %q", out)
	}
	if !strings.Contains(out, ChordSeparator) {
		t.Errorf("Chord should contain separator %q, got %q", ChordSeparator, out)
	}
}

func TestSequenceJoinsChordsWithComma(t *testing.T) {
	out := Sequence(Chord("g"), Chord("g"))
	gCount := strings.Count(out, "G")
	if gCount < 2 {
		t.Errorf("Sequence(Chord(g), Chord(g)) should contain two G's, got %d in %q", gCount, out)
	}
	if !strings.Contains(out, SequenceSeparator) {
		t.Errorf("Sequence should include %q, got %q", SequenceSeparator, out)
	}
}

func TestRenderStyledForegroundAppearsInANSI(t *testing.T) {
	// Pick a color whose ANSI representation is stable across lipgloss
	// renderers. lipgloss may emit truecolor or 256-color depending on
	// terminal detection at test time, so we accept either form by
	// searching for the hex digits in the output.
	red := lipgloss.Color("#ff0000")
	out := RenderStyled("K", Style{Foreground: red})
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("RenderStyled output should include an ANSI escape, got %q", out)
	}
	// Truecolor terminals encode #ff0000 as 255;0;0. Any reasonable
	// downgrade also keeps a red component, but the truecolor path is
	// what the test harness uses for deterministic output.
	if !strings.Contains(out, "255;0;0") && !strings.Contains(out, "ff0000") {
		t.Fatalf("RenderStyled with red foreground should encode the color in the ANSI stream, got %q", out)
	}
}

func TestRenderIsCaseInsensitive(t *testing.T) {
	a := Render("CTRL")
	b := Render("Ctrl")
	c := Render("ctrl")
	if a != b || b != c {
		t.Fatalf("Render must be case-insensitive: CTRL=%q Ctrl=%q ctrl=%q", a, b, c)
	}
}

func TestRenderEmptyStringReturnsEmpty(t *testing.T) {
	if got := Render(""); got != "" {
		t.Fatalf("Render(\"\") should return empty string, got %q", got)
	}
}

func TestChordSkipsEmptyEntries(t *testing.T) {
	out := Chord("", "K", "")
	if !strings.Contains(out, "K") {
		t.Fatalf("Chord should still render K when given empties, got %q", out)
	}
	if strings.Contains(out, ChordSeparator) {
		t.Fatalf("Chord with one usable key should not include separator, got %q", out)
	}
}

func TestSequenceSkipsEmptyEntries(t *testing.T) {
	if got := Sequence("", "", ""); got != "" {
		t.Fatalf("Sequence of empty strings should return empty, got %q", got)
	}
}

func TestArrowGlyphSubstitution(t *testing.T) {
	cases := map[string]string{
		"up":    "↑",
		"down":  "↓",
		"left":  "←",
		"right": "→",
	}
	for k, want := range cases {
		out := Render(k)
		if !strings.Contains(out, want) {
			t.Errorf("Render(%q) should contain %q, got %q", k, want, out)
		}
	}
}

func TestPassthroughCapitalizesFirstRune(t *testing.T) {
	out := Render("k")
	if !strings.Contains(out, "K") {
		t.Fatalf("Render(\"k\") should capitalize to K, got %q", out)
	}
}
