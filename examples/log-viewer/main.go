// Command log-viewer is a journalctl-style live log viewer composed from
// nine glyph components: log-stream, tabs, status-bar, key-hints, select,
// notification-toast, text-input, panel, and theme.
//
// The demo synthesizes a steady stream of log entries across four sources
// (server, auth, ratelim, db) and lets the operator filter by level
// (tabs at the top), filter by source (popover via Ctrl-S), search by
// substring (slash key opens a text-input prompt), pause/resume the live
// feed (Space), and clear the buffer (Ctrl-L).
//
// What runs on screen at once:
//
//   - status-bar at the top: live/paused mode, entry count, active filter
//   - tabs row: All / Info+ / Warn+ / Error+ level filter
//   - log-stream in the middle, scrollable
//   - key-hints at the bottom
//   - notification-toast tray, top-right
//
// Overlays opened on demand:
//
//   - select: source filter (Ctrl-F)
//   - text-input + panel: substring search ('/')
//
// Run it:
//
//	go run ./examples/log-viewer/
//
// Quit at any time with Ctrl-C or q.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	keyhints "github.com/truffle-dev/glyph/components/key-hints"
	logstream "github.com/truffle-dev/glyph/components/log-stream"
	notificationtoast "github.com/truffle-dev/glyph/components/notification-toast"
	"github.com/truffle-dev/glyph/components/panel"
	selectinput "github.com/truffle-dev/glyph/components/select"
	statusbar "github.com/truffle-dev/glyph/components/status-bar"
	"github.com/truffle-dev/glyph/components/tabs"
	textinput "github.com/truffle-dev/glyph/components/text-input"
	"github.com/truffle-dev/glyph/components/theme"
)

type mode int

const (
	modeView mode = iota
	modeSourcePicker
	modeSearchPrompt
)

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(800*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type toastTickMsg time.Time

func toastTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return toastTickMsg(t) })
}

// Sources used to synthesize entries.
var sources = []string{"server", "auth", "ratelim", "db"}

// canned message bodies keyed by source. Each line is realistic enough to
// read as a real log feed.
var canned = map[string][]struct {
	level logstream.Level
	text  string
}{
	"server": {
		{logstream.LevelInfo, "listening on :8080"},
		{logstream.LevelInfo, "accepted connection from 10.0.0.42"},
		{logstream.LevelInfo, "GET /api/users -> 200 (12ms)"},
		{logstream.LevelInfo, "POST /api/sessions -> 201 (38ms)"},
		{logstream.LevelWarn, "slow handler: /api/reports took 1.2s"},
		{logstream.LevelDebug, "keep-alive ping ok"},
	},
	"auth": {
		{logstream.LevelInfo, "token refresh for user 12"},
		{logstream.LevelInfo, "session opened for user 47"},
		{logstream.LevelWarn, "deprecated jwt format on user 99"},
		{logstream.LevelError, "invalid signature for token 0xab12"},
		{logstream.LevelDebug, "key rotation: kid=2026-05"},
	},
	"ratelim": {
		{logstream.LevelInfo, "bucket /api/messages: 38/120"},
		{logstream.LevelWarn, "upstream slow: openai 420ms (avg 180)"},
		{logstream.LevelWarn, "throttling tenant tx_123 — quota exhausted"},
		{logstream.LevelInfo, "bucket /api/files: 7/30"},
	},
	"db": {
		{logstream.LevelInfo, "pool size 12/20"},
		{logstream.LevelInfo, "query SELECT users by email (3ms)"},
		{logstream.LevelWarn, "slow query INSERT events (212ms)"},
		{logstream.LevelError, "connection refused: postgres://db-2:5432"},
		{logstream.LevelDebug, "vacuum analyze complete on table sessions"},
	},
}

// filter encodes the user-visible level filter.
type levelFilter int

const (
	filterAll levelFilter = iota
	filterInfo
	filterWarn
	filterError
)

