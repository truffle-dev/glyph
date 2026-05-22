package main

import (
	"bytes"
	"strings"
)

// glyphSourceModule is the upstream module path that the registry's source files
// reference. Local imports between components use this path so they compile
// in-place during component development; the CLI rewrites them on install to
// the consumer's module + aliases.
const glyphSourceModule = "github.com/truffle-dev/glyph"

// rewriteImports rewrites `github.com/truffle-dev/glyph/components/<x>` imports
// to `<consumer-module>/<components-alias>/<x>` and `.../lib/<x>` to
// `<consumer-module>/<lib-alias>/<x>`.
func rewriteImports(src []byte, c Config) []byte {
	if c.Module == "" {
		return src
	}
	mappings := map[string]string{
		glyphSourceModule + "/components": joinModule(c.Module, c.Aliases["components"]),
		glyphSourceModule + "/lib":        joinModule(c.Module, c.Aliases["lib"]),
		glyphSourceModule + "/hooks":      joinModule(c.Module, c.Aliases["hooks"]),
	}
	out := src
	for from, to := range mappings {
		if to == "" {
			continue
		}
		out = bytes.ReplaceAll(out, []byte(from), []byte(to))
	}
	return out
}

// applyStyles substitutes `{{theme.X}}` placeholders inside source with the
// component's style override (if any). Style values that themselves reference
// other tokens (e.g., `{{theme.primary}}`) stay as token references so the
// installed code points to the user's theme file.
func applyStyles(src []byte, styles map[string]string) []byte {
	if len(styles) == 0 {
		return src
	}
	out := string(src)
	for k, v := range styles {
		placeholder := "{{styles." + k + "}}"
		out = strings.ReplaceAll(out, placeholder, v)
	}
	return []byte(out)
}

// joinModule joins a module path with a sub-path, normalizing slashes.
func joinModule(module, sub string) string {
	if sub == "" {
		return module
	}
	sub = strings.TrimLeft(strings.ReplaceAll(sub, "\\", "/"), "./")
	return strings.TrimRight(module, "/") + "/" + sub
}
