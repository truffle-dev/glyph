//go:build glyph_story

package main

import (
	"fmt"

	chatbubble "github.com/truffle-dev/glyph/components/chat-bubble"
	chatthread "github.com/truffle-dev/glyph/components/chat-thread"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	th := chatthread.New(theme.Default).WithSize(72, 24).WithMessages([]chatthread.Message{
		{Role: chatbubble.RoleSystem, Label: "system", Text: "Session started at 09:42. Model: gpt-5-haiku."},
		{Role: chatbubble.RoleUser, Label: "you", Text: "What changed between v1.2 and v1.3?"},
		{Role: chatbubble.RoleAssistant, Label: "glyph", Text: "v1.3 added the chat-thread component and a viewport-aware scroll model. Two breaking changes: chatbubble.Bubble.View() now requires WithWidth to be set explicitly, and chatinput.SubmitMsg replaces the older OnSubmit callback."},
		{Role: chatbubble.RoleTool, Label: "tool · grep", Text: "matched 14 occurrences across 6 files"},
		{Role: chatbubble.RoleAssistant, Label: "glyph", Text: "I can show you the diff if you want. Say the word."},
	})

	fmt.Println(th.View())
}
