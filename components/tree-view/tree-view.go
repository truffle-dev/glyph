// Package treeview renders a generic, recursive collapsible tree of nodes.
//
// Where components/file-tree is shaped specifically for directories (icons,
// slash-joined paths, ├/└ branch glyphs, multi-select), treeview is the
// flexible primitive: a Node has a Label, an arbitrary Value, and a slice
// of Children. The same component renders an agent-state tree, a JSON
// document, an org chart, a runtime call graph, a build dependency tree,
// or any other directory-analog data.
//
// The model holds a flat slice of visible rows derived from the root
// honoring the expanded set; the cursor indexes into that slice and the
// visible window scrolls when the cursor moves off the rendered band.
package treeview

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Node is a single position in the tree. Children make a node a branch;
// the absence of children makes it a leaf. Value is the caller's payload,
// returned in SelectMsg.
type Node struct {
	Label    string
	Value    any
	Children []Node
}

// IsBranch reports whether the node has children and can be expanded.
func (n Node) IsBranch() bool { return len(n.Children) > 0 }

// SelectMsg fires when the user presses Enter on the selected row.
type SelectMsg struct {
	Node  Node
	Path  string
	Index int
}

// CursorMsg fires when the cursor moves to a new row.
type CursorMsg struct {
	Index int
}

const (
	expandedGlyph  = "▾"
	collapsedGlyph = "▸"
	leafGlyph      = "·"
)

// Model is the Bubble Tea state of the tree view.
type Model struct {
	th          theme.Theme
	root        Node
	expanded    map[string]bool
	visible     []visibleRow
	cursor      int
	offset      int
	width       int
	height      int
	highlight   bool
	rootVisible bool
	title       string
	placeholder string
	indent      int
}

type visibleRow struct {
	node  Node
	path  string
	depth int
}

// New constructs a Model with Default theme, width 60, height 15, with
// the root row visible and its direct children expanded.
func New() Model {
	return Model{
		th:          theme.Default,
		expanded:    map[string]bool{"": true},
		width:       60,
		height:      15,
		highlight:   true,
		rootVisible: true,
		placeholder: "no nodes",
		indent:      2,
	}
}

// WithTheme overrides the theme palette.
func (m Model) WithTheme(t theme.Theme) Model { m.th = t; return m }

// WithRoot replaces the root node and rebuilds the visible slice.
func (m Model) WithRoot(root Node) Model {
	m.root = root
	m.rebuild()
	return m
}

// WithExpandedDepth expands every branch whose depth is strictly less
// than d. Depth 0 means only the root expands; depth 1 expands root and
// its direct children; and so on.
func (m Model) WithExpandedDepth(d int) Model {
	m.expanded = map[string]bool{}
	walkExpand(m.root, "", 0, d, m.expanded)
	m.rebuild()
	return m
}

// WithExpandAll expands every branch in the tree.
func (m Model) WithExpandAll() Model {
	m.expanded = map[string]bool{}
	walkExpand(m.root, "", 0, -1, m.expanded)
	m.rebuild()
	return m
}

// WithCollapseAll collapses every branch, leaving only the root (when
// visible) on screen.
func (m Model) WithCollapseAll() Model {
	m.expanded = map[string]bool{}
	m.cursor = 0
	m.offset = 0
	m.rebuild()
	return m
}

// WithSize sets the rendered width and visible-row height in cells. Width
// clamps to 8, height to 3.
func (m Model) WithSize(w, h int) Model {
	if w < 8 {
		w = 8
	}
	if h < 3 {
		h = 3
	}
	m.width = w
	m.height = h
	m.clampWindow()
	return m
}

// WithRootVisible hides or shows the root row. When hidden, the root's
// children render at depth 0 and the root itself is never selectable.
func (m Model) WithRootVisible(b bool) Model {
	m.rootVisible = b
	m.rebuild()
	return m
}

// WithHighlightCursor toggles the selected-row background highlight.
func (m Model) WithHighlightCursor(b bool) Model { m.highlight = b; return m }

