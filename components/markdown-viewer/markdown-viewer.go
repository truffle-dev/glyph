// Package markdownviewer renders a small markdown subset to a styled
// terminal block with theme tokens. Headings, paragraphs, bullet lists,
// blockquotes, code blocks, inline code, bold, italic, links, and a
// horizontal rule. No tables, no nesting, no images.
//
// The viewer is a Bubble Tea model with a scrollable viewport. Up/Down
// scroll one line. PgUp/PgDn scroll a window. Home/End jump.
package markdownviewer

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"

	"github.com/truffle-dev/glyph/components/theme"
)

// Viewer is a Bubble Tea model that renders a markdown string to a
// scrollable terminal block.
type Viewer struct {
	theme  theme.Theme
	source string
	width  int
	height int
	offset int
}

// New constructs a Viewer with the given theme. Source is empty.
func New(t theme.Theme) Viewer {
	return Viewer{theme: t, width: 60, height: 20}
}

// WithSource sets the markdown string. Offset resets to 0.
func (v Viewer) WithSource(s string) Viewer {
	v.source = s
	v.offset = 0
	return v
}

// WithSize sets the rendered width and visible-row height. Minimums are
// enforced so the layout never collapses.
func (v Viewer) WithSize(w, h int) Viewer {
	if w < 20 {
		w = 20
	}
	if h < 3 {
		h = 3
	}
	v.width = w
	v.height = h
	return v
}

// Offset returns the current scroll offset in lines from the top.
func (v Viewer) Offset() int { return v.offset }

// Init implements tea.Model.
func (v Viewer) Init() tea.Cmd { return nil }

