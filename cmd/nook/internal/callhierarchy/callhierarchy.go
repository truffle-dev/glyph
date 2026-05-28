// Package callhierarchy maps LSP call-hierarchy results into multibuffer
// fragments for the overlay pane.
//
// LSP's call-hierarchy protocol is a two-step graph walk: first
// textDocument/prepareCallHierarchy at the cursor returns one or more
// CallHierarchyItem(s) (usually one symbol, but overloaded methods can
// return several); then callHierarchy/incomingCalls or callHierarchy/
// outgoingCalls each take an item and return a flat list of edges in the
// requested direction.
//
// This package wraps both halves and converts each returned call into a
// multibuffer.Fragment with a Suffix carrying the related symbol's name,
// so the user reads "function name" then a context window then Enter jumps.
//
// BuildFragments is pure (file reads through a Reader callback so tests can
// inject in-memory maps). CallHierarchyCmd wraps prepare + direction
// dispatch + fragment build into a single tea.Cmd factory.
package callhierarchy

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

// requestTimeout covers both halves of the prepare+calls round-trip. gopls
// resolves call hierarchy in ~100ms for a typical Go workspace; 5s leaves
// headroom for cold servers or multi-language workspaces.
const requestTimeout = 5 * time.Second

// DefaultContextLines mirrors findrefs and `git diff --unified=3` so the
// look-and-feel of the multibuffer pane stays consistent across loaders.
const DefaultContextLines = 3

// Direction selects between the two call-hierarchy queries.
type Direction int

const (
	// Incoming asks "who calls this?" — every site that points at the
	// prepared item.
	Incoming Direction = iota
	// Outgoing asks "what does this call?" — every call the prepared
	// item itself makes.
	Outgoing
)

// Label is the user-readable direction phrase used in the overlay title
// and status hints.
func (d Direction) Label() string {
	if d == Incoming {
		return "incoming calls"
	}
	return "outgoing calls"
}

// Source is the multibuffer.FragmentsMsg Source field — a stable identifier
// used by host-side log lines and tests.
func (d Direction) Source() string {
	if d == Incoming {
		return "callhierarchy.incoming"
	}
	return "callhierarchy.outgoing"
}

// ErrNoClient signals the LSP client was nil or not initialized. Wrapped
// in a FragmentsMsg so the host can wire alt+k unconditionally and surface
// a hint when the language server is still attaching.
var ErrNoClient = errors.New("no language server attached")

// NoIdentifierError signals the cursor was not on an identifier. The host
// catches this before issuing the LSP call and shows it in the status bar.
var NoIdentifierError = errors.New("no identifier under cursor")

// ErrNoTarget signals prepareCallHierarchy returned zero items. The server
// answered but said "nothing callable here," translated to a status hint.
var ErrNoTarget = errors.New("no callable symbol at cursor")

// Reader reads a file from disk as one string. Default is OSReader; tests
// inject map-backed readers via BuildFragments and CallHierarchyCmd.
type Reader func(path string) (string, error)

