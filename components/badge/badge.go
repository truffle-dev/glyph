// Package badge renders compact status labels — the small colored
// pills that mark a row "LIVE", a build "PASS", a release "BETA", or a
// count "3 NEW". It is the atom that gives a status its weight without
// stealing a whole line.
//
// The package is value-based: there is no Model, no Update, no message.
// A Badge is an immutable value built up with chained options and
// turned into a string with Render, which callers place inside their
// own View wherever a label belongs. Colors flow from the shared theme
// so the atom drops into any parent surface without coordination.
//
// A badge carries one of six variants (Neutral, Primary, Success,
// Warning, Error, Info) and one of two appearances. Filled is the
// default: the variant color fills the background and the label sits
// on it in the theme's inverse text color. Outline draws a rounded
// border in the variant color with the label tinted to match and no
// fill, for a lighter touch that composes onto busy surfaces. Empty
// labels render as the empty string so callers can pass conditional
// badges without filtering at the call site.
package badge

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Variant selects the semantic color a badge carries.
type Variant int

const (
	// Neutral is a quiet, themeless label (muted border / surface
	// fill). It is the zero value, so badge.New("x").Render() is
	// neutral.
	Neutral Variant = iota
	// Primary marks the dominant accent action or state.
	Primary
	// Success marks a healthy, passing, or completed state.
	Success
	// Warning marks a degraded or attention-needed state.
	Warning
	// Error marks a failed or blocking state.
	Error
	// Info marks a neutral-informational state.
	Info
)

// Badge is an immutable status-label value. Build it with New and the
// chained options, then call Render.
type Badge struct {
	th        theme.Theme
	label     string
	variant   Variant
	outline   bool
	uppercase bool
}

// New constructs a filled, Neutral badge carrying label, using the
// Default theme.
func New(label string) Badge {
	return Badge{th: theme.Default, label: label}
}

// WithTheme overrides the theme palette.
func (b Badge) WithTheme(t theme.Theme) Badge { b.th = t; return b }

// WithVariant sets the semantic variant directly.
func (b Badge) WithVariant(v Variant) Badge { b.variant = v; return b }

// Neutral, Primary, Success, Warning, Error, and Info are convenience
// setters that read at the call site like badge.New("LIVE").Success().
func (b Badge) Neutral() Badge { b.variant = Neutral; return b }

// Primary sets the Primary variant.
func (b Badge) Primary() Badge { b.variant = Primary; return b }

// Success sets the Success variant.
func (b Badge) Success() Badge { b.variant = Success; return b }

// Warning sets the Warning variant.
func (b Badge) Warning() Badge { b.variant = Warning; return b }

// Error sets the Error variant.
func (b Badge) Error() Badge { b.variant = Error; return b }

// Info sets the Info variant.
func (b Badge) Info() Badge { b.variant = Info; return b }

// Outline switches the badge to the bordered, no-fill appearance.
func (b Badge) Outline() Badge { b.outline = true; return b }

// Filled switches the badge back to the default filled appearance.
func (b Badge) Filled() Badge { b.outline = false; return b }

// Uppercase folds the label to upper case at render time, the common
// shape for status pills ("live" -> "LIVE").
func (b Badge) Uppercase() Badge { b.uppercase = true; return b }

// Label returns the badge's current label, before any case folding.
func (b Badge) Label() string { return b.label }

// Render produces the styled pill. An empty label renders as "".
func (b Badge) Render() string {
	if b.label == "" {
		return ""
	}
	text := b.label
	if b.uppercase {
		text = strings.ToUpper(text)
	}
	accent := b.accent()

	if b.outline {
		return lipgloss.NewStyle().
			Foreground(accent).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1).
			Render(text)
	}

	fg := b.th.TextInverse
	if fg == "" {
		fg = b.th.Bg
	}
	st := lipgloss.NewStyle().
		Foreground(fg).
		Background(accent).
		Bold(true).
		Padding(0, 1)
	// Neutral has no status color; render it as a quiet surface chip
	// with normal text rather than inverse-on-accent.
	if b.variant == Neutral {
		st = lipgloss.NewStyle().
			Foreground(b.th.Text).
			Background(b.th.Surface).
			Padding(0, 1)
	}
	return st.Render(text)
}

// accent resolves the variant to its theme color. Neutral falls back
// to the muted border so outline-neutral reads as a quiet chip.
func (b Badge) accent() lipgloss.Color {
	switch b.variant {
	case Primary:
		return b.th.Primary
	case Success:
		return b.th.Success
	case Warning:
		return b.th.Warning
	case Error:
		return b.th.Error
	case Info:
		return b.th.Info
	default:
		return b.th.Border
	}
}
