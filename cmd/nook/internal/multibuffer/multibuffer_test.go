package multibuffer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestParse_EmptyInput(t *testing.T) {
	t.Parallel()
	frags, err := Parse(nil, "")
	if err != nil {
		t.Fatalf("Parse(nil) err = %v", err)
	}
	if len(frags) != 0 {
		t.Errorf("Parse(nil) = %v, want empty", frags)
	}
}

func TestParse_SingleHunkPureAddition(t *testing.T) {
	t.Parallel()
	diff := []byte(`diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -10,0 +11,3 @@ func Foo() {
+	a := 1
+	b := 2
+	c := 3
`)
	frags, err := Parse(diff, "/repo")
	if err != nil {
		t.Fatalf("Parse err: %v", err)
	}
	if len(frags) != 1 {
		t.Fatalf("got %d fragments, want 1", len(frags))
	}
	f := frags[0]
	if f.Path != "/repo/foo.go" {
		t.Errorf("Path = %q, want /repo/foo.go", f.Path)
	}
	if f.StartLine != 11 || f.EndLine != 13 {
		t.Errorf("StartLine/EndLine = %d/%d, want 11/13", f.StartLine, f.EndLine)
	}
	if f.Suffix != "func Foo() {" {
		t.Errorf("Suffix = %q, want %q", f.Suffix, "func Foo() {")
	}
	if len(f.Lines) != 3 {
		t.Fatalf("len(Lines) = %d, want 3", len(f.Lines))
	}
	for i, ln := range f.Lines {
		if ln.Marker != Added {
			t.Errorf("Lines[%d].Marker = %v, want Added", i, ln.Marker)
		}
		if ln.FileLine != 11+i {
			t.Errorf("Lines[%d].FileLine = %d, want %d", i, ln.FileLine, 11+i)
		}
	}
}

func TestParse_MixedHunk(t *testing.T) {
	t.Parallel()
	diff := []byte(`diff --git a/x.go b/x.go
--- a/x.go
+++ b/x.go
@@ -5,5 +5,5 @@
 keep1
-old
+new
 keep2
 keep3
`)
	frags, err := Parse(diff, "")
	if err != nil {
		t.Fatalf("Parse err: %v", err)
	}
	if len(frags) != 1 {
		t.Fatalf("got %d fragments, want 1", len(frags))
	}
	want := []Line{
		{Marker: Context, FileLine: 5, Text: "keep1"},
		{Marker: Added, FileLine: 6, Text: "new"},
		{Marker: Context, FileLine: 7, Text: "keep2"},
		{Marker: Context, FileLine: 8, Text: "keep3"},
	}
	if len(frags[0].Lines) != len(want) {
		t.Fatalf("len(Lines) = %d, want %d (got %#v)", len(frags[0].Lines), len(want), frags[0].Lines)
	}
	for i, ln := range frags[0].Lines {
		if ln != want[i] {
			t.Errorf("Lines[%d] = %#v, want %#v", i, ln, want[i])
		}
	}
}

func TestParse_PureDeletionHunkSkipped(t *testing.T) {
	t.Parallel()
	// A pure-deletion hunk has nothing in the new file to anchor on; it
	// should produce no fragment at all.
	diff := []byte(`diff --git a/d.go b/d.go
--- a/d.go
+++ b/d.go
@@ -10,3 +9,0 @@
-a
-b
-c
`)
	frags, err := Parse(diff, "")
	if err != nil {
		t.Fatalf("Parse err: %v", err)
	}
	if len(frags) != 0 {
		t.Errorf("got %d fragments, want 0", len(frags))
	}
}

func TestParse_MultipleHunksOneFile(t *testing.T) {
	t.Parallel()
	diff := []byte(`diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,1 @@
-one
+ONE
@@ -10,1 +10,1 @@
-ten
+TEN
`)
	frags, err := Parse(diff, "")
	if err != nil {
		t.Fatalf("Parse err: %v", err)
	}
	if len(frags) != 2 {
		t.Fatalf("got %d fragments, want 2", len(frags))
	}
	if frags[0].StartLine != 1 || frags[0].EndLine != 1 {
		t.Errorf("frag[0] range = %d-%d, want 1-1", frags[0].StartLine, frags[0].EndLine)
	}
	if frags[1].StartLine != 10 || frags[1].EndLine != 10 {
		t.Errorf("frag[1] range = %d-%d, want 10-10", frags[1].StartLine, frags[1].EndLine)
	}
	if frags[0].Lines[0].Text != "ONE" || frags[1].Lines[0].Text != "TEN" {
		t.Errorf("contents wrong: %q / %q", frags[0].Lines[0].Text, frags[1].Lines[0].Text)
	}
}

