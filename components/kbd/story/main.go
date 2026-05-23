//go:build glyph_story

package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/kbd"
	"github.com/truffle-dev/glyph/components/theme"
)

type model struct {
	quit bool
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c", "esc":
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	title := lipgloss.NewStyle().
		Foreground(theme.Default.Primary).
		Bold(true).
		Render("kbd — keycap glyph atom")

	// Every special glyph in a row.
	special := []string{
		"ctrl", "shift", "alt", "cmd",
		"enter", "tab", "esc", "space",
		"backspace", "delete",
		"up", "down", "left", "right",
		"pageup", "pagedown", "home", "end",
	}
	row := make([]string, 0, len(special))
	for _, k := range special {
		row = append(row, kbd.Render(k))
	}
	caps := strings.Join(row, " ")

	chords := lipgloss.JoinVertical(lipgloss.Left,
		kbd.Chord("Ctrl", "K"),
		kbd.Chord("Cmd", "Shift", "P"),
		kbd.Chord("Alt", "Enter"),
	)

	sequence := kbd.Sequence(kbd.Chord("g"), kbd.Chord("g"))
	thenSeq := kbd.Sequence(kbd.Chord("Ctrl", "K"), kbd.Chord("P"))

	hint := lipgloss.NewStyle().
		Foreground(theme.Default.TextMuted).
		Render("Press q to quit")

	header := lipgloss.NewStyle().
		Foreground(theme.Default.TextMuted).
		Render

	return strings.Join([]string{
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
		"",
		hint,
	}, "\n")
}

func main() {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "kbd story:", err)
		os.Exit(1)
	}
}
