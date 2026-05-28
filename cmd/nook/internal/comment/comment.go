// Package comment turns a buffer path and a range of lines into a
// language-appropriate "comment / uncomment" transform. The package is
// pure: no editor state, no LSP, no I/O. It exists so the host
// (cmd/nook/main.go) and the editor pane can implement Ctrl+/ without
// having to relearn what each filetype's comment prefix is.
//
// The contract has two halves:
//
//   - Prefix(path) returns the line-comment marker for path's
//     extension. Markers end in a space — `// `, `# `, `-- ` — so that
//     pasted-on comments read like the surrounding code rather than a
//     greppable banner. Returns "" for filetypes nook doesn't yet
//     know how to comment (HTML, CSS, XML, anything block-only); the
//     caller is expected to no-op gracefully.
//
//   - ToggleLines(lines, prefix) takes a slice of strings (the buffer
//     rows the caller wants to toggle) and the prefix returned by
//     Prefix. It returns (newLines, op): if every non-blank line in
//     the input already starts with prefix (after its leading
//     whitespace), the op is Uncomment and the prefix is stripped from
//     each non-blank line. Otherwise the op is Comment and the prefix
//     is inserted at each non-blank line's indent column. Blank lines
//     pass through unchanged in both directions, matching the
//     long-established VSCode/Zed convention. The function never
//     mutates its input slice.
//
// Slice 1 of v0.39.0 is just this package and its tests. Slice 2 wires
// it into editor.Pane.ToggleComment(); slice 3 binds Ctrl+/ on the
// host and ships v0.39.0.
package comment

import (
	"path/filepath"
	"strings"
)

// Op identifies which direction ToggleLines went.
type Op int

const (
	// Comment means prefixes were inserted (the input had at least one
	// non-blank line that was not already prefixed).
	Comment Op = iota
	// Uncomment means prefixes were stripped (every non-blank line was
	// already prefixed).
	Uncomment
	// Noop means there was nothing to do (input was empty or
	// blank-only, or prefix was empty).
	Noop
)

// Prefix returns the line-comment marker for the file at path, based
// purely on its extension. Returns "" for unknown extensions or
// extensions where line-comments don't exist (CSS, HTML, XML); the
// caller should treat "" as "comment-toggle not supported here" and
// no-op.
//
// The table is intentionally small and explicit. New languages get
// added by editing the switch, not by config. The convention is
// "marker followed by one space" so a toggled line reads naturally —
// `// foo`, not `//foo`.
func Prefix(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	// Files with no extension fall back to the basename so Makefile,
	// Dockerfile, etc. still get a prefix.
	if ext == "" {
		ext = strings.ToLower(filepath.Base(path))
	}
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
		".rs", ".zig", ".c", ".h", ".cpp", ".cc", ".cxx", ".hpp",
		".java", ".kt", ".scala", ".swift", ".dart", ".d",
		".cs", ".fs", ".fsi", ".fsx", ".php", ".groovy":
		return "// "
	case ".py", ".sh", ".bash", ".zsh", ".fish", ".pl", ".rb",
		".toml", ".yaml", ".yml", ".tf", ".tfvars", ".hcl",
		".r", ".jl", ".ex", ".exs", ".elm",
		"makefile", "dockerfile", "rakefile", "gemfile":
		return "# "
	case ".lua", ".hs", ".sql", ".ada", ".vhd", ".vhdl":
		return "-- "
	case ".vim":
		return `" `
	case ".lisp", ".cl", ".el", ".clj", ".cljs", ".edn", ".scm", ".rkt":
		return "; "
	case ".tex", ".bib":
		return "% "
	}
	return ""
}

// ToggleLines applies the comment-toggle rule to the given slice and
// reports which direction it went. The returned slice is a fresh
// allocation; the input is never mutated. Blank lines (rows that are
// entirely whitespace) are passed through unchanged in both
// directions.
//
// The "are we already commented?" check inspects only non-blank lines.
// A range that is entirely blank lines returns Noop and a copy of the
// input. A range that mixes blank lines with already-commented lines
// still uncomments — the blanks pass through, the commented rows lose
// their prefix.
func ToggleLines(lines []string, prefix string) ([]string, Op) {
	if prefix == "" || len(lines) == 0 {
		return append([]string(nil), lines...), Noop
	}

	allCommented := true
	hasNonBlank := false
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		hasNonBlank = true
		if !strings.HasPrefix(trimmed, prefix) {
			allCommented = false
			break
		}
	}
	if !hasNonBlank {
		return append([]string(nil), lines...), Noop
	}

	out := make([]string, len(lines))
	if allCommented {
		for i, line := range lines {
			trimmed := strings.TrimLeft(line, " \t")
			if trimmed == "" {
				out[i] = line
				continue
			}
			indent := line[:len(line)-len(trimmed)]
			// Strip exactly one prefix; if the prefix is `// ` and the
			// row reads `// foo`, the result is `foo` — the trailing
			// space is part of the prefix.
			out[i] = indent + strings.TrimPrefix(trimmed, prefix)
		}
		return out, Uncomment
	}

	// Insert at the minimum non-blank indent so a partially-indented
	// block stays visually aligned. This matches Zed/VSCode behavior:
	// uncommented-then-recommented blocks come back to where they
	// started.
	minIndent := -1
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		ind := len(line) - len(trimmed)
		if minIndent < 0 || ind < minIndent {
			minIndent = ind
		}
	}
	if minIndent < 0 {
		minIndent = 0
	}

	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			out[i] = line
			continue
		}
		// Re-anchor at minIndent so all rows of the toggled block
		// share the same comment column.
		if len(line) >= minIndent {
			out[i] = line[:minIndent] + prefix + line[minIndent:]
		} else {
			out[i] = prefix + line
		}
	}
	return out, Comment
}
