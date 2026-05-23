//go:build glyph_snap

package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	findbar "github.com/truffle-dev/glyph/components/find-bar"
	"github.com/truffle-dev/glyph/components/theme"
)

var sampleLines = []string{
	"package main",
	"",
	"import (",
	"\t\"fmt\"",
	"\t\"os\"",
	")",
	"",
	"// greet prints a hello message to stdout.",
	"func greet(name string) {",
	"\tif name == \"\" {",
	"\t\tfmt.Println(\"hello, world\")",
	"\t\treturn",
	"\t}",
	"\tfmt.Printf(\"hello, %s\\n\", name)",
	"}",
	"",
	"func main() {",
	"\tif len(os.Args) > 1 {",
	"\t\tgreet(os.Args[1])",
	"\t\treturn",
	"\t}",
	"\tgreet(\"\")",
	"}",
}

func main() {
	title := lipgloss.NewStyle().
		Foreground(theme.Default.Primary).
		Bold(true).
		Render("find-bar — in-buffer search overlay")

	bar := findbar.New(theme.Default).WithWidth(56)

	// Type "hello" rune-by-rune to populate the input cleanly.
	for _, r := range []rune("hello") {
		updated, _ := bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		bar = updated
	}
	matches := findbar.FindMatches(sampleLines, bar.Query(), bar.CaseSensitive())
	bar = bar.WithMatches(matches, 0)

	// Step forward one so the counter reads "2 / N" — more honest than "1 / N"
	// because the user has just navigated.
	updated, cmd := bar.Update(tea.KeyMsg{Type: tea.KeyEnter})
	bar = updated
	if cmd != nil {
		if _, ok := cmd().(findbar.NextMsg); ok {
			bar = bar.WithMatches(matches, 1)
		}
	}

	buf := lipgloss.NewStyle().
		Foreground(theme.Default.TextMuted).
		PaddingLeft(1).
		Render(strings.Join(sampleLines, "\n"))

	meta := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Render(
		fmt.Sprintf("query %q · match %d/%d", bar.Query(), bar.Current()+1, bar.MatchCount()))

	hint := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Render(
		"type to search · Enter next · Alt+Enter prev · Ctrl-U clear · Esc close")

	fmt.Println(strings.Join([]string{
		title, "",
		buf, "",
		bar.View(), "",
		meta, "",
		hint,
	}, "\n"))
}
