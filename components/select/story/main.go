//go:build glyph_story

package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	selectinput "github.com/truffle-dev/glyph/components/select"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	models := []selectinput.Option{
		{Label: "claude-opus-4-7", Hint: "opus", Value: "opus-4-7"},
		{Label: "claude-opus-4-6", Hint: "opus", Value: "opus-4-6"},
		{Label: "claude-sonnet-4-6", Hint: "sonnet", Value: "sonnet-4-6"},
		{Label: "claude-sonnet-4-5", Hint: "sonnet", Value: "sonnet-4-5"},
		{Label: "claude-haiku-4-5", Hint: "haiku", Value: "haiku-4-5"},
		{Label: "gpt-5", Hint: "gpt", Value: "gpt-5"},
		{Label: "gpt-4o", Hint: "gpt", Value: "gpt-4o"},
	}

	empty := selectinput.New(theme.Default).
		WithSize(40, 5).
		WithTitle("Pick a model")

	defaults := selectinput.New(theme.Default).
		WithOptions(models).
		WithSize(40, 5).
		WithTitle("Pick a model").
		WithSelected(2)

	filtered := selectinput.New(theme.Default).
		WithOptions(models).
		WithSize(40, 5).
		WithTitle("Pick a model").
		WithFilter(true).
		WithPlaceholder("Type to filter…")
	for _, r := range []rune("opu") {
		filtered, _ = filtered.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	scenes := []struct {
		name string
		s    selectinput.Select
	}{
		{"empty", empty},
		{"default", defaults},
		{"filtered (typed 'opu', cursor on opus)", filtered},
	}
	for _, sc := range scenes {
		fmt.Printf("=== %s ===\n", sc.name)
		fmt.Println(sc.s.View())
		fmt.Println()
	}
}
