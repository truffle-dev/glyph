//go:build glyph_story

package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	findbar "github.com/truffle-dev/glyph/components/find-bar"
	"github.com/truffle-dev/glyph/components/theme"
)

var sampleLines = []string{
	"package main",
	"",
	"import (",
	"\t\"fmt\"",
	"\t\"os\"",
	")",
	"",
	"// greet prints a hello message to stdout.",
	"func greet(name string) {",
	"\tif name == \"\" {",
	"\t\tfmt.Println(\"hello, world\")",
	"\t\treturn",
	"\t}",
	"\tfmt.Printf(\"hello, %s\\n\", name)",
	"}",
	"",
	"func main() {",
	"\tif len(os.Args) > 1 {",
	"\t\tgreet(os.Args[1])",
	"\t\treturn",
	"\t}",
	"\tgreet(\"\")",
	"}",
}

type rootModel struct {
	bar findbar.Bar
	log []string
}

func initial() rootModel {
	bar := findbar.New(theme.Default).WithWidth(56)
	return rootModel{bar: bar}
}

func (m rootModel) Init() tea.Cmd { return nil }

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case findbar.CloseMsg:
		return m, tea.Quit
	case findbar.QueryMsg:
		matches := findbar.FindMatches(sampleLines, msg.Value, m.bar.CaseSensitive())
		m.bar = m.bar.WithMatches(matches, 0)
		m.log = appendLog(m.log, fmt.Sprintf("query → %q  (%d matches)", msg.Value, len(matches)))
		return m, nil
	case findbar.NextMsg:
		if m.bar.MatchCount() > 0 {
			next := (m.bar.Current() + 1) % m.bar.MatchCount()
			m.bar = m.bar.WithMatches(matchesFor(m.bar), next)
			m.log = appendLog(m.log, fmt.Sprintf("next → match %d/%d", next+1, m.bar.MatchCount()))
		}
		return m, nil
	case findbar.PrevMsg:
		if m.bar.MatchCount() > 0 {
			prev := m.bar.Current() - 1
			if prev < 0 {
				prev = m.bar.MatchCount() - 1
			}
			m.bar = m.bar.WithMatches(matchesFor(m.bar), prev)
			m.log = appendLog(m.log, fmt.Sprintf("prev → match %d/%d", prev+1, m.bar.MatchCount()))
		}
		return m, nil
	}

	updated, cmd := m.bar.Update(msg)
	m.bar = updated
	return m, cmd
}

func matchesFor(b findbar.Bar) []findbar.Match {
	return findbar.FindMatches(sampleLines, b.Query(), b.CaseSensitive())
}

func appendLog(buf []string, line string) []string {
	buf = append(buf, line)
	if len(buf) > 6 {
		buf = buf[len(buf)-6:]
	}
	return buf
}

func (m rootModel) View() string {
	title := lipgloss.NewStyle().
		Foreground(theme.Default.Primary).
		Bold(true).
		Render("find-bar — in-buffer search overlay")

	bufStyle := lipgloss.NewStyle().
		Foreground(theme.Default.TextMuted).
		PaddingLeft(1)
	buf := bufStyle.Render(strings.Join(sampleLines, "\n"))

	logStyle := lipgloss.NewStyle().Foreground(theme.Default.TextMuted)
	log := logStyle.Render(strings.Join(m.log, "\n"))

	hint := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Render(
		"type to search · Enter next · Alt+Enter prev · Ctrl-U clear · Esc quit")

	return strings.Join([]string{title, "", buf, "", m.bar.View(), "", log, "", hint}, "\n")
}

func main() {
	if _, err := tea.NewProgram(initial()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "find-bar story:", err)
		os.Exit(1)
	}
}
