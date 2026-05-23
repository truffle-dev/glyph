// Package codeview renders a single block of source code with optional
// line numbers, gutter highlights, and a small built-in tokenizer that
// tints keywords, strings, numbers, and comments.
//
// The package is stateless. Callers build a Block, call Render, and place
// the result inside their own View(). Compose Block.Line with .Highlight,
// .Underline, or .Gutter to mark up specific rows (current cursor line,
// diff hunks, failing-test stack frame, etc.).
//
// The tokenizer is intentionally tiny. It is a "good enough" lexer for
// quick reads: Go, JavaScript/TypeScript, Python, Rust, JSON, Bash, and
// any language that uses // or # line comments and double-quoted strings.
// Production-quality syntax highlighting belongs in a separate component
// that wraps a real lexer (chroma, tree-sitter) — codeview's bar is to
// make a peek-into-a-file surface readable inside an operator dashboard.
package codeview

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Language selects the tokenizer dialect. Unknown languages fall through
// to LangPlain which only highlights numbers and string literals.
type Language string

const (
	LangPlain Language = ""
	LangGo    Language = "go"
	LangJS    Language = "js"
	LangTS    Language = "ts"
	LangPy    Language = "py"
	LangRust  Language = "rust"
	LangJSON  Language = "json"
	LangBash  Language = "bash"
)

// LineMark tags one source line for special rendering.
type LineMark int

const (
	MarkNone      LineMark = iota
	MarkHighlight          // current/focus line — surface-strong background
	MarkAdded              // diff: line added — success-tinted gutter
	MarkRemoved            // diff: line removed — error-tinted gutter
	MarkWarning            // call out a line without diff semantics
	MarkError              // call out a failing line
)

// Block is the input to Render. Mark a subset of lines via Marks; keys
// are 1-based line numbers matching the rendered gutter.
type Block struct {
	Source     string
	Language   Language
	ShowGutter bool // render line-number gutter
	StartLine  int  // 1-based line number for the first row; 0 means 1
	Marks      map[int]LineMark
	MaxWidth   int // 0 means no truncation; otherwise hard-wrap with ellipsis
}

// Tokenize returns a single styled line for the given language without any
// gutter, mark, or row chrome. Intended for components that own their own
// row layout (an editable buffer with its own cursor) and only want to
// reuse the tokenizer. Empty line returns "".
func Tokenize(line string, lang Language) string {
	if line == "" {
		return ""
	}
	return tokenizerFor(lang)(line)
}

// Render returns the multi-line styled string. Empty source returns "".
func Render(b Block) string {
	if b.Source == "" {
		return ""
	}
	start := b.StartLine
	if start <= 0 {
		start = 1
	}

	lines := strings.Split(strings.TrimRight(b.Source, "\n"), "\n")
	gutterW := 0
	if b.ShowGutter {
		gutterW = len(strconv.Itoa(start + len(lines) - 1))
	}

	tokenize := tokenizerFor(b.Language)

	out := make([]string, 0, len(lines))
	for i, raw := range lines {
		lineNum := start + i
		mark := MarkNone
		if b.Marks != nil {
			mark = b.Marks[lineNum]
		}

		body := tokenize(raw)
		if b.MaxWidth > 0 {
			body = truncateRendered(body, raw, b.MaxWidth)
		}

		row := renderRow(body, lineNum, mark, gutterW)
		out = append(out, row)
	}

	return strings.Join(out, "\n")
}

// renderRow assembles gutter + body for one line.
func renderRow(body string, lineNum int, mark LineMark, gutterW int) string {
	gutter := ""
	if gutterW > 0 {
		num := strconv.Itoa(lineNum)
		pad := strings.Repeat(" ", gutterW-len(num))
		gutter = gutterStyle(mark).Render(pad + num + " ")
	}

	rowStyle := bodyStyle(mark)
	return gutter + rowStyle.Render(body)
}

// gutterStyle tints the line-number gutter per mark.
func gutterStyle(mark LineMark) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(theme.Default.TextMuted)
	switch mark {
	case MarkHighlight:
		s = s.Foreground(theme.Default.Text).Background(theme.Default.SurfaceStrong)
	case MarkAdded:
		s = s.Foreground(theme.Default.Success)
	case MarkRemoved:
		s = s.Foreground(theme.Default.Error)
	case MarkWarning:
		s = s.Foreground(theme.Default.Warning)
	case MarkError:
		s = s.Foreground(theme.Default.Error)
	}
	return s
}

// bodyStyle wraps the code body when a line carries a mark. Most marks
// only paint the gutter so the code itself stays readable; highlight
// fills the row background, error/warning underline.
func bodyStyle(mark LineMark) lipgloss.Style {
	s := lipgloss.NewStyle()
	switch mark {
	case MarkHighlight:
		s = s.Background(theme.Default.SurfaceStrong)
	case MarkError:
		s = s.Underline(true)
	}
	return s
}

// --- tokenizer ----------------------------------------------------------

type tokenizer func(string) string

func tokenizerFor(lang Language) tokenizer {
	switch lang {
	case LangGo:
		return tokenizeKeyword(goKeywords, "//", false)
	case LangJS, LangTS:
		return tokenizeKeyword(jsKeywords, "//", false)
	case LangPy:
		return tokenizeKeyword(pyKeywords, "#", false)
	case LangRust:
		return tokenizeKeyword(rustKeywords, "//", false)
	case LangBash:
		return tokenizeKeyword(bashKeywords, "#", false)
	case LangJSON:
		return tokenizeJSON
	default:
		return tokenizePlain
	}
}

