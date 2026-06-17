// Package themepicker renders nook's live theme switcher.
//
// Theme hot-reload already ships: edit `theme` in config.toml, press alt+,
// and the whole UI repaints. But that path makes you leave the editor, know
// the exact theme name, and reload by hand. This overlay is the discoverable
// counterpart — bound to alt+T, it lists every built-in theme and previews
// each one live as you move the cursor, the way Zed's `theme selector`
// (cmd-k cmd-t) does. Enter keeps the highlighted theme for the session; esc
// restores whatever was active when the picker opened.
//
// It is session-live, not persisted: config is read-only on disk, so the
// choice lasts until nook exits or config.toml is reloaded. The card states
// this plainly so a user isn't surprised when a restart reverts the look.
//
// The struct is small on purpose — a name list and a cursor. Movement clamps
// at both ends; the host drives the live preview by calling Selected() on
// every cursor move and broadcasting that theme to the panes.
package themepicker

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Picker is the theme-list overlay state: the ordered theme names and the
// highlighted row. It carries no theme of its own; the host owns the live
// preview and passes the active theme into View.
type Picker struct {
	names  []string
	cursor int
}

// New builds a picker over names, with the cursor parked on current if it is
// present in the list (so the picker opens on the theme you're already using).
// names is expected to be theme.Names() — already sorted, so the order is
// deterministic across runs and tests.
func New(names []string, current string) Picker {
	p := Picker{names: names}
	for i, n := range names {
		if n == current {
			p.cursor = i
			break
		}
	}
	return p
}

// Up moves the highlight one row toward the top, clamping at the first row.
func (p Picker) Up() Picker {
	if p.cursor > 0 {
		p.cursor--
	}
	return p
}

// Down moves the highlight one row toward the bottom, clamping at the last.
func (p Picker) Down() Picker {
	if p.cursor < len(p.names)-1 {
		p.cursor++
	}
	return p
}

// Selected returns the highlighted theme name, or "" if the list is empty.
func (p Picker) Selected() string {
	if p.cursor < 0 || p.cursor >= len(p.names) {
		return ""
	}
	return p.names[p.cursor]
}

// Cursor returns the highlighted row index (for tests and host bookkeeping).
func (p Picker) Cursor() int {
	return p.cursor
}

// View renders the picker card. t is the live preview theme (the one the host
// is currently showing), so the card itself recolors as you scan the list.
// width is the host's column count; the card clamps to ~48 columns so the
// list reads as a menu rather than a panel.
func View(t theme.Theme, width int, p Picker) string {
	inner := 48
	if width < inner+4 {
		inner = width - 4
		if inner < 24 {
			inner = 24
		}
	}

	title := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Render("theme")
	subtitle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Italic(true).
		Render("↑↓ preview · enter keep · esc cancel")

	cursorStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	selStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	rowStyle := lipgloss.NewStyle().
		Foreground(t.Text)
	footStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Italic(true)

	var body []string
	body = append(body, title)
	body = append(body, subtitle)
	body = append(body, "")

	for i, name := range p.names {
		marker := "  "
		render := rowStyle.Render
		if i == p.cursor {
			marker = cursorStyle.Render("> ")
			render = selStyle.Render
		}
		body = append(body, fmt.Sprintf("%s%s", marker, render(name)))
	}

	body = append(body, "")
	body = append(body, footStyle.Render("session only · disk config unchanged"))

	card := strings.Join(body, "\n")

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(1, 2).
		Width(inner).
		Background(t.Surface)
	return border.Render(card)
}
