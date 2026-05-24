// Package gitgutter computes per-line diff markers (added / modified /
// deleted) for a file by shelling out to `git diff` against the index.
//
// The host calls Compute (or the matching tea.Cmd factory MarkerCmd) when
// it opens or saves a file; the returned Markers map is handed to the
// editor pane which paints each marked row's gutter with a colored sigil.
//
// Three states are surfaced:
//
//   - Added — a line present in the working tree but not in the index.
//     This includes every line of an untracked file (compared against
//     /dev/null) and lines newly inserted into a tracked file.
//   - Modified — a line whose content differs between working tree and
//     index. Pure replacements ("oldCount > 0 && newCount > 0") in unified
//     diff terms.
//   - DeletedAbove — a line in the working tree immediately following a
//     stretch of lines that were deleted relative to the index. Painted
//     on the surviving line below the deletion (Zed / VS Code convention).
//
// Files outside a git repository, binary files, or paths git cannot read
// produce an empty map and a non-nil error. The host treats this as
// "no markers" and the editor renders an unadorned gutter.
package gitgutter

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Marker is a per-line diff state. Zero value (None) means the line is
// unchanged relative to the index.
type Marker int

const (
	None Marker = iota
	Added
	Modified
	DeletedAbove
)

// String renders the marker as a short tag for tests and debug logs.
func (m Marker) String() string {
	switch m {
	case Added:
		return "added"
	case Modified:
		return "modified"
	case DeletedAbove:
		return "deleted-above"
	default:
		return "none"
	}
}

// Compute runs git against the working tree to produce the per-line markers
// for path. Returns an empty map (and nil error) when the file is tracked
// and clean; an empty map with a non-nil error when git itself failed or
// the file is outside any git repo.
//
// Untracked files are detected via `git ls-files --error-unmatch` and then
// treated as wholly-added by reading the working-tree file's line count.
func Compute(ctx context.Context, root, path string) (map[int]Marker, error) {
	if root == "" || path == "" {
		return nil, fmt.Errorf("gitgutter: empty root or path")
	}

	// Repo check. If root isn't inside a git work tree, there's nothing to
	// paint — return an empty map without error so the host treats this as
	// "no markers" silently.
	insideCmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--is-inside-work-tree")
	insideCmd.Stdout = nil
	insideCmd.Stderr = nil
	if err := insideCmd.Run(); err != nil {
		return map[int]Marker{}, nil
	}

	// Tracked check. `--error-unmatch` exits 1 when the file isn't tracked.
	tracked := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--error-unmatch", "--", path)
	tracked.Stdout = nil
	tracked.Stderr = nil
	if err := tracked.Run(); err != nil {
		// Untracked file inside a repo. Diff against /dev/null so every line
		// surfaces as Added. `git diff --no-index` exits 1 when there are
		// differences (the normal case here); we distinguish that from a
		// genuine failure (exit > 1 or no output).
		out, derr := runGit(ctx, root, "diff", "--no-color", "--unified=0", "--no-index", "--", "/dev/null", path)
		if derr != nil {
			if exitErr, ok := derr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 && len(out) > 0 {
				return Parse(out), nil
			}
			return nil, derr
		}
		// No output means the working-tree file is identical to /dev/null,
		// i.e. empty. No markers to paint.
		return map[int]Marker{}, nil
	}

	out, err := runGit(ctx, root, "diff", "--no-color", "--unified=0", "--", path)
	if err != nil {
		return nil, err
	}
	return Parse(out), nil
}

