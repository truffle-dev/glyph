// Package finder is nook's in-file find/replace overlay.
//
// Two modes share state: ModeFind (Ctrl+F) and ModeReplace (Ctrl+H). The
// overlay renders as a one- or two-line bar at the bottom of the workspace.
// As the user types, the host re-runs Search and feeds the matches back via
// WithMatches. Navigation keys jump the cursor between matches; replace
// edits the buffer one match at a time, replace-all rewrites every line that
// contains at least one match.
//
// The finder does not own the buffer. The host calls Search to compute
// matches and ApplyReplacement to produce the rewritten line text; the host
// applies the edit through editor.Pane.SetLine so the editor's dirty flag,
// LSP didChange, and version counters stay coherent.
package finder

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/truffle-dev/glyph/components/theme"
)

// Mode is the finder's editing mode.
type Mode int

const (
	ModeFind Mode = iota
	ModeReplace
)

// Focus is which input the keystroke routes to (find vs replace box).
type Focus int

const (
	FocusFind Focus = iota
	FocusReplace
)

// Match is one hit inside the active buffer.
type Match struct {
	Row      int // 0-based row
	StartCol int // 0-based byte column, inclusive
	EndCol   int // 0-based byte column, exclusive
}

// Event is what the host needs to act on after Update. Most updates produce
// EventNone; specific keys produce typed events.
type Event int

const (
	EventNone           Event = iota
	EventClose                // Esc
	EventPatternChanged       // re-run Search
	EventJumpNext             // jump editor cursor to next match
	EventJumpPrev             // jump editor cursor to prev match
	EventReplaceCurrent       // replace match at current
	EventReplaceAll           // replace every match
)

// Finder owns the input fields, toggles, match list, and cursor.
type Finder struct {
	theme         theme.Theme
	mode          Mode
	focus         Focus
	pattern       string
	replacement   string
	useRegex      bool
	caseSensitive bool
	matches       []Match
	current       int // index into matches; -1 when no match active
	patternErr    error
	width         int
	open          bool
}

// New constructs a closed finder.
func New(t theme.Theme) Finder {
	return Finder{theme: t, current: -1, width: 80}
}

// WithSize sets the render width. Height is fixed (1 or 2 rows depending on
// mode).
func (f Finder) WithSize(w int) Finder { f.width = w; return f }

// SetTheme swaps the palette used for the input box, mode chip, and the
// `n of N` counter. Next View() picks up the new colors.
func (f Finder) SetTheme(t theme.Theme) Finder { f.theme = t; return f }

// Open shows the finder in the requested mode. If the mode is changed from
// ModeFind to ModeReplace (or vice versa) the existing pattern is preserved.
func (f Finder) Open(m Mode) Finder {
	f.open = true
	f.mode = m
	f.focus = FocusFind
	return f
}

// Close hides the finder. Pattern and replacement are preserved so reopening
// recalls the last query.
func (f Finder) Close() Finder {
	f.open = false
	return f
}

// IsOpen reports whether the bar should render.
func (f Finder) IsOpen() bool { return f.open }

// Mode returns the current mode.
func (f Finder) Mode() Mode { return f.mode }

// Focus returns which input is focused.
func (f Finder) Focus() Focus { return f.focus }

// Pattern returns the find pattern.
func (f Finder) Pattern() string { return f.pattern }

// Replacement returns the replacement text.
func (f Finder) Replacement() string { return f.replacement }

// UseRegex reports whether regex mode is on.
func (f Finder) UseRegex() bool { return f.useRegex }

// CaseSensitive reports whether case-sensitive mode is on.
func (f Finder) CaseSensitive() bool { return f.caseSensitive }

// Matches returns the current match list.
func (f Finder) Matches() []Match { return f.matches }

// PatternErr returns the most recent pattern-compile error (regex mode).
func (f Finder) PatternErr() error { return f.patternErr }

// CurrentIndex returns the 0-based index of the active match, or -1.
func (f Finder) CurrentIndex() int { return f.current }

