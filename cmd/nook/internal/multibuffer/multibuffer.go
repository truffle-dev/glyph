// Package multibuffer renders fragments stitched from multiple files into
// one scrollable surface. The first slice (v0.13.0) is read-only: each
// fragment shows a header (path + line range) followed by the lines from
// the source file with markers for added / context lines. Enter on any
// row fires an OpenAtMsg the host uses to open that file in the editor
// at the row's line.
//
// The single load source so far is `git diff --unified=3 HEAD` — every
// modified hunk from working tree against HEAD, including staged and
// unstaged. Future loaders (workspace symbol references, find-all-results)
// will return the same []Fragment shape.
//
// Editable multibuffer (edits flowing back to source) is intentionally
// deferred. Read-only is a complete and useful slice on its own.
package multibuffer

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Marker is the per-line annotation in a fragment. Context lines are
// unchanged in the diff; Added lines were inserted. (Modified is unused
// for diff-source fragments — git's unified diff models a modification
// as a delete + add pair, both of which the parser surfaces individually.)
type Marker int

const (
	Context Marker = iota
	Added
	Modified
)

// Line is one displayed line within a Fragment.
type Line struct {
	Marker   Marker
	FileLine int // 1-based row in the source file
	Text     string
}

// Fragment is a contiguous slice of one file rendered together.
type Fragment struct {
	Path      string // absolute
	StartLine int    // 1-based, first line in the source file
	EndLine   int    // 1-based, last line in the source file
	Lines     []Line // ordered top-to-bottom
	Suffix    string // optional, e.g. function name from "@@ -... @@ <suffix>"
}

// FragmentsMsg is the async response from a load command.
type FragmentsMsg struct {
	Fragments []Fragment
	Source    string // "diff" / "references" / "search" — informational
	Err       error
}

// OpenAtMsg fires when the user presses Enter on a fragment row.
type OpenAtMsg struct {
	Path string
	Line int // 1-based
}

// CancelMsg fires on Esc.
type CancelMsg struct{}

// rowKind tags a row in the rendered pane.
type rowKind int

const (
	rowHeader rowKind = iota
	rowContent
	rowSeparator
)

type row struct {
	kind    rowKind
	fragIdx int
	lineIdx int
}

// Pane is the multibuffer UI model. Construct with NewPane, populate with
// SetFragments (typically the response from LoadDiffCmd), focus via Focus.
// Pane is a value type — pass-by-value, return-by-value.
type Pane struct {
	theme     theme.Theme
	root      string
	fragments []Fragment
	rows      []row
	cursor    int
	scroll    int
	width     int
	height    int
	focused   bool
	title     string
	err       error
}

// NewPane constructs an empty pane rooted at root. The root is used only
// for path relativization in the header; absolute paths in fragments are
// preserved untouched.
func NewPane(t theme.Theme, root string) Pane {
	return Pane{
		theme:  t,
		root:   root,
		width:  80,
		height: 20,
	}
}

// WithSize sets render dimensions. The pane will clamp internal computations
// against these on every View call.
func (p Pane) WithSize(w, h int) Pane { p.width = w; p.height = h; return p }

// SetTheme swaps the palette used to render fragment headers, context lines,
// and the Added/Deleted accents. Next View() picks up the new colors.
func (p Pane) SetTheme(t theme.Theme) Pane { p.theme = t; return p }

// Focused reports keyboard focus.
func (p Pane) Focused() bool { return p.focused }

// Focus sets focused=true.
func (p Pane) Focus() Pane { p.focused = true; return p }

// Blur sets focused=false.
func (p Pane) Blur() Pane { p.focused = false; return p }

// Fragments returns the current fragment slice (read-only — callers must
// not mutate the underlying lines slice).
func (p Pane) Fragments() []Fragment { return p.fragments }

// Title returns the human-readable title (e.g. "uncommitted changes").
func (p Pane) Title() string { return p.title }

// Err returns the load error, if any.
func (p Pane) Err() error { return p.err }

// Reset clears state and prepares for a new load. Title shows above the
// fragment list while the load is in flight.
func (p Pane) Reset(title string) Pane {
	p.title = title
	p.fragments = nil
	p.rows = nil
	p.cursor = 0
	p.scroll = 0
	p.err = nil
	return p
}

// SetFragments replaces displayed fragments and rebuilds row positions.
// Cursor lands on the first non-separator row.
func (p Pane) SetFragments(frags []Fragment, err error) Pane {
	p.fragments = frags
	p.err = err
	p.rows = buildRows(frags)
	p.cursor = 0
	p.scroll = 0
	p = p.skipSeparators(+1)
	return p
}

