package gitgutter

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParse_PureAddition(t *testing.T) {
	t.Parallel()
	diff := []byte("@@ -10,0 +11,3 @@\n+a\n+b\n+c\n")
	got := Parse(diff)
	want := map[int]Marker{10: Added, 11: Added, 12: Added}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse() = %v, want %v", got, want)
	}
}

func TestParse_SingleLineAdditionOmittedCount(t *testing.T) {
	t.Parallel()
	// "+11" without a count means count=1.
	diff := []byte("@@ -10,0 +11 @@\n+a\n")
	got := Parse(diff)
	want := map[int]Marker{10: Added}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse() = %v, want %v", got, want)
	}
}

func TestParse_PureDeletion(t *testing.T) {
	t.Parallel()
	// Deletion of 3 old lines; new file has 0 lines at position 9 (1-based)
	// meaning the deletion happened after line 9 in the new file. The marker
	// goes on the row IMMEDIATELY BELOW the deletion: row 9 (0-based) = line
	// 10 (1-based).
	diff := []byte("@@ -10,3 +9,0 @@\n-a\n-b\n-c\n")
	got := Parse(diff)
	want := map[int]Marker{9: DeletedAbove}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse() = %v, want %v", got, want)
	}
}

func TestParse_TopOfFileDeletion(t *testing.T) {
	t.Parallel()
	// Deletion at the very top of the file: newStart=0 (deletion conceptually
	// before line 1 in the new file). Marker on row 0.
	diff := []byte("@@ -1,3 +0,0 @@\n-a\n-b\n-c\n")
	got := Parse(diff)
	want := map[int]Marker{0: DeletedAbove}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse() = %v, want %v", got, want)
	}
}

func TestParse_Modification(t *testing.T) {
	t.Parallel()
	diff := []byte("@@ -10,2 +10,2 @@\n-a\n-b\n+A\n+B\n")
	got := Parse(diff)
	want := map[int]Marker{9: Modified, 10: Modified}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse() = %v, want %v", got, want)
	}
}

func TestParse_MultipleHunks(t *testing.T) {
	t.Parallel()
	diff := []byte("@@ -1,0 +1,1 @@\n+top\n@@ -5,1 +6,1 @@\n-old\n+new\n@@ -10,2 +12,0 @@\n-x\n-y\n")
	got := Parse(diff)
	want := map[int]Marker{
		0:  Added,        // line 1 added
		5:  Modified,     // line 6 modified
		12: DeletedAbove, // deletion after line 12, paint line 13 (row 12)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse() = %v, want %v", got, want)
	}
}

func TestParse_HunkHeaderWithTrailingText(t *testing.T) {
	t.Parallel()
	// git includes the surrounding function name after the closing "@@".
	diff := []byte("@@ -10,1 +10,1 @@ func Foo() {\n-old\n+new\n")
	got := Parse(diff)
	want := map[int]Marker{9: Modified}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse() = %v, want %v", got, want)
	}
}

func TestParse_MalformedHeaderIgnored(t *testing.T) {
	t.Parallel()
	// Missing "+...", trailing "@@" — both should produce no markers.
	cases := [][]byte{
		[]byte("@@ -10,1 @@\n"),
		[]byte("@@@ -10,1 +11,1 @@@\n"),
		[]byte("not a hunk header\n"),
	}
	for i, diff := range cases {
		got := Parse(diff)
		if len(got) != 0 {
			t.Errorf("case %d: Parse() = %v, want empty", i, got)
		}
	}
}

func TestParse_EmptyInput(t *testing.T) {
	t.Parallel()
	got := Parse(nil)
	if len(got) != 0 {
		t.Errorf("Parse(nil) = %v, want empty", got)
	}
}

func TestMarker_String(t *testing.T) {
	t.Parallel()
	cases := []struct {
		m    Marker
		want string
	}{
		{None, "none"},
		{Added, "added"},
		{Modified, "modified"},
		{DeletedAbove, "deleted-above"},
	}
	for _, c := range cases {
		if got := c.m.String(); got != c.want {
			t.Errorf("Marker(%d).String() = %q, want %q", c.m, got, c.want)
		}
	}
}

// --- end-to-end tests against a real `git` binary ---

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

