// code-editor is the proto-IDE end-to-end demo. It composes seven
// glyph components into something that already looks like a terminal-
// native code editor: a file-tree on the left, a tab row on top, an
// editable text buffer below the tabs, a find-bar overlay you can pop
// with Ctrl-F, and a status-bar + key-hints pinned to the bottom.
//
// The shape is intentional. Every editor on the planet — Cursor, VS
// Code, Sublime, vim — is some variation on this layout. Once the
// glyph components below render the layout, they render every editor.
//
// Keys:
//
//	up / down               move the focused panel's cursor
//	enter                   on a tree leaf, open the file in a tab
//	tab / shift+tab         cycle open tabs
//	ctrl-f                  open the find-bar
//	esc                     close the find-bar
//	ctrl-w                  close the active tab
//	ctrl-l                  focus the file tree
//	ctrl-e                  focus the editor
//	q / ctrl-c              quit
package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	codeview "github.com/truffle-dev/glyph/components/code-view"
	"github.com/truffle-dev/glyph/components/editor"
	filetree "github.com/truffle-dev/glyph/components/file-tree"
	findbar "github.com/truffle-dev/glyph/components/find-bar"
	keyhints "github.com/truffle-dev/glyph/components/key-hints"
	"github.com/truffle-dev/glyph/components/panel"
	statusbar "github.com/truffle-dev/glyph/components/status-bar"
	"github.com/truffle-dev/glyph/components/tabs"
	"github.com/truffle-dev/glyph/components/theme"
)

type focus int

const (
	focusTree focus = iota
	focusEditor
	focusFindBar
)

// fileEntry is the seed payload for each leaf in the tree.
type fileEntry struct {
	Path     string
	Language codeview.Language
	Source   string
}

// openBuffer is one editor instance plus the metadata needed to label
// it on the tab row and track dirtiness.
type openBuffer struct {
	entry fileEntry
	ed    editor.Model
}

func (b openBuffer) tabLabel() string {
	name := b.entry.Path
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if b.ed.Dirty() {
		name = "• " + name
	}
	return name
}

type model struct {
	width, height int

	tree    filetree.Model
	tabs    tabs.Tabs
	status  statusbar.Bar
	hints   keyhints.Bar
	bar     findbar.Bar
	files   map[string]fileEntry
	bufs    []*openBuffer
	active  int
	focus   focus
	barOpen bool
}

func newModel() model {
	t := theme.Default

	files := seedFiles()
	tree := filetree.New(seedTree()).WithTitle("project/")
	tree.Expand("cmd")
	tree.Expand("internal")
	tree.Expand("internal/store")
	tree.SetCursor("cmd/main.go")

	// Pre-open one buffer so the demo greets you with a file.
	first := newBuffer(t, files["cmd/main.go"])

	tabRow := tabs.New(t).WithTabs([]string{first.tabLabel()})

	bar := findbar.New(t).WithWidth(56).Blur()

	status := statusbar.New(t).
		WithLeft(
			statusbar.Item{Text: "● ready", Style: statusbar.StyleSuccess},
			statusbar.Item{Text: "editor", Style: statusbar.StyleDefault},
		).
		WithCenter(
			statusbar.Item{Text: first.entry.Path, Style: statusbar.StyleMuted},
		).
		WithRight(
			statusbar.Item{Text: string(first.entry.Language), Style: statusbar.StylePrimary},
			statusbar.Item{Text: "Ln 1, Col 1", Style: statusbar.StyleMuted},
		)

	hints := keyhints.New(t).WithHints([]keyhints.Hint{
		{Key: "Ctrl-L", Desc: "tree"},
		{Key: "Ctrl-E", Desc: "edit"},
		{Key: "Tab", Desc: "next tab"},
		{Key: "Ctrl-F", Desc: "find"},
		{Key: "Ctrl-W", Desc: "close"},
		{Key: "q", Desc: "quit"},
	})

	return model{
		tree:   tree,
		tabs:   tabRow,
		status: status,
		hints:  hints,
		bar:    bar,
		files:  files,
		bufs:   []*openBuffer{first},
		active: 0,
		focus:  focusEditor,
	}
}

