// Package welcome renders nook's first-run / empty-editor surface.
//
// When the host model has no file open, the editor pane would otherwise
// render a column of tilde-fill lines (vim-style empty buffer). That works
// for someone who already knows what nook is and how to open a file. For
// a brand-new user it reads as "the binary did nothing." This package
// replaces that surface with a centered welcome card that names the
// project, surfaces AI/LSP availability, and lists the quick-start keys.
//
// View() returns a string ready to drop into the host's main column. It
// owns its own width/height clamp and centers its content so the host can
// keep its layout math agnostic.
package welcome

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Status names the runtime availability of an optional capability. The
// welcome card uses it to render dot+label rows so users see what's wired
// and what isn't before they type their first key.
type Status struct {
	Label  string // "claude CLI ready", "gopls not found", etc.
	OK     bool   // true → green ●, false → muted ○
	Detail string // optional second line; rendered muted under the label
}

// Info is the everything-the-welcome-needs-to-render bundle. Callers
// populate it from the host model + capability probes.
type Info struct {
	Root      string
	FileCount int
	AI        Status
	LSP       Status
}

// View renders the welcome card at the given size. If the terminal is too
// small to host the full card, a compact fallback renders instead so the
// pane never falls back to the raw tilde-fill.
func View(t theme.Theme, info Info, width, height int) string {
	if width < 50 || height < 14 {
		return compactView(t, info, width, height)
	}
	return fullView(t, info, width, height)
}

func fullView(t theme.Theme, info Info, width, height int) string {
	wordmark := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Render("n  o  o  k")

	tagline := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Italic(true).
		Render("a terminal-native AI IDE")

	label := lipgloss.NewStyle().Foreground(t.TextMuted)
	value := lipgloss.NewStyle().Foreground(t.Text)
	dim := lipgloss.NewStyle().Foreground(t.TextMuted).Faint(true)

	projectName := filepath.Base(info.Root)
	if projectName == "" || projectName == "." || projectName == "/" {
		projectName = info.Root
	}

	rows := []string{
		row(label, value, "project", projectName),
		row(label, value, "path", truncatePath(info.Root, 56)),
		row(label, value, "files", fmt.Sprintf("%d", info.FileCount)),
		"",
		statusRow(t, label, "AI", info.AI),
		statusRow(t, label, "LSP", info.LSP),
	}

	sep := lipgloss.NewStyle().
		Foreground(t.Border).
		Render(strings.Repeat("─", 56))

	keysHeader := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Render("Quick start")

	keys := [][2]string{
		{"ctrl+p", "Open file"},
		{"ctrl+f", "Search the project"},
		{"ctrl+g", "Git pane"},
		{"ctrl+`", "Embedded terminal"},
		{"ctrl+k", "AI inline edit on cursor line"},
		{"ctrl+l", "AI multi-file composer"},
		{"?", "Show all keybindings"},
		{"ctrl+q", "Quit"},
	}
	keyStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(t.Text)
	var keyRows []string
	for _, k := range keys {
		keyRows = append(keyRows,
			fmt.Sprintf("  %-12s  %s",
				keyStyle.Render(k[0]),
				descStyle.Render(k[1]),
			),
		)
	}

	hint := lipgloss.NewStyle().
		Foreground(t.Primary).
		Render("Press ctrl+p to find a file and start editing.")

	hintMeta := dim.Render("(Or press ? to see every keybinding.)")

	card := strings.Join([]string{
		wordmark,
		tagline,
		"",
		strings.Join(rows, "\n"),
		"",
		sep,
		keysHeader,
		"",
		strings.Join(keyRows, "\n"),
		"",
		hint,
		hintMeta,
	}, "\n")

	return centerBlock(card, width, height)
}

func compactView(t theme.Theme, info Info, width, height int) string {
	wordmark := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Render("nook")
	hint := lipgloss.NewStyle().
		Foreground(t.Text).
		Render("ctrl+p open · ctrl+f search · ? help · ctrl+q quit")
	files := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Render(fmt.Sprintf("%d files in %s", info.FileCount, filepath.Base(info.Root)))
	body := strings.Join([]string{wordmark, files, "", hint}, "\n")
	return centerBlock(body, width, height)
}

func row(label, value lipgloss.Style, k, v string) string {
	return fmt.Sprintf("  %s  %s",
		label.Render(fmt.Sprintf("%-9s", k)),
		value.Render(v),
	)
}

func statusRow(t theme.Theme, label lipgloss.Style, k string, s Status) string {
	var dot string
	if s.OK {
		dot = lipgloss.NewStyle().Foreground(t.Success).Render("●")
	} else {
		dot = lipgloss.NewStyle().Foreground(t.TextMuted).Faint(true).Render("○")
	}
	main := fmt.Sprintf("  %s  %s %s",
		label.Render(fmt.Sprintf("%-9s", k)),
		dot,
		lipgloss.NewStyle().Foreground(t.Text).Render(s.Label),
	)
	if s.Detail == "" {
		return main
	}
	detail := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Faint(true).
		Render(fmt.Sprintf("             %s", s.Detail))
	return main + "\n" + detail
}

func centerBlock(body string, width, height int) string {
	bw, bh := lipgloss.Size(body)
	if bw > width {
		bw = width
	}
	if bh > height {
		bh = height
	}
	leftPad := (width - bw) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	topPad := (height - bh) / 2
	if topPad < 0 {
		topPad = 0
	}
	pad := lipgloss.NewStyle().
		Padding(topPad, leftPad, 0, leftPad).
		Render(body)
	return pad
}

// truncatePath returns p clamped to max display columns. When p is longer,
// the head is elided with "…" so the final path segment (the most useful
// part — usually the current directory) stays visible. max counts runes,
// not bytes, so the multi-byte ellipsis doesn't blow the budget on
// non-ASCII paths.
func truncatePath(p string, max int) string {
	if max <= 1 {
		return "…"
	}
	runes := []rune(p)
	if len(runes) <= max {
		return p
	}
	return "…" + string(runes[len(runes)-max+1:])
}

// ProbeAI returns a Status describing whether the claude CLI is reachable.
// Callers thread the result of ai.Available() in instead, but this helper
// keeps the welcome package self-sufficient when used standalone (tests,
// snap-tours).
func ProbeAI(available bool) Status {
	if available {
		return Status{
			Label:  "claude CLI ready",
			OK:     true,
			Detail: "Haiku 4.5 for inline edits · Sonnet 4.6 for composer",
		}
	}
	return Status{
		Label:  "claude CLI not on PATH — AI wedges disabled",
		OK:     false,
		Detail: "Install: npm i -g @anthropic-ai/claude-code",
	}
}

// ProbeLSP returns a Status describing whether gopls is reachable. nook's
// LSP wiring is gopls-only today, so this is a binary "do we have it"
// check. The probe runs synchronously since exec.LookPath is just a stat.
func ProbeLSP() Status {
	if _, err := exec.LookPath("gopls"); err == nil {
		return Status{
			Label:  "gopls available",
			OK:     true,
			Detail: "diagnostics light up on any open .go file",
		}
	}
	return Status{
		Label:  "gopls not on PATH — Go diagnostics off",
		OK:     false,
		Detail: "Install: go install golang.org/x/tools/gopls@latest",
	}
}
