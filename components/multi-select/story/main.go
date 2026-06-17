//go:build glyph_story

package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	multiselect "github.com/truffle-dev/glyph/components/multi-select"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	files := []multiselect.Option{
		{Label: "src/main.go", Hint: "modified", Value: "main"},
		{Label: "src/util.go", Hint: "modified", Value: "util"},
		{Label: "src/parse.go", Hint: "new", Value: "parse"},
		{Label: "README.md", Hint: "modified", Value: "readme"},
		{Label: "go.mod", Hint: "modified", Value: "mod"},
		{Label: "go.sum", Hint: "modified", Value: "sum"},
	}

	empty := multiselect.New(theme.Default).
		WithSize(46, 6).
		WithTitle("Stage files")

	some := multiselect.New(theme.Default).
		WithOptions(files).
		WithSize(46, 6).
		WithTitle("Stage files").
		WithChecked([]string{"main", "parse", "readme"})

	filtered := multiselect.New(theme.Default).
		WithOptions(files).
		WithSize(46, 6).
		WithTitle("Stage files").
		WithChecked([]string{"main"}).
		WithFilter(true).
		WithPlaceholder("Type to filter…")
	for _, r := range []rune("go.") {
		filtered, _ = filtered.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	scenes := []struct {
		name string
		m    multiselect.MultiSelect
	}{
		{"empty", empty},
		{"three checked", some},
		{"filtered 'go.', one check carried in", filtered},
	}
	for _, sc := range scenes {
		fmt.Printf("=== %s ===\n", sc.name)
		fmt.Println(sc.m.View())
		fmt.Println()
	}
}
