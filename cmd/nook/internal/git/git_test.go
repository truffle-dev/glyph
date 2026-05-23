package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
}

// newRepo creates an isolated git repo with a deterministic identity config.
func newRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		// pin identity so commits succeed inside fresh tempdirs
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=nook-test",
			"GIT_AUTHOR_EMAIL=nook@example.com",
			"GIT_COMMITTER_NAME=nook-test",
			"GIT_COMMITTER_EMAIL=nook@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "nook-test")
	run("config", "user.email", "nook@example.com")
	run("config", "commit.gpgsign", "false")
	return root
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitInRepo(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=nook-test",
		"GIT_AUTHOR_EMAIL=nook@example.com",
		"GIT_COMMITTER_NAME=nook-test",
		"GIT_COMMITTER_EMAIL=nook@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestStatusCleanRepo(t *testing.T) {
	root := newRepo(t)
	write(t, root, "a.txt", "hello\n")
	gitInRepo(t, root, "add", "a.txt")
	gitInRepo(t, root, "commit", "-q", "-m", "init")
	s, err := RunStatus(context.Background(), root)
	if err != nil {
		t.Fatalf("status err: %v", err)
	}
	if s.Branch != "main" {
		t.Fatalf("expected branch main, got %q", s.Branch)
	}
	if len(s.Entries) != 0 {
		t.Fatalf("expected clean tree, got %+v", s.Entries)
	}
}

func TestStatusUntracked(t *testing.T) {
	root := newRepo(t)
	write(t, root, "a.txt", "hi\n")
	gitInRepo(t, root, "add", "a.txt")
	gitInRepo(t, root, "commit", "-q", "-m", "init")
	write(t, root, "new.txt", "new\n")
	s, err := RunStatus(context.Background(), root)
	if err != nil {
		t.Fatalf("status err: %v", err)
	}
	if len(s.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %+v", s.Entries)
	}
	if !s.Entries[0].Untracked() {
		t.Fatalf("expected untracked, got %+v", s.Entries[0])
	}
	if s.Entries[0].Path != "new.txt" {
		t.Fatalf("expected path new.txt, got %q", s.Entries[0].Path)
	}
}

func TestStageUnstageCycle(t *testing.T) {
	root := newRepo(t)
	write(t, root, "a.txt", "hi\n")
	gitInRepo(t, root, "add", "a.txt")
	gitInRepo(t, root, "commit", "-q", "-m", "init")
	write(t, root, "a.txt", "bye\n")

	// before stage: WorkTree = M
	s, _ := RunStatus(context.Background(), root)
	if len(s.Entries) != 1 || s.Entries[0].WorkTree != StatusModified {
		t.Fatalf("expected single modified entry, got %+v", s.Entries)
	}

	if err := Stage(context.Background(), root, "a.txt"); err != nil {
		t.Fatalf("stage err: %v", err)
	}
	s, _ = RunStatus(context.Background(), root)
	if len(s.Entries) != 1 || s.Entries[0].Index != StatusModified {
		t.Fatalf("expected staged modified entry, got %+v", s.Entries)
	}

	if err := Unstage(context.Background(), root, "a.txt"); err != nil {
		t.Fatalf("unstage err: %v", err)
	}
	s, _ = RunStatus(context.Background(), root)
	if len(s.Entries) != 1 || s.Entries[0].WorkTree != StatusModified || s.Entries[0].Index != ' ' {
		t.Fatalf("expected unstaged modified after Unstage, got %+v", s.Entries)
	}
}

func TestDiffShowsBody(t *testing.T) {
	root := newRepo(t)
	write(t, root, "a.txt", "hello\n")
	gitInRepo(t, root, "add", "a.txt")
	gitInRepo(t, root, "commit", "-q", "-m", "init")
	write(t, root, "a.txt", "hello\nworld\n")
	body, err := Diff(context.Background(), root, "a.txt", false)
	if err != nil {
		t.Fatalf("diff err: %v", err)
	}
	if !strings.Contains(body, "+world") {
		t.Fatalf("expected '+world' in diff:\n%s", body)
	}
}

func TestCommitProducesSHA(t *testing.T) {
	root := newRepo(t)
	write(t, root, "a.txt", "hello\n")
	gitInRepo(t, root, "add", "a.txt")
	sha, err := Commit(context.Background(), root, "init commit")
	if err != nil {
		t.Fatalf("commit err: %v", err)
	}
	if len(sha) < 7 {
		t.Fatalf("expected sha, got %q", sha)
	}
	logOut := gitInRepo(t, root, "log", "-1", "--format=%s")
	if logOut != "init commit" {
		t.Fatalf("expected commit subject, got %q", logOut)
	}
}

