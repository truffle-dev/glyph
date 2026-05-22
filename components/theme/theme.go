// Package theme provides the token palette every glyph component reads from.
//
// Components reference the Default theme; consumers replace tokens by editing
// this file or constructing their own Theme value.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme is the token palette shared across glyph components.
type Theme struct {
	// Foundational
	Bg            lipgloss.Color
	Surface       lipgloss.Color
	SurfaceStrong lipgloss.Color
	Border        lipgloss.Color
	BorderStrong  lipgloss.Color
	Text          lipgloss.Color
	TextMuted     lipgloss.Color
	TextInverse   lipgloss.Color

	// Accents
	Primary       lipgloss.Color
	PrimaryStrong lipgloss.Color
	Accent        lipgloss.Color

	// Status
	Success lipgloss.Color
	Warning lipgloss.Color
	Error   lipgloss.Color
	Info    lipgloss.Color
}
