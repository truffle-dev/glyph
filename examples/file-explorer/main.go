// file-explorer is an IDE-style two-pane navigator composed from five
// glyph components: a file-tree on the left, a breadcrumb above a
// syntax-tinted code-view on the right, all wrapped in panels with a
// status-bar and key-hints pinned to the bottom.
//
// The shape is the kind of operator surface every code-aware tool ends
// up building: a queue of artifacts on one side, a focused preview on
// the other. Here it's wired to an in-memory fixture so the example is
// a single-binary demo with no backing filesystem.
//
// Keys:
//
//	up / down / j / k       move the tree cursor
//	right / l               expand directory
//	left / h                collapse directory (or jump to parent on leaf)
//	enter                   toggle expand on a dir; "open" on a leaf
//	q / ctrl-c              quit
package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/breadcrumb"
	codeview "github.com/truffle-dev/glyph/components/code-view"
	filetree "github.com/truffle-dev/glyph/components/file-tree"
	keyhints "github.com/truffle-dev/glyph/components/key-hints"
	"github.com/truffle-dev/glyph/components/panel"
	statusbar "github.com/truffle-dev/glyph/components/status-bar"
	"github.com/truffle-dev/glyph/components/theme"
)

// fileEntry is the fixture payload attached to leaf nodes. Keeping the
// source inline as a Go string means the demo runs without any I/O.
type fileEntry struct {
	Path     string
	Language codeview.Language
	Source   string
	// FocusLine is 1-based; the line that should land under the
	// MarkHighlight cursor when the file opens.
	FocusLine int
}

// model is the file-explorer.
type model struct {
	width, height int

	tree    filetree.Model
	status  statusbar.Bar
	hints   keyhints.Bar
	files   map[string]fileEntry
	current fileEntry
}

func newModel() model {
	t := theme.Default

	files := seedFiles()

	tree := filetree.New(seedTree()).
		WithTitle("project/")

	// Pre-open the cmd/ directory so the demo opens with a file in view.
	tree.Expand("cmd")
	tree.Expand("internal")
	tree.Expand("internal/store")
	tree.SetCursor("cmd/main.go")

	status := statusbar.New(t).
		WithLeft(
			statusbar.Item{Text: "● ready", Style: statusbar.StyleSuccess},
			statusbar.Item{Text: "explorer", Style: statusbar.StyleDefault},
		).
		WithCenter(
			statusbar.Item{Text: "project / cmd / main.go", Style: statusbar.StyleMuted},
		).
		WithRight(
			statusbar.Item{Text: "go", Style: statusbar.StylePrimary},
			statusbar.Item{Text: "Ln 7", Style: statusbar.StyleMuted},
		)

	hints := keyhints.New(t).WithHints([]keyhints.Hint{
		{Key: "↑↓", Desc: "move"},
		{Key: "→", Desc: "expand"},
		{Key: "←", Desc: "collapse"},
		{Key: "Enter", Desc: "open"},
		{Key: "q", Desc: "quit"},
	})

	m := model{
		tree:    tree,
		status:  status,
		hints:   hints,
		files:   files,
		current: files["cmd/main.go"],
	}
	return m
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.tree, cmd = m.tree.Update(msg)
		// Sync the preview pane to whatever row the cursor lands on.
		m = m.syncPreview()
		return m, cmd
	}
	return m, nil
}

func (m model) syncPreview() model {
	path := m.tree.Selected()
	if entry, ok := m.files[path]; ok {
		m.current = entry
		// Patch status-bar center to the open path.
		m.status = m.status.WithCenter(
			statusbar.Item{Text: prettyCrumbs(path), Style: statusbar.StyleMuted},
		)
		// Right-side language + line indicator.
		lang := "txt"
		if entry.Language != "" {
			lang = string(entry.Language)
		}
		m.status = m.status.WithRight(
			statusbar.Item{Text: lang, Style: statusbar.StylePrimary},
			statusbar.Item{Text: lineLabel(entry), Style: statusbar.StyleMuted},
		)
	}
	return m
}

func (m model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	t := theme.Default

	// Layout: 32-col left pane, the rest for the right pane.
	leftW := 34
	if m.width < 100 {
		leftW = m.width / 3
	}
	rightW := m.width - leftW - 1 // 1-cell gap

	// Reserve rows: 2 for status+hints at the bottom.
	bodyH := m.height - 2
	if bodyH < 8 {
		bodyH = 8
	}

	// --- left pane: file tree ---------------------------------------------
	leftPanel := panel.New(t).
		WithTitle("files").
		WithContent(m.tree.View()).
		WithWidth(leftW).
		WithHeight(bodyH).
		WithPadding(1, 0).
		WithVariant(panel.VariantStrong)

	// --- right pane: breadcrumb + code-view --------------------------------
	cur := m.current
	crumbs := breadcrumb.RenderPath(cur.Path, breadcrumb.Options{MaxItems: 6})
	if cur.Path == "" {
		crumbs = lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Render("(select a file)")
	}

	codeBody := codeview.Render(codeview.Block{
		Source:     cur.Source,
		Language:   cur.Language,
		ShowGutter: true,
		StartLine:  1,
		Marks:      map[int]codeview.LineMark{cur.FocusLine: codeview.MarkHighlight},
		MaxWidth:   rightW - 4, // leave 2 pad cells on each side
	})

	rightInner := lipgloss.JoinVertical(lipgloss.Left,
		crumbs,
		"",
		codeBody,
	)

	rightPanel := panel.New(t).
		WithTitle(cur.Path).
		WithContent(rightInner).
		WithWidth(rightW).
		WithHeight(bodyH).
		WithPadding(1, 0).
		WithVariant(panel.VariantStrong)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel.View(), " ", rightPanel.View())

	// --- footer: status + hints --------------------------------------------
	m.status = m.status.WithWidth(m.width)
	m.hints = m.hints.WithWidth(m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		body,
		m.status.View(),
		m.hints.View(),
	)
}

