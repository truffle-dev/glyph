// Package mdpreview is the nook side of the markdown preview pane.
// It wraps the reusable markdownviewer.Viewer from glyph's components
// directory with nook's path/focus/size conventions so the host can
// place it on the right column alongside git and term.
//
// The pane is read-only: it shows what's in the active buffer (or the
// freshly-saved file). Editing happens in the editor pane. Scrolling
// is handled by the embedded Viewer.
package mdpreview

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	markdownviewer "github.com/truffle-dev/glyph/components/markdown-viewer"
	"github.com/truffle-dev/glyph/components/theme"
)

// CancelMsg is emitted when the user presses Esc while the pane has focus.
// The host treats it as a request to close the preview.
type CancelMsg struct{}

// Pane is the markdown preview pane.
type Pane struct {
	theme   theme.Theme
	viewer  markdownviewer.Viewer
	path    string
	width   int
	height  int
	focused bool
	hasSrc  bool
}

// NewPane constructs an empty preview pane bound to the given theme.
func NewPane(t theme.Theme) Pane {
	return Pane{
		theme:  t,
		viewer: markdownviewer.New(t),
		width:  80,
		height: 20,
	}
}

// WithSize sets pane dimensions. The viewer's body height is one row
// shorter than the pane height to leave room for the title row.
func (p Pane) WithSize(w, h int) Pane {
	p.width = w
	p.height = h
	bodyH := h - 1
	if bodyH < 3 {
		bodyH = 3
	}
	p.viewer = p.viewer.WithSize(w, bodyH)
	return p
}

// WithSource replaces the markdown content and resets the scroll offset
// to the top. Path is used purely for the title row.
func (p Pane) WithSource(path, contents string) Pane {
	p.path = path
	p.viewer = p.viewer.WithSource(contents)
	p.hasSrc = true
	return p
}

// HasSource reports whether the pane currently holds content. The host
// uses this to decide whether to open the pane vs. show "no buffer."
func (p Pane) HasSource() bool { return p.hasSrc }

// SetTheme swaps the palette. The underlying glyph markdown viewer is
// rebuilt with the new theme; any current source contents are re-fed so
// the rendered output uses the new colors. Size is preserved.
func (p Pane) SetTheme(t theme.Theme) Pane {
	p.theme = t
	src := p.viewer.Source()
	bodyH := p.height - 1
	if bodyH < 3 {
		bodyH = 3
	}
	p.viewer = markdownviewer.New(t).WithSize(p.width, bodyH)
	if src != "" {
		p.viewer = p.viewer.WithSource(src)
	}
	return p
}

// Path returns the source path the pane is currently rendering.
func (p Pane) Path() string { return p.path }

// Focused reports whether the pane has keyboard focus.
func (p Pane) Focused() bool { return p.focused }

// Focus sets focused=true.
func (p Pane) Focus() Pane { p.focused = true; return p }

// Blur sets focused=false.
func (p Pane) Blur() Pane { p.focused = false; return p }

// Update routes scroll keys to the embedded viewer when focused, and
// emits CancelMsg on Esc. All other keys pass through.
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	if !p.focused {
		return p, nil
	}
	if km.Type == tea.KeyEsc {
		return p, func() tea.Msg { return CancelMsg{} }
	}
	switch km.Type {
	case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
		p.viewer, _ = p.viewer.Update(km)
	}
	return p, nil
}

// IsMarkdownPath reports whether the given path is something the preview
// can render. The check is case-insensitive on the extension; .md and
// .markdown are the only accepted suffixes. Files without an extension
// are rejected so the host can show a status-line hint.
func IsMarkdownPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}

// View renders the pane. Layout is:
//
//	title row    "preview  <basename>"  (muted if no source)
//	body rows    viewer output (height-1 lines)
func (p Pane) View() string {
	t := p.theme
	titleStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Bold(true)
	pathStyle := lipgloss.NewStyle().Foreground(t.Primary)
	muted := lipgloss.NewStyle().Foreground(t.TextMuted)

	title := titleStyle.Render("preview")
	if p.hasSrc && p.path != "" {
		title += "  " + pathStyle.Render(filepath.Base(p.path))
	} else {
		title += "  " + muted.Render("(no markdown buffer)")
	}
	if p.focused {
		title += "  " + muted.Render("· PgUp/PgDn to scroll, Esc to close")
	}

	body := p.viewer.View()
	return title + "\n" + body
}
