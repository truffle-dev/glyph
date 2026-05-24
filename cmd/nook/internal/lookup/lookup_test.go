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
