package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The integration tests drive runInit and runAdd end-to-end against a
// local httptest registry. They cover the consumer's first install
// experience: project scaffold, manifest write, alias resolution,
// file fetch, import rewrite, dependency resolution. The fixture
// components have no `dependencies` so the test stays offline (no
// `go get` shells out).

const fixtureFooBody = `package foo

// Hello returns a greeting. Used to verify the import-rewrite path
// resolves to the consumer's module, not glyph's source module.
func Hello() string { return "hello" }
`

const fixtureHelloBody = `package hello

import (
	"fmt"

	"github.com/truffle-dev/glyph/components/foo"
)

// Greet composes the foo dependency into a sentence. After install, the
// import path must point at the consumer's module.
func Greet() string { return fmt.Sprintf("%s world", foo.Hello()) }
`

func newFixtureRegistry(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	fooItem := RegistryItem{
		Schema:      "test",
		Name:        "foo",
		Type:        "glyph:component",
		Title:       "Foo",
		Description: "Tiny dependency-free fixture component used by hello.",
		Frame:       "bubbletea",
		Files: []RegistryFile{
			{
				Path:   "components/foo/foo.go",
				Type:   "glyph:component",
				Target: "@components/foo/foo.go",
			},
		},
	}
	helloItem := RegistryItem{
		Schema:               "test",
		Name:                 "hello",
		Type:                 "glyph:component",
		Title:                "Hello",
		Description:          "Fixture component that imports foo. Verifies import rewriting.",
		Frame:                "bubbletea",
		RegistryDependencies: []string{"foo"},
		Files: []RegistryFile{
			{
				Path:   "components/hello/hello.go",
				Type:   "glyph:component",
				Target: "@components/hello/hello.go",
			},
		},
	}
	catalog := RegistryCatalog{
		Schema:   "test",
		Name:     "fixture",
		Homepage: "https://example.com",
		Items:    []RegistryItem{fooItem, helloItem},
	}

	mux.HandleFunc("/registry.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(catalog)
	})
	mux.HandleFunc("/foo.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fooItem)
	})
	mux.HandleFunc("/hello.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(helloItem)
	})
	mux.HandleFunc("/foo/foo.go", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, fixtureFooBody)
	})
	mux.HandleFunc("/hello/hello.go", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, fixtureHelloBody)
	})

	return httptest.NewServer(mux)
}

