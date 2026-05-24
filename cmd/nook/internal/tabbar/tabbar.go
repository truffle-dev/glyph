// Package tabbar renders the open-buffer strip above the editor surface.
// Pure renderer: given a slice of tabs, an active index, and a width, it
// returns a single styled row. No state, no I/O. Buffer ownership lives in
// bufman; this package is just the visual.
package tabbar

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/cmd/nook/internal/bufman"
	"github.com/truffle-dev/glyph/components/theme"
)

// View renders the tab bar as a single full-width row. Returns "" when there
// are no tabs (so the host can hide the row entirely on the welcome screen).
func View(t theme.Theme, tabs []bufman.TabInfo, active, width int) string {
	if len(tabs) == 0 || width <= 0 {
		return ""
	}

	labels := dedupLabels(tabs)
	dirty := make([]string, len(tabs))
	for i, tab := range tabs {
		if tab.Dirty {
			dirty[i] = "●"
		}
	}

	activeStyle := lipgloss.NewStyle().
		Foreground(t.TextInverse).
		Background(t.Primary).
		Bold(true).
		Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Padding(0, 1)
	dirtyStyle := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	sepStyle := lipgloss.NewStyle().Foreground(t.Border)
	overflowStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	barStyle := lipgloss.NewStyle().Background(t.Surface).Width(width)

	// Each rendered tab is "{label}{dirty?}". Width budget includes 2-char
	// horizontal padding from the style on each side.
	rendered := make([]string, len(tabs))
	visible := make([]int, len(tabs))
	for i, label := range labels {
		text := label
		if dirty[i] != "" {
			text = label + " " + dirtyStyle.Render(dirty[i])
		}
		if i == active {
			rendered[i] = activeStyle.Render(text)
		} else {
			rendered[i] = inactiveStyle.Render(text)
		}
		visible[i] = lipgloss.Width(rendered[i])
	}

	sep := sepStyle.Render(" │ ")
	sepW := lipgloss.Width(sep)

	// Try the no-truncation case first.
	total := 0
	for i, w := range visible {
		total += w
		if i > 0 {
			total += sepW
		}
	}
	if total <= width {
		row := strings.Join(rendered, sep)
		return barStyle.Render(row)
	}

	// Overflow path: budget around the active tab and prefer keeping recent
	// neighbours visible. Walk outward from active until the budget is gone.
	overflowMark := overflowStyle.Render(" … ")
	overflowW := lipgloss.Width(overflowMark)
	budget := width - overflowW // reserve room for ellipsis indicator

	pick := map[int]bool{}
	used := visible[active]
	pick[active] = true
	left, right := active-1, active+1
	for left >= 0 || right < len(tabs) {
		// Prefer right (newer) neighbour first so users see what they just opened.
		if right < len(tabs) {
			cost := visible[right] + sepW
			if used+cost <= budget {
				pick[right] = true
				used += cost
				right++
				continue
			}
			// Right exhausted further additions on that side.
			right = len(tabs)
		}
		if left >= 0 {
			cost := visible[left] + sepW
			if used+cost <= budget {
				pick[left] = true
				used += cost
				left--
				continue
			}
			left = -1
		}
		if right >= len(tabs) && left < 0 {
			break
		}
	}

	parts := []string{}
	hadGap := false
	for i := 0; i < len(tabs); i++ {
		if pick[i] {
			parts = append(parts, rendered[i])
			hadGap = false
			continue
		}
		if !hadGap {
			parts = append(parts, overflowMark)
			hadGap = true
		}
	}
	row := strings.Join(parts, sep)
	return barStyle.Render(row)
}

// dedupLabels returns one display label per tab. The default is the
// basename; when two tabs share the same basename, both labels get a parent
// dir prefix so the user can tell them apart. Three-way collisions get two
// parent dirs, etc. Labels always render with forward slashes regardless of
// OS; the tab bar is a display surface, not a filesystem path.
func dedupLabels(tabs []bufman.TabInfo) []string {
	out := make([]string, len(tabs))
	depth := 1
	for {
		labelOf := func(path string) string {
			parts := splitPath(path)
			if len(parts) == 0 {
				return ""
			}
			start := len(parts) - depth
			if start < 0 {
				start = 0
			}
			return strings.Join(parts[start:], "/")
		}
		seen := map[string][]int{}
		for i, tab := range tabs {
			label := labelOf(tab.Path)
			if label == "" {
				label = "(untitled)"
			}
			out[i] = label
			seen[label] = append(seen[label], i)
		}
		conflict := false
		for _, idxs := range seen {
			if len(idxs) > 1 {
				conflict = true
				break
			}
		}
		if !conflict || depth > 4 {
			return out
		}
		depth++
	}
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	parts := strings.Split(clean, "/")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
