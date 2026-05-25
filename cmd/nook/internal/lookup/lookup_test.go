package lookup

import (
	"errors"
	"testing"

	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
)

// TestHoverCmdNilClient confirms hover against a nil client returns a
// HoverMsg with errNoClient instead of panicking. The host model binds
// Alt+i unconditionally; nil-client must be a quiet message, not a
// crash.
func TestHoverCmdNilClient(t *testing.T) {
	t.Parallel()
	cmd := HoverCmd(nil, "main.go", 3, 7)
	if cmd == nil {
		t.Fatal("HoverCmd returned nil tea.Cmd")
	}
	msg, ok := cmd().(HoverMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want HoverMsg", cmd())
	}
	if msg.Path != "main.go" || msg.Row != 3 || msg.Col != 7 {
		t.Errorf("inputs not echoed: got %+v", msg)
	}
	if !errors.Is(msg.Err, errNoClient) {
		t.Errorf("err = %v, want errNoClient", msg.Err)
	}
	if msg.Info != (nooklsp.HoverInfo{}) {
		t.Errorf("Info should be zero on nil client, got %+v", msg.Info)
	}
}

// TestDefinitionCmdNilClient confirms goto-def against a nil client
// returns a DefinitionMsg with errNoClient and an empty Locations
// slice.
func TestDefinitionCmdNilClient(t *testing.T) {
	t.Parallel()
	cmd := DefinitionCmd(nil, "main.go", 12, 4)
	if cmd == nil {
		t.Fatal("DefinitionCmd returned nil tea.Cmd")
	}
	msg, ok := cmd().(DefinitionMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want DefinitionMsg", cmd())
	}
	if msg.Path != "main.go" || msg.Row != 12 || msg.Col != 4 {
		t.Errorf("inputs not echoed: got %+v", msg)
	}
	if !errors.Is(msg.Err, errNoClient) {
		t.Errorf("err = %v, want errNoClient", msg.Err)
	}
	if len(msg.Locations) != 0 {
		t.Errorf("Locations should be empty on nil client, got %d", len(msg.Locations))
	}
}

// TestFormattingCmdNilClient confirms a format request against a nil
// client returns FormattingMsg{Err: errNoClient} with the requested
// version echoed back, so the host can degrade to a plain save without
// branching on LSP readiness.
func TestFormattingCmdNilClient(t *testing.T) {
	t.Parallel()
	cmd := FormattingCmd(nil, "main.go", 7, 4, false)
	if cmd == nil {
		t.Fatal("FormattingCmd returned nil tea.Cmd")
	}
	msg, ok := cmd().(FormattingMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want FormattingMsg", cmd())
	}
	if msg.Path != "main.go" || msg.Version != 7 {
		t.Errorf("inputs not echoed: got %+v", msg)
	}
	if !errors.Is(msg.Err, errNoClient) {
		t.Errorf("err = %v, want errNoClient", msg.Err)
	}
	if len(msg.Edits) != 0 {
		t.Errorf("Edits should be empty on nil client, got %d", len(msg.Edits))
	}
}

// TestCompletionCmdNilClient confirms completion against a nil client
// returns a CompletionMsg with errNoClient and preserves the prefix
// length, so the host's accept path stays consistent across the
// nil-client edge.
func TestCompletionCmdNilClient(t *testing.T) {
	t.Parallel()
	cmd := CompletionCmd(nil, "main.go", 5, 9, 3)
	if cmd == nil {
		t.Fatal("CompletionCmd returned nil tea.Cmd")
	}
	msg, ok := cmd().(CompletionMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want CompletionMsg", cmd())
	}
	if msg.Path != "main.go" || msg.Row != 5 || msg.Col != 9 || msg.PrefixLen != 3 {
		t.Errorf("inputs not echoed: got %+v", msg)
	}
	if !errors.Is(msg.Err, errNoClient) {
		t.Errorf("err = %v, want errNoClient", msg.Err)
	}
	if len(msg.Items) != 0 {
		t.Errorf("Items should be empty on nil client, got %d", len(msg.Items))
	}
}

// TestResolveCompletionCmdNilClient confirms a resolve call against a nil
// client returns ResolveCompletionMsg{Err: errNoClient} carrying the
// original item, so the host's "drop stale response by label" path stays
// consistent when LSP is offline.
func TestResolveCompletionCmdNilClient(t *testing.T) {
	t.Parallel()
	item := nooklsp.CompletionItem{Label: "Println", Detail: "func(...)"}
	cmd := ResolveCompletionCmd(nil, item)
	if cmd == nil {
		t.Fatal("ResolveCompletionCmd returned nil tea.Cmd")
	}
	msg, ok := cmd().(ResolveCompletionMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want ResolveCompletionMsg", cmd())
	}
	if msg.ReqLabel != "Println" {
		t.Errorf("ReqLabel not echoed: %q", msg.ReqLabel)
	}
	if msg.Item.Label != "Println" {
		t.Errorf("Item.Label not echoed: %+v", msg.Item)
	}
	if !errors.Is(msg.Err, errNoClient) {
		t.Errorf("err = %v, want errNoClient", msg.Err)
	}
}