// Parse extracts per-row markers from unified-diff output. The input may
// contain output for any number of files, in any order; Parse takes them
// all (the caller is expected to pass output for the file it cares about).
// Row indices in the returned map are 0-based positions in the WORKING-TREE
// file.
func Parse(diff []byte) map[int]Marker {
	markers := map[int]Marker{}
	scanner := bytes.Split(diff, []byte("\n"))
	for _, raw := range scanner {
		if !bytes.HasPrefix(raw, []byte("@@")) {
			continue
		}
		oldCount, newStart, newCount, ok := parseHunkHeader(raw)
		if !ok {
			continue
		}
		switch {
		case oldCount == 0 && newCount > 0:
			// Pure addition. Mark every new line.
			for r := 0; r < newCount; r++ {
				markers[newStart-1+r] = Added
			}
		case newCount == 0 && oldCount > 0:
			// Pure deletion. newStart is the line BEFORE which the deletion
			// happened (1-based). Mark the surviving line below — that's
			// newStart itself in 1-based, newStart in 0-based interpreted as
			// "the row that visually has a deletion above it". For top-of-
			// file deletions (newStart == 0), clamp to row 0.
			row := newStart
			if row > 0 {
				// In unified-0 deletion shape, newStart is the LAST surviving
				// line BEFORE the deletion. The "line below the deletion" is
				// row (1-based) → row (0-based). But when there's no surviving
				// line after the deletion (deletion at EOF), the row may be
				// past the end of the file — the editor's render loop will
				// skip those.
			}
			if existing, ok := markers[row]; !ok || existing == None {
				markers[row] = DeletedAbove
			}
		case oldCount > 0 && newCount > 0:
			for r := 0; r < newCount; r++ {
				markers[newStart-1+r] = Modified
			}
		}
	}
	return markers
}

// MarkersMsg is the response from MarkerCmd. Path is the absolute path the
// markers belong to; the host compares against the active buffer's path
// before applying so a late response for an old buffer is dropped.
type MarkersMsg struct {
	Path    string
	Markers map[int]Marker
	Err     error
}

// MarkerCmd returns a tea.Cmd that runs Compute in a background goroutine
// and wraps the result into a MarkersMsg. Callers pin path at call time so
// a late response can be discarded if the active buffer has moved on.
//
// We deliberately do NOT thread a context here — the cost of a `git diff
// --unified=0` on a single file is bounded and the host fires this only on
// open/save (not per-keystroke). If a future profile demands cancellation
// the signature can grow a context without breaking callers.
func MarkerCmd(root, path string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		markers, err := Compute(ctx, root, path)
		return MarkersMsg{Path: path, Markers: markers, Err: err}
	}
}

// parseHunkHeader parses a unified-diff hunk header line of the form
//
//	@@ -<oldStart>[,<oldCount>] +<newStart>[,<newCount>] @@ ...
//
// and returns (oldCount, newStart, newCount, ok). oldStart is discarded:
// the host only needs the new-file coordinates for painting.
//
// Counts default to 1 when omitted (per the unified-diff spec). A
// malformed header returns ok=false and the caller skips it.
func parseHunkHeader(line []byte) (oldCount, newStart, newCount int, ok bool) {
	s := string(line)
	if !strings.HasPrefix(s, "@@ ") {
		return 0, 0, 0, false
	}
	rest := s[3:]
	end := strings.Index(rest, " @@")
	if end < 0 {
		return 0, 0, 0, false
	}
	body := rest[:end]
	parts := strings.Fields(body)
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "-") || !strings.HasPrefix(parts[1], "+") {
		return 0, 0, 0, false
	}
	_, oldCount, ok1 := parseRange(parts[0][1:])
	newStart, newCount, ok2 := parseRange(parts[1][1:])
	if !ok1 || !ok2 {
		return 0, 0, 0, false
	}
	return oldCount, newStart, newCount, true
}

// parseRange parses "N" or "N,M" into (N, M, ok). When the count is
// omitted, M defaults to 1.
func parseRange(s string) (start, count int, ok bool) {
	count = 1
	if i := strings.Index(s, ","); i >= 0 {
		n, err := strconv.Atoi(s[:i])
		if err != nil {
			return 0, 0, false
		}
		m, err := strconv.Atoi(s[i+1:])
		if err != nil {
			return 0, 0, false
		}
		return n, m, true
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, 0, false
	}
	return n, count, true
}

// runGit executes git in the given root with the supplied args and returns
// its combined stdout/stderr. The command's working directory is set via
// `-C` (not via cmd.Dir) so callers can pass any subdirectory of the repo.
func runGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	full := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}
