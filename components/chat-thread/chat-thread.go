// Package chatthread renders a vertically-stacked conversation of role-aware
// chat bubbles with viewport-style scrolling. It composes chat-bubble and
// expects the consumer to drive Append/Scroll via their own tea.Model.
//
// Scroll is measured in lines from the bottom. offset == 0 means the latest
// message is visible at the bottom of the viewport. Increase offset to look
// back in history.
package chatthread

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	chatbubble "github.com/truffle-dev/glyph/components/chat-bubble"
	"github.com/truffle-dev/glyph/components/theme"
)

// Message is one row in the thread.
type Message struct {
	Role  chatbubble.Role
	Label string
	Text  string
}

// Thread holds an ordered list of messages and a line-based scroll offset.
type Thread struct {
	theme    theme.Theme
	messages []Message
	width    int
	height   int
	offset   int // lines scrolled up from the bottom
}

// New constructs an empty thread.
func New(t theme.Theme) Thread {
	return Thread{
		theme:  t,
		width:  80,
		height: 20,
	}
}

// WithSize sets the rendered width and height in cells.
func (th Thread) WithSize(w, h int) Thread {
	if w < 10 {
		w = 10
	}
	if h < 3 {
		h = 3
	}
	th.width = w
	th.height = h
	return th
}

// WithMessages replaces the entire message list and snaps to the bottom.
func (th Thread) WithMessages(msgs []Message) Thread {
	th.messages = append([]Message(nil), msgs...)
	th.offset = 0
	return th
}

// Append adds a message to the bottom of the thread. Snaps to the bottom
// unless the user has scrolled up — preserve their position in that case.
func (th Thread) Append(m Message) Thread {
	wasAtBottom := th.offset == 0
	th.messages = append(th.messages, m)
	if !wasAtBottom {
		// keep the user where they were
		return th
	}
	return th
}

// Messages returns the current message list.
func (th Thread) Messages() []Message { return th.messages }

// ScrollUp moves the viewport up by n lines.
func (th Thread) ScrollUp(n int) Thread {
	th.offset += n
	max := th.maxOffset()
	if th.offset > max {
		th.offset = max
	}
	return th
}

// ScrollDown moves the viewport down by n lines.
func (th Thread) ScrollDown(n int) Thread {
	th.offset -= n
	if th.offset < 0 {
		th.offset = 0
	}
	return th
}

// ScrollToBottom snaps to the latest line.
func (th Thread) ScrollToBottom() Thread {
	th.offset = 0
	return th
}

// ScrollToTop scrolls all the way up.
func (th Thread) ScrollToTop() Thread {
	th.offset = th.maxOffset()
	return th
}

// Init implements tea.Model.
func (th Thread) Init() tea.Cmd { return nil }

// Update handles arrow-key and page scrolling. Unknown messages are no-ops.
func (th Thread) Update(msg tea.Msg) (Thread, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return th, nil
	}
	switch key.Type {
	case tea.KeyUp:
		return th.ScrollUp(1), nil
	case tea.KeyDown:
		return th.ScrollDown(1), nil
	case tea.KeyPgUp:
		return th.ScrollUp(th.height / 2), nil
	case tea.KeyPgDown:
		return th.ScrollDown(th.height / 2), nil
	case tea.KeyHome:
		return th.ScrollToTop(), nil
	case tea.KeyEnd:
		return th.ScrollToBottom(), nil
	}
	return th, nil
}

// View renders the thread to a fixed-size string.
func (th Thread) View() string {
	if len(th.messages) == 0 {
		return lipgloss.NewStyle().
			Foreground(th.theme.TextMuted).
			Width(th.width).
			Height(th.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No messages yet.")
	}

	bubbleWidth := th.width - 2
	if bubbleWidth < 6 {
		bubbleWidth = 6
	}

	rendered := make([]string, 0, len(th.messages))
	for _, m := range th.messages {
		b := chatbubble.New(th.theme).
			WithRole(m.Role).
			WithText(m.Text).
			WithWidth(bubbleWidth)
		if m.Label != "" {
			b = b.WithLabel(m.Label)
		}
		rendered = append(rendered, b.View())
	}
	full := strings.Join(rendered, "\n\n")

	lines := strings.Split(full, "\n")
	end := len(lines) - th.offset
	if end > len(lines) {
		end = len(lines)
	}
	if end < 0 {
		end = 0
	}
	start := end - th.height
	if start < 0 {
		start = 0
	}
	window := lines[start:end]
	for len(window) < th.height {
		window = append([]string{""}, window...)
	}

	return lipgloss.NewStyle().Width(th.width).Render(strings.Join(window, "\n"))
}

// maxOffset is the largest scrollback offset that still shows content.
func (th Thread) maxOffset() int {
	total := th.totalLines()
	if total <= th.height {
		return 0
	}
	return total - th.height
}

// totalLines counts the rendered line count without re-rendering full View().
func (th Thread) totalLines() int {
	if len(th.messages) == 0 {
		return 0
	}
	bubbleWidth := th.width - 2
	if bubbleWidth < 6 {
		bubbleWidth = 6
	}
	total := 0
	for i, m := range th.messages {
		b := chatbubble.New(th.theme).
			WithRole(m.Role).
			WithText(m.Text).
			WithWidth(bubbleWidth)
		if m.Label != "" {
			b = b.WithLabel(m.Label)
		}
		total += strings.Count(b.View(), "\n") + 1
		if i < len(th.messages)-1 {
			total++ // blank line separator
		}
	}
	return total
}
