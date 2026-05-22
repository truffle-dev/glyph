// Command reel is the glyph marketing demo: a self-playing TUI that
// walks through the library's headline components on a fixed timeline.
// It is the same model as examples/showcase but driven by a scripted
// playback goroutine instead of a keyboard. Run it under asciinema to
// record the canonical glyph demo GIF.
//
//	asciinema rec --cols 100 --rows 30 --command ./reel reel.cast
//	agg --theme monokai --font-size 18 reel.cast reel.gif
//
// The reel runs about thirty seconds and exits on its own.
package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	chatbubble "github.com/truffle-dev/glyph/components/chat-bubble"
	chatinput "github.com/truffle-dev/glyph/components/chat-input"
	chatthread "github.com/truffle-dev/glyph/components/chat-thread"
	commandpalette "github.com/truffle-dev/glyph/components/command-palette"
	diffview "github.com/truffle-dev/glyph/components/diff-view"
	keyhints "github.com/truffle-dev/glyph/components/key-hints"
	"github.com/truffle-dev/glyph/components/list"
	logstream "github.com/truffle-dev/glyph/components/log-stream"
	notificationtoast "github.com/truffle-dev/glyph/components/notification-toast"
	"github.com/truffle-dev/glyph/components/panel"
	progressbar "github.com/truffle-dev/glyph/components/progress-bar"
	"github.com/truffle-dev/glyph/components/spinner"
	statusbar "github.com/truffle-dev/glyph/components/status-bar"
	"github.com/truffle-dev/glyph/components/tabs"
	"github.com/truffle-dev/glyph/components/theme"
)

// scene enumerates the timed scenes the reel walks through.
type scene int

const (
	sceneChat scene = iota
	scenePalette
	sceneLogs
	sceneSidebar
	sceneProgress
	sceneDiff
	sceneEnd
)

type sceneTransition struct{ next scene }
type playKey struct{ key tea.KeyMsg }
type playType struct{ text string }
type typeNextMsg struct {
	text string
	at   int
}
type appendMessageMsg struct {
	role  chatbubble.Role
	label string
	text  string
}
type pushToastMsg struct{ t notificationtoast.Toast }
type progressTickMsg struct{ pct float64 }
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type model struct {
	theme    theme.Theme
	width    int
	height   int
	scene    scene
	tabs     tabs.Tabs
	chatIn   chatinput.Input
	chatThr  chatthread.Thread
	palette  commandpalette.Palette
	logs     logstream.Stream
	sidebar  list.List
	diff     diffview.View
	progBar  progressbar.Bar
	toasts   notificationtoast.Tray
	hints    keyhints.Bar
	spin     spinner.Spinner
	logTick  int
	progress float64
}

