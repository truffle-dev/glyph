package jsontreeview

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	treeview "github.com/truffle-dev/glyph/components/tree-view"
)

func sample() any {
	var v any
	_ = json.Unmarshal([]byte(`{
		"name": "phantom",
		"version": 7,
		"active": true,
		"owner": null,
		"tags": ["a", "b", "c"],
		"limits": {"cpu": 2, "mem": 512}
	}`), &v)
	return v
}

// stripAnsi removes ANSI escape sequences for substring comparisons.
func stripAnsi(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			in = true
		case in && (r == 'm' || r == 'K' || r == 'J' || r == 'H' || r == 'A' || r == 'B' || r == 'C' || r == 'D' || r == 'f'):
			in = false
		case in:
			// drop
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestEmptyRendersDefaultPlaceholder(t *testing.T) {
	m := New().WithSize(30, 5)
	out := stripAnsi(m.View())
	if !strings.Contains(out, "no nodes") {
		t.Fatalf("expected default placeholder, got %q", out)
	}
}

func TestRendersRootAndDirectChildren(t *testing.T) {
	m := New().WithValue(sample()).WithSize(60, 20)
	out := stripAnsi(m.View())
	// Root key
	if !strings.Contains(out, "$") {
		t.Fatalf("expected root key $: %s", out)
	}
	// Direct keys (sorted alphabetically)
	for _, want := range []string{"active", "limits", "name", "owner", "tags", "version"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing key %q in output:\n%s", want, out)
		}
	}
}

func TestKeysAreSortedAlphabeticallyByDefault(t *testing.T) {
	m := New().WithValue(sample()).WithSize(60, 20)
	out := stripAnsi(m.View())
	// Find positions of the keys; should be increasing.
	keys := []string{"active", "limits", "name", "owner", "tags", "version"}
	prev := -1
	for _, k := range keys {
		// match colon-suffixed to avoid finding substrings.
		needle := k + ":"
		idx := strings.Index(out, needle)
		if idx == -1 {
			// Last entry (highest position) might not have a colon if branch.
			// Try without colon.
			idx = strings.Index(out, k)
		}
		if idx <= prev {
			t.Fatalf("key %q at index %d not after previous %d. Output:\n%s", k, idx, prev, out)
		}
		prev = idx
	}
}

func TestUnsortedKeepsInsertionOrderDeterministicMap(t *testing.T) {
	// With sortKeys=false, the JSON map iteration order is non-deterministic,
	// so we just verify all keys still appear.
	m := New().WithSortKeys(false).WithValue(sample()).WithSize(60, 20)
	out := stripAnsi(m.View())
	for _, want := range []string{"active", "limits", "name", "owner", "tags", "version"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing key %q with sortKeys=false:\n%s", want, out)
		}
	}
}

func TestObjectCountSuffix(t *testing.T) {
	m := New().WithValue(sample()).WithSize(60, 20)
	out := stripAnsi(m.View())
	// Root is an object with 6 fields -> "{6}".
	if !strings.Contains(out, "{6}") {
		t.Fatalf("expected {6} for root object: %s", out)
	}
}

func TestArrayCountSuffix(t *testing.T) {
	m := New().WithValue(sample()).WithExpandedDepth(2).WithSize(60, 20)
	out := stripAnsi(m.View())
	// "tags" array has 3 entries -> "[3]" appears in tags row.
	if !strings.Contains(out, "[3]") {
		t.Fatalf("expected [3] for tags array: %s", out)
	}
}

func TestStringValuesAreQuoted(t *testing.T) {
	m := New().WithValue(sample()).WithExpandAll().WithSize(60, 20)
	out := stripAnsi(m.View())
	if !strings.Contains(out, `"phantom"`) {
		t.Fatalf("expected quoted string %q: %s", `"phantom"`, out)
	}
}

func TestBoolAndNullRender(t *testing.T) {
	m := New().WithValue(sample()).WithSize(60, 20)
	out := stripAnsi(m.View())
	if !strings.Contains(out, "true") {
		t.Fatalf("expected literal true: %s", out)
	}
	if !strings.Contains(out, "null") {
		t.Fatalf("expected literal null: %s", out)
	}
}

