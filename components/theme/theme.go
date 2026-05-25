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

	// Syntax (source-code highlighting palette). Components that don't render
	// source code can ignore these. Empty values fall back to Text or muted.
	SyntaxKeyword     lipgloss.Color
	SyntaxString      lipgloss.Color
	SyntaxComment     lipgloss.Color
	SyntaxNumber      lipgloss.Color
	SyntaxFunction    lipgloss.Color
	SyntaxType        lipgloss.Color
	SyntaxPunctuation lipgloss.Color

	// Semantic-token slots layered on top of the chroma palette. Populated by
	// language-server semanticTokens responses. Empty values fall back to a
	// reasonable underlying chroma token.
	SyntaxParameter  lipgloss.Color
	SyntaxProperty   lipgloss.Color
	SyntaxEnumMember lipgloss.Color
	SyntaxNamespace  lipgloss.Color
	SyntaxReadonly   lipgloss.Color
}
