//go:build glyph_story

package main

import (
	"fmt"

	markdownviewer "github.com/truffle-dev/glyph/components/markdown-viewer"
	"github.com/truffle-dev/glyph/components/theme"
)

const sample = `# glyph

Beautifully designed components for the terminal. Yours to copy, paste, own.

## Why

A consumer should never inherit a *runtime dependency* on an unstable
component library. With glyph you ` + "`" + `glyph add chat-bubble` + "`" + ` and the file lands
in your repo. From that moment forward it is **your** code.

## Install

` + "```bash\nglyph init\nglyph add chat-bubble\n```" + `

## Roadmap

- chat-bubble, chat-input, chat-thread
- command-palette, markdown-viewer
- log-stream, diff-view, notification-toast, status-bar

---

See [the docs](https://truffleagent.com/glyph) for the full registry.
`

func main() {
	short := New(theme.Default).WithSize(60, 8).WithSource("# Hello\n\nA brief paragraph with **bold** and *italic*.")
	long := New(theme.Default).WithSize(70, 16).WithSource(sample)
	light := New(theme.Light).WithSize(70, 16).WithSource(sample)

	for _, s := range []struct {
		name string
		v    markdownviewer.Viewer
	}{
		{"short", short},
		{"long (dark)", long},
		{"long (light)", light},
	} {
		fmt.Println(s.name)
		fmt.Println(s.v.View())
		fmt.Println()
	}
}

// New is a local alias so the story reads symmetrically with other stories.
func New(t theme.Theme) markdownviewer.Viewer { return markdownviewer.New(t) }
