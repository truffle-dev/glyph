// Package tabs renders a horizontal row of labeled tabs with one
// active label. It's a primitive: the component doesn't own the panels
// or their content. A parent model decides what to render below the
// tab row based on Active().
//
// Keys: left/right and tab/shift+tab cycle through the labels. Cycling
// wraps at the ends. Update returns the new Tabs and never produces a
// tea.Cmd; parents can listen for changes via Active().
package tabs

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Separator placed between adjacent tab labels.
const Separator = " · "

// Tabs is a horizontal row of selectable labels.
type Tabs struct {
	theme  theme.Theme
	labels []string
	active int
	width  int
}

// New constructs a Tabs primitive with the given theme. The label slice
// is empty until WithTabs is called.
func New(t theme.Theme) Tabs {
	return Tabs{theme: t}
}

// WithTabs replaces the label set. If the previous active index is out
// of range for the new labels, it resets to 0.
func (t Tabs) WithTabs(labels []string) Tabs {
	t.labels = labels
	if t.active >= len(t.labels) {
		t.active = 0
	}
	return t
}

// WithActive selects the active tab by index. Out-of-range values are
// clamped into the valid range; an empty label set forces 0.
func (t Tabs) WithActive(i int) Tabs {
	if len(t.labels) == 0 {
		t.active = 0
		return t
	}
	if i < 0 {
		i = 0
	}
	if i >= len(t.labels) {
		i = len(t.labels) - 1
	}
	t.active = i
	return t
}

// WithWidth sets an optional maximum render width. Values <= 0 mean no
// clamp; the row renders at its natural width.
func (t Tabs) WithWidth(w int) Tabs {
	if w < 0 {
		w = 0
	}
	t.width = w
	return t
}

// Update handles cycling keys. Left/Right and Tab/Shift+Tab move the
// active index by one, wrapping at the ends. All other messages pass
// through.
func (t Tabs) Update(msg tea.Msg) (Tabs, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return t, nil
	}
	if len(t.labels) == 0 {
		return t, nil
	}
	switch key.String() {
	case "right", "tab", "l":
		t.active = (t.active + 1) % len(t.labels)
	case "left", "shift+tab", "h":
		t.active = (t.active - 1 + len(t.labels)) % len(t.labels)
	}
	return t, nil
}

// View renders the labeled row. The active label is bold, underlined,
// and uses the theme's Primary color; inactive labels use Muted.
func (t Tabs) View() string {
	if len(t.labels) == 0 {
		return ""
	}
	active := lipgloss.NewStyle().Foreground(t.theme.Primary).Bold(true).Underline(true)
	inactive := lipgloss.NewStyle().Foreground(t.theme.TextMuted)
	sep := lipgloss.NewStyle().Foreground(t.theme.Border).Render(Separator)

	out := ""
	for i, label := range t.labels {
		if i > 0 {
			out += sep
		}
		if i == t.active {
			out += active.Render(label)
		} else {
			out += inactive.Render(label)
		}
	}
	if t.width > 0 {
		return lipgloss.NewStyle().MaxWidth(t.width).Render(out)
	}
	return out
}

// Active returns the index of the currently selected tab.
func (t Tabs) Active() int { return t.active }

// Labels returns a copy of the label list.
func (t Tabs) Labels() []string {
	out := make([]string, len(t.labels))
	copy(out, t.labels)
	return out
}
