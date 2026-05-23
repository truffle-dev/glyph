//go:build glyph_demo_snap

package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// snap drives the dashboard model through a scripted tour, pausing
// between scenes so asciinema captures each as a distinct frame.
//
// Build with: go build -tags glyph_demo_snap ./examples/dashboard
// This main replaces the interactive entry point when the tag is set.

const (
	cols      = 140
	rows      = 36
	sceneHold = 1500 * time.Millisecond
)

func main() {
	m := newModel()
	mi, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
	m = mi.(model)

	// Give asciinema a beat to settle before the first scene paint.
	time.Sleep(600 * time.Millisecond)

	steps := []struct {
		label string
		do    func(model) model
	}{
		{"engagements queue", func(m model) model { return m }},
		{"sort by PRs ↓", func(m model) model {
			// move right twice to the PRs column, press s to sort
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
			mi, _ = mi.(model).Update(tea.KeyMsg{Type: tea.KeyRight})
			mi, _ = mi.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
			return mi.(model)
		}},
		{"row open → toast", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			mi, _ = mi.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
			return mi.(model)
		}},
		{"throughput tab", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
			return mi.(model)
		}},
		{"revenue tab", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
			return mi.(model)
		}},
		{"filter modal /banned", func(m model) model {
			// back to engagements
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
			m = mi.(model)
			mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
			m = mi.(model)
			for _, r := range "banned" {
				mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				m = mi.(model)
			}
			return m
		}},
	}

	for i, st := range steps {
		m = st.do(m)
		clear()
		header := fmt.Sprintf("scene %d/%d · %s", i+1, len(steps), st.label)
		fmt.Println(strings.Repeat(" ", (cols-len(header))/2) + header)
		fmt.Println()
		fmt.Println(m.View())
		time.Sleep(sceneHold)
	}
}

// clear emits an ANSI clear so each scene replaces the previous one in
// the recorded cast. asciinema captures cell state, so a clear + reprint
// is enough; we don't redraw the whole terminal in place.
func clear() {
	fmt.Print("\x1b[2J\x1b[H")
}
