// Package filetree renders an interactive directory tree.
//
// The model holds a flat slice of visible nodes and tracks the cursor,
// the set of expanded directories, and an optional selection set for
// multi-select operations. Expand/collapse, navigation, and selection
// all run through the standard Bubble Tea Update loop.
//
// The tree is constructed from a nested Node literal; consumers own the
// data shape, so the same component renders a project file system, a
// JSON document, a Linear/Notion outline, or anything else with a
// directory analogy.
package filetree

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Node is a directory or leaf in the tree. Children make a node a directory.
type Node struct {
	Name     string
	Children []Node
	// Icon overrides the default folder/file glyph. Optional.
	Icon string
	// Meta is appended muted to the right of Name (e.g. "12kb", "modified").
	Meta string
}

// IsDir reports whether the node is a directory.
func (n Node) IsDir() bool { return n.Children != nil }

// Model is the Bubble Tea state of the file tree.
type Model struct {
	Root  Node
	Title string

	cursor    int
	expanded  map[string]bool // keyed by full path
	visible   []visibleRow
	selected  map[string]bool
	multi     bool
	width     int
	height    int
	scrollOff int
}

// SelectMsg fires when the user presses Enter on a leaf (file) or when a
// directory is opened/closed. Path is the slash-joined path from the root.
type SelectMsg struct {
	Path  string
	IsDir bool
}

// New constructs a Model with the root expanded.
func New(root Node) Model {
	m := Model{
		Root:     root,
		expanded: map[string]bool{"": true},
		selected: map[string]bool{},
	}
	m.rebuild()
	return m
}

// WithTitle sets a header line shown above the tree.
func (m Model) WithTitle(t string) Model { m.Title = t; return m }

// WithMultiSelect enables space-toggle selection.
func (m Model) WithMultiSelect(b bool) Model { m.multi = b; return m }

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update handles arrow keys, expand/collapse (right/left or l/h), and
// Enter / space.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampCursor()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
			}
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = len(m.visible) - 1
		case "right", "l":
			if cur, ok := m.current(); ok && cur.node.IsDir() {
				if !m.expanded[cur.path] {
					m.expanded[cur.path] = true
					m.rebuild()
					return m, m.emitSelect(cur, true)
				}
			}
		case "left", "h":
			if cur, ok := m.current(); ok && cur.node.IsDir() && m.expanded[cur.path] {
				delete(m.expanded, cur.path)
				m.rebuild()
				return m, m.emitSelect(cur, false)
			} else if ok && cur.path != "" {
				// Jump to parent directory.
				if pi := strings.LastIndex(cur.path, "/"); pi >= 0 {
					parent := cur.path[:pi]
					for i, row := range m.visible {
						if row.path == parent {
							m.cursor = i
							break
						}
					}
				} else {
					m.cursor = 0
				}
			}
		case "enter":
			if cur, ok := m.current(); ok {
				if cur.node.IsDir() {
					if m.expanded[cur.path] {
						delete(m.expanded, cur.path)
					} else {
						m.expanded[cur.path] = true
					}
					m.rebuild()
				}
				return m, m.emitSelect(cur, m.expanded[cur.path])
			}
		case " ":
			if m.multi {
				if cur, ok := m.current(); ok {
					if m.selected[cur.path] {
						delete(m.selected, cur.path)
					} else {
						m.selected[cur.path] = true
					}
				}
			}
		}
	}
	return m, nil
}

// View renders the tree. Title (if set) is on the first line, followed
// by visible rows. Long names are truncated to width-3 with an ellipsis.
func (m Model) View() string {
	if len(m.visible) == 0 {
		return lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Render("(empty tree)")
	}

	parts := []string{}
	if m.Title != "" {
		parts = append(parts, lipgloss.NewStyle().
			Foreground(theme.Default.TextMuted).
			Render(m.Title))
	}

	max := len(m.visible)
	for i := 0; i < max; i++ {
		row := m.visible[i]
		parts = append(parts, m.renderRow(row, i == m.cursor))
	}
	return strings.Join(parts, "\n")
}

