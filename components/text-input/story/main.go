//go:build glyph_story

package main

import (
	"fmt"

	textinput "github.com/truffle-dev/glyph/components/text-input"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	empty := textinput.New(theme.Default).
		WithPlaceholder("Write a commit message…").
		WithWidth(70).
		WithHeight(4)

	multi := textinput.New(theme.Default).
		WithPlaceholder("Write a commit message…").
		WithWidth(70).
		WithHeight(4).
		WithValue("Tighten the registry validator\n\nReject manifests whose files[].target escapes the alias root.")

	blurred := textinput.New(theme.Default).
		WithPlaceholder("Write a commit message…").
		WithWidth(70).
		WithHeight(4).
		WithValue("draft saved").
		Blur()

	cases := []struct {
		name  string
		input textinput.Input
	}{
		{"empty (focused, cursor + placeholder)", empty},
		{"multi-line (focused, cursor at end)", multi},
		{"blurred (no cursor)", blurred},
	}

	for _, c := range cases {
		fmt.Println(c.name)
		fmt.Println(c.input.View())
		fmt.Println()
	}
}
