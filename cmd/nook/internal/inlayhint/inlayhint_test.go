package inlayhint

import (
	"reflect"
	"testing"
)

func TestKindConstants(t *testing.T) {
	if KindUnknown != 0 || KindType != 1 || KindParameter != 2 {
		t.Fatalf("kind constants drifted: %d/%d/%d", KindUnknown, KindType, KindParameter)
	}
}

func TestByRow_Empty(t *testing.T) {
	if got := ByRow(nil); got != nil {
		t.Fatalf("ByRow(nil) = %v, want nil", got)
	}
	if got := ByRow([]Hint{}); got != nil {
		t.Fatalf("ByRow([]) = %v, want nil", got)
	}
}

func TestByRow_PreservesOrderWithinRow(t *testing.T) {
	hints := []Hint{
		{Row: 3, Col: 5, Label: ": int", Kind: KindType},
		{Row: 3, Col: 12, Label: "x:", Kind: KindParameter, PaddingRight: true},
		{Row: 3, Col: 15, Label: "y:", Kind: KindParameter, PaddingRight: true},
	}
	got := ByRow(hints)
	want := map[int][]Hint{
		3: hints,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ByRow preserves order within row\n got: %#v\nwant: %#v", got, want)
	}
}

func TestByRow_SplitsAcrossRows(t *testing.T) {
	hints := []Hint{
		{Row: 0, Col: 4, Label: ": string"},
		{Row: 7, Col: 1, Label: "ctx:", PaddingRight: true},
		{Row: 7, Col: 8, Label: "n:", PaddingRight: true},
		{Row: 11, Col: 0, Label: ": int"},
	}
	got := ByRow(hints)
	if len(got) != 3 {
		t.Fatalf("ByRow split rows count = %d, want 3", len(got))
	}
	if !reflect.DeepEqual(got[0], hints[0:1]) {
		t.Fatalf("row 0: got %#v want %#v", got[0], hints[0:1])
	}
	if !reflect.DeepEqual(got[7], hints[1:3]) {
		t.Fatalf("row 7: got %#v want %#v", got[7], hints[1:3])
	}
	if !reflect.DeepEqual(got[11], hints[3:4]) {
		t.Fatalf("row 11: got %#v want %#v", got[11], hints[3:4])
	}
}

func TestHintsMsgZeroValueOK(t *testing.T) {
	var m HintsMsg
	if m.Path != "" || m.Hints != nil || m.Err != nil || m.Version != 0 {
		t.Fatalf("zero HintsMsg not pristine: %+v", m)
	}
}