// tokenizeKeyword returns a tokenizer that highlights the given keyword
// set, line comments starting with the comment prefix, double-quoted
// strings, and integer/float literals. allowSingleQuote enables Python-
// style single-quoted strings as well.
func tokenizeKeyword(keywords map[string]bool, commentPrefix string, allowSingleQuote bool) tokenizer {
	return func(line string) string {
		// Comment fast-path: tint from prefix to EOL.
		if idx := indexComment(line, commentPrefix); idx >= 0 {
			head := tokenizeBody(line[:idx], keywords, allowSingleQuote)
			tail := commentStyle().Render(line[idx:])
			return head + tail
		}
		return tokenizeBody(line, keywords, allowSingleQuote)
	}
}

// tokenizeBody walks a line splitting it into identifier / number /
// string / other runs, applying styles. Whitespace is preserved verbatim.
func tokenizeBody(line string, keywords map[string]bool, allowSingleQuote bool) string {
	var b strings.Builder
	i := 0
	for i < len(line) {
		c := line[i]

		// Double-quoted string. Walk to the next unescaped quote.
		if c == '"' || (allowSingleQuote && c == '\'') {
			j := i + 1
			for j < len(line) {
				if line[j] == '\\' && j+1 < len(line) {
					j += 2
					continue
				}
				if line[j] == c {
					j++
					break
				}
				j++
			}
			b.WriteString(stringStyle().Render(line[i:j]))
			i = j
			continue
		}

		// Number literal: digits, then optional dot + digits, then optional suffix.
		if c >= '0' && c <= '9' {
			j := i + 1
			for j < len(line) && (isDigit(line[j]) || line[j] == '.' || line[j] == '_') {
				j++
			}
			b.WriteString(numberStyle().Render(line[i:j]))
			i = j
			continue
		}

		// Identifier: letters/digits/underscore. Tint if keyword.
		if isIdentStart(c) {
			j := i + 1
			for j < len(line) && isIdentPart(line[j]) {
				j++
			}
			word := line[i:j]
			if keywords[word] {
				b.WriteString(keywordStyle().Render(word))
			} else {
				b.WriteString(word)
			}
			i = j
			continue
		}

		b.WriteByte(c)
		i++
	}
	return b.String()
}

// tokenizePlain only highlights numbers and string literals.
func tokenizePlain(line string) string {
	return tokenizeBody(line, nil, false)
}

// tokenizeJSON highlights keys, strings, numbers, and the bare literals
// true/false/null.
func tokenizeJSON(line string) string {
	keywords := map[string]bool{"true": true, "false": true, "null": true}
	return tokenizeBody(line, keywords, false)
}

// indexComment reports the byte offset of a line-comment prefix that
// isn't inside a string. Returns -1 when none exists.
func indexComment(line, prefix string) int {
	if prefix == "" {
		return -1
	}
	inString := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inString != 0 {
			if c == '\\' && i+1 < len(line) {
				i++
				continue
			}
			if c == inString {
				inString = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inString = c
			continue
		}
		if i+len(prefix) <= len(line) && line[i:i+len(prefix)] == prefix {
			return i
		}
	}
	return -1
}

// truncateRendered clips an already-styled string at maxWidth visible
// columns by counting runes in the raw (untokenized) line.
func truncateRendered(rendered, raw string, maxWidth int) string {
	rawW := visibleWidth(raw)
	if rawW <= maxWidth {
		return rendered
	}
	// Walk the raw source up to maxWidth-1, then append an ellipsis.
	cut := 0
	w := 0
	for i, r := range raw {
		rw := runeWidth(r)
		if w+rw > maxWidth-1 {
			cut = i
			break
		}
		w += rw
	}
	// Re-tokenize the truncated raw so styles line up.
	return tokenizeBody(raw[:cut], nil, false) + "…"
}

func visibleWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func runeWidth(r rune) int {
	if r == 0 {
		return 0
	}
	if unicode.IsControl(r) {
		return 0
	}
	return 1
}

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isIdentStart(c byte) bool { return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isIdentPart(c byte) bool  { return isIdentStart(c) || isDigit(c) }

// --- styles -------------------------------------------------------------

func keywordStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Default.Primary).Bold(true)
}
func stringStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Default.Success) }
func numberStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Default.Accent) }
func commentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Italic(true)
}

// --- keyword sets -------------------------------------------------------

var goKeywords = setOf("break", "default", "func", "interface", "select", "case", "defer", "go", "map", "struct", "chan", "else", "goto", "package", "switch", "const", "fallthrough", "if", "range", "type", "continue", "for", "import", "return", "var", "true", "false", "nil")

var jsKeywords = setOf("async", "await", "break", "case", "catch", "class", "const", "continue", "debugger", "default", "delete", "do", "else", "export", "extends", "finally", "for", "from", "function", "if", "import", "in", "instanceof", "let", "new", "of", "return", "static", "super", "switch", "this", "throw", "try", "typeof", "var", "void", "while", "with", "yield", "true", "false", "null", "undefined", "interface", "type", "as")

var pyKeywords = setOf("False", "None", "True", "and", "as", "assert", "async", "await", "break", "class", "continue", "def", "del", "elif", "else", "except", "finally", "for", "from", "global", "if", "import", "in", "is", "lambda", "nonlocal", "not", "or", "pass", "raise", "return", "try", "while", "with", "yield")

var rustKeywords = setOf("as", "async", "await", "break", "const", "continue", "crate", "dyn", "else", "enum", "extern", "false", "fn", "for", "if", "impl", "in", "let", "loop", "match", "mod", "move", "mut", "pub", "ref", "return", "self", "Self", "static", "struct", "super", "trait", "true", "type", "unsafe", "use", "where", "while")

var bashKeywords = setOf("if", "then", "else", "elif", "fi", "case", "esac", "for", "while", "do", "done", "in", "function", "return", "exit", "export", "local", "readonly", "declare", "echo", "set", "unset")

func setOf(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it] = true
	}
	return m
}
