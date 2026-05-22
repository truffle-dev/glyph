// Package chatinput renders a single-line chat prompt with a placeholder,
// cursor, and submit/escape key bindings. It's a thin Bubble Tea model — the
// consumer owns the code and is expected to extend it (history, multi-line,
// slash commands, completions) by editing the file directly.
package chatinput

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// SubmitMsg is emitted when the user presses Enter on a non-empty value.
type SubmitMsg struct {
	Value string
}

// CancelMsg is emitted when the user presses Esc.
type CancelMsg struct{}

// Input is a Bubble Tea model for a chat-style single-line text input.
type Input struct {
	theme       theme.Theme
	value       string
	placeholder string
	prompt      string
	width       int
	focused     bool
}

// New constructs an Input using the given theme. Focused by default.
func New(t theme.Theme) Input {
	return Input{
		theme:   t,
		prompt:  "> ",
		width:   60,
		focused: true,
	}
}

// WithPlaceholder sets the placeholder shown when the value is empty.
func (i Input) WithPlaceholder(s string) Input { i.placeholder = s; return i }

// WithPrompt sets the prompt prefix (default "> "). Use "" for no prompt.
func (i Input) WithPrompt(s string) Input { i.prompt = s; return i }

// WithWidth sets the overall input width in cells. Clamped to >= 8.
func (i Input) WithWidth(w int) Input {
	if w < 8 {
		w = 8
	}
	i.width = w
	return i
}

// WithValue presets the value. Useful for restored sessions.
func (i Input) WithValue(s string) Input { i.value = s; return i }

// Focus enables key input.
func (i Input) Focus() Input { i.focused = true; return i }

// Blur disables key input.
func (i Input) Blur() Input { i.focused = false; return i }

// Focused reports whether the input is currently accepting keys.
func (i Input) Focused() bool { return i.focused }

// Value returns the current text.
func (i Input) Value() string { return i.value }

// Reset clears the value to "".
func (i Input) Reset() Input { i.value = ""; return i }

// Init implements tea.Model.
func (i Input) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (i Input) Update(msg tea.Msg) (Input, tea.Cmd) {
	if !i.focused {
		return i, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return i, nil
	}
	switch key.Type {
	case tea.KeyEnter:
		v := strings.TrimRight(i.value, " ")
		if v == "" {
			return i, nil
		}
		out := i.value
		i.value = ""
		return i, func() tea.Msg { return SubmitMsg{Value: out} }
	case tea.KeyEsc:
		return i, func() tea.Msg { return CancelMsg{} }
	case tea.KeyBackspace:
		if len(i.value) > 0 {
			r := []rune(i.value)
			i.value = string(r[:len(r)-1])
		}
		return i, nil
	case tea.KeyCtrlU:
		i.value = ""
		return i, nil
	case tea.KeySpace:
		i.value += " "
		return i, nil
	case tea.KeyRunes:
		i.value += string(key.Runes)
		return i, nil
	}
	return i, nil
}

// View renders the input. Safe to call repeatedly.
func (i Input) View() string {
	prompt := lipgloss.NewStyle().Foreground(i.theme.Primary).Render(i.prompt)

	contentWidth := i.width - lipgloss.Width(prompt) - 2 // 2 for border padding
	if contentWidth < 4 {
		contentWidth = 4
	}

	var body string
	if i.value == "" {
		body = lipgloss.NewStyle().
			Foreground(i.theme.TextMuted).
			Render(truncate(i.placeholder, contentWidth))
	} else {
		visible := i.value
		if lipgloss.Width(visible) > contentWidth-1 {
			r := []rune(visible)
			start := len(r) - (contentWidth - 1)
			if start < 0 {
				start = 0
			}
			visible = string(r[start:])
		}
		body = lipgloss.NewStyle().Foreground(i.theme.Text).Render(visible)
	}

	cursor := ""
	if i.focused {
		cursor = lipgloss.NewStyle().
			Background(i.theme.Text).
			Foreground(i.theme.Bg).
			Render(" ")
	}

	line := prompt + body + cursor

	border := i.theme.Border
	if i.focused {
		border = i.theme.PrimaryStrong
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(i.width).
		Render(line)
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max < 2 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}
