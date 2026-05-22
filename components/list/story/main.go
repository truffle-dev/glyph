//go:build glyph_story

package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/list"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	l := list.New(theme.Default).
		WithHeight(5).
		WithItems([]list.Item{
			{Label: "Inbox", Hint: "12 unread"},
			{Label: "Drafts"},
			{Label: "Sent"},
			{Label: "Spam", Disabled: true},
			{Label: "Archive"},
			{Label: "Trash"},
		})

	fmt.Println("initial:")
	fmt.Println(l.View())
	fmt.Println()

	// Walk the cursor down a few times.
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyDown})
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyDown})
	fmt.Println("after two downs (skips disabled):")
	fmt.Println(l.View())
	fmt.Println()

	// End jumps to last enabled.
	l, _ = l.Update(tea.KeyMsg{Type: tea.KeyEnd})
	fmt.Println("after End:")
	fmt.Println(l.View())
}
