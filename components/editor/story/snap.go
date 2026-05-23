//go:build glyph_snap

package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	codeview "github.com/truffle-dev/glyph/components/code-view"
	"github.com/truffle-dev/glyph/components/editor"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	title := lipgloss.NewStyle().
		Foreground(theme.Default.Primary).
		Bold(true).
		Render("editor — multi-line text buffer")

	ed := editor.New(theme.Default).
		WithContent(sample()).
		WithLanguage(codeview.LangGo).
		WithWidth(72).
		WithHeight(16)
	// Park cursor on line 12 (1-based) so the focus bar lands on the
	// fmt.Printf line. Also positions the cursor mid-expression so the
	// cursor cell visibly inverts a letter.
	for k := 0; k < 11; k++ {
		mi, _ := ed.Update(tea.KeyMsg{Type: tea.KeyDown})
		ed = mi
	}
	for k := 0; k < 10; k++ {
		mi, _ := ed.Update(tea.KeyMsg{Type: tea.KeyRight})
		ed = mi
	}

	row, col := ed.Cursor()
	meta := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).
		Render(fmt.Sprintf("Ln %d, Col %d · %d lines", row+1, col+1, ed.LineCount()))

	hint := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).
		Render("arrows move · Enter newline · Backspace delete · Ctrl-Z undo · Ctrl-Y redo")

	fmt.Println(strings.Join([]string{
		title, "",
		ed.View(), "",
		meta, "",
		hint,
	}, "\n"))
}

func sample() string {
	return `package main

import (
	"fmt"
	"os"
)

// greet writes a personal welcome to stdout, falling back to the
// generic banner when no name is supplied.
func greet(name string) {
	if name == "" {
		fmt.Println("hello, world")
		return
	}
	fmt.Printf("hello, %s\n", name)
}

func main() {
	if len(os.Args) > 1 {
		greet(os.Args[1])
		return
	}
	greet("")
}`
}
