//go:build glyph_story

package main

import (
	"fmt"

	chatinput "github.com/truffle-dev/glyph/components/chat-input"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	empty := chatinput.New(theme.Default).
		WithPlaceholder("Ask anything…").
		WithWidth(70)

	typed := chatinput.New(theme.Default).
		WithPlaceholder("Ask anything…").
		WithWidth(70).
		WithValue("show me the diff between main and HEAD")

	blurred := chatinput.New(theme.Default).
		WithPlaceholder("Ask anything…").
		WithWidth(70).
		Blur()

	for _, label := range []struct {
		name  string
		input chatinput.Input
	}{
		{"empty (focused)", empty},
		{"typed", typed},
		{"blurred", blurred},
	} {
		fmt.Println(label.name)
		fmt.Println(label.input.View())
		fmt.Println()
	}
}
