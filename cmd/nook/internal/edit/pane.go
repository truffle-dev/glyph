// Package edit implements Cmd-K-style inline AI edits. The pane wraps a
// short prompt for the model and streams the result back as a diff preview.
// One pane edits one line at a time; selection ranges are a follow-up.
package edit

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/cmd/nook/internal/ai"
	"github.com/truffle-dev/glyph/cmd/nook/internal/airules"
	"github.com/truffle-dev/glyph/components/theme"
)

// State enumerates the four shapes the pane can take.
type State int

const (
	StateComposing State = iota // user is typing an instruction
	StateStreaming              // model is streaming back
	StateReview                 // accept/reject the proposed edit
	StateError                  // surface an error and let user retry/cancel
)

// AcceptMsg is emitted when the user accepts the proposed edit. Host inserts
// NewText at the original line.
type AcceptMsg struct {
	Path    string
	Line    int    // 0-based line number to replace
	NewText string // proposed replacement line
}

// CancelMsg means the user dismissed the pane without applying.
type CancelMsg struct{}

// SubmittedMsg is fired internally when the user presses Enter from
// StateComposing. The host doesn't need to react to this.
type SubmittedMsg struct{}

// streamDeltaMsg arrives once per token from the model.
type streamDeltaMsg struct {
	text string
	more <-chan string
	done <-chan error
}

// streamDoneMsg fires after the stream's done channel emits.
type streamDoneMsg struct {
	err error
}

// Pane is the immutable model for the inline edit overlay.
type Pane struct {
	theme  theme.Theme
	client *ai.Client

	path     string
	line     int
	original string

	state    State
	prompt   string // user instruction
	proposed string // streamed model output
	errMsg   string

	width  int
	height int

	// rules is the trimmed contents of the workspace's .nookrules /
	// .cursorrules file, or "" when neither is present. Appended to the
	// system prompt at every startStream call.
	rules string

	cancel context.CancelFunc
}

// NewPane constructs an empty pane. The client may be nil; in that case the
// pane shows a status message instead of streaming.
func NewPane(t theme.Theme, client *ai.Client) Pane {
	return Pane{
		theme:  t,
		client: client,
		state:  StateComposing,
		width:  60,
		height: 10,
	}
}

// Open targets a (path, line) pair. The pane resets prompt+proposed and
// returns to StateComposing.
func (p Pane) Open(path string, line int, original string) Pane {
	p.path = path
	p.line = line
	p.original = original
	p.state = StateComposing
	p.prompt = ""
	p.proposed = ""
	p.errMsg = ""
	if p.cancel != nil {
		p.cancel()
	}
	p.cancel = nil
	return p
}

// WithRules binds repo-level AI conventions (.nookrules / .cursorrules
// content) to the pane. The rules are folded into the system prompt on
// every subsequent startStream. Empty rules is a no-op at call time;
// the pane behaves exactly as if no file were present.
func (p Pane) WithRules(rules string) Pane {
	p.rules = rules
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

// State reports the current state. Useful in tests.
func (p Pane) State() State { return p.state }

// Proposed returns the current proposed text (may be partial during streaming).
func (p Pane) Proposed() string { return p.proposed }

// Update handles keystrokes and stream messages.
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		return p.handleKey(m)
	case streamDeltaMsg:
		p.proposed += m.text
		return p, pumpStreamCmd(m.more, m.done)
	case streamDoneMsg:
		if m.err != nil {
			p.errMsg = m.err.Error()
			p.state = StateError
			return p, nil
		}
		p.state = StateReview
		// Strip any trailing newline so the apply step is exactly one line.
		p.proposed = strings.TrimRight(p.proposed, "\n")
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
		case tea.KeyEnter:
			return p, emit(AcceptMsg{Path: p.path, Line: p.line, NewText: p.proposed})
		case tea.KeyRunes:
			if len(m.Runes) > 0 && (m.Runes[0] == 'r' || m.Runes[0] == 'R') {
				return p.retry()
			}
		}
	case StateError:
		switch m.Type {
		case tea.KeyEsc:
			return p, emit(CancelMsg{})
		case tea.KeyRunes:
			if len(m.Runes) > 0 && (m.Runes[0] == 'r' || m.Runes[0] == 'R') {
				return p.retry()
			}
		}
	}
	return p, nil
}

