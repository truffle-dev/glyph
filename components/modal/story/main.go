//go:build glyph_story

package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/modal"
	"github.com/truffle-dev/glyph/components/theme"
)

// dimDots renders a w×h field of dim middle-dots so the overlay reads
// against a recognizable background.
func dimDots(w, h int) string {
	row := strings.Repeat("· ", (w+1)/2)
	if lipgloss.Width(row) > w {
		row = lipgloss.NewStyle().MaxWidth(w).Render(row)
	}
	style := lipgloss.NewStyle().Foreground(theme.Default.TextMuted)
	rows := make([]string, h)
	for i := range rows {
		rows[i] = style.Render(row)
	}
	return strings.Join(rows, "\n")
}

func main() {
	const bgW, bgH = 80, 24

	// Scene 1: small modal, title only.
	small := modal.New(theme.Default).
		WithTitle("Confirm action?").
		WithSize(34, 6).
		WithBody("This cannot be undone.")

	// Scene 2: medium modal with title + multi-line body + footer.
	body := strings.Join([]string{
		"You have unsaved edits in:",
		"  • components/modal/modal.go",
		"  • components/modal/modal_test.go",
		"  • components/modal/modal.json",
		"",
		"Saving will overwrite the staged copies.",
	}, "\n")
	medium := modal.New(theme.Default).
		WithTitle("Save changes").
		WithSize(54, 12).
		WithBody(body).
		WithFooter("esc cancel · enter save")

	// Scene 3: no title, just a body and footer.
	bare := modal.New(theme.Default).
		WithSize(40, 7).
		WithBody("Network unreachable.\nRetrying in 3s…").
		WithFooter("press q to dismiss").
		WithCloseKey("q")

	scenes := []struct {
		name string
		m    modal.Modal
	}{
		{"small modal, title only", small},
		{"medium modal, title + body + footer", medium},
		{"no title, body + footer (close key: q)", bare},
	}

	bg := dimDots(bgW, bgH)
	for _, s := range scenes {
		fmt.Println(s.name + ":")
		placed := lipgloss.Place(
			bgW, bgH,
			lipgloss.Center, lipgloss.Center,
			s.m.View(),
			lipgloss.WithWhitespaceChars(""),
		)
		// Overlay the modal box on the dotted background: composite by line.
		fmt.Println(composite(bg, placed))
		fmt.Println()
	}
}

// composite overlays top onto bottom line-by-line. Wherever top has a
// non-empty rendered cell, top wins; otherwise the bottom cell shows. We
// use lipgloss.Place above to position the modal inside an empty field,
// so any rendered cell from top is the modal itself.
func composite(bottom, top string) string {
	bRows := strings.Split(bottom, "\n")
	tRows := strings.Split(top, "\n")
	n := len(bRows)
	if len(tRows) < n {
		n = len(tRows)
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		// If the top row is fully blank, keep the bottom. Otherwise show top.
		if strings.TrimSpace(stripANSI(tRows[i])) == "" {
			out[i] = bRows[i]
		} else {
			out[i] = tRows[i]
		}
	}
	return strings.Join(out, "\n")
}

// stripANSI removes ANSI escape sequences for the blank-line check. We
// don't need a robust implementation — only enough to detect rows that
// carry no visible glyphs.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == 0x1b {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
