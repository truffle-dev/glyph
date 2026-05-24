// Package symbolsearch maps LSP workspace/symbol results into a
// multibuffer fragment list.
//
// The server takes a query string and returns a flat list of
// (name, kind, container, path, line, col) tuples. The multibuffer pane
// wants fragments — each one a contiguous slice of one file with
// per-line annotations. symbolsearch reads each referenced file, slices
// a window of context around every hit, merges overlapping windows on
// the same file, sorts by (path, startLine), and returns the resulting
// Fragment slice.
//
// Compute is pure (file reads go through a Reader callback so tests can
// inject an in-memory map). FindSymbolsCmd wraps Compute with the
// actual LSP call. Prompt is the small input modal the host opens on
// Ctrl+T to collect the query.
package symbolsearch

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

// requestTimeout matches the findrefs window. gopls answers workspace
// symbol queries quickly on a warm cache, but multi-root workspaces
// benefit from headroom.
const requestTimeout = 4 * time.Second

// DefaultContextLines is the half-window of source lines emitted around
// each hit. Three above + three below mirrors the find-references and
// diff-loader windows so the multibuffer overlay reads identically
// across all three loaders.
const DefaultContextLines = 3

// ErrNoClient signals the LSP client was nil or not initialized. Wrapped
// in a FragmentsMsg so the host can bind Ctrl+T unconditionally and
// surface a hint when the language server is still attaching.
var ErrNoClient = errors.New("no language server attached")

// ErrEmptyQuery signals the user submitted an empty / whitespace-only
// query. The host catches this before opening the modal in error mode.
var ErrEmptyQuery = errors.New("empty query")

// Reader reads a file from disk as one string. OSReader is the default;
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

// BuildFragments converts a flat symbol list into multibuffer Fragments.
// Steps:
//
//  1. Stable-sort symbols by (Path, Line, Col).
//  2. Group adjacent same-path runs.
//  3. For each file, read it via reader, then build windows of
//     [hit-contextLines, hit+contextLines] clamped to file bounds.
//     Windows that overlap or touch (gap of 0) are merged so two
//     symbols on lines 7 and 9 produce one fragment 4..12.
//  4. The symbol's declaration line is marked multibuffer.Added so the
//     pane paints it with the same highlight color used by the diff
//     and references loaders. Context rows are marked
//     multibuffer.Context.
//  5. The fragment Suffix is set to "kind container.Name" (or just
//     "kind Name" when Container is empty) so the multibuffer header
//     reads like "main.go:42 — func handleSearch".
//
// A file that fails to read produces a single-line placeholder fragment
// carrying the error text, so the user still sees the hit and Enter
// still works to open the file (which will surface the real error in
// the editor pane).
func BuildFragments(syms []nooklsp.WorkspaceSymbol, contextLines int, reader Reader) []multibuffer.Fragment {
	if len(syms) == 0 {
		return nil
	}
	if contextLines < 0 {
		contextLines = 0
	}
	if reader == nil {
		reader = OSReader
	}

	sorted := make([]nooklsp.WorkspaceSymbol, len(syms))
	copy(sorted, syms)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		if sorted[i].Line != sorted[j].Line {
			return sorted[i].Line < sorted[j].Line
		}
		return sorted[i].Col < sorted[j].Col
	})

	type hit struct {
		line   int
		suffix string
	}
	type group struct {
		path string
		hits []hit
	}
	var groups []group
	for _, s := range sorted {
		h := hit{line: s.Line, suffix: suffixFor(s)}
		if len(groups) > 0 && groups[len(groups)-1].path == s.Path {
			groups[len(groups)-1].hits = append(groups[len(groups)-1].hits, h)
			continue
		}
		groups = append(groups, group{path: s.Path, hits: []hit{h}})
	}

	var out []multibuffer.Fragment
	for _, g := range groups {
		text, err := reader(g.path)
		if err != nil {
			out = append(out, multibuffer.Fragment{
				Path:      g.path,
				StartLine: g.hits[0].line + 1,
				EndLine:   g.hits[0].line + 1,
				Suffix:    g.hits[0].suffix,
				Lines: []multibuffer.Line{{
					Marker:   multibuffer.Context,
					FileLine: g.hits[0].line + 1,
					Text:     fmt.Sprintf("<unreadable: %v>", err),
				}},
			})
			continue
		}
		lines := strings.Split(text, "\n")
		type window struct {
			start, end int
			hits       map[int]string // line → suffix
		}
		var windows []window
		for _, h := range g.hits {
			s := h.line - contextLines
			if s < 0 {
				s = 0
			}
			e := h.line + contextLines
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
				if _, ok := w.hits[h.line]; !ok {
					w.hits[h.line] = h.suffix
				}
				windows[len(windows)-1] = w
				continue
			}
			windows = append(windows, window{
				start: s,
				end:   e,
				hits:  map[int]string{h.line: h.suffix},
			})
		}
		for _, w := range windows {
			fragLines := make([]multibuffer.Line, 0, w.end-w.start+1)
			for i := w.start; i <= w.end; i++ {
				marker := multibuffer.Context
				if _, ok := w.hits[i]; ok {
					marker = multibuffer.Added
				}
				var ltext string
				if i < len(lines) {
					ltext = lines[i]
				}
				fragLines = append(fragLines, multibuffer.Line{
					Marker:   marker,
					FileLine: i + 1,
					Text:     ltext,
				})
			}
			// The window suffix is the first hit's suffix in line order
			// — picks the symbol that anchored the fragment, even when
			// multiple symbols were merged into one window.
			var winSuffix string
			for i := w.start; i <= w.end; i++ {
				if sx, ok := w.hits[i]; ok {
					winSuffix = sx
					break
				}
			}
			out = append(out, multibuffer.Fragment{
				Path:      g.path,
				StartLine: w.start + 1,
				EndLine:   w.end + 1,
				Suffix:    winSuffix,
				Lines:     fragLines,
			})
		}
	}
	return out
}

// suffixFor renders the "kind container.Name" tag carried on each
// fragment. Empty kind / container collapse so the result never has
// leading separators or stray dots. Used both for the Fragment.Suffix
// and any future status-bar render.
func suffixFor(s nooklsp.WorkspaceSymbol) string {
	var name string
	if s.Container != "" {
		name = s.Container + "." + s.Name
	} else {
		name = s.Name
	}
	kind := s.Kind.Short()
	if kind == "" {
		return name
	}
	if name == "" {
		return kind
	}
	return kind + " " + name
}

// FindSymbolsCmd is the tea.Cmd that calls client.WorkspaceSymbol with
// query and returns a multibuffer.FragmentsMsg with the resulting
// Fragments. reader is exposed for tests; production callers pass nil
// for the OS-backed default. The host pane title is set separately via
// Reset(...) before the command fires, so we don't carry the query back
// through the message.
func FindSymbolsCmd(client *nooklsp.Client, query string, contextLines int, reader Reader) tea.Cmd {
	q := strings.TrimSpace(query)
	return func() tea.Msg {
		if client == nil {
			return multibuffer.FragmentsMsg{Source: "symbols", Err: ErrNoClient}
		}
		if q == "" {
			return multibuffer.FragmentsMsg{Source: "symbols", Err: ErrEmptyQuery}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		syms, err := client.WorkspaceSymbol(ctx, q)
		if err != nil {
			return multibuffer.FragmentsMsg{Source: "symbols", Err: err}
		}
		frags := BuildFragments(syms, contextLines, reader)
		return multibuffer.FragmentsMsg{Source: "symbols", Fragments: frags}
	}
}