func TestParse_MultipleFiles(t *testing.T) {
	t.Parallel()
	diff := []byte(`diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,1 @@
-a
+A
diff --git a/sub/b.go b/sub/b.go
--- a/sub/b.go
+++ b/sub/b.go
@@ -2,1 +2,1 @@
-b
+B
`)
	frags, err := Parse(diff, "/repo")
	if err != nil {
		t.Fatalf("Parse err: %v", err)
	}
	if len(frags) != 2 {
		t.Fatalf("got %d fragments, want 2", len(frags))
	}
	if frags[0].Path != "/repo/a.go" {
		t.Errorf("frag[0].Path = %q", frags[0].Path)
	}
	if frags[1].Path != "/repo/sub/b.go" {
		t.Errorf("frag[1].Path = %q", frags[1].Path)
	}
}

func TestParse_NoNewlineAtEofMetadataIgnored(t *testing.T) {
	t.Parallel()
	diff := []byte(`diff --git a/x b/x
--- a/x
+++ b/x
@@ -1,1 +1,2 @@
-one
+ONE
+two
\ No newline at end of file
`)
	frags, err := Parse(diff, "")
	if err != nil {
		t.Fatalf("Parse err: %v", err)
	}
	if len(frags) != 1 {
		t.Fatalf("got %d fragments, want 1", len(frags))
	}
	if len(frags[0].Lines) != 2 {
		t.Fatalf("len(Lines) = %d, want 2", len(frags[0].Lines))
	}
	if frags[0].Lines[0].Text != "ONE" || frags[0].Lines[1].Text != "two" {
		t.Errorf("Lines = %#v", frags[0].Lines)
	}
}

func TestParse_OmittedNewCountDefaultsToOne(t *testing.T) {
	t.Parallel()
	// "+5" with no count means count=1.
	diff := []byte(`diff --git a/x b/x
--- a/x
+++ b/x
@@ -5 +5 @@
-old
+new
`)
	frags, err := Parse(diff, "")
	if err != nil {
		t.Fatalf("Parse err: %v", err)
	}
	if len(frags) != 1 || len(frags[0].Lines) != 1 {
		t.Fatalf("want 1 frag with 1 line, got %#v", frags)
	}
	if frags[0].Lines[0].FileLine != 5 || frags[0].Lines[0].Text != "new" {
		t.Errorf("Lines[0] = %#v", frags[0].Lines[0])
	}
}

func TestParse_AbsolutePathPreserved(t *testing.T) {
	t.Parallel()
	diff := []byte(`diff --git a//abs/path b//abs/path
--- a//abs/path
+++ b//abs/path
@@ -1,1 +1,1 @@
-a
+A
`)
	frags, err := Parse(diff, "/repo")
	if err != nil {
		t.Fatalf("Parse err: %v", err)
	}
	if len(frags) != 1 {
		t.Fatalf("len(frags) = %d", len(frags))
	}
	if frags[0].Path != "/abs/path" {
		t.Errorf("Path = %q, want /abs/path", frags[0].Path)
	}
}

func TestBuildRows_EmptyAndSingleAndMulti(t *testing.T) {
	t.Parallel()
	// Empty.
	rows := buildRows(nil)
	if len(rows) != 0 {
		t.Errorf("buildRows(nil) = %v, want empty", rows)
	}

	// Single fragment: 1 header + N content rows, no separator.
	frags := []Fragment{
		{Path: "a", StartLine: 1, EndLine: 2, Lines: []Line{
			{Marker: Context, FileLine: 1, Text: "x"},
			{Marker: Added, FileLine: 2, Text: "y"},
		}},
	}
	rows = buildRows(frags)
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	if rows[0].kind != rowHeader || rows[1].kind != rowContent || rows[2].kind != rowContent {
		t.Errorf("kinds = %v %v %v", rows[0].kind, rows[1].kind, rows[2].kind)
	}

	// Multiple fragments: separator between each.
	frags = append(frags, Fragment{Path: "b", StartLine: 5, EndLine: 5, Lines: []Line{
		{Marker: Added, FileLine: 5, Text: "z"},
	}})
	rows = buildRows(frags)
	// expect: header, content, content, separator, header, content
	if len(rows) != 6 {
		t.Fatalf("len(rows) = %d, want 6", len(rows))
	}
	if rows[3].kind != rowSeparator {
		t.Errorf("rows[3].kind = %v, want rowSeparator", rows[3].kind)
	}
	if rows[4].kind != rowHeader || rows[4].fragIdx != 1 {
		t.Errorf("rows[4] = %+v, want header for frag 1", rows[4])
	}
}