func TestCompute_CleanTracked(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	writeFile(t, repo, "a.txt", "alpha\nbravo\ncharlie\n")
	gitDo(t, repo, "add", "a.txt")
	gitDo(t, repo, "commit", "-q", "-m", "init")

	got, err := Compute(context.Background(), repo, filepath.Join(repo, "a.txt"))
	if err != nil {
		t.Fatalf("Compute err: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("clean file should have no markers; got %v", got)
	}
}

func TestCompute_AddedLine(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	writeFile(t, repo, "a.txt", "alpha\nbravo\ncharlie\n")
	gitDo(t, repo, "add", "a.txt")
	gitDo(t, repo, "commit", "-q", "-m", "init")

	// Insert a new line between "alpha" and "bravo".
	writeFile(t, repo, "a.txt", "alpha\ninserted\nbravo\ncharlie\n")

	got, err := Compute(context.Background(), repo, filepath.Join(repo, "a.txt"))
	if err != nil {
		t.Fatalf("Compute err: %v", err)
	}
	if m, ok := got[1]; !ok || m != Added {
		t.Errorf("expected Added at row 1, got %v (full map: %v)", m, got)
	}
}

func TestCompute_ModifiedLine(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	writeFile(t, repo, "a.txt", "alpha\nbravo\ncharlie\n")
	gitDo(t, repo, "add", "a.txt")
	gitDo(t, repo, "commit", "-q", "-m", "init")

	// Modify "bravo" in place.
	writeFile(t, repo, "a.txt", "alpha\nBRAVO\ncharlie\n")

	got, err := Compute(context.Background(), repo, filepath.Join(repo, "a.txt"))
	if err != nil {
		t.Fatalf("Compute err: %v", err)
	}
	if m, ok := got[1]; !ok || m != Modified {
		t.Errorf("expected Modified at row 1, got %v (full map: %v)", m, got)
	}
}

func TestCompute_DeletedLine(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	writeFile(t, repo, "a.txt", "alpha\nbravo\ncharlie\ndelta\n")
	gitDo(t, repo, "add", "a.txt")
	gitDo(t, repo, "commit", "-q", "-m", "init")

	// Delete "bravo".
	writeFile(t, repo, "a.txt", "alpha\ncharlie\ndelta\n")

	got, err := Compute(context.Background(), repo, filepath.Join(repo, "a.txt"))
	if err != nil {
		t.Fatalf("Compute err: %v", err)
	}
	// After deletion the working tree is "alpha\ncharlie\ndelta\n"; the
	// deletion sat between line 1 (alpha) and line 2 (charlie). The marker
	// should land on row 1 (charlie) as "deleted-above".
	if m, ok := got[1]; !ok || m != DeletedAbove {
		t.Errorf("expected DeletedAbove at row 1, got %v (full map: %v)", m, got)
	}
}

func TestCompute_Untracked(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	// Untracked file — every line should be Added.
	writeFile(t, repo, "fresh.txt", "first\nsecond\nthird\n")

	got, err := Compute(context.Background(), repo, filepath.Join(repo, "fresh.txt"))
	if err != nil {
		t.Fatalf("Compute err: %v", err)
	}
	for i := 0; i < 3; i++ {
		if got[i] != Added {
			t.Errorf("row %d: got %v, want Added (full map: %v)", i, got[i], got)
		}
	}
}

func TestCompute_OutsideRepo(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	writeFile(t, tmp, "loose.txt", "no repo here\n")

	got, err := Compute(context.Background(), tmp, filepath.Join(tmp, "loose.txt"))
	// Either err is non-nil or markers is empty; the important behavior is
	// that the host treats this as "no markers" without panicking.
	if err == nil && len(got) > 0 {
		t.Errorf("expected empty markers outside a repo; got %v", got)
	}
}

func TestCompute_EmptyArgs(t *testing.T) {
	t.Parallel()
	if _, err := Compute(context.Background(), "", "/some/path"); err == nil {
		t.Error("expected error for empty root")
	}
	if _, err := Compute(context.Background(), "/some/root", ""); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestMarkerCmd_ReturnsMatchingPath(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	writeFile(t, repo, "a.txt", "x\n")
	gitDo(t, repo, "add", "a.txt")
	gitDo(t, repo, "commit", "-q", "-m", "init")

	abs := filepath.Join(repo, "a.txt")
	msg := MarkerCmd(repo, abs)()
	mm, ok := msg.(MarkersMsg)
	if !ok {
		t.Fatalf("MarkerCmd msg type = %T, want MarkersMsg", msg)
	}
	if mm.Path != abs {
		t.Errorf("MarkersMsg.Path = %q, want %q", mm.Path, abs)
	}
	if mm.Err != nil {
		t.Errorf("MarkersMsg.Err = %v, want nil", mm.Err)
	}
}
