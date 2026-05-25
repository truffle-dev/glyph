// Package semtok decodes LSP textDocument/semanticTokens/full responses.
//
// The server returns a flat []uint32 in groups of five:
//
//	[deltaLine, deltaStart, length, tokenType, tokenModifiers]
//
// where deltaLine is relative to the previous token's line and deltaStart
// is relative to the previous token's column when on the same line, or
// absolute (from column 0) when deltaLine > 0. The tokenType and the
// individual bits of tokenModifiers index into the server-provided legend.
package semtok

// Token is a single decoded semantic token at an absolute position.
type Token struct {
	Line      int
	Col       int
	Length    int
	Type      string
	Modifiers []string
}

// Legend names the token-type and token-modifier indices declared by the
// server at initialize-time.
type Legend struct {
	TokenTypes     []string
	TokenModifiers []string
}

// Decode unrolls the delta-encoded uint32 array into resolved tokens.
// Returns nil if data is empty or length is not a multiple of 5.
// Out-of-range type indices resolve to the empty string; out-of-range
// modifier bits are dropped.
func Decode(data []uint32, legend Legend) []Token {
	if len(data) == 0 || len(data)%5 != 0 {
		return nil
	}
	out := make([]Token, 0, len(data)/5)
	var line, col int
	for i := 0; i < len(data); i += 5 {
		deltaLine := int(data[i])
		deltaStart := int(data[i+1])
		length := int(data[i+2])
		typeIdx := int(data[i+3])
		modBits := data[i+4]

		if deltaLine == 0 {
			col += deltaStart
		} else {
			line += deltaLine
			col = deltaStart
		}

		var typeName string
		if typeIdx >= 0 && typeIdx < len(legend.TokenTypes) {
			typeName = legend.TokenTypes[typeIdx]
		}

		var mods []string
		for b := 0; b < len(legend.TokenModifiers); b++ {
			if modBits&(1<<uint(b)) != 0 {
				mods = append(mods, legend.TokenModifiers[b])
			}
		}

		out = append(out, Token{
			Line:      line,
			Col:       col,
			Length:    length,
			Type:      typeName,
			Modifiers: mods,
		})
	}
	return out
}
