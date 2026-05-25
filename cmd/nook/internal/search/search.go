// Package search runs ripgrep as a subprocess and streams matches back to
// the host model. The Pane renders a scrollable, file-grouped result list
// (multibuffer-shaped) and emits OpenMsg when the user selects a match.
//
// Requires `rg` on PATH. Falls back to "ripgrep not installed" message if
// missing.
package search

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Match is one search hit.
type Match struct {
	Path    string
	Line    int    // 1-based
	Col     int    // 1-based byte column of the first submatch
	Len     int    // byte length of the first submatch on the line
	Snippet string // the matched line, trimmed
}

// MatchMsg is emitted by the streaming Cmd for each result.
type MatchMsg struct{ Match Match }

// DoneMsg is emitted when ripgrep finishes (success or otherwise).
type DoneMsg struct {
	Err     error
	Matches int
}

// OpenMsg is emitted when the user presses Enter on a result.
type OpenMsg struct {
	Path string
	Line int
	Col  int
}

// CancelMsg is emitted when the user presses Esc.
type CancelMsg struct{}

// ApplyMsg is emitted when the user presses Enter while in replace mode.
// The host runs ApplyAll(p.Matches(), msg.Replacement), reloads any
// touched buffers via bufman.RefreshIfOpen, and shows a status summary.
type ApplyMsg struct {
	Replacement string
}

// Run spawns ripgrep and streams matches. Cancel via ctx.
// Returns a function suitable for use as a tea.Cmd.
//
// The returned tea.Cmd reads ripgrep's stdout, emits one MatchMsg per match,
// and a final DoneMsg. To run inside a Bubble Tea program, wrap the channel
// reads into a recursive Cmd or use a goroutine that posts via Program.Send.
//
// For simplicity and testability, this package exposes both Run (channel-
// based) and a Cmd helper (RunCmd) that the host calls.
func Run(ctx context.Context, root, query string) (<-chan Match, <-chan error) {
	out := make(chan Match, 64)
	done := make(chan error, 1)
	if strings.TrimSpace(query) == "" {
		close(out)
		done <- nil
		close(done)
		return out, done
	}
	go func() {
		defer close(out)
		defer close(done)

		args := []string{"--json", "--smart-case", "--column", query, root}
		cmd := exec.CommandContext(ctx, "rg", args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			done <- err
			return
		}
		if err := cmd.Start(); err != nil {
			done <- err
			return
		}
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			var ev rgEvent
			if err := json.Unmarshal(line, &ev); err != nil {
				continue
			}
			if ev.Type != "match" {
				continue
			}
			snippet := strings.TrimRight(ev.Data.Lines.Text, "\n")
			col := 1
			matchLen := 0
			if len(ev.Data.Submatches) > 0 {
				col = ev.Data.Submatches[0].Start + 1
				matchLen = ev.Data.Submatches[0].End - ev.Data.Submatches[0].Start
			}
			select {
			case <-ctx.Done():
				_ = cmd.Process.Kill()
				return
			case out <- Match{
				Path:    ev.Data.Path.Text,
				Line:    ev.Data.LineNumber,
				Col:     col,
				Len:     matchLen,
				Snippet: strings.TrimSpace(snippet),
			}:
			}
		}
		err = cmd.Wait()
		// rg exits with status 1 when no matches; treat as success.
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			err = nil
		}
		done <- err
	}()
	return out, done
}

// rgEvent is a partial decode of rg --json output (only what we need).
type rgEvent struct {
	Type string `json:"type"`
	Data struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		Lines struct {
			Text string `json:"text"`
		} `json:"lines"`
		LineNumber int          `json:"line_number"`
		Submatches []rgSubmatch `json:"submatches"`
	} `json:"data"`
}

type rgSubmatch struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Pane is the search results UI model.
type Pane struct {
	theme       theme.Theme
	root        string
	query       string
	matches     []Match
	cursor      int
	width       int
	height      int
	focused     bool
	running     bool
	err         error
	replacing   bool   // when true, key input edits replacement, not query
	replacement string // current replacement text shown in the replace row
}

// NewPane constructs an empty search pane rooted at root.
func NewPane(t theme.Theme, root string) Pane {
	return Pane{theme: t, root: root, width: 80, height: 20}
}

// WithSize sets pane dimensions.
func (p Pane) WithSize(w, h int) Pane { p.width = w; p.height = h; return p }

