package diagnostics

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestSeverityMark(t *testing.T) {
	cases := map[Severity]string{
		SeverityError:   "E",
		SeverityWarning: "W",
		SeverityInfo:    "I",
		SeverityHint:    "H",
		Severity(99):    "?",
	}
	for sev, want := range cases {
		if got := sev.Mark(); got != want {
			t.Errorf("Severity(%d).Mark() = %q, want %q", sev, got, want)
		}
	}
}

func TestSeverityColor(t *testing.T) {
	th := theme.Default
	if got := SeverityError.Color(th); got != th.Error {
		t.Errorf("SeverityError.Color = %v, want %v", got, th.Error)
	}
	if got := SeverityWarning.Color(th); got != th.Warning {
		t.Errorf("SeverityWarning.Color = %v, want %v", got, th.Warning)
	}
	if got := SeverityInfo.Color(th); got != th.Info {
		t.Errorf("SeverityInfo.Color = %v, want %v", got, th.Info)
	}
	if got := SeverityHint.Color(th); got != th.TextMuted {
		t.Errorf("SeverityHint.Color = %v, want %v", got, th.TextMuted)
	}
	if got := Severity(42).Color(th); got != th.TextMuted {
		t.Errorf("unknown Severity Color = %v, want %v (muted fallback)", got, th.TextMuted)
	}
}

func TestSortBySeverityFirst(t *testing.T) {
	in := []Entry{
		{Path: "a.go", Severity: SeverityWarning},
		{Path: "b.go", Severity: SeverityError},
		{Path: "c.go", Severity: SeverityHint},
		{Path: "d.go", Severity: SeverityInfo},
	}
	out := Sort(in)
	wantSev := []Severity{SeverityError, SeverityWarning, SeverityInfo, SeverityHint}
	for i, w := range wantSev {
		if out[i].Severity != w {
			t.Errorf("out[%d].Severity = %v, want %v", i, out[i].Severity, w)
		}
	}
}

func TestSortStableByPathRowCol(t *testing.T) {
	in := []Entry{
		{Path: "z.go", Row: 0, Col: 0, Severity: SeverityError},
		{Path: "a.go", Row: 5, Col: 0, Severity: SeverityError},
		{Path: "a.go", Row: 1, Col: 10, Severity: SeverityError},
		{Path: "a.go", Row: 1, Col: 3, Severity: SeverityError},
	}
	out := Sort(in)
	// a.go:1:3, a.go:1:10, a.go:5:0, z.go:0:0
	want := []struct {
		path string
		row  int
		col  int
	}{
		{"a.go", 1, 3},
		{"a.go", 1, 10},
		{"a.go", 5, 0},
		{"z.go", 0, 0},
	}
	for i, w := range want {
		if out[i].Path != w.path || out[i].Row != w.row || out[i].Col != w.col {
			t.Errorf("out[%d] = %s:%d:%d, want %s:%d:%d",
				i, out[i].Path, out[i].Row, out[i].Col, w.path, w.row, w.col)
		}
	}
}

func TestSortDoesNotMutateInput(t *testing.T) {
	in := []Entry{
		{Path: "z.go", Severity: SeverityError},
		{Path: "a.go", Severity: SeverityError},
	}
	_ = Sort(in)
	if in[0].Path != "z.go" {
		t.Errorf("Sort mutated input slice: in[0].Path = %q, want %q", in[0].Path, "z.go")
	}
}

func TestNewPaneDefaults(t *testing.T) {
	p := NewPane(theme.Default, "/work")
	if p.IsFocused() {
		t.Error("new pane should not be focused")
	}
	if p.Count() != 0 {
		t.Errorf("Count = %d, want 0", p.Count())
	}
	if p.Cursor() != 0 {
		t.Errorf("Cursor = %d, want 0", p.Cursor())
	}
	if _, ok := p.Selected(); ok {
		t.Error("Selected on empty pane should be ok=false")
	}
}

