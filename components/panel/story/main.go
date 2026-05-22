//go:build glyph_story

package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/panel"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	fmt.Println("default:")
	fmt.Println(panel.New(theme.Default).
		WithTitle("Logs").
		WithFooter("3 entries").
		WithContent("18:02:17  build  starting\n18:02:31  build  green").
		View())
	fmt.Println()

	fmt.Println("strong + fixed size:")
	fmt.Println(panel.New(theme.Default).
		WithTitle("Diff").
		WithVariant(panel.VariantStrong).
		WithWidth(40).
		WithHeight(7).
		WithContent("- old line\n+ new line\n  context").
		View())
	fmt.Println()

	fmt.Println("padded:")
	fmt.Println(panel.New(theme.Default).
		WithPadding(2, 1).
		WithContent("centered body").
		View())
}
