package tasks

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStartRejectsInvalidTask(t *testing.T) {
	_, err := Start(context.Background(), t.TempDir(), Task{})
	if err == nil {
		t.Fatal("expected error for invalid task")
	}
}

func TestStartAndDrainExitsCleanly(t *testing.T) {
	r, err := Start(context.Background(), t.TempDir(), Task{
		Name:    "say hi",
		Command: "sh",
		Args:    []string{"-c", "echo hello && echo world"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	lines := drainAll(t, r, 3*time.Second)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %+v", len(lines), lines)
	}
	if lines[0].Line != "hello" || lines[1].Line != "world" {
		t.Fatalf("lines: %+v", lines)
	}

	exit := waitForExit(t, r, 3*time.Second)
	if exit.ExitCode != 0 {
		t.Fatalf("exit code %d, want 0; err=%v", exit.ExitCode, exit.Err)
	}
	if exit.RunID != r.ID() {
		t.Fatalf("exit RunID=%d, runner ID=%d", exit.RunID, r.ID())
	}
}

func TestStartStderrIsTaggedSeparately(t *testing.T) {
	r, err := Start(context.Background(), t.TempDir(), Task{
		Name:    "stderr",
		Command: "sh",
		Args:    []string{"-c", "echo to-stdout; echo to-stderr 1>&2"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	lines := drainAll(t, r, 3*time.Second)
	waitForExit(t, r, 3*time.Second)

	streams := map[string]Stream{}
	for _, l := range lines {
		streams[l.Line] = l.Stream
	}
	if streams["to-stdout"] != StreamStdout {
		t.Fatalf("to-stdout stream=%v", streams["to-stdout"])
	}
	if streams["to-stderr"] != StreamStderr {
		t.Fatalf("to-stderr stream=%v", streams["to-stderr"])
	}
}

func TestStartCwdRelativeResolvesAgainstRoot(t *testing.T) {
	root := t.TempDir()
	if err := mkdir(root + "/sub"); err != nil {
		t.Fatal(err)
	}
	r, err := Start(context.Background(), root, Task{
		Name:    "pwd",
		Command: "sh",
		Args:    []string{"-c", "pwd"},
		Cwd:     "sub",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	lines := drainAll(t, r, 3*time.Second)
	waitForExit(t, r, 3*time.Second)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	if !strings.HasSuffix(lines[0].Line, "/sub") {
		t.Fatalf("pwd output %q does not end in /sub", lines[0].Line)
	}
}

func TestKillTerminatesRunningProcess(t *testing.T) {
	r, err := Start(context.Background(), t.TempDir(), Task{
		Name:    "sleeper",
		Command: "sh",
		Args:    []string{"-c", "sleep 10"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Give the process a moment to register as running.
	time.Sleep(50 * time.Millisecond)
	r.Kill()
	exit := waitForExit(t, r, 3*time.Second)
	if exit.ExitCode == 0 {
		t.Fatalf("killed process exited 0; expected non-zero")
	}
}

func TestNonZeroExitCodePropagates(t *testing.T) {
	r, err := Start(context.Background(), t.TempDir(), Task{
		Name:    "fail",
		Command: "sh",
		Args:    []string{"-c", "exit 7"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	exit := waitForExit(t, r, 3*time.Second)
	if exit.ExitCode != 7 {
		t.Fatalf("exit code %d, want 7; err=%v", exit.ExitCode, exit.Err)
	}
}

func TestEnvOverridesAreVisibleToChild(t *testing.T) {
	r, err := Start(context.Background(), t.TempDir(), Task{
		Name:    "env",
		Command: "sh",
		Args:    []string{"-c", "echo \"$NOOK_TASK_TEST_VAR\""},
		Env:     map[string]string{"NOOK_TASK_TEST_VAR": "from-task"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	lines := drainAll(t, r, 3*time.Second)
	waitForExit(t, r, 3*time.Second)
	if len(lines) != 1 || lines[0].Line != "from-task" {
		t.Fatalf("env passthrough lines=%+v", lines)
	}
}

func TestStartedCmdProducesStartedMsg(t *testing.T) {
	r, err := Start(context.Background(), t.TempDir(), Task{
		Name:    "noop",
		Command: "sh",
		Args:    []string{"-c", "exit 0"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	msg := r.StartedCmd()()
	started, ok := msg.(StartedMsg)
	if !ok {
		t.Fatalf("StartedCmd: got %T, want StartedMsg", msg)
	}
	if started.RunID != r.ID() {
		t.Fatalf("StartedMsg.RunID=%d, want %d", started.RunID, r.ID())
	}
	if started.Task.Name != "noop" {
		t.Fatalf("StartedMsg.Task=%+v", started.Task)
	}
	waitForExit(t, r, 3*time.Second)
}

func drainAll(t *testing.T, r *Runner, timeout time.Duration) []LineMsg {
	t.Helper()
	var lines []LineMsg
	deadline := time.After(timeout)
	for {
		msgCh := make(chan tea.Msg, 1)
		go func() {
			cmd := r.NextLineCmd()
			msgCh <- cmd()
		}()
		select {
		case m := <-msgCh:
			if m == nil {
				return lines
			}
			line, ok := m.(LineMsg)
			if !ok {
				t.Fatalf("expected LineMsg, got %T", m)
			}
			lines = append(lines, line)
		case <-deadline:
			t.Fatalf("timed out draining; got %d lines", len(lines))
		}
	}
}

func waitForExit(t *testing.T, r *Runner, timeout time.Duration) ExitMsg {
	t.Helper()
	done := make(chan tea.Msg, 1)
	go func() {
		done <- r.WaitCmd()()
	}()
	select {
	case m := <-done:
		exit, ok := m.(ExitMsg)
		if !ok {
			t.Fatalf("expected ExitMsg, got %T", m)
		}
		return exit
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for exit")
		return ExitMsg{}
	}
}

func mkdir(p string) error {
	return os.MkdirAll(p, 0o755)
}
