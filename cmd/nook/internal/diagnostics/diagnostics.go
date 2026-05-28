// Package diagnostics renders nook's workspace-wide problems panel.
//
// The host already collects every `textDocument/publishDiagnostics` event
// the language server emits, keyed by file path. This package turns that
// flat per-path map into a sortable list with one row per diagnostic and
// renders it as a focused overlay. Enter on a row jumps to the source
// site; Esc closes.
//
// Severity ordering matches LSP: Error < Warning < Info < Hint (lower is
// worse). Within a severity, rows sort by path then row then column so
// the same buffer's errors group together and the cursor walks top-down
// inside each file.
package diagnostics

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Severity is the local enum the pane carries. Values mirror the LSP
// severity values so a caller can pass `protocol.DiagnosticSeverity`
// directly via int conversion.
type Severity int

const (
	SeverityError   Severity = 1
	SeverityWarning Severity = 2
	SeverityInfo    Severity = 3
	SeverityHint    Severity = 4
)

// Mark returns the single-character badge rendered in the leftmost
// column of each row.
func (s Severity) Mark() string {
	switch s {
	case SeverityError:
		return "E"
	case SeverityWarning:
		return "W"
	case SeverityInfo:
		return "I"
	case SeverityHint:
		return "H"
	}
	return "?"
}

// Color returns the theme-token color for the severity. Falls back to
// muted for unknown values rather than panicking.
func (s Severity) Color(t theme.Theme) lipgloss.Color {
	switch s {
	case SeverityError:
		return t.Error
	case SeverityWarning:
		return t.Warning
	case SeverityInfo:
		return t.Info
	case SeverityHint:
		return t.TextMuted
	}
	return t.TextMuted
}

// Entry is a single diagnostic row.
type Entry struct {
	Path     string
	Row      int // 0-indexed
	Col      int // 0-indexed
	Severity Severity
	Source   string // "gopls", "rust-analyzer", etc.
	Message  string
}

// Sort returns the input sorted by (severity ASC, path ASC, row ASC,
// col ASC). The result is a new slice; the input is not mutated so
// callers can keep a stable snapshot.
func Sort(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	copy(out, entries)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity < out[j].Severity
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Row != out[j].Row {
			return out[i].Row < out[j].Row
		}
		return out[i].Col < out[j].Col
	})
	return out
}

// OpenAtMsg is emitted when the user accepts a row (Enter on a focused
// pane). The host opens the buffer and jumps the cursor.
type OpenAtMsg struct {
	Path string
	Row  int // 0-indexed
	Col  int // 0-indexed
}

// CancelMsg is emitted when the user presses Esc.
type CancelMsg struct{}

// Pane is the diagnostics overlay UI.
type Pane struct {
	theme   theme.Theme
	root    string
	entries []Entry // sorted at WithEntries time
	cursor  int
	focused bool
	width   int
	height  int
}

// NewPane returns a Pane with the given theme and workspace root. The
// root is used to display paths relative to the project, falling back
// to absolute paths when no Rel is possible.
func NewPane(t theme.Theme, root string) Pane {
	return Pane{theme: t, root: root, width: 80, height: 20}
}

// WithSize sets the rendered overlay size. Width is the column count of
// the bordered card; height is the row count including the header.
func (p Pane) WithSize(w, h int) Pane {
	if w < 30 {
		w = 30
	}
	if h < 6 {
		h = 6
	}
	p.width = w
	p.height = h
	return p
}

// SetTheme swaps the palette used for the bordered overlay, header,
// severity dots, and selected row. Next View() picks up the new colors.
func (p Pane) SetTheme(t theme.Theme) Pane { p.theme = t; return p }

// WithEntries replaces the entry list. The new list is sorted; the
// cursor clamps to a valid row.
func (p Pane) WithEntries(entries []Entry) Pane {
	p.entries = Sort(entries)
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.entries) {
		if len(p.entries) == 0 {
			p.cursor = 0
		} else {
			p.cursor = len(p.entries) - 1
		}
	}
	return p
}

// Focus marks the pane as accepting key input.
func (p Pane) Focus() Pane { p.focused = true; return p }

// Blur releases focus without changing visibility (the host decides
// when to drop the overlay).
func (p Pane) Blur() Pane { p.focused = false; return p }

// IsFocused returns whether the pane currently consumes key input.
func (p Pane) IsFocused() bool { return p.focused }

// Count returns the number of entries.
func (p Pane) Count() int { return len(p.entries) }

// Cursor returns the current selected index for tests.
func (p Pane) Cursor() int { return p.cursor }

// Selected returns the entry under the cursor and ok=true; ok=false
// when the entry list is empty.
func (p Pane) Selected() (Entry, bool) {
	if len(p.entries) == 0 {
		return Entry{}, false
	}
	if p.cursor < 0 || p.cursor >= len(p.entries) {
		return Entry{}, false
	}
	return p.entries[p.cursor], true
}

