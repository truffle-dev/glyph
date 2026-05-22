//go:build glyph_story

package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/spinner"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	styles := []struct {
		name  string
		style spinner.Style
	}{
		{"dots", spinner.StyleDots},
		{"line", spinner.StyleLine},
		{"arc", spinner.StyleArc},
		{"pulse", spinner.StylePulse},
		{"bounce", spinner.StyleBounce},
	}

	for _, s := range styles {
		fmt.Println(s.name)
		sp := spinner.New(theme.Default).WithStyle(s.style).WithLabel("Working")
		// Print three consecutive frames for each style so the story file
		// captures motion in a static snapshot.
		for i := 0; i < 3; i++ {
			fmt.Println("  " + sp.View())
			sp, _ = sp.Update(spinner.TickMsg{ID: ""})
		}
		fmt.Println()
	}
}
