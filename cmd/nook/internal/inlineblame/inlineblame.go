// Package inlineblame computes per-line git blame for a file by shelling
// out to `git blame --porcelain`.
//
// The host calls Compute (or the matching tea.Cmd factory BlameCmd) when
// it opens or saves a file; the returned map is handed to the editor pane
// which renders the cursor row's blame as dim italic text after the end
// of the line.
//
// Files outside a git repository, binary files, or paths git cannot read
// produce an empty map and a non-nil error. The host treats this as "no
// blame" and the editor renders nothing after EOL.
package inlineblame

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// UncommittedSHA is the all-zero SHA git emits for working-tree lines that
// haven't been committed yet. Callers render these specially (no author /
// no time).
const UncommittedSHA = "0000000000000000000000000000000000000000"

// Line is the blame data for one row of a file. Row indices are stored
// 0-based to match the editor's internal coordinate system.
type Line struct {
	SHA     string
	Author  string
	Email   string
	Time    time.Time
	Summary string
}

// IsUncommitted reports whether the blame row corresponds to a line that
// hasn't been committed yet (still in the working tree).
func (l Line) IsUncommitted() bool {
	return l.SHA == UncommittedSHA
}

// Compute runs `git blame --porcelain` against path and parses the output
// into a row→Line map. Returns an empty map (and nil error) when the file
// is outside a git work tree; an empty map with a non-nil error when git
// itself failed.
func Compute(ctx context.Context, root, path string) (map[int]Line, error) {
	if root == "" || path == "" {
		return nil, fmt.Errorf("inlineblame: empty root or path")
	}

	inside := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--is-inside-work-tree")
	inside.Stdout = nil
	inside.Stderr = nil
	if err := inside.Run(); err != nil {
		return map[int]Line{}, nil
	}

	out, err := runGit(ctx, root, "blame", "--porcelain", "--", path)
	if err != nil {
		return nil, err
	}
	return Parse(out), nil
}

// Parse extracts row→Line entries from `git blame --porcelain` output.
// Row indices in the returned map are 0-based positions in the working-
// tree file.
//
// The porcelain format groups header lines per commit (first appearance
// gets author/summary/etc., subsequent appearances of the same commit
// just get the SHA + line numbers). We track per-SHA metadata so the
// abbreviated repeats fill in from the first sighting.
func Parse(out []byte) map[int]Line {
	type commitMeta struct {
		author     string
		email      string
		authorTime time.Time
		summary    string
	}
	commits := map[string]*commitMeta{}
	lines := map[int]Line{}

	var current *commitMeta
	var curSHA string
	var curResultLine int

	scanner := bytes.Split(out, []byte("\n"))
	for _, raw := range scanner {
		// Content lines start with a tab. Everything else is a header.
		if len(raw) > 0 && raw[0] == '\t' {
			// Finalize the line: record into the map.
			if current != nil {
				lines[curResultLine-1] = Line{
					SHA:     curSHA,
					Author:  current.author,
					Email:   current.email,
					Time:    current.authorTime,
					Summary: current.summary,
				}
			}
			current = nil
			continue
		}
		line := string(raw)
		if line == "" {
			continue
		}
		// SHA header: "<sha> <orig-line> <result-line> [<count>]"
		// SHA is 40 hex characters. Subsequent lines for an existing SHA
		// skip the per-commit metadata.
		if len(line) >= 40 && isHex(line[:40]) && (len(line) == 40 || line[40] == ' ') {
			fields := strings.Fields(line)
			curSHA = fields[0]
			if len(fields) >= 3 {
				if n, err := strconv.Atoi(fields[2]); err == nil {
					curResultLine = n
				}
			}
			if existing, ok := commits[curSHA]; ok {
				current = existing
			} else {
				current = &commitMeta{}
				commits[curSHA] = current
			}
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "author ") {
			current.author = strings.TrimPrefix(line, "author ")
			continue
		}
		if strings.HasPrefix(line, "author-mail ") {
			mail := strings.TrimPrefix(line, "author-mail ")
			mail = strings.TrimPrefix(mail, "<")
			mail = strings.TrimSuffix(mail, ">")
			current.email = mail
			continue
		}
		if strings.HasPrefix(line, "author-time ") {
			if ts, err := strconv.ParseInt(strings.TrimPrefix(line, "author-time "), 10, 64); err == nil {
				current.authorTime = time.Unix(ts, 0)
			}
			continue
		}
		if strings.HasPrefix(line, "summary ") {
			current.summary = strings.TrimPrefix(line, "summary ")
			continue
		}
	}
	return lines
}

// HumanizeSince renders the gap between t and now as a short relative phrase
// suitable for the inline-blame strip ("3 weeks ago", "just now", etc.).
// Returns empty string for the zero time (uncommitted lines).
func HumanizeSince(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		n := int(d / time.Minute)
		if n == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", n)
	case d < 24*time.Hour:
		n := int(d / time.Hour)
		if n == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", n)
	case d < 7*24*time.Hour:
		n := int(d / (24 * time.Hour))
		if n == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", n)
	case d < 30*24*time.Hour:
		n := int(d / (7 * 24 * time.Hour))
		if n == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", n)
	case d < 365*24*time.Hour:
		n := int(d / (30 * 24 * time.Hour))
		if n == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", n)
	default:
		n := int(d / (365 * 24 * time.Hour))
		if n == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", n)
	}
}

// Render formats a Line into the inline-blame strip. Uncommitted lines
// render as "(uncommitted)"; committed lines render as
// "Author • <relative time> • Summary" with summary truncated at maxSummary.
// When maxSummary <= 0 no truncation is performed.
func Render(line Line, now time.Time, maxSummary int) string {
	if line.IsUncommitted() {
		return "(uncommitted)"
	}
	if line.SHA == "" && line.Author == "" {
		return ""
	}
	parts := []string{}
	if line.Author != "" {
		parts = append(parts, line.Author)
	}
	if rel := HumanizeSince(now, line.Time); rel != "" {
		parts = append(parts, rel)
	}
	if s := line.Summary; s != "" {
		if maxSummary > 0 && len([]rune(s)) > maxSummary {
			rs := []rune(s)
			s = string(rs[:maxSummary-1]) + "…"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " • ")
}

// BlameMsg is the response from BlameCmd. Path is the absolute path the
// blame belongs to; the host compares against the active buffer's path
// before applying so a late response for an old buffer is dropped.
type BlameMsg struct {
	Path  string
	Lines map[int]Line
	Err   error
}

// BlameCmd returns a tea.Cmd that runs Compute in a background goroutine
// and wraps the result into a BlameMsg.
func BlameCmd(root, path string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		lines, err := Compute(ctx, root, path)
		return BlameMsg{Path: path, Lines: lines, Err: err}
	}
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

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
