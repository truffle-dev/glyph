// Command release-explorer is an interactive GitHub release browser composed
// from six glyph components: list, tabs, markdown-viewer, status-bar,
// key-hints, and theme.
//
// The shape is the kind of operator surface every release-cutting team
// reaches for: a list of releases on the left, a tabbed detail panel on
// the right that switches between the release body (rendered markdown),
// the asset list, and version metadata.
//
// The demo ships with a synthetic dataset modeled on glyph's own release
// history. The composition (list selects, tabs route, markdown-viewer
// scrolls, status-bar reports) is what the demo is about; the data shape
// is realistic so the surfaces stay legible.
//
// Layout (assumes ~120-col terminal):
//
//	┌ status-bar (app · repo · cursor / total · selected tag) ─────────┐
//	├ list (left, 36 wide) ─────────┬ tabs (Body · Assets · Meta) ─────┤
//	│ › v0.47.0                     │                                  │
//	│   v0.46.0                     │  ...body / assets / meta panel.. │
//	│   v0.45.0                     │  (markdown-viewer for Body)      │
//	│   ...                         │                                  │
//	├ key-hints ───────────────────────────────────────────────────────┤
//
// Keys:
//
//	up / down / k / j     move the release cursor
//	left / right          switch the tab
//	pgup / pgdn / u / d   scroll the body viewer
//	g / G                 first / last release
//	q / ctrl-c            quit
package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	keyhints "github.com/truffle-dev/glyph/components/key-hints"
	"github.com/truffle-dev/glyph/components/list"
	markdownviewer "github.com/truffle-dev/glyph/components/markdown-viewer"
	statusbar "github.com/truffle-dev/glyph/components/status-bar"
	"github.com/truffle-dev/glyph/components/tabs"
	"github.com/truffle-dev/glyph/components/theme"
)

const (
	repoLabel   = "truffle-dev/glyph"
	leftWidth   = 36
	rightWidth  = 72
	rightHeight = 18
)

type asset struct {
	name string
	size string
}

type release struct {
	tag         string
	name        string
	publishedAt string
	body        string
	prerelease  bool
	assets      []asset
}

type model struct {
	th       theme.Theme
	releases []release

	releaseList list.List
	rightTabs   tabs.Tabs
	bodyView    markdownviewer.Viewer

	statusBar statusbar.Bar
	hintsBar  keyhints.Bar
}