func newModel() model {
	t := theme.Default

	in := chatinput.New(t).
		WithPlaceholder("Ask me anything.").
		WithPrompt("you › ").
		WithWidth(82).
		Focus()

	thr := chatthread.New(t).WithSize(82, 10)
	thr = thr.Append(chatthread.Message{
		Role: chatbubble.RoleAssistant, Label: "glyph",
		Text: "Hi. I'm a TUI built out of glyph components. The whole conversation surface is three primitives composed together.",
	})

	pal := commandpalette.New(t).
		WithCommands([]commandpalette.Command{
			{ID: "save", Title: "Save file", Description: "Write the active buffer to disk", Group: "File", Keybinding: "ctrl+s"},
			{ID: "save-all", Title: "Save all", Description: "Persist every open buffer", Group: "File", Keybinding: "ctrl+shift+s"},
			{ID: "save-as", Title: "Save as", Description: "Pick a new path for this buffer", Group: "File"},
			{ID: "open", Title: "Open file", Description: "Browse the workspace tree", Group: "File", Keybinding: "ctrl+o"},
			{ID: "find", Title: "Find in files", Description: "Search the project tree", Group: "Search", Keybinding: "ctrl+shift+f"},
			{ID: "theme-toggle", Title: "Toggle theme", Description: "Switch between Default and Light", Group: "View"},
			{ID: "split", Title: "Split horizontal", Description: "Open a side pane", Group: "View", Keybinding: "ctrl+shift+d"},
			{ID: "quit", Title: "Quit", Description: "Close the application", Group: "App", Keybinding: "ctrl+q"},
		}).
		WithSize(82, 12).
		WithTitle("Commands")

	logs := logstream.New(t).WithSize(96, 10)
	logs = logs.
		Append(logstream.Entry{Time: time.Now().Add(-30 * time.Second), Level: logstream.LevelInfo, Message: "agent boot complete"}).
		Append(logstream.Entry{Time: time.Now().Add(-26 * time.Second), Level: logstream.LevelInfo, Message: "loaded 16 components"})

	sidebar := list.New(t).WithHeight(8).WithItems([]list.Item{
		{Label: "Inbox", Hint: "12 unread"},
		{Label: "Drafts"},
		{Label: "Sent"},
		{Label: "Archive"},
		{Label: "Spam", Disabled: true},
		{Label: "Trash"},
	})

	prog := progressbar.New(t).
		WithWidth(56).
		WithLabel("compiling").
		WithFillColor(t.Primary)

	diff := diffview.New(t).WithSize(96, 12).WithLines(diffview.ParseUnified(diffSample))

	toasts := notificationtoast.New(t).WithWidth(48).WithMaxItems(3)

	hints := keyhints.New(t).WithHints([]keyhints.Hint{
		{Key: "tab", Desc: "next"},
		{Key: "/", Desc: "search"},
		{Key: "?", Desc: "help"},
		{Key: "q", Desc: "quit"},
	})

	spin := spinner.New(t).WithStyle(spinner.StyleDots).WithLabel("thinking")

	tabBar := tabs.New(t).WithTabs([]string{"Chat", "Commands", "Logs", "Sidebar", "Progress", "Diff"})

	return model{
		theme:   t,
		scene:   sceneChat,
		tabs:    tabBar,
		chatIn:  in,
		chatThr: thr,
		palette: pal,
		logs:    logs,
		sidebar: sidebar,
		diff:    diff,
		progBar: prog,
		toasts:  toasts,
		hints:   hints,
		spin:    spin,
	}
}

func (m model) Init() tea.Cmd { return tea.Batch(tick(), m.spin.Init()) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		now := time.Time(msg)
		m.toasts = m.toasts.Tick(now)
		m.logTick++
		if m.scene == sceneLogs && m.logTick%2 == 0 {
			m.logs = m.logs.Append(sampleLog(m.logTick))
		}
		return m, tick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case sceneTransition:
		m.scene = msg.next
		// Map scenes to tab positions for the visual top bar.
		idx := int(msg.next)
		if idx >= 0 && idx < 6 {
			m.tabs = m.tabs.WithActive(idx)
		}
		return m, nil

	case playKey:
		return m.routeKey(msg.key)

	case typeNextMsg:
		if msg.at >= len(msg.text) {
			return m, nil
		}
		r := rune(msg.text[msg.at])
		m.chatIn, _ = m.chatIn.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		return m, tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
			return typeNextMsg{text: msg.text, at: msg.at + 1}
		})

	case appendMessageMsg:
		m.chatThr = m.chatThr.Append(chatthread.Message{Role: msg.role, Label: msg.label, Text: msg.text})
		return m, nil

	case pushToastMsg:
		m.toasts = m.toasts.Push(msg.t)
		return m, nil

	case progressTickMsg:
		m.progress = msg.pct
		m.progBar = m.progBar.WithPercent(msg.pct)
		return m, nil
	}
	return m, nil
}

