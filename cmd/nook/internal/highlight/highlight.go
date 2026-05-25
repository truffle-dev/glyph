// Package highlight tokenizes source files into per-row styled spans for the
// editor pane. The current backend is alecthomas/chroma, chosen because it is
// pure-Go (CGO-free) and covers the languages we care about today: Go,
// TypeScript, JavaScript, Python, Rust, and Markdown. Tree-sitter (via WASM)
// remains a future option behind the same Highlighter interface.
//
// The unit of work is "highlight one buffer," returning a Result that maps
// 0-based row -> []Span. Callers cache the Result keyed by buffer revision
// and re-run on edit. We deliberately re-tokenize the whole buffer each call;
// incremental parsing is a Wave-2 optimization once dogfooding surfaces a
// performance issue.
package highlight

import (
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/truffle-dev/glyph/cmd/nook/internal/semtok"
)

// Kind enumerates the semantic styles we emit. Mapping chroma's wide TokenType
// space onto this small set is intentional: the editor renders runs in
// theme-aware colors, and a 7-bucket palette keeps the screen calm.
type Kind uint8

const (
	KindPlain Kind = iota
	KindKeyword
	KindString
	KindComment
	KindNumber
	KindFunction
	KindType
	KindPunctuation
	KindParameter
	KindProperty
	KindEnumMember
	KindNamespace
	KindReadonly
)

// String returns a stable name for debugging and golden tests.
func (k Kind) String() string {
	switch k {
	case KindKeyword:
		return "keyword"
	case KindString:
		return "string"
	case KindComment:
		return "comment"
	case KindNumber:
		return "number"
	case KindFunction:
		return "function"
	case KindType:
		return "type"
	case KindPunctuation:
		return "punct"
	case KindParameter:
		return "parameter"
	case KindProperty:
		return "property"
	case KindEnumMember:
		return "enum-member"
	case KindNamespace:
		return "namespace"
	case KindReadonly:
		return "readonly"
	default:
		return "plain"
	}
}

// ClassifySemantic maps an LSP semantic-token type name + modifier set into
// one of our Kind buckets. The readonly modifier wins over the base type so
// constants render distinctly from their underlying variable kind. Unknown
// type names resolve to KindPlain so callers don't accidentally upgrade
// untyped runs to a bright color.
func ClassifySemantic(typeName string, modifiers []string) Kind {
	for _, m := range modifiers {
		if m == "readonly" {
			return KindReadonly
		}
	}
	switch typeName {
	case "function", "method", "macro":
		return KindFunction
	case "type", "class", "struct", "interface", "enum", "typeParameter":
		return KindType
	case "parameter":
		return KindParameter
	case "property", "event":
		return KindProperty
	case "enumMember":
		return KindEnumMember
	case "namespace":
		return KindNamespace
	case "keyword", "modifier", "operator":
		return KindKeyword
	case "string", "regexp":
		return KindString
	case "number":
		return KindNumber
	case "comment":
		return KindComment
	}
	return KindPlain
}

// MergeSemantic overlays LSP semantic tokens on top of a chroma highlight
// Result and returns a new Result. The chroma layer is the floor (every byte
// has a Kind from chroma); semantic tokens add or replace the Kind for the
// byte range they cover. Tokens that resolve to KindPlain are dropped so the
// chroma styling shines through. Tokens with Length <= 0 or Line < 0 are
// ignored.
func MergeSemantic(base Result, tokens []semtok.Token) Result {
	if len(tokens) == 0 {
		return base
	}
	out := Result{Rows: make(map[int][]Span, len(base.Rows)+8)}
	for row, spans := range base.Rows {
		clone := make([]Span, len(spans))
		copy(clone, spans)
		out.Rows[row] = clone
	}
	for _, tok := range tokens {
		if tok.Line < 0 || tok.Length <= 0 {
			continue
		}
		kind := ClassifySemantic(tok.Type, tok.Modifiers)
		if kind == KindPlain {
			continue
		}
		out.Rows[tok.Line] = overlaySpan(out.Rows[tok.Line], Span{
			Start: tok.Col,
			End:   tok.Col + tok.Length,
			Kind:  kind,
		})
	}
	return out
}

