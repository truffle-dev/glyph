package signature

import (
	"strings"
	"testing"

	"github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/components/theme"
)

func th() theme.Theme { return theme.Default }

func TestNewIsClosed(t *testing.T) {
	t.Parallel()
	p := New()
	if p.IsOpen() {
		t.Error("New() should be closed")
	}
	if p.View(th()) != "" {
		t.Error("View on closed pane should be empty")
	}
}

func TestOpenWithEmptySignaturesStaysClosed(t *testing.T) {
	t.Parallel()
	p := New().Open(lsp.SignatureInfo{ActiveSignature: -1})
	if p.IsOpen() {
		t.Error("empty signatures should leave pane closed")
	}
}

func TestOpenWithOneSignature(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{
				Label: "func Greet(name string)",
				Parameters: []lsp.SignatureParam{
					{Label: "name string", Start: 11, End: 22},
				},
				ActiveParameter: 0,
			},
		},
	}
	p := New().Open(info)
	if !p.IsOpen() {
		t.Fatal("Open with one signature should open")
	}
	sig := p.ActiveSignature()
	if sig.Label != "func Greet(name string)" {
		t.Errorf("Label: %q", sig.Label)
	}
}

func TestOpenClampsActiveSignatureOutOfRange(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 5,
		Signatures: []lsp.Signature{
			{Label: "a"}, {Label: "b"},
		},
	}
	p := New().Open(info)
	if p.Info().ActiveSignature != 0 {
		t.Errorf("ActiveSignature out of range should clamp to 0, got %d", p.Info().ActiveSignature)
	}
}

func TestOpenClampsNegativeActiveSignature(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: -2,
		Signatures: []lsp.Signature{
			{Label: "x"},
		},
	}
	p := New().Open(info)
	if p.Info().ActiveSignature != 0 {
		t.Errorf("negative ActiveSignature should clamp to 0, got %d", p.Info().ActiveSignature)
	}
}

func TestCloseClearsState(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures:      []lsp.Signature{{Label: "x"}},
	}
	p := New().Open(info).Close()
	if p.IsOpen() {
		t.Error("Close should leave pane closed")
	}
	if p.Info().ActiveSignature != -1 {
		t.Errorf("ActiveSignature after Close should be -1, got %d", p.Info().ActiveSignature)
	}
}

func TestWithSizeClampsToMin(t *testing.T) {
	t.Parallel()
	p := New().WithSize(5)
	if p.Width() != minWidth {
		t.Errorf("width should clamp to %d, got %d", minWidth, p.Width())
	}
}

func TestWithSizeAcceptsExplicitWidth(t *testing.T) {
	t.Parallel()
	p := New().WithSize(80)
	if p.Width() != 80 {
		t.Errorf("width: %d", p.Width())
	}
}

func TestViewRendersSignatureLabel(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{
				Label:           "func Greet(name string)",
				ActiveParameter: -1,
			},
		},
	}
	p := New().Open(info).WithSize(64)
	out := p.View(th())
	if out == "" {
		t.Fatal("View on open pane should be non-empty")
	}
	if !strings.Contains(out, "Greet") {
		t.Errorf("View missing label: %q", out)
	}
}

func TestViewWithSingleSignatureOmitsCounter(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{Label: "func A()"},
		},
	}
	out := New().Open(info).View(th())
	if strings.Contains(out, "of") {
		t.Errorf("View with single sig should not include overload counter: %q", out)
	}
}

func TestViewWithMultipleSignaturesShowsCounter(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 1,
		Signatures: []lsp.Signature{
			{Label: "func A()"},
			{Label: "func B()"},
			{Label: "func C()"},
		},
	}
	out := New().Open(info).View(th())
	if !strings.Contains(out, "2 of 3") {
		t.Errorf("counter missing: %q", out)
	}
}

func TestViewShowsParameterDoc(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{
				Label: "func Send(msg string)",
				Parameters: []lsp.SignatureParam{
					{Label: "msg string", Start: 10, End: 20, Doc: "the message body"},
				},
				ActiveParameter: 0,
			},
		},
	}
	out := New().Open(info).View(th())
	if !strings.Contains(out, "the message body") {
		t.Errorf("param doc missing: %q", out)
	}
}

