// Command showcase is a single-binary demo that composes every v0.1
// glyph component into one navigable TUI. Tab cycles tabs forward,
// Shift-Tab cycles backward, q or Ctrl-C quits.
//
// Each tab puts one component in a realistic scenario. The status bar
// at the bottom is the status-bar component. The toast tray overlays
// every tab and is driven by a per-second tick.
//
// This file is intentionally one self-contained program. It's the demo
// you run to feel the library, not a template to copy. The pattern for
// copying components into your own app is in CONTRIBUTING.md.
package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	chatbubble "github.com/truffle-dev/glyph/components/chat-bubble"
	chatinput "github.com/truffle-dev/glyph/components/chat-input"
	chatthread "github.com/truffle-dev/glyph/components/chat-thread"
	commandpalette "github.com/truffle-dev/glyph/components/command-palette"
	diffview "github.com/truffle-dev/glyph/components/diff-view"
	logstream "github.com/truffle-dev/glyph/components/log-stream"
	markdownviewer "github.com/truffle-dev/glyph/components/markdown-viewer"
	notificationtoast "github.com/truffle-dev/glyph/components/notification-toast"
	statusbar "github.com/truffle-dev/glyph/components/status-bar"
	"github.com/truffle-dev/glyph/components/theme"
)

// tabKey enumerates the tabs in display order.
type tabKey int

const (
	tabChat tabKey = iota
	tabPalette
	tabMarkdown
	tabLogs
	tabDiff
	tabNumTabs
)

var tabNames = map[tabKey]string{
	tabChat:     "Chat",
	tabPalette:  "Commands",
	tabMarkdown: "Markdown",
	tabLogs:     "Logs",
	tabDiff:     "Diff",
}

// tickMsg fires once per second to drive log generation and toast TTL.
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type model struct {
	theme       theme.Theme
	active      tabKey
	width       int
	height      int
	chatInput   chatinput.Input
	chatThread  chatthread.Thread
	palette     commandpalette.Palette
	markdown    markdownviewer.Viewer
	logs        logstream.Stream
	diff        diffview.View
	toasts      notificationtoast.Tray
	logCounter  int
	toastSerial int
}

func newModel() model {
	t := theme.Default

	input := chatinput.New(t).
		WithPlaceholder("Type a message and press Enter.").
		WithPrompt("you").
		WithWidth(72).
		Focus()

	thread := chatthread.New(t).WithSize(72, 12)
	thread = thread.Append(chatthread.Message{Role: chatbubble.RoleAssistant, Label: "glyph", Text: "Welcome. Tab cycles between component demos. Press t to fire a toast, l to push a log entry, q to quit."})
	thread = thread.Append(chatthread.Message{Role: chatbubble.RoleUser, Label: "you", Text: "What runs underneath?"})
	thread = thread.Append(chatthread.Message{Role: chatbubble.RoleAssistant, Label: "glyph", Text: "Bubble Tea for the loop, lipgloss for the styling, reflow for the wrap. All four chat components are right here in this binary, no runtime dependency on glyph."})

	cmds := []commandpalette.Command{
		{ID: "new-file", Title: "New file", Description: "Create a blank buffer", Group: "File", Keybinding: "ctrl+n"},
		{ID: "open", Title: "Open file", Description: "Browse the workspace tree", Group: "File", Keybinding: "ctrl+o"},
		{ID: "save", Title: "Save file", Description: "Write the active buffer to disk", Group: "File", Keybinding: "ctrl+s"},
		{ID: "find", Title: "Find in files", Description: "Search the project tree", Group: "Search", Keybinding: "ctrl+shift+f"},
		{ID: "goto", Title: "Go to line", Description: "Jump to a specific line", Group: "Search", Keybinding: "ctrl+g"},
		{ID: "theme-toggle", Title: "Toggle theme", Description: "Switch between Default and Light", Group: "View"},
		{ID: "split", Title: "Split horizontal", Description: "Open a side pane", Group: "View", Keybinding: "ctrl+shift+d"},
		{ID: "quit", Title: "Quit", Description: "Close the application", Group: "App", Keybinding: "ctrl+q"},
	}
	palette := commandpalette.New(t).WithCommands(cmds).WithSize(72, 14).WithTitle("Commands")

	md := markdownviewer.New(t).WithSize(80, 18).WithSource(markdownSample)

	logs := logstream.New(t).WithSize(96, 16)
	logs = seedLogs(logs)

	diff := diffview.New(t).WithSize(96, 18).WithLines(diffview.ParseUnified(diffSample))

	toasts := notificationtoast.New(t).WithWidth(48).WithMaxItems(3)

	return model{
		theme:      t,
		active:     tabChat,
		chatInput:  input,
		chatThread: thread,
		palette:    palette,
		markdown:   md,
		logs:       logs,
		diff:       diff,
		toasts:     toasts,
	}
}

