package tasks

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Mode is the pane's current display state. The pane starts in
// ModeList (picker over Tasks) and switches to ModeOutput when a task
// is launched.
type Mode int

const (
	// ModeList shows the task picker.
	ModeList Mode = iota
	// ModeOutput shows the running task's streamed output.
	ModeOutput
)

// RunTaskMsg is emitted when the user accepts a row in list mode. The
// host responds by spawning a Runner and feeding StartedMsg + LineMsg +
// ExitMsg back to the pane via Update.
type RunTaskMsg struct{ Task Task }

// CancelMsg is emitted when the user presses Esc. The host closes the
// overlay; if a task is running, the host also kills it.
type CancelMsg struct{}

// KillMsg is emitted when the user presses Ctrl+C in ModeOutput. The
// host kills the active runner but keeps the pane open so the exit
// summary is visible.
type KillMsg struct{}

// BackToListMsg is emitted when the user presses Esc in ModeOutput
// while no task is running. The host returns the pane to ModeList.
type BackToListMsg struct{}

// Pane is the tasks overlay UI: list mode + output mode in one widget.
type Pane struct {
	theme theme.Theme
	root  string

	mode    Mode
	tasks   []Task
	cursor  int
	focused bool

	width  int
	height int

	// running is the task currently being supervised; zero-value when
	// nothing is running. ExitCode is -1 while live.
	running     Task
	runningID   int
	runningLive bool
	startedAt   time.Time

	// output is the rolling buffer of streamed lines for the active
	// run. Drops old lines once the buffer exceeds 4 KLines so we
	// never grow without bound.
	output    []LineMsg
	outOffset int // 0 = stuck to bottom; positive = lines scrolled up

	exitCode     int
	exitErr      error
	exitDuration time.Duration
	exited       bool

	loadErr error
}

// NewPane returns a Pane with the given theme and workspace root.
// Default size 80x20; call WithSize before View().
func NewPane(t theme.Theme, root string) Pane {
	return Pane{theme: t, root: root, width: 80, height: 20, mode: ModeList}
}

// WithSize sets the overlay dimensions.
func (p Pane) WithSize(w, h int) Pane {
	if w < 40 {
		w = 40
	}
	if h < 8 {
		h = 8
	}
	p.width = w
	p.height = h
	return p
}

// SetTheme swaps the palette used for the picker list, output streaming
// viewport, and stdout/stderr tags. Next View() picks up the new colors.
func (p Pane) SetTheme(t theme.Theme) Pane { p.theme = t; return p }

// WithTasks replaces the picker list and clamps the cursor.
func (p Pane) WithTasks(ts []Task) Pane {
	// Filter out invalid tasks so the picker can't show them.
	valid := make([]Task, 0, len(ts))
	for _, t := range ts {
		if t.IsValid() {
			valid = append(valid, t)
		}
	}
	p.tasks = valid
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(valid) {
		if len(valid) == 0 {
			p.cursor = 0
		} else {
			p.cursor = len(valid) - 1
		}
	}
	return p
}

// WithLoadError attaches a non-fatal config-load error to the pane so
// it surfaces in the header. Use when LoadOrDefaults returns a parse
// error but you still have fallback defaults to show.
func (p Pane) WithLoadError(err error) Pane {
	p.loadErr = err
	return p
}

// Focus marks the pane as accepting key input.
func (p Pane) Focus() Pane { p.focused = true; return p }

// Blur releases focus.
func (p Pane) Blur() Pane { p.focused = false; return p }

// IsFocused reports whether the pane currently consumes key input.
func (p Pane) IsFocused() bool { return p.focused }

// Mode returns the current display mode.
func (p Pane) Mode() Mode { return p.mode }

// Cursor returns the picker cursor row, for tests.
func (p Pane) Cursor() int { return p.cursor }

// Count returns the number of tasks visible to the picker.
func (p Pane) Count() int { return len(p.tasks) }

// Selected returns the task under the picker cursor.
func (p Pane) Selected() (Task, bool) {
	if len(p.tasks) == 0 {
		return Task{}, false
	}
	if p.cursor < 0 || p.cursor >= len(p.tasks) {
		return Task{}, false
	}
	return p.tasks[p.cursor], true
}

// Running reports the currently-supervised task and whether it is
// still live (process not yet exited).
func (p Pane) Running() (Task, bool) {
	if p.runningID == 0 {
		return Task{}, false
	}
	return p.running, p.runningLive
}

// RunningID returns the current run ID (0 when nothing has been
// launched yet).
func (p Pane) RunningID() int { return p.runningID }

// OutputLineCount returns the number of buffered output lines for the
// active run. Used by tests.
func (p Pane) OutputLineCount() int { return len(p.output) }

// Exited reports whether the active run has finished. False before the
// first run and while the process is live; true after ExitMsg arrives.
func (p Pane) Exited() bool { return p.exited }

