//go:build glyph_snap

package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/kbd"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	title := lipgloss.NewStyle().
		Foreground(theme.Default.Primary).
		Bold(true).
		Render("kbd — keycap glyph atom")

	// Two rows so 18 keycaps fit in a 32-row terminal. Each cap is a
	// 3-line rounded-border box; horizontal join keeps them on one line.
	rowA := []string{"ctrl", "shift", "alt", "cmd", "enter", "tab", "esc", "space", "backspace"}
	rowB := []string{"delete", "up", "down", "left", "right", "pageup", "pagedown", "home", "end"}
	render := func(keys []string) string {
		caps := make([]string, 0, len(keys))
		for _, k := range keys {
			caps = append(caps, kbd.Render(k))
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, caps...)
	}
	caps := lipgloss.JoinVertical(lipgloss.Left, render(rowA), render(rowB))

	// kbd.Chord / kbd.Sequence join multi-line caps with a plain string
	// separator, which works for inline labels but stacks vertically when
	// caps are 3-line rounded-border boxes. Compose horizontally here so
	// the snap fits inside the snapshot terminal.
	sep := lipgloss.NewStyle().
		Foreground(theme.Default.TextMuted).
		Padding(1, 1).
		Render
	hjoin := func(parts ...string) string {
		return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
	}
	chord := func(keys ...string) string {
		parts := make([]string, 0, len(keys)*2-1)
		for i, k := range keys {
			if i > 0 {
				parts = append(parts, sep("+"))
			}
			parts = append(parts, kbd.Render(k))
		}
		return hjoin(parts...)
	}

	chords := lipgloss.JoinVertical(lipgloss.Left,
		chord("Ctrl", "K"),
		chord("Cmd", "Shift", "P"),
	)

	sequence := hjoin(chord("g"), sep(","), chord("g"))
	thenSeq := hjoin(chord("Ctrl", "K"), sep(","), chord("P"))

	header := lipgloss.NewStyle().
		Foreground(theme.Default.TextMuted).
		Render

	fmt.Println(strings.Join([]string{
		title,
		"",
		header("every special glyph:"),
		caps,
		"",
		header("chords:"),
		chords,
		"",
		header("sequences:"),
		sequence,
		thenSeq,
	}, "\n"))
}
