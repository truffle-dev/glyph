// Package config loads nook's user configuration from ~/.config/nook/config.toml.
//
// The file has two sections — [editor] for behavior toggles and [theme] for the
// named palette — and every key is optional. Missing fields fall back to
// Default(); unknown keys are accepted silently (forward-compat). A malformed
// file returns an error from Load; the host shows a status hint and falls
// back to defaults rather than refusing to start.
//
// Live reload is intentionally manual in v0.15.0: the host binds `alt+,` to
// re-read the file at runtime. fsnotify-based auto-reload is a follow-up.
package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the deserialized user configuration. New fields should default to
// the zero value that matches Default() so an empty TOML file is equivalent to
// no file at all.
type Config struct {
	Editor EditorConfig `toml:"editor"`
}

// EditorConfig holds the per-pane behavior toggles. Defaults match the
// hard-coded behavior shipped before settings existed, so an empty file is a
// no-op for existing users.
type EditorConfig struct {
	// TabWidth is the column count a hard tab expands to in the rendered
	// buffer. Default 4. The on-disk file bytes always stay tabs.
	TabWidth int `toml:"tab_width"`
	// FormatOnSave runs textDocument/formatting through the attached LSP
	// before writing. Default true. Alt+S still forces a no-format save.
	FormatOnSave bool `toml:"format_on_save"`
	// LineNumbers controls whether the gutter prints the row number.
	// Default true.
	LineNumbers bool `toml:"line_numbers"`
	// InlayHints toggles gopls type / parameter hint glyphs woven into
	// the source. Default true. Alt+Y still toggles at runtime.
	InlayHints bool `toml:"inlay_hints"`
	// Theme is the named palette to apply at startup. Must match one of
	// the names registered in components/theme; unknown names fall back
	// to "default" and surface a status hint.
	Theme string `toml:"theme"`
}

// Default returns the baseline configuration. Equivalent to loading an empty
// file. Use this as the fallback when ~/.config/nook/config.toml does not
// exist or fails to parse.
func Default() Config {
	return Config{
		Editor: EditorConfig{
			TabWidth:     4,
			FormatOnSave: true,
			LineNumbers:  true,
			InlayHints:   true,
			Theme:        "default",
		},
	}
}

// ErrNotFound is returned by Load when the config file is absent. The host
// treats this as "no settings file" and uses Default() without surfacing an
// error to the user. Distinct from a parse error, which IS surfaced.
var ErrNotFound = errors.New("config: file not found")

// Path returns the canonical config file location:
// $XDG_CONFIG_HOME/nook/config.toml when set, else ~/.config/nook/config.toml.
// Mirrors how alacritty, helix, and zellij resolve their config paths so the
// muscle memory carries over.
func Path() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "nook", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "nook", "config.toml"), nil
}

// Load reads and parses the config file at path. Returns ErrNotFound when
// the file doesn't exist (the host should fall back to Default silently);
// any other error indicates the file exists but is malformed (the host
// should surface the message and fall back to Default).
//
// Fields missing from the file are filled in from Default — so loading an
// empty file is identical to calling Default().
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), ErrNotFound
		}
		return Default(), err
	}
	cfg := Default()
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Default(), err
	}
	// A user can clear a string field but the zero value of a bool /
	// int means "field absent" too. We keep the toml-defaulting logic
	// here: a missing key in TOML deserializes to the Go zero value,
	// which overwrites our Default() seeds. Reapply defaults for any
	// field where Go zero value would be invalid.
	if cfg.Editor.TabWidth <= 0 {
		cfg.Editor.TabWidth = 4
	}
	if cfg.Editor.Theme == "" {
		cfg.Editor.Theme = "default"
	}
	return cfg, nil
}

// EnsureDir creates the parent directory for the config file if it doesn't
// exist. Useful when generating a starter config on first run.
func EnsureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

// ProjectPath returns the per-project config path: <root>/.nook/config.toml.
// Used to layer a repo-local override on top of the user's global config.
// A `.nook/` directory already houses `tasks.toml` (v0.19.0), so the
// settings file lives alongside it for discoverability.
func ProjectPath(root string) string {
	return filepath.Join(root, ".nook", "config.toml")
}

// LoadRaw reads and decodes a config file WITHOUT applying Default() seeds
// or the safety reapplies that Load performs. Returns the parsed Config —
// every field at its Go zero value unless the file set it explicitly —
// plus toml.MetaData so callers can distinguish "absent" from "explicit
// zero" for bool/int fields. Returns ErrNotFound when the file doesn't
// exist; any other read or parse error is surfaced as-is.
//
// This is the substrate for Merge. A per-project file decoded through
// LoadRaw only carries the keys the project explicitly set, so the merge
// only overlays those keys onto the user's choice.
func LoadRaw(path string) (Config, toml.MetaData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, toml.MetaData{}, ErrNotFound
		}
		return Config{}, toml.MetaData{}, err
	}
	var cfg Config
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, toml.MetaData{}, err
	}
	return cfg, md, nil
}

// Merge layers overlay on top of base. For each editor field, if the
// overlay's TOML metadata reports the key as defined, the overlay value
// wins; otherwise base passes through. Safety reapplies (TabWidth > 0,
// Theme != "") run at the end so the result is always usable.
//
// Use this to layer a per-project .nook/config.toml on top of the user's
// global ~/.config/nook/config.toml. Project values for keys it explicitly
// sets win; keys it omits inherit the user's choice. The MetaData
// parameter must come from the same LoadRaw that produced overlay; a
// zero-value MetaData treats every overlay field as undefined and the
// call collapses to a base passthrough plus safety reapply.
func Merge(base, overlay Config, overlayMeta toml.MetaData) Config {
	out := base
	if overlayMeta.IsDefined("editor", "tab_width") {
		out.Editor.TabWidth = overlay.Editor.TabWidth
	}
	if overlayMeta.IsDefined("editor", "format_on_save") {
		out.Editor.FormatOnSave = overlay.Editor.FormatOnSave
	}
	if overlayMeta.IsDefined("editor", "line_numbers") {
		out.Editor.LineNumbers = overlay.Editor.LineNumbers
	}
	if overlayMeta.IsDefined("editor", "inlay_hints") {
		out.Editor.InlayHints = overlay.Editor.InlayHints
	}
	if overlayMeta.IsDefined("editor", "theme") {
		out.Editor.Theme = overlay.Editor.Theme
	}
	if out.Editor.TabWidth <= 0 {
		out.Editor.TabWidth = 4
	}
	if out.Editor.Theme == "" {
		out.Editor.Theme = "default"
	}
	return out
}
