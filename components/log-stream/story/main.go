//go:build glyph_story

package main

import (
	"fmt"
	"time"

	logstream "github.com/truffle-dev/glyph/components/log-stream"
	"github.com/truffle-dev/glyph/components/theme"
)

func ts(s string) time.Time {
	t, _ := time.Parse("15:04:05", s)
	return t
}

func main() {
	mixed := logstream.New(theme.Default).WithSize(80, 10)
	for _, e := range []logstream.Entry{
		{Time: ts("09:00:01"), Level: logstream.LevelInfo, Source: "boot", Message: "starting up"},
		{Time: ts("09:00:02"), Level: logstream.LevelInfo, Source: "db", Message: "connected to postgres"},
		{Time: ts("09:00:05"), Level: logstream.LevelWarn, Source: "auth", Message: "deprecated session token format from 10.0.0.4"},
		{Time: ts("09:00:08"), Level: logstream.LevelInfo, Source: "http", Message: "GET /api/v1/orders 200 12ms"},
		{Time: ts("09:00:14"), Level: logstream.LevelError, Source: "queue", Message: "redis disconnect; reconnecting in 200ms"},
		{Time: ts("09:00:14"), Level: logstream.LevelInfo, Source: "queue", Message: "reconnected"},
		{Time: ts("09:00:20"), Level: logstream.LevelInfo, Source: "http", Message: "GET /api/v1/orders/9182 200 8ms"},
	} {
		mixed = mixed.Append(e)
	}

	debugIncluded := mixed.WithMinLevel(logstream.LevelDebug)
	errorsOnly := mixed.WithMinLevel(logstream.LevelError)

	for _, s := range []struct {
		name string
		l    logstream.Stream
	}{
		{"mixed levels (INFO+)", mixed},
		{"all (DEBUG+)", debugIncluded},
		{"errors only", errorsOnly},
	} {
		fmt.Println(s.name)
		fmt.Println(s.l.View())
		fmt.Println()
	}
}
