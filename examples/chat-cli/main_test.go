package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	chatinput "github.com/truffle-dev/glyph/components/chat-input"
	commandpalette "github.com/truffle-dev/glyph/components/command-palette"
	"github.com/truffle-dev/glyph/components/confirmation"
	selectinput "github.com/truffle-dev/glyph/components/select"
	textinput "github.com/truffle-dev/glyph/components/text-input"
)

func resize(t *testing.T, m model, w, h int) model {
	t.Helper()
	mi, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return mi.(model)
}

func TestInitialView(t *testing.T) {
	m := newModel()
	m = resize(t, m, 100, 28)
	out := m.View()
	if !strings.Contains(out, "you ›") {
		t.Fatal("initial view must show the input prompt")
	}
	if !strings.Contains(out, "Opus 4.7") {
		t.Fatal("status bar must show the default model name")
	}
	if !strings.Contains(out, "Welcome") {
		t.Fatal("seed assistant message must render")
	}
}

func TestSendMessageThenReceiveReply(t *testing.T) {
	m := newModel()
	m = resize(t, m, 100, 28)

	mi, cmd := m.Update(chatinput.SubmitMsg{Value: "hello"})
	m = mi.(model)
	if !m.thinking {
		t.Fatal("submitting a message should flip thinking=true")
	}
	if cmd == nil {
		t.Fatal("submit must dispatch a reply command")
	}

	mi, _ = m.Update(replyMsg{text: "Hi back."})
	m = mi.(model)
	if m.thinking {
		t.Fatal("receiving a reply should clear thinking")
	}
	if m.msgCount != 3 {
		t.Fatalf("expected 3 messages (seed + user + reply), got %d", m.msgCount)
	}
}

func TestPaletteOpensOnCtrlP(t *testing.T) {
	m := newModel()
	m = resize(t, m, 100, 28)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = mi.(model)
	if m.mode != modePalette {
		t.Fatalf("expected modePalette after ctrl+p, got %d", m.mode)
	}
	if !strings.Contains(m.View(), "Commands") {
		t.Fatal("palette view must render its title")
	}
}

func TestPaletteRunsClearViaConfirmation(t *testing.T) {
	m := newModel()
	m = resize(t, m, 100, 28)

	mi, _ := m.Update(commandpalette.SelectMsg{Command: commandpalette.Command{ID: "clear"}})
	m = mi.(model)
	if m.mode != modeClearConfirm {
		t.Fatalf("expected modeClearConfirm after selecting clear, got %d", m.mode)
	}

	mi, _ = m.Update(confirmation.ConfirmMsg{Value: true})
	m = mi.(model)
	if m.msgCount != 0 {
		t.Fatalf("expected msgCount=0 after confirmed clear, got %d", m.msgCount)
	}
	if m.mode != modeChat {
		t.Fatal("returning from confirm should restore chat mode")
	}
}

func TestSavedialogTextInputFlow(t *testing.T) {
	m := newModel()
	m = resize(t, m, 100, 28)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = mi.(model)
	if m.mode != modeSaveDialog {
		t.Fatalf("expected modeSaveDialog after ctrl+s, got %d", m.mode)
	}

	mi, _ = m.Update(textinput.SubmitMsg{Value: "session.md"})
	m = mi.(model)
	if m.mode != modeChat {
		t.Fatal("save submit should return to chat mode")
	}
	if got := len(m.tray.Toasts()); got != 1 {
		t.Fatalf("expected one toast after save, got %d", got)
	}
}

func TestModelPickerSwitchesModelName(t *testing.T) {
	m := newModel()
	m = resize(t, m, 100, 28)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = mi.(model)
	if m.mode != modeModelPicker {
		t.Fatalf("expected modeModelPicker, got %d", m.mode)
	}

	mi, _ = m.Update(selectinput.SelectMsg{Option: selectinput.Option{Label: "Haiku 4.5"}})
	m = mi.(model)
	if m.modelName != "Haiku 4.5" {
		t.Fatalf("expected modelName=Haiku 4.5, got %s", m.modelName)
	}
	if m.mode != modeChat {
		t.Fatal("returning from picker should restore chat mode")
	}
}

func TestToastsExpireOnTick(t *testing.T) {
	m := newModel()
	m = resize(t, m, 100, 28)
	mi, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlM})
	m = mi.(model)
	mi, _ = m.Update(selectinput.SelectMsg{Option: selectinput.Option{Label: "Sonnet 4.6"}})
	m = mi.(model)
	if got := len(m.tray.Toasts()); got == 0 {
		t.Fatal("expected at least one toast from model switch")
	}
	mi, _ = m.Update(tickMsg(time.Now().Add(1 * time.Hour)))
	m = mi.(model)
	if got := len(m.tray.Toasts()); got != 0 {
		t.Fatalf("expected toasts cleared after far-future tick, got %d", got)
	}
}

func TestCtrlCAlwaysQuits(t *testing.T) {
	m := newModel()
	m = resize(t, m, 100, 28)
	for _, mode := range []mode{modeChat, modePalette, modeSaveDialog, modeClearConfirm, modeModelPicker} {
		mm := m
		mm.mode = mode
		mi, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		_ = mi
		if cmd == nil {
			t.Fatalf("ctrl+c in mode %d returned no command (expected tea.Quit)", mode)
		}
		// tea.Quit is a function value; just confirm it produces a tea.QuitMsg.
		out := cmd()
		if _, ok := out.(tea.QuitMsg); !ok {
			t.Fatalf("ctrl+c in mode %d did not produce QuitMsg, got %T", mode, out)
		}
	}
}
