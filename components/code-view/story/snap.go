//go:build glyph_snap

package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	codeview "github.com/truffle-dev/glyph/components/code-view"
	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	title := lipgloss.NewStyle().
		Foreground(theme.Default.Primary).
		Bold(true).
		Render("code-view — syntax-tinted source block")

	header := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Render

	goSrc := `package main

import "fmt"

// greet prints a small message.
func greet(name string) {
	fmt.Println("hello,", name)
}`
	pySrc := `def fib(n):
    # generate the first n Fibonacci numbers
    a, b = 0, 1
    while a < n:
        yield a
        a, b = b, a + b`
	jsonSrc := `{
    "name": "glyph",
    "version": "0.3.0",
    "private": false
}`

	goBlock := codeview.Render(codeview.Block{
		Source: goSrc, Language: codeview.LangGo, ShowGutter: true,
		Marks: map[int]codeview.LineMark{6: codeview.MarkHighlight},
	})
	pyBlock := codeview.Render(codeview.Block{
		Source: pySrc, Language: codeview.LangPy, ShowGutter: true,
		Marks: map[int]codeview.LineMark{3: codeview.MarkAdded, 5: codeview.MarkWarning},
	})
	jsonBlock := codeview.Render(codeview.Block{
		Source: jsonSrc, Language: codeview.LangJSON, ShowGutter: false,
	})

	fmt.Println(strings.Join([]string{
		title, "",
		header("Go (current line marked):"),
		goBlock, "",
		header("Python (added + warning marks):"),
		pyBlock, "",
		header("JSON (no gutter):"),
		jsonBlock,
	}, "\n"))
}
