// Package dap drives a debug-adapter subprocess (delve via `dlv dap` by
// default) over stdio using the Debug Adapter Protocol. The wedge stays
// narrow: Start/Shutdown lifecycle, Launch/SetBreakpoints/Continue/Pause/
// Terminate requests, plus an events channel that surfaces the subset of
// adapter events the editor needs to render (Initialized, Stopped,
// Continued, Output, Terminated, Exited).
//
// DAP framing is identical to LSP: "Content-Length: <n>\r\n\r\n<json>".
// The bodies are DAP-specific. We hand-roll the protocol shapes rather
// than pull in github.com/google/go-dap because we only use ~10 of its
// ~80 message types and the dependency footprint matters for `go install
// @latest`.
package dap

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Event is the subset of DAP adapter events the editor consumes.
type Event struct {
	// Kind names the event ("initialized", "stopped", "continued",
	// "output", "terminated", "exited"). Unknown events still arrive
	// here so the editor can ignore or log them.
	Kind string
	// Stopped is populated for Kind=="stopped".
	Stopped *StoppedBody
	// Continued is populated for Kind=="continued".
	Continued *ContinuedBody
	// Output is populated for Kind=="output".
	Output *OutputBody
	// Terminated is populated for Kind=="terminated".
	Terminated *TerminatedBody
	// Exited is populated for Kind=="exited".
	Exited *ExitedBody
}

// StoppedBody mirrors the DAP "stopped" event body. We only project the
// fields nook displays.
type StoppedBody struct {
	Reason            string `json:"reason"`
	Description       string `json:"description,omitempty"`
	ThreadID          int    `json:"threadId,omitempty"`
	AllThreadsStopped bool   `json:"allThreadsStopped,omitempty"`
	Text              string `json:"text,omitempty"`
	HitBreakpointIDs  []int  `json:"hitBreakpointIds,omitempty"`
}

// ContinuedBody mirrors the DAP "continued" event body.
type ContinuedBody struct {
	ThreadID            int  `json:"threadId"`
	AllThreadsContinued bool `json:"allThreadsContinued,omitempty"`
}

// OutputBody mirrors the DAP "output" event body. Category is
// "stdout" / "stderr" / "console" / "telemetry" / etc.
type OutputBody struct {
	Category string `json:"category,omitempty"`
	Output   string `json:"output"`
}

// TerminatedBody mirrors the DAP "terminated" event body.
type TerminatedBody struct {
	Restart bool `json:"restart,omitempty"`
}

// ExitedBody mirrors the DAP "exited" event body.
type ExitedBody struct {
	ExitCode int `json:"exitCode"`
}

// Source identifies a source file for setBreakpoints requests.
type Source struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

// SourceBreakpoint requests a breakpoint at a 1-based line in Source.
type SourceBreakpoint struct {
	Line      int    `json:"line"`
	Condition string `json:"condition,omitempty"`
}

// Breakpoint is one entry in the response to setBreakpoints.
type Breakpoint struct {
	Verified bool   `json:"verified"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message,omitempty"`
}

// StackFrame is one entry from a stackTrace response.
type StackFrame struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Source Source `json:"source"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// Options configures the adapter subprocess. Zero values are safe.
type Options struct {
	// Binary names the executable to spawn. Defaults to "dlv".
	Binary string
	// Args are extra args to pass to the binary. Defaults to ["dap"].
	Args []string
	// WorkDir is the working directory for the spawned process.
	WorkDir string
	// EventBuffer sets the events channel capacity. Defaults to 64.
	EventBuffer int
}

// Client is a thin DAP client over a subprocess stdio pair.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	cancel context.CancelFunc

	// seq is a monotonic counter for outbound request ids.
	seq atomic.Int64

	// pendings maps a request seq to the channel waiting for that
	// response. Mutex-guarded because the reader goroutine signals on
	// these channels.
	mu       sync.Mutex
	pendings map[int64]chan rawMessage

	events chan Event

	// closed is set when Shutdown runs, so the reader goroutine can
	// distinguish "stream closed during normal shutdown" from "adapter
	// crashed."
	closed atomic.Bool
}

// Start spawns the debug adapter and begins reading messages. The reader
// goroutine routes responses to waiters and pushes events onto Events().
// The caller must drain Events() in some goroutine (or buffer overflow
// will silently drop events).
func Start(ctx context.Context, opts Options) (*Client, error) {
	if opts.Binary == "" {
		opts.Binary = "dlv"
	}
	if len(opts.Args) == 0 {
		opts.Args = []string{"dap"}
	}
	if opts.EventBuffer <= 0 {
		opts.EventBuffer = 64
	}

	cmdCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(cmdCtx, opts.Binary, opts.Args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("dap: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("dap: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("dap: start %s: %w", opts.Binary, err)
	}

	c := &Client{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   stdout,
		cancel:   cancel,
		pendings: make(map[int64]chan rawMessage),
		events:   make(chan Event, opts.EventBuffer),
	}
	go c.read()
	return c, nil
}

// NewWithStreams is a test seam: it constructs a Client around a pair of
// pre-existing streams (e.g. bytes.Buffer wrapped in io.Pipe). The caller
// owns the streams; Shutdown will close them. No process is started.
func NewWithStreams(stdin io.WriteCloser, stdout io.ReadCloser, eventBuffer int) *Client {
	if eventBuffer <= 0 {
		eventBuffer = 64
	}
	c := &Client{
		stdin:    stdin,
		stdout:   stdout,
		pendings: make(map[int64]chan rawMessage),
		events:   make(chan Event, eventBuffer),
	}
	go c.read()
	return c
}

// Events returns the receive end of the events channel. Always drain it.
func (c *Client) Events() <-chan Event { return c.events }

// Shutdown terminates the adapter. Safe to call more than once.
func (c *Client) Shutdown() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cancel != nil {
		c.cancel()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Wait()
	}
	// Drain pending waiters so they unblock with a useful error.
	c.mu.Lock()
	for seq, ch := range c.pendings {
		close(ch)
		delete(c.pendings, seq)
	}
	c.mu.Unlock()
	return nil
}

// rawMessage is the on-wire DAP envelope, partially decoded.
type rawMessage struct {
	Seq        int64           `json:"seq"`
	Type       string          `json:"type"`
	RequestSeq int64           `json:"request_seq,omitempty"`
	Command    string          `json:"command,omitempty"`
	Event      string          `json:"event,omitempty"`
	Success    bool            `json:"success,omitempty"`
	Message    string          `json:"message,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
	Arguments  json.RawMessage `json:"arguments,omitempty"`
}

