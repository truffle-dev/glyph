// Package rename renders nook's symbol-rename prompt — a small modal that
// asks for the new identifier name and emits the value back to the host on
// Enter. The prompt is a pure value: the host owns when it appears and
// what placeholder it carries.
package rename

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// minWidth is the smallest outer width we'll try to render at.
const minWidth = 32

// Prompt is the immutable state of the rename modal. New() returns a
// closed prompt; WithCurrent re-arms it with the symbol's current name
// (pre-filled into the input field) and a path label for the title.
type Prompt struct {
	open     bool
	current  string
	pathHint string
	value    []rune
	cursor   int
	err      string
}

// New returns a closed prompt.
func New() Prompt { return Prompt{} }

// WithCurrent opens the prompt pre-filled with the symbol's current name.
// pathHint is a short relative file path shown in the title so the user
// can confirm the right buffer is being renamed.
func (p Prompt) WithCurrent(current, pathHint string) Prompt {
	p.open = true
	p.current = current
	p.pathHint = pathHint
	p.value = []rune(current)
	p.cursor = len(p.value)
	p.err = ""
	return p
}

// Open reports whether the prompt is currently displayed.
func (p Prompt) Open() bool { return p.open }

// Close returns a closed prompt.
func (p Prompt) Close() Prompt {
	p.open = false
	p.value = nil
	p.cursor = 0
	p.err = ""
	return p
}

// Value returns the current input string (without leading/trailing space).
func (p Prompt) Value() string { return strings.TrimSpace(string(p.value)) }

// Current returns the symbol's original name (for display + change detection).
func (p Prompt) Current() string { return p.current }

// WithError surfaces an error message under the input row. Used by the
// host when the LSP rejects a name as invalid.
func (p Prompt) WithError(msg string) Prompt {
	p.err = msg
	return p
}

// Type appends a rune at the cursor. Non-printable / non-identifier
// characters are ignored so the input field only ever carries a valid
// identifier shape.
func (p Prompt) Type(r rune) Prompt {
	if !isIdentRune(r) {
		return p
	}
	// First rune of an identifier cannot be a digit.
	if p.cursor == 0 && unicode.IsDigit(r) {
		return p
	}
	p.value = append(p.value[:p.cursor], append([]rune{r}, p.value[p.cursor:]...)...)
	p.cursor++
	p.err = ""
	return p
}

// Backspace deletes the rune to the left of the cursor.
func (p Prompt) Backspace() Prompt {
	if p.cursor == 0 {
		return p
	}
	p.value = append(p.value[:p.cursor-1], p.value[p.cursor:]...)
	p.cursor--
	p.err = ""
	return p
}

// MoveLeft moves the cursor one rune to the left.
func (p Prompt) MoveLeft() Prompt {
	if p.cursor > 0 {
		p.cursor--
	}
	return p
}

// MoveRight moves the cursor one rune to the right.
func (p Prompt) MoveRight() Prompt {
	if p.cursor < len(p.value) {
		p.cursor++
	}
	return p
}

// MoveHome moves the cursor to the start of the line.
func (p Prompt) MoveHome() Prompt { p.cursor = 0; return p }

// MoveEnd moves the cursor to the end of the line.
func (p Prompt) MoveEnd() Prompt { p.cursor = len(p.value); return p }

// View renders the prompt as a bordered modal at the given outer width.
// Returns "" when the prompt is closed.
func (p Prompt) View(t theme.Theme, width int) string {
	if !p.open {
		return ""
	}
	if width < minWidth {
		width = minWidth
	}
	inner := width - 4 // 1+1 border, 1+1 padding
	if inner < 24 {
		inner = 24
	}

	titleLine := "rename"
	if p.pathHint != "" {
		titleLine = "rename — " + p.pathHint
	}
	titleStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Surface).
		Width(inner)
	title := titleStyle.Render(clip(titleLine, inner))

	// Input line: "current → <value>" with cursor block.
	currentLabel := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Surface).
		Render(p.current + " → ")
	used := lipgloss.Width(p.current + " → ")
	inputWidth := inner - used
	if inputWidth < 4 {
		inputWidth = 4
	}
	input := renderInput(p.value, p.cursor, t, inputWidth)
	inputRow := lipgloss.NewStyle().
		Background(t.Surface).
		Width(inner).
		Render(currentLabel + input)

	hint := "enter to apply · esc to cancel"
	hintStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Surface).
		Width(inner)
	hintRow := hintStyle.Render(clip(hint, inner))

	rows := []string{title, "", inputRow}
	if p.err != "" {
		errStyle := lipgloss.NewStyle().
			Foreground(t.Error).
			Background(t.Surface).
			Width(inner)
		rows = append(rows, errStyle.Render(clip(p.err, inner)))
	}
	rows = append(rows, "", hintRow)

	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Background(t.Surface).
		Padding(0, 1).
		Width(inner).
		Render(body)
}

// renderInput draws the current value with a block-style cursor at the
// given rune index. The result is clipped to width.
func renderInput(value []rune, cursor int, t theme.Theme, width int) string {
	textStyle := lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface)
	cursorStyle := lipgloss.NewStyle().Foreground(t.Bg).Background(t.Primary).Bold(true)

	left := string(value[:cursor])
	var caret string
	var right string
	if cursor < len(value) {
		caret = string(value[cursor])
		right = string(value[cursor+1:])
	} else {
		caret = " "
		right = ""
	}
	rendered := textStyle.Render(left) + cursorStyle.Render(caret) + textStyle.Render(right)
	// pad to width
	used := lipgloss.Width(string(value))
	if cursor == len(value) {
		used++ // for the trailing caret
	}
	pad := width - used
	if pad > 0 {
		rendered += textStyle.Render(strings.Repeat(" ", pad))
	}
	return rendered
}

// isIdentRune returns true for runes that are valid in an identifier
// (letters, digits, underscore, plus a permissive Unicode letter range so
// non-ASCII names work in Go, Rust, etc.).
func isIdentRune(r rune) bool {
	switch {
	case r == '_':
		return true
	case unicode.IsLetter(r):
		return true
	case unicode.IsDigit(r):
		return true
	}
	return false
}

// clip truncates s to the given display-cell width, appending "…" when cut.
func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > width-1 {
			break
		}
		b.WriteRune(r)
		used += w
	}
	b.WriteRune('…')
	return b.String()
}