// Update handles scroll key events. Mouse wheel events are not handled.
func (v Viewer) Update(msg tea.Msg) (Viewer, tea.Cmd) {
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

// View renders the markdown source as a slice of `height` lines starting
// at `offset`. Trailing space is preserved so callers can compose the
// block with a border.
func (v Viewer) View() string {
	lines := v.renderLines()
	end := v.offset + v.height
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[v.offset:end]
	for len(visible) < v.height {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n")
}

// TotalLines is the count of rendered (post-wrap) lines.
func (v Viewer) TotalLines() int { return len(v.renderLines()) }

func (v Viewer) maxOffset() int {
	total := v.TotalLines()
	if total <= v.height {
		return 0
	}
	return total - v.height
}

// renderLines walks the source line by line, classifies each, applies
// the matching style, and word-wraps the result. The output is a flat
// slice of rendered terminal lines.
func (v Viewer) renderLines() []string {
	if v.source == "" {
		return []string{}
	}

	h1 := lipgloss.NewStyle().Foreground(v.theme.PrimaryStrong).Bold(true)
	h2 := lipgloss.NewStyle().Foreground(v.theme.Text).Bold(true).Underline(true)
	h3 := lipgloss.NewStyle().Foreground(v.theme.Text).Bold(true)
	muted := lipgloss.NewStyle().Foreground(v.theme.TextMuted)
	quote := lipgloss.NewStyle().Foreground(v.theme.TextMuted).Italic(true)
	hr := lipgloss.NewStyle().Foreground(v.theme.Border)
	codeBlock := lipgloss.NewStyle().
		Foreground(v.theme.Success).
		Background(v.theme.SurfaceStrong).
		Padding(0, 1)

	rawLines := strings.Split(v.source, "\n")
	var out []string
	inCode := false
	codeLang := ""

	for _, raw := range rawLines {
		trimmed := strings.TrimRight(raw, " \t")
		switch {
		case strings.HasPrefix(trimmed, "```"):
			if inCode {
				inCode = false
				codeLang = ""
			} else {
				inCode = true
				codeLang = strings.TrimPrefix(trimmed, "```")
				if codeLang != "" {
					out = append(out, muted.Render(codeLang))
				}
			}
		case inCode:
			out = append(out, codeBlock.Width(v.width).Render(raw))
		case trimmed == "---" || trimmed == "***":
			out = append(out, hr.Render(strings.Repeat("─", v.width)))
		case strings.HasPrefix(trimmed, "# "):
			body := strings.TrimPrefix(trimmed, "# ")
			out = append(out, wrap(h1.Render(applyInline(body, v.theme)), v.width)...)
		case strings.HasPrefix(trimmed, "## "):
			body := strings.TrimPrefix(trimmed, "## ")
			out = append(out, wrap(h2.Render(applyInline(body, v.theme)), v.width)...)
		case strings.HasPrefix(trimmed, "### "):
			body := strings.TrimPrefix(trimmed, "### ")
			out = append(out, wrap(h3.Render(applyInline(body, v.theme)), v.width)...)
		case strings.HasPrefix(trimmed, "> "):
			body := strings.TrimPrefix(trimmed, "> ")
			rendered := quote.Render("│ " + applyInline(body, v.theme))
			out = append(out, wrap(rendered, v.width)...)
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
			body := trimmed[2:]
			marker := lipgloss.NewStyle().Foreground(v.theme.Primary).Render("• ")
			rendered := marker + applyInline(body, v.theme)
			out = append(out, wrap(rendered, v.width)...)
		case trimmed == "":
			out = append(out, "")
		default:
			rendered := applyInline(trimmed, v.theme)
			out = append(out, wrap(rendered, v.width)...)
		}
	}
	return out
}

// applyInline expands the common inline markers `**bold**`, `*italic*`,
// “ `code` “, and `[text](url)` into ANSI-styled equivalents.
func applyInline(s string, t theme.Theme) string {
	s = replacePairs(s, "**", lipgloss.NewStyle().Bold(true).Foreground(t.Text))
	s = replacePairs(s, "`", lipgloss.NewStyle().Foreground(t.Success).Background(t.SurfaceStrong))
	s = replaceLinks(s, t)
	s = replacePairs(s, "*", lipgloss.NewStyle().Italic(true).Foreground(t.Text))
	return s
}

// replacePairs swaps every paired occurrence of marker with the contents
// rendered by style. Unpaired markers are left alone.
func replacePairs(s, marker string, style lipgloss.Style) string {
	var b strings.Builder
	mlen := len(marker)
	for {
		i := strings.Index(s, marker)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		j := strings.Index(s[i+mlen:], marker)
		if j < 0 {
			b.WriteString(s)
			return b.String()
		}
		j += i + mlen
		b.WriteString(s[:i])
		b.WriteString(style.Render(s[i+mlen : j]))
		s = s[j+mlen:]
	}
}

// replaceLinks turns `[text](url)` into a styled link rendering of the
// text, with the URL shown in muted parentheses.
func replaceLinks(s string, t theme.Theme) string {
	textStyle := lipgloss.NewStyle().Foreground(t.Info).Underline(true)
	urlStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	var b strings.Builder
	for {
		i := strings.Index(s, "[")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		closeText := strings.Index(s[i:], "]")
		if closeText < 0 {
			b.WriteString(s)
			return b.String()
		}
		closeText += i
		if closeText+1 >= len(s) || s[closeText+1] != '(' {
			b.WriteString(s[:closeText+1])
			s = s[closeText+1:]
			continue
		}
		closeURL := strings.Index(s[closeText+2:], ")")
		if closeURL < 0 {
			b.WriteString(s)
			return b.String()
		}
		closeURL += closeText + 2
		text := s[i+1 : closeText]
		url := s[closeText+2 : closeURL]
		b.WriteString(s[:i])
		b.WriteString(textStyle.Render(text))
		b.WriteString(" ")
		b.WriteString(urlStyle.Render("(" + url + ")"))
		s = s[closeURL+1:]
	}
}

func wrap(s string, width int) []string {
	wrapped := wordwrap.String(s, width)
	return strings.Split(wrapped, "\n")
}
