package ai

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTierModel(t *testing.T) {
	if Fast.Model() != "claude-haiku-4-5" {
		t.Fatalf("Fast tier should map to claude-haiku-4-5, got %q", Fast.Model())
	}
	if Smart.Model() != "claude-sonnet-4-6" {
		t.Fatalf("Smart tier should map to claude-sonnet-4-6, got %q", Smart.Model())
	}
}

func TestNewClientWithoutBinary(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("HOME", t.TempDir())
	c, err := NewClient()
	if !errors.Is(err, ErrNoClaude) {
		t.Fatalf("expected ErrNoClaude, got err=%v client=%v", err, c)
	}
	if c != nil {
		t.Fatal("expected nil client when claude is absent")
	}
}

func TestAvailableMirrorsLookup(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	t.Setenv("HOME", t.TempDir())
	if Available() {
		t.Fatal("Available() should be false when claude binary is missing")
	}
}

func TestStreamEmitsTextDeltas(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub binary uses a POSIX shebang script")
	}
	stub := writeStubClaude(t, stubScript(`
{"type":"system","subtype":"init","session_id":"s","tools":[]}
{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello "}}}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"reasoning"}}}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}}
{"type":"result","subtype":"success","result":"hello world","total_cost_usd":0}
`))
	c, err := NewClientWithBinary(stub)
	if err != nil {
		t.Fatal(err)
	}

	deltas, done := c.Stream(context.Background(), Request{
		Tier: Fast,
		User: "anything",
	})
	var got strings.Builder
	for d := range deltas {
		got.WriteString(d)
	}
	if err := <-done; err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if got.String() != "hello world" {
		t.Fatalf("expected delta concat 'hello world', got %q", got.String())
	}
}

func TestStreamStopSequenceTruncates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub binary uses a POSIX shebang script")
	}
	stub := writeStubClaude(t, stubScript(`
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"keep this"}}}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"\nSTOP after newline"}}}
{"type":"result","subtype":"success","result":"keep this","total_cost_usd":0}
sleep 5
`))
	c, _ := NewClientWithBinary(stub)
	deltas, done := c.Stream(context.Background(), Request{
		Tier:          Fast,
		User:          "x",
		StopSequences: []string{"\n"},
	})
	var got strings.Builder
	for d := range deltas {
		got.WriteString(d)
	}
	<-done
	if got.String() != "keep this" {
		t.Fatalf("expected output truncated at newline, got %q", got.String())
	}
}

// stubScript wraps a JSONL body in a tiny shell script that emits it on
// stdout, exits 0, and ignores its argv. The script tolerates an inline
// `sleep N` line so a test can verify we kill the process on stop hit.
func stubScript(body string) string {
	return "#!/bin/sh\ncat <<'EOF'\n" + strings.TrimSpace(body) + "\nEOF\n"
}

func writeStubClaude(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "claude")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}