func newBuffer(t theme.Theme, e fileEntry) *openBuffer {
	ed := editor.New(t).
		WithContent(e.Source).
		WithLanguage(e.Language)
	return &openBuffer{entry: e, ed: ed}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m.resizeEditors(), nil

	case findbar.CloseMsg:
		m.barOpen = false
		m.bar = m.bar.Blur()
		m.focus = focusEditor
		return m, nil

	case findbar.QueryMsg:
		// Walk the active buffer for matches and reseat the bar.
		if b := m.currentBuffer(); b != nil {
			lines := strings.Split(b.ed.Value(), "\n")
			matches := findbar.FindMatches(lines, msg.Value, m.bar.CaseSensitive())
			m.bar = m.bar.WithMatches(matches, 0)
		}
		return m, nil

	case findbar.NextMsg:
		if m.bar.MatchCount() > 0 {
			next := (m.bar.Current() + 1) % m.bar.MatchCount()
			b := m.currentBuffer()
			lines := strings.Split(b.ed.Value(), "\n")
			matches := findbar.FindMatches(lines, m.bar.Query(), m.bar.CaseSensitive())
			m.bar = m.bar.WithMatches(matches, next)
		}
		return m, nil

	case findbar.PrevMsg:
		if m.bar.MatchCount() > 0 {
			prev := m.bar.Current() - 1
			if prev < 0 {
				prev = m.bar.MatchCount() - 1
			}
			b := m.currentBuffer()
			lines := strings.Split(b.ed.Value(), "\n")
			matches := findbar.FindMatches(lines, m.bar.Query(), m.bar.CaseSensitive())
			m.bar = m.bar.WithMatches(matches, prev)
		}
		return m, nil

	case tea.KeyMsg:
		// Global keys handled before delegating to the focused panel.
		switch msg.String() {
		case "ctrl+c", "q":
			if m.focus != focusEditor && !m.barOpen {
				return m, tea.Quit
			}
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		case "ctrl+l":
			m.focus = focusTree
			return m, nil
		case "ctrl+e":
			m.focus = focusEditor
			return m, nil
		case "ctrl+f":
			m.barOpen = true
			m.bar = m.bar.Focus()
			m.focus = focusFindBar
			return m, nil
		case "ctrl+w":
			return m.closeActiveTab(), nil
		case "tab", "shift+tab":
			if m.focus != focusFindBar {
				m.tabs, _ = m.tabs.Update(msg)
				m.active = m.tabs.Active()
				m = m.syncStatus()
				return m, nil
			}
		}

		switch m.focus {
		case focusTree:
			return m.routeToTree(msg)
		case focusEditor:
			return m.routeToEditor(msg)
		case focusFindBar:
			var cmd tea.Cmd
			m.bar, cmd = m.bar.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m model) routeToTree(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		path := m.tree.Selected()
		if entry, ok := m.files[path]; ok {
			m = m.openFile(entry)
			return m, nil
		}
		// Fall through to the tree so dir nodes expand/collapse.
	}
	var cmd tea.Cmd
	m.tree, cmd = m.tree.Update(msg)
	return m, cmd
}

func (m model) routeToEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if b := m.currentBuffer(); b != nil {
		ed, cmd := b.ed.Update(msg)
		b.ed = ed
		// Refresh tab label in case Dirty() flipped.
		m.tabs = m.tabs.WithTabs(m.tabLabels()).WithActive(m.active)
		m = m.syncStatus()
		return m, cmd
	}
	return m, nil
}

func (m model) openFile(e fileEntry) model {
	// If already open, just switch tab.
	for i, b := range m.bufs {
		if b.entry.Path == e.Path {
			m.active = i
			m.tabs = m.tabs.WithActive(i)
			m = m.syncStatus()
			return m
		}
	}
	b := newBuffer(theme.Default, e)
	m.bufs = append(m.bufs, b)
	m.active = len(m.bufs) - 1
	m.tabs = m.tabs.WithTabs(m.tabLabels()).WithActive(m.active)
	m = m.resizeEditors()
	m = m.syncStatus()
	return m
}

func (m model) closeActiveTab() model {
	if len(m.bufs) <= 1 {
		return m
	}
	m.bufs = append(m.bufs[:m.active], m.bufs[m.active+1:]...)
	if m.active >= len(m.bufs) {
		m.active = len(m.bufs) - 1
	}
	m.tabs = m.tabs.WithTabs(m.tabLabels()).WithActive(m.active)
	m = m.syncStatus()
	return m
}

func (m model) tabLabels() []string {
	out := make([]string, len(m.bufs))
	for i, b := range m.bufs {
		out[i] = b.tabLabel()
	}
	return out
}

func (m model) currentBuffer() *openBuffer {
	if m.active < 0 || m.active >= len(m.bufs) {
		return nil
	}
	return m.bufs[m.active]
}

func (m model) syncStatus() model {
	b := m.currentBuffer()
	if b == nil {
		return m
	}
	row, col := b.ed.Cursor()
	dirtyMark := ""
	if b.ed.Dirty() {
		dirtyMark = " · modified"
	}
	m.status = m.status.
		WithCenter(statusbar.Item{Text: b.entry.Path + dirtyMark, Style: statusbar.StyleMuted}).
		WithRight(
			statusbar.Item{Text: string(b.entry.Language), Style: statusbar.StylePrimary},
			statusbar.Item{Text: fmt.Sprintf("Ln %d, Col %d", row+1, col+1), Style: statusbar.StyleMuted},
		)
	return m
}

func (m model) resizeEditors() model {
	if m.width == 0 || m.height == 0 {
		return m
	}
	leftW := 30
	if m.width < 100 {
		leftW = m.width / 3
	}
	rightW := m.width - leftW - 1
	if rightW < 32 {
		rightW = 32
	}
	// Reserve rows: 1 status + 1 hints + tab row + panel borders (≈4).
	bodyH := m.height - 2
	if bodyH < 8 {
		bodyH = 8
	}
	editorH := bodyH - 4
	if editorH < 6 {
		editorH = 6
	}
	editorW := rightW - 4
	if editorW < 24 {
		editorW = 24
	}
	for _, b := range m.bufs {
		b.ed = b.ed.WithWidth(editorW).WithHeight(editorH)
	}
	return m
}

