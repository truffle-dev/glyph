// Package chatbubble renders a role-aware speech bubble with width-aware wrap.
package chatbubble

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"

	"github.com/truffle-dev/glyph/components/theme"
)

// Role drives the bubble's color and alignment.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Bubble renders one chat message.
type Bubble struct {
	theme theme.Theme
	role  Role
	text  string
	width int
	label string
}

// New constructs a bubble using the given theme.
func New(t theme.Theme) Bubble {
	return Bubble{
		theme: t,
		role:  RoleAssistant,
		width: 60,
	}
}

// WithRole sets the role used for styling.
func (b Bubble) WithRole(r Role) Bubble { b.role = r; return b }

// WithText sets the body text.
func (b Bubble) WithText(s string) Bubble { b.text = s; return b }

// WithWidth caps the bubble's content width in cells.
func (b Bubble) WithWidth(w int) Bubble {
	if w < 4 {
		w = 4
	}
	b.width = w
	return b
}

// WithLabel sets the role label printed above the bubble.
// If empty, no label is rendered.
func (b Bubble) WithLabel(s string) Bubble { b.label = s; return b }

// View renders the bubble to a string. Safe to call repeatedly.
func (b Bubble) View() string {
	style := b.style()
	wrapped := wordwrap.String(b.text, b.width-style.GetHorizontalPadding())
	body := style.Render(wrapped)

	if b.label != "" {
		labelStyle := lipgloss.NewStyle().
			Foreground(b.theme.TextMuted).
			MarginLeft(1)
		return strings.Join([]string{
			labelStyle.Render(b.label),
			body,
		}, "\n")
	}
	return body
}

// style returns the lipgloss style for the current role.
func (b Bubble) style() lipgloss.Style {
	base := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(b.width)

	switch b.role {
	case RoleUser:
		return base.
			BorderForeground(b.theme.PrimaryStrong).
			Background(b.theme.Primary).
			Foreground(b.theme.TextInverse)
	case RoleSystem:
		return base.
			BorderForeground(b.theme.Border).
			Foreground(b.theme.TextMuted)
	case RoleTool:
		return base.
			BorderForeground(b.theme.Accent).
			Foreground(b.theme.Text)
	default: // assistant
		return base.
			BorderForeground(b.theme.Border).
			Foreground(b.theme.Text)
	}
}
