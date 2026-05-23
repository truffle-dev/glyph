// Package git wraps the local git CLI for nook's git pane.
//
// Surfaces:
//   - Status: `git status --porcelain=v1 --branch -z` parsed into entries.
//   - Diff: `git diff -- <path>` and `git diff --cached -- <path>` for inspection.
//   - Stage/unstage: `git add` / `git reset HEAD -- <path>`.
//   - Commit: `git commit -m <msg>` (signed if config gpg.sign or commit.gpgsign set).
//
// The Pane renders a two-column status (staged + unstaged), a commit-message
// editor, and emits messages so the host can route to a diff buffer.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// StatusCode is the porcelain v1 single-letter status for one side
// (staged X or unstaged Y).
type StatusCode rune

const (
	StatusUnmodified StatusCode = ' '
	StatusModified   StatusCode = 'M'
	StatusAdded      StatusCode = 'A'
	StatusDeleted    StatusCode = 'D'
	StatusRenamed    StatusCode = 'R'
	StatusCopied     StatusCode = 'C'
	StatusUntracked  StatusCode = '?'
	StatusIgnored    StatusCode = '!'
	StatusConflicted StatusCode = 'U'
)

// Entry is one row in `git status --porcelain`.
type Entry struct {
	Index    StatusCode // staged side (X)
	WorkTree StatusCode // unstaged side (Y)
	Path     string
	OrigPath string // populated on rename
}

// Staged reports whether the index column shows a staged change.
func (e Entry) Staged() bool { return e.Index != ' ' && e.Index != '?' }

// Unstaged reports whether the working-tree column shows an unstaged change.
func (e Entry) Unstaged() bool { return e.WorkTree != ' ' }

// Untracked reports whether the file is untracked.
func (e Entry) Untracked() bool { return e.Index == '?' && e.WorkTree == '?' }

// Status is the parsed result of `git status`.
type Status struct {
	Branch  string
	Ahead   int
	Behind  int
	Entries []Entry
}

// StatusMsg is emitted by RefreshCmd.
type StatusMsg struct {
	Status Status
	Err    error
}

// DiffMsg is emitted by DiffCmd; the host renders this in a side pane.
type DiffMsg struct {
	Path   string
	Staged bool
	Body   string
	Err    error
}

// CommitMsg is emitted by CommitCmd.
type CommitMsg struct {
	SHA string // empty on error
	Err error
}

// CancelMsg is emitted when the user presses Esc.
type CancelMsg struct{}

// Status runs `git status --porcelain=v1 --branch -z` and parses the output.
func RunStatus(ctx context.Context, root string) (Status, error) {
	out, err := runGit(ctx, root, "status", "--porcelain=v1", "--branch", "-z")
	if err != nil {
		return Status{}, err
	}
	return parsePorcelainV1(out), nil
}

// Diff returns `git diff [-- cached] -- <path>`.
func Diff(ctx context.Context, root, path string, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", path)
	out, err := runGit(ctx, root, args...)
	return string(out), err
}

// Stage runs `git add -- <path>`.
func Stage(ctx context.Context, root, path string) error {
	_, err := runGit(ctx, root, "add", "--", path)
	return err
}

// Unstage runs `git reset HEAD -- <path>`.
func Unstage(ctx context.Context, root, path string) error {
	_, err := runGit(ctx, root, "reset", "HEAD", "--", path)
	return err
}

