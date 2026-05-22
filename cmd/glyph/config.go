package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultRegistryURL = "https://truffleagent.com/glyph/r"

// Config is the consumer-side glyph.json shape.
type Config struct {
	Schema   string            `json:"$schema"`
	Frame    string            `json:"frame"`
	Module   string            `json:"module"`
	Aliases  map[string]string `json:"aliases"`
	Theme    string            `json:"theme"`
	Registry string            `json:"registry,omitempty"`
}

func defaultConfig() Config {
	return Config{
		Schema: "https://truffleagent.com/glyph/schema/glyph.json",
		Frame:  "bubbletea",
		Aliases: map[string]string{
			"components": "internal/ui",
			"lib":        "internal/uilib",
			"hooks":      "internal/uihooks",
		},
		Theme: "default",
	}
}

func loadConfig() (Config, string, error) {
	path, err := findConfig()
	if err != nil {
		return Config{}, "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, "", err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, "", fmt.Errorf("parsing %s: %w", path, err)
	}
	return c, path, nil
}

func findConfig() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(wd, "glyph.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", fmt.Errorf("glyph.json not found in this directory or any parent; run `glyph init` first")
		}
		wd = parent
	}
}

func writeConfig(c Config, path string) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// resolveAlias expands `@components/foo` against the consumer's aliases map.
// Returns the path relative to the project root.
func (c Config) resolveAlias(p string) (string, error) {
	if !strings.HasPrefix(p, "@") {
		return p, nil
	}
	slash := strings.Index(p, "/")
	if slash == -1 {
		return "", fmt.Errorf("invalid alias path %q: expected `@alias/...`", p)
	}
	alias := p[1:slash]
	rest := p[slash+1:]
	target, ok := c.Aliases[alias]
	if !ok {
		return "", fmt.Errorf("unknown alias %q in path %q (known: %s)", alias, p, strings.Join(aliasKeys(c.Aliases), ", "))
	}
	return filepath.Join(target, rest), nil
}

func aliasKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func (c Config) registryURL() string {
	if c.Registry != "" {
		return strings.TrimRight(c.Registry, "/")
	}
	return defaultRegistryURL
}