// Focused reports whether the pane has keyboard focus.
func (p Pane) Focused() bool { return p.focused }

// Focus sets focused=true.
func (p Pane) Focus() Pane { p.focused = true; return p }

// Blur sets focused=false.
func (p Pane) Blur() Pane { p.focused = false; return p }

// Query returns the current search string.
func (p Pane) Query() string { return p.query }

// Matches returns the current results.
func (p Pane) Matches() []Match { return p.matches }

// Reset clears state and prepares for a new query. Replacing mode and
// replacement text are cleared too — a new search always starts in
// search mode.
func (p Pane) Reset(query string) Pane {
	p.query = query
	p.matches = nil
	p.cursor = 0
	p.running = true
	p.err = nil
	p.replacing = false
	p.replacement = ""
	return p
}

// Replacing reports whether the pane currently routes key input into the
// replacement field. False during normal search-result navigation.
func (p Pane) Replacing() bool { return p.replacing }

// Replacement returns the current replacement text. Empty before the
// user enters replace mode.
func (p Pane) Replacement() string { return p.replacement }

// EnterReplace flips the pane into replace mode. No-op when no matches
// are visible — replacing zero matches is meaningless. Callers route
// Alt+R here when the search pane has focus.
func (p Pane) EnterReplace() Pane {
	if len(p.matches) == 0 || p.running {
		return p
	}
	p.replacing = true
	return p
}

// ExitReplace flips the pane back to result navigation, preserving the
// already-typed replacement so a second Alt+R picks up where the user
// left off.
func (p Pane) ExitReplace() Pane {
	p.replacing = false
	return p
}

// AppendReplacementRune adds one rune to the replacement field. Used by
// the host's key router so the same character-input handling that drives
// the search query also drives the replacement (without duplicating
// rune-collection logic per pane state).
func (p Pane) AppendReplacementRune(s string) Pane {
	p.replacement += s
	return p
}

// BackspaceReplacement drops the last rune from the replacement field.
// Behaves like KeyBackspace in any text input — a no-op on empty input.
func (p Pane) BackspaceReplacement() Pane {
	if p.replacement == "" {
		return p
	}
	r := []rune(p.replacement)
	p.replacement = string(r[:len(r)-1])
	return p
}

// AppendMatch adds one match. Used by the host model after receiving a
// MatchMsg from the streaming Cmd.
func (p Pane) AppendMatch(m Match) Pane {
	p.matches = append(p.matches, m)
	return p
}

// MarkDone marks the search complete.
func (p Pane) MarkDone(err error) Pane {
	p.running = false
	p.err = err
	return p
}

// Selected returns the currently highlighted match (if any).
func (p Pane) Selected() (Match, bool) {
	if p.cursor < 0 || p.cursor >= len(p.matches) {
		return Match{}, false
	}
	return p.matches[p.cursor], true
}

// Update routes a key event. Returns a Cmd carrying OpenMsg, CancelMsg,
// or ApplyMsg depending on the pane's current mode.
//
// In replace mode (p.replacing == true): Enter emits ApplyMsg with the
// current replacement text; Esc collapses back to result navigation
// (without canceling the whole pane — a single Esc never throws away
// the user's typed replacement). Up/Down/PgUp/PgDn still navigate so
// the user can see what they're replacing.
//
// In search mode (default): Enter on a selected match emits OpenMsg;
// Esc emits CancelMsg.
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	if p.replacing {
		switch km.Type {
		case tea.KeyEsc:
			p.replacing = false
			return p, nil
		case tea.KeyEnter:
			return p, func() tea.Msg { return ApplyMsg{Replacement: p.replacement} }
		case tea.KeyUp, tea.KeyCtrlP:
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		case tea.KeyDown, tea.KeyCtrlN:
			if p.cursor < len(p.matches)-1 {
				p.cursor++
			}
			return p, nil
		case tea.KeyPgUp:
			p.cursor -= p.height - 4
			if p.cursor < 0 {
				p.cursor = 0
			}
			return p, nil
		case tea.KeyPgDown:
			p.cursor += p.height - 4
			if p.cursor >= len(p.matches) {
				p.cursor = len(p.matches) - 1
			}
			return p, nil
		}
		return p, nil
	}
	switch km.Type {
	case tea.KeyEsc:
		return p, func() tea.Msg { return CancelMsg{} }
	case tea.KeyEnter:
		if m, ok := p.Selected(); ok {
			return p, func() tea.Msg {
				return OpenMsg{Path: m.Path, Line: m.Line, Col: m.Col}
			}
		}
		return p, nil
	case tea.KeyUp, tea.KeyCtrlP:
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case tea.KeyDown, tea.KeyCtrlN:
		if p.cursor < len(p.matches)-1 {
			p.cursor++
		}
		return p, nil
	case tea.KeyPgUp:
		p.cursor -= p.height - 4
		if p.cursor < 0 {
			p.cursor = 0
		}
		return p, nil
	case tea.KeyPgDown:
		p.cursor += p.height - 4
		if p.cursor >= len(p.matches) {
			p.cursor = len(p.matches) - 1
		}
		return p, nil
	}
	return p, nil
}

