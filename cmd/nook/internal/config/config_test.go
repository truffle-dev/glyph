package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Editor.TabWidth != 4 {
		t.Errorf("Default TabWidth = %d, want 4", cfg.Editor.TabWidth)
	}
	if !cfg.Editor.FormatOnSave {
		t.Error("Default FormatOnSave = false, want true")
	}
	if !cfg.Editor.LineNumbers {
		t.Error("Default LineNumbers = false, want true")
	}
	if !cfg.Editor.InlayHints {
		t.Error("Default InlayHints = false, want true")
	}
	if cfg.Editor.Theme != "default" {
		t.Errorf("Default Theme = %q, want %q", cfg.Editor.Theme, "default")
	}
}

func TestPathXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")
	got, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	want := filepath.Join("/custom/xdg", "nook", "config.toml")
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestPathHomeFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	got, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	want := filepath.Join(home, ".config", "nook", "config.toml")
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "does-not-exist.toml"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load missing file err = %v, want ErrNotFound", err)
	}
	if cfg.Editor.TabWidth != 4 || cfg.Editor.Theme != "default" {
		t.Errorf("Load missing file did not return Default(): %+v", cfg)
	}
}

func TestLoadValidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `[editor]
tab_width = 2
format_on_save = false
line_numbers = false
inlay_hints = false
theme = "tokyo-night"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load valid TOML err = %v", err)
	}
	if cfg.Editor.TabWidth != 2 {
		t.Errorf("TabWidth = %d, want 2", cfg.Editor.TabWidth)
	}
	if cfg.Editor.FormatOnSave {
		t.Error("FormatOnSave = true, want false")
	}
	if cfg.Editor.LineNumbers {
		t.Error("LineNumbers = true, want false")
	}
	if cfg.Editor.InlayHints {
		t.Error("InlayHints = true, want false")
	}
	if cfg.Editor.Theme != "tokyo-night" {
		t.Errorf("Theme = %q, want %q", cfg.Editor.Theme, "tokyo-night")
	}
}

func TestLoadPartialTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `[editor]
tab_width = 8
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load partial TOML err = %v", err)
	}
	if cfg.Editor.TabWidth != 8 {
		t.Errorf("TabWidth = %d, want 8 (from file)", cfg.Editor.TabWidth)
	}
	if !cfg.Editor.FormatOnSave {
		t.Error("FormatOnSave = false, want true (default fallback)")
	}
	if cfg.Editor.Theme != "default" {
		t.Errorf("Theme = %q, want %q (default fallback)", cfg.Editor.Theme, "default")
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `this is = = not valid toml @@@`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err == nil {
		t.Fatal("Load invalid TOML err = nil, want parse error")
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("Load invalid TOML returned ErrNotFound, want parse error")
	}
	if cfg.Editor.TabWidth != 4 || cfg.Editor.Theme != "default" {
		t.Errorf("Load invalid TOML did not return Default(): %+v", cfg)
	}
}

func TestLoadZeroTabWidthReappliesSafety(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `[editor]
tab_width = 0
theme = "default"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	if cfg.Editor.TabWidth != 4 {
		t.Errorf("TabWidth = %d, want 4 (safety reapplied)", cfg.Editor.TabWidth)
	}
}

func TestLoadEmptyThemeReappliesSafety(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `[editor]
theme = ""
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	if cfg.Editor.Theme != "default" {
		t.Errorf("Theme = %q, want %q (safety reapplied)", cfg.Editor.Theme, "default")
	}
}

func TestLoadUnknownKeysIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `[editor]
tab_width = 2
unknown_future_key = "ignored"

[someday_new_section]
field = 42
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load with unknown keys err = %v (should be silent)", err)
	}
	if cfg.Editor.TabWidth != 2 {
		t.Errorf("TabWidth = %d, want 2", cfg.Editor.TabWidth)
	}
}

func TestEnsureDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c", "config.toml")
	if err := EnsureDir(nested); err != nil {
		t.Fatalf("EnsureDir err = %v", err)
	}
	info, err := os.Stat(filepath.Dir(nested))
	if err != nil {
		t.Fatalf("Stat parent dir err = %v", err)
	}
	if !info.IsDir() {
		t.Error("EnsureDir did not create a directory")
	}
}

func TestEnsureDirIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nook", "config.toml")
	if err := EnsureDir(path); err != nil {
		t.Fatalf("first EnsureDir err = %v", err)
	}
	if err := EnsureDir(path); err != nil {
		t.Fatalf("second EnsureDir err = %v (should be idempotent)", err)
	}
}

func TestLoadAllRegisteredThemeNames(t *testing.T) {
	// A user can set any registered theme name. config.Load doesn't validate
	// the theme name itself — that's the host's job at apply time — but it
	// must round-trip the string unchanged.
	for _, name := range []string{"default", "light", "tokyo-night", "catppuccin-mocha", "rose-pine"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			body := "[editor]\ntheme = \"" + name + "\"\n"
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load err = %v", err)
			}
			if cfg.Editor.Theme != name {
				t.Errorf("Theme = %q, want %q", cfg.Editor.Theme, name)
			}
		})
	}
}

func TestLoadParseErrorMessageReadable(t *testing.T) {
	// The error surfaced from a bad config file becomes a status hint in the
	// host. Sanity-check that it carries the substring "toml" or the BurntSushi
	// "expected" wording so the user has a clue what went wrong.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `[editor
tab_width = 4
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "toml") && !strings.Contains(msg, "expected") && !strings.Contains(msg, "bare") {
		t.Errorf("parse error message %q lacks a useful hint", err.Error())
	}
}
