package semtok

import (
	"reflect"
	"testing"
)

func TestDecode_Empty(t *testing.T) {
	if got := Decode(nil, Legend{}); got != nil {
		t.Errorf("nil input: want nil, got %v", got)
	}
	if got := Decode([]uint32{}, Legend{}); got != nil {
		t.Errorf("empty input: want nil, got %v", got)
	}
}

func TestDecode_MalformedLength(t *testing.T) {
	if got := Decode([]uint32{0, 1, 2, 3}, Legend{}); got != nil {
		t.Errorf("len=4: want nil, got %v", got)
	}
	if got := Decode([]uint32{0, 1, 2, 3, 0, 0, 1}, Legend{}); got != nil {
		t.Errorf("len=7: want nil, got %v", got)
	}
}

func TestDecode_SingleToken(t *testing.T) {
	legend := Legend{TokenTypes: []string{"function"}}
	got := Decode([]uint32{0, 0, 5, 0, 0}, legend)
	want := []Token{{Line: 0, Col: 0, Length: 5, Type: "function"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestDecode_SameLineDelta(t *testing.T) {
	legend := Legend{TokenTypes: []string{"keyword", "variable"}}
	// token 1 at (0, 4) length 3 type=keyword
	// token 2 at (0, 4+5)=9 length 4 type=variable (deltaLine=0, deltaStart=5)
	got := Decode([]uint32{
		0, 4, 3, 0, 0,
		0, 5, 4, 1, 0,
	}, legend)
	want := []Token{
		{Line: 0, Col: 4, Length: 3, Type: "keyword"},
		{Line: 0, Col: 9, Length: 4, Type: "variable"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestDecode_MultiLine(t *testing.T) {
	legend := Legend{TokenTypes: []string{"function", "parameter"}}
	// token 1 at (0, 2) length 4 type=function
	// token 2 at (2, 6) length 3 type=parameter — deltaLine=2 makes deltaStart absolute
	// token 3 at (2, 10) length 2 type=function — same line, deltaStart=4 relative to col 6
	got := Decode([]uint32{
		0, 2, 4, 0, 0,
		2, 6, 3, 1, 0,
		0, 4, 2, 0, 0,
	}, legend)
	want := []Token{
		{Line: 0, Col: 2, Length: 4, Type: "function"},
		{Line: 2, Col: 6, Length: 3, Type: "parameter"},
		{Line: 2, Col: 10, Length: 2, Type: "function"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestDecode_ModifierBits(t *testing.T) {
	legend := Legend{
		TokenTypes:     []string{"variable"},
		TokenModifiers: []string{"declaration", "readonly", "static"},
	}
	// 0b101 = declaration + static, skip readonly
	got := Decode([]uint32{0, 0, 3, 0, 0b101}, legend)
	want := []Token{{
		Line: 0, Col: 0, Length: 3, Type: "variable",
		Modifiers: []string{"declaration", "static"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestDecode_AllModifierBitsSet(t *testing.T) {
	legend := Legend{
		TokenTypes:     []string{"variable"},
		TokenModifiers: []string{"declaration", "readonly", "static"},
	}
	got := Decode([]uint32{0, 0, 3, 0, 0b111}, legend)
	want := []Token{{
		Line: 0, Col: 0, Length: 3, Type: "variable",
		Modifiers: []string{"declaration", "readonly", "static"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestDecode_OutOfRangeTypeIndex(t *testing.T) {
	legend := Legend{TokenTypes: []string{"function"}}
	got := Decode([]uint32{0, 0, 3, 5, 0}, legend)
	want := []Token{{Line: 0, Col: 0, Length: 3, Type: ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestDecode_ModifierBitBeyondLegend(t *testing.T) {
	legend := Legend{
		TokenTypes:     []string{"variable"},
		TokenModifiers: []string{"declaration"},
	}
	// bit 1 is set but legend only has one modifier — drop it, return only "declaration"
	got := Decode([]uint32{0, 0, 3, 0, 0b11}, legend)
	want := []Token{{
		Line: 0, Col: 0, Length: 3, Type: "variable",
		Modifiers: []string{"declaration"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestDecode_LargeDeltas(t *testing.T) {
	legend := Legend{TokenTypes: []string{"function"}}
	got := Decode([]uint32{1000, 500, 10, 0, 0}, legend)
	want := []Token{{Line: 1000, Col: 500, Length: 10, Type: "function"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestDecode_RealisticSequence(t *testing.T) {
	// Approximates a small Go file:
	//   line 0: "func main() {"
	//     func at (0, 0) len 4 keyword
	//     main at (0, 5) len 4 function declaration
	//   line 2: "    x := 1"
	//     x at (2, 4) len 1 variable declaration
	legend := Legend{
		TokenTypes:     []string{"keyword", "function", "variable"},
		TokenModifiers: []string{"declaration", "readonly"},
	}
	got := Decode([]uint32{
		0, 0, 4, 0, 0,
		0, 5, 4, 1, 0b01,
		2, 4, 1, 2, 0b01,
	}, legend)
	want := []Token{
		{Line: 0, Col: 0, Length: 4, Type: "keyword"},
		{Line: 0, Col: 5, Length: 4, Type: "function", Modifiers: []string{"declaration"}},
		{Line: 2, Col: 4, Length: 1, Type: "variable", Modifiers: []string{"declaration"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}
