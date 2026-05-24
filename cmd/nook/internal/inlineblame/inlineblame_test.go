package inlineblame

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSingleCommitTwoLines(t *testing.T) {
	out := []byte(strings.Join([]string{
		"abc1234def567890123456789012345678901234 1 1 2",
		"author Jane Doe",
		"author-mail <jane@example.com>",
		"author-time 1700000000",
		"author-tz +0000",
		"committer Jane Doe",
		"committer-mail <jane@example.com>",
		"committer-time 1700000000",
		"committer-tz +0000",
		"summary first commit",
		"filename hello.go",
		"\tfirst line",
		"abc1234def567890123456789012345678901234 2 2",
		"\tsecond line",
	}, "\n"))
	lines := Parse(out)
	if len(lines) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(lines))
	}
	first, ok := lines[0]
	if !ok {
		t.Fatalf("missing row 0")
	}
	if first.Author != "Jane Doe" {
		t.Errorf("author = %q want Jane Doe", first.Author)
	}
	if first.Email != "jane@example.com" {
		t.Errorf("email = %q want jane@example.com", first.Email)
	}
	if first.Summary != "first commit" {
		t.Errorf("summary = %q want first commit", first.Summary)
	}
	if first.SHA != "abc1234def567890123456789012345678901234" {
		t.Errorf("sha = %q", first.SHA)
	}
	if first.Time.Unix() != 1700000000 {
		t.Errorf("time = %v want unix 1700000000", first.Time)
	}
	second, ok := lines[1]
	if !ok {
		t.Fatalf("missing row 1")
	}
	if second.Author != first.Author {
		t.Errorf("repeat-commit author should fill from first sighting; got %q", second.Author)
	}
	if second.Summary != first.Summary {
		t.Errorf("repeat-commit summary should fill from first sighting; got %q", second.Summary)
	}
}

func TestParseUncommittedZeroSHA(t *testing.T) {
	out := []byte(strings.Join([]string{
		"0000000000000000000000000000000000000000 1 1 1",
		"author Not Committed Yet",
		"author-mail <not.committed.yet>",
		"author-time 1700001000",
		"author-tz +0000",
		"summary Version of hello.go from hello.go",
		"\tworking-tree edit",
	}, "\n"))
	lines := Parse(out)
	got, ok := lines[0]
	if !ok {
		t.Fatalf("missing row 0")
	}
	if !got.IsUncommitted() {
		t.Errorf("expected IsUncommitted=true for zero SHA")
	}
}

func TestParseTwoCommitsInterleaved(t *testing.T) {
	out := []byte(strings.Join([]string{
		"1111111111111111111111111111111111111111 1 1 1",
		"author Alice",
		"author-mail <a@x>",
		"author-time 1700000100",
		"summary alpha",
		"\tline one",
		"2222222222222222222222222222222222222222 2 2 1",
		"author Bob",
		"author-mail <b@x>",
		"author-time 1700000200",
		"summary beta",
		"\tline two",
		"1111111111111111111111111111111111111111 3 3 1",
		"\tline three",
	}, "\n"))
	lines := Parse(out)
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d", len(lines))
	}
	if lines[0].Author != "Alice" {
		t.Errorf("row 0 author = %q", lines[0].Author)
	}
	if lines[1].Author != "Bob" {
		t.Errorf("row 1 author = %q", lines[1].Author)
	}
	if lines[2].Author != "Alice" {
		t.Errorf("row 2 author (repeat of sha1) = %q want Alice", lines[2].Author)
	}
	if lines[2].SHA != "1111111111111111111111111111111111111111" {
		t.Errorf("row 2 sha = %q", lines[2].SHA)
	}
}

func TestParseEmptyInput(t *testing.T) {
	lines := Parse(nil)
	if len(lines) != 0 {
		t.Errorf("expected empty map, got %d entries", len(lines))
	}
	lines = Parse([]byte{})
	if len(lines) != 0 {
		t.Errorf("expected empty map, got %d entries", len(lines))
	}
}

func TestParseSkipsMalformedHeaders(t *testing.T) {
	out := []byte(strings.Join([]string{
		"not-a-sha 1 1 1",
		"author Alice",
		"\tcontent line",
	}, "\n"))
	lines := Parse(out)
	if len(lines) != 0 {
		t.Errorf("malformed header should produce no entries; got %d", len(lines))
	}
}

func TestIsHexAcceptsLowerAndUpperHex(t *testing.T) {
	if !isHex("0123456789abcdefABCDEF") {
		t.Errorf("isHex rejected valid hex")
	}
	if isHex("xyz") {
		t.Errorf("isHex accepted non-hex")
	}
	if isHex("0123456789!") {
		t.Errorf("isHex accepted punctuation")
	}
}

