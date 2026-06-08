package rangeslider

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestNewDefaults(t *testing.T) {
	m := New()
	if m.Min() != 0 {
		t.Errorf("New().Min() = %v, want 0", m.Min())
	}
	if m.Max() != 100 {
		t.Errorf("New().Max() = %v, want 100", m.Max())
	}
	if m.Step() != 1 {
		t.Errorf("New().Step() = %v, want 1", m.Step())
	}
	if m.Value() != 0 {
		t.Errorf("New().Value() = %v, want 0", m.Value())
	}
	if !m.AtMin() || m.AtMax() {
		t.Errorf("new slider should be AtMin and not AtMax, got min=%v max=%v", m.AtMin(), m.AtMax())
	}
}

func TestWithMinSwapsWhenAboveMax(t *testing.T) {
	m := New().WithMax(10).WithMin(50)
	if m.Min() > m.Max() {
		t.Errorf("WithMin past max should swap to keep min <= max, got [%v, %v]", m.Min(), m.Max())
	}
}

func TestWithMaxSwapsWhenBelowMin(t *testing.T) {
	m := New().WithMin(50).WithMax(10)
	if m.Min() > m.Max() {
		t.Errorf("WithMax below min should swap to keep min <= max, got [%v, %v]", m.Min(), m.Max())
	}
}

func TestWithStepClampsToOne(t *testing.T) {
	m := New().WithStep(0)
	if m.Step() != 1 {
		t.Errorf("WithStep(0) should clamp to 1, got %v", m.Step())
	}
	m = New().WithStep(-5)
	if m.Step() != 1 {
		t.Errorf("WithStep(-5) should clamp to 1, got %v", m.Step())
	}
}

func TestWithValueClampsInRange(t *testing.T) {
	m := New().WithValue(-50)
	if m.Value() != 0 {
		t.Errorf("below-min value should clamp to min, got %v", m.Value())
	}
	m = New().WithValue(500)
	if m.Value() != 100 {
		t.Errorf("above-max value should clamp to max, got %v", m.Value())
	}
}

func TestWithWidthClampsToThree(t *testing.T) {
	m := New().WithWidth(0)
	out := m.View()
	if !strings.Contains(out, thumbGlyph) {
		t.Errorf("width-clamped track should still render thumb, got %q", out)
	}
}

func TestWithPrecisionClampsToZero(t *testing.T) {
	m := New().WithPrecision(-3).WithValue(42)
	out := m.View()
	if !strings.Contains(out, "42") {
		t.Errorf("negative precision should clamp to 0, got %q", out)
	}
	if strings.Contains(out, "42.") {
		t.Errorf("precision 0 should not render a decimal, got %q", out)
	}
}

func TestUpdateRightAdvances(t *testing.T) {
	m := New()
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Value() != 1 {
		t.Errorf("right should advance by step (1), got %v", m.Value())
	}
	if cmd == nil {
		t.Fatal("value change should emit a tea.Cmd")
	}
	msg, ok := cmd().(ValueChangedMsg)
	if !ok {
		t.Fatalf("expected ValueChangedMsg, got %T", cmd())
	}
	if msg.Value != 1 {
		t.Errorf("ValueChangedMsg = %+v, want {1}", msg)
	}
}

func TestUpdateLeftRetreats(t *testing.T) {
	m := New().WithValue(50)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.Value() != 49 {
		t.Errorf("left should retreat by step, got %v", m.Value())
	}
	if cmd == nil {
		t.Fatal("value change should emit a tea.Cmd")
	}
}

func TestUpdateLeftAtMinIsNoOp(t *testing.T) {
	m := New()
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.Value() != 0 {
		t.Errorf("left at min should stay at min, got %v", m.Value())
	}
	if cmd != nil {
		t.Fatal("no value change should mean no tea.Cmd")
	}
}

func TestUpdateRightAtMaxIsNoOp(t *testing.T) {
	m := New().WithValue(100)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Value() != 100 {
		t.Errorf("right at max should stay at max, got %v", m.Value())
	}
	if cmd != nil {
		t.Fatal("no value change should mean no tea.Cmd")
	}
}

func TestUpdateHomeJumpsToMin(t *testing.T) {
	m := New().WithValue(50)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.Value() != 0 {
		t.Errorf("home should jump to min, got %v", m.Value())
	}
}

func TestUpdateEndJumpsToMax(t *testing.T) {
	m := New().WithValue(50)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.Value() != 100 {
		t.Errorf("end should jump to max, got %v", m.Value())
	}
}

func TestUpdatePageUpJumpsTenSteps(t *testing.T) {
	m := New().WithValue(50)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.Value() != 40 {
		t.Errorf("pgup should retreat 10×step, got %v", m.Value())
	}
}

func TestUpdatePageDownJumpsTenSteps(t *testing.T) {
	m := New().WithValue(50)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.Value() != 60 {
		t.Errorf("pgdown should advance 10×step, got %v", m.Value())
	}
}

