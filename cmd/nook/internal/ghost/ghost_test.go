package ghost

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewManagerDisabledWithNilClient(t *testing.T) {
	m := NewManager(nil)
	if m.Enabled() {
		t.Fatal("expected disabled manager with nil client")
	}
	if cmd := m.Tick(Site{Path: "x", Prefix: "abc"}, false, false); cmd != nil {
		t.Fatalf("expected nil cmd, got %v", cmd)
	}
	if got := m.Accept(); got != "" {
		t.Fatalf("expected empty accept, got %q", got)
	}
}

func TestNilManagerIsNoop(t *testing.T) {
	var m *Manager
	if m.Enabled() {
		t.Fatal("nil manager should not be enabled")
	}
	if cmd := m.Tick(Site{}, false, false); cmd != nil {
		t.Fatalf("expected nil cmd, got %v", cmd)
	}
	if got := m.Proposal(); got != "" {
		t.Fatalf("expected empty proposal, got %q", got)
	}
	m.Dismiss()
}

func TestSanitizeTrimsFenceAndTrailingNewlines(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"fmt.Println()", "fmt.Println()"},
		{"  fmt.Println()  ", "fmt.Println()"},
		{"```go\nfmt.Println()", "fmt.Println()"},
		{"fmt.Println()\n\n", "fmt.Println()"},
		{"\nfmt.Println()", ""},
		{"fmt.Println()\nrest of file", "fmt.Println()"},
	}
	for _, c := range cases {
		if got := sanitize(c.in); got != c.out {
			t.Errorf("sanitize(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}

func TestTickWithShortPrefixSchedulesNothing(t *testing.T) {
	// Fake-enabled by setting enabled=true without a real client. The test only
	// exercises gating, not the request itself.
	m := &Manager{enabled: true}
	if cmd := m.Tick(Site{Path: "f", Prefix: "x"}, false, false); cmd != nil {
		t.Fatalf("expected nil cmd for too-short prefix, got non-nil")
	}
}

func TestTickWithIdleCancelsAndReturnsNil(t *testing.T) {
	m := &Manager{enabled: true, proposal: "stale"}
	if cmd := m.Tick(Site{Path: "f", Prefix: "abc"}, true /*idle*/, false); cmd != nil {
		t.Fatalf("expected nil cmd when idle, got non-nil")
	}
	if m.proposal != "" {
		t.Fatalf("expected proposal cleared when idle, got %q", m.proposal)
	}
}

func TestTickReturnsDebounceCmd(t *testing.T) {
	m := &Manager{enabled: true}
	cmd := m.Tick(Site{Path: "f", Prefix: "abcdef", Row: 1, Col: 6}, false, false)
	if cmd == nil {
		t.Fatal("expected debounce cmd, got nil")
	}
	if m.currentSite.Prefix != "abcdef" {
		t.Fatalf("expected currentSite tracked, got %v", m.currentSite)
	}
	// Running the cmd should eventually yield a debounceMsg with the right gen.
	resultCh := make(chan tea.Msg, 1)
	go func() {
		resultCh <- cmd()
	}()
	select {
	case msg := <-resultCh:
		dm, ok := msg.(debounceMsg)
		if !ok {
			t.Fatalf("expected debounceMsg, got %T", msg)
		}
		if dm.generation != m.generation {
			t.Fatalf("expected generation %d, got %d", m.generation, dm.generation)
		}
	case <-time.After(DebounceDelay + 250*time.Millisecond):
		t.Fatal("debounce cmd did not fire in time")
	}
}

func TestStaleDebounceMsgIsDiscarded(t *testing.T) {
	m := &Manager{enabled: true, generation: 5}
	if cmd := m.Update(debounceMsg{generation: 3}); cmd != nil {
		t.Fatalf("expected nil from stale debounce, got non-nil")
	}
}

func TestSuggestMsgInWrongSiteIgnored(t *testing.T) {
	m := &Manager{enabled: true, currentSite: Site{Path: "a", Row: 1, Col: 2, Prefix: "ab"}}
	m.Update(SuggestMsg{Site: Site{Path: "b"}, Text: "x"})
	if m.proposal != "" {
		t.Fatalf("expected proposal unchanged, got %q", m.proposal)
	}
}

func TestSuggestMsgAppliesProposalForCurrentSite(t *testing.T) {
	site := Site{Path: "a", Row: 1, Col: 4, Prefix: "fmt."}
	m := &Manager{enabled: true, currentSite: site}
	m.Update(SuggestMsg{Site: site, Text: "Println(\"hi\")\nrest"})
	if got := m.Proposal(); got != "Println(\"hi\")" {
		t.Fatalf("expected sanitized proposal, got %q", got)
	}
}

func TestAcceptConsumesProposal(t *testing.T) {
	m := &Manager{enabled: true, proposal: "Println()"}
	if got := m.Accept(); got != "Println()" {
		t.Fatalf("expected text, got %q", got)
	}
	if m.proposal != "" {
		t.Fatalf("expected cleared, got %q", m.proposal)
	}
}

func TestDismissClearsProposalAndCancelsInflight(t *testing.T) {
	called := false
	m := &Manager{
		enabled:  true,
		proposal: "x",
		inflight: func() { called = true },
	}
	m.Dismiss()
	if m.proposal != "" || !called {
		t.Fatalf("expected proposal cleared and inflight cancelled (got %q, called=%v)", m.proposal, called)
	}
}

func TestTickOnNewSiteDropsOldProposal(t *testing.T) {
	m := &Manager{enabled: true, proposal: "x", currentSite: Site{Path: "a", Row: 1, Col: 3, Prefix: "abc"}}
	_ = m.Tick(Site{Path: "b", Row: 1, Col: 3, Prefix: "abc"}, false, false)
	if m.proposal != "" {
		t.Fatalf("expected proposal cleared on site change, got %q", m.proposal)
	}
}

func TestUserPromptIncludesFileAndPrefix(t *testing.T) {
	s := Site{Path: "main.go", Row: 5, Col: 4, Prefix: "fmt."}
	out := userPrompt(s)
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected file path in prompt, got %s", out)
	}
	if !strings.Contains(out, "fmt.") {
		t.Errorf("expected prefix in prompt, got %s", out)
	}
	if !strings.Contains(out, "row: 5") || !strings.Contains(out, "col: 4") {
		t.Errorf("expected row/col in prompt, got %s", out)
	}
}

func TestItoaRoundTrip(t *testing.T) {
	cases := []struct {
		n int
		s string
	}{
		{0, "0"}, {1, "1"}, {42, "42"}, {-7, "-7"}, {1000000, "1000000"},
	}
	for _, c := range cases {
		if got := itoa(c.n); got != c.s {
			t.Errorf("itoa(%d) = %q, want %q", c.n, got, c.s)
		}
	}
}

func TestDemoModeReturnsConfiguredText(t *testing.T) {
	t.Setenv(DemoEnvVar, "Println(\"hi\")|Errorf(\"boom: %w\", err)")
	m := NewManager(nil)
	if !m.Enabled() {
		t.Fatal("expected demo manager enabled regardless of client")
	}
	if len(m.demoProposals) != 2 {
		t.Fatalf("expected 2 demo proposals, got %d", len(m.demoProposals))
	}
	// Issue a request; it should yield the first proposal.
	cmd := m.request(Site{Path: "a", Row: 1, Col: 5, Prefix: "fmt."})
	if cmd == nil {
		t.Fatal("expected demo cmd, got nil")
	}
	msg := cmd()
	sm, ok := msg.(SuggestMsg)
	if !ok {
		t.Fatalf("expected SuggestMsg, got %T", msg)
	}
	if sm.Text != "Println(\"hi\")" {
		t.Fatalf("expected first demo, got %q", sm.Text)
	}
	// Second request cycles to the next proposal.
	cmd = m.request(Site{Path: "a", Row: 2, Col: 5, Prefix: "fmt."})
	msg = cmd()
	if got := msg.(SuggestMsg).Text; got != "Errorf(\"boom: %w\", err)" {
		t.Fatalf("expected second demo, got %q", got)
	}
}

func TestDemoModeOverridesNilClient(t *testing.T) {
	t.Setenv(DemoEnvVar, "foo()")
	m := NewManager(nil)
	if !m.Enabled() {
		t.Fatal("expected enabled with demo override even when client nil")
	}
}

func TestNoDemoNoClientStaysDisabled(t *testing.T) {
	t.Setenv(DemoEnvVar, "")
	m := NewManager(nil)
	if m.Enabled() {
		t.Fatal("expected disabled when no client and no demo")
	}
}

func TestLongPrefixTruncated(t *testing.T) {
	s := Site{Path: "a", Prefix: strings.Repeat("x", 5000)}
	out := userPrompt(s)
	// Should still contain a body but the prefix portion shouldn't be 5000 chars.
	if strings.Count(out, "x") > 2000 {
		t.Fatalf("expected prefix truncated, got %d x's", strings.Count(out, "x"))
	}
}

func TestSetRulesRoundTrip(t *testing.T) {
	m := NewManager(nil)
	if got := m.Rules(); got != "" {
		t.Fatalf("fresh manager should have empty rules, got %q", got)
	}
	m.SetRules("always lowercase identifiers")
	if got := m.Rules(); got != "always lowercase identifiers" {
		t.Fatalf("SetRules round-trip failed; got %q", got)
	}
	// SetRules on a nil manager is a no-op (no panic).
	var nilMgr *Manager
	nilMgr.SetRules("anything")
	if got := nilMgr.Rules(); got != "" {
		t.Fatalf("nil manager Rules() should be empty; got %q", got)
	}
}
