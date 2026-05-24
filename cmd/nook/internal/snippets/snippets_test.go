package snippets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandPlainBody(t *testing.T) {
	exp, err := Expand("hello", Variables{})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if exp.Text != "hello" {
		t.Fatalf("text=%q want hello", exp.Text)
	}
	if len(exp.Tabstops) != 0 || exp.Final != nil {
		t.Fatalf("plain body should have no tabstops, got %+v final=%v", exp.Tabstops, exp.Final)
	}
}

func TestExpandNumericTabstops(t *testing.T) {
	exp, err := Expand("a$1b$2c", Variables{})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if exp.Text != "abc" {
		t.Fatalf("text=%q want abc", exp.Text)
	}
	if len(exp.Tabstops) != 2 {
		t.Fatalf("want 2 tabstops, got %d", len(exp.Tabstops))
	}
	if exp.Tabstops[0].Index != 1 || exp.Tabstops[0].Offset != 1 || exp.Tabstops[0].Length != 0 {
		t.Fatalf("ts[0]=%+v", exp.Tabstops[0])
	}
	if exp.Tabstops[1].Index != 2 || exp.Tabstops[1].Offset != 2 || exp.Tabstops[1].Length != 0 {
		t.Fatalf("ts[1]=%+v", exp.Tabstops[1])
	}
}

func TestExpandPlaceholderWithDefault(t *testing.T) {
	exp, err := Expand("greet ${1:world}", Variables{})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if exp.Text != "greet world" {
		t.Fatalf("text=%q", exp.Text)
	}
	if len(exp.Tabstops) != 1 {
		t.Fatalf("want 1 tabstop, got %d", len(exp.Tabstops))
	}
	ts := exp.Tabstops[0]
	if ts.Index != 1 || ts.Offset != 6 || ts.Length != 5 {
		t.Fatalf("ts=%+v", ts)
	}
}

func TestExpandFinalTabstop(t *testing.T) {
	exp, err := Expand("a$1b$0c", Variables{})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if exp.Text != "abc" {
		t.Fatalf("text=%q", exp.Text)
	}
	if exp.Final == nil {
		t.Fatal("Final should be set when $0 present")
	}
	if exp.Final.Index != 0 || exp.Final.Offset != 2 {
		t.Fatalf("final=%+v", exp.Final)
	}
	if len(exp.Tabstops) != 1 {
		t.Fatalf("want 1 numbered tabstop, got %d", len(exp.Tabstops))
	}
}

func TestExpandFinalWithDefault(t *testing.T) {
	exp, err := Expand("${0:cursor}", Variables{})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if exp.Text != "cursor" {
		t.Fatalf("text=%q", exp.Text)
	}
	if exp.Final == nil {
		t.Fatal("Final missing")
	}
	if exp.Final.Length != 6 || exp.Final.Offset != 0 {
		t.Fatalf("final=%+v", exp.Final)
	}
}

func TestExpandOrdersTabstopsByIndex(t *testing.T) {
	exp, err := Expand("$3a$1b$2", Variables{})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(exp.Tabstops) != 3 {
		t.Fatalf("want 3, got %d", len(exp.Tabstops))
	}
	if exp.Tabstops[0].Index != 1 || exp.Tabstops[1].Index != 2 || exp.Tabstops[2].Index != 3 {
		t.Fatalf("order wrong: %+v", exp.Tabstops)
	}
}

func TestExpandResolvesVariables(t *testing.T) {
	exp, err := Expand("file: $TM_FILENAME ($CURRENT_YEAR-$CURRENT_DATE)", Variables{
		Filename:    "main.go",
		CurrentYear: "2026",
		CurrentDate: "2026-05-24",
	})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := "file: main.go (2026-2026-05-24)"
	if exp.Text != want {
		t.Fatalf("text=%q want %q", exp.Text, want)
	}
}

func TestExpandUnknownVariablePassesThrough(t *testing.T) {
	exp, err := Expand("$FOO bar", Variables{})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if exp.Text != "$FOO bar" {
		t.Fatalf("text=%q", exp.Text)
	}
}