func TestWithSizeFloors(t *testing.T) {
	// Passing absurdly small dimensions should not panic; floors apply.
	p := NewPane(theme.Default, "/work").WithSize(5, 1)
	v := p.View()
	if v == "" {
		t.Error("View should not be empty even with tiny size")
	}
}

func TestFocusBlur(t *testing.T) {
	p := NewPane(theme.Default, "/work")
	p = p.Focus()
	if !p.IsFocused() {
		t.Error("Focus should set focused=true")
	}
	p = p.Blur()
	if p.IsFocused() {
		t.Error("Blur should set focused=false")
	}
}

func TestWithEntriesClampsCursor(t *testing.T) {
	p := NewPane(theme.Default, "/work").
		WithSize(80, 20).
		WithEntries([]Entry{
			{Path: "a.go", Severity: SeverityError},
			{Path: "b.go", Severity: SeverityError},
			{Path: "c.go", Severity: SeverityError},
		}).
		Focus()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if p.Cursor() != 2 {
		t.Fatalf("End should put cursor at 2, got %d", p.Cursor())
	}

	// Shrink to one entry — cursor must clamp.
	p = p.WithEntries([]Entry{{Path: "a.go", Severity: SeverityError}})
	if p.Cursor() != 0 {
		t.Errorf("after shrink to 1 entry, cursor = %d, want 0", p.Cursor())
	}

	// Empty entries — cursor falls back to 0.
	p = p.WithEntries(nil)
	if p.Cursor() != 0 {
		t.Errorf("after WithEntries(nil), cursor = %d, want 0", p.Cursor())
	}
}

func TestUpdateBlurredIgnoresKeys(t *testing.T) {
	p := NewPane(theme.Default, "/work").
		WithEntries([]Entry{
			{Path: "a.go", Severity: SeverityError},
			{Path: "b.go", Severity: SeverityError},
		})
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.Cursor() != 0 {
		t.Errorf("blurred pane moved cursor to %d, want 0", p.Cursor())
	}
	if cmd != nil {
		t.Error("blurred pane should return nil cmd")
	}
}

func TestUpdateEscEmitsCancel(t *testing.T) {
	p := NewPane(theme.Default, "/work").Focus()
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should return a cmd")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Errorf("Esc cmd produced %T, want CancelMsg", cmd())
	}
}

func TestUpdateEnterEmitsOpenAt(t *testing.T) {
	p := NewPane(theme.Default, "/work").
		WithSize(80, 20).
		WithEntries([]Entry{
			// Warning sorts after error, so after sort: a.go (Error) at 0, b.go (Warning) at 1.
			{Path: "/work/a.go", Row: 5, Col: 12, Severity: SeverityError},
			{Path: "/work/b.go", Row: 2, Col: 3, Severity: SeverityWarning},
		}).
		Focus()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return a cmd")
	}
	msg, ok := cmd().(OpenAtMsg)
	if !ok {
		t.Fatalf("Enter cmd produced %T, want OpenAtMsg", cmd())
	}
	if msg.Path != "/work/b.go" || msg.Row != 2 || msg.Col != 3 {
		t.Errorf("OpenAtMsg = %+v, want {/work/b.go 2 3}", msg)
	}
}

func TestUpdateEnterEmptyIsNoOp(t *testing.T) {
	p := NewPane(theme.Default, "/work").Focus()
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("Enter on empty pane should be no-op, got cmd")
	}
}