func (f levelFilter) minLevel() logstream.Level {
	switch f {
	case filterInfo:
		return logstream.LevelInfo
	case filterWarn:
		return logstream.LevelWarn
	case filterError:
		return logstream.LevelError
	}
	return logstream.LevelDebug
}

func (f levelFilter) label() string {
	switch f {
	case filterInfo:
		return "INFO+"
	case filterWarn:
		return "WARN+"
	case filterError:
		return "ERROR"
	}
	return "ALL"
}

type model struct {
	theme theme.Theme

	width, height int
	mode          mode

	// always-visible
	stream logstream.Stream
	bar    statusbar.Bar
	tabs   tabs.Tabs
	hints  keyhints.Bar
	tray   notificationtoast.Tray

	// overlays
	picker  selectinput.Select
	search  textinput.Input
	searchM panel.Panel

	// state
	filter      levelFilter
	source      string // "" = all sources
	query       string
	paused      bool
	entryCount  int
	toastSerial int

	// raw buffer of all entries that ever streamed in, so we can re-filter on the fly.
	all []logstream.Entry
}

func newModel() model {
	t := theme.Default

	stream := logstream.New(t).
		WithCapacity(2000).
		WithSize(100, 14).
		WithTimestamps(true).
		WithTimeFormat("15:04:05").
		WithMinLevel(logstream.LevelDebug)

	bar := statusbar.New(t).
		WithWidth(100).
		WithLeft(statusbar.Item{Text: "● live", Style: statusbar.StyleSuccess}).
		WithCenter(statusbar.Item{Text: "ALL · all sources", Style: statusbar.StylePrimary}).
		WithRight(statusbar.Item{Text: "0 entries"})

	tbs := tabs.New(t).
		WithTabs([]string{"All", "Info+", "Warn+", "Error+"}).
		WithActive(0).
		WithWidth(100)

	hints := keyhints.New(t).
		WithWidth(100).
		WithHints(viewHints())

	tray := notificationtoast.New(t).WithWidth(40).WithMaxItems(3)

	picker := selectinput.New(t).
		WithTitle("Filter by source").
		WithOptions(sourceOptions()).
		WithSize(36, 6)

	search := textinput.New(t).
		WithPlaceholder("substring…").
		WithWidth(40)

	searchM := panel.New(t).
		WithTitle("Search").
		WithFooter("⌃D apply · Esc cancel")

	return model{
		theme:   t,
		mode:    modeView,
		stream:  stream,
		bar:     bar,
		tabs:    tbs,
		hints:   hints,
		tray:    tray,
		picker:  picker,
		search:  search,
		searchM: searchM,
	}
}

func sourceOptions() []selectinput.Option {
	out := []selectinput.Option{{Label: "All sources", Hint: "no filter", Value: ""}}
	for _, s := range sources {
		out = append(out, selectinput.Option{Label: s, Hint: "source", Value: s})
	}
	return out
}