// read consumes framed messages off stdout, routes responses to waiters,
// and pushes events onto c.events.
func (c *Client) read() {
	br := bufio.NewReader(c.stdout)
	for {
		body, err := readFrame(br)
		if err != nil {
			if !c.closed.Load() {
				c.publish(Event{Kind: "terminated", Terminated: &TerminatedBody{}})
			}
			return
		}
		var msg rawMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "response":
			c.mu.Lock()
			ch, ok := c.pendings[msg.RequestSeq]
			if ok {
				delete(c.pendings, msg.RequestSeq)
			}
			c.mu.Unlock()
			if ok {
				ch <- msg
				close(ch)
			}
		case "event":
			c.publish(c.decodeEvent(msg))
		}
	}
}

// publish pushes an event onto the channel, dropping silently if the
// channel buffer is full (so a slow consumer cannot block the reader).
func (c *Client) publish(e Event) {
	select {
	case c.events <- e:
	default:
	}
}

// decodeEvent decodes a "type=event" frame into a typed Event.
func (c *Client) decodeEvent(msg rawMessage) Event {
	out := Event{Kind: msg.Event}
	switch msg.Event {
	case "stopped":
		var b StoppedBody
		_ = json.Unmarshal(msg.Body, &b)
		out.Stopped = &b
	case "continued":
		var b ContinuedBody
		_ = json.Unmarshal(msg.Body, &b)
		out.Continued = &b
	case "output":
		var b OutputBody
		_ = json.Unmarshal(msg.Body, &b)
		out.Output = &b
	case "terminated":
		var b TerminatedBody
		_ = json.Unmarshal(msg.Body, &b)
		out.Terminated = &b
	case "exited":
		var b ExitedBody
		_ = json.Unmarshal(msg.Body, &b)
		out.Exited = &b
	}
	return out
}

// send marshals a request and writes a framed message. If wantResponse
// is true, the caller is registered to receive the response.
func (c *Client) send(ctx context.Context, command string, args any, wantResponse bool) (rawMessage, error) {
	seq := c.seq.Add(1)
	envelope := map[string]any{
		"seq":     seq,
		"type":    "request",
		"command": command,
	}
	if args != nil {
		envelope["arguments"] = args
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return rawMessage{}, fmt.Errorf("dap: marshal %s: %w", command, err)
	}

	var ch chan rawMessage
	if wantResponse {
		ch = make(chan rawMessage, 1)
		c.mu.Lock()
		c.pendings[seq] = ch
		c.mu.Unlock()
	}

	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(c.stdin, frame); err != nil {
		c.unregister(seq)
		return rawMessage{}, fmt.Errorf("dap: write header %s: %w", command, err)
	}
	if _, err := c.stdin.Write(body); err != nil {
		c.unregister(seq)
		return rawMessage{}, fmt.Errorf("dap: write body %s: %w", command, err)
	}
	if !wantResponse {
		return rawMessage{}, nil
	}

	select {
	case <-ctx.Done():
		c.unregister(seq)
		return rawMessage{}, ctx.Err()
	case resp, ok := <-ch:
		if !ok {
			return rawMessage{}, errors.New("dap: connection closed")
		}
		if !resp.Success {
			if resp.Message != "" {
				return resp, fmt.Errorf("dap: %s failed: %s", command, resp.Message)
			}
			return resp, fmt.Errorf("dap: %s failed", command)
		}
		return resp, nil
	}
}

