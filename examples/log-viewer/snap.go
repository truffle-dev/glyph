//go:build glyph_demo_snap

package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// snap drives the log-viewer model through a scripted tour, pausing
// between scenes so asciinema captures each as a distinct frame.
//
// The tour seeds a realistic stream by firing tickMsg repeatedly, then
// walks through the level filter, source picker, search prompt, and
// the pause toast.
//
// Build with: go build -tags glyph_demo_snap ./examples/log-viewer

const (
	cols      = 140
	rows      = 36
	sceneHold = 1600 * time.Millisecond
	seedTicks = 40
)

func main() {
	// Deterministic seed keeps the recorded GIF identical run to run.
	rand.Seed(42)

	m := newModel()
	mi, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
	m = mi.(model)

	// Seed the buffer so the first scene isn't empty.
	for i := 0; i < seedTicks; i++ {
		mi, _ = m.Update(tickMsg(time.Now()))
		m = mi.(model)
	}

	// Give asciinema a beat to settle before the first scene paint.
	time.Sleep(600 * time.Millisecond)

	type step struct {
		label string
		do    func(model) model
	}

	steps := []step{
		{"live stream — ALL levels", func(m model) model { return m }},

		{"filter WARN+", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
			m = mi.(model)
			mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
			return mi.(model)
		}},

		{"source picker ⌃F", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
			return mi.(model)
		}},

		{"search prompt /", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = mi.(model)
			mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
			m = mi.(model)
			for _, r := range "slow" {
				mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				m = mi.(model)
			}
			return m
		}},

		{"paused stream", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = mi.(model)
			mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
			return mi.(model)
		}},

		{"cleared buffer ⌃L", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
			return mi.(model)
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

func clear() {
	fmt.Print("\x1b[2J\x1b[H")
}
