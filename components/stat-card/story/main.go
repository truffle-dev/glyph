//go:build glyph_story

package main

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	statcard "github.com/truffle-dev/glyph/components/stat-card"
)

func main() {
	merged := statcard.New().
		WithLabel("Merged PRs").
		WithValue("88").
		WithDelta("+12").
		WithTrend(statcard.TrendUp).
		WithSublabel("this month")

	inFlight := statcard.New().
		WithLabel("In flight").
		WithValue("39").
		WithDelta("0").
		WithTrend(statcard.TrendNeutral).
		WithSublabel("across 24 repos")

	followers := statcard.New().
		WithLabel("Followers").
		WithValue("12").
		WithDelta("+3").
		WithTrend(statcard.TrendUp).
		WithSublabel("this week")

	revenue := statcard.New().
		WithLabel("Revenue").
		WithValue("$0").
		WithDelta("-100%").
		WithTrend(statcard.TrendDown).
		WithSublabel("since signup").
		WithEmphasis(true)

	row := lipgloss.JoinHorizontal(
		lipgloss.Top,
		merged.View(),
		"  ",
		inFlight.View(),
		"  ",
		followers.View(),
		"  ",
		revenue.View(),
	)

	fixed := statcard.New().
		WithLabel("Latency p99").
		WithValue("142ms").
		WithDelta("-8ms").
		WithTrend(statcard.TrendUp). // down-is-good for latency
		WithSublabel("last 5 min").
		WithWidth(40)

	fmt.Println(row)
	fmt.Println()
	fmt.Println(fixed.View())
	fmt.Println()
	fmt.Println("q to quit")
}
