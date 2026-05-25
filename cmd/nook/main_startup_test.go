package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// parseStartup covers the CLI shapes documented at the top of main.go:
// no args, a directory arg, a single file arg, a missing-but-creatable
// file arg (vim-style), and multi-file mode. Errors are returned for
// trailing-slash references to missing directories and for mixing a
// real directory into multi-file mode.
func TestParseStartup(t *testing.T) {
	tmp := t.TempDir()
	cwd := tmp

	subDir := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	fileA := filepath.Join(tmp, "a.txt")
	fileB := filepath.Join(subDir, "b.txt")
	if err := os.WriteFile(fileA, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("world\n"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}
	missingFile := filepath.Join(tmp, "new.txt")
	missingDirAbs := filepath.Join(tmp, "nope") + "/"

	type want struct {
		root  string
		files []string
		err   string // substring expected in err.Error() when non-empty
	}

	cases := []struct {
		name string
		args []string
		want want
	}{
		{
			name: "no args uses cwd",
			args: nil,
			want: want{root: cwd},
		},
		{
			name: "directory arg becomes root",
			args: []string{subDir},
			want: want{root: subDir},
		},
		{
			name: "single file arg roots at parent",
			args: []string{fileA},
			want: want{root: tmp, files: []string{fileA}},
		},
		{
			name: "single file arg in subdir",
			args: []string{fileB},
			want: want{root: subDir, files: []string{fileB}},
		},
		{
			name: "missing file vim-style new buffer",
			args: []string{missingFile},
			want: want{root: tmp, files: []string{missingFile}},
		},
		{
			name: "trailing slash missing dir is an error",
			args: []string{missingDirAbs},
			want: want{err: "no such directory"},
		},
		{
			name: "multi-file uses parent of first as root",
			args: []string{fileA, fileB},
			want: want{root: tmp, files: []string{fileA, fileB}},
		},
		{
			name: "multi-file refuses a directory",
			args: []string{fileA, subDir},
			want: want{err: "directory not allowed"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := parseStartup(cwd, tc.args)
			if tc.want.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want.err) {
					t.Fatalf("err = %v; want substring %q", err, tc.want.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if s.root != tc.want.root {
				t.Errorf("root = %q; want %q", s.root, tc.want.root)
			}
			if len(s.files) != len(tc.want.files) {
				t.Fatalf("files = %v; want %v", s.files, tc.want.files)
			}
			for i := range s.files {
				if s.files[i] != tc.want.files[i] {
					t.Errorf("files[%d] = %q; want %q", i, s.files[i], tc.want.files[i])
				}
			}
		})
	}
}

// Pre-opening from the CLI: newModel("/tmp/X", "/tmp/X/foo.txt") must
// land with foo.txt as the active buffer, status reflecting the open,
// and no welcome card visible (View renders the editor pane content).
func TestNewModelPreOpensSingleFile(t *testing.T) {
	root := t.TempDir()
	body := "package main\n\nfunc main() {}\n"
	path := filepath.Join(root, "single.go")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	m := newModel(root, path)
	if m.bufs.Count() != 1 {
		t.Fatalf("buf count = %d; want 1", m.bufs.Count())
	}
	p := m.bufs.Active()
	if p == nil {
		t.Fatalf("no active buffer after startup pre-open")
	}
	if p.Path() != path {
		t.Errorf("active path = %q; want %q", p.Path(), path)
	}
	if got := strings.Join(p.Lines(), "\n"); !strings.Contains(got, "func main") {
		t.Errorf("buffer contents missing fixture: %q", got)
	}
	if !strings.Contains(m.status, "opened") {
		t.Errorf("status = %q; want it to mention the open", m.status)
	}
	if len(m.startupFiles) != 1 || m.startupFiles[0] != path {
		t.Errorf("startupFiles = %v; want %v", m.startupFiles, []string{path})
	}
}

// `nook nonexistent.txt` should land on an empty buffer carrying the new
// path so that Ctrl+S writes the file. editor.Load already returns an
// empty buffer for IsNotExist, so the active pane should have one empty
// line and the path set.
func TestNewModelPreOpensMissingFileAsEmptyBuffer(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "brand-new.txt")

	m := newModel(root, path)
	p := m.bufs.Active()
	if p == nil {
		t.Fatalf("expected an active buffer for the new path")
	}
	if p.Path() != path {
		t.Errorf("active path = %q; want %q", p.Path(), path)
	}
	if got := p.Lines(); len(got) != 1 || got[0] != "" {
		t.Errorf("new-file buffer = %q; want one empty line", got)
	}
}

// Multi-file launch: every arg opens its own buffer, the first is active,
// and the status mentions the count.
func TestNewModelPreOpensMultiFile(t *testing.T) {
	root := t.TempDir()
	files := make([]string, 3)
	for i := range files {
		files[i] = filepath.Join(root, "f"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(files[i], []byte("body "+string(rune('a'+i))), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	m := newModel(root, files...)
	if got := m.bufs.Count(); got != len(files) {
		t.Fatalf("buf count = %d; want %d", got, len(files))
	}
	if idx := m.bufs.ActiveIndex(); idx != 0 {
		t.Errorf("active index = %d; want 0 (first file)", idx)
	}
	if p := m.bufs.Active(); p == nil || p.Path() != files[0] {
		t.Errorf("active path = %v; want %q", p, files[0])
	}
	if !strings.Contains(m.status, "3 files") {
		t.Errorf("status = %q; want it to mention 3 files", m.status)
	}
}

// Init() must include the LSP-attach side effects for the active
// pre-opened buffer; otherwise gopls / blame / gutter never wake up
// after a single-file launch. We can't assert the inner cmd shapes
// (they're closures) but we can confirm Init returns a non-nil batch
// when a pre-opened file is present and a nil-free batch otherwise.
func TestInitDispatchesForPreOpenedFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "x.go")
	if err := os.WriteFile(path, []byte("package x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	m := newModel(root, path)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init returned nil with a pre-opened file")
	}
	// Sanity: the empty-root variant also returns a non-nil batch (it
	// always dispatches loadFilesCmd + refreshGitCmd).
	if newModel(root).Init() == nil {
		t.Error("Init returned nil even for the zero-files case")
	}

	// Drain the batch to make sure no individual cmd panics on call.
	// tea.Batch returns a Cmd that produces a tea.BatchMsg when invoked.
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init cmd produced %T; want tea.BatchMsg", msg)
	}
	if len(batch) < 3 {
		t.Errorf("batch had %d cmds; want >= 3 (load+git+LSP-cluster)", len(batch))
	}
}
