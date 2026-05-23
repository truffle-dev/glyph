// Package composer is the multi-file Cursor-Composer-style side panel.
// The model is given a user instruction, the file index, and (optionally)
// the currently-open file's contents. It streams back a sequence of file
// blocks. Each block is a full replacement so applying is deterministic.
package composer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/cmd/nook/internal/ai"
	"github.com/truffle-dev/glyph/components/theme"
)

// State enumerates the four shapes the pane can take.
type State int

const (
	StateComposing State = iota // user is typing an instruction
	StateStreaming              // model is streaming back
	StateReview                 // user reviews the proposed edits
	StateError                  // a transient error is visible
)

// Edit describes one proposed file edit.
type Edit struct {
	Path     string // repo-relative
	Original string // original full contents (empty for new files)
	Proposed string // full proposed contents
	Applied  bool
	Rejected bool
}

// ApplyMsg fires when the user accepts a single Edit. The host writes it.
type ApplyMsg struct {
	Edit Edit
}

// ApplyAllMsg fires when the user accepts every pending Edit.
type ApplyAllMsg struct {
	Edits []Edit
}

// CancelMsg means the user dismissed the composer.
type CancelMsg struct{}

// streamDeltaMsg arrives once per token from the model.
type streamDeltaMsg struct {
	text string
	more <-chan string
	done <-chan error
}

type streamDoneMsg struct {
	err error
}

// Context is the user-facing summary of what we're sending to the model.
// The host populates it before invoking the composer.
type Context struct {
	Root         string   // repo root for relative paths
	OpenPath     string   // currently-focused file in the editor (optional)
	OpenContents string   // its contents (optional)
	Files        []string // relative paths in the workspace
}

// Pane is the immutable composer model.
type Pane struct {
	theme  theme.Theme
	client *ai.Client
	ctx    Context

	state    State
	prompt   string
	buffer   string // raw stream buffer (used for incremental parsing)
	edits    []Edit
	cursor   int // selected edit in review state
	errMsg   string
	width    int
	height   int
	focused  bool
	statusOn string // banner message ("apply…", "error: …")

	cancel context.CancelFunc
}

// NewPane builds an empty composer.
func NewPane(t theme.Theme, client *ai.Client) Pane {
	return Pane{
		theme:  t,
		client: client,
		state:  StateComposing,
		width:  40,
		height: 24,
	}
}

// WithContext refreshes the model's grounding.
func (p Pane) WithContext(c Context) Pane {
	p.ctx = c
	return p
}

// WithSize updates layout dimensions.
func (p Pane) WithSize(w, h int) Pane {
	if w > 0 {
		p.width = w
	}
	if h > 0 {
		p.height = h
	}
	return p
}

// Focus marks the pane as receiving keystrokes.
func (p Pane) Focus() Pane { p.focused = true; return p }

// Blur clears focus.
func (p Pane) Blur() Pane { p.focused = false; return p }

// Focused reports focus.
func (p Pane) Focused() bool { return p.focused }

// State reports the current state.
func (p Pane) State() State { return p.state }

// Edits returns the current list.
func (p Pane) Edits() []Edit { return p.edits }

// Reset wipes prompt + edits, transitions to StateComposing.
func (p Pane) Reset() Pane {
	p.prompt = ""
	p.buffer = ""
	p.edits = nil
	p.cursor = 0
	p.errMsg = ""
	p.state = StateComposing
	if p.cancel != nil {
		p.cancel()
	}
	p.cancel = nil
	return p
}

// Update handles keys and stream messages.
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		if !p.focused {
			return p, nil
		}
		return p.handleKey(m)
	case streamDeltaMsg:
		p.buffer += m.text
		p.edits = parseEdits(p.buffer, p.edits)
		return p, pumpStreamCmd(m.more, m.done)
	case streamDoneMsg:
		if m.err != nil {
			p.errMsg = m.err.Error()
			p.state = StateError
			return p, nil
		}
		// Finalize any open edit.
		p.edits = parseEdits(p.buffer, p.edits)
		p.edits = finalizeEdits(p.edits)
		p.state = StateReview
		p.cursor = 0
		return p, nil
	}
	return p, nil
}