// WithTitle sets an optional muted header line shown above the tree.
func (m Model) WithTitle(s string) Model { m.title = s; return m }

// WithPlaceholder overrides the empty-state text.
func (m Model) WithPlaceholder(s string) Model { m.placeholder = s; return m }

// WithIndent sets the per-level indent in cells. Default 2.
func (m Model) WithIndent(n int) Model {
	if n < 0 {
		n = 0
	}
	m.indent = n
	return m
}

// Cursor returns the current cursor index into the visible rows.
func (m Model) Cursor() int { return m.cursor }

// SelectedPath returns the slash-joined index path of the row under the
// cursor; the second value is false when no row is selectable.
func (m Model) SelectedPath() (string, bool) {
	if cur, ok := m.current(); ok {
		return cur.path, true
	}
	return "", false
}

// SelectedNode returns the Node under the cursor; second value is false
// when no row is selectable.
func (m Model) SelectedNode() (Node, bool) {
	if cur, ok := m.current(); ok {
		return cur.node, true
	}
	return Node{}, false
}

// IsExpanded reports whether the branch at the given path is currently open.
func (m Model) IsExpanded(path string) bool { return m.expanded[path] }

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update routes the Bubble Tea messages this component understands:
// arrow keys (or j/k) move the cursor, PgUp/PgDn move by half a screen,
// Home/g and End/G jump to the ends, Right/l expands the current branch,
// Left/h collapses or jumps to the parent, Enter toggles a branch and
// emits SelectMsg, Space toggles a branch without emitting.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampWindow()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
				return m, m.emitCursor()
			}
		case "down", "j":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
				m.ensureCursorVisible()
				return m, m.emitCursor()
			}
		case "pgup":
			step := m.pageStep()
			m.cursor -= step
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.ensureCursorVisible()
			return m, m.emitCursor()
		case "pgdown":
			step := m.pageStep()
			m.cursor += step
			if m.cursor > len(m.visible)-1 {
				m.cursor = len(m.visible) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.ensureCursorVisible()
			return m, m.emitCursor()
		case "home", "g":
			m.cursor = 0
			m.offset = 0
			return m, m.emitCursor()
		case "end", "G":
			if len(m.visible) > 0 {
				m.cursor = len(m.visible) - 1
				m.ensureCursorVisible()
				return m, m.emitCursor()
			}
		case "right", "l":
			if cur, ok := m.current(); ok && cur.node.IsBranch() && !m.expanded[cur.path] {
				m.expanded[cur.path] = true
				m.rebuild()
				m.ensureCursorVisible()
			}
			return m, nil
		case "left", "h":
			if cur, ok := m.current(); ok {
				if cur.node.IsBranch() && m.expanded[cur.path] {
					delete(m.expanded, cur.path)
					m.rebuild()
					m.ensureCursorVisible()
					return m, nil
				}
				if pi := strings.LastIndex(cur.path, "/"); pi >= 0 {
					parent := cur.path[:pi]
					for i, row := range m.visible {
						if row.path == parent {
							m.cursor = i
							m.ensureCursorVisible()
							return m, m.emitCursor()
						}
					}
				}
			}
		case "enter":
			if cur, ok := m.current(); ok {
				if cur.node.IsBranch() {
					if m.expanded[cur.path] {
						delete(m.expanded, cur.path)
					} else {
						m.expanded[cur.path] = true
					}
					m.rebuild()
					m.ensureCursorVisible()
				}
				return m, m.emitSelect(cur)
			}
		case " ":
			if cur, ok := m.current(); ok && cur.node.IsBranch() {
				if m.expanded[cur.path] {
					delete(m.expanded, cur.path)
				} else {
					m.expanded[cur.path] = true
				}
				m.rebuild()
				m.ensureCursorVisible()
			}
		}
	}
	return m, nil
}