// Selected returns (path, line, true) for the row under the cursor. Header
// rows resolve to (frag.Path, frag.StartLine); content rows resolve to
// (frag.Path, line.FileLine). Separators and out-of-range cursors return
// (empty, 0, false).
func (p Pane) Selected() (string, int, bool) {
	if p.cursor < 0 || p.cursor >= len(p.rows) {
		return "", 0, false
	}
	r := p.rows[p.cursor]
	if r.fragIdx < 0 || r.fragIdx >= len(p.fragments) {
		return "", 0, false
	}
	f := p.fragments[r.fragIdx]
	switch r.kind {
	case rowHeader:
		return f.Path, f.StartLine, true
	case rowContent:
		if r.lineIdx < 0 || r.lineIdx >= len(f.Lines) {
			return "", 0, false
		}
		return f.Path, f.Lines[r.lineIdx].FileLine, true
	}
	return "", 0, false
}

// Update routes a key event. Esc emits CancelMsg, Enter emits OpenAtMsg
// for the currently-selected row.
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch km.Type {
	case tea.KeyEsc:
		return p, func() tea.Msg { return CancelMsg{} }
	case tea.KeyEnter:
		if path, line, ok := p.Selected(); ok {
			return p, func() tea.Msg { return OpenAtMsg{Path: path, Line: line} }
		}
		return p, nil
	case tea.KeyUp, tea.KeyCtrlP:
		return p.moveBy(-1), nil
	case tea.KeyDown, tea.KeyCtrlN:
		return p.moveBy(+1), nil
	case tea.KeyPgUp:
		return p.moveBy(-(p.bodyHeight() - 2)), nil
	case tea.KeyPgDown:
		return p.moveBy(+(p.bodyHeight() - 2)), nil
	case tea.KeyHome:
		p.cursor = 0
		p = p.skipSeparators(+1)
		p.scroll = 0
		return p, nil
	case tea.KeyEnd:
		p.cursor = len(p.rows) - 1
		p = p.skipSeparators(-1)
		return p, nil
	}
	return p, nil
}

// moveBy advances the cursor by `n` rows. Separator rows are skipped in
// the direction of motion so the user never lands on a divider line.
func (p Pane) moveBy(n int) Pane {
	if len(p.rows) == 0 || n == 0 {
		return p
	}
	dir := 1
	if n < 0 {
		dir = -1
		n = -n
	}
	for i := 0; i < n; i++ {
		nx := p.cursor + dir
		for nx >= 0 && nx < len(p.rows) && p.rows[nx].kind == rowSeparator {
			nx += dir
		}
		if nx < 0 || nx >= len(p.rows) {
			break
		}
		p.cursor = nx
	}
	return p
}

// skipSeparators nudges the cursor in the given direction if it currently
// sits on a separator. Direction is +1 or -1.
func (p Pane) skipSeparators(dir int) Pane {
	if len(p.rows) == 0 {
		return p
	}
	for p.cursor >= 0 && p.cursor < len(p.rows) && p.rows[p.cursor].kind == rowSeparator {
		p.cursor += dir
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.rows) {
		p.cursor = len(p.rows) - 1
	}
	return p
}

// bodyHeight is the number of rendered rows below the title line.
func (p Pane) bodyHeight() int {
	if p.height < 3 {
		return 1
	}
	return p.height - 1
}

