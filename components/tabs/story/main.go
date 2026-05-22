//go:build glyph_story

package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/tabs"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	labels := []string{"chat", "commands", "logs", "diff"}

	for i := 0; i < len(labels); i++ {
		tb := tabs.New(theme.Default).WithTabs(labels).WithActive(i)
		fmt.Printf("active=%d  %s\n", i, tb.View())
	}
}
