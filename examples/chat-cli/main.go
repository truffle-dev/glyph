// Command chat-cli is a realistic agent-style chat REPL composed from
// thirteen glyph components. It is intentionally not a tabbed showcase —
// every component on screen earns its place by serving the chat surface.
//
// What runs on screen at once:
//
//   - status-bar at the top: model name, message count, mode badge
//   - chat-thread (chat-bubble) in the middle, scrollable
//   - chat-input at the bottom, focused by default
//   - key-hints below the input, advertising current bindings
//   - notification-toast tray floating in the top-right
//   - spinner inline next to the assistant label while a reply is in flight
//
// Surfaces that overlay the chat on demand:
//
//   - command-palette (Ctrl-P, /): slash commands
//   - modal + text-input: "Save conversation" filename prompt
//   - modal + confirmation: "Clear all messages?" prompt
//   - select: model picker
//
// All thirteen components are real, untouched copies of the registry
// versions. There is no glyph-specific glue in this file — only Bubble
// Tea composition. That's the demo: how the pieces fit together.
//
// Run it:
//
//	go run ./examples/chat-cli/
//
// Quit at any time with Ctrl-C.
package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	chatbubble "github.com/truffle-dev/glyph/components/chat-bubble"
	chatinput "github.com/truffle-dev/glyph/components/chat-input"
	chatthread "github.com/truffle-dev/glyph/components/chat-thread"
	commandpalette "github.com/truffle-dev/glyph/components/command-palette"
	"github.com/truffle-dev/glyph/components/confirmation"
	keyhints "github.com/truffle-dev/glyph/components/key-hints"
	"github.com/truffle-dev/glyph/components/modal"
	notificationtoast "github.com/truffle-dev/glyph/components/notification-toast"
	selectinput "github.com/truffle-dev/glyph/components/select"
	"github.com/truffle-dev/glyph/components/spinner"
	statusbar "github.com/truffle-dev/glyph/components/status-bar"
	textinput "github.com/truffle-dev/glyph/components/text-input"
	"github.com/truffle-dev/glyph/components/theme"
)

// mode controls which overlay (if any) intercepts key events.
type mode int

const (
	modeChat mode = iota
	modePalette
	modeSaveDialog
	modeClearConfirm
	modeModelPicker
)

// tickMsg drives toast expiry once per second.
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// replyMsg lands when the fake assistant has "finished thinking".
type replyMsg struct {
	text string
}

// fakeReply schedules a reply after a short delay. In a real CLI this
// would be a streaming model call; here it's a goroutine + Tick so the
// demo runs offline.
func fakeReply(prompt string) tea.Cmd {
	delay := time.Duration(700+rand.Intn(900)) * time.Millisecond
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return replyMsg{text: respondTo(prompt)}
	})
}

// respondTo returns a canned reply for the given user prompt. The set
// is deliberately small and deterministic so the demo reads well in
// a recorded GIF.
func respondTo(prompt string) string {
	p := strings.ToLower(strings.TrimSpace(prompt))
	switch {
	case strings.Contains(p, "hello"), strings.Contains(p, "hi"):
		return "Hi. I'm a demo of glyph's chat surface. Try **Ctrl-P** for the command palette, or **Ctrl-S** to save the conversation."
	case strings.Contains(p, "components"), strings.Contains(p, "what is"), strings.Contains(p, "explain"):
		return "Thirteen glyph components are on screen right now: status-bar (top), chat-thread + chat-bubble + spinner (here), chat-input + key-hints (bottom), notification-toast (overlays), and the four overlays you can open: command-palette, modal+text-input, modal+confirmation, and select."
	case strings.Contains(p, "save"):
		return "Press **Ctrl-S** to open a save dialog. It uses the `modal` component with `text-input` inside it."
	case strings.Contains(p, "clear"):
		return "Press **Ctrl-L** to clear. It uses `modal` with `confirmation` for a destructive-action prompt."
	case strings.Contains(p, "model"):
		return "Press **Ctrl-R** to switch models. That picker is the `select` component."
	default:
		// Generic acknowledgement keeps the demo conversational without
		// pretending to do real reasoning.
		return "Noted. Try a slash command via **Ctrl-P** to see more of the chat surface."
	}
}

