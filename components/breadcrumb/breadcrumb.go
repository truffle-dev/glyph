// Package breadcrumb renders a path-style breadcrumb trail.
//
// The package is stateless: callers build a slice of Crumbs, call Render,
// and place the result inside their own View(). Long trails collapse with
// an ellipsis crumb that hides the middle of the path while keeping the
// root and the last few segments visible.
//
// Typical use is the top of a navigator pane: project / src / cmd / main.go
// with the trailing crumb tinted as the current location.
package breadcrumb

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// DefaultSeparator is the glyph between crumbs.
const DefaultSeparator = " › "

// Crumb is one segment in the trail.
type Crumb struct {
	Label string
	// Icon is rendered before the label; optional.
	Icon string
}

// Style overrides the per-crumb colors. Zero values fall back to the
// theme palette.
type Style struct {
	Foreground       lipgloss.Color
	ForegroundActive lipgloss.Color
	Separator        lipgloss.Color
	Background       lipgloss.Color
}

// Options control rendering behavior.
type Options struct {
	Separator string
	// MaxItems caps the number of crumbs rendered. When the trail is
	// longer, the first crumb (root) and the last MaxItems-2 are kept
	// and the middle collapses into a "…" crumb. Zero means no cap.
	MaxItems int
	Style    Style
}

// Render produces the breadcrumb line. Empty input returns "".
func Render(crumbs []Crumb, opts Options) string {
	if len(crumbs) == 0 {
		return ""
	}
	sep := opts.Separator
	if sep == "" {
		sep = DefaultSeparator
	}

	display := crumbs
	if opts.MaxItems > 0 && len(crumbs) > opts.MaxItems {
		// Keep root + last (MaxItems - 2) + ellipsis between.
		keepTail := opts.MaxItems - 2
		if keepTail < 1 {
			keepTail = 1
		}
		display = make([]Crumb, 0, opts.MaxItems)
		display = append(display, crumbs[0])
		display = append(display, Crumb{Label: "…"})
		display = append(display, crumbs[len(crumbs)-keepTail:]...)
	}

	last := len(display) - 1
	out := make([]string, 0, len(display)*2-1)
	for i, c := range display {
		out = append(out, crumbStyle(opts.Style, i == last).Render(formatCrumb(c)))
		if i < last {
			out = append(out, sepStyle(opts.Style).Render(sep))
		}
	}
	return strings.Join(out, "")
}

// RenderPath splits "a/b/c" on slashes and renders the resulting crumbs.
// Handy when the source data is already a slash-joined path.
func RenderPath(path string, opts Options) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	crumbs := make([]Crumb, len(parts))
	for i, p := range parts {
		crumbs[i] = Crumb{Label: p}
	}
	return Render(crumbs, opts)
}

func formatCrumb(c Crumb) string {
	if c.Icon != "" {
		return c.Icon + " " + c.Label
	}
	return c.Label
}

func crumbStyle(s Style, active bool) lipgloss.Style {
	fg := s.Foreground
	if fg == "" {
		fg = theme.Default.TextMuted
	}
	if active {
		fg = s.ForegroundActive
		if fg == "" {
			fg = theme.Default.Text
		}
	}
	st := lipgloss.NewStyle().Foreground(fg)
	if active {
		st = st.Bold(true)
	}
	if s.Background != "" {
		st = st.Background(s.Background)
	}
	return st
}

func sepStyle(s Style) lipgloss.Style {
	sep := s.Separator
	if sep == "" {
		sep = theme.Default.BorderStrong
	}
	st := lipgloss.NewStyle().Foreground(sep)
	if s.Background != "" {
		st = st.Background(s.Background)
	}
	return st
}
