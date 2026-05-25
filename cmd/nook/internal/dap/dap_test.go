package dap

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAdapter is an in-process stand-in for a DAP adapter subprocess.
// It runs in a goroutine, reads framed requests off our stdin pipe,
// hands each request to a handler the test installs, writes the
// handler's response onto our stdout pipe, and can push events on demand.
type fakeAdapter struct {
	// editor-facing pipes (handed to NewWithStreams)
	editorIn  *io.PipeWriter
	editorOut *io.PipeReader
	// adapter-facing pipes (the inverses we read from / write to)
	adaptIn  *io.PipeReader
	adaptOut *io.PipeWriter

	mu        sync.Mutex
	responses func(req rawMessage) (success bool, body any, message string)
	stop      chan struct{}
	wg        sync.WaitGroup
	eventSeq  int64
}

func newFakeAdapter(t *testing.T) *fakeAdapter {
	t.Helper()
	editorOutR, editorOutW := io.Pipe() // editor reads from R, we write to W
	editorInR, editorInW := io.Pipe()   // editor writes to W, we read from R
	f := &fakeAdapter{
		editorIn:  editorInW,
		editorOut: editorOutR,
		adaptIn:   editorInR,
		adaptOut:  editorOutW,
		stop:      make(chan struct{}),
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func (f *fakeAdapter) Close() {
	select {
	case <-f.stop:
		return
	default:
		close(f.stop)
	}
	_ = f.editorIn.Close()
	_ = f.editorOut.Close()
	_ = f.adaptIn.Close()
	_ = f.adaptOut.Close()
	f.wg.Wait()
}

// pipes returns (stdin, stdout) suitable to feed NewWithStreams. The
// editor writes commands to stdin (we read them off f.adaptIn) and reads
// responses off stdout (we write them to f.adaptOut).
func (f *fakeAdapter) pipes() (io.WriteCloser, io.ReadCloser) {
	return f.editorIn, f.editorOut
}

func (f *fakeAdapter) respondWith(fn func(req rawMessage) (success bool, body any, message string)) {
	f.mu.Lock()
	f.responses = fn
	f.mu.Unlock()
}

func (f *fakeAdapter) serve() {
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		br := bufio.NewReader(f.adaptIn)
		for {
			body, err := readFrame(br)
			if err != nil {
				return
			}
			var req rawMessage
			if err := json.Unmarshal(body, &req); err != nil {
				continue
			}
			f.mu.Lock()
			h := f.responses
			f.mu.Unlock()
			if h == nil {
				continue
			}
			success, respBody, msg := h(req)
			resp := map[string]any{
				"seq":         f.nextSeq(),
				"type":        "response",
				"request_seq": req.Seq,
				"command":     req.Command,
				"success":     success,
			}
			if msg != "" {
				resp["message"] = msg
			}
			if respBody != nil {
				resp["body"] = respBody
			}
			_ = writeFrameForTest(f.adaptOut, resp)
		}
	}()
}

func (f *fakeAdapter) nextSeq() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventSeq++
	return f.eventSeq
}

func (f *fakeAdapter) sendEvent(kind string, body any) error {
	envelope := map[string]any{
		"seq":   f.nextSeq(),
		"type":  "event",
		"event": kind,
	}
	if body != nil {
		envelope["body"] = body
	}
	return writeFrameForTest(f.adaptOut, envelope)
}