func viewHints() []keyhints.Hint {
	return []keyhints.Hint{
		{Key: "Tab", Desc: "level"},
		{Key: "⌃F", Desc: "source"},
		{Key: "/", Desc: "search"},
		{Key: "Space", Desc: "pause"},
		{Key: "⌃L", Desc: "clear"},
		{Key: "q", Desc: "quit"},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), toastTick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		w := msg.Width
		if w < 40 {
			w = 40
		}
		streamH := msg.Height - 4 // status + tabs + hints + slack
		if streamH < 6 {
			streamH = 6
		}
		m.bar = m.bar.WithWidth(w)
		m.tabs = m.tabs.WithWidth(w)
		m.hints = m.hints.WithWidth(w)
		m.stream = m.stream.WithSize(w, streamH)
		return m, nil

	case tickMsg:
		if !m.paused {
			e := synthesize()
			m.all = append(m.all, e)
			if len(m.all) > 2000 {
				m.all = m.all[len(m.all)-2000:]
			}
			if m.matches(e) {
				m.stream = m.stream.Append(e)
				m.entryCount++
				m.bar = m.refreshBar()
			}
		}
		return m, tick()

	case toastTickMsg:
		m.tray = m.tray.Tick(time.Time(msg))
		return m, toastTick()

	case selectinput.SelectMsg:
		m.source = msg.Option.Value
		m.mode = modeView
		m.hints = m.hints.WithHints(viewHints())
		m = m.rebuildStream()
		label := msg.Option.Label
		if msg.Option.Value == "" {
			label = "all sources"
		}
		return m, m.pushToast(notificationtoast.LevelInfo, "Source filter", label)

	case selectinput.CancelMsg:
		m.mode = modeView
		m.hints = m.hints.WithHints(viewHints())
		return m, nil

	case textinput.SubmitMsg:
		m.query = strings.TrimSpace(msg.Value)
		m.mode = modeView
		m.hints = m.hints.WithHints(viewHints())
		m = m.rebuildStream()
		title := "Search cleared"
		body := "showing all matching entries"
		if m.query != "" {
			title = "Search applied"
			body = "matching: " + m.query
		}
		return m, m.pushToast(notificationtoast.LevelInfo, title, body)

	case textinput.CancelMsg:
		m.mode = modeView
		m.hints = m.hints.WithHints(viewHints())
		return m, nil

	case tea.KeyMsg:
		return m.routeKey(msg)
	}

	return m, nil
}

func (m model) routeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.mode {

	case modeSourcePicker:
		if msg.String() == "esc" {
			m.mode = modeView
			m.hints = m.hints.WithHints(viewHints())
			return m, nil
		}
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd

	case modeSearchPrompt:
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd

	case modeView:
		switch msg.String() {
		case "tab":
			next := (m.filter + 1) % 4
			m.filter = next
			m.tabs = m.tabs.WithActive(int(next))
			m = m.rebuildStream()
			return m, m.pushToast(notificationtoast.LevelInfo, "Level filter", m.filter.label())
		case "shift+tab":
			next := levelFilter((int(m.filter) + 3) % 4)
			m.filter = next
			m.tabs = m.tabs.WithActive(int(next))
			m = m.rebuildStream()
			return m, nil
		case "ctrl+f":
			m.mode = modeSourcePicker
			m.hints = m.hints.WithHints([]keyhints.Hint{
				{Key: "↑↓", Desc: "navigate"},
				{Key: "⏎", Desc: "select"},
				{Key: "Esc", Desc: "cancel"},
			})
			return m, nil
		case "/":
			m.mode = modeSearchPrompt
			m.search = m.search.Reset()
			m.hints = m.hints.WithHints([]keyhints.Hint{
				{Key: "⌃D", Desc: "apply"},
				{Key: "Esc", Desc: "cancel"},
			})
			return m, nil
		case " ":
			m.paused = !m.paused
			m.bar = m.refreshBar()
			level := notificationtoast.LevelWarning
			title := "Paused"
			body := "stream is no longer appending"
			if !m.paused {
				level = notificationtoast.LevelSuccess
				title = "Resumed"
				body = "stream is live again"
			}
			return m, m.pushToast(level, title, body)
		case "ctrl+l":
			m.all = nil
			m.stream = m.stream.Clear()
			m.entryCount = 0
			m.bar = m.refreshBar()
			return m, m.pushToast(notificationtoast.LevelInfo, "Cleared", "buffer is empty")
		case "q":
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.stream, cmd = m.stream.Update(msg)
		return m, cmd
	}

	return m, nil
}

// matches reports whether an entry passes the current level/source/query filter.
func (m model) matches(e logstream.Entry) bool {
	if e.Level < m.filter.minLevel() {
		return false
	}
	if m.source != "" && e.Source != m.source {
		return false
	}
	if m.query != "" {
		q := strings.ToLower(m.query)
		if !strings.Contains(strings.ToLower(e.Source), q) &&
			!strings.Contains(strings.ToLower(e.Message), q) {
			return false
		}
	}
	return true
}

