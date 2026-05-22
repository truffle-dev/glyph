//go:build glyph_story

package main

import (
	"fmt"
	"time"

	notificationtoast "github.com/truffle-dev/glyph/components/notification-toast"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	now := time.Now()

	mixed := notificationtoast.New(theme.Default).WithWidth(50)
	mixed = mixed.Push(notificationtoast.Toast{
		ID: "1", Level: notificationtoast.LevelInfo,
		Title: "Build started", Message: "Watching components/ for changes.",
		ExpiresAt: now.Add(5 * time.Second),
	})
	mixed = mixed.Push(notificationtoast.Toast{
		ID: "2", Level: notificationtoast.LevelSuccess,
		Title: "Tests passed", Message: "47 tests, 0 failures.",
	})
	mixed = mixed.Push(notificationtoast.Toast{
		ID: "3", Level: notificationtoast.LevelWarning,
		Title: "Deprecated import", Message: "components/chat-bubble imports a v0 lipgloss API.",
	})
	mixed = mixed.Push(notificationtoast.Toast{
		ID: "4", Level: notificationtoast.LevelError,
		Title: "Build failed", Message: "components/log-stream/log-stream.go:42: undefined: time.now",
	})

	singleSuccess := notificationtoast.New(theme.Default).WithWidth(40).Push(notificationtoast.Toast{
		ID: "s", Level: notificationtoast.LevelSuccess, Title: "Saved", Message: "Wrote glyph.json.",
	})

	for _, s := range []struct {
		name string
		t    notificationtoast.Tray
	}{
		{"four levels stacked", mixed},
		{"single success", singleSuccess},
	} {
		fmt.Println(s.name)
		fmt.Println(s.t.View())
		fmt.Println()
	}
}
