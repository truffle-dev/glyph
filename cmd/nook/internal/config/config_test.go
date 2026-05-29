package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
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

func TestProjectPath(t *testing.T) {
	for _, tc := range []struct {
		root string
		want string
	}{
		{"/home/me/proj", filepath.Join("/home/me/proj", ".nook", "config.toml")},
		{".", filepath.Join(".", ".nook", "config.toml")},
		{"", filepath.Join("", ".nook", "config.toml")},
	} {
		if got := ProjectPath(tc.root); got != tc.want {
			t.Errorf("ProjectPath(%q) = %q, want %q", tc.root, got, tc.want)
		}
	}
}

func TestLoadRawMissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, _, err := LoadRaw(filepath.Join(dir, "absent.toml"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("LoadRaw missing err = %v, want ErrNotFound", err)
	}
	// LoadRaw does NOT seed Default() — every field stays at Go zero.
	zero := Config{}
	if cfg != zero {
		t.Errorf("LoadRaw missing cfg = %+v, want zero Config", cfg)
	}
}

func TestLoadRawDoesNotApplyDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[editor]\ntheme = \"light\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, md, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("LoadRaw err = %v", err)
	}
	// theme was set; other fields stay at Go zero (NOT Default() values).
	if cfg.Editor.Theme != "light" {
		t.Errorf("Theme = %q, want %q", cfg.Editor.Theme, "light")
	}
	if cfg.Editor.TabWidth != 0 {
		t.Errorf("TabWidth = %d, want 0 (LoadRaw does not seed Default)", cfg.Editor.TabWidth)
	}
	if cfg.Editor.FormatOnSave {
		t.Error("FormatOnSave = true, want false (LoadRaw does not seed Default)")
	}
	// MetaData reports theme defined and the others absent.
	if !md.IsDefined("editor", "theme") {
		t.Error("metadata claims theme absent; want defined")
	}
	if md.IsDefined("editor", "tab_width") {
		t.Error("metadata claims tab_width defined; want absent")
	}
}

func TestLoadRawInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("not = = valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadRaw(path)
	if err == nil {
		t.Fatal("LoadRaw invalid TOML err = nil, want parse error")
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("LoadRaw invalid TOML returned ErrNotFound, want parse error")
	}
}

// loadRawFromBody is a test helper that writes a TOML body to a temp file
// and runs it through LoadRaw so tests get a realistic toml.MetaData.
func loadRawFromBody(t *testing.T, body string) (Config, toml.MetaData) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, md, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("LoadRaw err = %v", err)
	}
	return cfg, md
}

func TestMergeEmptyOverlay(t *testing.T) {
	// Empty file => MetaData reports nothing defined => base passes through
	// untouched (plus safety reapply, which is a no-op on a valid base).
	overlay, md := loadRawFromBody(t, "")
	base := Default()
	base.Editor.TabWidth = 2
	base.Editor.Theme = "tokyo-night"
	got := Merge(base, overlay, md)
	if got.Editor.TabWidth != 2 {
		t.Errorf("TabWidth = %d, want 2 (base passthrough)", got.Editor.TabWidth)
	}
	if got.Editor.Theme != "tokyo-night" {
		t.Errorf("Theme = %q, want %q (base passthrough)", got.Editor.Theme, "tokyo-night")
	}
}

func TestMergeFullOverlay(t *testing.T) {
	overlay, md := loadRawFromBody(t, `[editor]
tab_width = 8
format_on_save = false
line_numbers = false
inlay_hints = false
theme = "rose-pine"
`)
	base := Default()
	got := Merge(base, overlay, md)
	if got.Editor.TabWidth != 8 {
		t.Errorf("TabWidth = %d, want 8", got.Editor.TabWidth)
	}
	if got.Editor.FormatOnSave {
		t.Error("FormatOnSave = true, want false (overlay wins)")
	}
	if got.Editor.LineNumbers {
		t.Error("LineNumbers = true, want false (overlay wins)")
	}
	if got.Editor.InlayHints {
		t.Error("InlayHints = true, want false (overlay wins)")
	}
	if got.Editor.Theme != "rose-pine" {
		t.Errorf("Theme = %q, want %q", got.Editor.Theme, "rose-pine")
	}
}