// rebuildStream rebuilds the visible log-stream from m.all using the current filter.
func (m model) rebuildStream() model {
	m.stream = m.stream.Clear()
	m.entryCount = 0
	for _, e := range m.all {
		if m.matches(e) {
			m.stream = m.stream.Append(e)
			m.entryCount++
		}
	}
	m.bar = m.refreshBar()
	return m
}

func (m model) refreshBar() statusbar.Bar {
	left := statusbar.Item{Text: "● live", Style: statusbar.StyleSuccess}
	if m.paused {
		left = statusbar.Item{Text: "⏸ paused", Style: statusbar.StyleWarning}
	}
	center := m.filter.label()
	if m.source != "" {
		center = center + " · " + m.source
	} else {
		center = center + " · all sources"
	}
	if m.query != "" {
		center = center + " · /" + m.query
	}
	count := fmt.Sprintf("%d entries", m.entryCount)
	return m.bar.
		WithLeft(left).
		WithCenter(statusbar.Item{Text: center, Style: statusbar.StylePrimary}).
		WithRight(statusbar.Item{Text: count})
}

func (m *model) pushToast(level notificationtoast.Level, title, message string) tea.Cmd {
	m.toastSerial++
	id := fmt.Sprintf("t-%d", m.toastSerial)
	m.tray = m.tray.Push(notificationtoast.Toast{
		ID:        id,
		Level:     level,
		Title:     title,
		Message:   message,
		ExpiresAt: time.Now().Add(3 * time.Second),
	})
	return nil
}

func (m model) View() string {
	width := m.width
	if width == 0 {
		width = 100
	}

	parts := []string{
		m.refreshBar().View(),
		m.tabs.View(),
		m.stream.View(),
		m.hints.View(),
	}
	background := strings.Join(parts, "\n")

	overlay := m.overlayView()
	if overlay == "" {
		return m.layerToasts(background)
	}

	bgW := lipgloss.Width(background)
	bgH := len(strings.Split(background, "\n"))
	placed := lipgloss.Place(bgW, bgH, lipgloss.Center, lipgloss.Center, overlay)
	return m.layerToasts(placed)
}

func (m model) overlayView() string {
	switch m.mode {
	case modeSourcePicker:
		return m.picker.View()
	case modeSearchPrompt:
		p := m.searchM.
			WithContent(m.search.View())
		return p.View()
	}
	return ""
}

// layerToasts overlays the toast tray on the top-right of the rendered view.
func (m model) layerToasts(view string) string {
	trayView := m.tray.View()
	if strings.TrimSpace(trayView) == "" {
		return view
	}
	bg := strings.Split(view, "\n")
	tr := strings.Split(trayView, "\n")
	bgW := lipgloss.Width(view)
	trW := lipgloss.Width(trayView)
	xOffset := bgW - trW - 2
	if xOffset < 0 {
		xOffset = 0
	}
	yOffset := 1
	for i, line := range tr {
		row := yOffset + i
		if row >= len(bg) {
			break
		}
		bg[row] = overlayLine(bg[row], line, xOffset)
	}
	return strings.Join(bg, "\n")
}

func overlayLine(base, top string, x int) string {
	baseW := lipgloss.Width(base)
	topW := lipgloss.Width(top)
	if x < 0 {
		x = 0
	}
	if x+topW > baseW {
		base = base + strings.Repeat(" ", x+topW-baseW)
	}
	left := lipgloss.NewStyle().MaxWidth(x).Render(base)
	right := top
	tail := ""
	if x+topW < lipgloss.Width(base) {
		tail = lipgloss.NewStyle().MaxWidth(lipgloss.Width(base) - x - topW).Render(base)
	}
	return left + right + tail
}

// synthesize produces a fresh log entry. The level/source choice is weighted
// so the stream reads realistically: mostly info, occasional warn/error.
func synthesize() logstream.Entry {
	src := sources[rand.Intn(len(sources))]
	pool := canned[src]
	pick := pool[rand.Intn(len(pool))]
	return logstream.Entry{
		Time:    time.Now(),
		Level:   pick.level,
		Source:  src,
		Message: pick.text,
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "log-viewer:", err)
		os.Exit(1)
	}
}