func newModel() model {
	th := theme.Default
	releases := fixtureReleases()

	items := make([]list.Item, len(releases))
	for i, r := range releases {
		hint := r.publishedAt[:10]
		if r.prerelease {
			hint += " pre"
		}
		items[i] = list.Item{
			Label: r.tag,
			Hint:  hint,
			Value: i,
		}
	}

	releaseList := list.New(th).
		WithItems(items).
		WithHeight(rightHeight).
		WithWidth(leftWidth)

	rightTabs := tabs.New(th).
		WithTabs([]string{"Body", "Assets", "Meta"}).
		WithActive(0).
		WithWidth(rightWidth)

	bodyView := markdownviewer.New(th).
		WithSize(rightWidth, rightHeight-2).
		WithSource(releases[0].body)

	m := model{
		th:          th,
		releases:    releases,
		releaseList: releaseList,
		rightTabs:   rightTabs,
		bodyView:    bodyView,
		statusBar:   statusbar.New(th).WithWidth(leftWidth + rightWidth + 3),
		hintsBar:    keyhints.New(th).WithWidth(leftWidth + rightWidth + 3),
	}
	return m.refreshChrome()
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, isKey := msg.(tea.KeyMsg)
	if isKey {
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k", "down", "j", "g", "G", "home", "end":
			prev := m.releaseList.Cursor()
			m.releaseList, _ = m.releaseList.Update(msg)
			if m.releaseList.Cursor() != prev {
				m = m.refreshBody()
			}
			m = m.refreshChrome()
			return m, nil
		case "left", "right", "tab", "shift+tab":
			m.rightTabs, _ = m.rightTabs.Update(msg)
			m = m.refreshChrome()
			return m, nil
		case "pgup", "pgdown", "u", "d":
			if m.rightTabs.Active() == 0 {
				m.bodyView, _ = m.bodyView.Update(msg)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m model) View() string {
	left := m.releaseList.View()
	right := m.rightPanel()
	body := joinHorizontal(left, right, rightHeight)

	return strings.Join([]string{
		m.statusBar.View(),
		body,
		m.hintsBar.View(),
	}, "\n")
}

func (m model) rightPanel() string {
	header := m.rightTabs.View()
	var inner string
	switch m.rightTabs.Active() {
	case 0:
		inner = m.bodyView.View()
	case 1:
		inner = m.assetsPanel()
	case 2:
		inner = m.metaPanel()
	}
	innerLines := strings.Split(inner, "\n")
	if len(innerLines) < rightHeight-1 {
		pad := make([]string, rightHeight-1-len(innerLines))
		innerLines = append(innerLines, pad...)
	} else if len(innerLines) > rightHeight-1 {
		innerLines = innerLines[:rightHeight-1]
	}
	return header + "\n" + strings.Join(innerLines, "\n")
}

func (m model) assetsPanel() string {
	r := m.currentRelease()
	if len(r.assets) == 0 {
		return "  (no assets recorded for this release)"
	}
	rows := make([]string, 0, len(r.assets)+1)
	rows = append(rows, fmt.Sprintf("  %-44s  %s", "ASSET", "SIZE"))
	for _, a := range r.assets {
		name := a.name
		if len(name) > 44 {
			name = name[:43] + "…"
		}
		rows = append(rows, fmt.Sprintf("  %-44s  %s", name, a.size))
	}
	return strings.Join(rows, "\n")
}

func (m model) metaPanel() string {
	r := m.currentRelease()
	kind := "stable"
	if r.prerelease {
		kind = "prerelease"
	}
	lines := []string{
		fmt.Sprintf("  tag           %s", r.tag),
		fmt.Sprintf("  name          %s", r.name),
		fmt.Sprintf("  published     %s", r.publishedAt),
		fmt.Sprintf("  kind          %s", kind),
		fmt.Sprintf("  assets        %d", len(r.assets)),
		fmt.Sprintf("  body lines    %d", strings.Count(r.body, "\n")+1),
	}
	return strings.Join(lines, "\n")
}

func (m model) currentRelease() release {
	idx := m.releaseList.Cursor()
	if idx < 0 || idx >= len(m.releases) {
		return release{}
	}
	return m.releases[idx]
}

func (m model) refreshBody() model {
	m.bodyView = m.bodyView.WithSource(m.currentRelease().body)
	return m
}

func (m model) refreshChrome() model {
	r := m.currentRelease()
	m.statusBar = m.statusBar.
		WithLeft(
			statusbar.Item{Text: "release-explorer", Style: statusbar.StylePrimary},
			statusbar.Item{Text: repoLabel, Style: statusbar.StyleMuted},
		).
		WithCenter(
			statusbar.Item{Text: fmt.Sprintf("%d / %d", m.releaseList.Cursor()+1, len(m.releases))},
		).
		WithRight(
			statusbar.Item{Text: r.tag, Style: statusbar.StylePrimary},
		)
	m.hintsBar = m.hintsBar.WithHints([]keyhints.Hint{
		{Key: "↑↓", Desc: "release"},
		{Key: "←→", Desc: "tab"},
		{Key: "pgup/pgdn", Desc: "scroll"},
		{Key: "g/G", Desc: "first/last"},
		{Key: "q", Desc: "quit"},
	})
	return m
}

func joinHorizontal(left, right string, h int) string {
	leftLines := normalize(left, h, leftWidth)
	rightLines := normalize(right, h, rightWidth)
	rows := make([]string, h)
	for i := 0; i < h; i++ {
		rows[i] = leftLines[i] + "  " + rightLines[i]
	}
	return strings.Join(rows, "\n")
}

func normalize(s string, rows, cols int) []string {
	lines := strings.Split(s, "\n")
	if len(lines) < rows {
		pad := make([]string, rows-len(lines))
		lines = append(lines, pad...)
	} else if len(lines) > rows {
		lines = lines[:rows]
	}
	for i, ln := range lines {
		if visibleLen(ln) < cols {
			lines[i] = ln + strings.Repeat(" ", cols-visibleLen(ln))
		}
	}
	return lines
}

func visibleLen(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}
