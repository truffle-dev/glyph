package symbolsearch

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// minPromptWidth is the smallest outer width the modal will render at.
const minPromptWidth = 36

// Prompt is the immutable state of the workspace-symbol modal. Unlike
// rename.Prompt the input accepts any printable rune — workspace queries
// often include "." (gopls method-qualified search) and "/" (path-style
// filtering on language servers that support it). New() returns a closed
// prompt; Open() arms it with the empty query and the workspace label
// shown in the title.
type Prompt struct {
	open   bool
	label  string
	value  []rune
	cursor int
	err    string
}

// New returns a closed prompt.
func New() Prompt { return Prompt{} }

// Open arms the prompt with an empty query. label is shown in the title
// row (typically a short workspace name); empty is rendered as "symbol".
func (p Prompt) Open(label string) Prompt {
	p.open = true
	p.label = label
	p.value = nil
	p.cursor = 0
	p.err = ""
	return p
}

// OpenWith arms the prompt pre-filled with seed. Useful when the host
// wants to re-open the modal with the last query (the cursor is placed
// at the end of seed).
func (p Prompt) OpenWith(label, seed string) Prompt {
	p.open = true
	p.label = label
	p.value = []rune(seed)
	p.cursor = len(p.value)
	p.err = ""
	return p
}

// IsOpen reports whether the prompt is currently displayed.
func (p Prompt) IsOpen() bool { return p.open }

// Close returns a closed prompt. The previous value is dropped — callers
// who want to re-open with the prior query should snapshot Value() first.
func (p Prompt) Close() Prompt {
	p.open = false
	p.value = nil
	p.cursor = 0
	p.err = ""
	return p
}

// Value returns the current input string with leading/trailing space trimmed.
func (p Prompt) Value() string { return strings.TrimSpace(string(p.value)) }

// Raw returns the current input string without any trimming. Useful when
// the host wants to detect "no input" vs "whitespace input".
func (p Prompt) Raw() string { return string(p.value) }

// Cursor returns the current cursor rune index.
func (p Prompt) Cursor() int { return p.cursor }

// WithError surfaces an error message under the input row. Used by the
// host when the LSP returns an error or a zero-result query.
func (p Prompt) WithError(msg string) Prompt {
	p.err = msg
	return p
}

// Type appends a rune at the cursor. Any printable rune is accepted —
// workspace queries are free-form across language servers. Control runes
// and newlines are filtered.
func (p Prompt) Type(r rune) Prompt {
	if !isQueryRune(r) {
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

// Delete deletes the rune at the cursor (forward delete / Ctrl+D).
func (p Prompt) Delete() Prompt {
	if p.cursor >= len(p.value) {
		return p
	}
	p.value = append(p.value[:p.cursor], p.value[p.cursor+1:]...)
	p.err = ""
	return p
}

// Clear empties the input (Ctrl+U). Cursor returns to 0.
func (p Prompt) Clear() Prompt {
	p.value = nil
	p.cursor = 0
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
	if width < minPromptWidth {
		width = minPromptWidth
	}
	inner := width - 4 // 1+1 border, 1+1 padding
	if inner < 28 {
		inner = 28
	}

	label := p.label
	if label == "" {
		label = "symbol"
	}
	titleLine := "workspace symbol — " + label
	titleStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Surface).
		Width(inner)
	title := titleStyle.Render(clip(titleLine, inner))

	prefix := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Surface).
		Render("› ")
	used := lipgloss.Width("› ")
	inputWidth := inner - used
	if inputWidth < 4 {
		inputWidth = 4
	}
	input := renderInput(p.value, p.cursor, t, inputWidth)
	inputRow := lipgloss.NewStyle().
		Background(t.Surface).
		Width(inner).
		Render(prefix + input)

	hint := "enter to search · esc to cancel"
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
	used := lipgloss.Width(string(value))
	if cursor == len(value) {
		used++ // trailing caret
	}
	pad := width - used
	if pad > 0 {
		rendered += textStyle.Render(strings.Repeat(" ", pad))
	}
	return rendered
}

// isQueryRune returns true for printable runes accepted by the input.
// Letters, digits, underscore are obviously fine; we also accept dot,
// slash, dash, hash, dollar, asterisk, ampersand, colon, and most
// punctuation so language-server-specific query syntax (gopls method
// search "T.M", rust-analyzer fuzzy "f#m", etc.) passes through.
func isQueryRune(r rune) bool {
	if r < 0x20 || r == 0x7f {
		return false
	}
	if r == '\n' || r == '\r' || r == '\t' {
		return false
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	if unicode.IsPunct(r) || unicode.IsSymbol(r) || r == ' ' {
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
