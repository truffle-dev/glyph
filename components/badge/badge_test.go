package badge

import (
	"strings"
	"testing"
)

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEsc = true
		case inEsc && (r == 'm' || r == 'K'):
			inEsc = false
		case inEsc:
			// skip
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestEmptyLabelRendersEmpty(t *testing.T) {
	if got := New("").Render(); got != "" {
		t.Errorf("empty label should render empty, got %q", got)
	}
	if got := New("").Success().Outline().Render(); got != "" {
		t.Errorf("empty label should render empty regardless of options, got %q", got)
	}
}

func TestNewDefaultsToNeutralFilled(t *testing.T) {
	b := New("v0.49")
	if b.variant != Neutral {
		t.Errorf("new badge should be Neutral, got %v", b.variant)
	}
	if b.outline {
		t.Errorf("new badge should be filled, not outline")
	}
}

func TestLabelSurvivesRender(t *testing.T) {
	out := stripANSI(New("LIVE").Success().Render())
	if !strings.Contains(out, "LIVE") {
		t.Errorf("rendered badge should contain its label, got %q", out)
	}
}

func TestLabelAccessorIgnoresCaseFolding(t *testing.T) {
	b := New("beta").Uppercase()
	if b.Label() != "beta" {
		t.Errorf("Label() should return the original label, got %q", b.Label())
	}
}

func TestUppercaseFoldsRenderedLabel(t *testing.T) {
	out := stripANSI(New("beta").Uppercase().Warning().Render())
	if !strings.Contains(out, "BETA") {
		t.Errorf("uppercase badge should render folded label, got %q", out)
	}
	if strings.Contains(out, "beta") {
		t.Errorf("uppercase badge should not retain lower-case label, got %q", out)
	}
}

func TestVariantSettersSetVariant(t *testing.T) {
	cases := []struct {
		name string
		got  Variant
		want Variant
	}{
		{"primary", New("x").Primary().variant, Primary},
		{"success", New("x").Success().variant, Success},
		{"warning", New("x").Warning().variant, Warning},
		{"error", New("x").Error().variant, Error},
		{"info", New("x").Info().variant, Info},
		{"neutral", New("x").Success().Neutral().variant, Neutral},
		{"withvariant", New("x").WithVariant(Info).variant, Info},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got variant %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestOutlineAndFilledToggleAppearance(t *testing.T) {
	if !New("x").Outline().outline {
		t.Errorf("Outline() should set outline")
	}
	if New("x").Outline().Filled().outline {
		t.Errorf("Filled() should clear outline")
	}
}

func TestOutlineAndFilledProduceDifferentOutput(t *testing.T) {
	filled := New("PASS").Success().Render()
	outline := New("PASS").Success().Outline().Render()
	if filled == outline {
		t.Errorf("filled and outline appearances should differ, both = %q", filled)
	}
	// Both still carry the label once styling is stripped.
	if !strings.Contains(stripANSI(filled), "PASS") || !strings.Contains(stripANSI(outline), "PASS") {
		t.Errorf("both appearances should contain the label: filled=%q outline=%q",
			stripANSI(filled), stripANSI(outline))
	}
}

func TestBuilderIsImmutable(t *testing.T) {
	base := New("x")
	_ = base.Success().Outline().Uppercase()
	if base.variant != Neutral || base.outline || base.uppercase {
		t.Errorf("chained options must not mutate the receiver, got %+v", base)
	}
}