func TestNumbersRenderAsIntWhenInteger(t *testing.T) {
	m := New().WithValue(sample()).WithSize(60, 20)
	out := stripAnsi(m.View())
	// version is 7 (decoded as float64 7.0 by json.Unmarshal). Should print as "7".
	if !strings.Contains(out, "7") {
		t.Fatalf("expected integer-form 7: %s", out)
	}
	// And NOT "7.0" (formatFloat collapses integer-valued floats).
	if strings.Contains(out, "7.0") {
		t.Fatalf("expected no 7.0, got: %s", out)
	}
}

func TestNestedObjectExpandsToReveal(t *testing.T) {
	m := New().WithValue(sample()).WithExpandAll().WithSize(60, 20)
	out := stripAnsi(m.View())
	// limits has cpu and mem
	for _, want := range []string{"cpu", "mem", "512"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected nested %q after expand-all: %s", want, out)
		}
	}
}

func TestSelectedValueAccessor(t *testing.T) {
	m := New().WithValue(sample()).WithSize(60, 20)
	if v, ok := m.SelectedValue(); !ok {
		t.Fatal("expected SelectedValue ok on populated tree")
	} else if _, isMap := v.(map[string]any); !isMap {
		t.Fatalf("expected root value to be map, got %T", v)
	}
}

func TestEnterEmitsJsonSelectMsg(t *testing.T) {
	m := New().WithValue(sample()).WithExpandAll().WithSize(60, 20)
	// Cursor at root. Press Enter; should emit jsontreeview.SelectMsg.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected select cmd")
	}
	msg := cmd()
	sel, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("expected jsontreeview.SelectMsg, got %T", msg)
	}
	if sel.Index != 0 {
		t.Fatalf("expected index 0 on root, got %d", sel.Index)
	}
	if _, isMap := sel.Value.(map[string]any); !isMap {
		t.Fatalf("expected root SelectMsg.Value to be map, got %T", sel.Value)
	}
}

func TestWithJSONParsesOrSkipsOnError(t *testing.T) {
	// Valid JSON -> tree shows expected keys.
	m := New().WithJSON([]byte(`{"a": 1, "b": 2}`)).WithSize(40, 10)
	out := stripAnsi(m.View())
	for _, want := range []string{"a", "b", "{2}"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q after WithJSON: %s", want, out)
		}
	}
	// Invalid JSON -> Model unchanged (still empty placeholder).
	m2 := New().WithJSON([]byte("not json")).WithSize(40, 5)
	if !strings.Contains(stripAnsi(m2.View()), "no nodes") {
		t.Fatal("expected invalid JSON to leave tree empty")
	}
}

func TestRootKeyOverride(t *testing.T) {
	m := New().WithRootKey("payload").WithValue(map[string]any{"x": 1}).WithSize(40, 10)
	out := stripAnsi(m.View())
	if !strings.Contains(out, "payload") {
		t.Fatalf("expected custom root key: %s", out)
	}
	if strings.Contains(out, "$") && !strings.Contains(out, "payload") {
		t.Fatal("expected only custom root, not default $")
	}
}

func TestCursorAndSelectedNodeForwarded(t *testing.T) {
	m := New().WithValue(sample()).WithExpandAll().WithSize(60, 20)
	if m.Cursor() != 0 {
		t.Fatalf("expected initial cursor 0, got %d", m.Cursor())
	}
	if _, ok := m.SelectedNode(); !ok {
		t.Fatal("expected SelectedNode ok")
	}
	if _, ok := m.SelectedPath(); !ok {
		t.Fatal("expected SelectedPath ok")
	}
	// Underlying type check: the treeview.Node should be returned untouched.
	node, _ := m.SelectedNode()
	var _ treeview.Node = node
}

func TestFormatFloat(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{7, "7"},
		{7.5, "7.5"},
		{0, "0"},
		{-3.14, "-3.14"},
		{1000000, "1000000"},
	}
	for _, c := range cases {
		got := formatFloat(c.in)
		if got != c.want {
			t.Errorf("formatFloat(%v)=%q want %q", c.in, got, c.want)
		}
	}
}
