// Package settings renders nook's effective configuration as a read-only
// overlay.
//
// Config loading, per-project inheritance, and live reload all ship (the
// config package plus alt+, and the configwatch tick loop), but until now
// there was no way to *see* the values that actually took effect. A user
// who layers a project .nook/config.toml on top of their global file had
// to reason about the merge in their head. This overlay prints the merged
// result: one labeled row per editor setting, plus a footer naming the two
// config scopes it was assembled from.
//
// It is intentionally read-only. Editing settings in place is a larger
// surface (a focused, navigable form); this slice answers the smaller,
// more common question — "what is nook actually using right now?" — the
// way `git config --list` or Zed's settings view does. It mirrors the help
// overlay: a fixed-width card bound to alt+. and dismissed by Esc or alt+.
package settings

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/cmd/nook/internal/config"
	"github.com/truffle-dev/glyph/components/theme"
)

// onOff renders a bool as the same words the TOML keys read as, so the
// overlay and the config file speak the same language.
func onOff(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// rows returns the effective settings as ordered label/value pairs. The
// order matches config.EditorConfig's declaration so a reader scanning the
// overlay and the struct sees the same sequence.
func rows(cfg config.Config) [][2]string {
	e := cfg.Editor
	return [][2]string{
		{"theme", e.Theme},
		{"tab_width", fmt.Sprintf("%d", e.TabWidth)},
		{"format_on_save", onOff(e.FormatOnSave)},
		{"line_numbers", onOff(e.LineNumbers)},
		{"indent_guides", onOff(e.IndentGuides)},
		{"inlay_hints", onOff(e.InlayHints)},
		{"soft_wrap", onOff(e.SoftWrap)},
	}
}

// scopeLine renders one footer row describing a config scope: its path and
// whether a file is present there. An absent file is normal (both scopes
// are optional), so it reads "not present" rather than as an error.
func scopeLine(label, path string, present bool) string {
	state := "not present"
	if present {
		state = "loaded"
	}
	if path == "" {
		return fmt.Sprintf("%s: (unset)", label)
	}
	return fmt.Sprintf("%s: %s (%s)", label, path, state)
}

// View renders the settings card. cfg is the effective merged config (user
// global overlaid with the per-project file, exactly what the host applied).
// userPath/projectPath are the two scope locations and the *Exists flags say
// whether a file is actually present at each. width is the host's column
// count; the card clamps to ~64 columns so the label ladder lines up.
func View(t theme.Theme, width int, cfg config.Config, userPath, projectPath string, userExists, projectExists bool) string {
	inner := 64
	if width < inner+4 {
		inner = width - 4
		if inner < 30 {
			inner = 30
		}
	}

	title := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Render("nook settings")
	subtitle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Italic(true).
		Render("effective values · press alt+. or esc to dismiss")

	keyStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	valStyle := lipgloss.NewStyle().
		Foreground(t.Text)
	footStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	var body []string
	body = append(body, title)
	body = append(body, subtitle)
	body = append(body, "")

	for _, r := range rows(cfg) {
		body = append(body, fmt.Sprintf("  %-18s  %s",
			keyStyle.Render(r[0]),
			valStyle.Render(r[1]),
		))
	}

	body = append(body, "")
	body = append(body, footStyle.Render(scopeLine("user config", userPath, userExists)))
	body = append(body, footStyle.Render(scopeLine("project config", projectPath, projectExists)))

	card := strings.Join(body, "\n")

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(1, 2).
		Width(inner).
		Background(t.Surface)
	return border.Render(card)
}
