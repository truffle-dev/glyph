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

// TokyoNight is the dark-blue editor theme popularized by enfocado / Tokyo
// Night. Same shape as Default; consumers swap by reassigning the field.
// Palette adapted from folke/tokyonight.nvim's "night" variant.
var TokyoNight = Theme{
	Bg:            lipgloss.Color("#1a1b26"),
	Surface:       lipgloss.Color("#1f2335"),
	SurfaceStrong: lipgloss.Color("#292e42"),
	Border:        lipgloss.Color("#3b4261"),
	BorderStrong:  lipgloss.Color("#545c7e"),
	Text:          lipgloss.Color("#c0caf5"),
	TextMuted:     lipgloss.Color("#7986b3"),
	TextInverse:   lipgloss.Color("#1a1b26"),

	Primary:       lipgloss.Color("#7aa2f7"),
	PrimaryStrong: lipgloss.Color("#a4b8ff"),
	Accent:        lipgloss.Color("#bb9af7"),

	Success: lipgloss.Color("#9ece6a"),
	Warning: lipgloss.Color("#e0af68"),
	Error:   lipgloss.Color("#f7768e"),
	Info:    lipgloss.Color("#7dcfff"),

	SyntaxKeyword:     lipgloss.Color("#bb9af7"),
	SyntaxString:      lipgloss.Color("#9ece6a"),
	SyntaxComment:     lipgloss.Color("#565f89"),
	SyntaxNumber:      lipgloss.Color("#ff9e64"),
	SyntaxFunction:    lipgloss.Color("#7aa2f7"),
	SyntaxType:        lipgloss.Color("#2ac3de"),
	SyntaxPunctuation: lipgloss.Color("#a9b1d6"),
}

// CatppuccinMocha is the dark variant of the Catppuccin family. Warm, low-
// contrast palette popular in modern terminal editors. Adapted from the
// official catppuccin/palette catppuccin-mocha specification.
var CatppuccinMocha = Theme{
	Bg:            lipgloss.Color("#1e1e2e"),
	Surface:       lipgloss.Color("#181825"),
	SurfaceStrong: lipgloss.Color("#313244"),
	Border:        lipgloss.Color("#45475a"),
	BorderStrong:  lipgloss.Color("#585b70"),
	Text:          lipgloss.Color("#cdd6f4"),
	TextMuted:     lipgloss.Color("#a6adc8"),
	TextInverse:   lipgloss.Color("#1e1e2e"),

	Primary:       lipgloss.Color("#cba6f7"),
	PrimaryStrong: lipgloss.Color("#dabffe"),
	Accent:        lipgloss.Color("#f5c2e7"),

	Success: lipgloss.Color("#a6e3a1"),
	Warning: lipgloss.Color("#f9e2af"),
	Error:   lipgloss.Color("#f38ba8"),
	Info:    lipgloss.Color("#89dceb"),

	SyntaxKeyword:     lipgloss.Color("#cba6f7"),
	SyntaxString:      lipgloss.Color("#a6e3a1"),
	SyntaxComment:     lipgloss.Color("#7f849c"),
	SyntaxNumber:      lipgloss.Color("#fab387"),
	SyntaxFunction:    lipgloss.Color("#89b4fa"),
	SyntaxType:        lipgloss.Color("#f9e2af"),
	SyntaxPunctuation: lipgloss.Color("#bac2de"),
}

// RosePine is the soho-vibes dark theme. Muted pinks and teals against a deep
// surface. Adapted from rose-pine/rose-pine's "main" variant.
var RosePine = Theme{
	Bg:            lipgloss.Color("#191724"),
	Surface:       lipgloss.Color("#1f1d2e"),
	SurfaceStrong: lipgloss.Color("#26233a"),
	Border:        lipgloss.Color("#403d52"),
	BorderStrong:  lipgloss.Color("#524f67"),
	Text:          lipgloss.Color("#e0def4"),
	TextMuted:     lipgloss.Color("#908caa"),
	TextInverse:   lipgloss.Color("#191724"),

	Primary:       lipgloss.Color("#c4a7e7"),
	PrimaryStrong: lipgloss.Color("#dfc7f3"),
	Accent:        lipgloss.Color("#ebbcba"),

	Success: lipgloss.Color("#9ccfd8"),
	Warning: lipgloss.Color("#f6c177"),
	Error:   lipgloss.Color("#eb6f92"),
	Info:    lipgloss.Color("#31748f"),

	SyntaxKeyword:     lipgloss.Color("#c4a7e7"),
	SyntaxString:      lipgloss.Color("#f6c177"),
	SyntaxComment:     lipgloss.Color("#6e6a86"),
	SyntaxNumber:      lipgloss.Color("#eb6f92"),
	SyntaxFunction:    lipgloss.Color("#9ccfd8"),
	SyntaxType:        lipgloss.Color("#31748f"),
	SyntaxPunctuation: lipgloss.Color("#908caa"),
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
