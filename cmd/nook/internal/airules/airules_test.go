package airules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	src, content, err := Load(dir)
	if err != nil {
		t.Fatalf("Load with no files returned err: %v", err)
	}
	if src != SourceNone {
		t.Fatalf("source = %q, want %q", src, SourceNone)
	}
	if content != "" {
		t.Fatalf("content = %q, want empty", content)
	}
}

func TestLoadNookrulesWins(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, ".nookrules", "use tabs only\n")
	mustWrite(t, dir, ".cursorrules", "should be ignored\n")
	src, content, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned err: %v", err)
	}
	if src != SourceNookrules {
		t.Fatalf("source = %q, want %q (nookrules must take precedence)", src, SourceNookrules)
	}
	if content != "use tabs only" {
		t.Fatalf("content = %q, want %q (trimmed)", content, "use tabs only")
	}
}

func TestLoadCursorrulesFallback(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, ".cursorrules", "  prefer fmt.Errorf wrapping  \n")
	src, content, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned err: %v", err)
	}
	if src != SourceCursorrules {
		t.Fatalf("source = %q, want %q", src, SourceCursorrules)
	}
	if content != "prefer fmt.Errorf wrapping" {
		t.Fatalf("content = %q, want trimmed value", content)
	}
}

func TestLoadWhitespaceOnlyNookrulesFallsThroughToCursorrules(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, ".nookrules", "   \n\t\n   ")
	mustWrite(t, dir, ".cursorrules", "real rules")
	src, content, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned err: %v", err)
	}
	if src != SourceCursorrules {
		t.Fatalf("source = %q, want fallback to %q when nookrules is whitespace-only", src, SourceCursorrules)
	}
	if content != "real rules" {
		t.Fatalf("content = %q, want %q", content, "real rules")
	}
}

func TestLoadBothWhitespaceOnly(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, ".nookrules", "\n")
	mustWrite(t, dir, ".cursorrules", "   ")
	src, content, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned err: %v", err)
	}
	if src != SourceNone {
		t.Fatalf("source = %q, want SourceNone when both files are whitespace", src)
	}
	if content != "" {
		t.Fatalf("content = %q, want empty", content)
	}
}

func TestLoadPropagatesNonNotExistError(t *testing.T) {
	dir := t.TempDir()
	// Make a directory at the .nookrules path so ReadFile returns
	// EISDIR (not fs.ErrNotExist). Load should surface that.
	if err := os.Mkdir(filepath.Join(dir, ".nookrules"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src, content, err := Load(dir)
	if err == nil {
		t.Fatalf("expected error from ReadFile on directory; got src=%q content=%q", src, content)
	}
}

func TestAugmentSystemPromptEmpty(t *testing.T) {
	base := "you are an editor."
	got := AugmentSystemPrompt(base, "")
	if got != base {
		t.Fatalf("Augment with empty rules changed base; got %q", got)
	}
	got = AugmentSystemPrompt(base, "   \n\t")
	if got != base {
		t.Fatalf("Augment with whitespace-only rules changed base; got %q", got)
	}
}

func TestAugmentSystemPromptAttachesTrailer(t *testing.T) {
	base := "you are an editor."
	rules := "use tabs only."
	got := AugmentSystemPrompt(base, rules)
	if !strings.HasPrefix(got, base) {
		t.Fatalf("augmented prompt must start with base; got %q", got)
	}
	if !strings.Contains(got, "Repository conventions") {
		t.Fatalf("augmented prompt missing trailer header; got %q", got)
	}
	if !strings.HasSuffix(got, rules) {
		t.Fatalf("augmented prompt must end with rules body; got %q", got)
	}
}

func TestStatusChip(t *testing.T) {
	cases := []struct {
		in   Source
		want string
	}{
		{SourceNone, ""},
		{SourceNookrules, "rules:nookrules"},
		{SourceCursorrules, "rules:cursorrules"},
	}
	for _, tc := range cases {
		if got := StatusChip(tc.in); got != tc.want {
			t.Errorf("StatusChip(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSourceFilename(t *testing.T) {
	cases := []struct {
		in   Source
		want string
	}{
		{SourceNone, ""},
		{SourceNookrules, ".nookrules"},
		{SourceCursorrules, ".cursorrules"},
	}
	for _, tc := range cases {
		if got := tc.in.Filename(); got != tc.want {
			t.Errorf("%q.Filename() = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func mustWrite(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
