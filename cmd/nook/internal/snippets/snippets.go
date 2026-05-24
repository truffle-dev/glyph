// Package snippets is nook's VSCode-compatible snippet engine.
//
// A snippet is a named text body with placeholders. Users type a prefix
// in the editor and hit Alt+J; nook looks the prefix up in the active
// buffer's language scope, expands the body, and drops the editor into
// snippet mode where Tab cycles between the placeholders.
//
// The body grammar mirrors VSCode's snippet syntax for forward
// portability: `$1, $2, ... $9` are tab stops, `${1:default}` carry a
// default value the cursor lands on selected (text first, then the
// next Tab leaves it in place), `$0` is the final cursor target after
// the last Tab, and a small set of variables are resolved at expand
// time: `$TM_FILENAME`, `$TM_FILENAME_BASE`, `$CURRENT_YEAR`,
// `$CURRENT_DATE`. Choice tabstops `${1|a,b,c|}`, nested placeholders,
// and the rest of the VSCode variable namespace are deferred — the
// shape above covers ~95% of snippets in the wild.
package snippets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Tabstop marks one inserted placeholder inside an expanded body.
// Offset is the byte offset of the placeholder's start in Text;
// Length is the byte length of any default text already written
// at that offset.
type Tabstop struct {
	Index  int
	Offset int
	Length int
}

// Expansion is the output of Expand: the resolved text, the ordered
// list of tab stops in order of first occurrence by Index, and the
// optional final cursor target ($0). When Final is nil, the cursor
// lands at end-of-text after the last tab stop is exited.
type Expansion struct {
	Text     string
	Tabstops []Tabstop
	Final    *Tabstop
}

// Snippet is one declared expansion entry. Scope is the language tag
// the snippet applies to ("go", "ts", "py", "md", or "*" for any).
type Snippet struct {
	Name        string
	Prefix      string
	Body        string
	Description string
	Scope       string
}

// Variables is the set of $-variables resolved at expand time. Empty
// values are left in place as the variable name; callers should fill
// the file-relative names before calling Expand.
type Variables struct {
	Filename     string
	FilenameBase string
	CurrentYear  string
	CurrentDate  string
}

// DefaultVariables returns the time-based variables filled to now.
// Callers blend the file-relative names on top.
func DefaultVariables() Variables {
	now := time.Now()
	return Variables{
		CurrentYear: fmt.Sprintf("%04d", now.Year()),
		CurrentDate: now.Format("2006-01-02"),
	}
}

// Expand walks the body, resolving variables and recording tabstops,
// and returns the inserted text plus the tabstop layout the editor
// needs to navigate it. Unrecognised escapes (`\\$`) pass through as
// literal `$`. Unknown variables (`$FOO`) pass through unchanged so
// the body is never silently mangled.
func Expand(body string, vars Variables) (Expansion, error) {
	var out strings.Builder
	stops := map[int]*Tabstop{}
	var final *Tabstop
	var orderedIndices []int

	i := 0
	for i < len(body) {
		c := body[i]

		// `\\$` and `\\}` and `\\\\` escapes — emit the literal,
		// skip the backslash. Other backslashes pass through.
		if c == '\\' && i+1 < len(body) {
			next := body[i+1]
			if next == '$' || next == '\\' || next == '}' {
				out.WriteByte(next)
				i += 2
				continue
			}
		}

		if c != '$' {
			out.WriteByte(c)
			i++
			continue
		}

		// Past here: c == '$'.
		if i+1 >= len(body) {
			// Trailing `$` — emit literally.
			out.WriteByte(c)
			i++
			continue
		}

		next := body[i+1]

		// `$\\d+` plain tabstop.
		if next >= '0' && next <= '9' {
			end := i + 2
			for end < len(body) && body[end] >= '0' && body[end] <= '9' {
				end++
			}
			idx, _ := parseIntStrict(body[i+1 : end])
			ts := Tabstop{Index: idx, Offset: out.Len(), Length: 0}
			if idx == 0 {
				stop := ts
				final = &stop
			} else {
				if _, seen := stops[idx]; !seen {
					orderedIndices = append(orderedIndices, idx)
				}
				stops[idx] = &ts
			}
			i = end
			continue
		}

		// `${...}` form — placeholder with default OR variable.
		if next == '{' {
			closeIdx := findMatchingBrace(body, i+1)
			if closeIdx < 0 {
				// Unbalanced — emit literally and move on.
				out.WriteByte(c)
				i++
				continue
			}
			inner := body[i+2 : closeIdx]
			i = closeIdx + 1

			if len(inner) > 0 && inner[0] >= '0' && inner[0] <= '9' {
				colon := strings.IndexByte(inner, ':')
				var idxStr string
				var defText string
				if colon < 0 {
					idxStr = inner
				} else {
					idxStr = inner[:colon]
					defText = inner[colon+1:]
				}
				if !allDigits(idxStr) {
					out.WriteString("${" + inner + "}")
					continue
				}
				idx, _ := parseIntStrict(idxStr)
				start := out.Len()
				out.WriteString(defText)
				length := out.Len() - start
				ts := Tabstop{Index: idx, Offset: start, Length: length}
				if idx == 0 {
					stop := ts
					final = &stop
				} else {
					if _, seen := stops[idx]; !seen {
						orderedIndices = append(orderedIndices, idx)
					}
					stops[idx] = &ts
				}
				continue
			}

			// Variable in `${VAR}` form.
			if val, ok := resolveVariable(inner, vars); ok {
				out.WriteString(val)
				continue
			}
			// Unknown — keep literally so the user can see it.
			out.WriteString("${" + inner + "}")
			continue
		}

		// `$VAR` form — identifier variable.
		end := i + 1
		for end < len(body) && isVarRune(rune(body[end])) {
			end++
		}
		name := body[i+1 : end]
		if val, ok := resolveVariable(name, vars); ok {
			out.WriteString(val)
			i = end
			continue
		}
		out.WriteString(body[i:end])
		i = end
	}

	sort.Ints(orderedIndices)
	tabstops := make([]Tabstop, 0, len(orderedIndices))
	for _, idx := range orderedIndices {
		if ts := stops[idx]; ts != nil {
			tabstops = append(tabstops, *ts)
		}
	}

	return Expansion{
		Text:     out.String(),
		Tabstops: tabstops,
		Final:    final,
	}, nil
}