func (m model) Init() tea.Cmd { return tick() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		now := time.Time(msg)
		m.toasts = m.toasts.Tick(now)
		return m, tick()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.active == tabChat {
				// don't quit on q while typing
				break
			}
			return m, tea.Quit
		case "tab":
			m.active = (m.active + 1) % tabNumTabs
			return m, nil
		case "shift+tab":
			m.active = (m.active + tabNumTabs - 1) % tabNumTabs
			return m, nil
		case "t":
			if m.active != tabChat {
				m.toastSerial++
				m.toasts = m.toasts.Push(sampleToast(m.toastSerial))
				return m, nil
			}
		case "l":
			if m.active != tabChat {
				m.logCounter++
				m.logs = m.logs.Append(sampleLog(m.logCounter))
				return m, nil
			}
		}
	}

	switch m.active {
	case tabChat:
		return m.updateChat(msg)
	case tabPalette:
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		return m, cmd
	case tabMarkdown:
		var cmd tea.Cmd
		m.markdown, cmd = m.markdown.Update(msg)
		return m, cmd
	case tabLogs:
		var cmd tea.Cmd
		m.logs, cmd = m.logs.Update(msg)
		return m, cmd
	case tabDiff:
		var cmd tea.Cmd
		m.diff, cmd = m.diff.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) updateChat(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
		text := m.chatInput.Value()
		if text != "" {
			m.chatThread = m.chatThread.Append(chatthread.Message{
				Role: chatbubble.RoleUser, Label: "you", Text: text,
			})
			m.chatThread = m.chatThread.Append(chatthread.Message{
				Role: chatbubble.RoleAssistant, Label: "glyph", Text: "echo: " + text,
			})
			m.chatInput = m.chatInput.Reset()
		}
		return m, nil
	}
	var c1, c2 tea.Cmd
	m.chatInput, c1 = m.chatInput.Update(msg)
	m.chatThread, c2 = m.chatThread.Update(msg)
	return m, tea.Batch(c1, c2)
}

func (m model) View() string {
	tabs := m.renderTabs()
	body := m.renderBody()
	bar := m.renderStatusBar().View()

	composed := lipgloss.JoinVertical(lipgloss.Left, tabs, body, bar)
	toasts := m.toasts.View()
	if toasts == "" {
		return composed
	}
	return lipgloss.JoinVertical(lipgloss.Left, composed, "", toasts)
}

func (m model) renderTabs() string {
	pieces := make([]string, 0, int(tabNumTabs))
	for i := tabKey(0); i < tabNumTabs; i++ {
		label := " " + tabNames[i] + " "
		st := lipgloss.NewStyle().Foreground(m.theme.TextMuted)
		if i == m.active {
			st = lipgloss.NewStyle().Foreground(m.theme.PrimaryStrong).Bold(true).Underline(true)
		}
		pieces = append(pieces, st.Render(label))
	}
	joiner := lipgloss.NewStyle().Foreground(m.theme.Border).Render(" · ")
	return lipgloss.NewStyle().PaddingBottom(1).Render(joinStrings(pieces, joiner))
}

func (m model) renderBody() string {
	switch m.active {
	case tabChat:
		return lipgloss.JoinVertical(lipgloss.Left, m.chatThread.View(), m.chatInput.View())
	case tabPalette:
		return m.palette.View()
	case tabMarkdown:
		return m.markdown.View()
	case tabLogs:
		return m.logs.View()
	case tabDiff:
		return m.diff.View()
	}
	return ""
}

func (m model) renderStatusBar() statusbar.Bar {
	hint := "tab next · shift-tab prev · q quit"
	switch m.active {
	case tabChat:
		hint = "type to chat · enter send · tab next · ctrl-c quit"
	case tabPalette:
		hint = "type to filter · up/down navigate · esc cancel · tab next"
	case tabMarkdown, tabDiff, tabLogs:
		hint = "up/down scroll · pgup/pgdn page · t toast · l log · tab next"
	}
	return statusbar.New(m.theme).
		WithWidth(maxInt(60, m.width)).
		WithLeft(
			statusbar.Item{Text: "glyph", Style: statusbar.StylePrimary},
			statusbar.Item{Text: "showcase", Style: statusbar.StyleMuted},
		).
		WithCenter(
			statusbar.Item{Text: tabNames[m.active]},
		).
		WithRight(
			statusbar.Item{Text: hint, Style: statusbar.StyleMuted},
		)
}

