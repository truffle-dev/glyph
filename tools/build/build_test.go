package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests drive run() against a temp components/ tree and assert
// every visible shape of the registry it emits: per-item manifest fields,
// the sorted catalog, file copies on disk, embedded URLs, and a final
// HTTP round-trip that serves the output tree and confirms the URLs
// resolve to the right bodies. Together they close the loop between
// tools/build (producer) and cmd/glyph add (consumer).

const fixtureFooBuildManifest = `{
  "$schema": "test",
  "name": "foo",
  "type": "glyph:component",
  "title": "Foo",
  "description": "fixture",
  "version": "0.1.0",
  "frame": "bubbletea",
  "files": [
    {
      "path": "components/foo/foo.go",
      "type": "glyph:component",
      "target": "@components/foo/foo.go"
    }
  ]
}
`

const fixtureFooBuildBody = `package foo

func Hello() string { return "hello" }
`

const fixtureBarBuildManifest = `{
  "$schema": "test",
  "name": "bar",
  "type": "glyph:component",
  "title": "Bar",
  "description": "fixture with a registry dep and two files",
  "version": "0.1.0",
  "frame": "bubbletea",
  "registryDependencies": ["foo"],
  "files": [
    {
      "path": "components/bar/bar.go",
      "type": "glyph:component",
      "target": "@components/bar/bar.go"
    },
    {
      "path": "components/bar/bar_test.go",
      "type": "glyph:test",
      "target": "@components/bar/bar_test.go"
    }
  ]
}
`

const fixtureBarBuildBody = `package bar

import "github.com/truffle-dev/glyph/components/foo"

func Greet() string { return foo.Hello() + " world" }
`

const fixtureBarBuildTest = `package bar

import "testing"

func TestGreet(t *testing.T) { _ = Greet() }
`

func writeBuildFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"components/foo/foo.json":    fixtureFooBuildManifest,
		"components/foo/foo.go":      fixtureFooBuildBody,
		"components/bar/bar.json":    fixtureBarBuildManifest,
		"components/bar/bar.go":      fixtureBarBuildBody,
		"components/bar/bar_test.go": fixtureBarBuildTest,
	}
	for rel, body := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
}

func inBuildSandbox(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return dir
}

