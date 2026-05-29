// Package help renders nook's full-keymap overlay.
//
// The host's status bar lists a single-line key reference, but that's
// reference, not discovery — users scanning the status bar miss the
// boundary between "ctrl+f search" and "ctrl+g git." This overlay groups
// every binding by job (Files, Editing, AI, Git, Terminal, LSP) and gives
// each one a single-sentence description. It's bound to `?` and dismissed
// by Esc or `?`.
//
// The view is intentionally fixed-width (a card, not full-screen) so it
// reads like documentation, not a panel. The host centers it.
package help

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Section is a group of related bindings shown under a single header.
type Section struct {
	Name     string
	Bindings []Binding
}

// Binding is a single key with its description. Key strings are rendered
// verbatim ("ctrl+p", "?", "esc"); descriptions are plain English.
type Binding struct {
	Key  string
	Desc string
}

// Default returns the canonical nook keymap. The order is intentional:
// navigation first (what people reach for in the first 30 seconds), AI
// in the middle (where the differentiation lives), then panes and
// miscellany. Update this list whenever a new binding lands in routeKey.
func Default() []Section {
	return []Section{
		{Name: "Files", Bindings: []Binding{
			{"ctrl+p", "Fuzzy file picker"},
			{"alt+f", "Project-wide search"},
			{"ctrl+s", "Save (formats first when LSP is attached)"},
			{"alt+s", "Save without formatting"},
			{"alt+shift+s", "Toggle format-on-save"},
			{"ctrl+w", "Close the current buffer"},
		}},
		{Name: "Buffers", Bindings: []Binding{
			{"alt+]", "Next open buffer"},
			{"alt+[", "Previous open buffer"},
		}},
		{Name: "Navigate", Bindings: []Binding{
			{"alt+-", "Jump back through history (vim's ctrl-o)"},
			{"alt+=", "Jump forward through history (vim's ctrl-i)"},
		}},
		{Name: "Find / Replace", Bindings: []Binding{
			{"ctrl+f", "Find in current buffer"},
			{"ctrl+h", "Find and replace in current buffer"},
			{"enter / ↓", "Next match"},
			{"↑", "Previous match"},
			{"ctrl+r", "Replace current match"},
			{"alt+r", "Replace all matches"},
			{"alt+x", "Toggle regex"},
			{"alt+c", "Toggle case-sensitive"},
			{"tab", "Switch between find / replace field"},
			{"esc", "Close find bar"},
		}},
		{Name: "Project search (alt+f)", Bindings: []Binding{
			{"type", "Build the query; enter runs ripgrep"},
			{"↑ ↓ pgup pgdn", "Navigate the result list"},
			{"enter", "Open the selected match"},
			{"alt+r", "Toggle replace-all mode (across every match)"},
			{"enter (replace)", "Apply the replacement to every match"},
			{"esc (replace)", "Collapse back to result navigation"},
			{"esc", "Cancel and dismiss the pane"},
		}},
		{Name: "Editing", Bindings: []Binding{
			{"↑ ↓ ← →", "Move cursor"},
			{"home", "Start of line"},
			{"end / ctrl+e", "End of line"},
			{"pgup / pgdn", "Page up / down"},
			{"backspace", "Delete previous character"},
			{"delete", "Delete next character"},
			{"enter", "Insert newline"},
			{"tab", "Insert tab (or accept ghost text)"},
			{"ctrl+/", "Toggle line comment on selection or cursor row"},
			{"( [ { \" ' `", "Auto-pair: typing an opener inserts its closer"},
		}},
		{Name: "Selection / Clipboard", Bindings: []Binding{
			{"shift+← →", "Extend selection one character"},
			{"shift+↑ ↓", "Extend selection one row"},
			{"shift+home / end", "Extend selection to start / end of line"},
			{"ctrl+a", "Select entire buffer"},
			{"ctrl+c", "Copy selection (or current line)"},
			{"ctrl+x", "Cut selection (or current line)"},
			{"ctrl+v", "Paste at cursor (replacing any selection)"},
			{"any movement", "Collapse selection"},
		}},
		{Name: "Multi-cursor", Bindings: []Binding{
			{"ctrl+d", "Add cursor at next match of word under cursor"},
			{"ctrl+↑", "Add cursor on row above (column edit)"},
			{"ctrl+↓", "Add cursor on row below (column edit)"},
			{"esc", "Clear extra cursors"},
			{"any movement", "Collapse to primary cursor"},
		}},
		{Name: "Multibuffer", Bindings: []Binding{
			{"alt+m", "Open uncommitted changes as one scrollable surface"},
			{"↑ ↓", "Navigate rows (separators auto-skipped)"},
			{"pgup / pgdn", "Page through fragments"},
			{"home / end", "Jump to first / last row"},
			{"enter", "Open file at the row's line"},
			{"esc", "Close multibuffer"},
		}},
		{Name: "Problems", Bindings: []Binding{
			{"alt+p", "Workspace-wide diagnostics panel"},
			{"↑ ↓", "Navigate rows"},
			{"pgup / pgdn", "Page through entries"},
			{"home / end", "Jump to first / last row"},
			{"enter", "Open file at the diagnostic's source"},
			{"esc", "Close panel"},
		}},
		{Name: "AI wedges", Bindings: []Binding{
			{"ctrl+k", "Inline edit on current line (Haiku 4.5)"},
			{"ctrl+l", "Multi-file composer (Sonnet 4.6)"},
			{"alt+h", "Clear composer history for the current file"},
			{"tab", "Accept ghost-text suggestion"},
			{"esc", "Dismiss ghost text"},
		}},
		{Name: "Snippets", Bindings: []Binding{
			{"alt+j", "Expand snippet at cursor"},
			{"tab", "Next tabstop (while in snippet mode)"},
			{"shift+tab", "Previous tabstop (while in snippet mode)"},
			{"esc", "Exit snippet mode"},
		}},
		{Name: "Tasks", Bindings: []Binding{
			{"alt+t", "Open task picker (.nook/tasks.toml or Go defaults)"},
			{"↑ ↓", "Navigate tasks"},
			{"enter", "Run the selected task"},
			{"ctrl+c", "Kill the running task"},
			{"esc", "Close (kills if still running)"},
		}},
		{Name: "Debug (Go via delve)", Bindings: []Binding{
			{"f9", "Toggle breakpoint at the cursor row"},
			{"f5", "Launch (or continue when paused)"},
			{"alt+f5", "Terminate the running session"},
			{"f6", "Pause a running session"},
			{"f10", "Step over"},
			{"f11", "Step in"},
			{"alt+f11", "Step out"},
		}},
		{Name: "Language server", Bindings: []Binding{
			{"alt+i", "Hover info for symbol under cursor"},
			{"ctrl+]", "Go to definition"},
			{"ctrl+space", "Completion popup (↑/↓ to navigate, enter to accept)"},
			{"alt+enter", "Code actions (quickfix / refactor) at the cursor"},
			{"f2", "Rename symbol under cursor (workspace-wide)"},
			{"alt+y", "Toggle gopls inlay hints (type annotations, parameter names)"},
			{"(settle)", "Document highlights light up every occurrence of the identifier under the cursor"},
			{"alt+u", "Find references to identifier under cursor"},
			{"alt+k", "Call hierarchy — incoming (who calls this)"},
			{"alt+K", "Call hierarchy — outgoing (what does this call)"},
			{"ctrl+t", "Workspace symbol search (functions, types, vars)"},
			{"ctrl+\\", "File outline (document symbols in current file)"},
			{"(", "Signature help (parameter hints auto-fire on '(', close on ')' or esc)"},
			{"alt+↓ / alt+↑", "Cycle overloads while signature help is open"},
		}},
		{Name: "Panes", Bindings: []Binding{
			{"ctrl+b", "Toggle file tree (left)"},
			{"a", "Create file or directory (file tree focused, suffix / for dir)"},
			{"ctrl+g", "Toggle git pane"},
			{"ctrl+`", "Toggle embedded terminal"},
			{"alt+v", "Toggle markdown preview (.md / .markdown only)"},
			{"esc", "Close overlay / blur pane"},
		}},
		{Name: "Git", Bindings: []Binding{
			{"alt+b", "Toggle inline blame on cursor row (GitLens-style)"},
		}},
		{Name: "Settings", Bindings: []Binding{
			{"alt+,", "Reload ~/.config/nook/config.toml"},
		}},
		{Name: "Global", Bindings: []Binding{
			{"?", "Toggle this help"},
			{"ctrl+q", "Quit nook"},
		}},
	}
}

// View renders the help card. Width is the available column count from the
// host (the card clamps to ~74 columns so the inner ladder lines up nicely
// regardless of terminal width).
func View(t theme.Theme, width int) string {
	inner := 74
	if width < inner+4 {
		inner = width - 4
		if inner < 30 {
			inner = 30
		}
	}

	title := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Render("nook keymap")
	subtitle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Italic(true).
		Render("press ? or esc to dismiss")

	sectionStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Bold(true)
	keyStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(t.Text)

	var body []string
	body = append(body, title)
	body = append(body, subtitle)
	body = append(body, "")

	for i, sec := range Default() {
		if i > 0 {
			body = append(body, "")
		}
		body = append(body, sectionStyle.Render(sec.Name))
		body = append(body, "")
		for _, b := range sec.Bindings {
			body = append(body, fmt.Sprintf("  %-18s  %s",
				keyStyle.Render(b.Key),
				descStyle.Render(b.Desc),
			))
		}
	}

	card := strings.Join(body, "\n")

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(1, 2).
		Width(inner).
		Background(t.Surface)
	return border.Render(card)
}
