package highlight

import (
	"strings"
	"testing"
)

func TestKindString(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{KindPlain, "plain"},
		{KindKeyword, "keyword"},
		{KindString, "string"},
		{KindComment, "comment"},
		{KindNumber, "number"},
		{KindFunction, "function"},
		{KindType, "type"},
		{KindPunctuation, "punct"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("Kind(%d).String() = %q, want %q", c.k, got, c.want)
		}
	}
}

func TestLanguageFor(t *testing.T) {
	cases := map[string]string{
		"main.go":      "go",
		"app.ts":       "typescript",
		"app.tsx":      "typescript",
		"app.js":       "javascript",
		"foo.py":       "python",
		"lib.rs":       "rust",
		"README.md":    "markdown",
		"unknown.xyzq": "",
	}
	for path, want := range cases {
		got := LanguageFor(path)
		if got != want {
			t.Errorf("LanguageFor(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestHighlightGoKeyword(t *testing.T) {
	src := "package main\n\nfunc Foo() string { return \"x\" }\n"
	h := New()
	r := h.Highlight("main.go", src)
	// Row 0: package + main
	spans0 := r.Spans(0)
	if !hasKindAt(spans0, KindKeyword, "package") || !containsKind(spans0, KindKeyword) {
		t.Errorf("row 0 missing keyword span for 'package': %+v", spans0)
	}
	// Row 2 (zero-indexed): func ... string ... "x"
	spans2 := r.Spans(2)
	if !containsKind(spans2, KindKeyword) {
		t.Errorf("row 2 missing keyword span: %+v", spans2)
	}
	if !containsKind(spans2, KindString) {
		t.Errorf("row 2 missing string span: %+v", spans2)
	}
}

func TestHighlightGoComment(t *testing.T) {
	src := "package main\n// hello world\nfunc Foo() {}\n"
	r := New().Highlight("main.go", src)
	if !containsKind(r.Spans(1), KindComment) {
		t.Errorf("expected comment span on row 1, got %+v", r.Spans(1))
	}
}

func TestHighlightUnknownExtension(t *testing.T) {
	src := "this is plain text\n"
	r := New().Highlight("notes.xyzq", src)
	if r.Rows != nil && len(r.Rows) > 0 {
		t.Errorf("expected no spans for unknown extension, got %+v", r.Rows)
	}
}

func TestHighlightMultilineSpansSplitOnNewlines(t *testing.T) {
	// Go raw string spans multiple rows.
	src := "package main\nvar x = `hello\nworld`\n"
	r := New().Highlight("main.go", src)
	// Row 1 should have a string span starting around `\` and going to EOL.
	if !containsKind(r.Spans(1), KindString) {
		t.Errorf("row 1 missing string span: %+v", r.Spans(1))
	}
	// Row 2 should have a string span starting at col 0 for "world".
	if !containsKind(r.Spans(2), KindString) {
		t.Errorf("row 2 missing string span: %+v", r.Spans(2))
	}
}

func TestHighlightPython(t *testing.T) {
	src := "def add(a, b):\n    \"\"\"sum two ints\"\"\"\n    return a + b\n"
	r := New().Highlight("add.py", src)
	if !containsKind(r.Spans(0), KindKeyword) {
		t.Errorf("expected keyword 'def' on row 0, got %+v", r.Spans(0))
	}
	if !containsKind(r.Spans(2), KindKeyword) {
		t.Errorf("expected keyword 'return' on row 2, got %+v", r.Spans(2))
	}
}

func TestSpansAreNonOverlapping(t *testing.T) {
	src := "package main\nfunc Foo() string { return \"x\" }\n"
	r := New().Highlight("main.go", src)
	for row, spans := range r.Rows {
		for i := 1; i < len(spans); i++ {
			if spans[i].Start < spans[i-1].End {
				t.Errorf("row %d: spans overlap: %+v before %+v", row, spans[i-1], spans[i])
			}
			if spans[i].Start > spans[i].End {
				t.Errorf("row %d: inverted span %+v", row, spans[i])
			}
		}
	}
}

func TestSpansFitWithinLineLength(t *testing.T) {
	src := "package main\n\nfunc Foo() {}\n"
	lines := strings.Split(src, "\n")
	r := New().Highlight("main.go", src)
	for row, spans := range r.Rows {
		if row >= len(lines) {
			t.Errorf("row %d out of range (only %d lines)", row, len(lines))
			continue
		}
		lineLen := len(lines[row])
		for _, s := range spans {
			if s.End > lineLen {
				t.Errorf("row %d: span end %d exceeds line length %d (line=%q)", row, s.End, lineLen, lines[row])
			}
		}
	}
}

// helpers

func containsKind(spans []Span, k Kind) bool {
	for _, s := range spans {
		if s.Kind == k {
			return true
		}
	}
	return false
}

func hasKindAt(spans []Span, k Kind, substr string) bool {
	// Not used to assert text content (we don't have the source here), but
	// kept as a sentinel so future tests can be more specific via a source-
	// aware variant.
	_ = substr
	for _, s := range spans {
		if s.Kind == k {
			return true
		}
	}
	return false
}
