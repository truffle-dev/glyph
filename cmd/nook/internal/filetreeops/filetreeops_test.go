package filetreeops

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreatePathFile(t *testing.T) {
	root := t.TempDir()
	res, err := CreatePath(root, "hello.txt")
	if err != nil {
		t.Fatalf("CreatePath: %v", err)
	}
	if res.IsDir {
		t.Fatalf("expected file, got dir: %+v", res)
	}
	if res.Path != filepath.Join(root, "hello.txt") {
		t.Errorf("path mismatch: %q", res.Path)
	}
	info, err := os.Stat(res.Path)
	if err != nil {
		t.Fatalf("stat created file: %v", err)
	}
	if info.IsDir() {
		t.Errorf("expected regular file, stat says dir")
	}
	if info.Size() != 0 {
		t.Errorf("expected empty file, size=%d", info.Size())
	}
}

func TestCreatePathDirTrailingSlash(t *testing.T) {
	root := t.TempDir()
	res, err := CreatePath(root, "newdir/")
	if err != nil {
		t.Fatalf("CreatePath: %v", err)
	}
	if !res.IsDir {
		t.Fatalf("expected dir, got file: %+v", res)
	}
	if res.Path != filepath.Join(root, "newdir") {
		t.Errorf("path mismatch: %q", res.Path)
	}
	info, err := os.Stat(res.Path)
	if err != nil {
		t.Fatalf("stat created dir: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected dir, stat says file")
	}
}

func TestCreatePathNestedFile(t *testing.T) {
	root := t.TempDir()
	res, err := CreatePath(root, "a/b/c/leaf.go")
	if err != nil {
		t.Fatalf("CreatePath: %v", err)
	}
	if res.IsDir {
		t.Fatalf("expected file, got dir")
	}
	if _, err := os.Stat(filepath.Join(root, "a", "b", "c")); err != nil {
		t.Fatalf("nested parent dir missing: %v", err)
	}
	if _, err := os.Stat(res.Path); err != nil {
		t.Fatalf("leaf file missing: %v", err)
	}
}

func TestCreatePathNestedDir(t *testing.T) {
	root := t.TempDir()
	res, err := CreatePath(root, "x/y/z/")
	if err != nil {
		t.Fatalf("CreatePath: %v", err)
	}
	if !res.IsDir {
		t.Fatalf("expected dir")
	}
	info, err := os.Stat(res.Path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("created path is not a dir")
	}
}

func TestCreatePathCollisionReturnsErrPathExists(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "already.txt")
	if err := os.WriteFile(existing, []byte("hi"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := CreatePath(root, "already.txt")
	if !errors.Is(err, ErrPathExists) {
		t.Fatalf("expected ErrPathExists, got %v", err)
	}
	body, _ := os.ReadFile(existing)
	if string(body) != "hi" {
		t.Errorf("collision branch overwrote existing file")
	}
}

func TestCreatePathCollisionWithDir(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "subdir"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := CreatePath(root, "subdir/")
	if !errors.Is(err, ErrPathExists) {
		t.Fatalf("expected ErrPathExists, got %v", err)
	}
}

func TestCreatePathEmptyName(t *testing.T) {
	root := t.TempDir()
	if _, err := CreatePath(root, ""); !errors.Is(err, ErrEmptyName) {
		t.Errorf("empty input: expected ErrEmptyName, got %v", err)
	}
	if _, err := CreatePath(root, "   "); !errors.Is(err, ErrEmptyName) {
		t.Errorf("whitespace input: expected ErrEmptyName, got %v", err)
	}
	if _, err := CreatePath(root, "/"); !errors.Is(err, ErrAbsolutePath) {
		t.Errorf("bare slash: expected ErrAbsolutePath, got %v", err)
	}
}

func TestCreatePathAbsoluteRefused(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(root, "victim.txt")
	if _, err := CreatePath(root, abs); !errors.Is(err, ErrAbsolutePath) {
		t.Fatalf("absolute path: expected ErrAbsolutePath, got %v", err)
	}
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Errorf("absolute branch wrote anyway: %v", err)
	}
}

func TestCreatePathDotDotRefused(t *testing.T) {
	root := t.TempDir()
	if _, err := CreatePath(root, "../escape.txt"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("../foo: expected ErrInvalidName, got %v", err)
	}
	if _, err := CreatePath(root, "foo/../bar"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("foo/../bar: expected ErrInvalidName, got %v", err)
	}
	if _, err := CreatePath(root, ".."); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("..: expected ErrInvalidName, got %v", err)
	}
}

func TestCreatePathTrimsLeadingTrailingWhitespace(t *testing.T) {
	root := t.TempDir()
	res, err := CreatePath(root, "   spaced.txt   ")
	if err != nil {
		t.Fatalf("CreatePath: %v", err)
	}
	if filepath.Base(res.Path) != "spaced.txt" {
		t.Errorf("expected trimmed name spaced.txt, got %q", res.Path)
	}
}

func TestCreatePathNestedTrailingSlash(t *testing.T) {
	root := t.TempDir()
	res, err := CreatePath(root, "deep/nested/")
	if err != nil {
		t.Fatalf("CreatePath: %v", err)
	}
	if !res.IsDir {
		t.Errorf("expected dir, got file")
	}
	info, err := os.Stat(res.Path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("created path is not a dir")
	}
	if _, err := os.Stat(filepath.Join(root, "deep")); err != nil {
		t.Errorf("intermediate dir missing")
	}
}

func TestCreatePathRelativeWithDotPrefixOK(t *testing.T) {
	root := t.TempDir()
	res, err := CreatePath(root, ".env")
	if err != nil {
		t.Fatalf("dotfile create: %v", err)
	}
	if filepath.Base(res.Path) != ".env" {
		t.Errorf("expected .env, got %q", res.Path)
	}
}

func TestCreatePathDoubleSlashCollapsed(t *testing.T) {
	root := t.TempDir()
	if _, err := CreatePath(root, "a//b.go"); err != nil {
		t.Fatalf("CreatePath: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "a", "b.go")); err != nil {
		t.Errorf("expected a/b.go to be created, got %v", err)
	}
}