func writeFrameForTest(w io.Writer, env map[string]any) error {
	body, err := json.Marshal(env)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

func TestInitializeRoundtrips(t *testing.T) {
	f := newFakeAdapter(t)
	gotSeenInit := make(chan struct{}, 1)
	f.respondWith(func(req rawMessage) (bool, any, string) {
		if req.Command != "initialize" {
			t.Errorf("first request = %q; want initialize", req.Command)
			return false, nil, "unexpected"
		}
		select {
		case gotSeenInit <- struct{}{}:
		default:
		}
		return true, map[string]any{"supportsConfigurationDoneRequest": true}, ""
	})
	f.serve()

	stdin, stdout := f.pipes()
	c := NewWithStreams(stdin, stdout, 8)
	defer c.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Initialize(ctx, "nook-test"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	select {
	case <-gotSeenInit:
	default:
		t.Fatal("adapter never received initialize")
	}
}

func TestSetBreakpointsDecodesResponseBody(t *testing.T) {
	f := newFakeAdapter(t)
	f.respondWith(func(req rawMessage) (bool, any, string) {
		if req.Command != "setBreakpoints" {
			return false, nil, "unexpected"
		}
		var args struct {
			Lines       []int              `json:"lines"`
			Breakpoints []SourceBreakpoint `json:"breakpoints"`
		}
		_ = json.Unmarshal(req.Arguments, &args)
		if got := args.Lines; len(got) != 2 || got[0] != 7 || got[1] != 13 {
			t.Errorf("lines = %v; want [7 13]", got)
		}
		return true, map[string]any{
			"breakpoints": []map[string]any{
				{"verified": true, "line": 7},
				{"verified": false, "line": 13, "message": "no source mapping"},
			},
		}, ""
	})
	f.serve()

	stdin, stdout := f.pipes()
	c := NewWithStreams(stdin, stdout, 8)
	defer c.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	bps, err := c.SetBreakpoints(ctx, Source{Path: "/tmp/main.go"}, []int{7, 13})
	if err != nil {
		t.Fatalf("SetBreakpoints: %v", err)
	}
	if len(bps) != 2 {
		t.Fatalf("got %d breakpoints; want 2", len(bps))
	}
	if !bps[0].Verified || bps[0].Line != 7 {
		t.Errorf("bps[0] = %+v", bps[0])
	}
	if bps[1].Verified || bps[1].Message == "" {
		t.Errorf("bps[1] = %+v; want unverified with message", bps[1])
	}
}

func TestFailedResponsePropagates(t *testing.T) {
	f := newFakeAdapter(t)
	f.respondWith(func(req rawMessage) (bool, any, string) {
		return false, nil, "adapter rejected"
	})
	f.serve()

	stdin, stdout := f.pipes()
	c := NewWithStreams(stdin, stdout, 8)
	defer c.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := c.Launch(ctx, "/tmp/main.go", "debug")
	if err == nil {
		t.Fatal("expected error from failed response")
	}
	if !strings.Contains(err.Error(), "adapter rejected") {
		t.Errorf("err = %v; want substring 'adapter rejected'", err)
	}
}

func TestStoppedEventSurfaces(t *testing.T) {
	f := newFakeAdapter(t)
	f.respondWith(func(req rawMessage) (bool, any, string) { return true, map[string]any{}, "" })
	f.serve()

	stdin, stdout := f.pipes()
	c := NewWithStreams(stdin, stdout, 8)
	defer c.Shutdown()

	go func() {
		_ = f.sendEvent("stopped", map[string]any{
			"reason":            "breakpoint",
			"threadId":          1,
			"allThreadsStopped": true,
		})
	}()

	select {
	case e := <-c.Events():
		if e.Kind != "stopped" {
			t.Fatalf("event kind = %q; want stopped", e.Kind)
		}
		if e.Stopped == nil || e.Stopped.Reason != "breakpoint" || e.Stopped.ThreadID != 1 {
			t.Errorf("stopped body = %+v", e.Stopped)
		}
		if !e.Stopped.AllThreadsStopped {
			t.Error("allThreadsStopped = false; want true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stopped event")
	}
}

func TestOutputAndContinuedAndExitedEvents(t *testing.T) {
	f := newFakeAdapter(t)
	f.respondWith(func(req rawMessage) (bool, any, string) { return true, map[string]any{}, "" })
	f.serve()

	stdin, stdout := f.pipes()
	c := NewWithStreams(stdin, stdout, 16)
	defer c.Shutdown()

	go func() {
		_ = f.sendEvent("output", map[string]any{"category": "stdout", "output": "hello\n"})
		_ = f.sendEvent("continued", map[string]any{"threadId": 1, "allThreadsContinued": true})
		_ = f.sendEvent("exited", map[string]any{"exitCode": 7})
	}()

	want := []string{"output", "continued", "exited"}
	for i := 0; i < len(want); i++ {
		select {
		case e := <-c.Events():
			if e.Kind != want[i] {
				t.Errorf("event[%d] kind = %q; want %q", i, e.Kind, want[i])
			}
			switch e.Kind {
			case "output":
				if e.Output == nil || e.Output.Output != "hello\n" || e.Output.Category != "stdout" {
					t.Errorf("output = %+v", e.Output)
				}
			case "continued":
				if e.Continued == nil || e.Continued.ThreadID != 1 || !e.Continued.AllThreadsContinued {
					t.Errorf("continued = %+v", e.Continued)
				}
			case "exited":
				if e.Exited == nil || e.Exited.ExitCode != 7 {
					t.Errorf("exited = %+v", e.Exited)
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event[%d] = %s", i, want[i])
		}
	}
}

func TestShutdownIsIdempotent(t *testing.T) {
	f := newFakeAdapter(t)
	f.respondWith(func(req rawMessage) (bool, any, string) { return true, map[string]any{}, "" })
	f.serve()

	stdin, stdout := f.pipes()
	c := NewWithStreams(stdin, stdout, 4)
	if err := c.Shutdown(); err != nil {
		t.Errorf("first Shutdown: %v", err)
	}
	if err := c.Shutdown(); err != nil {
		t.Errorf("second Shutdown: %v", err)
	}
}

func TestStackTraceDecodes(t *testing.T) {
	f := newFakeAdapter(t)
	f.respondWith(func(req rawMessage) (bool, any, string) {
		return true, map[string]any{
			"stackFrames": []map[string]any{
				{"id": 1000, "name": "main.main", "source": map[string]any{"path": "/tmp/main.go", "name": "main.go"}, "line": 7, "column": 1},
				{"id": 1001, "name": "runtime.main", "source": map[string]any{"path": "", "name": ""}, "line": 0, "column": 0},
			},
		}, ""
	})
	f.serve()

	stdin, stdout := f.pipes()
	c := NewWithStreams(stdin, stdout, 4)
	defer c.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	frames, err := c.StackTrace(ctx, 1, 0)
	if err != nil {
		t.Fatalf("StackTrace: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("len = %d; want 2", len(frames))
	}
	if frames[0].Source.Path != "/tmp/main.go" || frames[0].Line != 7 {
		t.Errorf("frame[0] = %+v", frames[0])
	}
}