// ExitCode returns the exit code of the most recent finished run.
func (p Pane) ExitCode() int { return p.exitCode }

// Update handles key input and runner messages.
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case StartedMsg:
		if msg.RunID == p.runningID {
			p.runningLive = true
			p.startedAt = msg.When
		}
		return p, nil
	case LineMsg:
		// Discard messages from stale runs (e.g. user killed and
		// relaunched faster than the goroutine drained).
		if msg.RunID != p.runningID {
			return p, nil
		}
		p.output = append(p.output, msg)
		if len(p.output) > 4000 {
			drop := len(p.output) - 4000
			p.output = p.output[drop:]
		}
		return p, nil
	case ExitMsg:
		if msg.RunID != p.runningID {
			return p, nil
		}
		p.runningLive = false
		p.exited = true
		p.exitCode = msg.ExitCode
		p.exitErr = msg.Err
		p.exitDuration = msg.Duration
		return p, nil
	case tea.KeyMsg:
		if !p.focused {
			return p, nil
		}
		if p.mode == ModeList {
			return p.updateList(msg)
		}
		return p.updateOutput(msg)
	}
	return p, nil
}

func (p Pane) updateList(km tea.KeyMsg) (Pane, tea.Cmd) {
	switch km.Type {
	case tea.KeyEsc:
		return p, func() tea.Msg { return CancelMsg{} }
	case tea.KeyEnter:
		t, ok := p.Selected()
		if !ok {
			return p, nil
		}
		return p, func() tea.Msg { return RunTaskMsg{Task: t} }
	case tea.KeyUp:
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case tea.KeyDown:
		if p.cursor < len(p.tasks)-1 {
			p.cursor++
		}
		return p, nil
	case tea.KeyHome:
		p.cursor = 0
		return p, nil
	case tea.KeyEnd:
		if len(p.tasks) > 0 {
			p.cursor = len(p.tasks) - 1
		}
		return p, nil
	}
	return p, nil
}

func (p Pane) updateOutput(km tea.KeyMsg) (Pane, tea.Cmd) {
	switch km.Type {
	case tea.KeyEsc:
		if p.runningLive {
			return p, func() tea.Msg { return CancelMsg{} }
		}
		// Process already exited: Esc returns to the list view.
		return p, func() tea.Msg { return BackToListMsg{} }
	case tea.KeyCtrlC:
		if p.runningLive {
			return p, func() tea.Msg { return KillMsg{} }
		}
		return p, nil
	case tea.KeyUp:
		p.outOffset++
		return p, nil
	case tea.KeyDown:
		if p.outOffset > 0 {
			p.outOffset--
		}
		return p, nil
	case tea.KeyPgUp:
		p.outOffset += p.visibleOutputRows()
		return p, nil
	case tea.KeyPgDown:
		p.outOffset -= p.visibleOutputRows()
		if p.outOffset < 0 {
			p.outOffset = 0
		}
		return p, nil
	case tea.KeyHome:
		p.outOffset = 0
		return p, nil
	case tea.KeyEnd:
		if len(p.output) > p.visibleOutputRows() {
			p.outOffset = len(p.output) - p.visibleOutputRows()
		}
		return p, nil
	}
	return p, nil
}

// SwitchToOutput puts the pane into output mode and resets the
// run-state to a fresh runID. Call after the host has spawned a Runner.
func (p Pane) SwitchToOutput(t Task, runID int) Pane {
	p.mode = ModeOutput
	p.running = t
	p.runningID = runID
	p.runningLive = false // becomes true on StartedMsg
	p.output = p.output[:0]
	p.outOffset = 0
	p.exited = false
	p.exitCode = -1
	p.exitErr = nil
	p.exitDuration = 0
	return p
}

// BackToList returns the pane to picker mode. The output buffer is kept
// in memory so the user can scroll back; ResetForNewRun clears it.
func (p Pane) BackToList() Pane {
	p.mode = ModeList
	return p
}

// View renders the bordered card.
func (p Pane) View() string {
	body := p.body()
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.theme.Border).
		Background(p.theme.Surface).
		Padding(0, 1).
		Width(p.width - 2)
	return border.Render(body)
}

func (p Pane) body() string {
	innerWidth := p.width - 4
	if innerWidth < 30 {
		innerWidth = 30
	}
	if p.mode == ModeList {
		return p.bodyList(innerWidth)
	}
	return p.bodyOutput(innerWidth)
}