// CurrentMatch returns the active match, or false if there is none.
func (f Finder) CurrentMatch() (Match, bool) {
	if f.current < 0 || f.current >= len(f.matches) {
		return Match{}, false
	}
	return f.matches[f.current], true
}

// WithMatches replaces the match list, computed by the host via Search. If
// keepCurrent is true the cursor stays on the same row/column as before; if
// false it resets to the first match. Always clamps to len(matches)-1.
func (f Finder) WithMatches(ms []Match, keepCurrent bool) Finder {
	f.matches = ms
	if len(ms) == 0 {
		f.current = -1
		return f
	}
	if !keepCurrent || f.current < 0 || f.current >= len(ms) {
		f.current = 0
	}
	return f
}

// SelectMatchAt picks the first match at-or-after (row, col). Called by the
// host after a JumpTo so the counter "n/N" stays aligned with the editor's
// cursor position.
func (f Finder) SelectMatchAt(row, col int) Finder {
	for i, m := range f.matches {
		if m.Row > row || (m.Row == row && m.StartCol >= col) {
			f.current = i
			return f
		}
	}
	if len(f.matches) > 0 {
		f.current = 0
	}
	return f
}

// Next advances the cursor to the next match (wrapping). No-op when empty.
func (f Finder) Next() Finder {
	if len(f.matches) == 0 {
		f.current = -1
		return f
	}
	f.current = (f.current + 1) % len(f.matches)
	if f.current < 0 {
		f.current = 0
	}
	return f
}

// Prev advances the cursor to the previous match (wrapping). No-op when empty.
func (f Finder) Prev() Finder {
	if len(f.matches) == 0 {
		f.current = -1
		return f
	}
	if f.current <= 0 {
		f.current = len(f.matches) - 1
	} else {
		f.current--
	}
	return f
}

// ToggleRegex flips regex mode and clears the cached pattern-compile error.
func (f Finder) ToggleRegex() Finder {
	f.useRegex = !f.useRegex
	f.patternErr = nil
	return f
}

// ToggleCase flips case-sensitivity.
func (f Finder) ToggleCase() Finder {
	f.caseSensitive = !f.caseSensitive
	return f
}

// SetPatternErr stashes a compile/search error so View can surface it.
func (f Finder) SetPatternErr(err error) Finder { f.patternErr = err; return f }

// Update routes a key event. Returns the new finder and an Event the host
// should react to (e.g. EventPatternChanged → re-run Search, EventJumpNext →
// move editor cursor).
func (f Finder) Update(km tea.KeyMsg) (Finder, Event) {
	if !f.open {
		return f, EventNone
	}

	// Global keys that work regardless of focus.
	switch km.Type {
	case tea.KeyEsc:
		f.open = false
		return f, EventClose
	case tea.KeyEnter:
		return f, EventJumpNext
	}

	// Alt-modified keys: toggles and replace-all.
	if km.Alt && km.Type == tea.KeyRunes && len(km.Runes) == 1 {
		switch km.Runes[0] {
		case 'r':
			if f.mode == ModeReplace {
				return f, EventReplaceAll
			}
			return f, EventNone
		case 'x':
			f.useRegex = !f.useRegex
			f.patternErr = nil
			return f, EventPatternChanged
		case 'c':
			f.caseSensitive = !f.caseSensitive
			return f, EventPatternChanged
		}
		return f, EventNone
	}

	switch km.Type {
	case tea.KeyTab:
		if f.mode == ModeReplace {
			if f.focus == FocusFind {
				f.focus = FocusReplace
			} else {
				f.focus = FocusFind
			}
		}
		return f, EventNone
	case tea.KeyShiftTab:
		if f.mode == ModeReplace {
			if f.focus == FocusFind {
				f.focus = FocusReplace
			} else {
				f.focus = FocusFind
			}
		}
		return f, EventNone
	case tea.KeyDown:
		return f, EventJumpNext
	case tea.KeyUp:
		return f, EventJumpPrev
	case tea.KeyCtrlN:
		return f, EventJumpNext
	case tea.KeyCtrlP:
		return f, EventJumpPrev
	case tea.KeyCtrlR:
		if f.mode == ModeReplace {
			return f, EventReplaceCurrent
		}
		return f, EventNone
	case tea.KeyBackspace:
		if f.focus == FocusFind {
			if f.pattern != "" {
				f.pattern = f.pattern[:len(f.pattern)-1]
				return f, EventPatternChanged
			}
		} else {
			if f.replacement != "" {
				f.replacement = f.replacement[:len(f.replacement)-1]
			}
		}
		return f, EventNone
	case tea.KeyRunes:
		if f.focus == FocusFind {
			f.pattern += string(km.Runes)
			return f, EventPatternChanged
		}
		f.replacement += string(km.Runes)
		return f, EventNone
	case tea.KeySpace:
		if f.focus == FocusFind {
			f.pattern += " "
			return f, EventPatternChanged
		}
		f.replacement += " "
		return f, EventNone
	}
	return f, EventNone
}

