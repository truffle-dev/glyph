// Package rangeslider renders a single-line horizontal slider over a
// continuous numeric range with keyboard navigation. It owns the
// current value, the [min, max] bounds, the step size, and the
// keyboard handling; consumers feed it value changes via builders and
// read ValueChangedMsg back when the value moves.
//
// Keys: left/h steps the value down by one step, right/l steps it
// up by one step, home/g jumps to min, end/G jumps to max, pgup/K
// and pgdown/J step by 10×step. Motion clamps at both ends; nothing
// wraps. Each motion that changes the value emits ValueChangedMsg.
//
// WithPrecision controls how many decimal digits the displayed value
// uses; the default is 0. WithFormatter overrides the formatter
// entirely. WithShowValue toggles whether the value reads alongside
// the track. WithDisabled freezes the bar so keys are ignored and
// the track renders muted.
package rangeslider

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// ValueChangedMsg fires when the value actually changes via Update.
type ValueChangedMsg struct {
	Value float64
}

const (
	trackFilled   = "━"
	trackUnfilled = "─"
	thumbGlyph    = "●"
)

// Model is the Bubble Tea state of the range slider.
type Model struct {
	th        theme.Theme
	min       float64
	max       float64
	step      float64
	value     float64
	width     int
	precision int
	formatter func(float64) string
	disabled  bool
	label     string
	units     string
	showValue bool
}

// New constructs a Model with Default theme, range [0, 100], step 1,
// value 0, width 20 cells, precision 0, and value display enabled.
func New() Model {
	return Model{
		th:        theme.Default,
		min:       0,
		max:       100,
		step:      1,
		value:     0,
		width:     20,
		precision: 0,
		showValue: true,
	}
}

// WithTheme overrides the theme palette.
func (m Model) WithTheme(t theme.Theme) Model { m.th = t; return m }

// WithMin sets the lower bound. If min would exceed max, the two
// swap so the range stays well-ordered. The value re-clamps.
func (m Model) WithMin(v float64) Model {
	m.min = v
	if m.min > m.max {
		m.min, m.max = m.max, m.min
	}
	m.value = clamp(m.value, m.min, m.max)
	return m
}

// WithMax sets the upper bound. If max would fall below min, the
// two swap so the range stays well-ordered. The value re-clamps.
func (m Model) WithMax(v float64) Model {
	m.max = v
	if m.min > m.max {
		m.min, m.max = m.max, m.min
	}
	m.value = clamp(m.value, m.min, m.max)
	return m
}

// WithStep sets the per-keystroke step. Values <= 0 clamp to 1.
func (m Model) WithStep(v float64) Model {
	if v <= 0 {
		v = 1
	}
	m.step = v
	return m
}

// WithValue sets the current value. Out-of-range values clamp into
// the [min, max] range.
func (m Model) WithValue(v float64) Model {
	m.value = clamp(v, m.min, m.max)
	return m
}

// WithWidth sets the track render width in cells. Values < 3 clamp
// to 3 so the track always has room for a thumb plus context.
func (m Model) WithWidth(w int) Model {
	if w < 3 {
		w = 3
	}
	m.width = w
	return m
}

// WithPrecision sets the displayed decimal digit count. Negative
// values clamp to 0.
func (m Model) WithPrecision(p int) Model {
	if p < 0 {
		p = 0
	}
	m.precision = p
	return m
}

// WithFormatter overrides the default fmt.Sprintf("%.<precision>f")
// value formatting.
func (m Model) WithFormatter(f func(float64) string) Model {
	m.formatter = f
	return m
}

// WithDisabled toggles disabled state. A disabled bar ignores key
// input and renders with muted styles.
func (m Model) WithDisabled(d bool) Model { m.disabled = d; return m }

// WithLabel sets an optional leading label rendered before the
// track, e.g. "Volume: " so the rendered bar reads
// "Volume: ━━━━━●───── 50".
func (m Model) WithLabel(s string) Model { m.label = s; return m }

// WithUnits sets an optional trailing unit string appended to the
// formatted value, e.g. "%" so the value reads "50%".
func (m Model) WithUnits(s string) Model { m.units = s; return m }

// WithShowValue toggles the trailing value display. Default is true.
func (m Model) WithShowValue(on bool) Model { m.showValue = on; return m }

// Value returns the current numeric value.
func (m Model) Value() float64 { return m.value }

// Min returns the lower bound.
func (m Model) Min() float64 { return m.min }

// Max returns the upper bound.
func (m Model) Max() float64 { return m.max }

// Step returns the per-keystroke step size.
func (m Model) Step() float64 { return m.step }

// Percent returns the value's position in [0, 1] within [min, max].
// Returns 0 when min == max.
func (m Model) Percent() float64 {
	if m.max == m.min {
		return 0
	}
	return (m.value - m.min) / (m.max - m.min)
}

// Disabled reports whether the bar ignores key input.
func (m Model) Disabled() bool { return m.disabled }

// AtMin reports whether the current value is at the minimum.
func (m Model) AtMin() bool { return m.value <= m.min }

// AtMax reports whether the current value is at the maximum.
func (m Model) AtMax() bool { return m.value >= m.max }

// Init satisfies tea.Model. No initial command.
func (m Model) Init() tea.Cmd { return nil }

// Update handles motion keys. Returns a ValueChangedMsg command only
// when the value actually changes. A disabled bar ignores all input.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.disabled {
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	prev := m.value
	switch key.String() {
	case "left", "h":
		m.value = clamp(m.value-m.step, m.min, m.max)
	case "right", "l":
		m.value = clamp(m.value+m.step, m.min, m.max)
	case "home", "g":
		m.value = m.min
	case "end", "G":
		m.value = m.max
	case "pgup", "K":
		m.value = clamp(m.value-10*m.step, m.min, m.max)
	case "pgdown", "J":
		m.value = clamp(m.value+10*m.step, m.min, m.max)
	default:
		return m, nil
	}
	if m.value == prev {
		return m, nil
	}
	v := m.value
	return m, func() tea.Msg {
		return ValueChangedMsg{Value: v}
	}
}

// View renders the slider.
func (m Model) View() string {
	filledStyle := lipgloss.NewStyle().Foreground(m.th.Primary)
	unfilledStyle := lipgloss.NewStyle().Foreground(m.th.TextMuted)
	thumbStyle := lipgloss.NewStyle().Foreground(m.th.Primary).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(m.th.Text)
	labelStyle := lipgloss.NewStyle().Foreground(m.th.TextMuted)
	if m.disabled {
		filledStyle = lipgloss.NewStyle().Foreground(m.th.TextMuted)
		thumbStyle = lipgloss.NewStyle().Foreground(m.th.TextMuted)
		valueStyle = lipgloss.NewStyle().Foreground(m.th.TextMuted)
	}

	pos := int(m.Percent() * float64(m.width-1))
	if pos < 0 {
		pos = 0
	}
	if pos >= m.width {
		pos = m.width - 1
	}

	var track string
	for i := 0; i < m.width; i++ {
		switch {
		case i == pos:
			track += thumbStyle.Render(thumbGlyph)
		case i < pos:
			track += filledStyle.Render(trackFilled)
		default:
			track += unfilledStyle.Render(trackUnfilled)
		}
	}

	out := track
	if m.label != "" {
		out = labelStyle.Render(m.label) + out
	}
	if m.showValue {
		out += " " + valueStyle.Render(m.formatValue())
	}
	return out
}

func (m Model) formatValue() string {
	if m.formatter != nil {
		return m.formatter(m.value)
	}
	return fmt.Sprintf("%.*f%s", m.precision, m.value, m.units)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