// View renders the pane to a single string.
func (p Pane) View() string {
	t := p.theme
	titleStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	pathStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	locStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true)
	contextStyle := lipgloss.NewStyle().Foreground(t.Text)
	addedStyle := lipgloss.NewStyle().Foreground(t.Success)
	cursorStyle := lipgloss.NewStyle().Foreground(t.TextInverse).Background(t.Primary).Bold(true)
	sepStyle := lipgloss.NewStyle().Foreground(t.Border)
	errStyle := lipgloss.NewStyle().Foreground(t.Error)

	title := p.title
	if title == "" {
		title = "multibuffer"
	}
	header := titleStyle.Render(title) + "  " + mutedStyle.Render(p.summary())

	if p.err != nil {
		return strings.Join([]string{header, errStyle.Render("error: " + p.err.Error())}, "\n")
	}

	if len(p.rows) == 0 {
		empty := mutedStyle.Italic(true).Render("no fragments — press esc to close")
		return header + "\n" + empty
	}

	body := p.bodyHeight()
	if p.cursor >= p.scroll+body {
		p.scroll = p.cursor - body + 1
	}
	if p.cursor < p.scroll {
		p.scroll = p.cursor
	}
	if p.scroll < 0 {
		p.scroll = 0
	}
	end := p.scroll + body
	if end > len(p.rows) {
		end = len(p.rows)
	}

	innerW := p.width
	if innerW < 8 {
		innerW = 8
	}

	var lines []string
	for i := p.scroll; i < end; i++ {
		r := p.rows[i]
		var rendered string
		switch r.kind {
		case rowSeparator:
			sepW := innerW - 2
			if sepW < 4 {
				sepW = 4
			}
			rendered = sepStyle.Render(strings.Repeat("─", sepW))
		case rowHeader:
			f := p.fragments[r.fragIdx]
			rel := relativize(f.Path, p.root)
			rng := fmt.Sprintf("%d-%d", f.StartLine, f.EndLine)
			head := pathStyle.Render(rel) + "  " + locStyle.Render(rng)
			if s := strings.TrimSpace(f.Suffix); s != "" {
				head += "  " + locStyle.Render(s)
			}
			rendered = head
		case rowContent:
			f := p.fragments[r.fragIdx]
			ln := f.Lines[r.lineIdx]
			num := fmt.Sprintf("%4d ", ln.FileLine)
			mark := " "
			style := contextStyle
			switch ln.Marker {
			case Added:
				mark = "+"
				style = addedStyle
			case Modified:
				mark = "~"
				style = addedStyle
			}
			textW := innerW - len(num) - 3
			if textW < 4 {
				textW = 4
			}
			rendered = locStyle.Render(num) + style.Render(mark+" "+truncate(ln.Text, textW))
		}
		if i == p.cursor && p.focused {
			rendered = cursorStyle.Render(padRight(stripANSI(rendered), innerW-2))
		} else {
			rendered = padRight(rendered, innerW-2)
		}
		lines = append(lines, rendered)
	}
	for len(lines) < body {
		lines = append(lines, padRight("", innerW-2))
	}
	return header + "\n" + strings.Join(lines, "\n")
}

// summary renders the count blurb shown next to the title.
func (p Pane) summary() string {
	nf := len(p.fragments)
	if nf == 0 {
		return ""
	}
	var nLines int
	for _, f := range p.fragments {
		nLines += len(f.Lines)
	}
	return fmt.Sprintf("• %d fragments • %d lines", nf, nLines)
}

// LoadDiffCmd is a tea.Cmd that runs `git diff --no-color --unified=3 base`
// and returns a FragmentsMsg with the parsed hunks. base may be "" (working
// tree vs index — staged-only changes are excluded) or "HEAD" (working tree
// vs HEAD — covers staged and unstaged together) or any other rev. When the
// command exits non-zero, the err is wrapped in the message and the pane
// shows it in place of the fragment list.
func LoadDiffCmd(root, base string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		args := []string{"-C", root, "diff", "--no-color", "--unified=3"}
		if base != "" {
			args = append(args, base)
		}
		cmd := exec.CommandContext(ctx, "git", args...)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			return FragmentsMsg{Source: "diff", Err: fmt.Errorf("git diff: %w", err)}
		}
		frags, perr := Parse(out.Bytes(), root)
		return FragmentsMsg{Fragments: frags, Source: "diff", Err: perr}
	}
}