func TestExpandEscapedDollar(t *testing.T) {
	exp, err := Expand("price: \\$5", Variables{})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if exp.Text != "price: $5" {
		t.Fatalf("text=%q", exp.Text)
	}
	if len(exp.Tabstops) != 0 {
		t.Fatalf("escaped $ should not produce tabstops, got %+v", exp.Tabstops)
	}
}

func TestExpandUnbalancedBrace(t *testing.T) {
	exp, err := Expand("${1:open", Variables{})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if !strings.Contains(exp.Text, "$") {
		t.Fatalf("unbalanced brace should keep literal $, got %q", exp.Text)
	}
}

func TestExpandBracedVariable(t *testing.T) {
	exp, err := Expand("name=${TM_FILENAME_BASE}", Variables{FilenameBase: "main"})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if exp.Text != "name=main" {
		t.Fatalf("text=%q", exp.Text)
	}
}

func TestExpandNewlinesPreserved(t *testing.T) {
	body := "if err != nil {\n\treturn err\n}\n$0"
	exp, err := Expand(body, Variables{})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if !strings.Contains(exp.Text, "\n\treturn err\n") {
		t.Fatalf("newlines lost: %q", exp.Text)
	}
	if exp.Final == nil {
		t.Fatal("Final missing")
	}
}

func TestLibraryLookupExactMatch(t *testing.T) {
	l := NewLibrary()
	l.Add(Snippet{Prefix: "fn", Body: "go body", Scope: "go"})
	l.Add(Snippet{Prefix: "fn", Body: "ts body", Scope: "ts"})
	l.Add(Snippet{Prefix: "todo", Body: "global", Scope: "*"})

	results := l.Lookup("go", "fn")
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Body != "go body" {
		t.Fatalf("got %q", results[0].Body)
	}
}

func TestLibraryLookupIncludesGlobal(t *testing.T) {
	l := NewLibrary()
	l.Add(Snippet{Prefix: "todo", Body: "global", Scope: "*"})
	l.Add(Snippet{Prefix: "todo", Body: "go-specific", Scope: "go"})

	results := l.Lookup("go", "todo")
	if len(results) != 2 {
		t.Fatalf("want global + go, got %d: %+v", len(results), results)
	}
	if results[0].Body != "global" {
		t.Fatalf("global should come first: %+v", results)
	}
}

func TestLibraryLookupUnknownScopeFallsBackToGlobal(t *testing.T) {
	l := NewLibrary()
	l.Add(Snippet{Prefix: "todo", Body: "global", Scope: "*"})
	l.Add(Snippet{Prefix: "fn", Body: "go body", Scope: "go"})

	results := l.Lookup("unknown", "todo")
	if len(results) != 1 || results[0].Body != "global" {
		t.Fatalf("want global match only, got %+v", results)
	}
	if r := l.Lookup("unknown", "fn"); len(r) != 0 {
		t.Fatalf("want no go-scoped match, got %+v", r)
	}
}

func TestLibraryLookupEmptyPrefix(t *testing.T) {
	l := NewLibrary()
	l.Add(Snippet{Prefix: "fn", Body: "x", Scope: "*"})
	if r := l.Lookup("go", ""); r != nil {
		t.Fatalf("empty prefix should return nil, got %+v", r)
	}
}

func TestLibraryLoadFileObjectBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.json")
	body := `{
        "fn": {"prefix": "fn", "body": "func ${1:name}() {\n\t$0\n}", "description": "fn"}
    }`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	l := NewLibrary()
	if err := l.LoadFile(path, "go"); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	results := l.Lookup("go", "fn")
	if len(results) != 1 {
		t.Fatalf("want 1, got %d", len(results))
	}
	if !strings.Contains(results[0].Body, "func ${1:name}") {
		t.Fatalf("body=%q", results[0].Body)
	}
}

func TestLibraryLoadFileArrayBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.json")
	body := `{
        "fn": {"prefix": "fn", "body": ["line a", "line b"]}
    }`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	l := NewLibrary()
	if err := l.LoadFile(path, "go"); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	results := l.Lookup("go", "fn")
	if len(results) != 1 {
		t.Fatalf("want 1, got %d", len(results))
	}
	if results[0].Body != "line a\nline b" {
		t.Fatalf("body=%q", results[0].Body)
	}
}

func TestLibraryLoadFileMissingIsNotError(t *testing.T) {
	l := NewLibrary()
	if err := l.LoadFile("/no/such/file.json", "go"); err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
}

func TestLibraryLoadFileMalformedErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	l := NewLibrary()
	if err := l.LoadFile(path, "go"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLibraryLoadDirReadsScopedFiles(t *testing.T) {
	dir := t.TempDir()
	must := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	must("go.json", `{"fn":{"prefix":"fn","body":"go-body"}}`)
	must("ts.json", `{"fn":{"prefix":"fn","body":"ts-body"}}`)
	must("global.json", `{"todo":{"prefix":"todo","body":"global-body"}}`)
	must("ignored.txt", "unused")

	l := NewLibrary()
	if err := l.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if l.Count() != 3 {
		t.Fatalf("want 3 entries, got %d", l.Count())
	}
	if r := l.Lookup("go", "fn"); len(r) != 1 || r[0].Body != "go-body" {
		t.Fatalf("go fn lookup: %+v", r)
	}
	if r := l.Lookup("ts", "todo"); len(r) != 1 || r[0].Body != "global-body" {
		t.Fatalf("ts todo lookup: %+v", r)
	}
}

func TestLibraryLoadDirMissingNoError(t *testing.T) {
	l := NewLibrary()
	if err := l.LoadDir("/no/such/dir"); err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
}

func TestScopeFor(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"index.ts", "ts"},
		{"index.TSX", "ts"},
		{"foo.py", "py"},
		{"README.md", "md"},
		{"notes.MARKDOWN", "md"},
		{"lib.rs", "rs"},
		{"Dockerfile", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := ScopeFor(c.path); got != c.want {
			t.Errorf("ScopeFor(%q)=%q want %q", c.path, got, c.want)
		}
	}
}

func TestPrefixAt(t *testing.T) {
	cases := map[string]string{
		"":          "",
		"foo":       "foo",
		"  foo":     "foo",
		"foo bar":   "bar",
		"x.foo":     "foo",
		"foo123":    "foo123",
		"foo_bar":   "foo_bar",
		"trailing ": "",
	}
	for in, want := range cases {
		if got := PrefixAt(in); got != want {
			t.Errorf("PrefixAt(%q)=%q want %q", in, got, want)
		}
	}
}

func TestLoadDefaultsIsNonEmptyAndCoversCommonScopes(t *testing.T) {
	l := LoadDefaults()
	if l.Count() < 5 {
		t.Fatalf("defaults too small: %d", l.Count())
	}
	scopes := l.Scopes()
	have := map[string]bool{}
	for _, s := range scopes {
		have[s] = true
	}
	for _, want := range []string{"go", "ts", "py", "md", "*"} {
		if !have[want] {
			t.Errorf("defaults missing scope %q (got %v)", want, scopes)
		}
	}
	// Sanity: every default body must parse.
	for _, scope := range scopes {
		for _, s := range builtins {
			if _, err := Expand(s.Body, DefaultVariables()); err != nil {
				t.Errorf("default %s (scope %s) failed to expand: %v", s.Name, scope, err)
			}
		}
	}
}

func TestDefaultVariablesFilled(t *testing.T) {
	v := DefaultVariables()
	if v.CurrentYear == "" || len(v.CurrentYear) != 4 {
		t.Errorf("CurrentYear=%q", v.CurrentYear)
	}
	if v.CurrentDate == "" || len(v.CurrentDate) != 10 {
		t.Errorf("CurrentDate=%q", v.CurrentDate)
	}
}