func TestViewShowsSignatureDoc(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{
				Label: "func Open()",
				Doc:   "Open opens the resource.",
			},
		},
	}
	out := New().Open(info).View(th())
	if !strings.Contains(out, "Open opens the resource") {
		t.Errorf("sig doc missing: %q", out)
	}
}

func TestViewSuppressesDuplicateDocs(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{
				Label: "func Send(msg string)",
				Doc:   "the message body",
				Parameters: []lsp.SignatureParam{
					{Label: "msg string", Start: 10, End: 20, Doc: "the message body"},
				},
				ActiveParameter: 0,
			},
		},
	}
	out := New().Open(info).View(th())
	// Should appear exactly once.
	if strings.Count(out, "the message body") != 1 {
		t.Errorf("duplicate doc should be suppressed, got count=%d: %q",
			strings.Count(out, "the message body"), out)
	}
}

func TestViewWithNoActiveParameterStillRenders(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{
				Label:           "func Foo(a int)",
				ActiveParameter: -1,
				Parameters: []lsp.SignatureParam{
					{Label: "a int", Start: 9, End: 14},
				},
			},
		},
	}
	out := New().Open(info).View(th())
	if !strings.Contains(out, "Foo") {
		t.Errorf("View missing label without active param: %q", out)
	}
}

func TestViewClampsLongLabel(t *testing.T) {
	t.Parallel()
	long := "func Long(" + strings.Repeat("x", 200) + ")"
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures:      []lsp.Signature{{Label: long, ActiveParameter: -1}},
	}
	p := New().Open(info).WithSize(50)
	out := p.View(th())
	if !strings.Contains(out, "…") {
		t.Errorf("expected ellipsis on overflow, got: %q", out)
	}
}

func TestViewParameterOffsetsOutOfRangeFallsBackPlain(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{
				Label: "func Foo(a int)",
				Parameters: []lsp.SignatureParam{
					{Label: "bogus", Start: 50, End: 60},
				},
				ActiveParameter: 0,
			},
		},
	}
	out := New().Open(info).View(th())
	if !strings.Contains(out, "Foo") {
		t.Errorf("View missing label when offsets are out of range: %q", out)
	}
}

func TestNextOverloadAdvances(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{Label: "func A()"},
			{Label: "func B()"},
			{Label: "func C()"},
		},
	}
	p := New().Open(info).NextOverload()
	if p.Info().ActiveSignature != 1 {
		t.Errorf("NextOverload from 0 → %d, want 1", p.Info().ActiveSignature)
	}
}

func TestNextOverloadWrapsFromLast(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 2,
		Signatures: []lsp.Signature{
			{Label: "func A()"},
			{Label: "func B()"},
			{Label: "func C()"},
		},
	}
	p := New().Open(info).NextOverload()
	if p.Info().ActiveSignature != 0 {
		t.Errorf("NextOverload from last should wrap to 0, got %d", p.Info().ActiveSignature)
	}
}

func TestPrevOverloadSteps(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 2,
		Signatures: []lsp.Signature{
			{Label: "func A()"},
			{Label: "func B()"},
			{Label: "func C()"},
		},
	}
	p := New().Open(info).PrevOverload()
	if p.Info().ActiveSignature != 1 {
		t.Errorf("PrevOverload from 2 → %d, want 1", p.Info().ActiveSignature)
	}
}

func TestPrevOverloadWrapsFromFirst(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{Label: "func A()"},
			{Label: "func B()"},
		},
	}
	p := New().Open(info).PrevOverload()
	if p.Info().ActiveSignature != 1 {
		t.Errorf("PrevOverload from first should wrap to last, got %d", p.Info().ActiveSignature)
	}
}

func TestNextOverloadOnSingleSignatureIsNoOp(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures:      []lsp.Signature{{Label: "func A()"}},
	}
	p := New().Open(info).NextOverload()
	if p.Info().ActiveSignature != 0 {
		t.Errorf("NextOverload on single sig should stay at 0, got %d", p.Info().ActiveSignature)
	}
}

func TestPrevOverloadOnSingleSignatureIsNoOp(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures:      []lsp.Signature{{Label: "func A()"}},
	}
	p := New().Open(info).PrevOverload()
	if p.Info().ActiveSignature != 0 {
		t.Errorf("PrevOverload on single sig should stay at 0, got %d", p.Info().ActiveSignature)
	}
}

