//go:build glyph_story

package main

import (
	"fmt"

	progressbar "github.com/truffle-dev/glyph/components/progress-bar"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	steps := []float64{0, 0.25, 0.5, 0.75, 1.0}
	for _, p := range steps {
		bar := progressbar.New(theme.Default).WithPercent(p).WithLabel("upload")
		fmt.Println(bar.View())
	}

	fmt.Println()
	fmt.Println("colored (success) at 0.75:")
	fmt.Println(progressbar.New(theme.Default).
		WithPercent(0.75).
		WithFillColor(theme.Default.Success).
		WithWidth(40).
		View())

	fmt.Println()
	fmt.Println("ASCII runes:")
	fmt.Println(progressbar.New(theme.Default).
		WithPercent(0.6).
		WithRunes("=", "-").
		WithWidth(20).
		View())
}
