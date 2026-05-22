package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// loadGlyphSchema compiles schema/glyph.json from the repo root. The CLI test
// package runs from cmd/glyph/, so we walk two levels up to find the schema.
func loadGlyphSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	schemaPath := filepath.Join(wd, "..", "..", "schema", "glyph.json")
	f, err := os.Open(schemaPath)
	if err != nil {
		t.Fatalf("open schema: %v", err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaPath, doc); err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	sch, err := c.Compile(schemaPath)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return sch
}

// marshalRaw round-trips a typed config through JSON so the schema validator
// sees the same shape the CLI writes to disk.
func marshalRaw(t *testing.T, c Config) any {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	return raw
}

// TestDefaultConfigValidatesAgainstSchema confirms that a populated default
// config (the shape `glyph init` writes) passes schema/glyph.json.
func TestDefaultConfigValidatesAgainstSchema(t *testing.T) {
	sch := loadGlyphSchema(t)
	c := defaultConfig()
	c.Module = "github.com/example/app"
	if err := sch.Validate(marshalRaw(t, c)); err != nil {
		t.Fatalf("default config failed schema: %v", err)
	}
}

// TestSchemaRejectsInvalidConfigs walks five rejection cases. Each case names a
// distinct schema rule and confirms the validator catches a violation.
func TestSchemaRejectsInvalidConfigs(t *testing.T) {
	sch := loadGlyphSchema(t)
	base := defaultConfig()
	base.Module = "github.com/example/app"

	cases := []struct {
		name     string
		mutate   func(*Config)
		contains string
	}{
		{
			name:     "missing module",
			mutate:   func(c *Config) { c.Module = "" },
			contains: "module",
		},
		{
			name:     "unknown frame",
			mutate:   func(c *Config) { c.Frame = "qt" },
			contains: "frame",
		},
		{
			name:     "module with whitespace",
			mutate:   func(c *Config) { c.Module = "github.com/me/has space" },
			contains: "module",
		},
		{
			name:     "missing required components alias",
			mutate:   func(c *Config) { delete(c.Aliases, "components") },
			contains: "components",
		},
		{
			name:     "alias path with leading slash",
			mutate:   func(c *Config) { c.Aliases["components"] = "/abs/path" },
			contains: "components",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			c.Aliases = make(map[string]string, len(base.Aliases))
			for k, v := range base.Aliases {
				c.Aliases[k] = v
			}
			tc.mutate(&c)
			err := sch.Validate(marshalRaw(t, c))
			if err == nil {
				t.Fatalf("expected schema rejection for %q, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("expected schema error to mention %q, got: %v", tc.contains, err)
			}
		})
	}
}