// overlaySpan splices a new Span into an existing per-row span list, cutting
// or removing the chroma-base spans it covers. The result is sorted by Start
// and non-overlapping, matching the contract Result.Spans guarantees.
func overlaySpan(existing []Span, s Span) []Span {
	if s.End <= s.Start {
		return existing
	}
	out := make([]Span, 0, len(existing)+2)
	inserted := false
	for _, e := range existing {
		// Existing span entirely outside the new span — keep as-is.
		if e.End <= s.Start || e.Start >= s.End {
			out = append(out, e)
			continue
		}
		// Existing span entirely inside the new span — drop it.
		if e.Start >= s.Start && e.End <= s.End {
			continue
		}
		// Existing span crosses the left boundary — truncate its right edge.
		if e.Start < s.Start && e.End > s.Start && e.End <= s.End {
			out = append(out, Span{Start: e.Start, End: s.Start, Kind: e.Kind})
			continue
		}
		// Existing span crosses the right boundary — truncate its left edge.
		if e.Start >= s.Start && e.Start < s.End && e.End > s.End {
			out = append(out, Span{Start: s.End, End: e.End, Kind: e.Kind})
			continue
		}
		// Existing span fully contains the new span — split into left + right.
		if e.Start < s.Start && e.End > s.End {
			out = append(out, Span{Start: e.Start, End: s.Start, Kind: e.Kind})
			out = append(out, Span{Start: s.End, End: e.End, Kind: e.Kind})
		}
	}
	out = append(out, s)
	_ = inserted
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// Span is a contiguous run of bytes on a single line that share a Kind.
// Start is inclusive, End is exclusive, both 0-based byte offsets into the
// raw line text. Spans for a row are sorted and non-overlapping; runs of
// KindPlain are omitted (the editor renders un-spanned bytes with the
// default text color).
type Span struct {
	Start int
	End   int
	Kind  Kind
}

// Result is the highlighted form of a whole buffer. Rows is indexed by
// 0-based row; missing rows mean "nothing to style, render plain."
type Result struct {
	Rows map[int][]Span
}

// Spans returns the (possibly empty) spans for a 0-based row.
func (r Result) Spans(row int) []Span {
	if r.Rows == nil {
		return nil
	}
	return r.Rows[row]
}

// Highlighter tokenizes source text into per-row spans. Implementations should
// be safe to call from a single goroutine.
type Highlighter interface {
	// Highlight returns spans for the given path + source. Path is used for
	// language detection (extension and basename); source is the full buffer.
	Highlight(path, source string) Result
}

// Chroma is the default Highlighter, backed by alecthomas/chroma/v2.
type Chroma struct{}

// New constructs a Chroma highlighter.
func New() Chroma { return Chroma{} }

// Highlight implements Highlighter.
func (Chroma) Highlight(path, source string) Result {
	lexer := lexers.Match(path)
	if lexer == nil {
		// Try to guess from contents (chroma can sniff shebangs, etc.).
		lexer = lexers.Analyse(source)
	}
	if lexer == nil {
		return Result{}
	}
	// Chroma's tokenizer can be slow on pathological inputs; we tokenize the
	// whole buffer once. Caller invalidates by buffer revision.
	iter, err := lexer.Tokenise(nil, source)
	if err != nil {
		return Result{}
	}
	tokens := iter.Tokens()
	return spanFromTokens(source, tokens)
}

// spanFromTokens walks a chroma token stream and emits per-row Spans. We track
// the running byte offset within the source and the current row; when a token
// spans a newline we split it across rows.
func spanFromTokens(source string, tokens []chroma.Token) Result {
	rows := make(map[int][]Span)
	row, col := 0, 0
	for _, t := range tokens {
		kind := classify(t.Type)
		val := t.Value
		if val == "" {
			continue
		}
		// Walk runes (well, bytes — chroma values are UTF-8 strings) to split
		// on newlines. We accumulate byte-by-byte to keep column counts honest
		// for downstream styling.
		start := col
		i := 0
		for j := 0; j < len(val); j++ {
			if val[j] == '\n' {
				if kind != KindPlain && j > i {
					rows[row] = append(rows[row], Span{Start: start, End: start + (j - i), Kind: kind})
				}
				row++
				col = 0
				start = 0
				i = j + 1
			}
		}
		if i < len(val) {
			tail := val[i:]
			if kind != KindPlain {
				rows[row] = append(rows[row], Span{Start: start, End: start + len(tail), Kind: kind})
			}
			col = start + len(tail)
		}
	}
	mergeAdjacent(rows)
	return Result{Rows: rows}
}

// mergeAdjacent collapses neighbouring Spans of the same Kind so the renderer
// doesn't emit redundant style switches between adjacent tokens like keywords
// (e.g. chroma reports `func` + ` ` + name as three tokens; we merge runs of
// matching kind that touch with no plain gap in between).
func mergeAdjacent(rows map[int][]Span) {
	for r, spans := range rows {
		if len(spans) < 2 {
			continue
		}
		out := spans[:1]
		for _, s := range spans[1:] {
			last := &out[len(out)-1]
			if last.Kind == s.Kind && last.End == s.Start {
				last.End = s.End
				continue
			}
			out = append(out, s)
		}
		rows[r] = out
	}
}

// classify maps a chroma TokenType to our coarse Kind palette. Chroma's
// hierarchy is three-level: Category() (Keyword, Literal, Name, ...),
// SubCategory() (LiteralString, NameFunction, ...), and the leaf TokenType.
// We classify primarily by SubCategory with a Category fallback, folding
// subtypes onto our 7-bucket palette.
func classify(tt chroma.TokenType) Kind {
	// KeywordType comes before generic Keyword so 'string'/'int'/'bool'
	// render as types rather than keywords.
	if tt == chroma.KeywordType {
		return KindType
	}
	switch tt.SubCategory() {
	case chroma.LiteralString:
		return KindString
	case chroma.LiteralNumber:
		return KindNumber
	case chroma.NameFunction:
		return KindFunction
	case chroma.NameClass, chroma.NameNamespace, chroma.NameBuiltin, chroma.NameDecorator:
		return KindType
	}
	switch tt.Category() {
	case chroma.Keyword:
		return KindKeyword
	case chroma.Comment:
		return KindComment
	case chroma.Punctuation, chroma.Operator:
		return KindPunctuation
	}
	return KindPlain
}

// LanguageFor reports the canonical chroma lexer name for a path, or empty
// string if no lexer matches. Used in tests and diagnostics.
func LanguageFor(path string) string {
	l := lexers.Match(path)
	if l == nil {
		return ""
	}
	cfg := l.Config()
	if cfg == nil {
		return ""
	}
	return strings.ToLower(cfg.Name)
}