func (p Pane) bodyList(innerWidth int) string {
	header := p.renderListHeader(innerWidth)
	if p.loadErr != nil {
		warn := lipgloss.NewStyle().
			Foreground(p.theme.Warning).
			Render(truncateCells("config error: "+p.loadErr.Error(), innerWidth))
		header = header + "\n" + warn
	}
	if len(p.tasks) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(p.theme.TextMuted).
			Italic(true).
			Render("no tasks defined. create .nook/tasks.toml in this project.")
		return header + "\n\n" + empty
	}
	visible := p.visibleListRows()
	start := p.cursor - visible/2
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(p.tasks) {
		end = len(p.tasks)
		start = end - visible
		if start < 0 {
			start = 0
		}
	}
	lines := []string{header, ""}
	for i := start; i < end; i++ {
		lines = append(lines, p.renderTaskRow(i, innerWidth))
	}
	return strings.Join(lines, "\n")
}

func (p Pane) bodyOutput(innerWidth int) string {
	header := p.renderOutputHeader(innerWidth)
	visible := p.visibleOutputRows()
	if len(p.output) == 0 {
		var msg string
		if p.runningLive {
			msg = "task is starting, no output yet…"
		} else if p.exited {
			msg = "task produced no output."
		} else {
			msg = "no output yet."
		}
		body := lipgloss.NewStyle().
			Foreground(p.theme.TextMuted).
			Italic(true).
			Render(msg)
		return header + "\n\n" + body
	}
	end := len(p.output) - p.outOffset
	if end < 1 {
		end = 1
	}
	start := end - visible
	if start < 0 {
		start = 0
	}
	if end > len(p.output) {
		end = len(p.output)
	}
	lines := []string{header, ""}
	for i := start; i < end; i++ {
		lines = append(lines, p.renderOutputRow(p.output[i], innerWidth))
	}
	return strings.Join(lines, "\n")
}

func (p Pane) renderListHeader(width int) string {
	title := lipgloss.NewStyle().
		Foreground(p.theme.Primary).
		Bold(true).
		Render("tasks")
	hint := lipgloss.NewStyle().
		Foreground(p.theme.TextMuted).
		Render(fmt.Sprintf("%d available  ·  enter to run  ·  esc to close", len(p.tasks)))
	gap := width - lipgloss.Width(title) - lipgloss.Width(hint)
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + hint
}

func (p Pane) renderTaskRow(idx, width int) string {
	t := p.tasks[idx]
	mark := lipgloss.NewStyle().
		Foreground(p.theme.Primary).
		Bold(true).
		Render("▸")
	name := lipgloss.NewStyle().
		Foreground(p.theme.Text).
		Bold(true).
		Render(t.Name)
	cmdLine := t.Command
	if len(t.Args) > 0 {
		cmdLine += " " + strings.Join(t.Args, " ")
	}
	cmd := lipgloss.NewStyle().
		Foreground(p.theme.TextMuted).
		Render("  " + truncateCells(cmdLine, width-lipgloss.Width(name)-6))
	row := mark + " " + name + cmd
	if idx == p.cursor {
		row = lipgloss.NewStyle().
			Background(p.theme.SurfaceStrong).
			Foreground(p.theme.Text).
			Bold(true).
			Width(width).
			Render(stripStyle(row))
	}
	return row
}

func (p Pane) renderOutputHeader(width int) string {
	left := lipgloss.NewStyle().
		Foreground(p.theme.Primary).
		Bold(true).
		Render(p.running.Name)
	var right string
	switch {
	case p.exited && p.exitCode == 0:
		right = lipgloss.NewStyle().
			Foreground(p.theme.Success).
			Render(fmt.Sprintf("exit 0  in %s  ·  esc returns to tasks", p.exitDuration.Round(time.Millisecond)))
	case p.exited:
		right = lipgloss.NewStyle().
			Foreground(p.theme.Error).
			Render(fmt.Sprintf("exit %d  in %s  ·  esc returns to tasks", p.exitCode, p.exitDuration.Round(time.Millisecond)))
	case p.runningLive:
		right = lipgloss.NewStyle().
			Foreground(p.theme.Info).
			Render("running  ·  ctrl+c to kill  ·  esc to close")
	default:
		right = lipgloss.NewStyle().
			Foreground(p.theme.TextMuted).
			Render("starting…")
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (p Pane) renderOutputRow(line LineMsg, width int) string {
	var color lipgloss.Color
	switch line.Stream {
	case StreamStderr:
		color = p.theme.Warning
	case StreamSystem:
		color = p.theme.TextMuted
	default:
		color = p.theme.Text
	}
	text := truncateCells(line.Line, width)
	return lipgloss.NewStyle().Foreground(color).Render(text)
}

func (p Pane) visibleListRows() int {
	rows := p.height - 4
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (p Pane) visibleOutputRows() int {
	rows := p.height - 4
	if rows < 1 {
		rows = 1
	}
	return rows
}

// truncateCells truncates a styled string to at most n display cells.
func truncateCells(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	plain := stripStyle(s)
	if len(plain) <= n {
		return plain
	}
	if n == 1 {
		return "…"
	}
	return plain[:n-1] + "…"
}

// stripStyle removes ANSI CSI sequences. Cheaper than a regex; inputs
// here are short rendered cells.
func stripStyle(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
