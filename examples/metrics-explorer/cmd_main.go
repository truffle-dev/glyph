//go:build !glyph_demo_snap

package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "metrics-explorer:", err)
		os.Exit(1)
	}
}
