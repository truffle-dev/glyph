//go:build glyph_story

package main

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

func main() {
	rows := []struct {
		Label string
		Color lipgloss.Color
	}{
		{"bg            ", theme.Default.Bg},
		{"surface       ", theme.Default.Surface},
		{"surface-strong", theme.Default.SurfaceStrong},
		{"border        ", theme.Default.Border},
		{"border-strong ", theme.Default.BorderStrong},
		{"text          ", theme.Default.Text},
		{"text-muted    ", theme.Default.TextMuted},
		{"primary       ", theme.Default.Primary},
		{"primary-strong", theme.Default.PrimaryStrong},
		{"accent        ", theme.Default.Accent},
		{"success       ", theme.Default.Success},
		{"warning       ", theme.Default.Warning},
		{"error         ", theme.Default.Error},
		{"info          ", theme.Default.Info},
	}
	for _, r := range rows {
		swatch := lipgloss.NewStyle().Background(r.Color).Render("        ")
		label := lipgloss.NewStyle().Foreground(theme.Default.Text).Render(r.Label)
		hex := lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Render(string(r.Color))
		fmt.Printf("  %s  %s  %s\n", label, swatch, hex)
	}
}
