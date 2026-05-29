// Package filetreeops handles file-system mutations triggered from the
// file-tree pane. The current surface is create-only: CreatePath creates
// a new file (or a directory when the input ends in a slash) under a
// parent directory, materializing any intermediate dirs along the way.
//
// All entry points are pure-ish: input is the caller-supplied parent
// directory plus the user's typed name, output is the absolute path
// created. The package never reads the file-tree pane state; the host
// passes the parent directory in. Tests drive the package with t.TempDir
// roots.
package filetreeops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrEmptyName is returned when the caller passes an empty or
// whitespace-only name (after trimming and stripping a trailing slash).
var ErrEmptyName = errors.New("filetreeops: name is empty")

// ErrAbsolutePath is returned when the caller passes an absolute path.
// Tree-driven create is always relative to a parent directory.
var ErrAbsolutePath = errors.New("filetreeops: name must be relative")

// ErrInvalidName is returned when the input contains a ".." traversal
// component. The tree never asks the user to escape the workspace.
var ErrInvalidName = errors.New("filetreeops: name contains invalid path components")

// ErrPathExists is returned when the target path already exists. The
// host surfaces this back to the prompt so the user can retype.
var ErrPathExists = errors.New("filetreeops: path already exists")

// CreateResult is the outcome of a successful CreatePath call.
type CreateResult struct {
	// Path is the absolute path of the newly created entry.
	Path string
	// IsDir reports whether a directory was created (input ended in /).
	IsDir bool
}

// CreatePath creates a new file or directory under parentDir. A
// trailing slash on input means "create as directory"; otherwise an
// empty file is created. Intermediate directories are materialized
// (mkdir -p style) so a nested input like "internal/foo/bar.go" is a
// single call.
//
// parentDir must be an absolute directory path. input must be a non-
// empty relative path with no ".." components. The function refuses to
// overwrite an existing path.
func CreatePath(parentDir, input string) (CreateResult, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return CreateResult{}, ErrEmptyName
	}
	if filepath.IsAbs(trimmed) {
		return CreateResult{}, ErrAbsolutePath
	}

	isDir := endsWithSeparator(trimmed)
	clean := strings.TrimRight(trimmed, "/")
	clean = strings.TrimRight(clean, string(filepath.Separator))
	if clean == "" {
		return CreateResult{}, ErrEmptyName
	}

	for _, part := range strings.Split(filepath.ToSlash(clean), "/") {
		if part == ".." {
			return CreateResult{}, ErrInvalidName
		}
	}

	target := filepath.Join(parentDir, filepath.FromSlash(clean))

	if _, err := os.Stat(target); err == nil {
		return CreateResult{}, ErrPathExists
	} else if !os.IsNotExist(err) {
		return CreateResult{}, fmt.Errorf("filetreeops: stat %s: %w", target, err)
	}

	parents := filepath.Dir(target)
	if err := os.MkdirAll(parents, 0o755); err != nil {
		return CreateResult{}, fmt.Errorf("filetreeops: mkdir parents %s: %w", parents, err)
	}

	if isDir {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return CreateResult{}, fmt.Errorf("filetreeops: mkdir %s: %w", target, err)
		}
	} else {
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return CreateResult{}, fmt.Errorf("filetreeops: create %s: %w", target, err)
		}
		if err := f.Close(); err != nil {
			return CreateResult{}, fmt.Errorf("filetreeops: close %s: %w", target, err)
		}
	}

	return CreateResult{Path: target, IsDir: isDir}, nil
}

func endsWithSeparator(s string) bool {
	if s == "" {
		return false
	}
	last := s[len(s)-1]
	return last == '/' || last == filepath.Separator
}
