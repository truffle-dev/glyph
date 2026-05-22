// Package textinput renders a multi-line text input with a 2D cursor,
// placeholder, focus, word-jumps on Alt+Left/Right, and Ctrl-U to kill the
// current line up to the cursor. It is a thin Bubble Tea model — the consumer
// owns the code and is expected to extend it (history, paste handlers, line
// wrapping, syntax styling) by editing the file directly.
package textinput

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// SubmitMsg is emitted when the user presses Ctrl-D on a non-empty value.
type SubmitMsg struct {
	Value string
}

// CancelMsg is emitted when the user presses Esc.
type CancelMsg struct{}

// Input is a Bubble Tea model for a multi-line text input.
type Input struct {
	theme       theme.Theme
	lines       [][]rune
	row, col    int
	placeholder string
	width       int
	height      int
	focused     bool
}

// New constructs an Input using the given theme. Focused by default, with a
// single empty line, width 60 cells, and four visible rows.
func New(t theme.Theme) Input {
	return Input{
		theme:   t,
		lines:   [][]rune{{}},
		width:   60,
		height:  4,
		focused: true,
	}
}

// WithPlaceholder sets the placeholder shown when the value is empty.
func (i Input) WithPlaceholder(s string) Input { i.placeholder = s; return i }

// WithWidth sets the overall input width in cells. Clamped to >= 8.
func (i Input) WithWidth(w int) Input {
	if w < 8 {
		w = 8
	}
	i.width = w
	return i
}

// WithHeight sets the number of visible rows. Clamped to >= 1.
func (i Input) WithHeight(h int) Input {
	if h < 1 {
		h = 1
	}
	i.height = h
	return i
}

// WithValue presets the value. Newlines are honored; the cursor lands at the
// end of the last line.
func (i Input) WithValue(s string) Input {
	parts := strings.Split(s, "\n")
	i.lines = make([][]rune, len(parts))
	for k, p := range parts {
		i.lines[k] = []rune(p)
	}
	i.row = len(i.lines) - 1
	i.col = len(i.lines[i.row])
	return i
}

// Focus enables key input.
func (i Input) Focus() Input { i.focused = true; return i }

// Blur disables key input.
func (i Input) Blur() Input { i.focused = false; return i }

// Focused reports whether the input is currently accepting keys.
func (i Input) Focused() bool { return i.focused }

// Value returns the current text with rows joined by "\n".
func (i Input) Value() string {
	parts := make([]string, len(i.lines))
	for k, l := range i.lines {
		parts[k] = string(l)
	}
	return strings.Join(parts, "\n")
}

// Cursor returns the current 0-based (row, col) cursor position.
func (i Input) Cursor() (int, int) { return i.row, i.col }

// Reset clears the value back to a single empty line and homes the cursor.
func (i Input) Reset() Input {
	i.lines = [][]rune{{}}
	i.row, i.col = 0, 0
	return i
}

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
	case tea.KeyEsc:
		return i, func() tea.Msg { return CancelMsg{} }
	case tea.KeyCtrlD:
		if i.isEmpty() {
			return i, nil
		}
		return i, func() tea.Msg { return SubmitMsg{Value: i.Value()} }
	case tea.KeyEnter:
		cur := i.lines[i.row]
		before := append([]rune{}, cur[:i.col]...)
		after := append([]rune{}, cur[i.col:]...)
		i.lines[i.row] = before
		tail := append([][]rune{after}, i.lines[i.row+1:]...)
		i.lines = append(i.lines[:i.row+1], tail...)
		i.row++
		i.col = 0
		return i, nil
	case tea.KeyBackspace:
		if i.col > 0 {
			cur := i.lines[i.row]
			i.lines[i.row] = append(append([]rune{}, cur[:i.col-1]...), cur[i.col:]...)
			i.col--
		} else if i.row > 0 {
			prevLen := len(i.lines[i.row-1])
			i.lines[i.row-1] = append(i.lines[i.row-1], i.lines[i.row]...)
			i.lines = append(i.lines[:i.row], i.lines[i.row+1:]...)
			i.row--
			i.col = prevLen
		}
		return i, nil
	case tea.KeyCtrlU:
		cur := i.lines[i.row]
		i.lines[i.row] = append([]rune{}, cur[i.col:]...)
		i.col = 0
		return i, nil
	case tea.KeyCtrlK:
		cur := i.lines[i.row]
		i.lines[i.row] = append([]rune{}, cur[:i.col]...)
		return i, nil
	case tea.KeyLeft:
		if key.Alt {
			i.col = wordLeft(i.lines[i.row], i.col)
			return i, nil
		}
		if i.col > 0 {
			i.col--
		} else if i.row > 0 {
			i.row--
			i.col = len(i.lines[i.row])
		}
		return i, nil
	case tea.KeyRight:
		if key.Alt {
			i.col = wordRight(i.lines[i.row], i.col)
			return i, nil
		}
		if i.col < len(i.lines[i.row]) {
			i.col++
		} else if i.row < len(i.lines)-1 {
			i.row++
			i.col = 0
		}
		return i, nil
	case tea.KeyUp:
		if i.row > 0 {
			i.row--
			if i.col > len(i.lines[i.row]) {
				i.col = len(i.lines[i.row])
			}
		}
		return i, nil
	case tea.KeyDown:
		if i.row < len(i.lines)-1 {
			i.row++
			if i.col > len(i.lines[i.row]) {
				i.col = len(i.lines[i.row])
			}
		}
		return i, nil
	case tea.KeyHome:
		i.col = 0
		return i, nil
	case tea.KeyEnd:
		i.col = len(i.lines[i.row])
		return i, nil
	case tea.KeySpace:
		return i.insert([]rune{' '}), nil
	case tea.KeyRunes:
		return i.insert(key.Runes), nil
	}
	return i, nil
}

