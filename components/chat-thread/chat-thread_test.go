package chatthread

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	chatbubble "github.com/truffle-dev/glyph/components/chat-bubble"
	"github.com/truffle-dev/glyph/components/theme"
)

func TestEmptyThreadShowsPlaceholder(t *testing.T) {
	out := New(theme.Default).WithSize(40, 6).View()
	if !strings.Contains(out, "No messages yet") {
		t.Errorf("expected empty placeholder, got %q", out)
	}
}

func TestRendersBothMessagesWhenSpaceAllows(t *testing.T) {
	th := New(theme.Default).WithSize(60, 30)
	th = th.Append(Message{Role: chatbubble.RoleUser, Text: "hello"})
	th = th.Append(Message{Role: chatbubble.RoleAssistant, Text: "hi there"})
	out := th.View()
	if !strings.Contains(out, "hello") {
		t.Errorf("expected 'hello' in output, got %q", out)
	}
	if !strings.Contains(out, "hi there") {
		t.Errorf("expected 'hi there' in output, got %q", out)
	}
}

func TestScrollDownClampsAtBottom(t *testing.T) {
	th := New(theme.Default).WithSize(60, 30).WithMessages([]Message{
		{Role: chatbubble.RoleUser, Text: "first"},
	})
	th = th.ScrollDown(5)
	if got := th.offset; got != 0 {
		t.Errorf("expected offset 0 at bottom, got %d", got)
	}
}

func TestScrollUpClampsAtTop(t *testing.T) {
	msgs := []Message{}
	for i := 0; i < 20; i++ {
		msgs = append(msgs, Message{Role: chatbubble.RoleUser, Text: "msg"})
	}
	th := New(theme.Default).WithSize(40, 6).WithMessages(msgs)
	th = th.ScrollUp(1000)
	if got := th.offset; got > th.maxOffset() {
		t.Errorf("expected offset <= maxOffset (%d), got %d", th.maxOffset(), got)
	}
}

func TestKeyDownDecreasesOffset(t *testing.T) {
	msgs := []Message{}
	for i := 0; i < 30; i++ {
		msgs = append(msgs, Message{Role: chatbubble.RoleUser, Text: "msg"})
	}
	th := New(theme.Default).WithSize(40, 6).WithMessages(msgs)
	th = th.ScrollUp(5)
	before := th.offset
	next, _ := th.Update(tea.KeyMsg{Type: tea.KeyDown})
	if next.offset != before-1 {
		t.Errorf("expected offset to decrease by 1, got %d -> %d", before, next.offset)
	}
}

func TestEndKeyResetsOffsetToZero(t *testing.T) {
	msgs := []Message{}
	for i := 0; i < 30; i++ {
		msgs = append(msgs, Message{Role: chatbubble.RoleUser, Text: "msg"})
	}
	th := New(theme.Default).WithSize(40, 6).WithMessages(msgs)
	th = th.ScrollUp(100)
	next, _ := th.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if next.offset != 0 {
		t.Errorf("expected offset 0 after End, got %d", next.offset)
	}
}

func TestAppendPreservesOffsetWhenScrolledUp(t *testing.T) {
	msgs := []Message{}
	for i := 0; i < 30; i++ {
		msgs = append(msgs, Message{Role: chatbubble.RoleUser, Text: "msg"})
	}
	th := New(theme.Default).WithSize(40, 6).WithMessages(msgs).ScrollUp(5)
	before := th.offset
	next := th.Append(Message{Role: chatbubble.RoleAssistant, Text: "new"})
	if next.offset != before {
		t.Errorf("expected append to preserve offset %d when scrolled up, got %d", before, next.offset)
	}
}

func TestAppendStaysAtBottomWhenAlreadyThere(t *testing.T) {
	th := New(theme.Default).WithSize(60, 30).WithMessages([]Message{
		{Role: chatbubble.RoleUser, Text: "a"},
	})
	next := th.Append(Message{Role: chatbubble.RoleAssistant, Text: "b"})
	if next.offset != 0 {
		t.Errorf("expected offset 0 (still at bottom), got %d", next.offset)
	}
}

func TestViewMatchesHeight(t *testing.T) {
	msgs := []Message{}
	for i := 0; i < 10; i++ {
		msgs = append(msgs, Message{Role: chatbubble.RoleUser, Text: "msg"})
	}
	th := New(theme.Default).WithSize(40, 10).WithMessages(msgs)
	out := th.View()
	got := strings.Count(out, "\n") + 1
	if got != 10 {
		t.Errorf("expected View height of 10 lines, got %d", got)
	}
}
