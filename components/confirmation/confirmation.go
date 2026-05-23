// Package confirmation renders a two-button yes/no prompt with focus-managed
// buttons, single-keystroke y/n shortcuts, and dangerous-action styling.
//
// The component is a thin Bubble Tea model. The parent is expected to embed
// it inside a modal or inline panel and listen for ConfirmMsg / CancelMsg
// from the returned tea.Cmd. The prompt text reflows to the configured
// width using muesli/reflow/wordwrap so long questions render cleanly.
package confirmation

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"

	"github.com/truffle-dev/glyph/components/theme"
)

// ConfirmMsg is emitted when the user commits with Enter or a y/n keystroke.
// Value is true when Yes was selected, false when No.
type ConfirmMsg struct {
	Value bool
}

// CancelMsg is emitted when the user presses Esc.
type CancelMsg struct{}

// Confirm is a Bubble Tea model for a yes/no prompt.
type Confirm struct {
	theme      theme.Theme
	prompt     string
	yesLabel   string
	noLabel    string
	focusedYes bool
	dangerous  bool
	width      int
}

// New constructs a Confirm using the given theme. Defaults: labels "Yes" and
// "No", focus on No (the safer default for destructive prompts), reflow
// width 60.
func New(t theme.Theme) Confirm {
	return Confirm{
		theme:      t,
		yesLabel:   "Yes",
		noLabel:    "No",
		focusedYes: false,
		width:      60,
	}
}

// WithPrompt sets the question text shown above the buttons. Required for a
// sensible view; an empty prompt renders only the button row and hint.
func (c Confirm) WithPrompt(s string) Confirm { c.prompt = s; return c }

// WithYesLabel overrides the "Yes" button label.
func (c Confirm) WithYesLabel(s string) Confirm { c.yesLabel = s; return c }

// WithNoLabel overrides the "No" button label.
func (c Confirm) WithNoLabel(s string) Confirm { c.noLabel = s; return c }

// WithDefault sets which button has initial focus. Passing true focuses Yes;
// passing false focuses No. The default is No.
func (c Confirm) WithDefault(yes bool) Confirm { c.focusedYes = yes; return c }

// WithDangerous toggles destructive-action styling. When true, the Yes
// button uses theme.Error for its foreground and focus background so the
// prompt visually warns the user before they commit.
func (c Confirm) WithDangerous(d bool) Confirm { c.dangerous = d; return c }

// WithWidth caps the prompt reflow width in cells. Values below 20 clamp to
// 20 so the prompt always has room to breathe.
func (c Confirm) WithWidth(w int) Confirm {
	if w < 20 {
		w = 20
	}
	c.width = w
	return c
}

// FocusedYes reports whether Yes currently has focus.
func (c Confirm) FocusedYes() bool { return c.focusedYes }

// Init implements tea.Model. The component has no autonomous behavior.
func (c Confirm) Init() tea.Cmd { return nil }

// Update handles key input. Tab / Right / l move focus right; Shift+Tab /
// Left / h move focus left. y/Y and n/N focus the matching button and
// commit in a single keystroke. Enter commits the focused button. Esc
// emits CancelMsg.
func (c Confirm) Update(msg tea.Msg) (Confirm, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return c, nil
	}
	switch key.Type {
	case tea.KeyEsc:
		return c, func() tea.Msg { return CancelMsg{} }
	case tea.KeyEnter:
		value := c.focusedYes
		return c, func() tea.Msg { return ConfirmMsg{Value: value} }
	case tea.KeyTab, tea.KeyRight:
		c.focusedYes = !c.focusedYes
		return c, nil
	case tea.KeyShiftTab, tea.KeyLeft:
		c.focusedYes = !c.focusedYes
		return c, nil
	}
	switch key.String() {
	case "y", "Y":
		c.focusedYes = true
		return c, func() tea.Msg { return ConfirmMsg{Value: true} }
	case "n", "N":
		c.focusedYes = false
		return c, func() tea.Msg { return ConfirmMsg{Value: false} }
	case "l":
		c.focusedYes = !c.focusedYes
		return c, nil
	case "h":
		c.focusedYes = !c.focusedYes
		return c, nil
	}
	return c, nil
}

// View renders the prompt, the two buttons, and a right-aligned key hint.
// The prompt is word-wrapped to the configured width; buttons are
// foreground/background-styled per focus state and the dangerous flag.
func (c Confirm) View() string {
	var parts []string

	if c.prompt != "" {
		promptStyle := lipgloss.NewStyle().Foreground(c.theme.Text)
		wrapped := wordwrap.String(c.prompt, c.width)
		parts = append(parts, promptStyle.Render(wrapped))
	}

	parts = append(parts, "")
	parts = append(parts, c.renderButtons())
	parts = append(parts, "")
	parts = append(parts, c.renderHint())

	return strings.Join(parts, "\n")
}

// renderButtons lays out the yes/no button row with four spaces between
// the buttons. Each button is padded by one cell on either side so the
// focus background reads as a solid pill.
func (c Confirm) renderButtons() string {
	yes := c.buttonStyle(true, c.focusedYes).Render(" " + c.yesLabel + " ")
	no := c.buttonStyle(false, !c.focusedYes).Render(" " + c.noLabel + " ")
	return yes + "    " + no
}

// buttonStyle returns the lipgloss style for one button. isYes selects the
// dangerous palette when the Yes button is being rendered with the
// dangerous flag set; focused toggles the high-contrast pill.
func (c Confirm) buttonStyle(isYes, focused bool) lipgloss.Style {
	base := lipgloss.NewStyle()
	if focused {
		if isYes && c.dangerous {
			return base.
				Foreground(c.theme.TextInverse).
				Background(c.theme.Error).
				Bold(true)
		}
		return base.
			Foreground(c.theme.TextInverse).
			Background(c.theme.PrimaryStrong).
			Bold(true)
	}
	if isYes && c.dangerous {
		return base.Foreground(c.theme.Error)
	}
	return base.Foreground(c.theme.TextMuted)
}

// renderHint emits a subtle right-aligned cheatsheet. The width is the
// reflow width so the hint aligns with the right edge of the wrapped
// prompt.
func (c Confirm) renderHint() string {
	hint := "y/n · tab · enter · esc"
	style := lipgloss.NewStyle().
		Foreground(c.theme.TextMuted).
		Width(c.width).
		Align(lipgloss.Right)
	return style.Render(hint)
}
