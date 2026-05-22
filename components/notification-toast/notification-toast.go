// Package notificationtoast renders a stack of dismissible, level-aware
// notifications. Each toast carries a level, an optional title, and a
// message. Toasts auto-expire after a per-toast TTL or stay until the
// consumer explicitly dismisses them.
//
// The tray is render-only: pump time forward by calling Tick(now) on a
// timer in your Update function and the tray drops expired toasts. Push
// adds a new toast; Dismiss removes one by ID.
package notificationtoast

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"

	"github.com/truffle-dev/glyph/components/theme"
)

// Level controls the toast's color and icon.
type Level int

const (
	LevelInfo Level = iota
	LevelSuccess
	LevelWarning
	LevelError
)

// Toast is a single notification.
type Toast struct {
	ID        string
	Level     Level
	Title     string
	Message   string
	ExpiresAt time.Time // zero value means no auto-dismiss
}

// Tray holds the active toasts and renders them as a vertical stack.
type Tray struct {
	theme    theme.Theme
	toasts   []Toast
	width    int
	maxItems int
}

// New constructs an empty Tray with default width 50 and max 4 visible.
func New(t theme.Theme) Tray {
	return Tray{theme: t, width: 50, maxItems: 4}
}

// WithWidth sets the rendered width of each toast card. Minimum 20.
func (tr Tray) WithWidth(w int) Tray {
	if w < 20 {
		w = 20
	}
	tr.width = w
	return tr
}

// WithMaxItems caps the number of toasts shown at once. Older toasts
// drop first when capacity is exceeded.
func (tr Tray) WithMaxItems(n int) Tray {
	if n < 1 {
		n = 1
	}
	tr.maxItems = n
	if len(tr.toasts) > n {
		tr.toasts = tr.toasts[len(tr.toasts)-n:]
	}
	return tr
}

// Push adds a toast. If maxItems is exceeded, the oldest drops.
func (tr Tray) Push(t Toast) Tray {
	tr.toasts = append(tr.toasts, t)
	if len(tr.toasts) > tr.maxItems {
		tr.toasts = tr.toasts[len(tr.toasts)-tr.maxItems:]
	}
	return tr
}

// Dismiss removes the toast with the given ID. Missing IDs are ignored.
func (tr Tray) Dismiss(id string) Tray {
	out := tr.toasts[:0]
	for _, t := range tr.toasts {
		if t.ID != id {
			out = append(out, t)
		}
	}
	tr.toasts = append([]Toast(nil), out...)
	return tr
}

// DismissAll empties the tray.
func (tr Tray) DismissAll() Tray { tr.toasts = nil; return tr }

// Tick removes toasts whose ExpiresAt is non-zero and <= now.
func (tr Tray) Tick(now time.Time) Tray {
	out := tr.toasts[:0]
	for _, t := range tr.toasts {
		if !t.ExpiresAt.IsZero() && !t.ExpiresAt.After(now) {
			continue
		}
		out = append(out, t)
	}
	tr.toasts = append([]Toast(nil), out...)
	return tr
}

// Toasts returns a copy of the current toasts.
func (tr Tray) Toasts() []Toast {
	out := make([]Toast, len(tr.toasts))
	copy(out, tr.toasts)
	return out
}

// Init implements tea.Model. The tray has no autonomous behavior.
func (tr Tray) Init() tea.Cmd { return nil }

// Update is a no-op. The tray is driven by external Push/Dismiss/Tick.
// It is implemented so the type satisfies tea.Model for composition.
func (tr Tray) Update(_ tea.Msg) (Tray, tea.Cmd) { return tr, nil }

// View renders the tray as a vertical stack of cards separated by a
// blank line. Empty tray renders to empty string.
func (tr Tray) View() string {
	if len(tr.toasts) == 0 {
		return ""
	}
	cards := make([]string, len(tr.toasts))
	for i, t := range tr.toasts {
		cards[i] = tr.renderCard(t)
	}
	return strings.Join(cards, "\n\n")
}

func (tr Tray) renderCard(t Toast) string {
	icon, accent := tr.iconAndAccent(t.Level)
	titleStyle := lipgloss.NewStyle().Foreground(accent).Bold(true)
	msgStyle := lipgloss.NewStyle().Foreground(tr.theme.Text)
	iconStyle := lipgloss.NewStyle().Foreground(accent).Bold(true)

	innerWidth := tr.width - 4 // 2 padding + 2 border
	if innerWidth < 10 {
		innerWidth = 10
	}

	bodyWidth := innerWidth - 4 // icon + space
	if bodyWidth < 5 {
		bodyWidth = 5
	}

	header := iconStyle.Render(icon) + " "
	if t.Title != "" {
		header += titleStyle.Render(t.Title)
	}
	parts := []string{header}
	if t.Message != "" {
		wrapped := wordwrap.String(t.Message, bodyWidth)
		for _, ln := range strings.Split(wrapped, "\n") {
			parts = append(parts, "  "+msgStyle.Render(ln))
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Background(tr.theme.Surface).
		Padding(0, 1).
		Width(tr.width).
		Render(strings.Join(parts, "\n"))
}

func (tr Tray) iconAndAccent(l Level) (string, lipgloss.TerminalColor) {
	switch l {
	case LevelSuccess:
		return "✓", tr.theme.Success
	case LevelWarning:
		return "!", tr.theme.Warning
	case LevelError:
		return "✗", tr.theme.Error
	default:
		return "i", tr.theme.Info
	}
}