type model struct {
	theme theme.Theme

	mode mode

	// foundational
	width, height int

	// chat surface
	thread chatthread.Thread
	input  chatinput.Input
	bar    statusbar.Bar
	hints  keyhints.Bar
	tray   notificationtoast.Tray

	// thinking state
	thinking      bool
	thinkSpinner  spinner.Spinner
	thinkPrompt   string
	thinkingSince time.Time

	// overlays
	palette  commandpalette.Palette
	saveBox  textinput.Input
	saveBoxM modal.Modal
	confirm  confirmation.Confirm
	confirmM modal.Modal
	picker   selectinput.Select

	// session state
	modelName   string
	msgCount    int
	toastSerial int
}

func newModel() model {
	t := theme.Default

	thread := chatthread.New(t).
		WithSize(72, 16).
		WithMessages([]chatthread.Message{
			{
				Role:  chatbubble.RoleAssistant,
				Label: "glyph",
				Text:  "Welcome. I'm a demo agent composed entirely from glyph components. Type a message and press Enter, or open the command palette with Ctrl-P.",
			},
		})

	in := chatinput.New(t).
		WithPlaceholder("Type a message…").
		WithPrompt("you › ").
		WithWidth(72).
		Focus()

	bar := statusbar.New(t).
		WithWidth(72).
		WithLeft(statusbar.Item{Text: "● chat", Style: statusbar.StyleSuccess}).
		WithCenter(statusbar.Item{Text: "Opus 4.7", Style: statusbar.StylePrimary}).
		WithRight(statusbar.Item{Text: "1 msg"})

	hints := keyhints.New(t).
		WithWidth(72).
		WithHints(chatHints())

	tray := notificationtoast.New(t).WithWidth(36).WithMaxItems(3)

	sp := spinner.New(t).
		WithStyle(spinner.StyleDots).
		WithLabel("thinking").
		WithInterval(80 * time.Millisecond)

	palette := commandpalette.New(t).
		WithSize(56, 12).
		WithTitle("Commands").
		WithPlaceholder("type to filter…").
		WithCommands([]commandpalette.Command{
			{ID: "save", Title: "Save conversation", Description: "Write the thread to a markdown file.", Keybinding: "ctrl+s"},
			{ID: "clear", Title: "Clear messages", Description: "Drop every message and start over.", Keybinding: "ctrl+l"},
			{ID: "model", Title: "Switch model", Description: "Pick a different model for the next reply.", Keybinding: "ctrl+r"},
			{ID: "help", Title: "Help", Description: "List bindings the chat surface accepts."},
			{ID: "quit", Title: "Quit", Description: "Exit chat-cli.", Keybinding: "ctrl+c"},
		})

	saveBox := textinput.New(t).
		WithPlaceholder("filename.md").
		WithWidth(40)

	saveBoxM := modal.New(t).
		WithTitle("Save conversation").
		WithBody("").
		WithFooter("⌃D to save · Esc to cancel").
		WithSize(50, 8)

	confirm := confirmation.New(t).
		WithPrompt("Clear all messages? This cannot be undone.").
		WithYesLabel("Clear").
		WithNoLabel("Cancel").
		WithDangerous(true).
		WithWidth(46)

	confirmM := modal.New(t).
		WithTitle("Confirm").
		WithBody("").
		WithFooter("Tab / Y / N").
		WithSize(50, 8)

	picker := selectinput.New(t).
		WithTitle("Switch model").
		WithOptions([]selectinput.Option{
			{Label: "Opus 4.7", Hint: "deep reasoning"},
			{Label: "Sonnet 4.6", Hint: "fast and balanced"},
			{Label: "Haiku 4.5", Hint: "quick replies"},
		}).
		WithSize(40, 6)

	return model{
		theme:        t,
		mode:         modeChat,
		thread:       thread,
		input:        in,
		bar:          bar,
		hints:        hints,
		tray:         tray,
		thinkSpinner: sp,
		palette:      palette,
		saveBox:      saveBox,
		saveBoxM:     saveBoxM,
		confirm:      confirm,
		confirmM:     confirmM,
		picker:       picker,
		modelName:    "Opus 4.7",
		msgCount:     1,
	}
}