// Selected returns the slash-joined path under the cursor, or "" if the
// tree is empty.
func (m Model) Selected() string {
	if cur, ok := m.current(); ok {
		return cur.path
	}
	return ""
}

// SelectedNode returns the Node under the cursor; second value is false
// when the tree is empty.
func (m Model) SelectedNode() (Node, bool) {
	if cur, ok := m.current(); ok {
		return cur.node, true
	}
	return Node{}, false
}

// IsSelected reports whether the given path is in the multi-select set.
func (m Model) IsSelected(path string) bool { return m.selected[path] }

// SelectedPaths returns paths in the multi-select set, sorted by visible order.
func (m Model) SelectedPaths() []string {
	out := []string{}
	for _, row := range m.visible {
		if m.selected[row.path] {
			out = append(out, row.path)
		}
	}
	return out
}

// SetCursor moves the cursor to the row whose path matches; no-op if absent.
func (m *Model) SetCursor(path string) {
	for i, row := range m.visible {
		if row.path == path {
			m.cursor = i
			return
		}
	}
}

// Expand marks a directory open and rebuilds the visible slice.
func (m *Model) Expand(path string) { m.expanded[path] = true; m.rebuild() }

// Collapse removes a directory from the expanded set and rebuilds.
func (m *Model) Collapse(path string) { delete(m.expanded, path); m.rebuild() }

// --- internals ----------------------------------------------------------

type visibleRow struct {
	node  Node
	path  string
	depth int
	isDir bool
	last  bool // is this the last child of its parent? Used for ├ vs └.
}

func (m Model) current() (visibleRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return visibleRow{}, false
	}
	return m.visible[m.cursor], true
}

func (m *Model) clampCursor() {
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// rebuild walks the tree honoring m.expanded and rebuilds m.visible.
func (m *Model) rebuild() {
	m.visible = m.visible[:0]
	m.walk(m.Root, "", 0, true)
	m.clampCursor()
}

func (m *Model) walk(n Node, path string, depth int, last bool) {
	if depth > 0 || m.Root.Name != "" {
		m.visible = append(m.visible, visibleRow{
			node:  n,
			path:  path,
			depth: depth,
			isDir: n.IsDir(),
			last:  last,
		})
	}
	if n.IsDir() && (depth == 0 || m.expanded[path]) {
		for i, c := range n.Children {
			childPath := path + "/" + c.Name
			if path == "" {
				childPath = c.Name
			}
			m.walk(c, childPath, depth+1, i == len(n.Children)-1)
		}
	}
}

func (m Model) emitSelect(cur visibleRow, isOpen bool) tea.Cmd {
	return func() tea.Msg {
		return SelectMsg{Path: cur.path, IsDir: cur.isDir}
	}
}

func (m Model) renderRow(row visibleRow, cursor bool) string {
	prefix := ""
	if row.depth > 1 {
		prefix = strings.Repeat("  ", row.depth-1)
	}

	// Expand/collapse chevron for directories; space for leaves.
	chev := " "
	if row.isDir {
		if m.expanded[row.path] {
			chev = "▾"
		} else {
			chev = "▸"
		}
	}

	icon := row.node.Icon
	if icon == "" {
		if row.isDir {
			icon = "📁"
		} else {
			icon = "📄"
		}
	}

	name := row.node.Name
	if name == "" {
		name = "/"
	}

	checkbox := ""
	if m.multi {
		if m.selected[row.path] {
			checkbox = lipgloss.NewStyle().Foreground(theme.Default.Success).Render("[x] ")
		} else {
			checkbox = lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Render("[ ] ")
		}
	}

	body := chev + " " + icon + " " + name
	if row.node.Meta != "" {
		body += "  " + lipgloss.NewStyle().Foreground(theme.Default.TextMuted).Render(row.node.Meta)
	}

	full := prefix + checkbox + body

	if cursor {
		return lipgloss.NewStyle().
			Foreground(theme.Default.Text).
			Background(theme.Default.SurfaceStrong).
			Bold(true).
			Render(full)
	}
	if row.isDir {
		return lipgloss.NewStyle().Foreground(theme.Default.Primary).Render(full)
	}
	return lipgloss.NewStyle().Foreground(theme.Default.Text).Render(full)
}
