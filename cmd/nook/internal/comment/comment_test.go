package comment

import (
	"reflect"
	"testing"
)

func TestPrefixKnownExtensions(t *testing.T) {
	cases := map[string]string{
		"x.go":                   "// ",
		"foo/bar.ts":             "// ",
		"deep/nested/main.rs":    "// ",
		"a.js":                   "// ",
		"a.jsx":                  "// ",
		"a.tsx":                  "// ",
		"a.zig":                  "// ",
		"a.c":                    "// ",
		"a.cpp":                  "// ",
		"a.java":                 "// ",
		"a.swift":                "// ",
		"script.py":              "# ",
		"deploy.sh":              "# ",
		"Config.toml":            "# ",
		"docker-compose.yaml":    "# ",
		"k8s.yml":                "# ",
		"main.lua":               "-- ",
		"q.sql":                  "-- ",
		"theory.hs":              "-- ",
		"colors.vim":             "\" ",
		"util.el":                "; ",
		"core.clj":               "; ",
		"paper.tex":              "% ",
		"Makefile":               "# ",
		"Dockerfile":             "# ",
		"path/with/many/Gemfile": "# ",
	}
	for path, want := range cases {
		if got := Prefix(path); got != want {
			t.Errorf("Prefix(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestPrefixUnknownReturnsEmpty(t *testing.T) {
	for _, path := range []string{
		"page.html",       // block comments only
		"styles.css",      // block comments only
		"data.json",       // no canonical comment
		"unknown.xyz",     // not registered
		"BinaryFileNoExt", // not a known basename
	} {
		if got := Prefix(path); got != "" {
			t.Errorf("Prefix(%q) = %q, want empty (unsupported)", path, got)
		}
	}
}

func TestToggleLinesCommentsFreshRows(t *testing.T) {
	in := []string{
		"x := 1",
		"y := 2",
	}
	out, op := ToggleLines(in, "// ")
	if op != Comment {
		t.Errorf("op = %v, want Comment", op)
	}
	want := []string{
		"// x := 1",
		"// y := 2",
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("out = %#v, want %#v", out, want)
	}
}

func TestToggleLinesUncommentsRows(t *testing.T) {
	in := []string{
		"// x := 1",
		"// y := 2",
	}
	out, op := ToggleLines(in, "// ")
	if op != Uncomment {
		t.Errorf("op = %v, want Uncomment", op)
	}
	want := []string{
		"x := 1",
		"y := 2",
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("out = %#v, want %#v", out, want)
	}
}

func TestToggleLinesIsIdempotentRoundTrip(t *testing.T) {
	original := []string{
		"\tfoo := 1",
		"\tbar := 2",
		"",
		"\tbaz := 3",
	}
	commented, op := ToggleLines(original, "// ")
	if op != Comment {
		t.Fatalf("forward op = %v, want Comment", op)
	}
	back, op := ToggleLines(commented, "// ")
	if op != Uncomment {
		t.Fatalf("reverse op = %v, want Uncomment", op)
	}
	if !reflect.DeepEqual(back, original) {
		t.Errorf("round-trip diff:\n got  %#v\n want %#v", back, original)
	}
}

func TestToggleLinesPreservesBlankLinesInBothDirections(t *testing.T) {
	in := []string{
		"x := 1",
		"",
		"y := 2",
	}
	out, op := ToggleLines(in, "// ")
	if op != Comment {
		t.Errorf("op = %v, want Comment", op)
	}
	want := []string{
		"// x := 1",
		"",
		"// y := 2",
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("out = %#v, want %#v", out, want)
	}

	uncomm, op := ToggleLines(want, "// ")
	if op != Uncomment {
		t.Errorf("op = %v, want Uncomment", op)
	}
	if !reflect.DeepEqual(uncomm, in) {
		t.Errorf("round-trip with blank produced %#v, want %#v", uncomm, in)
	}
}

func TestToggleLinesUncommentsMixedBlankAndCommented(t *testing.T) {
	in := []string{
		"// x",
		"",
		"// y",
	}
	out, op := ToggleLines(in, "// ")
	if op != Uncomment {
		t.Errorf("op = %v, want Uncomment", op)
	}
	want := []string{
		"x",
		"",
		"y",
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("out = %#v, want %#v", out, want)
	}
}

func TestToggleLinesCommentsWhenAnyLineIsUncommented(t *testing.T) {
	in := []string{
		"// already",
		"fresh",
	}
	out, op := ToggleLines(in, "// ")
	if op != Comment {
		t.Errorf("op = %v, want Comment", op)
	}
	want := []string{
		"// // already",
		"// fresh",
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("out = %#v, want %#v", out, want)
	}
}

func TestToggleLinesIndentedBlockAnchorsAtMinIndent(t *testing.T) {
	in := []string{
		"    x := 1",
		"        nested",
		"    y := 2",
	}
	out, op := ToggleLines(in, "// ")
	if op != Comment {
		t.Errorf("op = %v, want Comment", op)
	}
	// All three rows share the same comment column (min indent = 4
	// spaces). Inner indent is preserved beyond the prefix.
	want := []string{
		"    // x := 1",
		"    // " + "    nested",
		"    // y := 2",
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("out = %#v, want %#v", out, want)
	}
}

func TestToggleLinesAllBlankIsNoop(t *testing.T) {
	in := []string{"", "  ", "\t"}
	out, op := ToggleLines(in, "// ")
	if op != Noop {
		t.Errorf("op = %v, want Noop", op)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("out = %#v, want %#v", out, in)
	}
}

func TestToggleLinesEmptyInputIsNoop(t *testing.T) {
	out, op := ToggleLines(nil, "// ")
	if op != Noop {
		t.Errorf("op = %v, want Noop", op)
	}
	if len(out) != 0 {
		t.Errorf("out len = %d, want 0", len(out))
	}
}

func TestToggleLinesEmptyPrefixIsNoop(t *testing.T) {
	in := []string{"x", "y"}
	out, op := ToggleLines(in, "")
	if op != Noop {
		t.Errorf("op = %v, want Noop", op)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("out = %#v, want %#v", out, in)
	}
}

func TestToggleLinesDoesNotMutateInput(t *testing.T) {
	in := []string{"x", "y", "z"}
	snapshot := append([]string(nil), in...)
	_, _ = ToggleLines(in, "// ")
	if !reflect.DeepEqual(in, snapshot) {
		t.Errorf("input was mutated: in=%#v snapshot=%#v", in, snapshot)
	}
}

func TestToggleLinesHashPrefixForPython(t *testing.T) {
	in := []string{
		"def f():",
		"    pass",
	}
	out, op := ToggleLines(in, Prefix("a.py"))
	if op != Comment {
		t.Errorf("op = %v, want Comment", op)
	}
	want := []string{
		"# def f():",
		"# " + "    pass",
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("out = %#v, want %#v", out, want)
	}
}
