//go:build glyph_story

package main

import (
	"fmt"

	statusbar "github.com/truffle-dev/glyph/components/status-bar"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	editor := statusbar.New(theme.Default).WithWidth(80).
		WithLeft(
			statusbar.Item{Text: "NORMAL", Style: statusbar.StylePrimary},
			statusbar.Item{Text: "main.go"},
		).
		WithCenter(
			statusbar.Item{Text: "go", Style: statusbar.StyleMuted},
		).
		WithRight(
			statusbar.Item{Text: "modified", Style: statusbar.StyleWarning},
			statusbar.Item{Text: "L 42:18"},
			statusbar.Item{Text: "100%"},
		)

	build := statusbar.New(theme.Default).WithWidth(80).
		WithLeft(
			statusbar.Item{Text: "glyph"},
			statusbar.Item{Text: "build", Style: statusbar.StyleMuted},
		).
		WithRight(
			statusbar.Item{Text: "✓ 47 tests", Style: statusbar.StyleSuccess},
			statusbar.Item{Text: "8 components"},
		)

	tight := statusbar.New(theme.Default).WithWidth(40).
		WithLeft(
			statusbar.Item{Text: "components/notification-toast/notification-toast.go"},
		).
		WithRight(
			statusbar.Item{Text: "OK", Style: statusbar.StyleSuccess},
		)

	for _, s := range []struct {
		name string
		b    statusbar.Bar
	}{
		{"editor-style", editor},
		{"build-status", build},
		{"narrow (truncates left)", tight},
	} {
		fmt.Println(s.name)
		fmt.Println(s.b.View())
		fmt.Println()
	}
}
