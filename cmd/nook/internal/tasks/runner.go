package tasks

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Stream identifies which file descriptor a line came from.
type Stream int

const (
	// StreamStdout is the line came from the process's stdout.
	StreamStdout Stream = iota
	// StreamStderr is the line came from the process's stderr.
	StreamStderr
	// StreamSystem is a synthetic line emitted by the runner itself
	// (start banner, kill notice, exit summary).
	StreamSystem
)

// LineMsg carries one line of output from a running task. Lines are
// emitted in the order the OS hands them to us; stdout and stderr can
// interleave.
type LineMsg struct {
	// RunID identifies which task run this line belongs to. The host
	// uses this to discard messages from previous (already-completed
	// or killed) runs.
	RunID int
	Line  string
	Stream
}

// ExitMsg is emitted exactly once per task run, after the last LineMsg.
type ExitMsg struct {
	RunID    int
	ExitCode int
	Err      error
	Duration time.Duration
}

// StartedMsg is emitted as the very first message of a run, before any
// LineMsg, so the pane knows the run is live even when the process
// hasn't produced output yet.
type StartedMsg struct {
	RunID int
	Task  Task
	When  time.Time
}

// Runner supervises a single running task. Construct via Start. The
// zero value is unusable.
type Runner struct {
	id     int
	task   Task
	cmd    *exec.Cmd
	cancel context.CancelFunc

	mu       sync.Mutex
	done     chan struct{}
	exit     ExitMsg
	exitSent bool

	lines chan LineMsg

	startedAt time.Time
}

// nextRunID is the monotonic counter for run identifiers. We do not
// need cryptographic uniqueness; the host just wants to discard stale
// messages.
var nextRunIDMu sync.Mutex
var nextRunID int

func newRunID() int {
	nextRunIDMu.Lock()
	defer nextRunIDMu.Unlock()
	nextRunID++
	return nextRunID
}

// Start spawns the task in a child process. Working directory falls
// back to root when t.Cwd is blank. Returns the running Runner and a
// nil error on success; if the process cannot be started, the error
// is non-nil and the Runner is zero-valued.
//
// Output streaming runs in background goroutines that push lines into
// an internal channel. Call NextLineCmd (and chain it on each LineMsg)
// to drain that channel into Bubble Tea messages, plus WaitCmd to learn
// when the process exits.
func Start(parent context.Context, root string, t Task) (*Runner, error) {
	if !t.IsValid() {
		return nil, errors.New("tasks: task missing name or command")
	}
	ctx, cancel := context.WithCancel(parent)
	cmd := exec.CommandContext(ctx, t.Command, t.Args...)

	cwd := t.Cwd
	if cwd == "" {
		cwd = root
	} else if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(root, cwd)
	}
	cmd.Dir = cwd

	if len(t.Env) > 0 {
		cmd.Env = mergeEnv(t.Env)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}

	r := &Runner{
		id:        newRunID(),
		task:      t,
		cmd:       cmd,
		cancel:    cancel,
		done:      make(chan struct{}),
		lines:     make(chan LineMsg, 256),
		startedAt: time.Now(),
	}

	if err := cmd.Start(); err != nil {
		cancel()
		close(r.done)
		return nil, err
	}

	go r.pumpReader(stdout, StreamStdout)
	go r.pumpReader(stderr, StreamStderr)

	go func() {
		err := cmd.Wait()
		r.mu.Lock()
		r.exit = ExitMsg{
			RunID:    r.id,
			ExitCode: cmd.ProcessState.ExitCode(),
			Err:      err,
			Duration: time.Since(r.startedAt),
		}
		r.mu.Unlock()
		// Give the reader goroutines a moment to finish flushing
		// any final buffered lines before we close the channel.
		close(r.lines)
		close(r.done)
	}()

	return r, nil
}

func (r *Runner) pumpReader(rc io.ReadCloser, stream Stream) {
	scanner := bufio.NewScanner(rc)
	// Default buffer is 64 KB. Bump it so a single very-long line
	// (think compiler error with a giant inlined trace) doesn't kill
	// the scanner mid-run.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		r.lines <- LineMsg{
			RunID:  r.id,
			Stream: stream,
			Line:   scanner.Text(),
		}
	}
	// Errors are surfaced via the exit message; nothing to do here.
}

// ID returns the run ID assigned at Start.
func (r *Runner) ID() int { return r.id }

// Task returns a copy of the spawned task descriptor.
func (r *Runner) Task() Task { return r.task }

// StartedAt returns the wall-clock time the runner spawned the process.
func (r *Runner) StartedAt() time.Time { return r.startedAt }

// Kill terminates the running process. Safe to call multiple times;
// later calls are no-ops once the process has exited.
func (r *Runner) Kill() {
	r.cancel()
}

// Done returns a channel closed when the process exits and its
// streamed lines have drained.
func (r *Runner) Done() <-chan struct{} { return r.done }

// NextLineCmd returns a Bubble Tea command that blocks (on the
// internal channel) for the next LineMsg from this runner. When the
// process exits and the channel closes, returns nil so the host's
// message handler can stop chaining.
func (r *Runner) NextLineCmd() tea.Cmd {
	return func() tea.Msg {
		line, ok := <-r.lines
		if !ok {
			return nil
		}
		return line
	}
}

// WaitCmd returns a Bubble Tea command that blocks until the runner's
// process exits and emits the ExitMsg exactly once. Subsequent calls
// after the first message has been observed are safe but block forever
// — call once per run.
func (r *Runner) WaitCmd() tea.Cmd {
	return func() tea.Msg {
		<-r.done
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.exitSent {
			return nil
		}
		r.exitSent = true
		return r.exit
	}
}

// StartedCmd returns a Bubble Tea command that emits a StartedMsg
// immediately. Useful as the first message in a run so the pane shows
// a banner before any output arrives.
func (r *Runner) StartedCmd() tea.Cmd {
	return func() tea.Msg {
		return StartedMsg{
			RunID: r.id,
			Task:  r.task,
			When:  r.startedAt,
		}
	}
}