func TestCommitRejectsEmptyMessage(t *testing.T) {
	root := newRepo(t)
	if _, err := Commit(context.Background(), root, "   "); err == nil {
		t.Fatal("expected error on empty message")
	}
}

func TestParseBranchAheadBehind(t *testing.T) {
	cases := []struct {
		in     string
		branch string
		ahead  int
		behind int
	}{
		{"main", "main", 0, 0},
		{"main...origin/main", "main", 0, 0},
		{"main...origin/main [ahead 3]", "main", 3, 0},
		{"main...origin/main [ahead 1, behind 2]", "main", 1, 2},
		{"main...origin/main [behind 5]", "main", 0, 5},
	}
	for _, c := range cases {
		b, a, bh := parseBranchLine(c.in)
		if b != c.branch || a != c.ahead || bh != c.behind {
			t.Fatalf("parseBranchLine(%q) = (%q, %d, %d), want (%q, %d, %d)",
				c.in, b, a, bh, c.branch, c.ahead, c.behind)
		}
	}
}

func TestPaneStageEmitsMsg(t *testing.T) {
	root := newRepo(t)
	write(t, root, "a.txt", "hello\n")
	gitInRepo(t, root, "add", "a.txt")
	gitInRepo(t, root, "commit", "-q", "-m", "init")
	write(t, root, "a.txt", "changed\n")

	s, _ := RunStatus(context.Background(), root)
	p := NewPane(theme.Default, root).WithSize(80, 20).Focus().SetStatus(s)
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected Cmd from 's'")
	}
	msg := cmd()
	st, ok := msg.(StagedMsg)
	if !ok {
		t.Fatalf("expected StagedMsg, got %T", msg)
	}
	if st.Err != nil {
		t.Fatalf("stage err: %v", st.Err)
	}
	if st.Path != "a.txt" {
		t.Fatalf("expected path a.txt, got %q", st.Path)
	}
}

func TestPaneCommitEditor(t *testing.T) {
	root := newRepo(t)
	p := NewPane(theme.Default, root).WithSize(80, 20).Focus()
	// enter editor
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if !p.Editing() {
		t.Fatal("expected editing mode after 'c'")
	}
	// type some text
	for _, r := range "hello" {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if p.CommitMessage() != "hello" {
		t.Fatalf("expected 'hello', got %q", p.CommitMessage())
	}
	// backspace once
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.CommitMessage() != "hell" {
		t.Fatalf("expected 'hell', got %q", p.CommitMessage())
	}
	// esc exits editor
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.Editing() {
		t.Fatal("expected editing=false after Esc")
	}
}

func TestPaneDiffEnter(t *testing.T) {
	root := newRepo(t)
	write(t, root, "a.txt", "hello\n")
	gitInRepo(t, root, "add", "a.txt")
	gitInRepo(t, root, "commit", "-q", "-m", "init")
	write(t, root, "a.txt", "hello\nworld\n")
	s, _ := RunStatus(context.Background(), root)
	p := NewPane(theme.Default, root).WithSize(80, 20).Focus().SetStatus(s)
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected diff Cmd from Enter")
	}
	msg := cmd()
	d, ok := msg.(DiffMsg)
	if !ok {
		t.Fatalf("expected DiffMsg, got %T", msg)
	}
	if d.Err != nil {
		t.Fatalf("diff err: %v", d.Err)
	}
	if !strings.Contains(d.Body, "+world") {
		t.Fatalf("expected +world, got:\n%s", d.Body)
	}
}

func TestPaneViewRenders(t *testing.T) {
	root := newRepo(t)
	write(t, root, "a.txt", "hello\n")
	gitInRepo(t, root, "add", "a.txt")
	gitInRepo(t, root, "commit", "-q", "-m", "init")
	write(t, root, "a.txt", "changed\n")
	s, _ := RunStatus(context.Background(), root)
	p := NewPane(theme.Default, root).WithSize(80, 20).SetStatus(s)
	out := p.View()
	if out == "" {
		t.Fatal("expected non-empty View()")
	}
	if !strings.Contains(out, "main") {
		t.Fatalf("expected 'main' branch in output:\n%s", out)
	}
}

func TestPaneCleanRepo(t *testing.T) {
	root := newRepo(t)
	write(t, root, "a.txt", "hello\n")
	gitInRepo(t, root, "add", "a.txt")
	gitInRepo(t, root, "commit", "-q", "-m", "init")
	s, _ := RunStatus(context.Background(), root)
	p := NewPane(theme.Default, root).WithSize(80, 20).SetStatus(s)
	out := p.View()
	if !strings.Contains(out, "clean") {
		t.Fatalf("expected 'clean' in view:\n%s", out)
	}
}
