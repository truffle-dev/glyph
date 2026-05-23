//go:build glyph_demo_snap

package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	chatinput "github.com/truffle-dev/glyph/components/chat-input"
)

// snap drives the chat-cli model through a scripted tour, pausing between
// scenes so asciinema captures each as a distinct frame. The scenes pace a
// realistic conversation: a greeting, an explanation, then the three overlay
// surfaces (palette, save modal, model picker) each opened in turn.
//
// Build with: go build -tags glyph_demo_snap ./examples/chat-cli
//
// This main replaces the interactive entry point when the tag is set.

const (
	cols      = 140
	rows      = 36
	sceneHold = 1600 * time.Millisecond
)

func main() {
	// Deterministic seed keeps the recorded GIF identical run to run.
	rand.Seed(1)

	m := newModel()
	mi, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
	m = mi.(model)

	// Give asciinema a beat to settle before the first scene paint, so the
	// initial frame captures the welcome screen rather than a bare cursor.
	time.Sleep(600 * time.Millisecond)

	type step struct {
		label string
		do    func(model) model
	}

	steps := []step{
		{"welcome", func(m model) model { return m }},

		{"user message + thinking", func(m model) model {
			m = typeText(m, "hello")
			// chatinput emits SubmitMsg via an async Cmd in normal runtime.
			// In snap mode we send it directly so the user message and the
			// thinking spinner both land in this scene's view.
			mi, _ := m.Update(chatinput.SubmitMsg{Value: "hello"})
			return mi.(model)
		}},

		{"assistant reply landed", func(m model) model {
			mi, _ := m.Update(replyMsg{text: respondTo("hello")})
			return mi.(model)
		}},

		{"command palette open", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
			return mi.(model)
		}},

		{"save modal", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = mi.(model)
			mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
			m = mi.(model)
			return typeIntoSave(m, "chat-2026-05-23.md")
		}},

		{"model picker", func(m model) model {
			mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = mi.(model)
			mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
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

// typeText sends each rune of s as a KeyMsg into the chat input.
func typeText(m model, s string) model {
	for _, r := range s {
		mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mi.(model)
	}
	return m
}

// typeIntoSave routes runes to the save dialog instead of chat input.
func typeIntoSave(m model, s string) model {
	for _, r := range s {
		mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mi.(model)
	}
	return m
}

// clear emits an ANSI clear so each scene replaces the previous one in
// the recorded cast. asciinema captures cell state, so a clear + reprint
// is enough; we don't redraw the whole terminal in place.
func clear() {
	fmt.Print("\x1b[2J\x1b[H")
}