// Library holds parsed snippets keyed by scope. The empty value is
// ready to use; LoadDefaults seeds it with the built-in shipped set.
type Library struct {
	bySnippet map[string][]Snippet
}

// NewLibrary returns an empty library.
func NewLibrary() Library {
	return Library{bySnippet: map[string][]Snippet{}}
}

// Add inserts a snippet under its declared scope.
func (l *Library) Add(s Snippet) {
	if l.bySnippet == nil {
		l.bySnippet = map[string][]Snippet{}
	}
	scope := s.Scope
	if scope == "" {
		scope = "*"
	}
	l.bySnippet[scope] = append(l.bySnippet[scope], s)
}

// Lookup returns every snippet from the global scope `*` plus the
// requested scope whose prefix matches. Exact-prefix-equal is the
// match rule for first-slice simplicity; substring/fuzzy can layer
// on later without changing the call site.
func (l Library) Lookup(scope, prefix string) []Snippet {
	if l.bySnippet == nil {
		return nil
	}
	if prefix == "" {
		return nil
	}
	var out []Snippet
	for _, s := range l.bySnippet["*"] {
		if s.Prefix == prefix {
			out = append(out, s)
		}
	}
	if scope != "" && scope != "*" {
		for _, s := range l.bySnippet[scope] {
			if s.Prefix == prefix {
				out = append(out, s)
			}
		}
	}
	return out
}

// Count returns the total number of snippets in the library across
// all scopes.
func (l Library) Count() int {
	if l.bySnippet == nil {
		return 0
	}
	n := 0
	for _, ss := range l.bySnippet {
		n += len(ss)
	}
	return n
}

// Scopes returns the sorted list of non-empty scope tags present.
func (l Library) Scopes() []string {
	if l.bySnippet == nil {
		return nil
	}
	out := make([]string, 0, len(l.bySnippet))
	for k := range l.bySnippet {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ScopeFor maps a path's extension to a snippet-library scope tag.
// Unrecognised extensions return "" — only `*` snippets apply.
func ScopeFor(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return "ts"
	case ".py":
		return "py"
	case ".md", ".markdown":
		return "md"
	case ".rs":
		return "rs"
	}
	return ""
}

// vsCodeFile is the on-disk JSON shape: { "<name>": { prefix, body,
// description } }. body can be a string or string array (each entry
// is one line, joined with \n on parse).
type vsCodeFile map[string]struct {
	Prefix      string          `json:"prefix"`
	Body        json.RawMessage `json:"body"`
	Description string          `json:"description"`
}

// LoadFile reads a VSCode-format snippet JSON file and adds its
// entries to the library under the given scope. Missing files are
// not an error — the caller already chose to look there.
func (l *Library) LoadFile(path, scope string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read snippet file %s: %w", path, err)
	}
	var raw vsCodeFile
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("parse snippet file %s: %w", path, err)
	}
	for name, entry := range raw {
		bodyStr, err := decodeBody(entry.Body)
		if err != nil {
			return fmt.Errorf("snippet %q in %s: %w", name, path, err)
		}
		l.Add(Snippet{
			Name:        name,
			Prefix:      entry.Prefix,
			Body:        bodyStr,
			Description: entry.Description,
			Scope:       scope,
		})
	}
	return nil
}