func (p Pane) handleKey(m tea.KeyMsg) (Pane, tea.Cmd) {
	switch p.state {
	case StateComposing:
		switch m.Type {
		case tea.KeyEsc:
			return p, emit(CancelMsg{})
		case tea.KeyEnter:
			if strings.TrimSpace(p.prompt) == "" {
				return p, nil
			}
			return p.startStream()
		case tea.KeyBackspace:
			if len(p.prompt) > 0 {
				p.prompt = p.prompt[:len(p.prompt)-1]
			}
			return p, nil
		case tea.KeyRunes, tea.KeySpace:
			p.prompt += string(m.Runes)
			if m.Type == tea.KeySpace && len(m.Runes) == 0 {
				p.prompt += " "
			}
			return p, nil
		}
	case StateStreaming:
		if m.Type == tea.KeyEsc {
			if p.cancel != nil {
				p.cancel()
				p.cancel = nil
			}
			return p, emit(CancelMsg{})
		}
	case StateReview:
		switch m.Type {
		case tea.KeyEsc:
			return p, emit(CancelMsg{})
		case tea.KeyUp:
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		case tea.KeyDown:
			if p.cursor < len(p.edits)-1 {
				p.cursor++
			}
			return p, nil
		case tea.KeyEnter:
			if len(p.edits) == 0 {
				return p, emit(CancelMsg{})
			}
			edit := p.edits[p.cursor]
			edit.Applied = true
			p.edits[p.cursor] = edit
			return p, emit(ApplyMsg{Edit: edit})
		case tea.KeyRunes:
			switch m.Runes[0] {
			case 'a', 'A':
				return p.applyAll()
			case 'r', 'R':
				return p.Reset(), nil
			case 'x', 'X':
				if p.cursor < len(p.edits) {
					e := p.edits[p.cursor]
					e.Rejected = true
					p.edits[p.cursor] = e
				}
				return p, nil
			}
		}
	case StateError:
		switch m.Type {
		case tea.KeyEsc:
			return p, emit(CancelMsg{})
		case tea.KeyRunes:
			if len(m.Runes) > 0 && (m.Runes[0] == 'r' || m.Runes[0] == 'R') {
				return p.Reset(), nil
			}
		}
	}
	return p, nil
}

func (p Pane) applyAll() (Pane, tea.Cmd) {
	pending := make([]Edit, 0, len(p.edits))
	for _, e := range p.edits {
		if e.Applied || e.Rejected {
			continue
		}
		pending = append(pending, e)
	}
	return p, emit(ApplyAllMsg{Edits: pending})
}

func (p Pane) startStream() (Pane, tea.Cmd) {
	if p.client == nil {
		p.errMsg = "ANTHROPIC_API_KEY not set"
		p.state = StateError
		return p, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.state = StateStreaming
	p.buffer = ""
	p.edits = nil

	req := ai.Request{
		Tier:   ai.Smart,
		System: systemPrompt,
		User:   buildUserPrompt(p.ctx, p.prompt),
	}
	deltas, done := p.client.Stream(ctx, req)
	return p, pumpStreamCmd(deltas, done)
}

func pumpStreamCmd(deltas <-chan string, done <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case d, ok := <-deltas:
			if !ok {
				err := <-done
				return streamDoneMsg{err: err}
			}
			return streamDeltaMsg{text: d, more: deltas, done: done}
		}
	}
}

func emit(m tea.Msg) tea.Cmd {
	return func() tea.Msg { return m }
}