// Parse extracts fragments from unified-diff output. Each `@@` hunk becomes
// one Fragment. The walker tracks the current new-file line counter through
// each hunk's body so every emitted Line gets the correct 1-based row in
// the new file.
//
// Inputs:
//   - diff: the raw `git diff` output (any number of files)
//   - root: optional repo root used to absolutize the b/<path> the diff
//     emits; pass "" to keep paths as-is.
//
// Pure-deletion hunks (oldCount > 0, newCount == 0) produce no Fragment
// because there's nothing in the new file to anchor on. The gitgutter
// package surfaces those via a DeletedAbove marker on the line below.
func Parse(diff []byte, root string) ([]Fragment, error) {
	var frags []Fragment
	var newPath string

	lines := bytes.Split(diff, []byte("\n"))
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		switch {
		case bytes.HasPrefix(l, []byte("diff --git ")):
			newPath = ""
			continue
		case bytes.HasPrefix(l, []byte("+++ b/")):
			newPath = string(l[len("+++ b/"):])
			continue
		case bytes.HasPrefix(l, []byte("+++ ")):
			newPath = ""
			continue
		}
		if !bytes.HasPrefix(l, []byte("@@")) {
			continue
		}
		newStart, _, suffix, ok := parseHunkHeader(l)
		if !ok || newPath == "" {
			continue
		}
		var body []Line
		cur := newStart
		for i+1 < len(lines) {
			next := lines[i+1]
			if bytes.HasPrefix(next, []byte("@@")) ||
				bytes.HasPrefix(next, []byte("diff --git ")) {
				break
			}
			i++
			if len(next) == 0 {
				continue
			}
			mark := next[0]
			text := string(next[1:])
			switch mark {
			case ' ':
				body = append(body, Line{Marker: Context, FileLine: cur, Text: text})
				cur++
			case '+':
				body = append(body, Line{Marker: Added, FileLine: cur, Text: text})
				cur++
			case '-':
				// removed line — not in the new file, skip.
			case '\\':
				// "\ No newline at end of file" — diff metadata, ignore.
			}
		}
		if len(body) == 0 {
			continue
		}
		// Diff paths are POSIX. Use `path` (not `path/filepath`) so the
		// joined absolute stays POSIX on Windows; filepath.Join inserts
		// backslashes and filepath.IsAbs("/x") is false without a drive.
		abs := newPath
		if !path.IsAbs(abs) && root != "" {
			abs = path.Join(filepath.ToSlash(root), newPath)
		}
		frags = append(frags, Fragment{
			Path:      abs,
			StartLine: body[0].FileLine,
			EndLine:   body[len(body)-1].FileLine,
			Lines:     body,
			Suffix:    suffix,
		})
	}
	return frags, nil
}

// parseHunkHeader extracts (newStart, newCount, suffix, ok) from a line of
// the form "@@ -A,B +C,D @@ <suffix>". oldStart/oldCount are intentionally
// discarded — the parser only needs new-file coordinates.
func parseHunkHeader(line []byte) (newStart, newCount int, suffix string, ok bool) {
	s := string(line)
	if !strings.HasPrefix(s, "@@ ") {
		return 0, 0, "", false
	}
	rest := s[3:]
	end := strings.Index(rest, " @@")
	if end < 0 {
		return 0, 0, "", false
	}
	body := rest[:end]
	if end+3 < len(rest) {
		suffix = strings.TrimSpace(rest[end+3:])
	}
	parts := strings.Fields(body)
	if len(parts) < 2 || !strings.HasPrefix(parts[1], "+") {
		return 0, 0, "", false
	}
	rng := parts[1][1:]
	if i := strings.Index(rng, ","); i >= 0 {
		n, err := strconv.Atoi(rng[:i])
		if err != nil {
			return 0, 0, "", false
		}
		m, err := strconv.Atoi(rng[i+1:])
		if err != nil {
			return 0, 0, "", false
		}
		return n, m, suffix, true
	}
	n, err := strconv.Atoi(rng)
	if err != nil {
		return 0, 0, "", false
	}
	return n, 1, suffix, true
}

func buildRows(frags []Fragment) []row {
	var rows []row
	for i, f := range frags {
		if i > 0 {
			rows = append(rows, row{kind: rowSeparator, fragIdx: -1})
		}
		rows = append(rows, row{kind: rowHeader, fragIdx: i})
		for j := range f.Lines {
			rows = append(rows, row{kind: rowContent, fragIdx: i, lineIdx: j})
		}
	}
	return rows
}

func relativize(p, root string) string {
	if root == "" {
		return p
	}
	// Fragment paths are POSIX (see Parse); normalize root to slashes so the
	// prefix trim stays consistent on Windows where filepath.Rel disagrees.
	r := strings.TrimRight(filepath.ToSlash(root), "/")
	if r == "" {
		return p
	}
	if p == r {
		return "."
	}
	if strings.HasPrefix(p, r+"/") {
		return p[len(r)+1:]
	}
	return p
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if len(s) <= w {
		return s
	}
	if w <= 1 {
		return s[:w]
	}
	return s[:w-1] + "…"
}

func padRight(s string, w int) string {
	if w <= 0 {
		return ""
	}
	n := lipgloss.Width(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

// stripANSI removes lipgloss-injected CSI sequences so a cursor highlight
// can re-style a row uniformly. Without stripping, the inverse-primary
// background would leak the underlying path/loc foreground colors.
//
// We recognize the form ESC `[` <params> <final> where final is in the
// 0x40-0x7E range. Anything else passes through untouched.
func stripANSI(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == 0x1b && i+1 < len(runes) && runes[i+1] == '[' {
			i += 2
			for i < len(runes) {
				r := runes[i]
				if r >= '@' && r <= '~' {
					break
				}
				i++
			}
			continue
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}
