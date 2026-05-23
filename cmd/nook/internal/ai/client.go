// Package ai wraps the Anthropic Messages API in a small streaming interface
// designed for terminal-driven UIs. The wrapper is intentionally thin: it
// exposes (1) a single Client.Stream call that returns a channel of text
// deltas plus a done channel, and (2) tier-specific helpers that pin model
// choices (Haiku for fast inline edits, Sonnet for multi-step Composer work).
package ai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Tier selects the model. We pin two tiers and nothing else so the editor's
// keybindings stay model-agnostic.
type Tier int

const (
	// Fast pins claude-haiku-4-5: low-latency inline edits.
	Fast Tier = iota
	// Smart pins claude-sonnet-4-6: multi-file Composer edits.
	Smart
)

// Model returns the SDK Model constant for the tier.
func (t Tier) Model() anthropic.Model {
	switch t {
	case Smart:
		return anthropic.ModelClaudeSonnet4_6
	default:
		return anthropic.ModelClaudeHaiku4_5
	}
}

// MaxTokens is the per-call output cap for the tier.
func (t Tier) MaxTokens() int64 {
	if t == Smart {
		return 8192
	}
	return 4096
}

// ErrNoAPIKey is returned when neither the env var nor an explicit option
// provides ANTHROPIC_API_KEY. We surface this clearly because nook should
// remain useful (search, git, editor) even when AI features aren't wired.
var ErrNoAPIKey = errors.New("ANTHROPIC_API_KEY is not set")

// Client owns the anthropic.Client. It is safe for concurrent use.
type Client struct {
	sdk anthropic.Client
}

// NewClient constructs a Client. If the env var is missing it returns
// ErrNoAPIKey so the caller can degrade gracefully.
func NewClient() (*Client, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, ErrNoAPIKey
	}
	c := anthropic.NewClient(option.WithAPIKey(key))
	return &Client{sdk: c}, nil
}

// Available reports whether the env is set. Useful for status-bar rendering.
func Available() bool {
	return strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) != ""
}

// Request is the small surface for a single streaming call.
type Request struct {
	Tier   Tier
	System string
	User   string
	// StopSequences cuts the stream early when the model emits one of these
	// markers. We use this for fence-delimited edits.
	StopSequences []string
}

// Stream issues a streaming Messages.New call and returns a delta channel
// plus a done channel. The delta channel emits text fragments as they arrive;
// the done channel emits exactly one error (or nil) when the stream closes.
//
// Callers MUST drain the delta channel until close. Cancelling ctx aborts the
// underlying HTTP request.
func (c *Client) Stream(ctx context.Context, req Request) (<-chan string, <-chan error) {
	deltas := make(chan string, 64)
	done := make(chan error, 1)

	go func() {
		defer close(deltas)
		defer close(done)

		params := anthropic.MessageNewParams{
			Model:     req.Tier.Model(),
			MaxTokens: req.Tier.MaxTokens(),
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(req.User)),
			},
		}
		if req.System != "" {
			params.System = []anthropic.TextBlockParam{{Text: req.System}}
		}
		if len(req.StopSequences) > 0 {
			params.StopSequences = req.StopSequences
		}

		stream := c.sdk.Messages.NewStreaming(ctx, params)
		for stream.Next() {
			evt := stream.Current()
			switch v := evt.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				if text := v.Delta.Text; text != "" {
					select {
					case deltas <- text:
					case <-ctx.Done():
						done <- ctx.Err()
						return
					}
				}
			}
		}
		if err := stream.Err(); err != nil {
			done <- fmt.Errorf("anthropic stream: %w", err)
			return
		}
		done <- nil
	}()

	return deltas, done
}