func TestHumanizeSinceBuckets(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		offset time.Duration
		want   string
	}{
		{0, "just now"},
		{-30 * time.Second, "just now"},
		{-1 * time.Minute, "1 minute ago"},
		{-5 * time.Minute, "5 minutes ago"},
		{-1 * time.Hour, "1 hour ago"},
		{-3 * time.Hour, "3 hours ago"},
		{-25 * time.Hour, "yesterday"},
		{-3 * 24 * time.Hour, "3 days ago"},
		{-7 * 24 * time.Hour, "1 week ago"},
		{-14 * 24 * time.Hour, "2 weeks ago"},
		{-31 * 24 * time.Hour, "1 month ago"},
		{-90 * 24 * time.Hour, "3 months ago"},
		{-400 * 24 * time.Hour, "1 year ago"},
		{-365 * 3 * 24 * time.Hour, "3 years ago"},
	}
	for _, c := range cases {
		got := HumanizeSince(now, now.Add(c.offset))
		if got != c.want {
			t.Errorf("offset %v: got %q want %q", c.offset, got, c.want)
		}
	}
}

func TestHumanizeSinceZeroTime(t *testing.T) {
	if got := HumanizeSince(time.Now(), time.Time{}); got != "" {
		t.Errorf("zero time should return empty; got %q", got)
	}
}

func TestRenderCommittedLine(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	line := Line{
		SHA:     "abc1234567890abcdef1234567890abcdef12345",
		Author:  "Jane Doe",
		Time:    now.Add(-3 * 24 * time.Hour),
		Summary: "fix: handle empty inputs",
	}
	got := Render(line, now, 0)
	want := "Jane Doe • 3 days ago • fix: handle empty inputs"
	if got != want {
		t.Errorf("render = %q want %q", got, want)
	}
}

func TestRenderTruncatesSummary(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	line := Line{
		SHA:     "abc1234567890abcdef1234567890abcdef12345",
		Author:  "Jane Doe",
		Time:    now.Add(-3 * 24 * time.Hour),
		Summary: "this summary is intentionally far longer than the cap to verify truncation",
	}
	got := Render(line, now, 20)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated render should end in ellipsis: %q", got)
	}
	// 20 runes of summary + ellipsis = budget; verify final piece length.
	idx := strings.LastIndex(got, " • ")
	if idx < 0 {
		t.Fatalf("no bullet separator in %q", got)
	}
	suffix := []rune(got[idx+len(" • "):])
	if len(suffix) != 20 {
		t.Errorf("summary part should be 20 runes; got %d (%q)", len(suffix), string(suffix))
	}
}

func TestRenderUncommitted(t *testing.T) {
	now := time.Now()
	line := Line{SHA: UncommittedSHA, Author: "Not Committed Yet"}
	if got := Render(line, now, 0); got != "(uncommitted)" {
		t.Errorf("render = %q", got)
	}
}

func TestRenderEmptyLine(t *testing.T) {
	if got := Render(Line{}, time.Now(), 0); got != "" {
		t.Errorf("empty line should render empty; got %q", got)
	}
}

func TestComputeOutsideRepoReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	out, err := Compute(context.Background(), dir, filepath.Join(dir, "x"))
	if err != nil {
		t.Fatalf("Compute outside repo errored: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty map outside repo, got %d entries", len(out))
	}
}

func TestComputeEmptyArgs(t *testing.T) {
	if _, err := Compute(context.Background(), "", "x"); err == nil {
		t.Errorf("empty root should error")
	}
	if _, err := Compute(context.Background(), "/tmp", ""); err == nil {
		t.Errorf("empty path should error")
	}
}

func TestComputeEndToEndAgainstRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-q", "-b", "main")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "Test User")
	run("git", "config", "commit.gpgsign", "false")
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("line one\nline two\nline three\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "hello.txt")
	run("git", "commit", "-q", "-m", "initial: add hello")
	if err := os.WriteFile(path, []byte("line one\nline TWO\nline three\nline four\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := Compute(context.Background(), dir, "hello.txt")
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected 4 blamed lines (3 from initial + 1 uncommitted), got %d", len(out))
	}
	row0, ok := out[0]
	if !ok || row0.Author != "Test User" {
		t.Errorf("row 0: %+v", row0)
	}
	if row0.Summary != "initial: add hello" {
		t.Errorf("row 0 summary = %q", row0.Summary)
	}
	// Row 1 was modified — still uncommitted relative to working tree.
	if !out[1].IsUncommitted() {
		t.Errorf("row 1 should be uncommitted (modified line); got SHA %q", out[1].SHA)
	}
	if !out[3].IsUncommitted() {
		t.Errorf("row 3 should be uncommitted (new line); got SHA %q", out[3].SHA)
	}
}

func TestBlameCmdPathRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cmd := BlameCmd(dir, filepath.Join(dir, "missing.txt"))
	msg := cmd().(BlameMsg)
	if msg.Path != filepath.Join(dir, "missing.txt") {
		t.Errorf("path = %q", msg.Path)
	}
	// Outside-repo path returns empty map + nil error.
	if msg.Err != nil {
		t.Errorf("err = %v", msg.Err)
	}
}
