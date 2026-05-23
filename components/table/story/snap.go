//go:build glyph_snap

package main

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/table"
)

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

	help := "↑/↓ or j/k move • ←/→ active col • s sort • PgUp/PgDn page • g/G home/end • enter select • q quit"
	fmt.Println(tbl.View() + "\n\n(no event yet — try ←/→ then s, or enter to select)\n" + help)
}