func TestPane_SetFragmentsLandsOnFirstNonSeparator(t *testing.T) {
	t.Parallel()
	frags := []Fragment{{
		Path: "a", StartLine: 1, EndLine: 1,
		Lines: []Line{{Marker: Added, FileLine: 1, Text: "x"}},
	}}
	p := NewPane(theme.Default, "/repo").SetFragments(frags, nil)
	if p.cursor != 0 || p.rows[p.cursor].kind != rowHeader {
		t.Errorf("cursor lands on %v at row %d, want rowHeader at 0", p.rows[p.cursor].kind, p.cursor)
	}
}

func TestPane_MoveBySkipsSeparators(t *testing.T) {
	t.Parallel()
	// Two fragments. Between them is a separator at row 2 (header, content, separator, header, content).
	frags := []Fragment{
		{Path: "a", StartLine: 1, EndLine: 1, Lines: []Line{{FileLine: 1, Text: "x"}}},
		{Path: "b", StartLine: 5, EndLine: 5, Lines: []Line{{FileLine: 5, Text: "y"}}},
	}
	p := NewPane(theme.Default, "/repo").SetFragments(frags, nil)
	if got := p.rows[2].kind; got != rowSeparator {
		t.Fatalf("rows[2].kind = %v, want rowSeparator (precondition)", got)
	}
	// Cursor starts at 0 (header). Move +2 → should land on 3 (next header), skipping the separator at 2.
	p2 := p.moveBy(+2)
	if p2.cursor != 3 {
		t.Errorf("after moveBy(+2): cursor = %d, want 3 (skipped separator)", p2.cursor)
	}
	// Move back: -2 from row 4 should land at 1, skipping separator on the way.
	p3 := p2.moveBy(+1) // row 4
	p4 := p3.moveBy(-2) // expect row 1
	if p4.cursor != 1 {
		t.Errorf("after moveBy(-2) from %d: cursor = %d, want 1", p3.cursor, p4.cursor)
	}
}

func TestPane_MoveByClampsAtEnds(t *testing.T) {
	t.Parallel()
	frags := []Fragment{{Path: "a", StartLine: 1, EndLine: 1, Lines: []Line{{FileLine: 1, Text: "x"}}}}
	p := NewPane(theme.Default, "/repo").SetFragments(frags, nil)
	// Two rows total. Move -10 should clamp to 0.
	if got := p.moveBy(-10).cursor; got != 0 {
		t.Errorf("moveBy(-10) cursor = %d, want 0", got)
	}
	if got := p.moveBy(+10).cursor; got != 1 {
		t.Errorf("moveBy(+10) cursor = %d, want 1 (last row)", got)
	}
}

func TestPane_SelectedResolvesByKind(t *testing.T) {
	t.Parallel()
	frags := []Fragment{{
		Path: "/repo/a.go", StartLine: 10, EndLine: 11,
		Lines: []Line{
			{Marker: Context, FileLine: 10, Text: "ten"},
			{Marker: Added, FileLine: 11, Text: "eleven"},
		},
	}}
	p := NewPane(theme.Default, "/repo").SetFragments(frags, nil)

	// Cursor on header → (path, StartLine)
	path, line, ok := p.Selected()
	if !ok || path != "/repo/a.go" || line != 10 {
		t.Errorf("header Selected = (%q,%d,%v), want (/repo/a.go,10,true)", path, line, ok)
	}

	// Move to content row 1 (FileLine 10).
	p = p.moveBy(+1)
	path, line, ok = p.Selected()
	if !ok || path != "/repo/a.go" || line != 10 {
		t.Errorf("content[0] Selected = (%q,%d,%v), want (/repo/a.go,10,true)", path, line, ok)
	}

	// Move to content row 2 (FileLine 11).
	p = p.moveBy(+1)
	path, line, ok = p.Selected()
	if !ok || path != "/repo/a.go" || line != 11 {
		t.Errorf("content[1] Selected = (%q,%d,%v), want (/repo/a.go,11,true)", path, line, ok)
	}
}

