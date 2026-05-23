//go:build glyph_demo_snap

// snap renders one frame of the code-editor demo: file-tree on the
// left with one expanded directory, the editor open on cmd/main.go
// with syntax tinting, the find-bar popped open with a populated
// query, and the status-bar / key-hints pinned along the bottom.
package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	findbar "github.com/truffle-dev/glyph/components/find-bar"
)

func main() {
	m := newModel()
	m.width = 120
	m.height = 36
	m = m.resizeEditors()

	// Open Ctrl-F and pre-fill the query so the snap shows live matches.
	m.barOpen = true
	m.bar = m.bar.Focus()
	for _, r := range []rune("Agent") {
		updated, _ := m.bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m.bar = updated
	}
	if b := m.currentBuffer(); b != nil {
		lines := strings.Split(b.ed.Value(), "\n")
		matches := findbar.FindMatches(lines, m.bar.Query(), m.bar.CaseSensitive())
		m.bar = m.bar.WithMatches(matches, 0)
	}
	m.focus = focusFindBar

	fmt.Println(m.View())
}
