// Package help renders nook's full-keymap overlay.
//
// The host's status bar lists a single-line key reference, but that's
// reference, not discovery — users scanning the status bar miss the
// boundary between "ctrl+f search" and "ctrl+g git." This overlay groups
// every binding by job (Files, Editing, AI, Git, Terminal, LSP) and gives
// each one a single-sentence description. It's bound to `?` and dismissed
// by Esc or `?`.
//
// The view is intentionally fixed-width (a card, not full-screen) so it
// reads like documentation, not a panel. The host centers it.
package help

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Section is a group of related bindings shown under a single header.
type Section struct {
	Name     string
	Bindings []Binding
}

// Binding is a single key with its description. Key strings are rendered
// verbatim ("ctrl+p", "?", "esc"); descriptions are plain English.
type Binding struct {
	Key  string
	Desc string
}

// Default returns the canonical nook keymap. The order is intentional:
// navigation first (what people reach for in the first 30 seconds), AI
// in the middle (where the differentiation lives), then panes and
// miscellany. Update this list whenever a new binding lands in routeKey.
func Default() []Section {
	return []Section{
		{Name: "Files", Bindings: []Binding{
			{"ctrl+p", "Fuzzy file picker"},
			{"ctrl+f", "Project-wide search"},
			{"ctrl+s", "Save the current buffer"},
		}},
		{Name: "Editing", Bindings: []Binding{
			{"↑ ↓ ← →", "Move cursor"},
			{"home / ctrl+a", "Start of line"},
			{"end / ctrl+e", "End of line"},
			{"pgup / pgdn", "Page up / down"},
			{"backspace", "Delete previous character"},
			{"delete", "Delete next character"},
			{"enter", "Insert newline"},
			{"tab", "Insert tab (or accept ghost text)"},
		}},
		{Name: "AI wedges", Bindings: []Binding{
			{"ctrl+k", "Inline edit on current line (Haiku 4.5)"},
			{"ctrl+l", "Multi-file composer (Sonnet 4.6)"},
			{"tab", "Accept ghost-text suggestion"},
			{"esc", "Dismiss ghost text"},
		}},
		{Name: "Panes", Bindings: []Binding{
			{"ctrl+g", "Toggle git pane"},
			{"ctrl+`", "Toggle embedded terminal"},
			{"esc", "Close overlay / blur pane"},
		}},
		{Name: "Global", Bindings: []Binding{
			{"?", "Toggle this help"},
			{"ctrl+q", "Quit nook"},
		}},
	}
}

// View renders the help card. Width is the available column count from the
// host (the card clamps to ~74 columns so the inner ladder lines up nicely
// regardless of terminal width).
func View(t theme.Theme, width int) string {
	inner := 74
	if width < inner+4 {
		inner = width - 4
		if inner < 30 {
			inner = 30
		}
	}

	title := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Render("nook keymap")
	subtitle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Italic(true).
		Render("press ? or esc to dismiss")

	sectionStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Bold(true)
	keyStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(t.Text)

	var body []string
	body = append(body, title)
	body = append(body, subtitle)
	body = append(body, "")

	for i, sec := range Default() {
		if i > 0 {
			body = append(body, "")
		}
		body = append(body, sectionStyle.Render(sec.Name))
		body = append(body, "")
		for _, b := range sec.Bindings {
			body = append(body, fmt.Sprintf("  %-18s  %s",
				keyStyle.Render(b.Key),
				descStyle.Render(b.Desc),
			))
		}
	}

	card := strings.Join(body, "\n")

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(1, 2).
		Width(inner).
		Background(t.Surface)
	return border.Render(card)
}