func TestBuildEmitsCatalogAndPerItemManifests(t *testing.T) {
	root := inBuildSandbox(t)
	writeBuildFixture(t, root)

	if err := run("components", "r", "https://example.test/glyph/r", nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	catBytes, err := os.ReadFile(filepath.Join(root, "r", "registry.json"))
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	var cat catalog
	if err := json.Unmarshal(catBytes, &cat); err != nil {
		t.Fatalf("parse catalog: %v", err)
	}
	if len(cat.Items) != 2 {
		t.Fatalf("catalog items = %d, want 2", len(cat.Items))
	}
	if cat.Items[0].Name != "bar" || cat.Items[1].Name != "foo" {
		t.Errorf("catalog not sorted by name: got %v", []string{cat.Items[0].Name, cat.Items[1].Name})
	}

	fooBytes, err := os.ReadFile(filepath.Join(root, "r", "foo.json"))
	if err != nil {
		t.Fatalf("read foo.json: %v", err)
	}
	var foo registryItem
	if err := json.Unmarshal(fooBytes, &foo); err != nil {
		t.Fatalf("parse foo.json: %v", err)
	}
	if foo.Name != "foo" || foo.Version != "0.1.0" || foo.Frame != "bubbletea" {
		t.Errorf("foo manifest fields wrong: %+v", foo)
	}
	if len(foo.Files) != 1 {
		t.Fatalf("foo files = %d, want 1", len(foo.Files))
	}
	wantURL := "https://example.test/glyph/r/foo/foo.go"
	if foo.Files[0].URL != wantURL {
		t.Errorf("foo file URL = %q, want %q", foo.Files[0].URL, wantURL)
	}
	if foo.Files[0].Target != "@components/foo/foo.go" {
		t.Errorf("target lost in emit: %q", foo.Files[0].Target)
	}

	gotBody, err := os.ReadFile(filepath.Join(root, "r", "foo", "foo.go"))
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if !strings.Contains(string(gotBody), "func Hello") {
		t.Errorf("file copy missing expected body; got:\n%s", gotBody)
	}
}

func TestBuildPreservesRegistryDependenciesAndMultipleFiles(t *testing.T) {
	root := inBuildSandbox(t)
	writeBuildFixture(t, root)

	if err := run("components", "r", "https://example.test/glyph/r", nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	barBytes, err := os.ReadFile(filepath.Join(root, "r", "bar.json"))
	if err != nil {
		t.Fatalf("read bar.json: %v", err)
	}
	var bar registryItem
	if err := json.Unmarshal(barBytes, &bar); err != nil {
		t.Fatalf("parse bar.json: %v", err)
	}
	if len(bar.RegistryDependencies) != 1 || bar.RegistryDependencies[0] != "foo" {
		t.Errorf("bar registryDependencies = %v, want [foo]", bar.RegistryDependencies)
	}
	if len(bar.Files) != 2 {
		t.Fatalf("bar files = %d, want 2", len(bar.Files))
	}
	seen := map[string]string{}
	for _, f := range bar.Files {
		seen[f.Target] = f.URL
	}
	for _, target := range []string{"@components/bar/bar.go", "@components/bar/bar_test.go"} {
		if seen[target] == "" {
			t.Errorf("missing file with target %q", target)
		}
	}

	for _, fname := range []string{"bar.go", "bar_test.go"} {
		if _, err := os.Stat(filepath.Join(root, "r", "bar", fname)); err != nil {
			t.Errorf("file copy %s missing: %v", fname, err)
		}
	}
}

func TestBuildSetsCatalogMetadata(t *testing.T) {
	root := inBuildSandbox(t)
	writeBuildFixture(t, root)

	if err := run("components", "r", "https://example.test/glyph/r", nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	catBytes, err := os.ReadFile(filepath.Join(root, "r", "registry.json"))
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	var cat catalog
	if err := json.Unmarshal(catBytes, &cat); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cat.Name != "glyph" {
		t.Errorf("catalog name = %q, want glyph", cat.Name)
	}
	if cat.Homepage != "https://truffleagent.com/glyph" {
		t.Errorf("catalog homepage = %q", cat.Homepage)
	}
	if !strings.HasPrefix(cat.Schema, "https://truffleagent.com/glyph/schema/") {
		t.Errorf("catalog schema = %q", cat.Schema)
	}
}

func TestBuildRejectsInvalidManifests(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		wantSub  string
	}{
		{
			"missing name",
			`{"type":"glyph:component","version":"0.1.0","files":[{"path":"components/x/x.go","type":"glyph:component","target":"@x.go"}]}`,
			"missing name",
		},
		{
			"missing type",
			`{"name":"x","version":"0.1.0","files":[{"path":"components/x/x.go","type":"glyph:component","target":"@x.go"}]}`,
			"missing type",
		},
		{
			"missing version",
			`{"name":"x","type":"glyph:component","files":[{"path":"components/x/x.go","type":"glyph:component","target":"@x.go"}]}`,
			"missing version",
		},
		{
			"no files",
			`{"name":"x","type":"glyph:component","version":"0.1.0"}`,
			"no files declared",
		},
		{
			"file missing target",
			`{"name":"x","type":"glyph:component","version":"0.1.0","files":[{"path":"components/x/x.go","type":"glyph:component"}]}`,
			"missing target",
		},
		{
			"file missing type",
			`{"name":"x","type":"glyph:component","version":"0.1.0","files":[{"path":"components/x/x.go","target":"@x.go"}]}`,
			"missing type",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := inBuildSandbox(t)
			if err := os.MkdirAll(filepath.Join(root, "components", "x"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "components", "x", "x.json"), []byte(tc.manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "components", "x", "x.go"), []byte("package x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			err := run("components", "r", "https://example.test/glyph/r", nil)
			if err == nil {
				t.Fatalf("expected validation error containing %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

// TestBuildSchemaRejectsInvalidPatterns drives the schema-validation
// layer that runs after the in-code validator catches missing fields.
// Each case has every required field present, but one field violates a
// pattern or enum that lives in schema/registry-item.json. The schema
// is loaded from disk; if anyone breaks the link between the manifest
// shape and the published schema, this test fails.
func TestBuildSchemaRejectsInvalidPatterns(t *testing.T) {
	pkgDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	schemaAbs := filepath.Join(pkgDir, "..", "..", "schema", "registry-item.json")
	if _, err := os.Stat(schemaAbs); err != nil {
		t.Skipf("schema file not present at %s: %v", schemaAbs, err)
	}
	sch, err := loadSchema(schemaAbs)
	if err != nil {
		t.Fatalf("loadSchema: %v", err)
	}

	cases := []struct {
		name     string
		manifest string
		wantSub  string
	}{
		{
			"name not kebab-case",
			`{"$schema":"test","name":"FooBar","type":"glyph:component","title":"x","description":"x","version":"0.1.0","frame":"bubbletea","files":[{"path":"components/x/x.go","type":"glyph:component","target":"@components/x/x.go"}]}`,
			"name",
		},
		{
			"version not semver",
			`{"$schema":"test","name":"x","type":"glyph:component","title":"x","description":"x","version":"1.0","frame":"bubbletea","files":[{"path":"components/x/x.go","type":"glyph:component","target":"@components/x/x.go"}]}`,
			"version",
		},
		{
			"unknown frame",
			`{"$schema":"test","name":"x","type":"glyph:component","title":"x","description":"x","version":"0.1.0","frame":"qt","files":[{"path":"components/x/x.go","type":"glyph:component","target":"@components/x/x.go"}]}`,
			"frame",
		},
		{
			"target missing alias prefix",
			`{"$schema":"test","name":"x","type":"glyph:component","title":"x","description":"x","version":"0.1.0","frame":"bubbletea","files":[{"path":"components/x/x.go","type":"glyph:component","target":"components/x/x.go"}]}`,
			"target",
		},
		{
			"unknown item type",
			`{"$schema":"test","name":"x","type":"glyph:widget","title":"x","description":"x","version":"0.1.0","frame":"bubbletea","files":[{"path":"components/x/x.go","type":"glyph:component","target":"@components/x/x.go"}]}`,
			"type",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := inBuildSandbox(t)
			if err := os.MkdirAll(filepath.Join(root, "components", "x"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "components", "x", "x.json"), []byte(tc.manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "components", "x", "x.go"), []byte("package x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			err := run("components", "r", "https://example.test/glyph/r", sch)
			if err == nil {
				t.Fatalf("expected schema error mentioning %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), "schema validating") {
				t.Errorf("error = %v, want 'schema validating' prefix", err)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

// TestBuildSchemaAcceptsRealManifests compiles the on-disk schema and
// validates every committed component manifest against it. If a manifest
// ever drifts out of conformance with the schema (or the schema tightens
// in a way that breaks a real manifest) this test catches it.
func TestBuildSchemaAcceptsRealManifests(t *testing.T) {
	pkgDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Join(pkgDir, "..", "..")
	schemaAbs := filepath.Join(repoRoot, "schema", "registry-item.json")
	if _, err := os.Stat(schemaAbs); err != nil {
		t.Skipf("schema file not present at %s: %v", schemaAbs, err)
	}
	sch, err := loadSchema(schemaAbs)
	if err != nil {
		t.Fatalf("loadSchema: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(repoRoot, "components"))
	if err != nil {
		t.Fatalf("read components dir: %v", err)
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(repoRoot, "components", e.Name(), e.Name()+".json")
		b, err := os.ReadFile(manifestPath)
		if err != nil {
			continue // not every directory has a manifest; skip silently
		}
		var raw any
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Errorf("%s: parse: %v", manifestPath, err)
			continue
		}
		if err := sch.Validate(raw); err != nil {
			t.Errorf("%s: schema validation failed: %v", e.Name(), err)
		}
		count++
	}
	if count == 0 {
		t.Fatal("found no component manifests to validate")
	}
}

func TestBuildRefusesEmptyComponentTree(t *testing.T) {
	root := inBuildSandbox(t)
	if err := os.MkdirAll(filepath.Join(root, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := run("components", "r", "https://example.test/glyph/r", nil)
	if err == nil {
		t.Fatal("expected error on empty components tree")
	}
	if !strings.Contains(err.Error(), "no manifests found") {
		t.Errorf("error = %v, want 'no manifests found' substring", err)
	}
}

// TestBuildOutputServesAsValidRegistry is the round-trip: build the
// registry, serve r/ over HTTP, then walk the catalog → per-item manifest
// → file URLs and confirm each one resolves. This is the producer-side
// twin of cmd/glyph/integration_test.go's consumer-side coverage.
func TestBuildOutputServesAsValidRegistry(t *testing.T) {
	root := inBuildSandbox(t)
	writeBuildFixture(t, root)

	const baseURL = "https://example.test/glyph/r"
	if err := run("components", "r", baseURL, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	srv := httptest.NewServer(http.FileServer(http.Dir(filepath.Join(root, "r"))))
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/registry.json")
	if err != nil {
		t.Fatalf("fetch catalog: %v", err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		t.Fatalf("catalog HTTP %d", resp.StatusCode)
	}
	var cat catalog
	if err := json.NewDecoder(resp.Body).Decode(&cat); err != nil {
		resp.Body.Close()
		t.Fatalf("decode catalog: %v", err)
	}
	resp.Body.Close()

	if len(cat.Items) == 0 {
		t.Fatal("catalog empty after build")
	}

	for _, entry := range cat.Items {
		itemResp, err := http.Get(srv.URL + "/" + entry.Name + ".json")
		if err != nil {
			t.Errorf("fetch %s.json: %v", entry.Name, err)
			continue
		}
		if itemResp.StatusCode != 200 {
			itemResp.Body.Close()
			t.Errorf("%s.json HTTP %d", entry.Name, itemResp.StatusCode)
			continue
		}
		var item registryItem
		if err := json.NewDecoder(itemResp.Body).Decode(&item); err != nil {
			itemResp.Body.Close()
			t.Errorf("decode %s.json: %v", entry.Name, err)
			continue
		}
		itemResp.Body.Close()

		if item.Name != entry.Name {
			t.Errorf("catalog entry %q points at item with name %q", entry.Name, item.Name)
		}

		for _, f := range item.Files {
			if !strings.HasPrefix(f.URL, baseURL+"/") {
				t.Errorf("file URL %q missing base prefix", f.URL)
				continue
			}
			rel := strings.TrimPrefix(f.URL, baseURL)
			fr, err := http.Get(srv.URL + rel)
			if err != nil {
				t.Errorf("fetch %s: %v", f.URL, err)
				continue
			}
			body, _ := io.ReadAll(fr.Body)
			fr.Body.Close()
			if fr.StatusCode != 200 {
				t.Errorf("file %s HTTP %d", f.URL, fr.StatusCode)
			}
			if len(body) == 0 {
				t.Errorf("file %s empty after round-trip", f.URL)
			}
		}
	}
}
