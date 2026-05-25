// Package signature renders nook's LSP signature-help overlay: a small
// floating box that shows the call signature of the function the cursor
// is inside, with the parameter that would receive the next argument
// highlighted. Auto-fires when the user types '(' inside a buffer
// connected to a language server; auto-closes on ')' or Esc.
//
// The pane is a pure value type — every mutator returns an updated copy
// — mirroring the outline / lookup pattern so the host pumps it as
// `m.sigPane = m.sigPane.Open(info)` with no pointer aliasing surprises.
//
// Overload cycling (Alt+↓/↑ between multiple matching signatures) is
// scoped out for v0.26; the LSP response carries ActiveSignature and we
// honor it, but the user can't currently switch between overloads.
package signature

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/components/theme"
)

const (
	// defaultWidth is the width we render at when the caller passes zero.
	defaultWidth = 64
	// minWidth is the smallest width we'll bother painting at. Below this
	// the overlay becomes unreadable and the host should skip rendering.
	minWidth = 24
	// maxDocLines clamps the signature/parameter documentation so a
	// chatty server (typescript-language-server is the worst offender)
	// doesn't blow out the editor real estate.
	maxDocLines = 4
)

// Pane carries the open/closed state and the most recent signature info
// from the server. Construct via New().
type Pane struct {
	open  bool
	width int
	info  lsp.SignatureInfo
}

// New returns a closed pane.
func New() Pane {
	return Pane{width: defaultWidth, info: lsp.SignatureInfo{ActiveSignature: -1}}
}

// WithSize returns a copy of the pane with the given render width. Width
// less than minWidth is clamped to minWidth.
func (p Pane) WithSize(width int) Pane {
	if width < minWidth {
		width = minWidth
	}
	p.width = width
	return p
}

// Width returns the pane's render width.
func (p Pane) Width() int { return p.width }

// Open returns a copy of the pane displaying the given signature info.
// An empty Signatures slice (or one without any valid active signature)
// returns a closed pane, so the host can call this with whatever the
// server responded without checking first.
func (p Pane) Open(info lsp.SignatureInfo) Pane {
	if len(info.Signatures) == 0 {
		p.open = false
		p.info = lsp.SignatureInfo{ActiveSignature: -1}
		return p
	}
	if info.ActiveSignature < 0 || info.ActiveSignature >= len(info.Signatures) {
		info.ActiveSignature = 0
	}
	p.open = true
	p.info = info
	return p
}

// Close returns a copy of the pane with no active signature.
func (p Pane) Close() Pane {
	p.open = false
	p.info = lsp.SignatureInfo{ActiveSignature: -1}
	return p
}

// IsOpen reports whether the pane is currently displaying a signature.
func (p Pane) IsOpen() bool { return p.open && len(p.info.Signatures) > 0 }

// Info returns the underlying signature info. Useful for tests.
func (p Pane) Info() lsp.SignatureInfo { return p.info }

// ActiveSignature returns the active signature, or the zero value when
// the pane is closed. Callers should gate on IsOpen first.
func (p Pane) ActiveSignature() lsp.Signature {
	if !p.IsOpen() {
		return lsp.Signature{ActiveParameter: -1}
	}
	return p.info.Signatures[p.info.ActiveSignature]
}

// View renders the overlay. Returns "" when the pane is closed so the
// host can render unconditionally and let the empty string collapse out
// of the layout.
func (p Pane) View(t theme.Theme) string {
	if !p.IsOpen() {
		return ""
	}
	width := p.width
	if width < minWidth {
		width = minWidth
	}
	// Reserve 4 cells for border + padding.
	inner := width - 4
	if inner < 12 {
		inner = 12
	}

	sig := p.ActiveSignature()
	sigLine := renderSignatureLabel(t, sig, inner)

	var body strings.Builder
	body.WriteString(sigLine)

	// Overload counter, e.g. "1 of 3" when multiple signatures.
	if len(p.info.Signatures) > 1 {
		counter := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Italic(true).
			Render(formatCounter(p.info.ActiveSignature+1, len(p.info.Signatures)))
		body.WriteString("\n")
		body.WriteString(counter)
	}

	// Parameter doc (if available) for the active parameter.
	if sig.ActiveParameter >= 0 && sig.ActiveParameter < len(sig.Parameters) {
		if doc := strings.TrimSpace(sig.Parameters[sig.ActiveParameter].Doc); doc != "" {
			body.WriteString("\n")
			body.WriteString(wrapAndClamp(doc, inner, maxDocLines, t.TextMuted))
		}
	}

	// Signature-level doc. Skip when redundant with the param doc, i.e.
	// when both exist and look identical — gopls sometimes echoes the
	// summary into both fields.
	sigDoc := strings.TrimSpace(sig.Doc)
	paramDoc := ""
	if sig.ActiveParameter >= 0 && sig.ActiveParameter < len(sig.Parameters) {
		paramDoc = strings.TrimSpace(sig.Parameters[sig.ActiveParameter].Doc)
	}
	if sigDoc != "" && sigDoc != paramDoc {
		body.WriteString("\n")
		body.WriteString(wrapAndClamp(sigDoc, inner, maxDocLines, t.TextMuted))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Foreground(t.Text).
		Background(t.Surface).
		Padding(0, 1).
		Width(inner).
		Render(body.String())
}

// renderSignatureLabel paints the signature with the active parameter
// region highlighted via reversed style. When no active parameter is
// resolvable the full label renders plain.
func renderSignatureLabel(t theme.Theme, sig lsp.Signature, inner int) string {
	runes := []rune(sig.Label)
	if sig.ActiveParameter < 0 || sig.ActiveParameter >= len(sig.Parameters) {
		return clampLine(sig.Label, inner)
	}
	p := sig.Parameters[sig.ActiveParameter]
	if p.End <= p.Start || p.Start < 0 || p.End > len(runes) {
		return clampLine(sig.Label, inner)
	}
	before := string(runes[:p.Start])
	mid := string(runes[p.Start:p.End])
	after := string(runes[p.End:])

	hl := lipgloss.NewStyle().
		Foreground(t.TextInverse).
		Background(t.Primary).
		Bold(true).
		Render(mid)

	combined := before + hl + after
	return clampLine(combined, inner)
}

// clampLine truncates a single line to width display cells, appending
// "…" when truncated. Width-aware via lipgloss so styled glyphs count
// correctly.
func clampLine(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i]) + "…"
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}
	return "…"
}

// wrapAndClamp hard-wraps each input line at width characters then
// keeps the first maxLines rows, replacing the final row with a "…"
// marker when the original content was longer. The result is styled
// with the muted color so docs read as supporting text rather than
// command surface.
func wrapAndClamp(s string, width, maxLines int, color lipgloss.Color) string {
	var rows []string
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			rows = append(rows, "")
			continue
		}
		for len([]rune(line)) > width {
			r := []rune(line)
			rows = append(rows, string(r[:width]))
			line = string(r[width:])
		}
		rows = append(rows, line)
	}
	if len(rows) > maxLines {
		rows = rows[:maxLines]
		rows[len(rows)-1] = strings.TrimRight(rows[len(rows)-1], " ") + " …"
	}
	out := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Foreground(color).Render(out)
}

// formatCounter formats an "n of m" overload indicator.
func formatCounter(n, total int) string {
	return strconv.Itoa(n) + " of " + strconv.Itoa(total)
}