// Update handles key input when focused. Returns the updated pane and a
// command (OpenAtMsg on Enter, CancelMsg on Esc).
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	if !p.focused {
		return p, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch km.Type {
	case tea.KeyEsc:
		return p, func() tea.Msg { return CancelMsg{} }
	case tea.KeyEnter:
		if e, ok := p.Selected(); ok {
			return p, func() tea.Msg { return OpenAtMsg{Path: e.Path, Row: e.Row, Col: e.Col} }
		}
		return p, nil
	case tea.KeyUp:
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case tea.KeyDown:
		if p.cursor < len(p.entries)-1 {
			p.cursor++
		}
		return p, nil
	case tea.KeyHome:
		p.cursor = 0
		return p, nil
	case tea.KeyEnd:
		if len(p.entries) > 0 {
			p.cursor = len(p.entries) - 1
		}
		return p, nil
	case tea.KeyPgUp:
		step := p.visibleRows()
		p.cursor -= step
		if p.cursor < 0 {
			p.cursor = 0
		}
		return p, nil
	case tea.KeyPgDown:
		step := p.visibleRows()
		p.cursor += step
		if p.cursor >= len(p.entries) {
			if len(p.entries) > 0 {
				p.cursor = len(p.entries) - 1
			} else {
				p.cursor = 0
			}
		}
		return p, nil
	}
	return p, nil
}

// visibleRows returns the body height in rows (the height passed to
// WithSize minus header + borders).
func (p Pane) visibleRows() int {
	rows := p.height - 4
	if rows < 1 {
		rows = 1
	}
	return rows
}

// View renders the bordered card with the entries.
func (p Pane) View() string {
	body := p.body()
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.theme.Border).
		Background(p.theme.Surface).
		Padding(0, 1).
		Width(p.width - 2)
	return border.Render(body)
}

func (p Pane) body() string {
	innerWidth := p.width - 4
	if innerWidth < 20 {
		innerWidth = 20
	}

	header := p.renderHeader(innerWidth)

	if len(p.entries) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(p.theme.TextMuted).
			Italic(true).
			Render("no diagnostics in workspace.")
		return header + "\n\n" + empty
	}

	rows := p.visibleRows() - 1
	if rows < 1 {
		rows = 1
	}
	start := p.cursor - rows/2
	if start < 0 {
		start = 0
	}
	end := start + rows
	if end > len(p.entries) {
		end = len(p.entries)
		start = end - rows
		if start < 0 {
			start = 0
		}
	}

	lines := []string{header, ""}
	for i := start; i < end; i++ {
		lines = append(lines, p.renderRow(i, innerWidth))
	}
	return strings.Join(lines, "\n")
}

func (p Pane) renderHeader(width int) string {
	errs, warns, info, hints := p.counts()
	title := lipgloss.NewStyle().
		Foreground(p.theme.Primary).
		Bold(true).
		Render("workspace diagnostics")
	summary := lipgloss.NewStyle().
		Foreground(p.theme.TextMuted).
		Render(fmt.Sprintf("%d errors  %d warnings  %d info  %d hints", errs, warns, info, hints))
	gap := width - lipgloss.Width(title) - lipgloss.Width(summary)
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + summary
}

func (p Pane) counts() (errs, warns, info, hints int) {
	for _, e := range p.entries {
		switch e.Severity {
		case SeverityError:
			errs++
		case SeverityWarning:
			warns++
		case SeverityInfo:
			info++
		case SeverityHint:
			hints++
		}
	}
	return
}

func (p Pane) renderRow(idx, width int) string {
	e := p.entries[idx]
	mark := lipgloss.NewStyle().
		Foreground(e.Severity.Color(p.theme)).
		Bold(true).
		Render(e.Severity.Mark())
	location := p.formatLocation(e)
	loc := lipgloss.NewStyle().
		Foreground(p.theme.TextMuted).
		Render(location)
	source := ""
	if e.Source != "" {
		source = lipgloss.NewStyle().
			Foreground(p.theme.TextMuted).
			Italic(true).
			Render(" [" + e.Source + "]")
	}
	msg := lipgloss.NewStyle().Foreground(p.theme.Text).Render(e.Message)

	prefix := mark + " " + loc + source + "  "
	usedCells := lipgloss.Width(prefix)
	remaining := width - usedCells
	if remaining < 10 {
		remaining = 10
	}
	msg = truncateCells(msg, remaining)

	row := prefix + msg
	if idx == p.cursor {
		row = lipgloss.NewStyle().
			Background(p.theme.SurfaceStrong).
			Foreground(p.theme.Text).
			Bold(true).
			Width(width).
			Render(stripStyle(row))
	}
	return row
}

func (p Pane) formatLocation(e Entry) string {
	display := e.Path
	if p.root != "" {
		if rel, err := filepath.Rel(p.root, e.Path); err == nil && !strings.HasPrefix(rel, "..") {
			display = rel
		}
	}
	display = filepath.ToSlash(display)
	return fmt.Sprintf("%s:%d:%d", display, e.Row+1, e.Col+1)
}

// truncateCells truncates a styled string to at most n display cells.
// Strips ANSI for the measurement step, then truncates on the visible
// portion. Best-effort — relies on lipgloss.Width.
func truncateCells(s string, n int) string {
	if lipgloss.Width(s) <= n {
		return s
	}
	plain := stripStyle(s)
	if len(plain) > n {
		plain = plain[:n-1] + "…"
	}
	return plain
}

// stripStyle removes ANSI CSI sequences. Cheaper than a regex; the
// inputs here are short rendered cells.
func stripStyle(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
