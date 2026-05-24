package tasks

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func newTestPane() Pane {
	return NewPane(theme.Default, "/tmp/proj").WithSize(80, 20).Focus()
}

func TestPaneStartsInListMode(t *testing.T) {
	p := newTestPane()
	if p.Mode() != ModeList {
		t.Fatalf("default Mode=%v, want ModeList", p.Mode())
	}
}

func TestWithTasksFiltersInvalid(t *testing.T) {
	p := newTestPane().WithTasks([]Task{
		{Name: "ok", Command: "echo"},
		{Name: "no-cmd"},
		{Command: "echo"},
		{Name: "ok2", Command: "ls"},
	})
	if p.Count() != 2 {
		t.Fatalf("Count=%d, want 2", p.Count())
	}
	if sel, _ := p.Selected(); sel.Name != "ok" {
		t.Fatalf("Selected.Name=%q, want ok", sel.Name)
	}
}

func TestListNavigation(t *testing.T) {
	p := newTestPane().WithTasks([]Task{
		{Name: "a", Command: "x"},
		{Name: "b", Command: "x"},
		{Name: "c", Command: "x"},
	})

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.Cursor() != 1 {
		t.Fatalf("after down, cursor=%d", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if p.Cursor() != 2 {
		t.Fatalf("after end, cursor=%d", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.Cursor() != 2 {
		t.Fatalf("down past end, cursor=%d", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyHome})
	if p.Cursor() != 0 {
		t.Fatalf("after home, cursor=%d", p.Cursor())
	}
}

func TestEnterEmitsRunTaskMsg(t *testing.T) {
	p := newTestPane().WithTasks([]Task{
		{Name: "test", Command: "go", Args: []string{"test"}},
	})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return a Cmd")
	}
	msg := cmd()
	run, ok := msg.(RunTaskMsg)
	if !ok {
		t.Fatalf("got %T, want RunTaskMsg", msg)
	}
	if run.Task.Name != "test" {
		t.Fatalf("run.Task.Name=%q, want test", run.Task.Name)
	}
}

func TestEscInListEmitsCancel(t *testing.T) {
	p := newTestPane().WithTasks([]Task{{Name: "x", Command: "y"}})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should emit Cmd")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Fatalf("Esc should emit CancelMsg, got %T", cmd())
	}
}

func TestKeysIgnoredWhenBlurred(t *testing.T) {
	p := newTestPane().WithTasks([]Task{
		{Name: "a", Command: "x"},
		{Name: "b", Command: "x"},
	}).Blur()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.Cursor() != 0 {
		t.Fatalf("blurred pane should not move cursor; got %d", p.Cursor())
	}
}

func TestSwitchToOutputResetsState(t *testing.T) {
	t1 := Task{Name: "first", Command: "echo"}
	t2 := Task{Name: "second", Command: "echo"}
	p := newTestPane().WithTasks([]Task{t1, t2}).SwitchToOutput(t1, 42)
	if p.Mode() != ModeOutput {
		t.Fatalf("Mode after SwitchToOutput=%v", p.Mode())
	}
	if p.RunningID() != 42 {
		t.Fatalf("RunningID=%d, want 42", p.RunningID())
	}
	if r, live := p.Running(); r.Name != "first" || live {
		t.Fatalf("Running()=%+v live=%v", r, live)
	}
	if p.Exited() {
		t.Fatal("Exited should be false for a fresh run")
	}
}

func TestStartedMsgFlipsLive(t *testing.T) {
	p := newTestPane().SwitchToOutput(Task{Name: "t", Command: "x"}, 5)
	p, _ = p.Update(StartedMsg{RunID: 5, Task: Task{Name: "t"}, When: time.Now()})
	if _, live := p.Running(); !live {
		t.Fatal("StartedMsg should flip live=true")
	}
}

func TestLineMsgFromActiveRunAccumulates(t *testing.T) {
	p := newTestPane().SwitchToOutput(Task{Name: "t", Command: "x"}, 7)
	p, _ = p.Update(LineMsg{RunID: 7, Line: "alpha", Stream: StreamStdout})
	p, _ = p.Update(LineMsg{RunID: 7, Line: "beta", Stream: StreamStderr})
	if p.OutputLineCount() != 2 {
		t.Fatalf("OutputLineCount=%d, want 2", p.OutputLineCount())
	}
}

func TestLineMsgFromStaleRunIgnored(t *testing.T) {
	p := newTestPane().SwitchToOutput(Task{Name: "t", Command: "x"}, 7)
	p, _ = p.Update(LineMsg{RunID: 6, Line: "stale", Stream: StreamStdout})
	if p.OutputLineCount() != 0 {
		t.Fatalf("stale LineMsg should be dropped; got %d", p.OutputLineCount())
	}
}

func TestExitMsgFlipsExited(t *testing.T) {
	p := newTestPane().SwitchToOutput(Task{Name: "t", Command: "x"}, 9)
	p, _ = p.Update(ExitMsg{RunID: 9, ExitCode: 1, Duration: 100 * time.Millisecond})
	if !p.Exited() {
		t.Fatal("ExitMsg should flip Exited=true")
	}
	if p.ExitCode() != 1 {
		t.Fatalf("ExitCode=%d, want 1", p.ExitCode())
	}
	if _, live := p.Running(); live {
		t.Fatal("Running should report live=false after exit")
	}
}

func TestCtrlCInOutputModeEmitsKillWhileLive(t *testing.T) {
	p := newTestPane().SwitchToOutput(Task{Name: "t", Command: "x"}, 1)
	p, _ = p.Update(StartedMsg{RunID: 1, Task: Task{Name: "t"}})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Ctrl+C while live should emit KillMsg")
	}
	if _, ok := cmd().(KillMsg); !ok {
		t.Fatalf("Ctrl+C should emit KillMsg, got %T", cmd())
	}
}

func TestCtrlCAfterExitIsInert(t *testing.T) {
	p := newTestPane().SwitchToOutput(Task{Name: "t", Command: "x"}, 1)
	p, _ = p.Update(ExitMsg{RunID: 1, ExitCode: 0})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		t.Fatal("Ctrl+C after exit should be inert")
	}
}

