// Package filetree is nook's left-side file-tree pane.
//
// It wraps the glyph components/file-tree Model with the file-system
// walking and bordered surface nook needs. The walk uses the same
// ignore rules as the project picker (skip .git, node_modules, vendor,
// dist, target, and dotfiles) so the two surfaces stay consistent.
//
// The pane emits an OpenMsg when the user presses Enter on a file leaf;
// the host turns that into a buffer-open command. Directory expand /
// collapse stays inside the pane.
package filetree

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	glyphtree "github.com/truffle-dev/glyph/components/file-tree"
	"github.com/truffle-dev/glyph/components/theme"
)

// OpenMsg is emitted when the user opens a file leaf in the tree.
// Path is absolute (rooted at the pane's root).
type OpenMsg struct{ Path string }

// Pane is nook's file-tree side pane.
type Pane struct {
	model         glyphtree.Model
	root          string
	width         int
	height        int
	theme         theme.Theme
	focused       bool
	built         bool
	pendingReveal string
}

// New constructs a pane rooted at root. Construction is constant-time
// and does NOT walk the file system — that work happens asynchronously
// via BuildTreeCmd so first paint is never gated on the walk. View()
// renders a "Scanning…" placeholder until SetNode lands. This was the
// last sync FS call in nook's startup path; lightning-fast startup is
// a load-bearing property and must be preserved going forward.
func New(t theme.Theme, root string) Pane {
	return Pane{
		model: glyphtree.New(glyphtree.Node{Name: filepath.Base(root)}).WithTitle(""),
		root:  root,
		theme: t,
	}
}

// BuildTreeMsg carries the result of an async tree walk.
type BuildTreeMsg struct {
	Root string
	Node glyphtree.Node
}

// BuildTreeCmd walks root in a goroutine and returns a BuildTreeMsg the
// host model can hand to Pane.SetNode. Safe to fire multiple times — the
// handler ignores results whose Root no longer matches the pane's root.
func BuildTreeCmd(root string) tea.Cmd {
	return func() tea.Msg {
		return BuildTreeMsg{Root: root, Node: BuildTree(root)}
	}
}

// SetNode replaces the pane's tree with a freshly walked node and
// replays any reveal target that came in while the walk was pending.
// Caller should match msg.Root against the pane's root before calling
// to discard stale walks (e.g. after a project-root change).
func (p *Pane) SetNode(node glyphtree.Node) {
	cur := ""
	if p.built {
		cur = p.model.Selected()
	}
	p.model = glyphtree.New(node).WithTitle("")
	p.built = true
	if p.pendingReveal != "" {
		p.revealLocked(p.pendingReveal)
		p.pendingReveal = ""
	} else if cur != "" {
		p.model.SetCursor(cur)
	}
}

// Built reports whether SetNode has landed at least once. Callers can
// use this to decide whether to show the pane's view or skip allocation.
func (p Pane) Built() bool { return p.built }

// SetSize sets the width and height the pane may draw within.
func (p *Pane) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.model, _ = p.model.Update(tea.WindowSizeMsg{Width: w - 4, Height: h - 2})
}

// Focus gives the pane keyboard focus.
func (p *Pane) Focus() { p.focused = true }

// Blur removes keyboard focus.
func (p *Pane) Blur() { p.focused = false }

// Focused reports the pane's focus state.
func (p Pane) Focused() bool { return p.focused }

// SetTheme swaps the palette used for borders, the file/folder glyphs, and
// the cursor row. The wrapped glyph file-tree model uses its own internal
// palette tied to lipgloss tokens, so visual changes are limited to the
// nook-side framing; deeper recolor is a follow-up.
func (p *Pane) SetTheme(t theme.Theme) { p.theme = t }

// RefreshCmd returns a tea.Cmd that re-walks the file system off the
// UI thread and produces a BuildTreeMsg the host can hand back to
// SetNode. The previous synchronous Refresh blocked first paint on a
// home-directory launch (nook ~/.zshrc walks all of ~/repos and
// ~/Downloads); the async form preserves the snappy "Scanning…"
// placeholder behavior used at startup.
func (p *Pane) RefreshCmd() tea.Cmd {
	return BuildTreeCmd(p.root)
}