// View renders the panel.
func (p Pane) View() string {
	t := p.theme
	headerStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	errStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)

	var body strings.Builder
	body.WriteString(headerStyle.Render("composer"))
	body.WriteString(" ")
	body.WriteString(mutedStyle.Render(modelLabel(p.state)))
	body.WriteString("\n")

	switch p.state {
	case StateComposing:
		body.WriteString(mutedStyle.Render("multi-file instruction"))
		body.WriteString("\n")
		body.WriteString("> " + p.prompt + cursorBlock(t))
		body.WriteString("\n\n")
		body.WriteString(mutedStyle.Render(fmt.Sprintf("workspace: %d files", len(p.ctx.Files))))
		if p.ctx.OpenPath != "" {
			body.WriteString("\n")
			body.WriteString(mutedStyle.Render("open: " + filepath.Base(p.ctx.OpenPath)))
		}
		body.WriteString("\n\n")
		body.WriteString(mutedStyle.Render("enter submit • esc cancel"))
	case StateStreaming:
		body.WriteString(mutedStyle.Render("streaming…"))
		body.WriteString("\n")
		for _, e := range p.edits {
			body.WriteString(renderEditRow(t, e, false))
			body.WriteString("\n")
		}
		body.WriteString("\n")
		body.WriteString(mutedStyle.Render("esc cancel"))
	case StateReview:
		body.WriteString(mutedStyle.Render(fmt.Sprintf("%d edit(s)", len(p.edits))))
		body.WriteString("\n")
		for i, e := range p.edits {
			body.WriteString(renderEditRow(t, e, i == p.cursor))
			body.WriteString("\n")
		}
		body.WriteString("\n")
		body.WriteString(mutedStyle.Render("enter apply • a apply all • x reject • r retry • esc cancel"))
	case StateError:
		body.WriteString(errStyle.Render("error"))
		body.WriteString("\n")
		body.WriteString(p.errMsg)
		body.WriteString("\n\n")
		body.WriteString(mutedStyle.Render("r retry • esc cancel"))
	}

	if p.statusOn != "" {
		body.WriteString("\n")
		body.WriteString(mutedStyle.Render(p.statusOn))
	}

	return lipgloss.NewStyle().Width(p.width).Render(body.String())
}

func renderEditRow(t theme.Theme, e Edit, selected bool) string {
	prefix := "  "
	if selected {
		prefix = "› "
	}
	marker := "  "
	switch {
	case e.Applied:
		marker = "✓ "
	case e.Rejected:
		marker = "✗ "
	}
	stat := fmt.Sprintf("(%d lines)", strings.Count(strings.TrimRight(e.Proposed, "\n"), "\n")+1)
	style := lipgloss.NewStyle().Foreground(t.Text)
	if selected {
		style = style.Foreground(t.Primary).Bold(true)
	}
	if e.Applied {
		style = style.Foreground(t.Success)
	}
	if e.Rejected {
		style = style.Foreground(t.TextMuted)
	}
	return style.Render(prefix + marker + e.Path + " " + stat)
}

func cursorBlock(t theme.Theme) string {
	return lipgloss.NewStyle().Background(t.Text).Foreground(t.Bg).Render(" ")
}

func modelLabel(s State) string {
	switch s {
	case StateStreaming:
		return "sonnet streaming"
	case StateReview:
		return "review"
	case StateError:
		return "error"
	}
	return "sonnet ready"
}