func TestPane_SelectedOutOfRange(t *testing.T) {
	t.Parallel()
	p := NewPane(theme.Default, "")
	if _, _, ok := p.Selected(); ok {
		t.Error("Selected on empty pane should be false")
	}
}

func TestPane_UpdateEscEmitsCancel(t *testing.T) {
	t.Parallel()
	p := NewPane(theme.Default, "")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc returned nil cmd")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Errorf("Esc cmd produced %T, want CancelMsg", cmd())
	}
}

func TestPane_UpdateEnterEmitsOpenAt(t *testing.T) {
	t.Parallel()
	frags := []Fragment{{
		Path: "/x/y.go", StartLine: 42, EndLine: 42,
		Lines: []Line{{Marker: Added, FileLine: 42, Text: "answer"}},
	}}
	p := NewPane(theme.Default, "/x").SetFragments(frags, nil)
	// Move to the content row (FileLine 42).
	p = p.moveBy(+1)
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter returned nil cmd")
	}
	msg, ok := cmd().(OpenAtMsg)
	if !ok {
		t.Fatalf("Enter cmd produced %T, want OpenAtMsg", cmd())
	}
	if msg.Path != "/x/y.go" || msg.Line != 42 {
		t.Errorf("OpenAtMsg = %+v, want {Path:/x/y.go Line:42}", msg)
	}
}

