// Package findrefs maps LSP textDocument/references results into a
// multibuffer fragment list.
//
// The server returns a flat slice of (path, line, col) sites with no
// surrounding context. The multibuffer pane wants Fragments — each one a
// contiguous slice of one file with per-line annotations. findrefs reads
// each referenced file, slices a window of context around every hit,
// merges overlapping windows on the same file, sorts by (path,
// startLine), and returns the resulting Fragment slice.
//
// Compute is pure (file reads go through a Reader callback so tests can
// inject an in-memory map). FindReferencesCmd wraps Compute with the
// actual LSP call.
package findrefs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/cmd/nook/internal/multibuffer"
)

// requestTimeout matches the existing lookup package window. gopls
// answers references in well under a second for a typical Go workspace,
// but multi-root or multi-language projects benefit from headroom.
const requestTimeout = 3 * time.Second

// DefaultContextLines is the half-window of source lines emitted around
// each hit. Three above + three below mirrors `git diff --unified=3` —
// the same window the user already sees when the multibuffer pane is
// loaded with diff fragments.
const DefaultContextLines = 3

// ErrNoClient signals the LSP client was nil or not initialized. Wrapped
// in a FragmentsMsg so the host can bind alt+u unconditionally and
// surface a hint when the language server is still attaching.
var ErrNoClient = errors.New("no language server attached")

// NoIdentifierError signals the cursor was not on an identifier. The
// host catches this before issuing the LSP call and shows it in the
// status bar.
var NoIdentifierError = errors.New("no identifier under cursor")

// Reader reads a file from disk as one string. Default is OSReader;
// tests inject map-backed readers via BuildFragments and Compute.
type Reader func(path string) (string, error)

// OSReader is the default Reader implementation. Errors propagate.
func OSReader(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Symbol returns the identifier at (row, col) in source. Identifiers are
// `[A-Za-z_][A-Za-z0-9_]*`; numeric-only spans return "". row and col
// are 0-indexed. The half-open contract is "cursor is on the identifier
// if col is anywhere within or at the right edge of the identifier" —
// i.e. col may equal len(ident) and still resolve, matching how editors
// place the caret after the last keystroke.
func Symbol(source string, row, col int) string {
	lines := strings.Split(source, "\n")
	if row < 0 || row >= len(lines) {
		return ""
	}
	line := lines[row]
	if col < 0 || col > len(line) {
		return ""
	}
	start := col
	for start > 0 && isIdent(rune(line[start-1])) {
		start--
	}
	end := col
	for end < len(line) && isIdent(rune(line[end])) {
		end++
	}
	if start == end {
		return ""
	}
	candidate := line[start:end]
	if isAllDigits(candidate) {
		return ""
	}
	return candidate
}

func isIdent(r rune) bool {
	return r == '_' ||
		(r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9')
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// BuildFragments converts a flat location list into Fragments suitable
// for the multibuffer pane. Steps:
//
//  1. Stable-sort locations by (Path, Line, Col).
//  2. Group adjacent same-path runs.
//  3. For each file, read it via reader, then build windows of
//     [hit-contextLines, hit+contextLines] clamped to file bounds.
//     Windows that overlap or touch (gap of 0) are merged so a method
//     called twice on lines 7 and 9 produces one fragment 4..12 rather
//     than two overlapping ones.
//  4. The hit row itself is marked multibuffer.Added so the pane paints
//     it with the highlight color the diff loader uses for "+" lines.
//     Context rows are marked multibuffer.Context.
//
// A file that fails to read produces a single-line placeholder fragment
// carrying the error text, so the user still sees the hit and Enter
// still works to open the file (which will surface the real error in the
// editor pane).
func BuildFragments(locs []nooklsp.Location, contextLines int, reader Reader) []multibuffer.Fragment {
	if len(locs) == 0 {
		return nil
	}
	if contextLines < 0 {
		contextLines = 0
	}
	if reader == nil {
		reader = OSReader
	}

	sorted := make([]nooklsp.Location, len(locs))
	copy(sorted, locs)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		if sorted[i].Line != sorted[j].Line {
			return sorted[i].Line < sorted[j].Line
		}
		return sorted[i].Col < sorted[j].Col
	})

	type group struct {
		path string
		hits []int
	}
	var groups []group
	for _, l := range sorted {
		if len(groups) > 0 && groups[len(groups)-1].path == l.Path {
			groups[len(groups)-1].hits = append(groups[len(groups)-1].hits, l.Line)
			continue
		}
		groups = append(groups, group{path: l.Path, hits: []int{l.Line}})
	}

	var out []multibuffer.Fragment
	for _, g := range groups {
		text, err := reader(g.path)
		if err != nil {
			out = append(out, multibuffer.Fragment{
				Path:      g.path,
				StartLine: g.hits[0] + 1,
				EndLine:   g.hits[0] + 1,
				Lines: []multibuffer.Line{{
					Marker:   multibuffer.Context,
					FileLine: g.hits[0] + 1,
					Text:     fmt.Sprintf("<unreadable: %v>", err),
				}},
			})
			continue
		}
		lines := strings.Split(text, "\n")
		type window struct {
			start, end int
			hits       map[int]struct{}
		}
		var windows []window
		for _, h := range g.hits {
			s := h - contextLines
			if s < 0 {
				s = 0
			}
			e := h + contextLines
			if e >= len(lines) {
				e = len(lines) - 1
			}
			if e < s {
				e = s
			}
			if len(windows) > 0 && s <= windows[len(windows)-1].end+1 {
				w := windows[len(windows)-1]
				if e > w.end {
					w.end = e
				}
				w.hits[h] = struct{}{}
				windows[len(windows)-1] = w
				continue
			}
			windows = append(windows, window{
				start: s,
				end:   e,
				hits:  map[int]struct{}{h: {}},
			})
		}
		for _, w := range windows {
			fragLines := make([]multibuffer.Line, 0, w.end-w.start+1)
			for i := w.start; i <= w.end; i++ {
				marker := multibuffer.Context
				if _, ok := w.hits[i]; ok {
					marker = multibuffer.Added
				}
				var text string
				if i < len(lines) {
					text = lines[i]
				}
				fragLines = append(fragLines, multibuffer.Line{
					Marker:   marker,
					FileLine: i + 1,
					Text:     text,
				})
			}
			out = append(out, multibuffer.Fragment{
				Path:      g.path,
				StartLine: w.start + 1,
				EndLine:   w.end + 1,
				Lines:     fragLines,
			})
		}
	}
	return out
}

// FindReferencesCmd is the tea.Cmd that calls client.References at
// (path, row, col) and returns a multibuffer.FragmentsMsg with the
// resulting Fragments. reader is exposed for tests; production callers
// pass nil for the OS-backed default.
//
// The host pane title is set separately via Reset(...) before the
// command fires, so we don't carry symbol back through the message.
func FindReferencesCmd(client *nooklsp.Client, path string, row, col int, contextLines int, reader Reader) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return multibuffer.FragmentsMsg{Source: "references", Err: ErrNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		locs, err := client.References(ctx, path, row, col)
		if err != nil {
			return multibuffer.FragmentsMsg{Source: "references", Err: err}
		}
		frags := BuildFragments(locs, contextLines, reader)
		return multibuffer.FragmentsMsg{Source: "references", Fragments: frags}
	}
}
