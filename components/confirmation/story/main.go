//go:build glyph_story

package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/confirmation"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	scenes := []struct {
		name string
		c    confirmation.Confirm
	}{
		{
			name: "default (No focused, safe)",
			c: confirmation.New(theme.Default).
				WithPrompt("Discard unsaved changes?").
				WithWidth(50),
		},
		{
			name: "dangerous (Yes turns red, No focused for safety)",
			c: confirmation.New(theme.Default).
				WithPrompt("Delete the production database? This cannot be undone.").
				WithDangerous(true).
				WithWidth(50),
		},
		{
			name: "yes-default committed (Yes focused)",
			c: confirmation.New(theme.Default).
				WithPrompt("Save and exit?").
				WithDefault(true).
				WithWidth(50),
		},
	}

	for _, s := range scenes {
		fmt.Println(s.name)
		fmt.Println(s.c.View())
		fmt.Println()
	}
}