func (c *Client) unregister(seq int64) {
	c.mu.Lock()
	if ch, ok := c.pendings[seq]; ok {
		delete(c.pendings, seq)
		close(ch)
	}
	c.mu.Unlock()
}

// readFrame consumes one "Content-Length: N\r\n\r\n<body>" frame.
func readFrame(br *bufio.Reader) ([]byte, error) {
	var length int
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			n, err := strconv.Atoi(strings.TrimSpace(line[len("Content-Length:"):]))
			if err != nil {
				return nil, fmt.Errorf("dap: bad Content-Length: %w", err)
			}
			length = n
		}
	}
	if length <= 0 {
		return nil, errors.New("dap: missing Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(br, body); err != nil {
		return nil, err
	}
	return body, nil
}

// Initialize sends the DAP initialize request. clientID names this editor.
func (c *Client) Initialize(ctx context.Context, clientID string) error {
	args := map[string]any{
		"clientID":                     clientID,
		"clientName":                   clientID,
		"adapterID":                    "go",
		"linesStartAt1":                true,
		"columnsStartAt1":              true,
		"pathFormat":                   "path",
		"supportsRunInTerminalRequest": false,
	}
	_, err := c.send(ctx, "initialize", args, true)
	return err
}

// Launch sends "launch" with type=go, request=launch, program=<path>.
// The path should be a directory or a single file containing a `main`
// package. mode controls dlv's launch mode ("debug" / "test" /
// "exec"); defaults to "debug".
func (c *Client) Launch(ctx context.Context, program, mode string) error {
	if mode == "" {
		mode = "debug"
	}
	args := map[string]any{
		"type":    "go",
		"request": "launch",
		"name":    "nook debug",
		"mode":    mode,
		"program": program,
	}
	_, err := c.send(ctx, "launch", args, true)
	return err
}

// SetBreakpoints replaces every breakpoint in source with the given lines.
// Empty lines clears all breakpoints in that source.
func (c *Client) SetBreakpoints(ctx context.Context, source Source, lines []int) ([]Breakpoint, error) {
	bps := make([]SourceBreakpoint, len(lines))
	for i, line := range lines {
		bps[i] = SourceBreakpoint{Line: line}
	}
	args := map[string]any{
		"source":      source,
		"breakpoints": bps,
		"lines":       lines,
	}
	resp, err := c.send(ctx, "setBreakpoints", args, true)
	if err != nil {
		return nil, err
	}
	var body struct {
		Breakpoints []Breakpoint `json:"breakpoints"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("dap: decode setBreakpoints body: %w", err)
	}
	return body.Breakpoints, nil
}

// ConfigurationDone signals the adapter that the editor has finished
// setting initial breakpoints. The adapter usually replies with a
// "stopped" event once the debuggee hits its first breakpoint (or runs
// to completion).
func (c *Client) ConfigurationDone(ctx context.Context) error {
	_, err := c.send(ctx, "configurationDone", map[string]any{}, true)
	return err
}

// Continue resumes the stopped thread.
func (c *Client) Continue(ctx context.Context, threadID int) error {
	_, err := c.send(ctx, "continue", map[string]any{"threadId": threadID}, true)
	return err
}

// Pause requests the adapter to pause the running thread.
func (c *Client) Pause(ctx context.Context, threadID int) error {
	_, err := c.send(ctx, "pause", map[string]any{"threadId": threadID}, true)
	return err
}

// Next steps over the current line on threadID.
func (c *Client) Next(ctx context.Context, threadID int) error {
	_, err := c.send(ctx, "next", map[string]any{"threadId": threadID}, true)
	return err
}

// StepIn steps into the call on threadID.
func (c *Client) StepIn(ctx context.Context, threadID int) error {
	_, err := c.send(ctx, "stepIn", map[string]any{"threadId": threadID}, true)
	return err
}

// StepOut steps out of the current frame on threadID.
func (c *Client) StepOut(ctx context.Context, threadID int) error {
	_, err := c.send(ctx, "stepOut", map[string]any{"threadId": threadID}, true)
	return err
}

// StackTrace requests the top `levels` frames for threadID. Pass 0 for
// "all frames."
func (c *Client) StackTrace(ctx context.Context, threadID, levels int) ([]StackFrame, error) {
	args := map[string]any{"threadId": threadID}
	if levels > 0 {
		args["levels"] = levels
	}
	resp, err := c.send(ctx, "stackTrace", args, true)
	if err != nil {
		return nil, err
	}
	var body struct {
		StackFrames []StackFrame `json:"stackFrames"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("dap: decode stackTrace body: %w", err)
	}
	return body.StackFrames, nil
}

// Terminate asks the adapter to kill the debuggee.
func (c *Client) Terminate(ctx context.Context) error {
	_, err := c.send(ctx, "terminate", map[string]any{}, true)
	return err
}

// Disconnect closes the debug session. terminateDebuggee=true asks dlv
// to kill the inferior.
func (c *Client) Disconnect(ctx context.Context, terminateDebuggee bool) error {
	args := map[string]any{
		"terminateDebuggee": terminateDebuggee,
	}
	_, err := c.send(ctx, "disconnect", args, true)
	return err
}