func sampleToast(n int) notificationtoast.Toast {
	now := time.Now()
	level := notificationtoast.LevelInfo
	title := "Heads up"
	body := "Toast number " + itoa(n) + "."
	switch n % 4 {
	case 0:
		level = notificationtoast.LevelInfo
		title = "Info"
	case 1:
		level = notificationtoast.LevelSuccess
		title = "Success"
		body = "Background build completed cleanly."
	case 2:
		level = notificationtoast.LevelWarning
		title = "Warning"
		body = "Component manifest is missing a description field."
	case 3:
		level = notificationtoast.LevelError
		title = "Error"
		body = "Failed to fetch registry: network unreachable."
	}
	return notificationtoast.Toast{
		ID:        "t-" + itoa(n),
		Level:     level,
		Title:     title,
		Message:   body,
		ExpiresAt: now.Add(6 * time.Second),
	}
}

func sampleLog(n int) logstream.Entry {
	now := time.Now()
	level := logstream.LevelInfo
	src := "demo"
	msg := "tick number " + itoa(n)
	switch n % 5 {
	case 0:
		level = logstream.LevelInfo
		src = "http"
		msg = "GET /api/orders/9182 200 8ms"
	case 1:
		level = logstream.LevelInfo
		src = "queue"
		msg = "drained 42 items in 12ms"
	case 2:
		level = logstream.LevelWarn
		src = "auth"
		msg = "deprecated session token format from 10.0.0.4"
	case 3:
		level = logstream.LevelInfo
		src = "db"
		msg = "checkpoint complete (38ms)"
	case 4:
		level = logstream.LevelError
		src = "cache"
		msg = "redis disconnect; reconnecting in 200ms"
	}
	return logstream.Entry{Time: now, Level: level, Source: src, Message: msg}
}

func seedLogs(s logstream.Stream) logstream.Stream {
	base := time.Now().Add(-30 * time.Second)
	entries := []logstream.Entry{
		{Time: base.Add(0 * time.Second), Level: logstream.LevelInfo, Source: "boot", Message: "starting glyph showcase"},
		{Time: base.Add(1 * time.Second), Level: logstream.LevelInfo, Source: "db", Message: "connected to postgres"},
		{Time: base.Add(4 * time.Second), Level: logstream.LevelInfo, Source: "http", Message: "listening on :8080"},
		{Time: base.Add(5 * time.Second), Level: logstream.LevelInfo, Source: "http", Message: "GET /api/orders 200 12ms"},
		{Time: base.Add(7 * time.Second), Level: logstream.LevelWarn, Source: "auth", Message: "deprecated session token format from 10.0.0.4"},
		{Time: base.Add(8 * time.Second), Level: logstream.LevelInfo, Source: "queue", Message: "drained 42 items in 12ms"},
		{Time: base.Add(10 * time.Second), Level: logstream.LevelError, Source: "cache", Message: "redis disconnect; reconnecting in 200ms"},
		{Time: base.Add(10 * time.Second), Level: logstream.LevelInfo, Source: "cache", Message: "reconnected after 192ms"},
		{Time: base.Add(11 * time.Second), Level: logstream.LevelInfo, Source: "http", Message: "GET /api/orders/9182 200 8ms"},
	}
	for _, e := range entries {
		s = s.Append(e)
	}
	return s
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func joinStrings(parts []string, joiner string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += joiner + p
	}
	return out
}

const markdownSample = `# glyph

A copy-paste component library for terminal UIs. Yours to **own**.

## Why

You should never inherit a runtime dependency on an unstable terminal
component library. With glyph you run ` + "`glyph add chat-bubble`" + ` and the file
lands in your repo as plain Go. From that moment forward it is *your*
code.

## How it works

- Static JSON registry, one entry per component
- CLI fetches, rewrites imports, runs ` + "`go mod tidy`" + `
- No glyph runtime dependency

## Roadmap

- Adapters for ratatui, Textual, Ink
- More chat surfaces: tool-result card, citation footer
- Layout primitives: tabs, sheet, split-pane

---

See [the docs](https://truffleagent.com/glyph) for the full registry.
`

const diffSample = `--- a/server.go
+++ b/server.go
@@ -1,8 +1,12 @@
 package server

 import (
+	"context"
 	"net/http"
+	"time"
 )

-func Run(addr string) error {
-	return http.ListenAndServe(addr, nil)
+func Run(ctx context.Context, addr string) error {
+	srv := &http.Server{Addr: addr, ReadHeaderTimeout: 5 * time.Second}
+	go func() { <-ctx.Done(); srv.Close() }()
+	return srv.ListenAndServe()
 }
`

func main() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "showcase:", err)
		os.Exit(1)
	}
}
