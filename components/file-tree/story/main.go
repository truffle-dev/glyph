//go:build glyph_story

package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	filetree "github.com/truffle-dev/glyph/components/file-tree"
	"github.com/truffle-dev/glyph/components/theme"
)

type rootModel struct {
	tree filetree.Model
	last string
}

func initial() rootModel {
	return rootModel{
		tree: filetree.New(sample()).WithTitle("project/"),
	}
}

func (m rootModel) Init() tea.Cmd { return nil }

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case filetree.SelectMsg:
		verb := "opened"
		if msg.IsDir {
			if !m.tree.IsSelected(msg.Path) {
				verb = "closed"
			} else {
				verb = "expanded"
			}
		}
		m.last = fmt.Sprintf("%s: %s", verb, msg.Path)
	}
	updated, cmd := m.tree.Update(msg)
	m.tree = updated
	return m, cmd
}

func (m rootModel) View() string {
	title := lipgloss.NewStyle().
		Foreground(theme.Default.Primary).
		Bold(true).
		Render("file-tree — directory navigator")

	tree := m.tree.View()
	footer := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Render(m.last)
	hint := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).
		Render("↑↓ move · → expand · ← collapse · Enter toggle · q quit")

	return strings.Join([]string{title, "", tree, "", footer, "", hint}, "\n")
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

func main() {
	if _, err := tea.NewProgram(initial()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "file-tree story:", err)
		os.Exit(1)
	}
}
