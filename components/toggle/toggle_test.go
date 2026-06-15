package toggle

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(s string) tea.KeyMsg {
	switch s {
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestNewDefaults(t *testing.T) {
	m := New()
	if m.On() {
		t.Errorf("new toggle should be off, got on")
	}
	if m.Disabled() {
		t.Errorf("new toggle should not be disabled")
	}
}

func TestWithOnSetsState(t *testing.T) {
	if !New().WithOn(true).On() {
		t.Errorf("WithOn(true) should report On() == true")
	}
}

func TestSpaceFlips(t *testing.T) {
	m := New()
	m, cmd := m.Update(key(" "))
	if !m.On() {
		t.Errorf("space should flip off -> on, got off")
	}
	if cmd == nil {
		t.Fatalf("a flip should emit a command")
	}
	if msg, ok := cmd().(ToggleChangedMsg); !ok || !msg.On {
		t.Errorf("expected ToggleChangedMsg{On: true}, got %#v", cmd())
	}
	m, _ = m.Update(key(" "))
	if m.On() {
		t.Errorf("second space should flip on -> off, got on")
	}
}

func TestEnterFlips(t *testing.T) {
	m, cmd := New().Update(key("enter"))
	if !m.On() {
		t.Errorf("enter should flip off -> on")
	}
	if cmd == nil {
		t.Errorf("enter that flips should emit a command")
	}
}

func TestDirectionalKeysForceState(t *testing.T) {
	m := New().WithOn(true)
	m, _ = m.Update(key("left"))
	if m.On() {
		t.Errorf("left should force off")
	}
	m, _ = m.Update(key("right"))
	if !m.On() {
		t.Errorf("right should force on")
	}
	m, _ = m.Update(key("h"))
	if m.On() {
		t.Errorf("h should force off")
	}
	m, _ = m.Update(key("l"))
	if !m.On() {
		t.Errorf("l should force on")
	}
}

func TestYNKeysSetState(t *testing.T) {
	m, _ := New().Update(key("y"))
	if !m.On() {
		t.Errorf("y should set on")
	}
	m, _ = m.Update(key("n"))
	if m.On() {
		t.Errorf("n should set off")
	}
}

func TestNoChangeEmitsNoCommand(t *testing.T) {
	// Already off; forcing off again must not emit.
	m := New()
	_, cmd := m.Update(key("left"))
	if cmd != nil {
		t.Errorf("forcing the held state should emit no command")
	}
	// Already on; forcing on again must not emit.
	m = New().WithOn(true)
	_, cmd = m.Update(key("right"))
	if cmd != nil {
		t.Errorf("forcing the held state should emit no command")
	}
}

func TestDisabledIgnoresKeys(t *testing.T) {
	m := New().WithDisabled(true)
	m, cmd := m.Update(key(" "))
	if m.On() {
		t.Errorf("disabled toggle should ignore space")
	}
	if cmd != nil {
		t.Errorf("disabled toggle should emit no command")
	}
}

func TestNonKeyMsgIgnored(t *testing.T) {
	m := New()
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 10, Height: 2})
	if cmd != nil {
		t.Errorf("non-key message should emit no command")
	}
}

func TestViewShowsCaption(t *testing.T) {
	on := New().WithOn(true).WithOnLabel("Enabled").View()
	if !strings.Contains(on, "Enabled") {
		t.Errorf("on view should contain the on caption, got %q", on)
	}
	off := New().WithOffLabel("Disabled").View()
	if !strings.Contains(off, "Disabled") {
		t.Errorf("off view should contain the off caption, got %q", off)
	}
}

func TestWithShowTextHidesCaption(t *testing.T) {
	v := New().WithOnLabel("Enabled").WithOn(true).WithShowText(false).View()
	if strings.Contains(v, "Enabled") {
		t.Errorf("WithShowText(false) should hide the caption, got %q", v)
	}
}

func TestViewShowsLeadingLabel(t *testing.T) {
	v := New().WithLabel("Wrap: ").View()
	if !strings.Contains(v, "Wrap: ") {
		t.Errorf("view should contain the leading label, got %q", v)
	}
}

func TestKnobPositionFollowsState(t *testing.T) {
	// The knob glyph sits after the track when on, before it when off.
	// Strip styling by checking rune order against the track runes.
	off := stripANSI(New().View())
	on := stripANSI(New().WithOn(true).View())
	if strings.IndexRune(off, []rune(knob)[0]) > strings.IndexRune(on, []rune(knob)[0]) {
		t.Errorf("knob should move right when on: off=%q on=%q", off, on)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEsc = true
		case inEsc && r == 'm':
			inEsc = false
		case inEsc:
			// skip
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
