package search

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Result is the outcome of a project-wide ApplyAll call.
type Result struct {
	// FilesChanged counts paths whose contents were actually rewritten.
	// A path appears here at most once even if it carried many matches.
	FilesChanged int
	// ReplacementsApplied counts individual hits successfully replaced
	// across all files. May be less than len(matches) when a recorded
	// hit no longer aligns with the on-disk byte at apply time (file
	// edited between search and apply); those hits are skipped silently.
	ReplacementsApplied int
	// PathsTouched lists every path that received at least one
	// replacement. The host iterates this list to call RefreshIfOpen
	// so open buffers reload after the disk write.
	PathsTouched []string
}

// ApplyAll rewrites every Match on disk by replacing the recorded byte
// span [Col-1, Col-1+Len) on Match.Line with replacement. Files are read
// once, all hits applied right-to-left within each line for byte-index
// stability, then written once. Matches are grouped by Path; a Match
// with Len <= 0 is treated as a corrupt entry and skipped silently
// (ripgrep always supplies a non-zero submatch length, but the
// defensive guard keeps the helper safe against future callers that
// might build Match values without going through Run).
//
// Returned error wraps the OS-level read or write that broke; the
// caller can surface it through a status hint. Partial application
// across files is possible: an error mid-iteration leaves earlier
// files rewritten and later files untouched. The Result fields
// describe the state of the disk regardless of error.
func ApplyAll(matches []Match, replacement string) (Result, error) {
	if len(matches) == 0 {
		return Result{}, nil
	}
	byPath := make(map[string][]Match, 8)
	pathOrder := make([]string, 0, 8)
	for _, m := range matches {
		if _, seen := byPath[m.Path]; !seen {
			pathOrder = append(pathOrder, m.Path)
		}
		byPath[m.Path] = append(byPath[m.Path], m)
	}

	var res Result
	for _, path := range pathOrder {
		hits := byPath[path]
		applied, err := applyOneFile(path, hits, replacement)
		if err != nil {
			return res, err
		}
		if applied > 0 {
			res.FilesChanged++
			res.ReplacementsApplied += applied
			res.PathsTouched = append(res.PathsTouched, path)
		}
	}
	return res, nil
}

// applyOneFile reads path, rewrites every hit, writes back. Returns the
// number of replacements actually performed (skipping any whose Len <= 0
// or whose recorded line is now out of range).
func applyOneFile(path string, hits []Match, replacement string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	// Preserve final-newline state so a file with no trailing newline
	// stays that way and a file with one keeps it.
	original := string(raw)
	trailingNewline := strings.HasSuffix(original, "\n")
	body := original
	if trailingNewline {
		body = body[:len(body)-1]
	}
	lines := strings.Split(body, "\n")

	// Group hits by line so we can sort+apply right-to-left per line.
	byLine := make(map[int][]Match, len(hits))
	for _, h := range hits {
		if h.Len <= 0 {
			continue
		}
		if h.Line < 1 || h.Line > len(lines) {
			continue
		}
		byLine[h.Line] = append(byLine[h.Line], h)
	}
	if len(byLine) == 0 {
		return 0, nil
	}

	applied := 0
	for lineNum, lineHits := range byLine {
		// Right-to-left: largest Col first so earlier Col indices
		// into the same line stay valid after the in-place rewrite.
		sort.Slice(lineHits, func(i, j int) bool {
			return lineHits[i].Col > lineHits[j].Col
		})
		row := lines[lineNum-1]
		for _, h := range lineHits {
			start := h.Col - 1
			end := start + h.Len
			if start < 0 || end > len(row) || start > end {
				continue
			}
			row = row[:start] + replacement + row[end:]
			applied++
		}
		lines[lineNum-1] = row
	}

	if applied == 0 {
		return 0, nil
	}

	out := strings.Join(lines, "\n")
	if trailingNewline {
		out += "\n"
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return applied, fmt.Errorf("write %s: %w", path, err)
	}
	return applied, nil
}