func TestEscInOutputLiveEmitsCancel(t *testing.T) {
	p := newTestPane().SwitchToOutput(Task{Name: "t", Command: "x"}, 1)
	p, _ = p.Update(StartedMsg{RunID: 1, Task: Task{Name: "t"}})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc while live should emit Cmd")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Fatalf("Esc while live should emit CancelMsg, got %T", cmd())
	}
}

func TestEscInOutputExitedReturnsToList(t *testing.T) {
	p := newTestPane().SwitchToOutput(Task{Name: "t", Command: "x"}, 1)
	p, _ = p.Update(ExitMsg{RunID: 1, ExitCode: 0})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc after exit should emit a Cmd")
	}
	if _, ok := cmd().(BackToListMsg); !ok {
		t.Fatalf("Esc after exit should emit BackToListMsg, got %T", cmd())
	}
}

func TestBackToListMethodFlipsMode(t *testing.T) {
	p := newTestPane().SwitchToOutput(Task{Name: "t", Command: "x"}, 1).BackToList()
	if p.Mode() != ModeList {
		t.Fatalf("BackToList Mode=%v, want ModeList", p.Mode())
	}
}

func TestViewMentionsTasksHeader(t *testing.T) {
	p := newTestPane().WithTasks([]Task{{Name: "alpha", Command: "echo"}})
	out := p.View()
	if !strings.Contains(stripStyle(out), "tasks") {
		t.Fatalf("View should mention 'tasks'; got:\n%s", out)
	}
	if !strings.Contains(stripStyle(out), "alpha") {
		t.Fatalf("View should mention task name; got:\n%s", out)
	}
}

func TestViewEmptyListShowsHint(t *testing.T) {
	p := newTestPane().WithTasks(nil)
	out := stripStyle(p.View())
	if !strings.Contains(out, "no tasks defined") {
		t.Fatalf("empty view should hint creating tasks.toml; got:\n%s", out)
	}
}

func TestViewOutputModeShowsTaskName(t *testing.T) {
	p := newTestPane().SwitchToOutput(Task{Name: "go test", Command: "go"}, 1)
	p, _ = p.Update(LineMsg{RunID: 1, Line: "running stuff", Stream: StreamStdout})
	out := stripStyle(p.View())
	if !strings.Contains(out, "go test") {
		t.Fatalf("output mode should show task name; got:\n%s", out)
	}
	if !strings.Contains(out, "running stuff") {
		t.Fatalf("output mode should include streamed line; got:\n%s", out)
	}
}

func TestWithLoadErrorSurfacesInHeader(t *testing.T) {
	p := newTestPane().WithTasks([]Task{{Name: "x", Command: "y"}}).WithLoadError(errors.New("boom"))
	out := stripStyle(p.View())
	if !strings.Contains(out, "config error: boom") {
		t.Fatalf("load error should surface in view; got:\n%s", out)
	}
}