func TestUpdateVimKeys(t *testing.T) {
	m := New().WithValue(50)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.Value() != 51 {
		t.Errorf("l should advance, got %v", m.Value())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.Value() != 50 {
		t.Errorf("h should retreat, got %v", m.Value())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if m.Value() != 100 {
		t.Errorf("G should jump to max, got %v", m.Value())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if m.Value() != 0 {
		t.Errorf("g should jump to min, got %v", m.Value())
	}
	m = m.WithValue(50)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if m.Value() != 60 {
		t.Errorf("J should advance 10×step, got %v", m.Value())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if m.Value() != 50 {
		t.Errorf("K should retreat 10×step, got %v", m.Value())
	}
}

func TestUpdateIgnoresUnrelatedKeys(t *testing.T) {
	m := New().WithValue(50)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Fatal("unrelated keys should not emit a cmd")
	}
}

func TestUpdateIgnoresNonKeyMessages(t *testing.T) {
	m := New().WithValue(50)
	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m2.Value() != m.Value() {
		t.Errorf("non-key message should not change value")
	}
	if cmd != nil {
		t.Fatal("non-key message should not emit a cmd")
	}
}

func TestUpdateDisabledIgnoresKeys(t *testing.T) {
	m := New().WithValue(50).WithDisabled(true)
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m2.Value() != 50 {
		t.Errorf("disabled slider should ignore keys, got %v", m2.Value())
	}
	if cmd != nil {
		t.Fatal("disabled slider should not emit a cmd")
	}
}

func TestPercentZeroWhenMinEqualsMax(t *testing.T) {
	m := New().WithMin(50).WithMax(50)
	if got := m.Percent(); got != 0 {
		t.Errorf("Percent on degenerate range should be 0, got %v", got)
	}
}

func TestPercentMiddle(t *testing.T) {
	m := New().WithValue(50)
	if got := m.Percent(); got != 0.5 {
		t.Errorf("Percent at midpoint should be 0.5, got %v", got)
	}
}

func TestViewIncludesValue(t *testing.T) {
	m := New().WithValue(42)
	out := m.View()
	if !strings.Contains(out, "42") {
		t.Errorf("View should include the value, got %q", out)
	}
}

func TestViewWithLabel(t *testing.T) {
	m := New().WithValue(50).WithLabel("Volume: ")
	out := stripANSI(m.View())
	if !strings.HasPrefix(out, "Volume: ") {
		t.Errorf("View should start with label, got %q", out)
	}
}

func TestViewWithUnits(t *testing.T) {
	m := New().WithValue(75).WithUnits("%")
	out := m.View()
	if !strings.Contains(out, "75%") {
		t.Errorf("View should include units suffix, got %q", out)
	}
}

func TestViewHidesValueWhenShowValueFalse(t *testing.T) {
	m := New().WithValue(42).WithShowValue(false)
	out := stripANSI(m.View())
	if strings.Contains(out, "42") {
		t.Errorf("View should not include value when ShowValue is off, got %q", out)
	}
}

func TestViewCustomFormatter(t *testing.T) {
	m := New().WithValue(42).WithFormatter(func(v float64) string {
		return fmt.Sprintf("[%v]", int(v))
	})
	out := m.View()
	if !strings.Contains(out, "[42]") {
		t.Errorf("View should honor custom formatter, got %q", out)
	}
}

func TestAtMinAndAtMax(t *testing.T) {
	m := New()
	if !m.AtMin() {
		t.Errorf("value 0 should be AtMin with default range")
	}
	if m.AtMax() {
		t.Errorf("value 0 should not be AtMax with default range")
	}
	m = m.WithValue(100)
	if m.AtMin() {
		t.Errorf("value 100 should not be AtMin with default range")
	}
	if !m.AtMax() {
		t.Errorf("value 100 should be AtMax with default range")
	}
}

func TestValueChangedMsgNotEmittedOnNoOp(t *testing.T) {
	m := New()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if cmd != nil {
		t.Fatal("home at min should not emit ValueChangedMsg")
	}
	m = New().WithValue(100)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if cmd != nil {
		t.Fatal("end at max should not emit ValueChangedMsg")
	}
}

func TestWithThemeAppliesPalette(t *testing.T) {
	m := New().WithTheme(theme.Default).WithValue(50)
	if !strings.Contains(m.View(), "50") {
		t.Errorf("WithTheme should not break rendering")
	}
}

func TestFractionalRangeAndStep(t *testing.T) {
	m := New().WithMin(0).WithMax(1).WithStep(0.1).WithValue(0.5).WithPrecision(1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Value() < 0.59 || m.Value() > 0.61 {
		t.Errorf("right on fractional slider should land near 0.6, got %v", m.Value())
	}
	if !strings.Contains(m.View(), "0.6") {
		t.Errorf("fractional view should render with one decimal, got %q", m.View())
	}
}

// stripANSI removes ANSI escape sequences for prefix-matching tests.
func stripANSI(s string) string {
	out := ""
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		out += string(s[i])
	}
	return out
}
