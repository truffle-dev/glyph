package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/cmd/nook/internal/callhierarchy"
	"github.com/truffle-dev/glyph/cmd/nook/internal/multibuffer"
)

// altRune simulates an alt+<rune> keystroke as bubbletea emits it.
func altRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Alt: true, Runes: []rune{r}}
}

func TestAltKWithoutOpenFileIsStatusHint(t *testing.T) {
	m := newModel(t.TempDir())
	m.width = 120
	m.height = 32
	m = m.resize()
	updated, cmd := m.Update(altRune('k'))
	mm := updated.(model)
	if mm.overlay != overlayNone {
		t.Fatalf("expected no overlay, got %v", mm.overlay)
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd when no file is open, got %T", cmd)
	}
	if !strings.Contains(mm.status, "open a file first") {
		t.Fatalf("expected open-a-file hint, got %q", mm.status)
	}
}

func TestAltKWithoutLSPIsStatusHint(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.bufs.OpenOrSwitch(path)
	updated, cmd := m.Update(altRune('k'))
	mm := updated.(model)
	if mm.overlay != overlayNone {
		t.Fatalf("expected no overlay (no LSP), got %v", mm.overlay)
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd without LSP, got %T", cmd)
	}
	if !strings.Contains(mm.status, "incoming calls: no language server") {
		t.Fatalf("expected no-LSP hint with incoming-calls label, got %q", mm.status)
	}
}

func TestAltKShiftWithoutLSPIsStatusHint(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.bufs.OpenOrSwitch(path)
	updated, cmd := m.Update(altRune('K'))
	mm := updated.(model)
	if cmd != nil {
		t.Fatalf("expected no Cmd without LSP, got %T", cmd)
	}
	if !strings.Contains(mm.status, "outgoing calls: no language server") {
		t.Fatalf("expected no-LSP hint with outgoing-calls label, got %q", mm.status)
	}
}

func TestCallHierarchyCmdFiresFragmentsMsg(t *testing.T) {
	// Without a real LSP client we exercise the nil-client guarded path so
	// the FragmentsMsg always lands, never panics.
	cmd := callhierarchy.CallHierarchyCmd(nil, "/x.go", 0, 0, callhierarchy.Incoming, 3, nil)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd")
	}
	msg := cmd()
	frag, ok := msg.(multibuffer.FragmentsMsg)
	if !ok {
		t.Fatalf("expected FragmentsMsg, got %T", msg)
	}
	if frag.Source != "callhierarchy.incoming" {
		t.Fatalf("Source=%q want callhierarchy.incoming", frag.Source)
	}
	if frag.Err == nil {
		t.Fatal("expected ErrNoClient on nil client")
	}
}

func TestHostHandlesIncomingFragmentsMsg(t *testing.T) {
	// Verify the host routes a callhierarchy FragmentsMsg into the
	// multibuffer pane the same way it routes findrefs / symbol-search
	// results — by Source not source-of-truth. The pane shows the fragments
	// regardless of which loader fired the request.
	root := t.TempDir()
	m := newModel(root)
	m.width = 120
	m.height = 32
	m = m.resize()
	m.overlay = overlayMultibuffer
	m.multibufPane = m.multibufPane.WithSize(m.width-4, m.height-4).
		Reset(callhierarchy.Incoming.Label() + " — Foo").
		Focus()
	frags := []multibuffer.Fragment{{
		Path:      filepath.Join(root, "caller.go"),
		StartLine: 5,
		EndLine:   7,
		Lines: []multibuffer.Line{
			{Marker: multibuffer.Context, FileLine: 5, Text: "func Caller() {"},
			{Marker: multibuffer.Added, FileLine: 6, Text: "\tFoo()"},
			{Marker: multibuffer.Context, FileLine: 7, Text: "}"},
		},
		Suffix: "Caller — func()",
	}}
	updated, _ := m.Update(multibuffer.FragmentsMsg{
		Source:    callhierarchy.Incoming.Source(),
		Fragments: frags,
	})
	mm := updated.(model)
	if mm.overlay != overlayMultibuffer {
		t.Fatalf("expected overlayMultibuffer to remain, got %v", mm.overlay)
	}
	view := mm.multibufPane.View()
	if view == "" {
		t.Fatal("expected non-empty multibuffer view after FragmentsMsg")
	}
	if !strings.Contains(view, "Foo") && !strings.Contains(view, "Caller") {
		t.Fatalf("expected view to mention symbol or caller name; got truncated view")
	}
}
