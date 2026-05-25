// Package airules loads repo-level AI instructions from a conventional
// dotfile at the workspace root. nook supports two filenames:
//
//   - .nookrules — nook-native, takes precedence
//   - .cursorrules — Cursor's convention, accepted as a drop-in fallback
//
// The intent is that a user dropping nook into an existing Cursor repo
// inherits the same model behavior with zero re-onboarding. The loaded
// content is appended to every AI wedge's system prompt (ghost / edit /
// composer) so prose conventions ("use tabs, fmt.Errorf wraps errors,
// never wrap at 80 cols") ride along on every call.
//
// Both files missing is the common case: Load returns (SourceNone, "", nil)
// and the AI wedges run unchanged. A whitespace-only file is treated as
// empty (no source, no content) so an empty `touch .nookrules` doesn't
// register a useless indicator.
package airules

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Source names which file the rules came from. The empty SourceNone is
// the no-file case; the host treats it as "do not surface a rules chip
// in the status bar."
type Source string

const (
	// SourceNone is the zero value: no rules file present (or both are
	// whitespace-only).
	SourceNone Source = ""
	// SourceNookrules is the nook-native filename.
	SourceNookrules Source = "nookrules"
	// SourceCursorrules is the Cursor-compatibility filename.
	SourceCursorrules Source = "cursorrules"
)

// Filename returns the dotfile name on disk that backs the source.
func (s Source) Filename() string {
	switch s {
	case SourceNookrules:
		return ".nookrules"
	case SourceCursorrules:
		return ".cursorrules"
	}
	return ""
}

// Load reads rules from `root`. It tries `.nookrules` first and falls
// back to `.cursorrules`. A file that exists but is whitespace-only is
// skipped (treated as if absent). Errors other than os.ErrNotExist are
// surfaced so the host can show a status hint and still launch nook
// (the AI wedges keep working — they just don't get the extra prompt).
//
// The returned content is trimmed (leading/trailing whitespace removed)
// so we don't tack stray newlines onto the system prompt.
func Load(root string) (Source, string, error) {
	for _, src := range []Source{SourceNookrules, SourceCursorrules} {
		path := filepath.Join(root, src.Filename())
		b, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return SourceNone, "", err
		}
		s := strings.TrimSpace(string(b))
		if s == "" {
			// Whitespace-only file: skip it. The user probably touched
			// the file but hasn't written anything yet.
			continue
		}
		return src, s, nil
	}
	return SourceNone, "", nil
}

// AugmentSystemPrompt returns base with a tagged trailer carrying the
// rules. Empty rules is a no-op (returns base unchanged) so callers can
// pipe the result through unconditionally. The trailer is fenced with a
// header line so the model can distinguish task-specific instructions
// from repo conventions.
func AugmentSystemPrompt(base, rules string) string {
	if strings.TrimSpace(rules) == "" {
		return base
	}
	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\n---\nRepository conventions (from project rules file). Treat these as additional constraints layered on top of the task above:\n\n")
	b.WriteString(rules)
	return b.String()
}

// StatusChip returns the short label shown in the status bar when a
// rules file is loaded. Empty source returns "" so the host can drop
// the chip entirely.
func StatusChip(s Source) string {
	if s == SourceNone {
		return ""
	}
	return "rules:" + string(s)
}