func chatHints() []keyhints.Hint {
	return []keyhints.Hint{
		{Key: "⏎", Desc: "send"},
		{Key: "⌃P", Desc: "palette"},
		{Key: "⌃S", Desc: "save"},
		{Key: "⌃L", Desc: "clear"},
		{Key: "⌃R", Desc: "model"},
		{Key: "⌃C", Desc: "quit"},
	}
}

func paletteHints() []keyhints.Hint {
	return []keyhints.Hint{
		{Key: "↑↓", Desc: "navigate"},
		{Key: "⏎", Desc: "run"},
		{Key: "Esc", Desc: "close"},
	}
}

func modalHints(extra ...keyhints.Hint) []keyhints.Hint {
	return append(extra, keyhints.Hint{Key: "Esc", Desc: "cancel"})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.thinkSpinner.Init(), tick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		threadH := msg.Height - 6 // status + input + hints + spinner + padding
		if threadH < 6 {
			threadH = 6
		}
		w := msg.Width
		if w < 40 {
			w = 40
		}
		m.thread = m.thread.WithSize(w, threadH)
		m.input = m.input.WithWidth(w)
		m.bar = m.bar.WithWidth(w)
		m.hints = m.hints.WithWidth(w)
		return m, nil

	case tickMsg:
		m.tray = m.tray.Tick(time.Time(msg))
		return m, tick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.thinkSpinner, cmd = m.thinkSpinner.Update(msg)
		return m, cmd

	case replyMsg:
		m.thinking = false
		m.thread = m.thread.Append(chatthread.Message{
			Role:  chatbubble.RoleAssistant,
			Label: "glyph",
			Text:  msg.text,
		}).ScrollToBottom()
		m.msgCount++
		m.bar = m.refreshBar()
		return m, nil

	case commandpalette.SelectMsg:
		m.mode = modeChat
		m.hints = m.hints.WithHints(chatHints())
		return m.runCommand(msg.Command.ID)

	case commandpalette.CancelMsg:
		m.mode = modeChat
		m.hints = m.hints.WithHints(chatHints())
		return m, nil

	case selectinput.SelectMsg:
		m.modelName = msg.Option.Label
		m.bar = m.refreshBar()
		m.mode = modeChat
		m.hints = m.hints.WithHints(chatHints())
		return m, m.pushToast(notificationtoast.LevelInfo, "Model", "Switched to "+msg.Option.Label)

	case selectinput.CancelMsg:
		m.mode = modeChat
		m.hints = m.hints.WithHints(chatHints())
		return m, nil

	case confirmation.ConfirmMsg:
		m.mode = modeChat
		m.hints = m.hints.WithHints(chatHints())
		if !msg.Value {
			return m, nil
		}
		m.thread = m.thread.WithMessages(nil)
		m.msgCount = 0
		m.bar = m.refreshBar()
		return m, m.pushToast(notificationtoast.LevelSuccess, "Cleared", "All messages dropped.")

	case confirmation.CancelMsg:
		m.mode = modeChat
		m.hints = m.hints.WithHints(chatHints())
		return m, nil

	case textinput.SubmitMsg:
		if m.mode == modeSaveDialog {
			name := strings.TrimSpace(msg.Value)
			if name == "" {
				name = "chat.md"
			}
			m.mode = modeChat
			m.hints = m.hints.WithHints(chatHints())
			m.saveBox = m.saveBox.Reset()
			return m, m.pushToast(notificationtoast.LevelSuccess, "Saved", "Wrote "+name)
		}

	case textinput.CancelMsg:
		if m.mode == modeSaveDialog {
			m.mode = modeChat
			m.hints = m.hints.WithHints(chatHints())
			m.saveBox = m.saveBox.Reset()
			return m, nil
		}

	case chatinput.SubmitMsg:
		text := strings.TrimSpace(msg.Value)
		if text == "" {
			return m, nil
		}
		m.input = m.input.Reset()
		m.thread = m.thread.Append(chatthread.Message{
			Role:  chatbubble.RoleUser,
			Label: "you",
			Text:  text,
		}).ScrollToBottom()
		m.msgCount++
		m.thinking = true
		m.thinkPrompt = text
		m.thinkingSince = time.Now()
		m.bar = m.refreshBar()
		return m, tea.Batch(m.thinkSpinner.Init(), fakeReply(text))

	case tea.KeyMsg:
		return m.routeKey(msg)
	}

	return m, nil
}

