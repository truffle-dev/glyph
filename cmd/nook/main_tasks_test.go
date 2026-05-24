package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/cmd/nook/internal/tasks"
)

func TestAltTOpensTasksOverlay(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModel(root)
	m.width = 120
	m.height = 30
	m = m.resize()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}, Alt: true})
	mm := updated.(model)
	if mm.overlay != overlayTasks {
		t.Fatalf("alt+t did not open tasks overlay; got overlay=%v", mm.overlay)
	}
	if !mm.tasksPane.IsFocused() {
		t.Errorf("alt+t did not focus the tasks pane")
	}
	if mm.tasksPane.Count() != 4 {
		t.Errorf("expected 4 Go defaults, got %d", mm.tasksPane.Count())
	}
	if mm.tasksPane.Mode() != tasks.ModeList {
		t.Errorf("tasks pane should open in list mode, got %v", mm.tasksPane.Mode())
	}
}

func TestTasksOverlayRoutesEscToCancel(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModel(root)
	m.width = 100
	m.height = 24
	m = m.resize()
	m.overlay = overlayTasks
	m.tasksPane = m.tasksPane.WithTasks([]tasks.Task{{Name: "x", Command: "y"}}).Focus()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc on tasks overlay returned nil cmd")
	}
	if _, ok := cmd().(tasks.CancelMsg); !ok {
		t.Fatalf("Esc on tasks overlay produced %T, want CancelMsg", cmd())
	}
}

func TestTasksCancelMsgClosesOverlay(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	m.overlay = overlayTasks
	m.tasksPane = m.tasksPane.WithTasks([]tasks.Task{{Name: "x", Command: "y"}}).Focus()

	updated, _ := m.Update(tasks.CancelMsg{})
	mm := updated.(model)
	if mm.overlay != overlayNone {
		t.Errorf("CancelMsg should clear overlay; got %v", mm.overlay)
	}
	if mm.tasksPane.IsFocused() {
		t.Errorf("CancelMsg should blur the tasks pane")
	}
}

func TestTasksRunTaskMsgSpawnsRunner(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	m.overlay = overlayTasks
	m.tasksPane = m.tasksPane.WithTasks([]tasks.Task{{Name: "echo", Command: "echo", Args: []string{"hi"}}}).Focus()

	updated, cmd := m.Update(tasks.RunTaskMsg{Task: tasks.Task{Name: "echo", Command: "echo", Args: []string{"hi"}}})
	mm := updated.(model)
	if mm.activeRunner == nil {
		t.Fatal("RunTaskMsg should populate activeRunner")
	}
	if mm.tasksPane.Mode() != tasks.ModeOutput {
		t.Errorf("after RunTaskMsg pane.Mode=%v, want ModeOutput", mm.tasksPane.Mode())
	}
	if cmd == nil {
		t.Fatal("RunTaskMsg should batch streaming Cmds")
	}
	// Drain the runner to completion so the test cleans up.
	deadline := time.After(3 * time.Second)
	for !mm.tasksPane.Exited() {
		select {
		case <-deadline:
			t.Fatal("runner did not exit within 3s")
		case <-mm.activeRunner.Done():
		}
		// Pump remaining messages by reading from the runner channels.
		next := mm.activeRunner.WaitCmd()
		msg := next()
		exit, ok := msg.(tasks.ExitMsg)
		if !ok {
			continue
		}
		u, _ := mm.Update(exit)
		mm = u.(model)
	}
}

func TestTasksKillMsgKillsActiveRunner(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	m.overlay = overlayTasks
	m.tasksPane = m.tasksPane.Focus()

	// Spawn a slow task so we can observe the kill.
	updated, _ := m.Update(tasks.RunTaskMsg{Task: tasks.Task{
		Name:    "sleep",
		Command: "sh",
		Args:    []string{"-c", "sleep 10"},
	}})
	mm := updated.(model)
	if mm.activeRunner == nil {
		t.Fatal("RunTaskMsg should populate activeRunner")
	}
	runner := mm.activeRunner

	// KillMsg should send a kill signal but keep the runner pointer
	// (so the pane can still display the exit summary).
	mm.Update(tasks.KillMsg{})
	select {
	case <-runner.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("KillMsg did not terminate runner within 3s")
	}
}

func TestTasksCancelMsgKillsRunningTask(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	m.overlay = overlayTasks
	m.tasksPane = m.tasksPane.Focus()

	updated, _ := m.Update(tasks.RunTaskMsg{Task: tasks.Task{
		Name:    "sleep",
		Command: "sh",
		Args:    []string{"-c", "sleep 10"},
	}})
	mm := updated.(model)
	runner := mm.activeRunner
	if runner == nil {
		t.Fatal("RunTaskMsg should populate activeRunner")
	}

	updated2, _ := mm.Update(tasks.CancelMsg{})
	mm2 := updated2.(model)
	if mm2.activeRunner != nil {
		t.Errorf("CancelMsg should clear activeRunner")
	}
	if mm2.overlay != overlayNone {
		t.Errorf("CancelMsg should close overlay; got %v", mm2.overlay)
	}
	select {
	case <-runner.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("CancelMsg did not terminate runner within 3s")
	}
}

func TestTasksBackToListResetsPane(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	m.overlay = overlayTasks
	m.tasksPane = m.tasksPane.WithTasks([]tasks.Task{{Name: "x", Command: "y"}}).Focus()
	m.tasksPane = m.tasksPane.SwitchToOutput(tasks.Task{Name: "x", Command: "y"}, 1)

	updated, _ := m.Update(tasks.BackToListMsg{})
	mm := updated.(model)
	if mm.tasksPane.Mode() != tasks.ModeList {
		t.Errorf("BackToListMsg should flip Mode to ModeList; got %v", mm.tasksPane.Mode())
	}
	if mm.activeRunner != nil {
		t.Errorf("BackToListMsg should clear activeRunner")
	}
}

func TestTasksViewIncludesTaskName(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 100
	m.height = 24
	m = m.resize()
	m.overlay = overlayTasks
	m.tasksPane = m.tasksPane.WithTasks([]tasks.Task{{Name: "go test", Command: "go", Args: []string{"test"}}}).Focus()

	out := m.View()
	if !strings.Contains(out, "go test") {
		t.Errorf("tasks overlay view should contain the task name; got:\n%s", out)
	}
}