// OSReader is the default Reader implementation. Errors propagate.
func OSReader(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Symbol returns the identifier at (row, col) in source. Identical contract
// to findrefs.Symbol so the host can reuse the same cursor-on-identifier
// gate before firing either lookup.
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

// BuildFragments converts call-hierarchy results into multibuffer fragments.
// One fragment per call: one overlay row carries one caller (Incoming) or
// one callee (Outgoing). When a single call has multiple FromRanges in the
// same file (e.g., Foo calls Bar twice), the windows are merged into one
// span with both hit rows marked Added.
//
// sourcePath is the file containing the cursor when the call-hierarchy was
// prepared. Outgoing FromRanges are relative to that source file; Incoming
// FromRanges are relative to each call.Item.Path.
func BuildFragments(calls []nooklsp.CallHierarchyCall, direction Direction, sourcePath string, contextLines int, reader Reader) []multibuffer.Fragment {
	if len(calls) == 0 {
		return nil
	}
	if contextLines < 0 {
		contextLines = 0
	}
	if reader == nil {
		reader = OSReader
	}

	sorted := make([]nooklsp.CallHierarchyCall, len(calls))
	copy(sorted, calls)
	sort.SliceStable(sorted, func(i, j int) bool {
		pi, pj := pathFor(sorted[i], direction, sourcePath), pathFor(sorted[j], direction, sourcePath)
		if pi != pj {
			return pi < pj
		}
		return firstLine(sorted[i]) < firstLine(sorted[j])
	})

	var out []multibuffer.Fragment
	for _, call := range sorted {
		path := pathFor(call, direction, sourcePath)
		suffix := callSuffix(call)
		if frag := buildOneFragment(call, path, suffix, contextLines, reader); frag != nil {
			out = append(out, *frag)
		}
	}
	return out
}

func pathFor(call nooklsp.CallHierarchyCall, direction Direction, sourcePath string) string {
	if direction == Outgoing {
		return sourcePath
	}
	return call.Item.Path
}

func firstLine(call nooklsp.CallHierarchyCall) int {
	if len(call.FromRanges) == 0 {
		return call.Item.SelStartLine
	}
	return call.FromRanges[0].StartLine
}

func callSuffix(call nooklsp.CallHierarchyCall) string {
	name := call.Item.Name
	if call.Item.Detail != "" {
		return name + " — " + call.Item.Detail
	}
	return name
}

func buildOneFragment(call nooklsp.CallHierarchyCall, path, suffix string, contextLines int, reader Reader) *multibuffer.Fragment {
	if len(call.FromRanges) == 0 {
		return placeholderFragment(call, path, suffix)
	}
	text, err := reader(path)
	if err != nil {
		return &multibuffer.Fragment{
			Path:      path,
			StartLine: call.FromRanges[0].StartLine + 1,
			EndLine:   call.FromRanges[0].StartLine + 1,
			Lines: []multibuffer.Line{{
				Marker:   multibuffer.Context,
				FileLine: call.FromRanges[0].StartLine + 1,
				Text:     fmt.Sprintf("<unreadable: %v>", err),
			}},
			Suffix: suffix,
		}
	}
	lines := strings.Split(text, "\n")
	type window struct {
		start, end int
		hits       map[int]struct{}
	}
	ranges := make([]nooklsp.Range, len(call.FromRanges))
	copy(ranges, call.FromRanges)
	sort.SliceStable(ranges, func(i, j int) bool {
		if ranges[i].StartLine != ranges[j].StartLine {
			return ranges[i].StartLine < ranges[j].StartLine
		}
		return ranges[i].StartCol < ranges[j].StartCol
	})
	var windows []window
	for _, r := range ranges {
		s := r.StartLine - contextLines
		if s < 0 {
			s = 0
		}
		e := r.EndLine + contextLines
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
			for h := r.StartLine; h <= r.EndLine; h++ {
				w.hits[h] = struct{}{}
			}
			windows[len(windows)-1] = w
			continue
		}
		hits := map[int]struct{}{}
		for h := r.StartLine; h <= r.EndLine; h++ {
			hits[h] = struct{}{}
		}
		windows = append(windows, window{start: s, end: e, hits: hits})
	}

	// multibuffer.Fragment carries one [StartLine, EndLine] pair, so when
	// the call's FromRanges spread far enough that we built multiple
	// disjoint windows, collapse them into one span and let the Context
	// gap between hits read as "skipped lines." This matches how diff
	// fragments render multi-hunk regions in the same file.
	span := windows[0]
	if len(windows) > 1 {
		last := windows[len(windows)-1]
		span = window{start: windows[0].start, end: last.end, hits: map[int]struct{}{}}
		for _, w := range windows {
			for h := range w.hits {
				span.hits[h] = struct{}{}
			}
		}
	}
	fragLines := make([]multibuffer.Line, 0, span.end-span.start+1)
	for i := span.start; i <= span.end; i++ {
		marker := multibuffer.Context
		if _, ok := span.hits[i]; ok {
			marker = multibuffer.Added
		}
		var line string
		if i < len(lines) {
			line = lines[i]
		}
		fragLines = append(fragLines, multibuffer.Line{
			Marker:   marker,
			FileLine: i + 1,
			Text:     line,
		})
	}
	return &multibuffer.Fragment{
		Path:      path,
		StartLine: span.start + 1,
		EndLine:   span.end + 1,
		Lines:     fragLines,
		Suffix:    suffix,
	}
}

func placeholderFragment(call nooklsp.CallHierarchyCall, path, suffix string) *multibuffer.Fragment {
	line := call.Item.SelStartLine
	return &multibuffer.Fragment{
		Path:      path,
		StartLine: line + 1,
		EndLine:   line + 1,
		Lines: []multibuffer.Line{{
			Marker:   multibuffer.Added,
			FileLine: line + 1,
			Text:     "(no ranges reported)",
		}},
		Suffix: suffix,
	}
}

// CallHierarchyCmd is the tea.Cmd that runs prepare + direction-specific
// calls and returns a multibuffer.FragmentsMsg. The host pane's title is
// set separately via multibuffer.Pane.Reset(...) before the command fires,
// so we don't carry the symbol name back through the message.
//
// For overloaded methods (Java, C++) prepareCallHierarchy returns multiple
// items; we union the per-item call results so every overload's neighbors
// land in the overlay together.
func CallHierarchyCmd(client *nooklsp.Client, path string, row, col int, direction Direction, contextLines int, reader Reader) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return multibuffer.FragmentsMsg{Source: direction.Source(), Err: ErrNoClient}
		}
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		items, err := client.PrepareCallHierarchy(ctx, path, row, col)
		if err != nil {
			return multibuffer.FragmentsMsg{Source: direction.Source(), Err: err}
		}
		if len(items) == 0 {
			return multibuffer.FragmentsMsg{Source: direction.Source(), Err: ErrNoTarget}
		}
		var all []nooklsp.CallHierarchyCall
		for _, item := range items {
			var calls []nooklsp.CallHierarchyCall
			var callErr error
			if direction == Incoming {
				calls, callErr = client.IncomingCalls(ctx, item)
			} else {
				calls, callErr = client.OutgoingCalls(ctx, item)
			}
			if callErr != nil {
				return multibuffer.FragmentsMsg{Source: direction.Source(), Err: callErr}
			}
			all = append(all, calls...)
		}
		frags := BuildFragments(all, direction, path, contextLines, reader)
		return multibuffer.FragmentsMsg{Source: direction.Source(), Fragments: frags}
	}
}