func TestMergePartialOverlay(t *testing.T) {
	overlay, md := loadRawFromBody(t, `[editor]
theme = "light"
`)
	base := Default()
	base.Editor.TabWidth = 2
	base.Editor.FormatOnSave = false
	base.Editor.LineNumbers = false
	base.Editor.InlayHints = false
	base.Editor.Theme = "tokyo-night"
	got := Merge(base, overlay, md)
	if got.Editor.Theme != "light" {
		t.Errorf("Theme = %q, want %q (overlay wins for explicit field)", got.Editor.Theme, "light")
	}
	if got.Editor.TabWidth != 2 {
		t.Errorf("TabWidth = %d, want 2 (base passes through)", got.Editor.TabWidth)
	}
	if got.Editor.FormatOnSave {
		t.Error("FormatOnSave = true, want false (base passes through)")
	}
	if got.Editor.LineNumbers {
		t.Error("LineNumbers = true, want false (base passes through)")
	}
	if got.Editor.InlayHints {
		t.Error("InlayHints = true, want false (base passes through)")
	}
}

func TestMergeOverlayExplicitFalseOverridesBaseTrue(t *testing.T) {
	// The killer case for the metadata-driven merge: overlay explicitly
	// sets format_on_save = false; base has it true. Without metadata
	// this would look indistinguishable from "absent" and base would win.
	overlay, md := loadRawFromBody(t, `[editor]
format_on_save = false
`)
	base := Default() // FormatOnSave = true
	got := Merge(base, overlay, md)
	if got.Editor.FormatOnSave {
		t.Error("FormatOnSave = true, want false (overlay's explicit false must win)")
	}
}

func TestMergeOverlayExplicitZeroTabWidthSafetyReapplies(t *testing.T) {
	// Overlay sets tab_width = 0 explicitly. Merge honors the overlay
	// (so the underlying value moves to 0) and then the safety reapply
	// at the end bumps it back to 4 — same behavior as Load.
	overlay, md := loadRawFromBody(t, `[editor]
tab_width = 0
`)
	base := Default() // TabWidth = 4
	got := Merge(base, overlay, md)
	if got.Editor.TabWidth != 4 {
		t.Errorf("TabWidth = %d, want 4 (safety reapply)", got.Editor.TabWidth)
	}
}

func TestMergeOverlayExplicitEmptyThemeSafetyReapplies(t *testing.T) {
	overlay, md := loadRawFromBody(t, `[editor]
theme = ""
`)
	base := Default()
	base.Editor.Theme = "tokyo-night"
	got := Merge(base, overlay, md)
	if got.Editor.Theme != "default" {
		t.Errorf("Theme = %q, want %q (safety reapply)", got.Editor.Theme, "default")
	}
}

func TestMergeDoesNotMutateBase(t *testing.T) {
	overlay, md := loadRawFromBody(t, `[editor]
theme = "rose-pine"
tab_width = 8
`)
	base := Default()
	_ = Merge(base, overlay, md)
	if base.Editor.Theme != "default" {
		t.Errorf("base.Theme = %q, want %q (caller's base must stay untouched)", base.Editor.Theme, "default")
	}
	if base.Editor.TabWidth != 4 {
		t.Errorf("base.TabWidth = %d, want 4 (caller's base must stay untouched)", base.Editor.TabWidth)
	}
}

func TestMergeBaseAlreadyMergedTwoLayers(t *testing.T) {
	// Demonstrates the host's intended use: user config loaded with
	// Load (defaults applied), then merged with project config from
	// LoadRaw. Project's explicit fields override the user's.
	user, _ := loadRawFromBody(t, `[editor]
tab_width = 2
theme = "tokyo-night"
format_on_save = true
inlay_hints = true
line_numbers = true
`)
	project, projectMeta := loadRawFromBody(t, `[editor]
tab_width = 8
format_on_save = false
`)
	// Bring user up to Default()'s safety baseline first (matches what
	// Load does at the end of its decode path).
	if user.Editor.TabWidth <= 0 {
		user.Editor.TabWidth = 4
	}
	if user.Editor.Theme == "" {
		user.Editor.Theme = "default"
	}
	got := Merge(user, project, projectMeta)
	if got.Editor.TabWidth != 8 {
		t.Errorf("TabWidth = %d, want 8 (project wins)", got.Editor.TabWidth)
	}
	if got.Editor.FormatOnSave {
		t.Error("FormatOnSave = true, want false (project wins)")
	}
	if got.Editor.Theme != "tokyo-night" {
		t.Errorf("Theme = %q, want %q (user passes through)", got.Editor.Theme, "tokyo-night")
	}
	if !got.Editor.InlayHints {
		t.Error("InlayHints = false, want true (user passes through)")
	}
	if !got.Editor.LineNumbers {
		t.Error("LineNumbers = false, want true (user passes through)")
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
