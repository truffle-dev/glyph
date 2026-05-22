//go:build glyph_story

package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/chat-bubble"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	bubbles := []chatbubble.Bubble{
		chatbubble.New(theme.Default).
			WithRole(chatbubble.RoleUser).
			WithLabel("you").
			WithText("What's the difference between a glyph and a grapheme?").
			WithWidth(70),
		chatbubble.New(theme.Default).
			WithRole(chatbubble.RoleAssistant).
			WithLabel("glyph").
			WithText("A grapheme is the smallest unit of writing in a script; a glyph is the specific visual shape the renderer draws for it. One grapheme can be rendered by different glyphs depending on font and context.").
			WithWidth(70),
		chatbubble.New(theme.Default).
			WithRole(chatbubble.RoleSystem).
			WithLabel("system").
			WithText("Conversation reset at 09:42.").
			WithWidth(70),
		chatbubble.New(theme.Default).
			WithRole(chatbubble.RoleTool).
			WithLabel("tool · grep").
			WithText("matched 14 occurrences across 6 files").
			WithWidth(70),
	}

	for _, b := range bubbles {
		fmt.Println(b.View())
		fmt.Println()
	}
}