// parseEdits scans `text` and produces (in order) the file blocks it can
// fully recognize so far. The fenced format we ask the model to produce is:
//
//	=== path/to/file.ext ===
//	```
//	<contents>
//	```
//
// Incomplete trailing blocks are emitted with whatever contents the stream
// has produced so far so the UI can show progress. previous is reused to
// preserve Applied/Rejected flags across reparses.
func parseEdits(text string, previous []Edit) []Edit {
	idx := func(s string) []int { return findAll(text, s) }
	starts := idx("\n=== ")
	if strings.HasPrefix(text, "=== ") {
		starts = append([]int{-1}, starts...)
	}

	var edits []Edit
	for i, s := range starts {
		// Each block starts at s+1 (after the leading newline) or at 0 if
		// the very first line is a marker.
		headStart := s + 1
		if s == -1 {
			headStart = 0
		}
		// Find the end of the header line.
		nl := strings.Index(text[headStart:], "\n")
		if nl == -1 {
			continue
		}
		headLine := text[headStart : headStart+nl]
		path := extractPath(headLine)
		if path == "" {
			continue
		}
		bodyStart := headStart + nl + 1

		var bodyEnd int
		if i+1 < len(starts) {
			bodyEnd = starts[i+1]
		} else {
			bodyEnd = len(text)
		}
		body := text[bodyStart:bodyEnd]
		body = stripFences(body)

		ed := Edit{Path: path, Proposed: body}
		if existing := findEdit(previous, path); existing != nil {
			ed.Applied = existing.Applied
			ed.Rejected = existing.Rejected
			ed.Original = existing.Original
		}
		edits = append(edits, ed)
	}
	return edits
}

// finalizeEdits drops any edit whose body contains an obviously-unfinished
// fence (a leading ``` without trailing fence). For nook v1 we keep all
// edits and just trim — the renderer can still display partial bodies.
func finalizeEdits(in []Edit) []Edit {
	out := make([]Edit, 0, len(in))
	for _, e := range in {
		e.Proposed = strings.TrimRight(e.Proposed, "\n")
		if e.Proposed == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

func findAll(text, sep string) []int {
	var out []int
	for i := 0; ; {
		j := strings.Index(text[i:], sep)
		if j == -1 {
			return out
		}
		out = append(out, i+j)
		i = i + j + len(sep)
	}
}

func extractPath(line string) string {
	// Form: "=== path ==="  (with optional trailing whitespace)
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "=== ") {
		return ""
	}
	s = strings.TrimPrefix(s, "=== ")
	s = strings.TrimSuffix(s, "===")
	s = strings.TrimSpace(s)
	return s
}

func stripFences(body string) string {
	// Drop a leading ```lang and a trailing ```.
	lines := strings.Split(body, "\n")
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		lines = lines[1:]
	}
	// Cut at the next standalone fence (handles incremental parses).
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "```" {
			return strings.Join(lines[:i], "\n")
		}
	}
	return strings.Join(lines, "\n")
}

func findEdit(edits []Edit, path string) *Edit {
	for i := range edits {
		if edits[i].Path == path {
			return &edits[i]
		}
	}
	return nil
}

const systemPrompt = `You are a precise multi-file code editor inside a terminal IDE.

When asked to make changes:
- Reply with ONE OR MORE file edit blocks, each in this exact format:

=== relative/path/to/file.ext ===
` + "```" + `
<full proposed file contents>
` + "```" + `

- Use ONLY this format. No prose, no commentary outside the blocks.
- Each block is a full file replacement. Preserve existing imports and indentation unless the change requires otherwise.
- Use repo-relative paths. Do NOT include leading "/".
- If you need to create a new file, use the same format.
- After all edit blocks, stop. Do not add a summary.`

func buildUserPrompt(ctx Context, instruction string) string {
	var b strings.Builder
	if ctx.OpenPath != "" {
		fmt.Fprintf(&b, "Currently open: %s\n\n", ctx.OpenPath)
		fmt.Fprintf(&b, "Contents:\n```\n%s\n```\n\n", ctx.OpenContents)
	}
	if len(ctx.Files) > 0 {
		const cap = 64
		files := ctx.Files
		if len(files) > cap {
			files = files[:cap]
		}
		b.WriteString("Workspace files (truncated):\n")
		for _, f := range files {
			b.WriteString("- ")
			b.WriteString(f)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Instruction: ")
	b.WriteString(instruction)
	b.WriteString("\n")
	return b.String()
}
