// Package ai drives the AI wedges in nook by spawning the user's existing
// Claude Code CLI as a subprocess. There is no separate API key requirement
// and no direct HTTP call to api.anthropic.com — whatever auth `claude` is
// already configured with (OAuth session or ANTHROPIC_API_KEY) is what nook
// uses. The wrapper exposes a single Client.Stream call that returns a
// channel of text deltas plus a done channel, matching the surface the
// edit/composer wedges already consume.
package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Tier selects the model. We pin two tiers and nothing else so the editor's
// keybindings stay model-agnostic.
type Tier int

const (
	// Fast pins claude-haiku-4-5: low-latency inline edits.
	Fast Tier = iota
	// Smart pins claude-sonnet-4-6: multi-file Composer edits.
	Smart
)

// Model returns the --model flag value for the tier.
func (t Tier) Model() string {
	if t == Smart {
		return "claude-sonnet-4-6"
	}
	return "claude-haiku-4-5"
}

// ErrNoClaude is returned when the `claude` binary cannot be located on PATH
// or in any of the well-known install locations. nook stays useful (search,
// git, editor, terminal, LSP) when this fires; only the AI wedges go dark.
var ErrNoClaude = errors.New("claude CLI not found on PATH (install: npm i -g @anthropic-ai/claude-code)")

// Client owns the resolved path to the claude binary. It is safe for
// concurrent use; each Stream call spawns its own subprocess.
type Client struct {
	binary string
}

// NewClient locates the claude binary. The lookup mirrors the official
// claude-agent-sdk install search order so nook works with npm-global, brew,
// nvm, and the per-user ~/.claude/local install.
func NewClient() (*Client, error) {
	p := findClaude()
	if p == "" {
		return nil, ErrNoClaude
	}
	return &Client{binary: p}, nil
}

// NewClientWithBinary constructs a Client bound to an explicit binary path.
// Tests use this to point at a stub `claude` so the suite stays hermetic.
func NewClientWithBinary(path string) (*Client, error) {
	if path == "" {
		return nil, errors.New("ai: binary path is empty")
	}
	return &Client{binary: path}, nil
}

// Available reports whether the claude binary is reachable. Useful for
// status-bar rendering before any wedge fires.
func Available() bool {
	return findClaude() != ""
}

func findClaude() string {
	if p, err := exec.LookPath("claude"); err == nil {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, p := range []string{
		filepath.Join(home, ".claude", "local", "claude"),
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, ".npm-global", "bin", "claude"),
		"/usr/local/bin/claude",
	} {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// Request is the small surface for a single streaming call.
type Request struct {
	Tier   Tier
	System string
	User   string
	// StopSequences cuts the stream early once the accumulated text contains
	// one of these markers. claude CLI doesn't natively expose stop_sequences,
	// so we detect client-side and SIGTERM the subprocess.
	StopSequences []string
}

// Stream spawns `claude --print --output-format stream-json …` and emits each
// text_delta as a delta on the returned channel. The done channel emits one
// error (or nil) when the stream closes. Callers MUST drain deltas until
// close. Cancelling ctx kills the subprocess.
func (c *Client) Stream(ctx context.Context, req Request) (<-chan string, <-chan error) {
	deltas := make(chan string, 64)
	done := make(chan error, 1)

	go func() {
		defer close(deltas)
		defer close(done)

		args := []string{
			"--print", req.User,
			"--output-format", "stream-json",
			"--verbose",
			"--include-partial-messages",
			"--model", req.Tier.Model(),
			"--tools", "",
			"--no-session-persistence",
			"--disable-slash-commands",
			"--permission-mode", "bypassPermissions",
			"--effort", "low",
		}
		if req.System != "" {
			args = append(args, "--system-prompt", req.System)
		}

		cmd := exec.CommandContext(ctx, c.binary, args...)
		cmd.Env = append(os.Environ(), "CLAUDE_CODE_ENTRYPOINT=nook")

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			done <- fmt.Errorf("claude stdout pipe: %w", err)
			return
		}
		// Stderr is captured so a failed launch surfaces something useful.
		var stderrBuf strings.Builder
		cmd.Stderr = &writerTee{w: &stderrBuf}

		if err := cmd.Start(); err != nil {
			done <- fmt.Errorf("claude start: %w", err)
			return
		}

		var emitted strings.Builder
		streamErr := streamDeltas(ctx, stdout, deltas, &emitted, req.StopSequences, cmd)

		waitErr := cmd.Wait()
		switch {
		case streamErr != nil && !errors.Is(streamErr, errStopHit):
			done <- streamErr
		case waitErr != nil && !errors.Is(streamErr, errStopHit):
			// Suppress wait error when we deliberately killed the process
			// because of a stop-sequence hit.
			s := strings.TrimSpace(stderrBuf.String())
			if s != "" {
				done <- fmt.Errorf("claude exited: %w (%s)", waitErr, lastLine(s))
			} else {
				done <- fmt.Errorf("claude exited: %w", waitErr)
			}
		default:
			done <- nil
		}
	}()

	return deltas, done
}

// errStopHit is the sentinel used internally to signal the streamer reached a
// stop sequence and killed the subprocess on purpose.
var errStopHit = errors.New("stop sequence reached")

// streamDeltas parses the claude stream-json output and pushes text_delta
// fragments onto deltas. It returns errStopHit when a stop sequence is found,
// nil on clean EOF, or a parse/IO error otherwise.
func streamDeltas(ctx context.Context, r io.Reader, deltas chan<- string, emitted *strings.Builder, stops []string, cmd *exec.Cmd) error {
	sc := bufio.NewScanner(r)
	// stream-json lines can be large (full assistant messages, status events,
	// init blocks). Lift the scanner buffer well past the default 64KB.
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var env struct {
			Type  string `json:"type"`
			Event struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			} `json:"event"`
		}
		if err := json.Unmarshal(line, &env); err != nil {
			// Tolerate non-JSON output (banners, ANSI). Skip the line.
			continue
		}
		if env.Type != "stream_event" {
			continue
		}
		if env.Event.Type != "content_block_delta" {
			continue
		}
		if env.Event.Delta.Type != "text_delta" {
			// Skip thinking_delta, signature_delta, etc.
			continue
		}
		text := env.Event.Delta.Text
		if text == "" {
			continue
		}

		// Trim to the stop sequence if the accumulated text crosses one.
		out := text
		if len(stops) > 0 {
			combined := emitted.String() + text
			cut := findStopCut(combined, stops)
			if cut >= 0 {
				keep := cut - emitted.Len()
				if keep < 0 {
					keep = 0
				}
				if keep > len(text) {
					keep = len(text)
				}
				out = text[:keep]
			}
			if out != "" {
				emitted.WriteString(out)
				select {
				case deltas <- out:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			if cut >= 0 {
				if cmd != nil && cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				return errStopHit
			}
			continue
		}

		emitted.WriteString(out)
		select {
		case deltas <- out:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return sc.Err()
}

func findStopCut(s string, stops []string) int {
	best := -1
	for _, stop := range stops {
		if stop == "" {
			continue
		}
		idx := strings.Index(s, stop)
		if idx < 0 {
			continue
		}
		if best < 0 || idx < best {
			best = idx
		}
	}
	return best
}

// writerTee is a tiny io.Writer that fans out to a strings.Builder. We use
// it to capture stderr without locking ourselves into the type.
type writerTee struct {
	w *strings.Builder
}

func (t *writerTee) Write(p []byte) (int, error) { return t.w.Write(p) }

func lastLine(s string) string {
	s = strings.TrimRight(s, "\n")
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		return s[i+1:]
	}
	return s
}