func (m model) View() string {
	if m.width == 0 {
		return "loading..."
	}
	t := theme.Default

	leftW := 30
	if m.width < 100 {
		leftW = m.width / 3
	}
	rightW := m.width - leftW - 1
	bodyH := m.height - 2
	if bodyH < 8 {
		bodyH = 8
	}

	leftTitle := "files"
	if m.focus == focusTree {
		leftTitle = "files (focused)"
	}
	leftPanel := panel.New(t).
		WithTitle(leftTitle).
		WithContent(m.tree.View()).
		WithWidth(leftW).
		WithHeight(bodyH).
		WithPadding(1, 0).
		WithVariant(panel.VariantStrong)

	// Right side: tabs row above the editor.
	cur := m.currentBuffer()
	editorBody := "(no file open)"
	if cur != nil {
		editorBody = cur.ed.View()
	}

	tabRow := m.tabs.WithWidth(rightW - 4).View()

	rightInner := lipgloss.JoinVertical(lipgloss.Left,
		tabRow,
		"",
		editorBody,
	)

	rightTitle := "editor"
	if m.focus == focusEditor {
		rightTitle = "editor (focused)"
	}
	if cur != nil {
		rightTitle = cur.entry.Path
	}
	rightPanel := panel.New(t).
		WithTitle(rightTitle).
		WithContent(rightInner).
		WithWidth(rightW).
		WithHeight(bodyH).
		WithPadding(1, 0).
		WithVariant(panel.VariantStrong)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel.View(), " ", rightPanel.View())

	// Find-bar overlays beneath the body when open.
	if m.barOpen {
		barRow := lipgloss.NewStyle().PaddingLeft(leftW + 2).Render(m.bar.View())
		body = lipgloss.JoinVertical(lipgloss.Left, body, barRow)
	}

	m.status = m.status.WithWidth(m.width)
	m.hints = m.hints.WithWidth(m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		body,
		m.status.View(),
		m.hints.View(),
	)
}

// --- fixture ----------------------------------------------------------

func seedTree() filetree.Node {
	return filetree.Node{
		Name: "project",
		Children: []filetree.Node{
			{Name: "cmd", Children: []filetree.Node{
				{Name: "main.go", Meta: "1.2 kB"},
				{Name: "root.go", Meta: "780 B"},
			}},
			{Name: "internal", Children: []filetree.Node{
				{Name: "store", Children: []filetree.Node{
					{Name: "sqlite.go", Meta: "3.0 kB"},
				}},
				{Name: "agent.go", Meta: "5.6 kB"},
			}},
			{Name: "go.mod", Meta: "modified"},
			{Name: "README.md", Meta: "8.4 kB"},
		},
	}
}

func seedFiles() map[string]fileEntry {
	return map[string]fileEntry{
		"cmd/main.go": {
			Path:     "cmd/main.go",
			Language: codeview.LangGo,
			Source: `package main

import (
	"context"
	"fmt"

	"project/internal"
)

func main() {
	ctx := context.Background()
	a, err := internal.NewAgent(ctx, "production")
	if err != nil {
		fmt.Println("boot failed:", err)
		return
	}
	a.Run(ctx)
}`,
		},
		"cmd/root.go": {
			Path:     "cmd/root.go",
			Language: codeview.LangGo,
			Source: `package main

import "fmt"

func printBanner(version string) {
	fmt.Println("project", version)
}`,
		},
		"internal/store/sqlite.go": {
			Path:     "internal/store/sqlite.go",
			Language: codeview.LangGo,
			Source: `package store

import (
	"database/sql"
	"errors"

	_ "modernc.org/sqlite"
)

// Open returns a sqlite-backed Store at the given path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, errors.New("open: " + err.Error())
	}
	return &Store{db: db}, nil
}`,
		},
		"internal/agent.go": {
			Path:     "internal/agent.go",
			Language: codeview.LangGo,
			Source: `package internal

import (
	"context"

	"project/internal/store"
)

// Agent is the per-process supervisor.
type Agent struct {
	Name  string
	store store.Store
}

func NewAgent(ctx context.Context, name string) (*Agent, error) {
	return &Agent{Name: name}, nil
}

func (a *Agent) Run(ctx context.Context) error {
	return nil
}`,
		},
		"go.mod": {
			Path:     "go.mod",
			Language: codeview.LangPlain,
			Source: `module project

go 1.24

require (
	github.com/charmbracelet/bubbletea v1.3.10
	modernc.org/sqlite v1.40.0
)`,
		},
		"README.md": {
			Path:     "README.md",
			Language: codeview.LangPlain,
			Source: `# project

A small operator surface built on glyph.

## Build

    go build ./cmd

## Test

    go test ./...`,
		},
	}
}
