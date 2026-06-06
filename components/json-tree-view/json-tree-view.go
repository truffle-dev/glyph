// Package jsontreeview renders an arbitrary JSON value as an interactive
// collapsible tree.
//
// The component is a thin shell over components/tree-view: a single
// FromAny pass converts any `any` (the standard json.Unmarshal target)
// into a tree of treeview.Node values whose labels carry type-aware
// formatting — strings get quotes, numbers and booleans are rendered as
// literals, null is muted, and objects and arrays advertise their
// element count next to the key. The Model forwards the Bubble Tea
// Model/Update/View contract to its embedded tree, so expand, collapse,
// cursor, scroll, SelectMsg, and keyboard bindings all behave exactly
// like tree-view.
//
// This is the JSON specialization of tree-view in the same way that
// file-tree is the file-system specialization: the data shape changes,
// the navigation primitive stays.
package jsontreeview

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
	treeview "github.com/truffle-dev/glyph/components/tree-view"
)

// Model wraps a tree-view configured for JSON rendering.
type Model struct {
	th       theme.Theme
	tree     treeview.Model
	sortKeys bool
	rootKey  string
}

// SelectMsg fires on Enter on the selected node. The wrapped Path and
// Index come straight from treeview.SelectMsg; Node is the original
// treeview.Node; Value is the underlying JSON value (string / float64 /
// bool / nil / map[string]any / []any), populated from the Value field
// of the treeview.Node.
type SelectMsg struct {
	Node  treeview.Node
	Value any
	Path  string
	Index int
}

// New constructs a Model with Default theme, the tree starting empty,
// keys sorted alphabetically, and the root labeled "$".
func New() Model {
	return Model{
		th:       theme.Default,
		tree:     treeview.New(),
		sortKeys: true,
		rootKey:  "$",
	}
}

// WithTheme replaces the theme on both this model and the embedded tree.
func (m Model) WithTheme(t theme.Theme) Model {
	m.th = t
	m.tree = m.tree.WithTheme(t)
	return m
}

// WithValue rebuilds the tree from any JSON-compatible value (the type
// `encoding/json` returns for an unknown shape). Objects render as
// branches keyed by their entries; arrays render as branches keyed by
// `[i]`; scalars render as leaves with type-aware value formatting.
func (m Model) WithValue(v any) Model {
	root := buildNode(m.rootKey, v, m.th, m.sortKeys, true)
	m.tree = m.tree.WithRoot(root)
	return m
}

// WithJSON parses the supplied bytes as JSON and rebuilds the tree.
// Returns the same Model unchanged on parse error; callers that care
// about parse failure should json.Unmarshal first and call WithValue.
func (m Model) WithJSON(b []byte) Model {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return m
	}
	return m.WithValue(v)
}

// WithRootKey overrides the label shown on the root row (default "$").
func (m Model) WithRootKey(s string) Model { m.rootKey = s; return m }

// WithSortKeys toggles alphabetical ordering of object entries.
func (m Model) WithSortKeys(b bool) Model { m.sortKeys = b; return m }

// WithSize forwards to the embedded tree.
func (m Model) WithSize(w, h int) Model {
	m.tree = m.tree.WithSize(w, h)
	return m
}

// WithExpandAll forwards to the embedded tree.
func (m Model) WithExpandAll() Model {
	m.tree = m.tree.WithExpandAll()
	return m
}

// WithCollapseAll forwards to the embedded tree.
func (m Model) WithCollapseAll() Model {
	m.tree = m.tree.WithCollapseAll()
	return m
}

// WithExpandedDepth forwards to the embedded tree.
func (m Model) WithExpandedDepth(d int) Model {
	m.tree = m.tree.WithExpandedDepth(d)
	return m
}

// WithHighlightCursor forwards to the embedded tree.
func (m Model) WithHighlightCursor(b bool) Model {
	m.tree = m.tree.WithHighlightCursor(b)
	return m
}

// WithTitle forwards to the embedded tree.
func (m Model) WithTitle(s string) Model {
	m.tree = m.tree.WithTitle(s)
	return m
}

// WithPlaceholder forwards to the embedded tree.
func (m Model) WithPlaceholder(s string) Model {
	m.tree = m.tree.WithPlaceholder(s)
	return m
}

// WithRootVisible forwards to the embedded tree.
func (m Model) WithRootVisible(b bool) Model {
	m.tree = m.tree.WithRootVisible(b)
	return m
}

// Cursor returns the cursor index from the embedded tree.
func (m Model) Cursor() int { return m.tree.Cursor() }

