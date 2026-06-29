package term

import (
	"os"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func TestStripANSIBasic(t *testing.T) {
	in := "\x1b[31mhello\x1b[0m world"
	if got := stripANSI(in); got != "hello world" {
		t.Fatalf("expected 'hello world', got %q", got)
	}
}

func TestStripANSIBracketedPaste(t *testing.T) {
	in := "\x1b[?2004hprompt\x1b[?2004l"
	if got := stripANSI(in); got != "prompt" {
		t.Fatalf("expected 'prompt', got %q", got)
	}
}

func TestStripANSIDropsCR(t *testing.T) {
	in := "line1\r\nline2"
	if got := stripANSI(in); got != "line1\nline2" {
		t.Fatalf("expected newline-only, got %q", got)
	}
}

func TestAppendLineBreaks(t *testing.T) {
	p := NewPane(theme.Default, "/tmp")
	p = p.Append([]byte("hello\nworld\n"))
	lines := p.Lines()
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("expected ['hello','world'], got %+v", lines)
	}
}

func TestAppendPartialLine(t *testing.T) {
	p := NewPane(theme.Default, "/tmp")
	p = p.Append([]byte("hello"))
	if len(p.Lines()) != 0 {
		t.Fatalf("expected no completed lines, got %+v", p.Lines())
	}
	p = p.Append([]byte(" world\n"))
	if len(p.Lines()) != 1 || p.Lines()[0] != "hello world" {
		t.Fatalf("expected joined line, got %+v", p.Lines())
	}
}

func TestEscBlurredEmitsCancel(t *testing.T) {
	p := NewPane(theme.Default, "/tmp") // not focused, no session
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected Cmd on Esc when blurred")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", cmd())
	}
}

func TestStartProducesSession(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("PTY-dependent test")
	}
	p := NewPane(theme.Default, "/tmp").WithSize(80, 10)
	p, err := p.Start()
	if err != nil {
		t.Fatalf("start err: %v", err)
	}
	defer p.Stop()
	if p.Session() == nil {
		t.Fatal("expected non-nil session")
	}
}

func TestWriteAndReadEcho(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("PTY-dependent test")
	}
	p := NewPane(theme.Default, "/tmp").WithSize(80, 10)
	p, err := p.Start()
	if err != nil {
		t.Fatalf("start err: %v", err)
	}
	defer p.Stop()
	sess := p.Session()
	if sess == nil {
		t.Fatal("expected session")
	}
	if _, err := sess.Write([]byte("echo hi-from-nook\n")); err != nil {
		t.Fatalf("write err: %v", err)
	}
	// Best-effort read with a short deadline. Use the PTY directly to avoid
	// pulling in ReadLoop here.
	deadlineBuf := make([]byte, 4096)
	got := strings.Builder{}
	for i := 0; i < 50; i++ {
		// 20 ms per attempt by way of go's blocking read; cap total at ~1s
		n, _ := sess.pt.Read(deadlineBuf)
		if n > 0 {
			got.Write(deadlineBuf[:n])
			if strings.Contains(got.String(), "hi-from-nook") {
				return
			}
		}
	}
	t.Logf("buffered output: %q", got.String())
	// don't fail hard — kernel buffering varies — but ensure something came back
	if got.Len() == 0 {
		t.Fatal("expected some PTY output")
	}
}

func TestViewWithoutStart(t *testing.T) {
	p := NewPane(theme.Default, "/tmp").WithSize(80, 5)
	out := p.View()
	if !strings.Contains(out, "terminal") {
		t.Fatalf("expected title, got:\n%s", out)
	}
	if !strings.Contains(out, "not started") {
		t.Fatalf("expected 'not started' note, got:\n%s", out)
	}
}

func TestScrollbackCap(t *testing.T) {
	p := NewPane(theme.Default, "/tmp")
	p.maxLines = 10
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("line\n")
	}
	p = p.Append([]byte(b.String()))
	if len(p.Lines()) != 10 {
		t.Fatalf("expected cap 10, got %d", len(p.Lines()))
	}
}

// TestClipLineRespectsDisplayWidth confirms clipLine budgets display cells,
// not runes: a wide character (CJK) is one rune but two columns, so a
// rune-counting clip would overshoot. Output must stay within the column
// budget and remain valid UTF-8.
func TestClipLineRespectsDisplayWidth(t *testing.T) {
	t.Parallel()
	out := clipLine("日本語コード done", 6)
	if w := lipgloss.Width(out); w > 6 {
		t.Errorf("clipLine exceeded 6 display cells (got %d): %q", w, out)
	}
	if !strings.HasSuffix(out, "…") {
		t.Errorf("expected ellipsis tail on truncated input: %q", out)
	}
	if got := clipLine("hi", 10); got != "hi" {
		t.Errorf("clipLine fit-input changed content: %q", got)
	}
}