// View renders the title (if any), then the visible window of rows.
func (m Model) View() string {
	if len(m.visible) == 0 {
		return lipgloss.NewStyle().
			Foreground(m.th.TextMuted).
			Width(m.width).
			Render(m.placeholder)
	}

	var b strings.Builder
	if m.title != "" {
		b.WriteString(lipgloss.NewStyle().
			Foreground(m.th.TextMuted).
			Render(m.title))
		b.WriteString("\n")
	}
	end := m.offset + m.height
	if end > len(m.visible) {
		end = len(m.visible)
	}
	for i := m.offset; i < end; i++ {
		if i > m.offset {
			b.WriteString("\n")
		}
		b.WriteString(m.renderRow(m.visible[i], i == m.cursor))
	}
	return b.String()
}

func (m Model) renderRow(row visibleRow, selected bool) string {
	pad := strings.Repeat(" ", row.depth*m.indent)

	glyph := leafGlyph
	glyphColor := m.th.TextMuted
	if row.node.IsBranch() {
		if m.expanded[row.path] {
			glyph = expandedGlyph
		} else {
			glyph = collapsedGlyph
		}
		glyphColor = m.th.Primary
	}
	glyphStr := lipgloss.NewStyle().Foreground(glyphColor).Render(glyph)

	label := row.node.Label
	prefix := pad + glyphStr + " "
	used := lipgloss.Width(prefix)
	budget := m.width - used
	if budget < 1 {
		budget = 1
	}
	if lipgloss.Width(label) > budget {
		label = truncCell(label, budget)
	}
	base := prefix + label

	if selected && m.highlight {
		return lipgloss.NewStyle().
			Background(m.th.SurfaceStrong).
			Foreground(m.th.Text).
			Bold(true).
			Width(m.width).
			Render(base)
	}
	return base
}

func (m Model) pageStep() int {
	step := m.height / 2
	if step < 1 {
		return 1
	}
	return step
}

func (m Model) emitSelect(row visibleRow) tea.Cmd {
	idx := m.cursor
	return func() tea.Msg {
		return SelectMsg{Node: row.node, Path: row.path, Index: idx}
	}
}

func (m Model) emitCursor() tea.Cmd {
	idx := m.cursor
	return func() tea.Msg { return CursorMsg{Index: idx} }
}

func (m *Model) current() (visibleRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return visibleRow{}, false
	}
	return m.visible[m.cursor], true
}

func (m *Model) rebuild() {
	m.visible = m.visible[:0]
	if m.rootVisible {
		m.walk(m.root, "", 0)
	} else {
		for i, c := range m.root.Children {
			m.walk(c, fmt.Sprintf("%d", i), 0)
		}
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampWindow()
}

func (m *Model) walk(n Node, path string, depth int) {
	m.visible = append(m.visible, visibleRow{node: n, path: path, depth: depth})
	if n.IsBranch() && m.expanded[path] {
		for i, c := range n.Children {
			cp := childPath(path, i)
			m.walk(c, cp, depth+1)
		}
	}
}

func walkExpand(n Node, path string, depth, maxDepth int, expanded map[string]bool) {
	if !n.IsBranch() {
		return
	}
	if maxDepth >= 0 && depth >= maxDepth {
		return
	}
	expanded[path] = true
	for i, c := range n.Children {
		walkExpand(c, childPath(path, i), depth+1, maxDepth, expanded)
	}
}

func childPath(parent string, idx int) string {
	if parent == "" {
		return fmt.Sprintf("%d", idx)
	}
	return fmt.Sprintf("%s/%d", parent, idx)
}

func (m *Model) ensureCursorVisible() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
	m.clampWindow()
}

func (m *Model) clampWindow() {
	if m.offset < 0 {
		m.offset = 0
	}
	max := len(m.visible) - m.height
	if max < 0 {
		max = 0
	}
	if m.offset > max {
		m.offset = max
	}
}

func truncCell(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if max == 1 {
		return "…"
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > max-1 {
			break
		}
		b.WriteRune(r)
		used += w
	}
	b.WriteString("…")
	return b.String()
}