func TestPane_UpdateArrowsMoveCursor(t *testing.T) {
	t.Parallel()
	frags := []Fragment{{
		Path: "a", StartLine: 1, EndLine: 2,
		Lines: []Line{
			{Marker: Context, FileLine: 1, Text: "x"},
			{Marker: Context, FileLine: 2, Text: "y"},
		},
	}}
	p := NewPane(theme.Default, "").SetFragments(frags, nil)
	if p.cursor != 0 {
		t.Fatalf("precondition: cursor = %d, want 0", p.cursor)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.cursor != 1 {
		t.Errorf("after Down: cursor = %d, want 1", p.cursor)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.cursor != 0 {
		t.Errorf("after Up: cursor = %d, want 0", p.cursor)
	}
}

func TestPane_UpdateHomeEnd(t *testing.T) {
	t.Parallel()
	frags := []Fragment{
		{Path: "a", StartLine: 1, EndLine: 1, Lines: []Line{{FileLine: 1, Text: "x"}}},
		{Path: "b", StartLine: 5, EndLine: 5, Lines: []Line{{FileLine: 5, Text: "y"}}},
	}
	p := NewPane(theme.Default, "").SetFragments(frags, nil)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if p.cursor != len(p.rows)-1 {
		t.Errorf("after End: cursor = %d, want %d", p.cursor, len(p.rows)-1)
	}
	if got := p.rows[p.cursor].kind; got == rowSeparator {
		t.Errorf("End landed on separator (kind=%v)", got)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyHome})
	if p.cursor != 0 {
		t.Errorf("after Home: cursor = %d, want 0", p.cursor)
	}
}

func TestPane_UpdateIgnoresNonKeyMsg(t *testing.T) {
	t.Parallel()
	p := NewPane(theme.Default, "")
	p2, cmd := p.Update("not a key")
	if cmd != nil {
		t.Errorf("non-KeyMsg returned cmd = %v", cmd)
	}
	if p2.cursor != p.cursor {
		t.Errorf("non-KeyMsg moved cursor")
	}
}

func TestPane_FocusBlur(t *testing.T) {
	t.Parallel()
	p := NewPane(theme.Default, "")
	if p.Focused() {
		t.Error("pane focused by default")
	}
	p = p.Focus()
	if !p.Focused() {
		t.Error("Focus() did not set focused")
	}
	p = p.Blur()
	if p.Focused() {
		t.Error("Blur() did not clear focused")
	}
}

func TestPane_ResetClearsState(t *testing.T) {
	t.Parallel()
	frags := []Fragment{{Path: "a", StartLine: 1, EndLine: 1, Lines: []Line{{FileLine: 1, Text: "x"}}}}
	p := NewPane(theme.Default, "").SetFragments(frags, nil)
	p = p.Reset("loading…")
	if p.Title() != "loading…" {
		t.Errorf("Title = %q", p.Title())
	}
	if len(p.Fragments()) != 0 || len(p.rows) != 0 {
		t.Error("Reset did not clear fragments/rows")
	}
	if p.Err() != nil {
		t.Error("Reset did not clear err")
	}
}

func TestPane_ViewEmpty(t *testing.T) {
	t.Parallel()
	p := NewPane(theme.Default, "/repo").Reset("uncommitted changes").WithSize(40, 10)
	out := p.View()
	if !strings.Contains(out, "uncommitted") {
		t.Errorf("View missing title: %q", out)
	}
	if !strings.Contains(out, "no fragments") {
		t.Errorf("View missing empty hint: %q", out)
	}
}

func TestPane_ViewWithFragmentsRendersHeaderAndContent(t *testing.T) {
	t.Parallel()
	frags := []Fragment{{
		Path: "/repo/a.go", StartLine: 1, EndLine: 1,
		Lines:  []Line{{Marker: Added, FileLine: 1, Text: "hello"}},
		Suffix: "func main",
	}}
	p := NewPane(theme.Default, "/repo").WithSize(60, 10).Focus().SetFragments(frags, nil)
	out := p.View()
	if !strings.Contains(out, "a.go") {
		t.Errorf("View missing path: %q", out)
	}
	if !strings.Contains(out, "1-1") {
		t.Errorf("View missing line range: %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("View missing line text: %q", out)
	}
	if !strings.Contains(out, "func main") {
		t.Errorf("View missing suffix: %q", out)
	}
}

func TestPane_ViewError(t *testing.T) {
	t.Parallel()
	p := NewPane(theme.Default, "").WithSize(40, 6).SetFragments(nil, exec.ErrNotFound)
	out := p.View()
	if !strings.Contains(out, "error") {
		t.Errorf("View missing error label: %q", out)
	}
}

// --- end-to-end against a real git binary ---

func newTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	cmds := [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"config", "commit.gpgsign", "false"},
	}
	for _, args := range cmds {
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	return dir
}

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func gitDo(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestLoadDiffCmd_NoChanges(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	writeFile(t, repo, "a.go", "package x\n")
	gitDo(t, repo, "add", "a.go")
	gitDo(t, repo, "commit", "-q", "-m", "init")

	msg := LoadDiffCmd(repo, "HEAD")()
	fm, ok := msg.(FragmentsMsg)
	if !ok {
		t.Fatalf("LoadDiffCmd returned %T, want FragmentsMsg", msg)
	}
	if fm.Err != nil {
		t.Fatalf("FragmentsMsg.Err = %v", fm.Err)
	}
	if len(fm.Fragments) != 0 {
		t.Errorf("clean repo returned %d fragments, want 0", len(fm.Fragments))
	}
}

func TestLoadDiffCmd_WorkingTreeChange(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	writeFile(t, repo, "a.go", "package x\n\nfunc Foo() int { return 1 }\n")
	gitDo(t, repo, "add", "a.go")
	gitDo(t, repo, "commit", "-q", "-m", "init")

	// Modify the function body.
	writeFile(t, repo, "a.go", "package x\n\nfunc Foo() int { return 42 }\n")

	msg := LoadDiffCmd(repo, "HEAD")()
	fm := msg.(FragmentsMsg)
	if fm.Err != nil {
		t.Fatalf("err: %v", fm.Err)
	}
	if len(fm.Fragments) == 0 {
		t.Fatal("expected at least one fragment")
	}
	// Resolve symlinks because /tmp may be symlinked (e.g. /private/tmp on
	// macOS, /var/private/folders elsewhere). git resolves to the symlink
	// target; t.TempDir gives us the alias.
	wantAbs, _ := filepath.EvalSymlinks(filepath.Join(repo, "a.go"))
	gotAbs, _ := filepath.EvalSymlinks(fm.Fragments[0].Path)
	if gotAbs != wantAbs {
		t.Errorf("frag.Path = %q, want %q", gotAbs, wantAbs)
	}
}

func TestLoadDiffCmd_BaseRevPassed(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	writeFile(t, repo, "a.go", "alpha\n")
	gitDo(t, repo, "add", "a.go")
	gitDo(t, repo, "commit", "-q", "-m", "init")

	// Stage a change. Working-tree diff (base="") sees nothing; HEAD diff sees it.
	writeFile(t, repo, "a.go", "ALPHA\n")
	gitDo(t, repo, "add", "a.go")

	// base="" → staged-only changes are excluded (diff is working vs index, both equal).
	if m := LoadDiffCmd(repo, "")().(FragmentsMsg); len(m.Fragments) != 0 {
		t.Errorf("base=\"\" returned %d fragments, want 0 (staged-but-not-touched-since)", len(m.Fragments))
	}
	// base="HEAD" → staged change is visible.
	if m := LoadDiffCmd(repo, "HEAD")().(FragmentsMsg); len(m.Fragments) != 1 {
		t.Errorf("base=HEAD returned %d fragments, want 1", len(m.Fragments))
	}
}

func TestLoadDiffCmd_OutsideRepo(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	msg := LoadDiffCmd(tmp, "HEAD")()
	fm := msg.(FragmentsMsg)
	if fm.Err == nil {
		t.Error("expected error running diff outside a repo")
	}
}

func TestRelativize(t *testing.T) {
	t.Parallel()
	if got := relativize("/repo/a.go", "/repo"); got != "a.go" {
		t.Errorf("relativize(/repo/a.go, /repo) = %q, want a.go", got)
	}
	if got := relativize("/elsewhere/x", "/repo"); got != "/elsewhere/x" {
		t.Errorf("relativize escapes repo, want absolute pass-through, got %q", got)
	}
	if got := relativize("/abs", ""); got != "/abs" {
		t.Errorf("empty root should pass through, got %q", got)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    string
		w    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 6, "hello…"},
		{"hi", 0, ""},
		{"hi", 1, "h"},
	}
	for _, c := range cases {
		if got := truncate(c.s, c.w); got != c.want {
			t.Errorf("truncate(%q,%d) = %q, want %q", c.s, c.w, got, c.want)
		}
	}
}

func TestStripANSI(t *testing.T) {
	t.Parallel()
	in := "\x1b[31mred\x1b[0m and \x1b[1;33;44mbold\x1b[0m"
	if got := stripANSI(in); got != "red and bold" {
		t.Errorf("stripANSI = %q, want %q", got, "red and bold")
	}
}

func TestParseHunkHeader_OmittedCounts(t *testing.T) {
	t.Parallel()
	// Both old and new counts omitted — defaults to 1.
	ns, nc, suf, ok := parseHunkHeader([]byte("@@ -5 +6 @@"))
	if !ok || ns != 6 || nc != 1 || suf != "" {
		t.Errorf("parseHunkHeader omitted = (%d,%d,%q,%v), want (6,1,\"\",true)", ns, nc, suf, ok)
	}
}

func TestParseHunkHeader_SuffixCaptured(t *testing.T) {
	t.Parallel()
	_, _, suf, ok := parseHunkHeader([]byte("@@ -1,3 +1,3 @@ func main() {"))
	if !ok || strings.TrimSpace(suf) != "func main() {" {
		t.Errorf("suffix = %q, ok=%v", suf, ok)
	}
}

func TestParseHunkHeader_Malformed(t *testing.T) {
	t.Parallel()
	cases := [][]byte{
		[]byte("not a hunk"),
		[]byte("@@ malformed @@"),
		[]byte("@@ -1,3 -1,3 @@"),
		[]byte("@@ -1,3 +abc @@"),
	}
	for _, c := range cases {
		if _, _, _, ok := parseHunkHeader(c); ok {
			t.Errorf("parseHunkHeader(%q) ok=true, want false", c)
		}
	}
}

func TestLoadDiff_ContextCanceled(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	// canceled context shouldn't panic the cmd
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = ctx // not threaded into LoadDiffCmd today; this test reserves the slot.
	msg := LoadDiffCmd(repo, "")()
	if _, ok := msg.(FragmentsMsg); !ok {
		t.Errorf("got %T, want FragmentsMsg", msg)
	}
}

// threeFragmentPane builds a pane with three two-line fragments. The
// resulting rows are: header(0) content(1) content(2) sep(3) header(4)
// content(5) content(6) sep(7) header(8) content(9) content(10), so the
// fragment headers sit at rows 0, 4, and 8.
func threeFragmentPane(t *testing.T) Pane {
	t.Helper()
	frags := []Fragment{
		{Path: "a.go", StartLine: 1, EndLine: 2, Lines: []Line{
			{Marker: Context, FileLine: 1, Text: "a1"},
			{Marker: Added, FileLine: 2, Text: "a2"},
		}},
		{Path: "b.go", StartLine: 10, EndLine: 11, Lines: []Line{
			{Marker: Context, FileLine: 10, Text: "b1"},
			{Marker: Added, FileLine: 11, Text: "b2"},
		}},
		{Path: "c.go", StartLine: 20, EndLine: 21, Lines: []Line{
			{Marker: Context, FileLine: 20, Text: "c1"},
			{Marker: Added, FileLine: 21, Text: "c2"},
		}},
	}
	p := NewPane(theme.Default, "/repo").WithSize(80, 20).SetFragments(frags, nil)
	if p.rows[0].kind != rowHeader || p.rows[4].kind != rowHeader || p.rows[8].kind != rowHeader {
		t.Fatalf("precondition: headers expected at 0/4/8, got kinds %v/%v/%v",
			p.rows[0].kind, p.rows[4].kind, p.rows[8].kind)
	}
	return p
}

func TestPane_JumpFragmentForwardWalksHeaders(t *testing.T) {
	t.Parallel()
	p := threeFragmentPane(t) // cursor at header 0
	p = p.jumpFragment(+1)
	if p.cursor != 4 {
		t.Fatalf("first ]: cursor = %d, want 4", p.cursor)
	}
	p = p.jumpFragment(+1)
	if p.cursor != 8 {
		t.Fatalf("second ]: cursor = %d, want 8", p.cursor)
	}
	// No header past the last one: cursor stays.
	p = p.jumpFragment(+1)
	if p.cursor != 8 {
		t.Errorf("] past last header should stay at 8, got %d", p.cursor)
	}
}

func TestPane_JumpFragmentBackwardWalksHeaders(t *testing.T) {
	t.Parallel()
	p := threeFragmentPane(t)
	p.cursor = 8 // last header
	p = p.jumpFragment(-1)
	if p.cursor != 4 {
		t.Fatalf("first [: cursor = %d, want 4", p.cursor)
	}
	p = p.jumpFragment(-1)
	if p.cursor != 0 {
		t.Fatalf("second [: cursor = %d, want 0", p.cursor)
	}
	// No header before the first: cursor stays.
	p = p.jumpFragment(-1)
	if p.cursor != 0 {
		t.Errorf("[ before first header should stay at 0, got %d", p.cursor)
	}
}

func TestPane_JumpFragmentFromContentRow(t *testing.T) {
	t.Parallel()
	p := threeFragmentPane(t)
	// From a content row inside frag 1 (row 5), forward jumps to the next
	// section header (8), backward jumps to this section's own header (4) —
	// the [[ / ]] asymmetry.
	p.cursor = 5
	if got := p.jumpFragment(+1).cursor; got != 8 {
		t.Errorf("] from content row 5: cursor = %d, want 8", got)
	}
	if got := p.jumpFragment(-1).cursor; got != 4 {
		t.Errorf("[ from content row 5: cursor = %d, want 4 (current section head)", got)
	}
}

func TestPane_UpdateBracketKeysJumpFragments(t *testing.T) {
	t.Parallel()
	p := threeFragmentPane(t).Focus()
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if p.cursor != 4 {
		t.Errorf("] key: cursor = %d, want 4", p.cursor)
	}
	if cmd != nil {
		t.Error("] key should not emit a cmd")
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if p.cursor != 0 {
		t.Errorf("[ key: cursor = %d, want 0", p.cursor)
	}
}

func TestPane_UpdateUnknownRuneIsNoOp(t *testing.T) {
	t.Parallel()
	p := threeFragmentPane(t).Focus()
	start := p.cursor
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if p.cursor != start {
		t.Errorf("unknown rune moved cursor to %d, want %d", p.cursor, start)
	}
	if cmd != nil {
		t.Error("unknown rune should not emit a cmd")
	}
}

func TestPane_JumpFragmentEmptyIsNoOp(t *testing.T) {
	t.Parallel()
	p := NewPane(theme.Default, "/repo")
	if got := p.jumpFragment(+1).cursor; got != 0 {
		t.Errorf("jumpFragment on empty pane moved cursor to %d, want 0", got)
	}
}

// changeBlockPane builds a single-fragment pane whose rows are, by index:
//
//	0 header
//	1 context (a1)
//	2 added   (a2)  ─ first edit, a two-line block
//	3 added   (a3)  ─┘
//	4 context (a4)
//	5 added   (a5)  ─ second, distinct edit
//
// so jumpChange has both a contiguous changed run and a context gap to
// step across.
func changeBlockPane(t *testing.T) Pane {
	t.Helper()
	frags := []Fragment{{Path: "a.go", StartLine: 1, EndLine: 5, Lines: []Line{
		{Marker: Context, FileLine: 1, Text: "a1"},
		{Marker: Added, FileLine: 2, Text: "a2"},
		{Marker: Added, FileLine: 3, Text: "a3"},
		{Marker: Context, FileLine: 4, Text: "a4"},
		{Marker: Added, FileLine: 5, Text: "a5"},
	}}}
	p := NewPane(theme.Default, "/repo").WithSize(80, 20).SetFragments(frags, nil)
	if p.rows[0].kind != rowHeader {
		t.Fatalf("precondition: header expected at row 0, got %v", p.rows[0].kind)
	}
	if !p.isChangeRow(2) || !p.isChangeRow(3) || !p.isChangeRow(5) {
		t.Fatalf("precondition: rows 2/3/5 should be change rows")
	}
	if p.isChangeRow(1) || p.isChangeRow(4) {
		t.Fatalf("precondition: rows 1/4 should be context rows")
	}
	return p
}

func TestPane_JumpChangeForwardStepsBetweenEdits(t *testing.T) {
	t.Parallel()
	p := changeBlockPane(t) // cursor at header 0
	p = p.jumpChange(+1)
	if p.cursor != 2 {
		t.Fatalf("first }: cursor = %d, want 2 (first added line)", p.cursor)
	}
	// From inside the two-line block, the next } clears the whole block
	// and lands on the next distinct edit, not row 3.
	p = p.jumpChange(+1)
	if p.cursor != 5 {
		t.Fatalf("second }: cursor = %d, want 5 (next distinct edit)", p.cursor)
	}
	// No change past the last: cursor stays.
	p = p.jumpChange(+1)
	if p.cursor != 5 {
		t.Errorf("} past last change should stay at 5, got %d", p.cursor)
	}
}

func TestPane_JumpChangeBackwardStepsBetweenEdits(t *testing.T) {
	t.Parallel()
	p := changeBlockPane(t)
	p.cursor = 5
	p = p.jumpChange(-1)
	if p.cursor != 3 {
		t.Fatalf("first {: cursor = %d, want 3 (edge of prior block)", p.cursor)
	}
	// No change before this block: cursor stays.
	p = p.jumpChange(-1)
	if p.cursor != 3 {
		t.Errorf("{ before first change should stay at 3, got %d", p.cursor)
	}
}

func TestPane_JumpChangeNeverLandsOnContext(t *testing.T) {
	t.Parallel()
	p := changeBlockPane(t)
	for i := 0; i < 6; i++ {
		p = p.jumpChange(+1)
		if p.isChangeRow(p.cursor) == false {
			t.Fatalf("jumpChange landed on non-change row %d", p.cursor)
		}
	}
}

func TestPane_UpdateBraceKeysJumpChange(t *testing.T) {
	t.Parallel()
	p := changeBlockPane(t).Focus()
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
	if p.cursor != 2 {
		t.Errorf("} key: cursor = %d, want 2", p.cursor)
	}
	if cmd != nil {
		t.Error("} key should not emit a cmd")
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'{'}})
	if p.cursor != 2 {
		t.Errorf("{ key from row 2 (only block above is its own): cursor = %d, want 2", p.cursor)
	}
}

func TestPane_JumpChangeEmptyIsNoOp(t *testing.T) {
	t.Parallel()
	p := NewPane(theme.Default, "/repo")
	if got := p.jumpChange(+1).cursor; got != 0 {
		t.Errorf("jumpChange on empty pane moved cursor to %d, want 0", got)
	}
}