// inSandbox chdirs into a temp project root, restoring the original
// working directory when the test ends. Returns the project root.
func inSandbox(t *testing.T) string {
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

func TestInitWritesConfigAndAliasDirs(t *testing.T) {
	proj := inSandbox(t)

	if code := runInit([]string{
		"-y",
		"-module", "example.com/myapp",
		"-components", "internal/ui",
		"-lib", "internal/uilib",
	}); code != 0 {
		t.Fatalf("init exit code %d", code)
	}

	cfgBytes, err := os.ReadFile(filepath.Join(proj, "glyph.json"))
	if err != nil {
		t.Fatalf("glyph.json missing: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		t.Fatalf("parse glyph.json: %v", err)
	}
	if cfg.Module != "example.com/myapp" {
		t.Errorf("module = %q, want example.com/myapp", cfg.Module)
	}
	if cfg.Aliases["components"] != "internal/ui" {
		t.Errorf("components alias = %q, want internal/ui", cfg.Aliases["components"])
	}
	if cfg.Aliases["lib"] != "internal/uilib" {
		t.Errorf("lib alias = %q, want internal/uilib", cfg.Aliases["lib"])
	}
	if cfg.Frame != "bubbletea" {
		t.Errorf("frame = %q, want bubbletea", cfg.Frame)
	}

	for _, d := range []string{"internal/ui", "internal/uilib"} {
		full := filepath.Join(proj, d)
		info, err := os.Stat(full)
		if err != nil {
			t.Errorf("alias dir %s not created: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("alias path %s is not a directory", d)
		}
	}
}

func TestInitRefusesToOverwriteExistingConfig(t *testing.T) {
	proj := inSandbox(t)
	if err := os.WriteFile(filepath.Join(proj, "glyph.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if code := runInit([]string{"-y"}); code == 0 {
		t.Fatal("init should refuse to overwrite an existing glyph.json")
	}
}

func TestAddResolvesRegistryDependenciesAndRewritesImports(t *testing.T) {
	proj := inSandbox(t)
	srv := newFixtureRegistry(t)
	t.Cleanup(srv.Close)

	if code := runInit([]string{
		"-y",
		"-module", "example.com/myapp",
		"-components", "internal/ui",
		"-lib", "internal/uilib",
	}); code != 0 {
		t.Fatalf("init exit code %d", code)
	}

	if code := runAdd([]string{"-y", "-registry", srv.URL, "hello"}); code != 0 {
		t.Fatalf("add exit code %d", code)
	}

	// foo must be installed first (resolved as a registryDependency of hello).
	fooPath := filepath.Join(proj, "internal/ui/foo/foo.go")
	if _, err := os.Stat(fooPath); err != nil {
		t.Errorf("foo not installed at %s: %v", fooPath, err)
	}

	helloPath := filepath.Join(proj, "internal/ui/hello/hello.go")
	helloBytes, err := os.ReadFile(helloPath)
	if err != nil {
		t.Fatalf("hello not installed at %s: %v", helloPath, err)
	}
	wantImport := []byte("example.com/myapp/internal/ui/foo")
	if !bytes.Contains(helloBytes, wantImport) {
		t.Errorf("import not rewritten to consumer module; got:\n%s", helloBytes)
	}
	if bytes.Contains(helloBytes, []byte("github.com/truffle-dev/glyph/components/foo")) {
		t.Errorf("source import still present after rewrite; got:\n%s", helloBytes)
	}
}

func TestAddRefusesOverwriteWithoutForce(t *testing.T) {
	proj := inSandbox(t)
	srv := newFixtureRegistry(t)
	t.Cleanup(srv.Close)

	if code := runInit([]string{"-y", "-module", "example.com/myapp", "-components", "internal/ui"}); code != 0 {
		t.Fatalf("init exit code %d", code)
	}
	if code := runAdd([]string{"-y", "-registry", srv.URL, "foo"}); code != 0 {
		t.Fatalf("first add exit code %d", code)
	}
	// Mutate the installed file so we can detect whether a second add overwrote it.
	fooPath := filepath.Join(proj, "internal/ui/foo/foo.go")
	if err := os.WriteFile(fooPath, []byte("// preserved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runAdd([]string{"-y", "-registry", srv.URL, "foo"}); code == 0 {
		t.Error("second add should refuse to overwrite without -f")
	}
	body, err := os.ReadFile(fooPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "// preserved") {
		t.Errorf("file was overwritten despite refusal; got:\n%s", body)
	}
}

func TestListPrintsCatalogFromCustomRegistry(t *testing.T) {
	inSandbox(t)
	srv := newFixtureRegistry(t)
	t.Cleanup(srv.Close)

	// Capture stdout for the duration of runList.
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	exit := runList([]string{"-registry", srv.URL})
	_ = w.Close()
	os.Stdout = origStdout

	if exit != 0 {
		t.Fatalf("list exit code %d", exit)
	}
	out, _ := io.ReadAll(r)
	got := string(out)
	for _, name := range []string{"foo", "hello"} {
		if !strings.Contains(got, name) {
			t.Errorf("list output missing %q; got:\n%s", name, got)
		}
	}
}

func TestResolveAliasUnknownAliasErrors(t *testing.T) {
	c := defaultConfig()
	if _, err := c.resolveAlias("@nope/x.go"); err == nil {
		t.Fatal("resolveAlias must reject unknown alias")
	}
}

func TestResolveAliasPassthroughForNonAlias(t *testing.T) {
	c := defaultConfig()
	got, err := c.resolveAlias("plain/path/x.go")
	if err != nil {
		t.Fatal(err)
	}
	if got != "plain/path/x.go" {
		t.Errorf("non-alias path got rewritten: %q", got)
	}
}
