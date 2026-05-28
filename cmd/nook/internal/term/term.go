// Package term embeds a PTY-backed shell pane inside nook.
//
// The Pane spawns $SHELL (or sh) inside a PTY, reads stdout, and renders a
// scrollback buffer with a cursor on the last line. Input is forwarded to the
// PTY when the pane is focused. The renderer is intentionally simple: it
// preserves bytes verbatim (so colors, prompt sequences, and bell pass
// through) and tracks a flat scrollback of decoded lines.
//
// Not a terminal emulator. We don't interpret cursor-positioning, scrollback
// regions, or alternate-screen mode. Commands that need a full emulator
// (vim, htop, less) will still draw, but layout assumptions may break. For
// nook's MVP this is acceptable: it covers `git push`, `go test`, `npm run`,
// REPLs, file operations.
package term

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// OutputMsg carries a chunk of bytes from the PTY.
type OutputMsg struct {
	Data []byte
}

// ExitMsg is emitted when the shell process exits.
type ExitMsg struct {
	Err error
}

// CancelMsg is emitted on Esc-when-blurred or explicit Quit.
type CancelMsg struct{}

// Session is a live PTY-backed shell. Use New to construct; call Close to
// terminate.
type Session struct {
	pt   *os.File
	cmd  *exec.Cmd
	once sync.Once
}

// New spawns a shell in a PTY rooted at cwd. The caller owns the returned
// Session and must Close it on exit.
func New(cwd string) (*Session, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell, "-i")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	pt, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	return &Session{pt: pt, cmd: cmd}, nil
}

// Resize sends a winsize ioctl to the PTY.
func (s *Session) Resize(cols, rows uint16) error {
	if s == nil || s.pt == nil {
		return errors.New("nil session")
	}
	return pty.Setsize(s.pt, &pty.Winsize{Cols: cols, Rows: rows})
}

// Write forwards bytes to the PTY.
func (s *Session) Write(b []byte) (int, error) {
	if s == nil || s.pt == nil {
		return 0, errors.New("nil session")
	}
	return s.pt.Write(b)
}

// Close terminates the session and releases the PTY.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	var err error
	s.once.Do(func() {
		if s.cmd != nil && s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		if s.pt != nil {
			err = s.pt.Close()
		}
	})
	return err
}

