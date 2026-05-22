//go:build glyph_story

package main

import (
	"fmt"

	commandpalette "github.com/truffle-dev/glyph/components/command-palette"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	cmds := []commandpalette.Command{
		{ID: "open", Title: "Open file", Description: "Browse and open any file in the workspace", Group: "File", Keybinding: "ctrl+o"},
		{ID: "save", Title: "Save file", Description: "Write the current buffer to disk", Group: "File", Keybinding: "ctrl+s"},
		{ID: "save-as", Title: "Save as…", Description: "Save the current buffer under a new path", Group: "File", Keybinding: "ctrl+shift+s"},
		{ID: "find", Title: "Find in files", Description: "Search across the project tree", Group: "Search", Keybinding: "ctrl+shift+f"},
		{ID: "goto", Title: "Go to line", Description: "Jump to a specific line number", Group: "Search", Keybinding: "ctrl+g"},
		{ID: "theme-light", Title: "Switch to light theme", Description: "Use the warm-paper palette", Group: "View"},
		{ID: "theme-dark", Title: "Switch to dark theme", Description: "Use the default dark palette", Group: "View"},
		{ID: "quit", Title: "Quit", Description: "Close the application", Group: "App", Keybinding: "ctrl+q"},
	}

	defaults := commandpalette.New(theme.Default).
		WithCommands(cmds).
		WithSize(60, 12).
		WithTitle("Commands")

	filtered := commandpalette.New(theme.Default).
		WithCommands(cmds).
		WithSize(60, 12).
		WithTitle("Commands").
		WithFilter("save")

	empty := commandpalette.New(theme.Default).
		WithCommands(cmds).
		WithSize(60, 12).
		WithTitle("Commands").
		WithFilter("xyzzy")

	for _, s := range []struct {
		name string
		p    commandpalette.Palette
	}{
		{"default (all commands)", defaults},
		{"filtered", filtered},
		{"no matches", empty},
	} {
		fmt.Println(s.name)
		fmt.Println(s.p.View())
		fmt.Println()
	}
}