func (m model) routeKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.scene {
	case sceneChat:
		if key.Type == tea.KeyEnter {
			text := m.chatIn.Value()
			if text != "" {
				m.chatThr = m.chatThr.Append(chatthread.Message{Role: chatbubble.RoleUser, Label: "you", Text: text})
				m.chatIn = m.chatIn.Reset()
			}
			return m, nil
		}
		var c1 tea.Cmd
		m.chatIn, c1 = m.chatIn.Update(key)
		return m, c1
	case scenePalette:
		var c tea.Cmd
		m.palette, c = m.palette.Update(key)
		return m, c
	case sceneSidebar:
		var c tea.Cmd
		m.sidebar, c = m.sidebar.Update(key)
		return m, c
	case sceneLogs:
		var c tea.Cmd
		m.logs, c = m.logs.Update(key)
		return m, c
	case sceneDiff:
		var c tea.Cmd
		m.diff, c = m.diff.Update(key)
		return m, c
	}
	return m, nil
}

func (m model) View() string {
	if m.scene == sceneEnd {
		return ""
	}
	top := m.tabs.View()
	body := m.renderBody()
	bar := m.renderStatusBar()
	hintsRow := m.hints.View()

	composed := lipgloss.JoinVertical(lipgloss.Left,
		top,
		"",
		body,
		"",
		bar,
		hintsRow,
	)
	toasts := m.toasts.View()
	if toasts == "" {
		return composed
	}
	return lipgloss.JoinVertical(lipgloss.Left, composed, "", toasts)
}

func (m model) renderBody() string {
	switch m.scene {
	case sceneChat:
		body := lipgloss.JoinVertical(lipgloss.Left, m.chatThr.View(), m.chatIn.View())
		return panel.New(m.theme).WithTitle("Chat").WithContent(body).View()
	case scenePalette:
		return m.palette.View()
	case sceneLogs:
		return panel.New(m.theme).WithTitle("Logs").WithFooter("tailing").WithContent(m.logs.View()).View()
	case sceneSidebar:
		side := m.sidebar.View()
		detail := lipgloss.NewStyle().Foreground(m.theme.TextMuted).Render("Select a folder to preview.")
		row := lipgloss.JoinHorizontal(lipgloss.Top,
			panel.New(m.theme).WithTitle("Mailboxes").WithWidth(28).WithHeight(12).WithContent(side).View(),
			"  ",
			panel.New(m.theme).WithTitle("Preview").WithWidth(48).WithHeight(12).WithContent(detail).View(),
		)
		return row
	case sceneProgress:
		spinLine := lipgloss.NewStyle().Foreground(m.theme.TextMuted).Render(m.spin.View())
		body := lipgloss.JoinVertical(lipgloss.Left,
			m.progBar.View(),
			"",
			spinLine,
		)
		return panel.New(m.theme).WithTitle("Build").WithContent(body).View()
	case sceneDiff:
		return panel.New(m.theme).WithTitle("Diff").WithContent(m.diff.View()).View()
	}
	return ""
}

func (m model) renderStatusBar() string {
	sceneName := []string{"Chat", "Commands", "Logs", "Sidebar", "Build", "Diff"}[clampScene(m.scene)]
	return statusbar.New(m.theme).
		WithWidth(maxInt(80, m.width)).
		WithLeft(
			statusbar.Item{Text: "glyph", Style: statusbar.StylePrimary},
			statusbar.Item{Text: "reel", Style: statusbar.StyleMuted},
		).
		WithCenter(statusbar.Item{Text: sceneName}).
		WithRight(statusbar.Item{Text: "ready", Style: statusbar.StyleSuccess}).
		View()
}

func clampScene(s scene) int {
	i := int(s)
	if i < 0 {
		return 0
	}
	if i > 5 {
		return 5
	}
	return i
}

const diffSample = `@@ -1,5 +1,7 @@
 package theme

-var Default = Theme{
-    Primary: "62",
+var Default = Theme{
+    Primary:       "62",
+    PrimaryStrong: "63",
+    Accent:        "212",
 }`

