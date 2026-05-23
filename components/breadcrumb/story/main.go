//go:build glyph_story

package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/breadcrumb"
	"github.com/truffle-dev/glyph/components/theme"
)

type model struct{}

func (m model) Init() tea.Cmd { return nil }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && (k.String() == "q" || k.String() == "ctrl+c") {
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string {
	title := lipgloss.NewStyle().
		Foreground(theme.Default.Primary).
		Bold(true).
		Render("breadcrumb — path-trail navigator")

	header := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Render

	a := breadcrumb.RenderPath("project/src/cmd/main.go", breadcrumb.Options{})
	b := breadcrumb.Render([]breadcrumb.Crumb{
		{Icon: "📁", Label: "project"},
		{Icon: "📁", Label: "internal"},
		{Icon: "📁", Label: "store"},
		{Icon: "📄", Label: "sqlite.go"},
	}, breadcrumb.Options{})
	c := breadcrumb.RenderPath("project/a/b/c/d/e/f/leaf.go", breadcrumb.Options{MaxItems: 4})
	d := breadcrumb.Render([]breadcrumb.Crumb{{Label: "settings"}, {Label: "billing"}}, breadcrumb.Options{Separator: " / "})

	hint := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Render("Press q to quit")

	return strings.Join([]string{
		title, "",
		header("simple path:"), a, "",
		header("crumbs with icons:"), b, "",
		header("collapsed with MaxItems=4:"), c, "",
		header("custom separator:"), d, "",
		hint,
	}, "\n")
}

func main() {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "breadcrumb story:", err)
		os.Exit(1)
	}
}