// ReadLoop pumps bytes from the PTY into a channel until ctx is cancelled or
// the process exits. Closes the channel on exit. Suitable for wrapping into a
// recursive tea.Cmd via Program.Send.
func (s *Session) ReadLoop(ctx context.Context, out chan<- []byte) error {
	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, err := s.pt.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			select {
			case out <- chunk:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// Pane is the terminal UI model.
type Pane struct {
	theme    theme.Theme
	root     string
	session  *Session
	lines    []string
	current  []byte
	width    int
	height   int
	focused  bool
	exitErr  error
	maxLines int
}

// NewPane constructs an unstarted terminal pane.
func NewPane(t theme.Theme, root string) Pane {
	return Pane{theme: t, root: root, width: 80, height: 20, maxLines: 5000}
}

// WithSize sets pane dimensions.
func (p Pane) WithSize(w, h int) Pane {
	p.width = w
	p.height = h
	if p.session != nil {
		_ = p.session.Resize(uint16(w), uint16(h-1))
	}
	return p
}

// SetTheme swaps the palette used for the surrounding box, status row, and
// fallback line styling. The shell child's own ANSI escapes are untouched.
func (p Pane) SetTheme(t theme.Theme) Pane { p.theme = t; return p }

// Start spawns the shell. Idempotent.
func (p Pane) Start() (Pane, error) {
	if p.session != nil {
		return p, nil
	}
	s, err := New(p.root)
	if err != nil {
		return p, err
	}
	p.session = s
	_ = s.Resize(uint16(p.width), uint16(p.height-1))
	return p, nil
}

// Session returns the underlying PTY session (for ReadLoop wiring).
func (p Pane) Session() *Session { return p.session }

// Stop kills the shell process.
func (p Pane) Stop() Pane {
	if p.session != nil {
		_ = p.session.Close()
		p.session = nil
	}
	return p
}

// Focused reports whether the pane has keyboard focus.
func (p Pane) Focused() bool { return p.focused }

// Focus sets focused=true.
func (p Pane) Focus() Pane { p.focused = true; return p }

// Blur sets focused=false.
func (p Pane) Blur() Pane { p.focused = false; return p }

// Lines returns the decoded scrollback.
func (p Pane) Lines() []string { return p.lines }

// Append decodes bytes from the PTY into scrollback. Strips most ANSI escapes
// for the MVP renderer; future versions can preserve color/style.
func (p Pane) Append(b []byte) Pane {
	p.current = append(p.current, b...)
	// split on \n while keeping the partial last line
	for {
		idx := indexByte(p.current, '\n')
		if idx < 0 {
			break
		}
		line := stripANSI(string(p.current[:idx]))
		line = strings.TrimRight(line, "\r")
		p.lines = append(p.lines, line)
		p.current = p.current[idx+1:]
	}
	if len(p.lines) > p.maxLines {
		drop := len(p.lines) - p.maxLines
		p.lines = p.lines[drop:]
	}
	return p
}

// MarkExit records process exit.
func (p Pane) MarkExit(err error) Pane {
	p.exitErr = err
	return p
}

// Update routes keys.
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch m := msg.(type) {
	case OutputMsg:
		p = p.Append(m.Data)
		return p, nil
	case ExitMsg:
		p = p.MarkExit(m.Err)
		return p, nil
	case tea.KeyMsg:
		if !p.focused || p.session == nil {
			if m.Type == tea.KeyEsc {
				return p, func() tea.Msg { return CancelMsg{} }
			}
			return p, nil
		}
		return p.forwardKey(m), nil
	}
	return p, nil
}

func (p Pane) forwardKey(km tea.KeyMsg) Pane {
	if p.session == nil {
		return p
	}
	var b []byte
	switch km.Type {
	case tea.KeyEnter:
		b = []byte{'\r'}
	case tea.KeyBackspace:
		b = []byte{0x7f}
	case tea.KeyTab:
		b = []byte{'\t'}
	case tea.KeyUp:
		b = []byte{0x1b, '[', 'A'}
	case tea.KeyDown:
		b = []byte{0x1b, '[', 'B'}
	case tea.KeyRight:
		b = []byte{0x1b, '[', 'C'}
	case tea.KeyLeft:
		b = []byte{0x1b, '[', 'D'}
	case tea.KeyEsc:
		b = []byte{0x1b}
	case tea.KeyCtrlC:
		b = []byte{0x03}
	case tea.KeyCtrlD:
		b = []byte{0x04}
	case tea.KeyCtrlL:
		b = []byte{0x0c}
	case tea.KeySpace:
		b = []byte{' '}
	case tea.KeyRunes:
		b = []byte(string(km.Runes))
	}
	if len(b) > 0 {
		_, _ = p.session.Write(b)
	}
	return p
}

// View renders the pane.
func (p Pane) View() string {
	t := p.theme
	title := lipgloss.NewStyle().Foreground(t.TextMuted).Bold(true).Render("terminal")
	muted := lipgloss.NewStyle().Foreground(t.TextMuted)
	if p.session == nil {
		title += "  " + muted.Render("(not started — press i to start)")
	} else if p.exitErr != nil {
		title += "  " + lipgloss.NewStyle().Foreground(t.Error).Render("exited: "+p.exitErr.Error())
	} else if p.focused {
		title += "  " + muted.Render("focus: input goes to shell")
	}

	bodyH := p.height - 1
	if bodyH < 1 {
		bodyH = 1
	}
	start := 0
	if len(p.lines) > bodyH {
		start = len(p.lines) - bodyH
	}
	rows := make([]string, 0, bodyH)
	for i := start; i < len(p.lines); i++ {
		rows = append(rows, clipLine(p.lines[i], p.width))
	}
	if len(p.current) > 0 && len(rows) < bodyH {
		rows = append(rows, clipLine(stripANSI(string(p.current)), p.width))
	}
	for len(rows) < bodyH {
		rows = append(rows, "")
	}
	return strings.Join(append([]string{title}, rows...), "\n")
}

// --- helpers ---

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

// stripANSI removes CSI sequences (e.g. \x1b[31m) and a few common control
// codes. It keeps printable ASCII, tabs, and UTF-8 multibyte runes. This is
// not a full VT100 emulator — it's a best-effort cleaner for the scrollback
// renderer.
func stripANSI(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	i := 0
	r := []byte(s)
	for i < len(r) {
		b := r[i]
		if b == 0x1b && i+1 < len(r) {
			// CSI: ESC [ ... <final byte 0x40-0x7e>
			if r[i+1] == '[' {
				j := i + 2
				for j < len(r) && (r[j] < 0x40 || r[j] > 0x7e) {
					j++
				}
				if j < len(r) {
					i = j + 1
					continue
				}
				i = len(r)
				continue
			}
			// OSC: ESC ] ... BEL or ESC \
			if r[i+1] == ']' {
				j := i + 2
				for j < len(r) && r[j] != 0x07 && !(r[j] == 0x1b && j+1 < len(r) && r[j+1] == '\\') {
					j++
				}
				if j < len(r) && r[j] == 0x07 {
					i = j + 1
				} else if j < len(r) {
					i = j + 2
				} else {
					i = len(r)
				}
				continue
			}
			// 2-byte escape like ESC c, ESC =, ESC > — drop both
			i += 2
			continue
		}
		if b == '\r' {
			i++
			continue
		}
		if b < 0x20 && b != '\t' && b != '\n' {
			i++
			continue
		}
		out.WriteByte(b)
		i++
	}
	return out.String()
}

func clipLine(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return string(r[:w])
	}
	return string(r[:w-1]) + "…"
}