// Reveal expands the directories on the path to file and moves the
// cursor onto it. When the tree has not finished its initial walk yet,
// the reveal target is queued and replayed once SetNode lands. path
// must be slash-separated relative to root, or the function is a no-op.
func (p *Pane) Reveal(absPath string) {
	if !p.built {
		p.pendingReveal = absPath
		return
	}
	p.revealLocked(absPath)
}

// revealLocked performs the actual expand + cursor move. Callers must
// ensure the tree has been built before invoking it (SetNode + Reveal
// both check p.built before calling in).
func (p *Pane) revealLocked(absPath string) {
	rel, err := filepath.Rel(p.root, absPath)
	if err != nil {
		return
	}
	rel = filepath.ToSlash(rel)
	if rel == "" || rel == "." {
		return
	}
	parts := strings.Split(rel, "/")
	for i := 1; i < len(parts); i++ {
		p.model.Expand(strings.Join(parts[:i], "/"))
	}
	p.model.SetCursor(rel)
}

// Selected returns the absolute path of the row under the cursor, or
// the empty string if the tree is empty.
func (p Pane) Selected() string {
	s := p.model.Selected()
	if s == "" {
		return ""
	}
	return filepath.Join(p.root, filepath.FromSlash(s))
}

// Update routes a message through the underlying glyph tree model and
// rewrites its SelectMsg into a nook OpenMsg when the user opens a
// file leaf.
func (p Pane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok && !p.focused {
		return p, nil
	}
	var cmd tea.Cmd
	p.model, cmd = p.model.Update(msg)
	if cmd == nil {
		return p, nil
	}
	// Inspect what the child cmd returns; if it's a SelectMsg for a
	// file leaf, lift it into an OpenMsg the host knows how to handle.
	wrapped := func() tea.Msg {
		raw := cmd()
		sm, ok := raw.(glyphtree.SelectMsg)
		if !ok {
			return raw
		}
		if sm.IsDir {
			return nil
		}
		return OpenMsg{Path: filepath.Join(p.root, filepath.FromSlash(sm.Path))}
	}
	return p, wrapped
}

// View renders the pane inside a rounded border. When width or height
// is too small the function returns an empty string; the host should
// not allocate space for the pane in that case. Until the first
// BuildTreeMsg lands the pane renders a "Scanning…" placeholder so
// first paint is never gated on the file-system walk.
func (p Pane) View() string {
	if p.width < 12 || p.height < 4 {
		return ""
	}
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.theme.Border).
		Background(p.theme.Surface).
		Foreground(p.theme.Text).
		Padding(0, 1).
		Width(p.width - 2).
		Height(p.height - 2)
	header := lipgloss.NewStyle().
		Foreground(p.theme.TextMuted).
		Render(filepath.Base(p.root))
	var body string
	if !p.built {
		body = lipgloss.NewStyle().
			Foreground(p.theme.TextMuted).
			Render("Scanning…")
	} else {
		body = p.model.View()
	}
	return border.Render(header + "\n" + body)
}

// BuildTree walks root and returns a single glyphtree.Node representing
// the project. Directories are sorted alphabetically with directories
// before files within each level. Errors during walk are silently
// skipped — the pane should never crash because a directory is
// unreadable.
func BuildTree(root string) glyphtree.Node {
	node := glyphtree.Node{Name: filepath.Base(root), Children: []glyphtree.Node{}}
	entries, err := os.ReadDir(root)
	if err != nil {
		return node
	}
	dirs := []os.DirEntry{}
	files := []os.DirEntry{}
	for _, e := range entries {
		if skipEntry(e) {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	for _, d := range dirs {
		child := BuildTree(filepath.Join(root, d.Name()))
		child.Name = d.Name()
		node.Children = append(node.Children, child)
	}
	for _, f := range files {
		node.Children = append(node.Children, glyphtree.Node{Name: f.Name()})
	}
	return node
}

// skipEntry reports whether walkRepo / BuildTree should skip a
// directory entry. Mirrors the rules in main.walkRepo so the picker
// and the tree show the same set of files.
func skipEntry(e fs.DirEntry) bool {
	name := e.Name()
	if e.IsDir() {
		switch name {
		case ".git", "node_modules", "vendor", "dist", "target":
			return true
		}
		if strings.HasPrefix(name, ".") && name != "." {
			return true
		}
	}
	return false
}
