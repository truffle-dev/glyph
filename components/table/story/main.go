//go:build glyph_story

package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/table"
)

// engagement is a fake "scout queue" snapshot: repos the operator might
// hand off to an outreach agent, with light counts and a status flag.
type engagement struct {
	Repo, Owner, Issues, PRs, LastTouch, Status string
}

func sample() []engagement {
	return []engagement{
		{"glyph", "truffle-dev", "3", "1", "2026-05-23", "active"},
		{"vigil", "baudsmithstudios", "12", "4", "2026-05-22", "active"},
		{"voltagent", "VoltAgent", "27", "8", "2026-05-22", "merged"},
		{"clap", "clap-rs", "104", "16", "2026-05-19", "blocked"},
		{"jj", "jj-vcs", "412", "23", "2026-05-15", "merged"},
		{"auto-subs", "tmoroney", "8", "2", "2026-05-21", "merged"},
		{"opencli", "opencli-tools", "9", "2", "2026-05-19", "merged"},
		{"helix", "helix-editor", "319", "11", "2026-05-10", "off-limits"},
		{"sprocket", "stjude-rust-labs", "47", "5", "2026-05-12", "off-limits"},
		{"ratatui", "ratatui-org", "63", "9", "2026-05-22", "ready"},
		{"DuckDB", "duckdb", "892", "47", "2026-05-22", "active"},
		{"openclaw", "anthropics", "147", "12", "2026-05-22", "active"},
		{"NemoClaw", "nvidia", "284", "19", "2026-05-22", "active"},
		{"astro", "withastro", "0", "0", "2026-05-22", "off-limits"},
		{"turso", "tursodatabase", "0", "0", "2026-05-22", "off-limits"},
	}
}

type model struct {
	tbl    table.Model
	status string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case table.SelectMsg:
		m.status = fmt.Sprintf("selected: %s/%s (row %d)", msg.Row.Cells[1], msg.Row.Cells[0], msg.Index)
		return m, nil
	case table.SortMsg:
		dir := "asc"
		if msg.Descending {
			dir = "desc"
		}
		m.status = fmt.Sprintf("sorted by %q %s", msg.ColumnKey, dir)
		return m, nil
	case table.CursorMsg:
		m.status = fmt.Sprintf("cursor: row %d", msg.Index)
		return m, nil
	}
	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)
	return m, cmd
}

func (m model) View() string {
	help := "↑/↓ or j/k move • ←/→ active col • s sort • PgUp/PgDn page • g/G home/end • enter select • q quit"
	out := m.tbl.View() + "\n\n" + m.status + "\n" + help
	return out
}

func main() {
	rows := make([]table.Row, 0, len(sample()))
	for _, e := range sample() {
		rows = append(rows, table.Row{
			Cells: []string{e.Repo, e.Owner, e.Issues, e.PRs, e.LastTouch, e.Status},
			Value: e,
		})
	}

	tbl := table.New().
		WithColumns(
			table.Column{Key: "repo", Title: "Repo", Sortable: true},
			table.Column{Key: "owner", Title: "Owner", Sortable: true},
			table.Column{Key: "issues", Title: "Issues", Align: table.AlignRight, Sortable: true},
			table.Column{Key: "prs", Title: "PRs", Align: table.AlignRight, Sortable: true},
			table.Column{Key: "touched", Title: "Last Touch", Sortable: true},
			table.Column{Key: "status", Title: "Status", Sortable: true},
		).
		WithRows(rows...).
		WithSize(100, 14).
		WithRowSelection(true)

	m := model{tbl: tbl, status: "(no event yet — try ←/→ then s, or enter to select)"}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "story:", err)
		os.Exit(1)
	}
}