// Commit runs `git commit -m <msg>` and returns the new SHA.
func Commit(ctx context.Context, root, msg string) (string, error) {
	if strings.TrimSpace(msg) == "" {
		return "", errors.New("empty commit message")
	}
	if _, err := runGit(ctx, root, "commit", "-m", msg); err != nil {
		return "", err
	}
	out, err := runGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func runGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// parsePorcelainV1 parses NUL-separated `git status --porcelain=v1 --branch -z`.
// Header: "## branch...remote [ahead N, behind M]\x00"
// Each entry: "XY pathname\x00" or for renames "RC orig\x00new\x00".
func parsePorcelainV1(out []byte) Status {
	s := Status{}
	parts := bytes.Split(out, []byte{0})
	i := 0
	if i < len(parts) && bytes.HasPrefix(parts[i], []byte("## ")) {
		s.Branch, s.Ahead, s.Behind = parseBranchLine(string(parts[i][3:]))
		i++
	}
	for ; i < len(parts); i++ {
		raw := parts[i]
		if len(raw) < 3 {
			continue
		}
		e := Entry{
			Index:    StatusCode(raw[0]),
			WorkTree: StatusCode(raw[1]),
			Path:     string(raw[3:]),
		}
		// Rename: orig path is next field.
		if e.Index == StatusRenamed || e.Index == StatusCopied {
			if i+1 < len(parts) {
				e.OrigPath = string(parts[i+1])
				i++
			}
		}
		s.Entries = append(s.Entries, e)
	}
	return s
}

func parseBranchLine(line string) (branch string, ahead, behind int) {
	// Forms we care about:
	//   "main"                              (no upstream)
	//   "main...origin/main"
	//   "main...origin/main [ahead 1]"
	//   "main...origin/main [ahead 1, behind 2]"
	idx := strings.Index(line, "...")
	if idx == -1 {
		return strings.TrimSpace(line), 0, 0
	}
	branch = line[:idx]
	rest := line[idx+3:]
	if br := strings.Index(rest, " ["); br != -1 {
		ext := rest[br+2:]
		ext = strings.TrimSuffix(ext, "]")
		for _, tok := range strings.Split(ext, ", ") {
			var n int
			fmt.Sscanf(tok, "ahead %d", &n)
			if strings.HasPrefix(tok, "ahead ") {
				ahead = n
			}
			fmt.Sscanf(tok, "behind %d", &n)
			if strings.HasPrefix(tok, "behind ") {
				behind = n
			}
		}
	}
	return branch, ahead, behind
}

// --- Pane ---

// Pane is the git status UI.
type Pane struct {
	theme   theme.Theme
	root    string
	status  Status
	cursor  int
	width   int
	height  int
	focused bool
	msg     []rune // commit message editor
	editing bool
	err     error
}

// NewPane constructs an empty pane rooted at root.
func NewPane(t theme.Theme, root string) Pane {
	return Pane{theme: t, root: root, width: 80, height: 20}
}

// WithSize sets pane dimensions.
func (p Pane) WithSize(w, h int) Pane { p.width = w; p.height = h; return p }

// Focused reports whether the pane has keyboard focus.
func (p Pane) Focused() bool { return p.focused }

// Focus sets focused=true.
func (p Pane) Focus() Pane { p.focused = true; return p }

// Blur sets focused=false and exits commit editing.
func (p Pane) Blur() Pane { p.focused = false; p.editing = false; return p }

// Editing reports whether the commit-message editor has focus.
func (p Pane) Editing() bool { return p.editing }

// CommitMessage returns the current draft message.
func (p Pane) CommitMessage() string { return string(p.msg) }

// SetStatus replaces the pane's status snapshot.
func (p Pane) SetStatus(s Status) Pane {
	p.status = s
	if p.cursor >= len(s.Entries) {
		p.cursor = len(s.Entries) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	return p
}

// Selected returns the currently highlighted entry (if any).
func (p Pane) Selected() (Entry, bool) {
	if p.cursor < 0 || p.cursor >= len(p.status.Entries) {
		return Entry{}, false
	}
	return p.status.Entries[p.cursor], true
}

// Update routes a key event. Returns one of: nil, CancelMsg, or a tea.Cmd
// the host should execute (Stage/Unstage/Commit/Diff).
//
// Keymap (when not editing):
//
//	j / down     move cursor
//	k / up       move cursor
//	enter        request diff
//	s            stage
//	u            unstage
//	c            enter commit-message editor
//	esc          emit CancelMsg
//
// Keymap (when editing):
//
//	ctrl+enter   commit
//	esc          exit editor
//	chars        append to message
//	backspace    delete last char
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	if p.editing {
		return p.updateEditing(km)
	}
	return p.updateBrowsing(km)
}

func (p Pane) updateBrowsing(km tea.KeyMsg) (Pane, tea.Cmd) {
	switch km.Type {
	case tea.KeyEsc:
		return p, func() tea.Msg { return CancelMsg{} }
	case tea.KeyDown, tea.KeyCtrlN:
		if p.cursor < len(p.status.Entries)-1 {
			p.cursor++
		}
		return p, nil
	case tea.KeyUp, tea.KeyCtrlP:
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case tea.KeyEnter:
		if e, ok := p.Selected(); ok {
			path := e.Path
			staged := e.Staged()
			root := p.root
			return p, func() tea.Msg {
				body, err := Diff(context.Background(), root, path, staged)
				return DiffMsg{Path: path, Staged: staged, Body: body, Err: err}
			}
		}
		return p, nil
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "j":
			if p.cursor < len(p.status.Entries)-1 {
				p.cursor++
			}
		case "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case "s":
			if e, ok := p.Selected(); ok {
				path := e.Path
				root := p.root
				return p, func() tea.Msg {
					err := Stage(context.Background(), root, path)
					return StagedMsg{Path: path, Err: err}
				}
			}
		case "u":
			if e, ok := p.Selected(); ok {
				path := e.Path
				root := p.root
				return p, func() tea.Msg {
					err := Unstage(context.Background(), root, path)
					return UnstagedMsg{Path: path, Err: err}
				}
			}
		case "c":
			p.editing = true
		}
	}
	return p, nil
}

