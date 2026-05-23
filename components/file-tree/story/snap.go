//go:build glyph_snap

package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	filetree "github.com/truffle-dev/glyph/components/file-tree"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	title := lipgloss.NewStyle().
		Foreground(theme.Default.Primary).
		Bold(true).
		Render("file-tree — directory navigator")

	t := filetree.New(sample()).WithTitle("project/")
	// Expand cmd + internal/store so the preview shows depth.
	t.Expand("cmd")
	t.Expand("internal")
	t.Expand("internal/store")
	// Move cursor to a leaf so the focus row is interesting.
	mi, _ := t.Update(tea.KeyMsg{Type: tea.KeyDown})
	mi, _ = mi.Update(tea.KeyMsg{Type: tea.KeyDown})
	mi, _ = mi.Update(tea.KeyMsg{Type: tea.KeyDown})
	mi, _ = mi.Update(tea.KeyMsg{Type: tea.KeyDown})
	t = mi

	hint := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).
		Render("↑↓ move · → expand · ← collapse · Enter toggle")

	fmt.Println(strings.Join([]string{
		title, "",
		t.View(), "",
		hint,
	}, "\n"))
}

func sample() filetree.Node {
	return filetree.Node{
		Name: "project",
		Children: []filetree.Node{
			{Name: "cmd", Children: []filetree.Node{
				{Name: "main.go", Meta: "1.2 kB"},
				{Name: "root.go", Meta: "843 B"},
			}},
			{Name: "internal", Children: []filetree.Node{
				{Name: "store", Children: []filetree.Node{
					{Name: "sqlite.go", Meta: "4.1 kB"},
					{Name: "memory.go", Meta: "2.0 kB"},
				}},
				{Name: "agent.go", Meta: "6.3 kB"},
			}},
			{Name: "go.mod", Meta: "modified"},
			{Name: "README.md", Meta: "9.2 kB"},
		},
	}
}
