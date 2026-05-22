//go:build glyph_story

package main

import (
	"fmt"

	keyhints "github.com/truffle-dev/glyph/components/key-hints"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	bar := keyhints.New(theme.Default).WithHints([]keyhints.Hint{
		{Key: "Tab", Desc: "next pane"},
		{Key: "Shift+Tab", Desc: "prev pane"},
		{Key: "/", Desc: "filter"},
		{Key: "q", Desc: "quit"},
	})
	fmt.Println(bar.View())

	fmt.Println()
	fmt.Println("custom separator:")
	fmt.Println(bar.WithSeparator(" · ").View())

	fmt.Println()
	fmt.Println("clamped to 28 columns:")
	fmt.Println(bar.WithWidth(28).View())
}