func (i Input) insert(rs []rune) Input {
	cur := i.lines[i.row]
	out := make([]rune, 0, len(cur)+len(rs))
	out = append(out, cur[:i.col]...)
	out = append(out, rs...)
	out = append(out, cur[i.col:]...)
	i.lines[i.row] = out
	i.col += len(rs)
	return i
}

func (i Input) isEmpty() bool {
	return len(i.lines) == 1 && len(i.lines[0]) == 0
}

// View renders the input. Safe to call repeatedly.
func (i Input) View() string {
	contentWidth := i.width - 4 // border (2) + padding (2)
	if contentWidth < 4 {
		contentWidth = 4
	}

	rows := make([]string, i.height)

	if i.isEmpty() {
		phStyle := lipgloss.NewStyle().Foreground(i.theme.TextMuted)
		if i.focused {
			cursor := lipgloss.NewStyle().
				Background(i.theme.Text).
				Foreground(i.theme.Bg).
				Render(" ")
			rows[0] = cursor + phStyle.Render(truncate(i.placeholder, contentWidth-1))
		} else {
			rows[0] = phStyle.Render(truncate(i.placeholder, contentWidth))
		}
		for k := 1; k < i.height; k++ {
			rows[k] = ""
		}
	} else {
		for k := 0; k < i.height; k++ {
			if k < len(i.lines) {
				rows[k] = i.renderLine(k, contentWidth)
			} else {
				rows[k] = ""
			}
		}
	}

	body := strings.Join(rows, "\n")

	border := i.theme.Border
	if i.focused {
		border = i.theme.PrimaryStrong
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(i.width).
		Render(body)
}

func (i Input) renderLine(idx, contentWidth int) string {
	line := i.lines[idx]
	textStyle := lipgloss.NewStyle().Foreground(i.theme.Text)
	if !(i.focused && idx == i.row) {
		return textStyle.Render(truncate(string(line), contentWidth))
	}

	visible := line
	col := i.col
	// horizontal scroll: keep the cursor visible
	if len(visible) > contentWidth-1 {
		half := contentWidth / 2
		start := col - half
		if start < 0 {
			start = 0
		}
		end := start + contentWidth - 1
		if end > len(visible) {
			end = len(visible)
			start = end - (contentWidth - 1)
			if start < 0 {
				start = 0
			}
		}
		visible = visible[start:end]
		col = i.col - start
	}

	if col < 0 {
		col = 0
	}
	if col > len(visible) {
		col = len(visible)
	}

	before := textStyle.Render(string(visible[:col]))
	cursorChar := " "
	afterStart := col
	if col < len(visible) {
		cursorChar = string(visible[col])
		afterStart = col + 1
	}
	cursor := lipgloss.NewStyle().
		Background(i.theme.Text).
		Foreground(i.theme.Bg).
		Render(cursorChar)
	after := textStyle.Render(string(visible[afterStart:]))
	return before + cursor + after
}

func wordLeft(line []rune, col int) int {
	if col <= 0 {
		return 0
	}
	i := col
	for i > 0 && unicode.IsSpace(line[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(line[i-1]) {
		i--
	}
	return i
}

func wordRight(line []rune, col int) int {
	if col >= len(line) {
		return len(line)
	}
	i := col
	for i < len(line) && !unicode.IsSpace(line[i]) {
		i++
	}
	for i < len(line) && unicode.IsSpace(line[i]) {
		i++
	}
	return i
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