// LoadDir walks dir for `<scope>.json` files and merges each into the
// library. Missing dir returns nil. Per-file errors stop the walk so
// the caller sees the first malformed file rather than partial state.
func (l *Library) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read snippet dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		scope := strings.TrimSuffix(name, ".json")
		if scope == "global" {
			scope = "*"
		}
		if err := l.LoadFile(filepath.Join(dir, name), scope); err != nil {
			return err
		}
	}
	return nil
}

func decodeBody(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		return "", fmt.Errorf("body must be string or string array")
	}
	return strings.Join(arr, "\n"), nil
}

// LoadDefaults returns a library seeded with the built-in snippet set.
// The defaults are intentionally small — one or two anchors per
// language so the feature is discoverable without overwriting users'
// muscle memory. Users add or override via ~/.config/nook/snippets/.
func LoadDefaults() Library {
	l := NewLibrary()
	for _, s := range builtins {
		l.Add(s)
	}
	return l
}

var builtins = []Snippet{
	// Go.
	{
		Name: "func", Prefix: "fn", Scope: "go",
		Description: "function declaration",
		Body:        "func ${1:name}(${2:args}) ${3:error} {\n\t$0\n}",
	},
	{
		Name: "if err", Prefix: "iferr", Scope: "go",
		Description: "if err != nil { return err }",
		Body:        "if err != nil {\n\treturn ${1:err}\n}\n$0",
	},
	{
		Name: "test func", Prefix: "tfn", Scope: "go",
		Description: "func Test...(t *testing.T)",
		Body:        "func Test${1:Name}(t *testing.T) {\n\t$0\n}",
	},
	// TypeScript / JavaScript.
	{
		Name: "arrow fn", Prefix: "afn", Scope: "ts",
		Description: "arrow function",
		Body:        "const ${1:name} = (${2:args}) => {\n\t$0\n}",
	},
	{
		Name: "interface", Prefix: "intf", Scope: "ts",
		Description: "interface declaration",
		Body:        "interface ${1:Name} {\n\t$0\n}",
	},
	// Python.
	{
		Name: "def", Prefix: "def", Scope: "py",
		Description: "function definition",
		Body:        "def ${1:name}(${2:args}):\n\t$0",
	},
	{
		Name: "main guard", Prefix: "main", Scope: "py",
		Description: "if __name__ == '__main__'",
		Body:        "if __name__ == \"__main__\":\n\t$0",
	},
	// Markdown.
	{
		Name: "link", Prefix: "link", Scope: "md",
		Description: "markdown link",
		Body:        "[${1:text}](${2:url})$0",
	},
	{
		Name: "fenced code", Prefix: "code", Scope: "md",
		Description: "fenced code block",
		Body:        "```${1:lang}\n$0\n```",
	},
	// Rust.
	{
		Name: "fn", Prefix: "fn", Scope: "rs",
		Description: "function declaration",
		Body:        "fn ${1:name}(${2:args}) -> ${3:()} {\n\t$0\n}",
	},
	// Global.
	{
		Name: "TODO", Prefix: "todo", Scope: "*",
		Description: "TODO comment with author + date",
		Body:        "TODO(${1:truffle}): $0 ($CURRENT_DATE)",
	},
	{
		Name: "FIXME", Prefix: "fixme", Scope: "*",
		Description: "FIXME comment",
		Body:        "FIXME: $0",
	},
}

// PrefixAt returns the trailing word at end of s — the longest run of
// identifier-runes (letter, digit, underscore). Used by the host to
// figure out what to look up when the user fires the expand key.
func PrefixAt(s string) string {
	runes := []rune(s)
	i := len(runes)
	for i > 0 {
		r := runes[i-1]
		if !isIdentRune(r) {
			break
		}
		i--
	}
	return string(runes[i:])
}

func isIdentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isVarRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func parseIntStrict(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// findMatchingBrace scans body starting at the position of an opening
// `{` and returns the position of the matching `}` accounting for
// nesting. Returns -1 if unbalanced.
func findMatchingBrace(body string, openIdx int) int {
	if openIdx >= len(body) || body[openIdx] != '{' {
		return -1
	}
	depth := 1
	for j := openIdx + 1; j < len(body); j++ {
		switch body[j] {
		case '\\':
			j++ // skip the next byte; backslash escapes
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

func resolveVariable(name string, vars Variables) (string, bool) {
	switch name {
	case "TM_FILENAME":
		if vars.Filename != "" {
			return vars.Filename, true
		}
	case "TM_FILENAME_BASE":
		if vars.FilenameBase != "" {
			return vars.FilenameBase, true
		}
	case "CURRENT_YEAR":
		if vars.CurrentYear != "" {
			return vars.CurrentYear, true
		}
	case "CURRENT_DATE":
		if vars.CurrentDate != "" {
			return vars.CurrentDate, true
		}
	}
	return "", false
}