func TestUpdateNavigation(t *testing.T) {
	entries := make([]Entry, 10)
	for i := range entries {
		entries[i] = Entry{Path: "a.go", Row: i, Severity: SeverityError}
	}
	p := NewPane(theme.Default, "/work").
		WithSize(80, 20).
		WithEntries(entries).
		Focus()

	if p.Cursor() != 0 {
		t.Fatalf("initial cursor = %d, want 0", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.Cursor() != 1 {
		t.Errorf("after Down: %d, want 1", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.Cursor() != 0 {
		t.Errorf("after Up: %d, want 0", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp}) // clamp at 0
	if p.Cursor() != 0 {
		t.Errorf("Up at 0 should clamp: %d", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if p.Cursor() != 9 {
		t.Errorf("after End: %d, want 9", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown}) // clamp at end
	if p.Cursor() != 9 {
		t.Errorf("Down at end should clamp: %d", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyHome})
	if p.Cursor() != 0 {
		t.Errorf("after Home: %d, want 0", p.Cursor())
	}
}

func TestUpdatePageNavigation(t *testing.T) {
	entries := make([]Entry, 100)
	for i := range entries {
		entries[i] = Entry{Path: "a.go", Row: i, Severity: SeverityError}
	}
	p := NewPane(theme.Default, "/work").
		WithSize(80, 20).
		WithEntries(entries).
		Focus()

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if p.Cursor() == 0 {
		t.Error("PgDown should advance cursor")
	}
	if p.Cursor() >= 100 {
		t.Errorf("PgDown overshoot: %d", p.Cursor())
	}

	for i := 0; i < 25; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	if p.Cursor() != 99 {
		t.Errorf("after many PgDowns, cursor = %d, want 99", p.Cursor())
	}

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if p.Cursor() >= 99 {
		t.Errorf("PgUp should retreat: %d", p.Cursor())
	}

	for i := 0; i < 25; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	}
	if p.Cursor() != 0 {
		t.Errorf("after many PgUps, cursor = %d, want 0", p.Cursor())
	}
}

func TestSelectedReturnsCursorEntry(t *testing.T) {
	p := NewPane(theme.Default, "/work").
		WithEntries([]Entry{
			{Path: "x.go", Row: 1, Col: 2, Severity: SeverityError, Message: "first"},
			{Path: "y.go", Row: 3, Col: 4, Severity: SeverityError, Message: "second"},
		}).
		Focus()
	e, ok := p.Selected()
	if !ok {
		t.Fatal("Selected ok=false on non-empty pane")
	}
	if e.Path != "x.go" || e.Message != "first" {
		t.Errorf("Selected = %+v, want path=x.go message=first", e)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	e, _ = p.Selected()
	if e.Path != "y.go" {
		t.Errorf("after Down, Selected.Path = %q, want y.go", e.Path)
	}
}

func TestViewEmptyState(t *testing.T) {
	p := NewPane(theme.Default, "/work").WithSize(80, 20)
	v := p.View()
	if !strings.Contains(v, "no diagnostics") {
		t.Errorf("empty view missing 'no diagnostics':\n%s", v)
	}
	if !strings.Contains(v, "workspace diagnostics") {
		t.Errorf("empty view missing title:\n%s", v)
	}
}

func TestViewHeader(t *testing.T) {
	p := NewPane(theme.Default, "/work").
		WithSize(80, 20).
		WithEntries([]Entry{
			{Path: "a.go", Severity: SeverityError, Message: "broken"},
			{Path: "b.go", Severity: SeverityError, Message: "smelly"},
			{Path: "c.go", Severity: SeverityWarning, Message: "warn"},
			{Path: "d.go", Severity: SeverityInfo, Message: "info"},
			{Path: "e.go", Severity: SeverityHint, Message: "hint"},
		})
	v := p.View()
	if !strings.Contains(v, "workspace diagnostics") {
		t.Errorf("view missing title:\n%s", v)
	}
	if !strings.Contains(v, "2 errors") {
		t.Errorf("view missing '2 errors':\n%s", v)
	}
	if !strings.Contains(v, "1 warnings") {
		t.Errorf("view missing '1 warnings':\n%s", v)
	}
	if !strings.Contains(v, "1 info") {
		t.Errorf("view missing '1 info':\n%s", v)
	}
	if !strings.Contains(v, "1 hints") {
		t.Errorf("view missing '1 hints':\n%s", v)
	}
}

func TestViewIncludesEntryFields(t *testing.T) {
	p := NewPane(theme.Default, "/work").
		WithSize(120, 20).
		WithEntries([]Entry{
			{
				Path:     "/work/main.go",
				Row:      41,
				Col:      12,
				Severity: SeverityError,
				Source:   "gopls",
				Message:  "undefined-foo",
			},
		})
	v := p.View()
	if !strings.Contains(v, "main.go") {
		t.Errorf("view missing path:\n%s", v)
	}
	if !strings.Contains(v, ":42:13") {
		t.Errorf("view missing 1-indexed location ':42:13':\n%s", v)
	}
	if !strings.Contains(v, "gopls") {
		t.Errorf("view missing source 'gopls':\n%s", v)
	}
	if !strings.Contains(v, "undefined-foo") {
		t.Errorf("view missing message:\n%s", v)
	}
}

func TestFormatLocationRelativeWithinRoot(t *testing.T) {
	p := NewPane(theme.Default, "/work")
	got := p.formatLocation(Entry{Path: "/work/sub/file.go", Row: 0, Col: 0})
	if !strings.Contains(got, "sub/file.go") {
		t.Errorf("expected relative path 'sub/file.go', got %q", got)
	}
	if !strings.HasSuffix(got, ":1:1") {
		t.Errorf("expected 1-indexed line:col, got %q", got)
	}
}

func TestFormatLocationFallsBackOutsideRoot(t *testing.T) {
	p := NewPane(theme.Default, "/work")
	got := p.formatLocation(Entry{Path: "/other/file.go", Row: 5, Col: 10})
	if !strings.Contains(got, "/other/file.go") {
		t.Errorf("expected absolute fallback, got %q", got)
	}
	if !strings.HasSuffix(got, ":6:11") {
		t.Errorf("expected 1-indexed line:col, got %q", got)
	}
}

func TestFormatLocationNoRoot(t *testing.T) {
	p := NewPane(theme.Default, "")
	got := p.formatLocation(Entry{Path: "/abs/file.go", Row: 0, Col: 0})
	if !strings.Contains(got, "/abs/file.go") {
		t.Errorf("expected absolute path when root='', got %q", got)
	}
}

func TestStripStyleRemovesCSI(t *testing.T) {
	in := "\x1b[31mhello\x1b[0m world"
	got := stripStyle(in)
	if got != "hello world" {
		t.Errorf("stripStyle = %q, want %q", got, "hello world")
	}
}

func TestTruncateCellsShortReturnedUnchanged(t *testing.T) {
	in := "hello"
	got := truncateCells(in, 10)
	if got != in {
		t.Errorf("short string truncated: got %q", got)
	}
}

func TestTruncateCellsLongTruncatesWithEllipsis(t *testing.T) {
	got := truncateCells("abcdefghij", 5)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated string should end with ellipsis, got %q", got)
	}
}

// TestCodeStringHandlesWireTypes verifies the interface{} code value from the
// LSP wire renders correctly for each shape the spec allows (string | number),
// and collapses to "" for the omitted-code case.
func TestCodeStringHandlesWireTypes(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"string code", "SA1019", "SA1019"},
		{"int code", 277, "277"},
		{"int32 code", int32(277), "277"},
		{"int64 code", int64(277), "277"},
		{"json float code", float64(277), "277"},
		{"nil code", nil, ""},
		{"unsupported type", []string{"x"}, ""},
	}
	for _, c := range cases {
		if got := CodeString(c.in); got != c.want {
			t.Errorf("%s: CodeString(%v) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// TestSourceCodeTag verifies the provenance label degrades cleanly when either
// the source or the code is absent, so a code-only diagnostic (rustc) and a
// source-only diagnostic (a vet check with no code) both render, and the
// no-provenance case produces no bracket at all.
func TestSourceCodeTag(t *testing.T) {
	cases := []struct {
		source, code, want string
	}{
		{"gopls", "SA1019", "gopls: SA1019"},
		{"gopls", "", "gopls"},
		{"", "E0277", "E0277"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := sourceCodeTag(c.source, c.code); got != c.want {
			t.Errorf("sourceCodeTag(%q,%q) = %q, want %q", c.source, c.code, got, c.want)
		}
	}
}

// TestRenderRowShowsCode verifies the rendered row carries the diagnostic code
// through to the visible output, not just the source.
func TestRenderRowShowsCode(t *testing.T) {
	p := NewPane(theme.Default, "/work").WithSize(120, 20)
	p = p.WithEntries([]Entry{
		{Path: "/work/a.go", Row: 4, Col: 2, Severity: SeverityError, Source: "gopls", Code: "SA1019", Message: "deprecated"},
	})
	got := stripStyle(p.renderRow(0, 116))
	if !strings.Contains(got, "gopls: SA1019") {
		t.Errorf("rendered row %q does not contain the source:code tag", got)
	}
}

func TestSeverityFilterLabel(t *testing.T) {
	cases := map[Severity]string{
		SeverityError:   "errors only",
		SeverityWarning: "errors + warnings",
		SeverityInfo:    "errors + warnings + info",
		SeverityHint:    "all",
		Severity(99):    "all",
	}
	for sev, want := range cases {
		if got := sev.FilterLabel(); got != want {
			t.Errorf("Severity(%d).FilterLabel() = %q, want %q", sev, got, want)
		}
	}
}

// mixedEntries is one of each severity plus a second error, so a filter
// at each threshold admits a distinct, predictable count: errors-only=2,
// errors+warnings=3, all=5.
func mixedEntries() []Entry {
	return []Entry{
		{Path: "a.go", Severity: SeverityError, Message: "err-a"},
		{Path: "b.go", Severity: SeverityError, Message: "err-b"},
		{Path: "c.go", Severity: SeverityWarning, Message: "warn-c"},
		{Path: "d.go", Severity: SeverityInfo, Message: "info-d"},
		{Path: "e.go", Severity: SeverityHint, Message: "hint-e"},
	}
}

func TestNewPaneStartsUnfiltered(t *testing.T) {
	p := NewPane(theme.Default, "/work").WithSize(80, 20).WithEntries(mixedEntries())
	if p.Filter() != SeverityHint {
		t.Errorf("new pane Filter() = %v, want SeverityHint (unfiltered)", p.Filter())
	}
	if p.Count() != 5 {
		t.Errorf("Count() = %d, want 5", p.Count())
	}
	if p.Shown() != 5 {
		t.Errorf("Shown() = %d, want 5 (all visible when unfiltered)", p.Shown())
	}
}

func TestCycleFilterCyclesThresholds(t *testing.T) {
	p := NewPane(theme.Default, "/work").
		WithSize(80, 20).
		WithEntries(mixedEntries()).
		Focus()

	pressF := func() {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	}

	pressF() // all -> errors only
	if p.Filter() != SeverityError {
		t.Errorf("after 1st f, Filter() = %v, want SeverityError", p.Filter())
	}
	if p.Shown() != 2 {
		t.Errorf("errors-only Shown() = %d, want 2", p.Shown())
	}

	pressF() // errors -> errors+warnings
	if p.Filter() != SeverityWarning {
		t.Errorf("after 2nd f, Filter() = %v, want SeverityWarning", p.Filter())
	}
	if p.Shown() != 3 {
		t.Errorf("errors+warnings Shown() = %d, want 3", p.Shown())
	}

	pressF() // errors+warnings -> all
	if p.Filter() != SeverityHint {
		t.Errorf("after 3rd f, Filter() = %v, want SeverityHint (all)", p.Filter())
	}
	if p.Shown() != 5 {
		t.Errorf("all Shown() = %d, want 5", p.Shown())
	}

	// Count never changes regardless of filter.
	if p.Count() != 5 {
		t.Errorf("Count() drifted to %d under filtering, want 5", p.Count())
	}
}

func TestFilterScopesSelectedToVisible(t *testing.T) {
	p := NewPane(theme.Default, "/work").
		WithSize(80, 20).
		WithEntries(mixedEntries()).
		Focus()

	// Move the cursor down into the hint band first.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if e, _ := p.Selected(); e.Severity != SeverityHint {
		t.Fatalf("End should select the hint row, got %v", e.Severity)
	}

	// Filter to errors only: cursor resets to the top of the filtered view,
	// and every visible row is an error.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if p.Cursor() != 0 {
		t.Errorf("CycleFilter should reset cursor to 0, got %d", p.Cursor())
	}
	e, ok := p.Selected()
	if !ok {
		t.Fatal("Selected ok=false after filtering to errors")
	}
	if e.Severity != SeverityError {
		t.Errorf("filtered Selected severity = %v, want SeverityError", e.Severity)
	}

	// Walking to the end of the filtered view must stay within the error band.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if p.Cursor() != 1 {
		t.Errorf("End in errors-only view: cursor = %d, want 1", p.Cursor())
	}
	e, _ = p.Selected()
	if e.Severity != SeverityError {
		t.Errorf("end-of-filtered Selected severity = %v, want SeverityError", e.Severity)
	}

	// Down past the filtered end clamps; it must not reach hidden rows.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.Cursor() != 1 {
		t.Errorf("Down past filtered end should clamp at 1, got %d", p.Cursor())
	}
}

func TestFilterIgnoredWhenBlurred(t *testing.T) {
	p := NewPane(theme.Default, "/work").
		WithSize(80, 20).
		WithEntries(mixedEntries()) // not focused
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if p.Filter() != SeverityHint {
		t.Errorf("blurred pane changed filter to %v, want SeverityHint", p.Filter())
	}
	if cmd != nil {
		t.Error("blurred pane should return nil cmd for 'f'")
	}
}

func TestViewFilteredEmptyStateMessage(t *testing.T) {
	// Only warnings and below; filtering to errors-only hides everything.
	p := NewPane(theme.Default, "/work").
		WithSize(80, 20).
		WithEntries([]Entry{
			{Path: "a.go", Severity: SeverityWarning, Message: "warn"},
			{Path: "b.go", Severity: SeverityInfo, Message: "info"},
		}).
		Focus()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}) // errors only
	if p.Shown() != 0 {
		t.Fatalf("errors-only Shown() = %d, want 0", p.Shown())
	}
	v := p.View()
	if !strings.Contains(v, "no diagnostics match the errors only filter") {
		t.Errorf("filtered-empty view missing filter-aware message:\n%s", v)
	}
	// The blanket "no diagnostics in workspace" copy must not appear when
	// entries exist but are filtered out.
	if strings.Contains(v, "no diagnostics in workspace") {
		t.Errorf("filtered-empty view wrongly shows the empty-workspace copy:\n%s", v)
	}
}

func TestViewHeaderShowsFilterLabel(t *testing.T) {
	p := NewPane(theme.Default, "/work").
		WithSize(80, 20).
		WithEntries(mixedEntries()).
		Focus()
	// Unfiltered: no parenthetical filter label in the title.
	if v := p.View(); strings.Contains(v, "(errors only)") {
		t.Errorf("unfiltered header should not carry a filter label:\n%s", v)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}) // errors only
	v := p.View()
	if !strings.Contains(v, "errors only") {
		t.Errorf("filtered header missing 'errors only' label:\n%s", v)
	}
	// Counts in the summary stay over the full set, not the filtered view.
	if !strings.Contains(v, "2 errors") {
		t.Errorf("header summary should report total counts, got:\n%s", v)
	}
	if !strings.Contains(v, "1 hints") {
		t.Errorf("header summary should still report hidden hints, got:\n%s", v)
	}
}
