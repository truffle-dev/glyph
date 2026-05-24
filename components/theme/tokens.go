package theme

import "github.com/charmbracelet/lipgloss"

// Default is the dark terminal theme. Edit this file to retheme the entire app.
var Default = Theme{
	Bg:            lipgloss.Color("#0e0e10"),
	Surface:       lipgloss.Color("#16171b"),
	SurfaceStrong: lipgloss.Color("#1f2026"),
	Border:        lipgloss.Color("#2a2c34"),
	BorderStrong:  lipgloss.Color("#3a3c46"),
	Text:          lipgloss.Color("#e6e6ea"),
	TextMuted:     lipgloss.Color("#9a9aa6"),
	TextInverse:   lipgloss.Color("#0e0e10"),

	Primary:       lipgloss.Color("#5e6ad2"),
	PrimaryStrong: lipgloss.Color("#7a86e6"),
	Accent:        lipgloss.Color("#d57e5e"),

	Success: lipgloss.Color("#41b883"),
	Warning: lipgloss.Color("#d8a83c"),
	Error:   lipgloss.Color("#e25555"),
	Info:    lipgloss.Color("#5e9bd2"),

	SyntaxKeyword:     lipgloss.Color("#c586c0"),
	SyntaxString:      lipgloss.Color("#ce9178"),
	SyntaxComment:     lipgloss.Color("#6a7080"),
	SyntaxNumber:      lipgloss.Color("#b5cea8"),
	SyntaxFunction:    lipgloss.Color("#dcdcaa"),
	SyntaxType:        lipgloss.Color("#4ec9b0"),
	SyntaxPunctuation: lipgloss.Color("#9a9aa6"),
}

// Light is the light-paper theme. Most components target Default; Light is a
// drop-in replacement when the consumer's terminal background is bright.
var Light = Theme{
	Bg:            lipgloss.Color("#fbf6ec"),
	Surface:       lipgloss.Color("#f4ecdb"),
	SurfaceStrong: lipgloss.Color("#e8dcc1"),
	Border:        lipgloss.Color("#d6c8a8"),
	BorderStrong:  lipgloss.Color("#a6987a"),
	Text:          lipgloss.Color("#2a261f"),
	TextMuted:     lipgloss.Color("#7d7460"),
	TextInverse:   lipgloss.Color("#fbf6ec"),

	Primary:       lipgloss.Color("#5e6ad2"),
	PrimaryStrong: lipgloss.Color("#7a86e6"),
	Accent:        lipgloss.Color("#d57e5e"),

	Success: lipgloss.Color("#41b883"),
	Warning: lipgloss.Color("#d8a83c"),
	Error:   lipgloss.Color("#e25555"),
	Info:    lipgloss.Color("#5e9bd2"),

	SyntaxKeyword:     lipgloss.Color("#a040a0"),
	SyntaxString:      lipgloss.Color("#a5562a"),
	SyntaxComment:     lipgloss.Color("#8a7d65"),
	SyntaxNumber:      lipgloss.Color("#4f7a23"),
	SyntaxFunction:    lipgloss.Color("#7a5e0c"),
	SyntaxType:        lipgloss.Color("#0e7a6a"),
	SyntaxPunctuation: lipgloss.Color("#7d7460"),
}