func (p Pane) retry() (Pane, tea.Cmd) {
	p.proposed = ""
	p.errMsg = ""
	p.state = StateComposing
	return p, nil
}

func (p Pane) startStream() (Pane, tea.Cmd) {
	if p.client == nil {
		p.errMsg = "claude CLI not found on PATH"
		p.state = StateError
		return p, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.state = StateStreaming

	req := ai.Request{
		Tier:   ai.Fast,
		System: airules.AugmentSystemPrompt(systemPrompt, p.rules),
		User:   buildUserPrompt(p.path, p.line, p.original, p.prompt),
		// Fence the output. We strip the fence before applying.
		StopSequences: []string{"\n```", "```\n"},
	}
	deltas, done := p.client.Stream(ctx, req)
	return p, pumpStreamCmd(deltas, done)
}

// pumpStreamCmd reads one delta and reschedules itself. When the deltas
// channel closes, it switches to reading the done channel and emits
// streamDoneMsg.
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

// View renders the floating overlay.
func (p Pane) View() string {
	t := p.theme

	headerStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	errStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	removed := lipgloss.NewStyle().Foreground(t.Error)
	added := lipgloss.NewStyle().Foreground(t.Success)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1).
		Width(p.width)

	var body strings.Builder
	header := fmt.Sprintf("edit %s:%d", relPathHint(p.path), p.line+1)
	body.WriteString(headerStyle.Render(header))
	body.WriteString("\n")

	switch p.state {
	case StateComposing:
		body.WriteString(mutedStyle.Render("instruction"))
		body.WriteString("\n")
		body.WriteString("> " + p.prompt + cursorBlock(t))
		body.WriteString("\n\n")
		body.WriteString(mutedStyle.Render(fmt.Sprintf("line: %s", trimTo(p.original, p.width-4))))
		body.WriteString("\n")
		body.WriteString(mutedStyle.Render("enter submit • esc cancel"))
	case StateStreaming:
		body.WriteString(mutedStyle.Render("streaming…"))
		body.WriteString("\n")
		body.WriteString(removed.Render("- " + trimTo(p.original, p.width-4)))
		body.WriteString("\n")
		body.WriteString(added.Render("+ " + trimTo(p.proposed, p.width-4) + cursorBlock(t)))
		body.WriteString("\n\n")
		body.WriteString(mutedStyle.Render("esc cancel"))
	case StateReview:
		body.WriteString(mutedStyle.Render("review"))
		body.WriteString("\n")
		body.WriteString(removed.Render("- " + trimTo(p.original, p.width-4)))
		body.WriteString("\n")
		body.WriteString(added.Render("+ " + trimTo(p.proposed, p.width-4)))
		body.WriteString("\n\n")
		body.WriteString(mutedStyle.Render("enter apply • r retry • esc cancel"))
	case StateError:
		body.WriteString(errStyle.Render("error"))
		body.WriteString("\n")
		body.WriteString(p.errMsg)
		body.WriteString("\n\n")
		body.WriteString(mutedStyle.Render("r retry • esc cancel"))
	}

	return borderStyle.Render(body.String())
}

func cursorBlock(t theme.Theme) string {
	return lipgloss.NewStyle().
		Background(t.Text).
		Foreground(t.Bg).
		Render(" ")
}

func trimTo(s string, max int) string {
	if max < 4 {
		max = 4
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func relPathHint(p string) string {
	// Avoid noisy absolute paths in the header.
	idx := strings.LastIndex(p, "/")
	if idx == -1 {
		return p
	}
	return p[idx+1:]
}

const systemPrompt = `You are a code editing assistant inside a terminal IDE.
When asked to edit a single line of code, return ONLY the replacement line.
- No prose, no commentary, no preamble.
- Preserve the original indentation unless the instruction says otherwise.
- Output a single line. If the instruction requires multiple lines, still output them but no extra explanation.
- Do not wrap your answer in markdown fences.`

func buildUserPrompt(path string, line int, original, instruction string) string {
	return fmt.Sprintf(
		"File: %s\nLine %d (1-based):\n%s\n\nInstruction: %s\n\nReturn ONLY the replacement line.",
		path, line+1, original, instruction,
	)
}