// View renders the pane.
func (p Pane) View() string {
	t := p.theme
	titleStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Bold(true)
	muted := lipgloss.NewStyle().Foreground(t.TextMuted)
	file := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	location := lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true)
	cursor := lipgloss.NewStyle().Foreground(t.TextInverse).Background(t.Primary).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(t.Error)

	header := titleStyle.Render("search")
	if p.query != "" {
		header += "  " + muted.Render("›") + " " + lipgloss.NewStyle().Foreground(t.Text).Render(p.query)
	}
	header += "  " + muted.Render(formatStatus(p))

	// In replace mode a second prompt row appears between the header
	// and the result list. The caret is a thin underscore so it reads
	// as "ready for input" without stealing visual weight from the
	// result list. Replaces one body row when shown.
	bodyH := p.height - 2
	var replaceRow string
	if p.replacing {
		caret := lipgloss.NewStyle().Foreground(t.Primary).Render("▎")
		label := lipgloss.NewStyle().Foreground(t.TextMuted).Bold(true).Render("replace")
		value := lipgloss.NewStyle().Foreground(t.Text).Render(p.replacement)
		hint := lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render(
			"  enter applies to " + formatInt(len(p.matches)) + " match(es) • esc back to results",
		)
		replaceRow = caret + " " + label + "  " + muted.Render("›") + " " + value + hint
		bodyH--
	}
	if bodyH < 1 {
		bodyH = 1
	}
	start := 0
	if p.cursor >= bodyH {
		start = p.cursor - bodyH + 1
	}
	end := start + bodyH
	if end > len(p.matches) {
		end = len(p.matches)
	}

	var rows []string
	lastFile := ""
	for i := start; i < end; i++ {
		m := p.matches[i]
		rel := relativize(m.Path, p.root)
		if rel != lastFile {
			lastFile = rel
			rows = append(rows, file.Render(rel))
			bodyH--
			if bodyH <= 0 {
				break
			}
		}
		loc := location.Render(formatInt(m.Line) + ":" + formatInt(m.Col))
		body := truncate(m.Snippet, p.width-len(loc)-4)
		line := loc + "  " + body
		if i == p.cursor {
			line = cursor.Render(padRight(stripPrefix(line), p.width-2))
		} else {
			line = padRight(line, p.width-2)
		}
		rows = append(rows, line)
	}
	padTarget := p.height - 2
	if p.replacing {
		padTarget--
	}
	for len(rows) < padTarget {
		rows = append(rows, padRight("", p.width-2))
	}

	footer := ""
	if p.err != nil {
		footer = errStyle.Render("error: " + p.err.Error())
	}

	parts := []string{header}
	if replaceRow != "" {
		parts = append(parts, replaceRow)
	}
	parts = append(parts, rows...)
	if footer != "" {
		parts = append(parts, footer)
	}
	return strings.Join(parts, "\n")
}

// --- helpers ---

func formatStatus(p Pane) string {
	switch {
	case p.running:
		return formatInt(len(p.matches)) + " matches • searching…"
	case p.query == "":
		return "type a query and press Enter"
	case p.err != nil:
		return "error"
	default:
		return formatInt(len(p.matches)) + " matches"
	}
}

func relativize(path, root string) string {
	if root == "" {
		return path
	}
	root = strings.TrimRight(root, "/")
	if strings.HasPrefix(path, root+"/") {
		return path[len(root)+1:]
	}
	return path
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return string(r[:w])
	}
	return string(r[:w-1]) + "…"
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

func stripPrefix(s string) string { return s }

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + formatInt(-n)
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