// Height reports how many rows the finder occupies. 1 for find, 2 for
// replace mode.
func (f Finder) Height() int {
	if !f.open {
		return 0
	}
	if f.mode == ModeReplace {
		return 2
	}
	return 1
}

// View renders the bar.
func (f Finder) View() string {
	if !f.open {
		return ""
	}
	t := f.theme
	label := lipgloss.NewStyle().Foreground(t.TextMuted).Bold(true)
	field := lipgloss.NewStyle().Foreground(t.Text)
	focusField := lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface)
	cursor := lipgloss.NewStyle().Foreground(t.TextInverse).Background(t.Primary)
	muted := lipgloss.NewStyle().Foreground(t.TextMuted)
	errStyle := lipgloss.NewStyle().Foreground(t.Error)
	chip := lipgloss.NewStyle().Foreground(t.Bg).Background(t.Primary).Bold(true).Padding(0, 1)
	chipOff := lipgloss.NewStyle().Foreground(t.TextMuted).Padding(0, 1)

	chipCase := chipOff.Render("Aa")
	if f.caseSensitive {
		chipCase = chip.Render("Aa")
	}
	chipRe := chipOff.Render(".*")
	if f.useRegex {
		chipRe = chip.Render(".*")
	}

	counter := ""
	switch {
	case f.patternErr != nil:
		counter = errStyle.Render("bad pattern")
	case f.pattern == "":
		counter = muted.Render("no results")
	case len(f.matches) == 0:
		counter = muted.Render("0/0")
	default:
		counter = field.Render(fmt.Sprintf("%d/%d", f.current+1, len(f.matches)))
	}

	hintFind := muted.Render("↑/↓ next/prev  alt+x regex  alt+c case  esc close")
	hintReplace := muted.Render("tab switch  ctrl+r replace  alt+r replace all  esc close")

	findRow := f.renderInput(label.Render("find    "), f.pattern, f.focus == FocusFind, field, focusField, cursor)
	findRow += "  " + chipCase + " " + chipRe + "  " + counter
	right := f.width - lipgloss.Width(findRow)
	if right > 4 {
		findRow += "  " + clip(hintFind, right-2)
	}
	if f.mode == ModeFind {
		return findRow
	}

	replaceRow := f.renderInput(label.Render("replace "), f.replacement, f.focus == FocusReplace, field, focusField, cursor)
	right = f.width - lipgloss.Width(replaceRow)
	if right > 4 {
		replaceRow += "  " + clip(hintReplace, right-2)
	}
	return findRow + "\n" + replaceRow
}

func (f Finder) renderInput(label, value string, focused bool, field, focusField, cursor lipgloss.Style) string {
	const inputWidth = 28
	body := value
	style := field
	if focused {
		style = focusField
	}
	body = clip(body, inputWidth-1)
	rendered := style.Render(body)
	pad := inputWidth - lipgloss.Width(body) - 1
	if pad < 0 {
		pad = 0
	}
	cur := " "
	if focused {
		cur = cursor.Render(" ")
	}
	return label + rendered + cur + strings.Repeat(" ", pad)
}

