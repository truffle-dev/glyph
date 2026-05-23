//go:build glyph_demo_snap

package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// snap drives the file-explorer model through a scripted tour, pausing
// between scenes so asciinema captures each as a distinct frame.
//
// Build with: go build -tags glyph_demo_snap ./examples/file-explorer

const (
	cols      = 140
	rows      = 36
	sceneHold = 1500 * time.Millisecond
)

func main() {
	m := newModel()
	mi, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
	m = mi.(model)

	// Give asciinema a beat to settle before the first paint.
	time.Sleep(600 * time.Millisecond)

	// Each step describes how to drive the cursor from where the last
	// step left it. The model's syncPreview keeps the right pane in step
	// with the tree cursor automatically.
	steps := []struct {
		label string
		do    func(model) model
	}{
		{"open: cmd/main.go", func(m model) model { return m }},
		{"navigate: cmd/root.go", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			return mi.(model)
		}},
		{"expand: internal/store", func(m model) model {
			// step down past root.go, into internal (already expanded),
			// land on store (also expanded), then onto sqlite.go.
			for i := 0; i < 3; i++ {
				mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
				m = mi.(model)
			}
			return m
		}},
		{"preview: internal/store/memory.go", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			return mi.(model)
		}},
		{"preview: scripts/deploy.sh", func(m model) model {
			// down past agent.go, config.go, into scripts (collapsed),
			// expand it, then down onto deploy.sh.
			for i := 0; i < 3; i++ {
				mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
				m = mi.(model)
			}
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
			m = mi.(model)
			mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
			return mi.(model)
		}},
		{"preview: config.json", func(m model) model {
			// down to go.mod, README.md, config.json.
			for i := 0; i < 3; i++ {
				mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
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

func clear() {
	fmt.Print("\x1b[2J\x1b[H")
}