func TestNextOverloadOnClosedPaneIsNoOp(t *testing.T) {
	t.Parallel()
	p := New().NextOverload()
	if p.IsOpen() {
		t.Error("NextOverload should not open a closed pane")
	}
}

func TestPrevOverloadOnClosedPaneIsNoOp(t *testing.T) {
	t.Parallel()
	p := New().PrevOverload()
	if p.IsOpen() {
		t.Error("PrevOverload should not open a closed pane")
	}
}

func TestOpenResetsAfterManualCycle(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{Label: "func A()"},
			{Label: "func B()"},
		},
	}
	p := New().Open(info).NextOverload()
	if p.Info().ActiveSignature != 1 {
		t.Fatalf("setup: NextOverload should land at 1, got %d", p.Info().ActiveSignature)
	}
	// A fresh server response with the same ActiveSignature should win;
	// the manual selection doesn't persist across Open().
	p = p.Open(info)
	if p.Info().ActiveSignature != 0 {
		t.Errorf("Open after NextOverload should reset to server's ActiveSignature, got %d",
			p.Info().ActiveSignature)
	}
}

func TestNextOverloadActiveSignatureRendersAtNewIndex(t *testing.T) {
	t.Parallel()
	info := lsp.SignatureInfo{
		ActiveSignature: 0,
		Signatures: []lsp.Signature{
			{Label: "func A()"},
			{Label: "func B()"},
		},
	}
	p := New().Open(info).NextOverload()
	out := p.View(th())
	if !strings.Contains(out, "func B") {
		t.Errorf("View after NextOverload should show second sig, got: %q", out)
	}
	if !strings.Contains(out, "2 of 2") {
		t.Errorf("counter should track active sig, got: %q", out)
	}
}

func TestActiveSignatureOnClosedPaneReturnsZeroValue(t *testing.T) {
	t.Parallel()
	p := New()
	sig := p.ActiveSignature()
	if sig.ActiveParameter != -1 {
		t.Errorf("zero signature should have ActiveParameter=-1, got %d", sig.ActiveParameter)
	}
	if len(sig.Parameters) != 0 {
		t.Errorf("zero signature should have empty Parameters, got %d", len(sig.Parameters))
	}
}

func TestFormatCounterShape(t *testing.T) {
	t.Parallel()
	got := formatCounter(2, 5)
	if got != "2 of 5" {
		t.Errorf("formatCounter: %q", got)
	}
}

func TestClampLineNoTruncation(t *testing.T) {
	t.Parallel()
	got := clampLine("hello", 10)
	if got != "hello" {
		t.Errorf("no truncation expected: %q", got)
	}
}

func TestClampLineTruncates(t *testing.T) {
	t.Parallel()
	got := clampLine("abcdefghijklmnop", 5)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix: %q", got)
	}
	if len([]rune(got)) > 5 {
		t.Errorf("clampLine exceeded width: %q (runes=%d)", got, len([]rune(got)))
	}
}

func TestWrapAndClampKeepsShortText(t *testing.T) {
	t.Parallel()
	color := th().TextMuted
	got := wrapAndClamp("ok", 10, 3, color)
	if !strings.Contains(got, "ok") {
		t.Errorf("wrapAndClamp dropped content: %q", got)
	}
}

func TestWrapAndClampWrapsLongLine(t *testing.T) {
	t.Parallel()
	color := th().TextMuted
	got := wrapAndClamp(strings.Repeat("a", 20), 5, 3, color)
	lines := strings.Split(stripStyles(got), "\n")
	if len(lines) < 3 {
		t.Errorf("expected >=3 lines, got %d: %q", len(lines), got)
	}
}

func TestWrapAndClampClampsToMaxLines(t *testing.T) {
	t.Parallel()
	color := th().TextMuted
	body := strings.Repeat(strings.Repeat("a", 5)+"\n", 10)
	got := wrapAndClamp(body, 10, 2, color)
	lines := strings.Split(stripStyles(got), "\n")
	if len(lines) > 2 {
		t.Errorf("expected <=2 lines, got %d: %q", len(lines), got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("expected ellipsis when clamped: %q", got)
	}
}

// stripStyles is a crude ANSI escape stripper for assertions. Real-world
// terminal output is styled, but tests just need to count lines.
func stripStyles(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
