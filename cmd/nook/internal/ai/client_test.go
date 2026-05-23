package ai

import (
	"os"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestTierModelAndTokens(t *testing.T) {
	if Fast.Model() != anthropic.ModelClaudeHaiku4_5 {
		t.Fatalf("Fast should map to Haiku, got %q", Fast.Model())
	}
	if Smart.Model() != anthropic.ModelClaudeSonnet4_6 {
		t.Fatalf("Smart should map to Sonnet, got %q", Smart.Model())
	}
	if Fast.MaxTokens() >= Smart.MaxTokens() {
		t.Fatalf("Smart tier should have larger MaxTokens; fast=%d smart=%d",
			Fast.MaxTokens(), Smart.MaxTokens())
	}
}

func TestNewClientWithoutKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	c, err := NewClient()
	if err != ErrNoAPIKey {
		t.Fatalf("expected ErrNoAPIKey, got err=%v client=%v", err, c)
	}
	if c != nil {
		t.Fatal("expected nil client when key is absent")
	}
}

func TestAvailableMirrorsEnv(t *testing.T) {
	prev := os.Getenv("ANTHROPIC_API_KEY")
	defer os.Setenv("ANTHROPIC_API_KEY", prev)

	os.Setenv("ANTHROPIC_API_KEY", "")
	if Available() {
		t.Fatal("Available() should be false when env is empty")
	}
	os.Setenv("ANTHROPIC_API_KEY", "sk-test")
	if !Available() {
		t.Fatal("Available() should be true when env is set")
	}
}

func TestNewClientWithKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-fake-test-key")
	c, err := NewClient()
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
