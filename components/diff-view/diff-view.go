// Package diffview renders a unified diff as a color-coded, scrollable
// terminal block. Additions are green, removals are red, hunk headers
// are highlighted, context lines are muted. Optional line numbers from
// either side of the diff.
//
// The component is render-only: a consumer parses (or generates) a list
// of Line values, hands them to WithLines, and the view does the rest.
// A ParseUnified helper is provided for the common `diff -u` case.
package diffview

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Kind classifies a diff line.
type Kind int

const (
	KindContext Kind = iota
	KindAdded
	KindRemoved
	KindHunk // @@ -1,3 +1,4 @@
	KindFile // --- a/file, +++ b/file
)

// Line is a single rendered row in the diff.
type Line struct {
	Kind    Kind
	Old     int // 1-based old line number, 0 if not applicable
	New     int // 1-based new line number, 0 if not applicable
	Content string
}

// View is a Bubble Tea model that renders a slice of Line values.
type View struct {
	theme        theme.Theme
	lines        []Line
	width        int
	height       int
	offset       int
	showLineNums bool
}

// New constructs an empty View with line numbers enabled.
func New(t theme.Theme) View {
	return View{theme: t, width: 80, height: 20, showLineNums: true}
}

// WithLines sets the diff content. Offset resets to 0.
func (v View) WithLines(lines []Line) View {
	v.lines = append([]Line(nil), lines...)
	v.offset = 0
	return v
}

// WithSize sets rendered width and visible-row height.
func (v View) WithSize(w, h int) View {
	if w < 30 {
		w = 30
	}
	if h < 3 {
		h = 3
	}
	v.width = w
	v.height = h
	return v
}

// WithLineNumbers toggles the left-side line-number columns.
func (v View) WithLineNumbers(show bool) View { v.showLineNums = show; return v }

// Offset returns the current scroll offset (lines from top).
func (v View) Offset() int { return v.offset }

// Init implements tea.Model.
func (v View) Init() tea.Cmd { return nil }

// Update handles scroll keys.
func (v View) Update(msg tea.Msg) (View, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return v, nil
	}
	max := v.maxOffset()
	switch key.Type {
	case tea.KeyUp:
		v.offset--
	case tea.KeyDown:
		v.offset++
	case tea.KeyPgUp:
		v.offset -= v.height
	case tea.KeyPgDown:
		v.offset += v.height
	case tea.KeyHome:
		v.offset = 0
	case tea.KeyEnd:
		v.offset = max
	}
	if v.offset < 0 {
		v.offset = 0
	}
	if v.offset > max {
		v.offset = max
	}
	return v, nil
}

// View renders the visible window. Lines longer than the available
// content width are truncated with an ellipsis.
func (v View) View() string {
	if len(v.lines) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(v.theme.TextMuted).
			Italic(true).
			Render("No diff.")
		out := []string{empty}
		for len(out) < v.height {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}
	rendered := v.renderAll()
	end := v.offset + v.height
	if end > len(rendered) {
		end = len(rendered)
	}
	visible := rendered[v.offset:end]
	for len(visible) < v.height {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n")
}

// TotalLines returns the number of rendered rows. Equals len(WithLines).
func (v View) TotalLines() int { return len(v.lines) }

func (v View) maxOffset() int {
	if len(v.lines) <= v.height {
		return 0
	}
	return len(v.lines) - v.height
}

func (v View) renderAll() []string {
	width := v.lineNumWidth()
	bodyWidth := v.width - width - 2 // marker col + space
	if bodyWidth < 10 {
		bodyWidth = 10
	}
	out := make([]string, len(v.lines))
	for i, ln := range v.lines {
		out[i] = v.renderOne(ln, width, bodyWidth)
	}
	return out
}

func (v View) renderOne(ln Line, numWidth, bodyWidth int) string {
	var marker string
	var bodyStyle lipgloss.Style
	switch ln.Kind {
	case KindAdded:
		marker = "+"
		bodyStyle = lipgloss.NewStyle().Foreground(v.theme.Success)
	case KindRemoved:
		marker = "-"
		bodyStyle = lipgloss.NewStyle().Foreground(v.theme.Error)
	case KindHunk:
		marker = "@"
		bodyStyle = lipgloss.NewStyle().Foreground(v.theme.Accent).Bold(true)
	case KindFile:
		marker = " "
		bodyStyle = lipgloss.NewStyle().Foreground(v.theme.PrimaryStrong).Bold(true)
	default:
		marker = " "
		bodyStyle = lipgloss.NewStyle().Foreground(v.theme.Text)
	}

	body := truncate(ln.Content, bodyWidth)

	if !v.showLineNums || ln.Kind == KindHunk || ln.Kind == KindFile {
		blanks := ""
		if v.showLineNums {
			blanks = strings.Repeat(" ", numWidth)
		}
		return blanks + bodyStyle.Render(marker+" "+body)
	}

	numStyle := lipgloss.NewStyle().Foreground(v.theme.TextMuted)
	oldStr := formatNum(ln.Old)
	newStr := formatNum(ln.New)
	col := numStyle.Render(padLeft(oldStr, (numWidth-1)/2) + " " + padLeft(newStr, numWidth-((numWidth-1)/2)-1))
	return col + bodyStyle.Render(marker+" "+body)
}

func (v View) lineNumWidth() int {
	if !v.showLineNums {
		return 0
	}
	maxOld, maxNew := 0, 0
	for _, ln := range v.lines {
		if ln.Old > maxOld {
			maxOld = ln.Old
		}
		if ln.New > maxNew {
			maxNew = ln.New
		}
	}
	wOld := digits(maxOld)
	wNew := digits(maxNew)
	if wOld < 1 {
		wOld = 1
	}
	if wNew < 1 {
		wNew = 1
	}
	return wOld + wNew + 2 // old col + space + new col + trailing space
}

func digits(n int) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}

