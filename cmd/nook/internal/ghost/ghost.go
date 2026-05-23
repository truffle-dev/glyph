// Package ghost owns nook's inline autocomplete state: idle-trigger debouncing,
// AI request lifecycle, and the rendered "ghost" text that floats after the
// cursor until accepted or dismissed.
//
// The host calls Tick on every key event with the editor's path + row + col +
// the line prefix and suffix. Tick decides whether to:
//
//   - cancel an in-flight proposal (cursor moved off the request site, or the
//     prefix changed),
//   - schedule a new debounced request (idle, prefix length >= MinPrefix,
//     no current proposal at this site).
//
// When a proposal arrives the host receives a SuggestMsg; rendering is the
// host's job (it paints muted dim text after the cursor).
package ghost

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/cmd/nook/internal/ai"
)

// DemoEnvVar is the env var that, when set, replaces every ghost-text AI
// request with the literal value. This lets demos and screencasts show the
// completion flow without spending real API tokens. Multiple proposals are
// pipe-separated (|) so individual proposals can freely contain commas.
const DemoEnvVar = "NOOK_GHOST_DEMO"

// demoSeparator is the field separator for NOOK_GHOST_DEMO. We chose | over
// comma because completions commonly contain commas.
const demoSeparator = "|"

// DebounceDelay is how long the editor must be idle before we request a
// completion. 400ms feels like cursor-cursor: not so fast that every keystroke
// burns a call, not so slow that it feels laggy.
const DebounceDelay = 400 * time.Millisecond

// MinPrefix is the smallest line-prefix length that earns a suggestion. Shorter
// prefixes have too little context and the model proposes generic boilerplate.
const MinPrefix = 2

// SuggestMsg carries a proposal from the AI back to the host.
//
// Site fields uniquely identify the location the proposal targets so the host
// can drop the proposal if the cursor moved.
type SuggestMsg struct {
	Site Site
	Text string // line continuation. May be empty if model refused.
	Err  error
}

// debounceMsg fires after the idle delay; the host returns it unchanged to
// Update which then issues the actual request.
type debounceMsg struct{ generation uint64 }

// Site uniquely names a request location.
type Site struct {
	Path string
	Row  int
	Col  int
	// Prefix is the line text up to Col; we use it as a tie-breaker so a
	// proposal made for "fmt.Pri" doesn't accidentally apply to "fmt.Pin".
	Prefix string
}

// Manager runs the autocomplete loop.
type Manager struct {
	client *ai.Client

	enabled bool

	// demoProposals, when non-empty, replaces every AI call with one of these
	// strings (cycled). Honored when NOOK_GHOST_DEMO is set at construction.
	demoProposals []string
	demoCursor    int

	// generation increments on every Tick. The debounce timer and the in-flight
	// request both carry the generation they were started with; a stale
	// generation means we move on.
	generation uint64

	// currentSite is the site the latest active proposal (or in-flight request)
	// belongs to. Zero value when nothing is in flight.
	currentSite Site

	// proposal is the current ghost text, if any. Cleared on cursor move or
	// accept.
	proposal string

	// inflight is set while a request is running so we can cancel it.
	inflight context.CancelFunc

	// totals — small counters useful for tests and the status bar.
	requested int64
	completed int64
}

// NewManager constructs a Manager.
//
// If client is nil the manager is disabled and Tick/Accept/etc. are no-ops.
// This lets nook keep ghost-text behavior wired in without crashing when the
// claude CLI is missing.
//
// If the NOOK_GHOST_DEMO env var is set, the manager is enabled regardless of
// the client (demo path); each debounce fire emits the configured demo text
// instead of calling the real AI. This is how screencasts and the PTY snapshot
// tour render ghost-text without spawning claude.
func NewManager(client *ai.Client) *Manager {
	demo := os.Getenv(DemoEnvVar)
	if demo != "" {
		// Pipe-separated proposals cycle through requests so a multi-step demo
		// can show distinct completions on each idle.
		parts := strings.Split(demo, demoSeparator)
		clean := parts[:0]
		for _, p := range parts {
			s := strings.TrimSpace(p)
			if s != "" {
				clean = append(clean, s)
			}
		}
		if len(clean) > 0 {
			return &Manager{client: client, enabled: true, demoProposals: clean}
		}
	}
	return &Manager{client: client, enabled: client != nil}
}

// Enabled reports whether ghost-text is wired (client present).
func (m *Manager) Enabled() bool {
	if m == nil {
		return false
	}
	return m.enabled
}

// Proposal returns the current ghost-text proposal, or "" if none.
func (m *Manager) Proposal() string {
	if m == nil {
		return ""
	}
	return m.proposal
}

// CurrentSite returns the site of the current proposal/in-flight request.
func (m *Manager) CurrentSite() Site {
	if m == nil {
		return Site{}
	}
	return m.currentSite
}

// Counters returns (requested, completed) for diagnostic surfaces.
func (m *Manager) Counters() (int64, int64) {
	if m == nil {
		return 0, 0
	}
	return atomic.LoadInt64(&m.requested), atomic.LoadInt64(&m.completed)
}