// SelectedPath returns the slash-joined child-index path from the
// embedded tree.
func (m Model) SelectedPath() (string, bool) { return m.tree.SelectedPath() }

// SelectedNode returns the underlying treeview.Node under the cursor.
func (m Model) SelectedNode() (treeview.Node, bool) { return m.tree.SelectedNode() }

// SelectedValue returns the JSON value carried in the cursor's Node.Value.
func (m Model) SelectedValue() (any, bool) {
	if n, ok := m.tree.SelectedNode(); ok {
		return n.Value, true
	}
	return nil, false
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return m.tree.Init() }

// Update forwards to the embedded tree and translates its SelectMsg into
// our SelectMsg with the underlying JSON Value attached.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	nt, cmd := m.tree.Update(msg)
	m.tree = nt
	if cmd == nil {
		return m, nil
	}
	wrapped := func() tea.Msg {
		inner := cmd()
		if sel, ok := inner.(treeview.SelectMsg); ok {
			return SelectMsg{
				Node:  sel.Node,
				Value: sel.Node.Value,
				Path:  sel.Path,
				Index: sel.Index,
			}
		}
		return inner
	}
	return m, wrapped
}

// View forwards to the embedded tree.
func (m Model) View() string { return m.tree.View() }

// buildNode converts a JSON value into a treeview.Node. Top-level
// callers pass true for isRoot so the root label uses the configured
// rootKey directly instead of a "key: " formatted prefix.
func buildNode(key string, v any, th theme.Theme, sortKeys, isRoot bool) treeview.Node {
	keyStyle := lipgloss.NewStyle().Foreground(th.Primary)
	mutedStyle := lipgloss.NewStyle().Foreground(th.TextMuted)

	keyPart := ""
	if isRoot {
		keyPart = keyStyle.Render(key)
	} else if key != "" {
		keyPart = keyStyle.Render(key) + mutedStyle.Render(": ")
	}

	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		if sortKeys {
			sort.Strings(keys)
		}
		children := make([]treeview.Node, 0, len(keys))
		for _, k := range keys {
			children = append(children, buildNode(k, t[k], th, sortKeys, false))
		}
		label := keyPart + mutedStyle.Render(fmt.Sprintf("{%d}", len(t)))
		return treeview.Node{Label: label, Value: v, Children: children}
	case []any:
		children := make([]treeview.Node, len(t))
		for i, c := range t {
			children[i] = buildNode(fmt.Sprintf("[%d]", i), c, th, sortKeys, false)
		}
		label := keyPart + mutedStyle.Render(fmt.Sprintf("[%d]", len(t)))
		return treeview.Node{Label: label, Value: v, Children: children}
	case string:
		valStyle := lipgloss.NewStyle().Foreground(th.Success)
		return treeview.Node{Label: keyPart + valStyle.Render(strconv.Quote(t)), Value: v}
	case bool:
		valStyle := lipgloss.NewStyle().Foreground(th.Warning)
		return treeview.Node{Label: keyPart + valStyle.Render(strconv.FormatBool(t)), Value: v}
	case nil:
		return treeview.Node{Label: keyPart + mutedStyle.Render("null"), Value: v}
	case float64:
		valStyle := lipgloss.NewStyle().Foreground(th.Info)
		return treeview.Node{Label: keyPart + valStyle.Render(formatFloat(t)), Value: v}
	case int:
		valStyle := lipgloss.NewStyle().Foreground(th.Info)
		return treeview.Node{Label: keyPart + valStyle.Render(strconv.Itoa(t)), Value: v}
	case int64:
		valStyle := lipgloss.NewStyle().Foreground(th.Info)
		return treeview.Node{Label: keyPart + valStyle.Render(strconv.FormatInt(t, 10)), Value: v}
	case json.Number:
		valStyle := lipgloss.NewStyle().Foreground(th.Info)
		return treeview.Node{Label: keyPart + valStyle.Render(string(t)), Value: v}
	default:
		valStyle := lipgloss.NewStyle().Foreground(th.TextMuted)
		return treeview.Node{Label: keyPart + valStyle.Render(fmt.Sprintf("%v", v)), Value: v}
	}
}

// formatFloat renders a float64 in the smallest form that round-trips.
// Integers come back without a trailing `.0`; non-integers keep their
// natural decimal form. This matches the way most JSON browsers format
// the same numbers.
func formatFloat(f float64) string {
	if f == float64(int64(f)) && !strings.Contains(strconv.FormatFloat(f, 'f', -1, 64), ".") {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}