func formatNum(n int) string {
	if n <= 0 {
		return ""
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func padLeft(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

func truncate(s string, w int) string {
	if len(s) <= w {
		return s
	}
	if w <= 1 {
		return "…"
	}
	return s[:w-1] + "…"
}

// ParseUnified converts a standard unified-diff string into a slice of
// Line values. Lines starting with "+++", "---", "diff", or "index" are
// classified as KindFile. Lines starting with "@@" are KindHunk. The
// usual leading-space / + / - convention drives KindContext, KindAdded,
// KindRemoved.
func ParseUnified(diff string) []Line {
	if diff == "" {
		return nil
	}
	var out []Line
	oldNo, newNo := 0, 0
	for _, raw := range strings.Split(diff, "\n") {
		if raw == "" {
			continue
		}
		switch {
		case strings.HasPrefix(raw, "diff "),
			strings.HasPrefix(raw, "index "),
			strings.HasPrefix(raw, "--- "),
			strings.HasPrefix(raw, "+++ "):
			out = append(out, Line{Kind: KindFile, Content: raw})
		case strings.HasPrefix(raw, "@@"):
			oldNo, newNo = parseHunkStart(raw)
			out = append(out, Line{Kind: KindHunk, Content: raw})
		case strings.HasPrefix(raw, "+"):
			out = append(out, Line{Kind: KindAdded, New: newNo, Content: raw[1:]})
			newNo++
		case strings.HasPrefix(raw, "-"):
			out = append(out, Line{Kind: KindRemoved, Old: oldNo, Content: raw[1:]})
			oldNo++
		default:
			body := raw
			if strings.HasPrefix(raw, " ") {
				body = raw[1:]
			}
			out = append(out, Line{Kind: KindContext, Old: oldNo, New: newNo, Content: body})
			oldNo++
			newNo++
		}
	}
	return out
}

// parseHunkStart extracts the starting old/new line numbers from a hunk
// header like "@@ -12,4 +14,6 @@ optional comment". Returns 0/0 on
// malformed input rather than erroring; the renderer still draws the
// header correctly.
func parseHunkStart(s string) (oldStart, newStart int) {
	rest := strings.TrimPrefix(s, "@@")
	rest = strings.TrimSpace(rest)
	end := strings.Index(rest, "@@")
	if end > 0 {
		rest = rest[:end]
	}
	parts := strings.Fields(rest)
	for _, p := range parts {
		var dst *int
		if strings.HasPrefix(p, "-") {
			dst = &oldStart
		} else if strings.HasPrefix(p, "+") {
			dst = &newStart
		} else {
			continue
		}
		p = p[1:]
		comma := strings.Index(p, ",")
		if comma >= 0 {
			p = p[:comma]
		}
		n := 0
		for i := 0; i < len(p); i++ {
			c := p[i]
			if c < '0' || c > '9' {
				n = 0
				break
			}
			n = n*10 + int(c-'0')
		}
		*dst = n
	}
	return oldStart, newStart
}