// Tick reports a key/state change at the given Site. It returns a tea.Cmd that
// schedules the next step:
//   - cancel a stale in-flight request
//   - schedule a debounced request if the site is fresh and AI is enabled
//
// The host calls Tick after applying its own editor update.
//
// Idle is true when the host wants to suppress new requests (e.g. inside a
// menu/overlay). Suppress is true when the host explicitly does not want a
// completion at this site (e.g. cursor inside a comment, future heuristic).
func (m *Manager) Tick(site Site, idle, suppress bool) tea.Cmd {
	if m == nil || !m.enabled {
		return nil
	}

	// Site changed: drop existing proposal and cancel in-flight.
	if site != m.currentSite {
		m.cancelInflight()
		m.proposal = ""
		m.currentSite = site
	}

	if idle || suppress {
		m.cancelInflight()
		m.proposal = ""
		return nil
	}

	if len(strings.TrimSpace(site.Prefix)) < MinPrefix {
		// Not enough context.
		m.proposal = ""
		return nil
	}

	m.generation++
	gen := m.generation
	return tea.Tick(DebounceDelay, func(time.Time) tea.Msg {
		return debounceMsg{generation: gen}
	})
}

// Accept consumes the current proposal and returns the text the host should
// insert. The proposal is cleared so subsequent calls return "".
func (m *Manager) Accept() string {
	if m == nil {
		return ""
	}
	t := m.proposal
	m.proposal = ""
	return t
}

// Dismiss clears the current proposal without applying it.
func (m *Manager) Dismiss() {
	if m == nil {
		return
	}
	m.cancelInflight()
	m.proposal = ""
}

// Update handles ghost-internal messages. The host forwards anything it
// doesn't recognize to the manager via this function.
func (m *Manager) Update(msg tea.Msg) tea.Cmd {
	if m == nil || !m.enabled {
		return nil
	}
	switch v := msg.(type) {
	case debounceMsg:
		if v.generation != m.generation {
			// Stale: the user kept typing or moved. Drop.
			return nil
		}
		return m.request(m.currentSite)
	case SuggestMsg:
		atomic.AddInt64(&m.completed, 1)
		if v.Err != nil {
			// Silent failure: ghost-text is unobtrusive by design. Status bar
			// is the host's job if it wants to surface anything.
			return nil
		}
		// Only accept if the site is still the same (cursor didn't move,
		// nothing else interrupted us).
		if v.Site != m.currentSite {
			return nil
		}
		clean := sanitize(v.Text)
		if clean == "" {
			return nil
		}
		m.proposal = clean
		return nil
	}
	return nil
}

// request kicks off a streaming AI call for the given site. The first complete
// line of the response becomes the proposal.
//
// When demo mode is on, the request synthesizes a SuggestMsg from the configured
// demo proposals instead of hitting the network.
func (m *Manager) request(site Site) tea.Cmd {
	atomic.AddInt64(&m.requested, 1)

	if len(m.demoProposals) > 0 {
		text := m.demoProposals[m.demoCursor%len(m.demoProposals)]
		m.demoCursor++
		return func() tea.Msg {
			// Small delay so the ghost appears after a beat, like the real
			// thing would.
			time.Sleep(120 * time.Millisecond)
			return SuggestMsg{Site: site, Text: text}
		}
	}

	if m.client == nil {
		return nil
	}
	m.cancelInflight()
	ctx, cancel := context.WithCancel(context.Background())
	m.inflight = cancel

	client := m.client
	return func() tea.Msg {
		deltas, done := client.Stream(ctx, ai.Request{
			Tier:          ai.Fast,
			System:        systemPrompt(),
			User:          userPrompt(site),
			StopSequences: []string{"\n"},
		})
		var buf strings.Builder
		for chunk := range deltas {
			buf.WriteString(chunk)
		}
		err := <-done
		text := buf.String()
		return SuggestMsg{Site: site, Text: text, Err: err}
	}
}

func (m *Manager) cancelInflight() {
	if m.inflight != nil {
		m.inflight()
		m.inflight = nil
	}
}

// sanitize trims fences and surrounding whitespace from the proposal. The
// model occasionally returns ```go\nfoo() so we strip the obvious noise.
func sanitize(s string) string {
	s = strings.TrimRight(s, " \t\r\n")
	s = strings.TrimLeft(s, " \t")
	// Strip code fence prefix first; if the fence is followed by a language
	// tag (e.g. ```go), skip past the next newline so we keep the code body.
	if strings.HasPrefix(s, "```") {
		rest := strings.TrimPrefix(s, "```")
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		}
		s = rest
		s = strings.TrimLeft(s, " \t")
	}
	// Drop anything past the first newline; ghost-text is one line.
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimRight(s, " \t")
	return s
}

func systemPrompt() string {
	return `You are an inline code completion engine inside a terminal editor.
You receive a file path, the line the cursor is on, and the cursor column.

Rules:
- Output ONLY the continuation of the current line, nothing else.
- Do NOT repeat the prefix that already exists before the cursor.
- Do NOT include backticks, markdown, or any prose.
- Do NOT include the rest of the file.
- Stop at the first newline.
- If you are unsure, output nothing.

Examples (cursor is just after the "|"):

  prefix: 'func hello() {'  suffix: '}'
  completion: '\n\tfmt.Println("hello")\n'  → NO, that includes a newline. Output empty.

  prefix: 'fmt.Pri'  suffix: ''
  completion: 'ntln("hello, world")'

  prefix: 'for i := 0; i < '  suffix: ' {'
  completion: 'len(items); i++'`
}

func userPrompt(s Site) string {
	const maxPrefix = 1500
	const maxSuffix = 1500
	prefix := s.Prefix
	if len(prefix) > maxPrefix {
		prefix = prefix[len(prefix)-maxPrefix:]
	}
	var b strings.Builder
	b.WriteString("file: ")
	b.WriteString(s.Path)
	b.WriteString("\nrow: ")
	b.WriteString(itoa(s.Row))
	b.WriteString("\ncol: ")
	b.WriteString(itoa(s.Col))
	b.WriteString("\n\nLine prefix (before cursor):\n")
	b.WriteString(prefix)
	b.WriteString("\n\nReturn ONLY the completion. Stop at end of line.")
	_ = maxSuffix
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
