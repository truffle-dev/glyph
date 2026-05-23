// Package kbd renders single keystrokes and chords as terminal-styled
// keycap glyphs. It is the atom that turns "ctrl+k" into a readable
// ⌃ K inside hint rows, command palettes, and modals.
//
// The package is stateless: there is no Model, no Update, no message.
// Callers invoke Render, Chord, or Sequence from inside their own View
// and place the resulting string wherever a keycap belongs. Colors
// flow from the shared theme so the atom drops into any parent surface
// without coordination.
package kbd

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// ChordSeparator is placed between caps inside a single chord.
const ChordSeparator = " + "

// SequenceSeparator is placed between chords in a keystroke series
// like "g g" or "Ctrl+K then P".
const SequenceSeparator = " , "

// glyphFor maps a case-folded key name to its Unicode keycap glyph.
// Names not in the table fall through to a label-style render.
var glyphFor = map[string]string{
	"ctrl":      "⌃",
	"control":   "⌃",
	"shift":     "⇧",
	"alt":       "⌥",
	"option":    "⌥",
	"opt":       "⌥",
	"cmd":       "⌘",
	"command":   "⌘",
	"meta":      "⌘",
	"super":     "⌘",
	"win":       "⌘",
	"enter":     "⏎",
	"return":    "⏎",
	"tab":       "⇥",
	"esc":       "⎋",
	"escape":    "⎋",
	"space":     "␣",
	"spacebar":  "␣",
	"backspace": "⌫",
	"delete":    "⌦",
	"del":       "⌦",
	"up":        "↑",
	"down":      "↓",
	"left":      "←",
	"right":     "→",
	"pageup":    "⇞",
	"pgup":      "⇞",
	"pagedown":  "⇟",
	"pgdn":      "⇟",
	"home":      "⤒",
	"end":       "⤓",
}

// Style overrides the per-cap colors. Zero values fall back to the
// theme palette: Foreground -> Text, Border -> Border. Background
// defaults to unset (transparent) so the cap composes onto any
// parent surface.
type Style struct {
	Foreground lipgloss.Color
	Background lipgloss.Color
	Border     lipgloss.Color
}

// Render formats a single key as a styled keycap using the default
// theme. Returns "" when key is empty.
func Render(key string) string {
	return RenderStyled(key, Style{})
}

// RenderStyled formats a single key with explicit style overrides.
// Any zero-value field in s falls back to the default theme palette.
func RenderStyled(key string, s Style) string {
	if key == "" {
		return ""
	}
	label := glyph(key)
	return capStyle(s).Render(label)
}

// Chord renders a sequence of keys joined with ChordSeparator. Empty
// keys are skipped. Returns "" when no usable keys are passed.
func Chord(keys ...string) string {
	caps := make([]string, 0, len(keys))
	for _, k := range keys {
		c := Render(k)
		if c == "" {
			continue
		}
		caps = append(caps, c)
	}
	if len(caps) == 0 {
		return ""
	}
	return strings.Join(caps, ChordSeparator)
}

// Sequence joins already-rendered chord strings with SequenceSeparator.
// Empty entries are skipped so callers can pass conditional chords
// without filtering at the call site.
func Sequence(chords ...string) string {
	parts := make([]string, 0, len(chords))
	for _, c := range chords {
		if c == "" {
			continue
		}
		parts = append(parts, c)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, SequenceSeparator)
}

// glyph resolves a key name to its display label. Mapped names return
// the Unicode keycap; unmapped names pass through with the first rune
// uppercased so single letters render as "K" not "k".
func glyph(key string) string {
	if g, ok := glyphFor[strings.ToLower(key)]; ok {
		return g
	}
	// Pass-through: capitalize the first rune so "k" → "K" but
	// multi-character words like "F1" or "PrtSc" keep their shape.
	if len(key) == 0 {
		return key
	}
	r := []rune(key)
	if len(r) == 1 {
		return strings.ToUpper(string(r))
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// capStyle builds the lipgloss style for one keycap given an override.
// Falls back to theme.Default for any zero-value field in s.
func capStyle(s Style) lipgloss.Style {
	fg := s.Foreground
	if fg == "" {
		fg = theme.Default.Text
	}
	border := s.Border
	if border == "" {
		border = theme.Default.Border
	}
	st := lipgloss.NewStyle().
		Foreground(fg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1)
	if s.Background != "" {
		st = st.Background(s.Background)
	}
	return st
}
