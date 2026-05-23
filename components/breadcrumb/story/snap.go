//go:build glyph_snap

package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/breadcrumb"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
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

	fmt.Println(strings.Join([]string{
		title, "",
		header("simple path:"), a, "",
		header("crumbs with icons:"), b, "",
		header("collapsed with MaxItems=4:"), c, "",
		header("custom separator:"), d,
	}, "\n"))
}