// --- tree + file fixture -----------------------------------------------

func seedTree() filetree.Node {
	return filetree.Node{
		Name: "project",
		Children: []filetree.Node{
			{Name: "cmd", Children: []filetree.Node{
				{Name: "main.go", Meta: "1.4 kB"},
				{Name: "root.go", Meta: "892 B"},
			}},
			{Name: "internal", Children: []filetree.Node{
				{Name: "store", Children: []filetree.Node{
					{Name: "sqlite.go", Meta: "3.1 kB"},
					{Name: "memory.go", Meta: "1.8 kB"},
				}},
				{Name: "agent.go", Meta: "5.6 kB"},
				{Name: "config.go", Meta: "780 B"},
			}},
			{Name: "scripts", Children: []filetree.Node{
				{Name: "deploy.sh", Meta: "412 B"},
			}},
			{Name: "go.mod", Meta: "modified"},
			{Name: "README.md", Meta: "8.4 kB"},
			{Name: "config.json", Meta: "256 B"},
		},
	}
}

func seedFiles() map[string]fileEntry {
	return map[string]fileEntry{
		"cmd/main.go": {
			Path:      "cmd/main.go",
			Language:  codeview.LangGo,
			FocusLine: 9,
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
			Path:      "cmd/root.go",
			Language:  codeview.LangGo,
			FocusLine: 5,
			Source: `package main

import "fmt"

func printBanner(version string) {
	fmt.Println("project", version)
}`,
		},
		"internal/store/sqlite.go": {
			Path:      "internal/store/sqlite.go",
			Language:  codeview.LangGo,
			FocusLine: 11,
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
		"internal/store/memory.go": {
			Path:      "internal/store/memory.go",
			Language:  codeview.LangGo,
			FocusLine: 7,
			Source: `package store

import "sync"

type memStore struct {
	mu   sync.RWMutex
	rows map[string][]byte
}

func NewMemory() *memStore { return &memStore{rows: map[string][]byte{}} }`,
		},
		"internal/agent.go": {
			Path:      "internal/agent.go",
			Language:  codeview.LangGo,
			FocusLine: 12,
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
}`,
		},
		"internal/config.go": {
			Path:      "internal/config.go",
			Language:  codeview.LangGo,
			FocusLine: 5,
			Source: `package internal

import "os"

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}`,
		},
		"scripts/deploy.sh": {
			Path:      "scripts/deploy.sh",
			Language:  codeview.LangBash,
			FocusLine: 4,
			Source: `#!/usr/bin/env bash
set -euo pipefail

readonly TAG="${1:?missing tag}"
echo "deploying $TAG"
docker buildx build --push --tag "ghcr.io/truffle/project:$TAG" .`,
		},
		"go.mod": {
			Path:      "go.mod",
			Language:  codeview.LangPlain,
			FocusLine: 1,
			Source: `module project

go 1.24

require (
	github.com/charmbracelet/bubbletea v1.3.10
	modernc.org/sqlite v1.40.0
)`,
		},
		"README.md": {
			Path:      "README.md",
			Language:  codeview.LangPlain,
			FocusLine: 1,
			Source: `# project

A small operator surface built on glyph.

## Build

    go build ./cmd

## Test

    go test ./...`,
		},
		"config.json": {
			Path:      "config.json",
			Language:  codeview.LangJSON,
			FocusLine: 3,
			Source: `{
  "name": "project",
  "version": "0.3.0",
  "private": false
}`,
		},
	}
}

// prettyCrumbs maps a path into a glyph-style center label, e.g.
// "cmd/main.go" → "project / cmd / main.go".
func prettyCrumbs(path string) string {
	if path == "" {
		return "project"
	}
	out := "project"
	cur := ""
	for _, c := range path {
		if c == '/' {
			out += " / " + cur
			cur = ""
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out += " / " + cur
	}
	return out
}

func lineLabel(e fileEntry) string {
	if e.FocusLine <= 0 {
		return ""
	}
	return "Ln " + itoa(e.FocusLine)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	if neg {
		out = "-" + out
	}
	return out
}
