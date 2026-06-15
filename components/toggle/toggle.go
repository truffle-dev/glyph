// Package toggle renders a single-line boolean switch with keyboard
// control. It owns one on/off value and the keyboard handling;
// consumers set the initial state via builders and read
// ToggleChangedMsg back when the state flips.
//
// Keys: space and enter flip the current state, left/h forces off,
// right/l forces on, and the lowercase letters y and n set on and off
// directly. Each key that changes the value emits ToggleChangedMsg;
// keys that land on the value already held emit nothing.
//
// WithOnLabel and WithOffLabel override the captions rendered next to
// the switch (defaults "On" and "Off"). WithLabel sets an optional
// leading label, e.g. "Wrap: ". WithDisabled freezes the switch so
// keys are ignored and the track renders muted. The rendered switch
// reads as a pill: a filled knob sits on the right when on and the
// left when off, recolored to Success when on and TextMuted when off.
package toggle

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// ToggleChangedMsg fires when the value actually changes via Update.
type ToggleChangedMsg struct {
	On bool
}

const (
	trackOn  = "──"
	trackOff = "──"
	knob     = "●"
)

// Model is the Bubble Tea state of the toggle.
type Model struct {
	th       theme.Theme
	on       bool
	disabled bool
	label    string
	onLabel  string
	offLabel string
	showText bool
}

// New constructs a Model with Default theme, off state, captions
// "On" and "Off", and the trailing caption shown.
func New() Model {
	return Model{
		th:       theme.Default,
		on:       false,
		onLabel:  "On",
		offLabel: "Off",
		showText: true,
	}
}

// WithTheme overrides the theme palette.
func (m Model) WithTheme(t theme.Theme) Model { m.th = t; return m }

// WithOn sets the initial on/off state.
func (m Model) WithOn(on bool) Model { m.on = on; return m }

// WithDisabled toggles disabled state. A disabled switch ignores key
// input and renders with muted styles.
func (m Model) WithDisabled(d bool) Model { m.disabled = d; return m }

// WithLabel sets an optional leading label rendered before the
// switch, e.g. "Wrap: " so the rendered switch reads "Wrap: ●── On".
func (m Model) WithLabel(s string) Model { m.label = s; return m }

// WithOnLabel overrides the caption shown when on. Default "On".
func (m Model) WithOnLabel(s string) Model { m.onLabel = s; return m }

// WithOffLabel overrides the caption shown when off. Default "Off".
func (m Model) WithOffLabel(s string) Model { m.offLabel = s; return m }

// WithShowText toggles the trailing caption display. Default is true.
func (m Model) WithShowText(on bool) Model { m.showText = on; return m }

// On returns the current state.
func (m Model) On() bool { return m.on }

// Disabled reports whether the switch ignores key input.
func (m Model) Disabled() bool { return m.disabled }

// Init satisfies tea.Model. No initial command.
func (m Model) Init() tea.Cmd { return nil }

// Update handles toggle keys. Returns a ToggleChangedMsg command only
// when the value actually changes. A disabled switch ignores all
// input.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.disabled {
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	prev := m.on
	switch key.String() {
	case " ", "enter":
		m.on = !m.on
	case "left", "h", "n":
		m.on = false
	case "right", "l", "y":
		m.on = true
	default:
		return m, nil
	}
	if m.on == prev {
		return m, nil
	}
	v := m.on
	return m, func() tea.Msg {
		return ToggleChangedMsg{On: v}
	}
}

// View renders the switch.
func (m Model) View() string {
	knobStyle := lipgloss.NewStyle().Foreground(m.th.Success).Bold(true)
	trackStyle := lipgloss.NewStyle().Foreground(m.th.Success)
	textStyle := lipgloss.NewStyle().Foreground(m.th.Text)
	labelStyle := lipgloss.NewStyle().Foreground(m.th.TextMuted)
	if !m.on {
		knobStyle = lipgloss.NewStyle().Foreground(m.th.TextMuted).Bold(true)
		trackStyle = lipgloss.NewStyle().Foreground(m.th.TextMuted)
	}
	if m.disabled {
		knobStyle = lipgloss.NewStyle().Foreground(m.th.TextMuted).Bold(true)
		trackStyle = lipgloss.NewStyle().Foreground(m.th.TextMuted)
		textStyle = lipgloss.NewStyle().Foreground(m.th.TextMuted)
	}

	var sw string
	if m.on {
		sw = trackStyle.Render(trackOn) + knobStyle.Render(knob)
	} else {
		sw = knobStyle.Render(knob) + trackStyle.Render(trackOff)
	}

	out := sw
	if m.label != "" {
		out = labelStyle.Render(m.label) + out
	}
	if m.showText {
		out += " " + textStyle.Render(m.caption())
	}
	return out
}

func (m Model) caption() string {
	if m.on {
		return m.onLabel
	}
	return m.offLabel
}