func sampleLog(n int) logstream.Entry {
	levels := []logstream.Level{
		logstream.LevelInfo,
		logstream.LevelInfo,
		logstream.LevelDebug,
		logstream.LevelInfo,
		logstream.LevelWarn,
		logstream.LevelInfo,
		logstream.LevelError,
		logstream.LevelInfo,
	}
	msgs := []string{
		"connected to ws://api.example.com/agent",
		"prompt assembled (2.4kb)",
		"cache hit on system prompt",
		"streaming response (78 tokens/s)",
		"tool call rate-limited, retrying in 2s",
		"tool call returned 200 OK",
		"upstream returned 503, backing off",
		"reconnected, session restored",
	}
	idx := n % len(msgs)
	return logstream.Entry{Time: time.Now(), Level: levels[idx], Message: msgs[idx]}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// playback drives the reel. It blocks while sending scripted messages
// to the running tea.Program. Each step is followed by a sleep so the
// recording captures the transition cleanly.
func playback(p *tea.Program) {
	// 1. Chat: type a question, press Enter, assistant replies.
	time.Sleep(800 * time.Millisecond)
	p.Send(typeNextMsg{text: "What runs underneath?", at: 0})
	time.Sleep(1500 * time.Millisecond)
	p.Send(playKey{key: tea.KeyMsg{Type: tea.KeyEnter}})
	time.Sleep(400 * time.Millisecond)
	p.Send(appendMessageMsg{
		role: chatbubble.RoleAssistant, label: "glyph",
		text: "Bubble Tea for the loop, lipgloss for the styling, reflow for wrap. Three components compose this surface and you copied the source.",
	})
	time.Sleep(2500 * time.Millisecond)

	// 2. Commands palette.
	p.Send(sceneTransition{next: scenePalette})
	time.Sleep(700 * time.Millisecond)
	for _, r := range "save" {
		p.Send(playKey{key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}})
		time.Sleep(120 * time.Millisecond)
	}
	time.Sleep(1500 * time.Millisecond)

	// 3. Logs (with a toast).
	p.Send(sceneTransition{next: sceneLogs})
	time.Sleep(800 * time.Millisecond)
	p.Send(pushToastMsg{t: notificationtoast.Toast{
		ID: "deploy", Level: notificationtoast.LevelSuccess,
		Title: "Deployed", Message: "v0.1.2 live on production.",
		ExpiresAt: time.Now().Add(6 * time.Second),
	}})
	time.Sleep(3500 * time.Millisecond)

	// 4. Sidebar list, walk the cursor.
	p.Send(sceneTransition{next: sceneSidebar})
	time.Sleep(600 * time.Millisecond)
	for i := 0; i < 4; i++ {
		p.Send(playKey{key: tea.KeyMsg{Type: tea.KeyDown}})
		time.Sleep(450 * time.Millisecond)
	}
	time.Sleep(700 * time.Millisecond)

	// 5. Progress bar fills.
	p.Send(sceneTransition{next: sceneProgress})
	time.Sleep(500 * time.Millisecond)
	for i := 0; i <= 100; i += 5 {
		p.Send(progressTickMsg{pct: float64(i) / 100})
		time.Sleep(70 * time.Millisecond)
	}
	time.Sleep(1000 * time.Millisecond)

	// 6. Diff view (static reveal). Hold long enough that even after agg
	// caps idle time to 1s the diff still owns several GIF frames.
	p.Send(sceneTransition{next: sceneDiff})
	time.Sleep(4500 * time.Millisecond)

	// 7. Exit on the diff frame (no sceneEnd clear; the recorded final
	// frame should be the finished diff, not an empty buffer).
	p.Quit()
}

func main() {
	// The reel is meant to be recorded under asciinema's headless PTY,
	// where termenv otherwise downgrades to no-color. Force TrueColor
	// so the GIF preserves the indigo accents and status colors.
	lipgloss.SetColorProfile(termenv.TrueColor)
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	go playback(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "reel:", err)
		os.Exit(1)
	}
}