// clip clips s to w display columns with a trailing "…" when content was
// dropped. It counts display cells, not runes, so wide characters (CJK,
// emoji) that occupy two columns don't overshoot the budget; ansi.Truncate
// is grapheme- and wide-character-aware.
func clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if w == 1 {
		return ansi.Truncate(s, w, "")
	}
	return ansi.Truncate(s, w, "…")
}

// Search scans buf and returns every match of pattern. When useRegex is
// false the pattern is treated as a literal substring (case-folded when
// caseSensitive is false). When useRegex is true the pattern compiles as
// Go's RE2; on compile error the returned matches are nil and the error
// is returned so the caller can stash it via SetPatternErr.
//
// Each line is searched independently; multi-line regex patterns can use
// `.` to match within a line but cannot cross a newline.
func Search(buf []string, pattern string, useRegex, caseSensitive bool) ([]Match, error) {
	if pattern == "" {
		return nil, nil
	}
	if useRegex {
		expr := pattern
		if !caseSensitive {
			expr = "(?i)" + expr
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, err
		}
		var out []Match
		for row, line := range buf {
			for _, loc := range re.FindAllStringIndex(line, -1) {
				start, end := loc[0], loc[1]
				if start == end {
					// Skip empty matches to avoid infinite navigation. RE2 returns
					// them for anchors like ^$ — we keep them out of the cycle.
					continue
				}
				out = append(out, Match{Row: row, StartCol: start, EndCol: end})
			}
		}
		return out, nil
	}

	// Literal substring search.
	needle := pattern
	var lineFold func(string) string
	if !caseSensitive {
		needle = strings.ToLower(pattern)
		lineFold = strings.ToLower
	}
	var out []Match
	for row, line := range buf {
		hay := line
		if lineFold != nil {
			hay = lineFold(line)
		}
		start := 0
		for {
			idx := strings.Index(hay[start:], needle)
			if idx < 0 {
				break
			}
			abs := start + idx
			out = append(out, Match{Row: row, StartCol: abs, EndCol: abs + len(needle)})
			start = abs + len(needle)
			if len(needle) == 0 {
				break
			}
		}
	}
	return out, nil
}

// ApplyReplacement returns the new text for the row after substituting the
// match's range with replacement. The host calls this then writes the result
// back via editor.Pane.SetLine. If useRegex is true and the pattern compiles,
// regex expansion (`$1`, `${name}`) is applied to replacement; the caller
// passes the compiled regex via re so we don't recompile per call.
//
// When re is nil the substitution is a plain byte-range splice.
func ApplyReplacement(line string, m Match, replacement string, re *regexp.Regexp) string {
	if m.Row < 0 || m.StartCol < 0 || m.EndCol > len(line) || m.StartCol > m.EndCol {
		return line
	}
	if re != nil {
		// Use regex expansion on the matched substring so capture groups work.
		matched := line[m.StartCol:m.EndCol]
		idx := re.FindStringSubmatchIndex(matched)
		if idx == nil {
			return line[:m.StartCol] + replacement + line[m.EndCol:]
		}
		expanded := re.ExpandString(nil, replacement, matched, idx)
		return line[:m.StartCol] + string(expanded) + line[m.EndCol:]
	}
	return line[:m.StartCol] + replacement + line[m.EndCol:]
}

// CompileRegex compiles the pattern under the same flags Search applies.
// The host calls this once to obtain a *Regexp it can pass into
// ApplyReplacement for capture-group expansion.
func CompileRegex(pattern string, caseSensitive bool) (*regexp.Regexp, error) {
	expr := pattern
	if !caseSensitive {
		expr = "(?i)" + expr
	}
	return regexp.Compile(expr)
}