func (m model) routeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Universal quit binding. Ctrl-C always exits no matter the mode.
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.mode {

	case modePalette:
		// Esc closes the palette without selecting anything.
		if msg.String() == "esc" {
			m.mode = modeChat
			m.hints = m.hints.WithHints(chatHints())
			return m, nil
		}
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		return m, cmd

	case modeSaveDialog:
		var c1, c2 tea.Cmd
		m.saveBox, c1 = m.saveBox.Update(msg)
		m.saveBoxM, c2 = m.saveBoxM.Update(msg)
		return m, tea.Batch(c1, c2)

	case modeClearConfirm:
		var c1, c2 tea.Cmd
		m.confirm, c1 = m.confirm.Update(msg)
		m.confirmM, c2 = m.confirmM.Update(msg)
		return m, tea.Batch(c1, c2)

	case modeModelPicker:
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd

	case modeChat:
		switch msg.String() {
		case "ctrl+p", "ctrl+k":
			m.mode = modePalette
			m.palette = m.palette.WithFilter("")
			m.hints = m.hints.WithHints(paletteHints())
			return m, nil
		case "ctrl+s":
			return m.openSaveDialog()
		case "ctrl+l":
			return m.openClearConfirm()
		case "ctrl+r":
			m.mode = modeModelPicker
			m.hints = m.hints.WithHints(modalHints())
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) runCommand(id string) (tea.Model, tea.Cmd) {
	switch id {
	case "save":
		return m.openSaveDialog()
	case "clear":
		return m.openClearConfirm()
	case "model":
		m.mode = modeModelPicker
		m.hints = m.hints.WithHints(modalHints())
		return m, nil
	case "help":
		help := "Bindings:\n\n• ⏎ send  • ⌃P palette  • ⌃S save\n• ⌃L clear  • ⌃R model  • Esc close overlay\n• ⌃C quit"
		m.thread = m.thread.Append(chatthread.Message{
			Role:  chatbubble.RoleSystem,
			Label: "help",
			Text:  help,
		}).ScrollToBottom()
		m.msgCount++
		m.bar = m.refreshBar()
		return m, nil
	case "quit":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) openSaveDialog() (tea.Model, tea.Cmd) {
	m.mode = modeSaveDialog
	m.saveBox = m.saveBox.Reset()
	m.hints = m.hints.WithHints([]keyhints.Hint{
		{Key: "⌃D", Desc: "save"},
		{Key: "Esc", Desc: "cancel"},
	})
	return m, nil
}

func (m model) openClearConfirm() (tea.Model, tea.Cmd) {
	if m.msgCount == 0 {
		return m, m.pushToast(notificationtoast.LevelInfo, "Nothing to clear", "The thread is already empty.")
	}
	m.mode = modeClearConfirm
	m.confirm = confirmation.New(m.theme).
		WithPrompt(fmt.Sprintf("Clear all %d messages? This cannot be undone.", m.msgCount)).
		WithYesLabel("Clear").
		WithNoLabel("Cancel").
		WithDangerous(true).
		WithWidth(46)
	m.hints = m.hints.WithHints([]keyhints.Hint{
		{Key: "Tab", Desc: "switch"},
		{Key: "y/n", Desc: "shortcut"},
		{Key: "Esc", Desc: "cancel"},
	})
	return m, nil
}

func (m *model) pushToast(level notificationtoast.Level, title, message string) tea.Cmd {
	m.toastSerial++
	id := fmt.Sprintf("t-%d", m.toastSerial)
	m.tray = m.tray.Push(notificationtoast.Toast{
		ID:        id,
		Level:     level,
		Title:     title,
		Message:   message,
		ExpiresAt: time.Now().Add(4 * time.Second),
	})
	return nil
}

func (m model) refreshBar() statusbar.Bar {
	mode := statusbar.Item{Text: "● chat", Style: statusbar.StyleSuccess}
	switch m.mode {
	case modePalette:
		mode = statusbar.Item{Text: "▶ palette", Style: statusbar.StylePrimary}
	case modeSaveDialog:
		mode = statusbar.Item{Text: "▶ save", Style: statusbar.StylePrimary}
	case modeClearConfirm:
		mode = statusbar.Item{Text: "▶ confirm", Style: statusbar.StyleWarning}
	case modeModelPicker:
		mode = statusbar.Item{Text: "▶ model", Style: statusbar.StylePrimary}
	}
	count := fmt.Sprintf("%d msg", m.msgCount)
	if m.msgCount != 1 {
		count = fmt.Sprintf("%d msgs", m.msgCount)
	}
	return m.bar.
		WithLeft(mode).
		WithCenter(statusbar.Item{Text: m.modelName, Style: statusbar.StylePrimary}).
		WithRight(statusbar.Item{Text: count})
}

func (m model) View() string {
	width := m.width
	if width == 0 {
		width = 80
	}

	body := m.thread.View()
	if m.thinking {
		elapsed := time.Since(m.thinkingSince).Truncate(100 * time.Millisecond)
		spinView := lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Render(fmt.Sprintf("  glyph › %s  %s", m.thinkSpinner.View(), elapsed))
		body = body + "\n" + spinView
	}

	parts := []string{
		m.refreshBar().View(),
		body,
		m.input.View(),
		m.hints.View(),
	}
	background := strings.Join(parts, "\n")

	overlay := m.overlayView()
	if overlay == "" {
		return m.layerToasts(background)
	}

	// Place the overlay over the chat background.
	lines := strings.Split(background, "\n")
	bgH := len(lines)
	bgW := lipgloss.Width(background)
	placed := lipgloss.Place(bgW, bgH, lipgloss.Center, lipgloss.Center, overlay)
	return m.layerToasts(placed)
}

// overlayView returns the active overlay rendered as a string, or "" when
// the chat is the only thing on screen.
func (m model) overlayView() string {
	switch m.mode {
	case modePalette:
		return m.palette.View()
	case modeSaveDialog:
		inside := m.saveBox.View()
		body := lipgloss.NewStyle().Foreground(m.theme.TextMuted).Render("Save the conversation as:") + "\n\n" + inside
		return m.saveBoxM.WithBody(body).View()
	case modeClearConfirm:
		return m.confirmM.WithBody(m.confirm.View()).View()
	case modeModelPicker:
		return m.picker.View()
	}
	return ""
}

// layerToasts paints the floating toast tray over the top-right of the
// rendered view. Lines that fall under a toast are replaced cell-for-cell.
func (m model) layerToasts(view string) string {
	trayView := m.tray.View()
	if strings.TrimSpace(trayView) == "" {
		return view
	}
	bg := strings.Split(view, "\n")
	tr := strings.Split(trayView, "\n")
	bgW := lipgloss.Width(view)
	trW := lipgloss.Width(trayView)
	margin := 2
	xOffset := bgW - trW - margin
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

// overlayLine replaces the printable cells of base starting at column x
// with the cells of top, preserving ANSI styling of top. The implementation
// keeps it simple: it pads base with spaces, then concatenates by visual
// columns using lipgloss.PlaceHorizontal.
func overlayLine(base, top string, x int) string {
	baseW := lipgloss.Width(base)
	topW := lipgloss.Width(top)
	if x < 0 {
		x = 0
	}
	if x+topW > baseW {
		// Pad base out with spaces until top fits.
		base = base + strings.Repeat(" ", x+topW-baseW)
	}
	left := lipgloss.NewStyle().Width(x).Render(truncatePlain(base, x))
	right := top
	tail := ""
	if x+topW < lipgloss.Width(base) {
		tail = trimVisibleLeft(base, x+topW)
	}
	return left + right + tail
}

func truncatePlain(s string, w int) string {
	if w <= 0 {
		return ""
	}
	out := lipgloss.NewStyle().MaxWidth(w).Render(s)
	return out
}

func trimVisibleLeft(s string, x int) string {
	// Crude trim: render the string at full width, then crop with lipgloss.
	width := lipgloss.Width(s)
	if x >= width {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width - x).Render(s)
}