func (p Pane) updateEditing(km tea.KeyMsg) (Pane, tea.Cmd) {
	switch km.Type {
	case tea.KeyEsc:
		p.editing = false
		return p, nil
	case tea.KeyCtrlS, tea.KeyCtrlD:
		// Ctrl+S / Ctrl+D submit the commit.
		msg := strings.TrimSpace(string(p.msg))
		if msg == "" {
			return p, nil
		}
		root := p.root
		p.editing = false
		p.msg = nil
		return p, func() tea.Msg {
			sha, err := Commit(context.Background(), root, msg)
			return CommitMsg{SHA: sha, Err: err}
		}
	case tea.KeyBackspace:
		if len(p.msg) > 0 {
			p.msg = p.msg[:len(p.msg)-1]
		}
		return p, nil
	case tea.KeySpace:
		p.msg = append(p.msg, ' ')
		return p, nil
	case tea.KeyEnter:
		p.msg = append(p.msg, '\n')
		return p, nil
	case tea.KeyRunes:
		p.msg = append(p.msg, km.Runes...)
		return p, nil
	}
	return p, nil
}

// StagedMsg is emitted after `git add -- <path>`.
type StagedMsg struct {
	Path string
	Err  error
}

// UnstagedMsg is emitted after `git reset HEAD -- <path>`.
type UnstagedMsg struct {
	Path string
	Err  error
}

// View renders the pane.
func (p Pane) View() string {
	t := p.theme
	titleStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Bold(true)
	branchStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	muted := lipgloss.NewStyle().Foreground(t.TextMuted)
	cursor := lipgloss.NewStyle().Foreground(t.TextInverse).Background(t.Primary).Bold(true)
	stagedStyle := lipgloss.NewStyle().Foreground(t.Success).Bold(true)
	unstagedStyle := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	conflictedStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	editorBorder := lipgloss.NewStyle().Foreground(t.Border)

	header := titleStyle.Render("git")
	if p.status.Branch != "" {
		header += "  " + branchStyle.Render(p.status.Branch)
		if p.status.Ahead+p.status.Behind > 0 {
			tail := ""
			if p.status.Ahead > 0 {
				tail += fmt.Sprintf(" ↑%d", p.status.Ahead)
			}
			if p.status.Behind > 0 {
				tail += fmt.Sprintf(" ↓%d", p.status.Behind)
			}
			header += " " + muted.Render(tail)
		}
	}

	bodyH := p.height - 6 // leave room for editor and footer
	if bodyH < 1 {
		bodyH = 1
	}
	start := 0
	if p.cursor >= bodyH {
		start = p.cursor - bodyH + 1
	}
	end := start + bodyH
	if end > len(p.status.Entries) {
		end = len(p.status.Entries)
	}

	var rows []string
	for i := start; i < end; i++ {
		e := p.status.Entries[i]
		idx, work := byte(e.Index), byte(e.WorkTree)
		idxStr := stylePair(stagedStyle, unstagedStyle, conflictedStyle, idx, true)
		workStr := stylePair(stagedStyle, unstagedStyle, conflictedStyle, work, false)
		line := fmt.Sprintf("%s%s %s", idxStr, workStr, e.Path)
		if e.OrigPath != "" {
			line += muted.Render("  ← " + e.OrigPath)
		}
		if i == p.cursor {
			line = cursor.Render(padRight(line, p.width-2))
		}
		rows = append(rows, line)
	}
	if len(p.status.Entries) == 0 {
		rows = append(rows, muted.Render("clean working tree"))
	}

	for len(rows) < bodyH {
		rows = append(rows, "")
	}

	editor := ""
	if p.editing {
		editor = editorBorder.Render("commit: ") + string(p.msg) + cursor.Render(" ")
	} else if len(p.msg) > 0 {
		editor = muted.Render("commit: ") + string(p.msg)
	} else {
		editor = muted.Render("press c to write commit message")
	}

	footer := muted.Render("j/k move • s stage • u unstage • c commit • enter diff • esc back")
	if p.err != nil {
		footer = lipgloss.NewStyle().Foreground(t.Error).Render("error: "+p.err.Error()) + "  " + footer
	}

	parts := []string{header}
	parts = append(parts, rows...)
	parts = append(parts, "", editor, footer)
	return strings.Join(parts, "\n")
}

func stylePair(staged, unstaged, conflicted lipgloss.Style, c byte, indexCol bool) string {
	if c == ' ' {
		return " "
	}
	if c == 'U' {
		return conflicted.Render(string(c))
	}
	if indexCol {
		return staged.Render(string(c))
	}
	return unstaged.Render(string(c))
}

func padRight(s string, w int) string {
	if w <= 0 {
		return ""
	}
	n := lipgloss.Width(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}
