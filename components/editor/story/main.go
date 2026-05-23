//go:build glyph_story

package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	codeview "github.com/truffle-dev/glyph/components/code-view"
	"github.com/truffle-dev/glyph/components/editor"
	"github.com/truffle-dev/glyph/components/theme"
)

type rootModel struct {
	ed editor.Model
}

func initial() rootModel {
	ed := editor.New(theme.Default).
		WithContent(sample()).
		WithLanguage(codeview.LangGo).
		WithWidth(72).
		WithHeight(16)
	// Park cursor on line 5 so the focus bar lands inside the body.
	for k := 0; k < 4; k++ {
		mi, _ := ed.Update(tea.KeyMsg{Type: tea.KeyDown})
		ed = mi
	}
	return rootModel{ed: ed}
}

func (m rootModel) Init() tea.Cmd { return nil }

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	updated, cmd := m.ed.Update(msg)
	m.ed = updated
	return m, cmd
}

func (m rootModel) View() string {
	title := lipgloss.NewStyle().
		Foreground(theme.Default.Primary).
		Bold(true).
		Render("editor — multi-line text buffer")

	row, col := m.ed.Cursor()
	dirty := ""
	if m.ed.Dirty() {
		dirty = " · modified"
	}
	meta := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).
		Render(fmt.Sprintf("Ln %d, Col %d · %d lines%s", row+1, col+1, m.ed.LineCount(), dirty))

	hint := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).
		Render("arrows move · Enter newline · Backspace delete · Ctrl-Z undo · Ctrl-Y redo · Esc quit")

	return strings.Join([]string{title, "", m.ed.View(), "", meta, "", hint}, "\n")
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

func main() {
	if _, err := tea.NewProgram(initial()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "editor story:", err)
		os.Exit(1)
	}
}
