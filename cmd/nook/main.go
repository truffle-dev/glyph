// Command nook is a terminal-native AI IDE built from glyph components.
//
// Usage:
//
//	nook [project-root]
//
// If project-root is omitted, nook opens the current working directory.
//
// Keymap:
//
//	ctrl+p     file picker
//	ctrl+f     find in current buffer
//	ctrl+h     find and replace in current buffer
//	alt+f      project search
//	ctrl+b     file tree (left side)
//	ctrl+g     git pane
//	ctrl+k     inline AI edit on current line (Haiku)
//	ctrl+l     composer multi-file AI edit (Sonnet)
//	ctrl+`     terminal pane
//	ctrl+s     save current buffer
//	alt+i      LSP hover info for symbol under cursor
//	alt+j      expand snippet at cursor (Tab cycles tabstops, Esc exits)
//	alt+y      toggle gopls inlay hints (type annotations, parameter names)
//	alt+b      toggle inline git blame on cursor row (GitLens-style)
//	ctrl+]     LSP go to definition
//	ctrl+space LSP completion popup (↑/↓ to navigate, enter to accept)
//	alt+enter  LSP code actions at the cursor
//	f2         LSP rename symbol under cursor
//	alt+,      reload ~/.config/nook/config.toml
//	alt+p      workspace-wide diagnostics panel
//	alt+t      task picker (.nook/tasks.toml or Go defaults)
//	tab        accept ghost-text completion (when present)
//	ctrl+q     quit
//	esc        close overlay / blur / dismiss ghost-text
//
// LSP diagnostics: opening a .go file starts gopls under the project root
// and surfaces publishDiagnostics as colored ● markers in the gutter (red
// for errors, yellow for warnings, blue for info/hint). The status bar
// shows the per-file E/W count.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/truffle-dev/glyph/components/theme"

	"github.com/truffle-dev/glyph/cmd/nook/internal/ai"
	"github.com/truffle-dev/glyph/cmd/nook/internal/aihistory"
	"github.com/truffle-dev/glyph/cmd/nook/internal/bufman"
	"github.com/truffle-dev/glyph/cmd/nook/internal/codeaction"
	"github.com/truffle-dev/glyph/cmd/nook/internal/complete"
	"github.com/truffle-dev/glyph/cmd/nook/internal/composer"
	"github.com/truffle-dev/glyph/cmd/nook/internal/config"
	"github.com/truffle-dev/glyph/cmd/nook/internal/diagnostics"
	"github.com/truffle-dev/glyph/cmd/nook/internal/edit"
	"github.com/truffle-dev/glyph/cmd/nook/internal/editor"
	"github.com/truffle-dev/glyph/cmd/nook/internal/filetree"
	"github.com/truffle-dev/glyph/cmd/nook/internal/finder"
	"github.com/truffle-dev/glyph/cmd/nook/internal/findrefs"
	"github.com/truffle-dev/glyph/cmd/nook/internal/ghost"
	"github.com/truffle-dev/glyph/cmd/nook/internal/git"
	"github.com/truffle-dev/glyph/cmd/nook/internal/gitgutter"
	"github.com/truffle-dev/glyph/cmd/nook/internal/help"
	"github.com/truffle-dev/glyph/cmd/nook/internal/highlight"
	"github.com/truffle-dev/glyph/cmd/nook/internal/hover"
	"github.com/truffle-dev/glyph/cmd/nook/internal/inlineblame"
	"github.com/truffle-dev/glyph/cmd/nook/internal/lookup"
	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/cmd/nook/internal/mdpreview"
	"github.com/truffle-dev/glyph/cmd/nook/internal/multibuffer"
	"github.com/truffle-dev/glyph/cmd/nook/internal/picker"
	"github.com/truffle-dev/glyph/cmd/nook/internal/rename"
	"github.com/truffle-dev/glyph/cmd/nook/internal/search"
	"github.com/truffle-dev/glyph/cmd/nook/internal/snippets"
	"github.com/truffle-dev/glyph/cmd/nook/internal/tabbar"
	"github.com/truffle-dev/glyph/cmd/nook/internal/tasks"
	"github.com/truffle-dev/glyph/cmd/nook/internal/term"
	"github.com/truffle-dev/glyph/cmd/nook/internal/welcome"
)

// overlay is the modal layer currently above the workspace.
type overlay int

const (
	overlayNone overlay = iota
	overlayFilePicker
	overlayProjectSearch
	overlayInlineEdit
	overlayHelp
	overlayHover
	overlayCompletion
	overlayFinder
	overlayCodeAction
	overlayRename
	overlayMultibuffer
	overlayDiagnostics
	overlayTasks
)

// rightPane is which of git/term/diff/composer occupies the lower-right slot.
type rightPane int

const (
	rightNone rightPane = iota
	rightGit
	rightTerm
	rightDiff
	rightComposer
	rightPreview
)

type model struct {
	theme   theme.Theme
	root    string
	width   int
	height  int
	files   []string // walked once at startup; used as picker corpus
	overlay overlay

	bufs     *bufman.Manager
	gitPane  git.Pane
	termPane term.Pane
	picker   picker.Picker
	search   search.Pane
	editPane edit.Pane
	composer composer.Pane
	ghost    *ghost.Manager

	right     rightPane
	diffBody  string
	diffTitle string
	status    string

	aiClient  *ai.Client
	aiHistory *aihistory.Store // per-file composer transcript

	// LSP wiring. lsp is lazily started on the first .go file open. Diagnostics
	// are tracked per absolute path; lspVersions feeds incrementing didChange
	// versions per document.
	lsp         *nooklsp.Client
	lspStarting bool
	lspVersions map[string]int32
	diagnostics map[string][]protocol.Diagnostic

	// streaming search context
	searchCancel context.CancelFunc
	searchProg   *tea.Program // set in Init for streaming

	// LSP hover overlay: filled by lookup.HoverMsg, displayed under
	// overlayHover. The host stashes the symbol the user asked about so
	// late responses (cursor already moved on) can be discarded.
	hoverContents string
	hoverPath     string
	hoverRow      int
	hoverCol      int

	// LSP completion popup: filled by lookup.CompletionMsg, displayed
	// under overlayCompletion. completeReqPath/Row/Col echo the request
	// inputs so a late response after the cursor moved gets discarded.
	completePopup   complete.Popup
	completeReqPath string
	completeReqRow  int
	completeReqCol  int

	// In-file find/replace overlay (Ctrl+F / Ctrl+H). The host owns search
	// execution; the finder owns input state and renders the bar.
	finder finder.Finder

	// formatOnSave runs textDocument/formatting before each save when the
	// active buffer has a connected language server. Defaults to true;
	// alt+s saves without formatting as the per-save escape hatch.
	formatOnSave bool

	// treePane is the persistent left-side file tree, toggled by ctrl+B.
	// showTree drives visibility; the pane also tracks its own focus so
	// the host can route keys to it when the user is browsing files.
	treePane filetree.Pane
	showTree bool

	// LSP code-action popup: filled by lookup.CodeActionMsg, displayed
	// under overlayCodeAction. caReqPath/Row/Col echo the request inputs
	// so a late response after the cursor moved gets discarded.
	caPopup   codeaction.Popup
	caReqPath string
	caReqRow  int
	caReqCol  int

	// LSP rename modal: armed by lookup.PrepareRenameMsg, displayed under
	// overlayRename. pendingRename stashes the cursor that started the
	// flow so the Enter handler can fire lookup.RenameCmd against the
	// original position even if the user moved the cursor inside the
	// prompt (which they can't, but the pin is defensive).
	renamePrompt  rename.Prompt
	pendingRename pendingRename

	// Multibuffer overlay (Zed's signature: stitch hunks from multiple
	// files into one scrollable surface). Alt+m loads working-tree-vs-HEAD
	// diff fragments; Enter on any row jumps to that file+line.
	multibufPane multibuffer.Pane

	// Workspace-wide diagnostics panel (alt+p). Rebuilt from m.diagnostics
	// every time the overlay opens so it reflects the current LSP state;
	// Enter on a row jumps to the source site.
	diagPane diagnostics.Pane

	// Markdown preview pane (alt+v). Right-column sibling of git/term/
	// composer — opening one closes the others. Refreshed from the active
	// buffer on toggle and after every save; non-.md/.markdown buffers are
	// rejected with a status hint so the toggle never silently fails.
	mdPane mdpreview.Pane

	// inlayHintsOn drives whether gopls inlay hints are rendered. Alt+y
	// toggles. Default true. When false the host suppresses hint requests
	// and clears any stale hints from the active editor pane.
	inlayHintsOn bool

	// blameOn drives whether the cursor row's git-blame entry is rendered
	// as a dim italic strip after the line. Alt+b toggles. Default false
	// (opt-in). When toggled on the host fires a BlameCmd for the active
	// buffer and flips SetBlameVisible(true) on every open pane; the
	// blame map is retained when toggled off so re-enabling is instant.
	blameOn bool

	// cfgPath is the resolved ~/.config/nook/config.toml location. Stored
	// on the model so alt+, can re-read the same file without re-resolving
	// XDG_CONFIG_HOME each time. Empty when the host couldn't determine a
	// path (e.g. no home dir) — the reload key surfaces an error in that
	// case rather than crashing.
	cfgPath string
	// themeName is the name of the currently-applied theme. Stored so the
	// reload handler can detect a theme change and surface a "restart to
	// apply" hint, since deeply-themed sub-panes aren't live-reskinned.
	themeName string
	// tabWidth is the editor's hard-tab expansion. Sourced from config at
	// startup and reload; forwarded to bufman and lookup.FormattingCmd.
	tabWidth int

	// snipLib is the snippet library. Seeded with the builtin defaults at
	// startup and topped up from ~/.config/nook/snippets/<scope>.json when
	// the user has placed their own. Alt+J looks up the prefix-before-cursor
	// against the active buffer's language scope.
	snipLib snippets.Library

	// tasksPane is the alt+t overlay: a two-mode list/output picker over
	// `.nook/tasks.toml` (with auto-defaults for Go projects). activeRunner
	// is the currently-supervised child process; nil when no task is
	// running. The host keeps the runner pointer so Ctrl+C / Esc in
	// ModeOutput can kill the process and the pump goroutines can be
	// stopped cleanly when the overlay closes.
	tasksPane    tasks.Pane
	activeRunner *tasks.Runner
}

// pendingRename holds the cursor position the rename flow was launched from.
// Filled by openRenamePrompt and consumed when the user accepts the prompt.
type pendingRename struct {
	path string
	row  int
	col  int
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "nook:", err)
		os.Exit(1)
	}
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nook:", err)
		os.Exit(1)
	}
	if _, err := os.Stat(abs); err != nil {
		fmt.Fprintln(os.Stderr, "nook:", err)
		os.Exit(1)
	}

	m := newModel(abs)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "nook:", err)
		os.Exit(1)
	}
}

// resolveTheme returns the named theme from the registry, falling back to
// theme.Default when the name is unregistered. The bool tells the caller
// whether the fallback fired so the host can surface a status hint.
func resolveTheme(name string) (theme.Theme, bool) {
	t, ok := theme.ByName(name)
	if !ok {
		return theme.Default, false
	}
	return t, true
}

func newModel(root string) model {
	cfgPath, _ := config.Path()
	cfg := config.Default()
	loadErr := error(nil)
	loadMissing := false
	if cfgPath != "" {
		c, err := config.Load(cfgPath)
		switch {
		case err == nil:
			cfg = c
		case errors.Is(err, config.ErrNotFound):
			loadMissing = true
		default:
			loadErr = err
		}
	}

	t, themeOK := resolveTheme(cfg.Editor.Theme)
	aiClient, _ := ai.NewClient() // tolerated nil; AI panes surface their own error
	aiHistory := aihistory.NewStore()
	snipLib := snippets.LoadDefaults()
	if home, err := os.UserHomeDir(); err == nil {
		// Best-effort overlay; missing dir is not an error.
		_ = snipLib.LoadDir(filepath.Join(home, ".config", "nook", "snippets"))
	}
	// The welcome card carries the full keymap; the status bar just nudges
	// new users toward the help overlay. Once a file is open, callers
	// overwrite m.status with feedback for whatever they just did.
	status := "press ? for keymap"
	switch {
	case loadErr != nil:
		status = "config: " + loadErr.Error() + " (using defaults)"
	case !themeOK && !loadMissing:
		status = "theme " + cfg.Editor.Theme + " not found; using default"
	}

	return model{
		theme:        t,
		root:         root,
		width:        80,
		height:       24,
		bufs:         bufman.New(t).WithHighlighter(highlight.New()).WithTabWidth(cfg.Editor.TabWidth).WithLineNumbers(cfg.Editor.LineNumbers),
		gitPane:      git.NewPane(t, root),
		termPane:     term.NewPane(t, root),
		picker:       picker.New(t).WithTitle("Open file").WithPlaceholder("type to filter…"),
		search:       search.NewPane(t, root),
		editPane:     edit.NewPane(t, aiClient),
		composer:     composer.NewPane(t, aiClient).WithHistory(aiHistory),
		ghost:        ghost.NewManager(aiClient),
		right:        rightNone,
		status:       status,
		aiClient:     aiClient,
		aiHistory:    aiHistory,
		lspVersions:  map[string]int32{},
		diagnostics:  map[string][]protocol.Diagnostic{},
		finder:       finder.New(t),
		formatOnSave: cfg.Editor.FormatOnSave,
		treePane:     filetree.New(t, root),
		showTree:     false,
		caPopup:      codeaction.New(),
		renamePrompt: rename.New(),
		multibufPane: multibuffer.NewPane(t, root),
		diagPane:     diagnostics.NewPane(t, root),
		mdPane:       mdpreview.NewPane(t),
		inlayHintsOn: cfg.Editor.InlayHints,
		cfgPath:      cfgPath,
		themeName:    cfg.Editor.Theme,
		tabWidth:     cfg.Editor.TabWidth,
		snipLib:      snipLib,
		tasksPane:    tasks.NewPane(t, root),
	}
}

// reloadConfig re-reads m.cfgPath and applies the runtime-mutable knobs.
// Editor toggles (format-on-save, inlay hints, tab width, line numbers) take
// effect immediately. A theme change is detected and surfaced as a status
// hint asking the user to restart, since deeply-themed sub-panes aren't
// live-reskinned in v0.15.0. Returns the updated model.
func (m model) reloadConfig() model {
	if m.cfgPath == "" {
		m.status = "config: no path resolved"
		return m
	}
	cfg, err := config.Load(m.cfgPath)
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		m.status = "config: " + err.Error() + " (kept current settings)"
		return m
	}
	prevTheme := m.themeName
	m.formatOnSave = cfg.Editor.FormatOnSave
	m.inlayHintsOn = cfg.Editor.InlayHints
	m.tabWidth = cfg.Editor.TabWidth
	m.themeName = cfg.Editor.Theme
	m.bufs.WithTabWidth(cfg.Editor.TabWidth).WithLineNumbers(cfg.Editor.LineNumbers)
	if !m.inlayHintsOn {
		m = m.clearInlayHints()
	}
	switch {
	case errors.Is(err, config.ErrNotFound):
		m.status = "config: no file at " + m.cfgPath + " (using defaults)"
	case prevTheme != cfg.Editor.Theme:
		m.status = "settings reloaded — restart nook to apply theme change"
	default:
		m.status = "settings reloaded"
	}
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.loadFilesCmd(),
		m.refreshGitCmd(),
	)
}

type filesLoadedMsg struct{ files []string }
type errMsg struct{ err error }

func (m model) loadFilesCmd() tea.Cmd {
	root := m.root
	return func() tea.Msg {
		files := walkRepo(root)
		return filesLoadedMsg{files: files}
	}
}

func (m model) refreshGitCmd() tea.Cmd {
	root := m.root
	return func() tea.Msg {
		s, err := git.RunStatus(context.Background(), root)
		return git.StatusMsg{Status: s, Err: err}
	}
}

// refreshGutterCmd returns a tea.Cmd that recomputes per-line git markers for
// the active buffer's file. Empty when there is no active buffer or no path.
// Called whenever a buffer is opened or saved.
func (m model) refreshGutterCmd() tea.Cmd {
	p := m.bufs.Active()
	if p == nil {
		return nil
	}
	path := p.Path()
	if path == "" {
		return nil
	}
	return gitgutter.MarkerCmd(m.root, path)
}

// applyLineMarkers locates the pane whose path matches the msg and applies the
// marker map. Late responses for a path no longer open are dropped silently.
func (m model) applyLineMarkers(msg gitgutter.MarkersMsg) model {
	if msg.Err != nil || msg.Path == "" {
		return m
	}
	idx := m.bufs.Find(msg.Path)
	if idx < 0 {
		return m
	}
	p := m.bufs.At(idx)
	if p == nil {
		return m
	}
	*p = p.SetLineMarkers(msg.Markers)
	return m
}

// refreshInlayHintsCmd requests a fresh inlay-hint batch for the active
// buffer's path. Returns nil when hints are disabled, no LSP is wired up,
// no active buffer, or no path. The request covers the whole file (start
// line 0 to endLine == line count); gopls returns hints in document
// order which the host stashes keyed by row.
func (m model) refreshInlayHintsCmd() tea.Cmd {
	if !m.inlayHintsOn || m.lsp == nil {
		return nil
	}
	p := m.bufs.Active()
	if p == nil {
		return nil
	}
	path := p.Path()
	if path == "" {
		return nil
	}
	version := m.lspVersions[path]
	return lookup.InlayHintCmd(m.lsp, path, version, 0, p.LineCount()+1)
}

// applyInlayHints locates the pane whose path matches msg and applies the
// hint map. Late responses for a stale version (the buffer changed after
// the request was issued) or for a path no longer open are dropped.
func (m model) applyInlayHints(msg lookup.InlayHintMsg) model {
	if msg.Err != nil || msg.Path == "" {
		return m
	}
	idx := m.bufs.Find(msg.Path)
	if idx < 0 {
		return m
	}
	p := m.bufs.At(idx)
	if p == nil {
		return m
	}
	if msg.Version != 0 && m.lspVersions[msg.Path] != msg.Version {
		return m
	}
	*p = p.SetInlayHints(msg.Hints)
	return m
}

// clearInlayHints wipes inlay hints from every open buffer. Called when
// the user toggles hints off via alt+y.
func (m model) clearInlayHints() model {
	for i := 0; i < m.bufs.Count(); i++ {
		p := m.bufs.At(i)
		if p == nil {
			continue
		}
		*p = p.SetInlayHints(nil)
	}
	return m
}

// refreshBlameCmd requests a fresh git-blame map for the active buffer's
// path. Returns nil when the feature is off, no active buffer, or no path.
func (m model) refreshBlameCmd() tea.Cmd {
	if !m.blameOn {
		return nil
	}
	p := m.bufs.Active()
	if p == nil {
		return nil
	}
	path := p.Path()
	if path == "" {
		return nil
	}
	return inlineblame.BlameCmd(m.root, path)
}

// refreshBlameOnPathCmd is like refreshBlameCmd but pins to a specific path
// (used by editor.SavedMsg where the saved file may not be the active one).
func (m model) refreshBlameOnPathCmd(path string) tea.Cmd {
	if !m.blameOn || path == "" {
		return nil
	}
	return inlineblame.BlameCmd(m.root, path)
}

// applyBlame locates the pane matching msg.Path and stores the blame map.
// Late responses for a path no longer open are dropped silently.
func (m model) applyBlame(msg inlineblame.BlameMsg) model {
	if msg.Err != nil || msg.Path == "" {
		return m
	}
	idx := m.bufs.Find(msg.Path)
	if idx < 0 {
		return m
	}
	p := m.bufs.At(idx)
	if p == nil {
		return m
	}
	*p = p.SetBlame(msg.Lines).SetBlameVisible(m.blameOn)
	return m
}

// setBlameVisibility flips SetBlameVisible(b) on every open buffer. Called
// when the user toggles alt+b — keeps blame data in place so re-enable is
// instant.
func (m model) setBlameVisibility(b bool) model {
	for i := 0; i < m.bufs.Count(); i++ {
		p := m.bufs.At(i)
		if p == nil {
			continue
		}
		*p = p.SetBlameVisible(b)
	}
	return m
}

func walkRepo(root string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "target" || strings.HasPrefix(name, ".") && name != "." {
				return fs.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.resize()
		return m, nil

	case tea.KeyMsg:
		return m.routeKey(msg)

	case filesLoadedMsg:
		m.files = msg.files
		items := make([]picker.Item, len(msg.files))
		for i, f := range msg.files {
			items[i] = picker.Item{Title: f, Value: f}
		}
		m.picker = m.picker.WithItems(items)
		// Only overwrite the prompt status while the welcome card is
		// showing — once a file is open, "loaded N files" would clobber
		// useful per-action feedback.
		if m.activePath() == "" {
			m.status = "press ? for keymap"
		}
		return m, nil

	case picker.SelectMsg:
		path, _ := msg.Item.Value.(string)
		full := filepath.Join(m.root, path)
		_, action := m.bufs.OpenOrSwitch(full)
		m.overlay = overlayNone
		m = m.resize()
		switch action {
		case bufman.Switched:
			m.status = "switched to " + path
		default:
			m.status = "opened " + path
		}
		if m.showTree {
			m.treePane.Reveal(full)
		}
		m = m.applyDiagnosticsToActive()
		return m, tea.Batch(m.ensureLSPForFile(full), m.refreshGutterCmd(), m.refreshInlayHintsCmd(), m.refreshBlameCmd())

	case picker.CancelMsg:
		m.overlay = overlayNone
		return m, nil

	case editor.SavedMsg:
		if msg.Err != nil {
			m.status = "save failed: " + msg.Err.Error()
		} else {
			if p := m.bufs.Active(); p != nil {
				*p = p.ApplySave()
			}
			m.status = "saved " + msg.Path
			// Refresh the markdown preview when the saved file is the
			// one currently being previewed. Updating from disk would be
			// equivalent here, but reading from the buffer keeps the
			// refresh in-memory.
			if m.right == rightPreview && m.mdPane.Path() == msg.Path {
				if p := m.bufs.Active(); p != nil && p.Path() == msg.Path {
					m.mdPane = m.mdPane.WithSource(msg.Path, p.Contents())
				}
			}
		}
		return m, tea.Batch(m.refreshGitCmd(), gitgutter.MarkerCmd(m.root, msg.Path), m.refreshInlayHintsCmd(), m.refreshBlameOnPathCmd(msg.Path))

	case gitgutter.MarkersMsg:
		m = m.applyLineMarkers(msg)
		return m, nil

	case lookup.InlayHintMsg:
		m = m.applyInlayHints(msg)
		return m, nil

	case inlineblame.BlameMsg:
		m = m.applyBlame(msg)
		return m, nil

	case editor.CancelMsg:
		if p := m.bufs.Active(); p != nil {
			*p = p.Blur()
		}
		return m, nil

	case search.MatchMsg:
		m.search = m.search.AppendMatch(msg.Match)
		return m, nil

	case search.DoneMsg:
		m.search = m.search.MarkDone(msg.Err)
		m.status = fmt.Sprintf("search complete (%d matches)", msg.Matches)
		return m, nil

	case search.OpenMsg:
		m.bufs.OpenOrSwitch(msg.Path)
		if p := m.bufs.Active(); p != nil {
			*p = p.JumpTo(msg.Line, msg.Col).Focus()
		}
		m.search = m.search.Blur()
		m.overlay = overlayNone
		m = m.resize()
		m.status = fmt.Sprintf("opened %s:%d:%d", msg.Path, msg.Line, msg.Col)
		if m.showTree {
			m.treePane.Reveal(msg.Path)
		}
		m = m.applyDiagnosticsToActive()
		return m, tea.Batch(m.ensureLSPForFile(msg.Path), m.refreshGutterCmd(), m.refreshInlayHintsCmd(), m.refreshBlameCmd())

	case search.CancelMsg:
		m.overlay = overlayNone
		if m.searchCancel != nil {
			m.searchCancel()
			m.searchCancel = nil
		}
		return m, nil

	case git.StatusMsg:
		if msg.Err == nil {
			m.gitPane = m.gitPane.SetStatus(msg.Status)
		} else {
			m.status = "git: " + msg.Err.Error()
		}
		return m, nil

	case git.StagedMsg:
		if msg.Err != nil {
			m.status = "stage failed: " + msg.Err.Error()
		} else {
			m.status = "staged " + msg.Path
		}
		return m, m.refreshGitCmd()

	case git.UnstagedMsg:
		if msg.Err != nil {
			m.status = "unstage failed: " + msg.Err.Error()
		} else {
			m.status = "unstaged " + msg.Path
		}
		return m, m.refreshGitCmd()

	case git.DiffMsg:
		if msg.Err != nil {
			m.status = "diff failed: " + msg.Err.Error()
			return m, nil
		}
		m.diffBody = msg.Body
		m.diffTitle = msg.Path
		m.right = rightDiff
		m = m.resize()
		return m, nil

	case git.CommitMsg:
		if msg.Err != nil {
			m.status = "commit failed: " + msg.Err.Error()
		} else {
			short := msg.SHA
			if len(short) > 8 {
				short = short[:8]
			}
			m.status = "committed " + short
		}
		return m, m.refreshGitCmd()

	case git.CancelMsg:
		m.right = rightNone
		m.gitPane = m.gitPane.Blur()
		return m, nil

	case term.OutputMsg:
		m.termPane = m.termPane.Append(msg.Data)
		return m, nil

	case term.ExitMsg:
		m.termPane = m.termPane.MarkExit(msg.Err)
		return m, nil

	case term.CancelMsg:
		m.right = rightNone
		m.termPane = m.termPane.Blur()
		return m, nil

	case mdpreview.CancelMsg:
		m.right = rightNone
		m.mdPane = m.mdPane.Blur()
		m = m.resize()
		return m, nil

	case searchPumpMsg:
		m.search = m.search.AppendMatch(msg.match)
		return m, m.searchPumpCmd(msg.out, msg.done)

	case termPumpMsg:
		m.termPane = m.termPane.Append(msg.data)
		return m, m.termPumpCmd(msg.ch)

	case edit.AcceptMsg:
		if p := m.bufs.Active(); p != nil {
			*p = p.SetLine(msg.Line, msg.NewText).Focus()
		}
		m.overlay = overlayNone
		m.status = fmt.Sprintf("ai edit applied at line %d", msg.Line+1)
		return m, nil

	case edit.CancelMsg:
		m.overlay = overlayNone
		if p := m.bufs.Active(); p != nil {
			*p = p.Focus()
		}
		return m, nil

	case composer.ApplyMsg:
		return m.applyComposerEdit(msg.Edit)

	case composer.ApplyAllMsg:
		var cmds []tea.Cmd
		for _, e := range msg.Edits {
			_, cmd := m.applyComposerEdit(e)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		m.status = fmt.Sprintf("applied %d edits", len(msg.Edits))
		return m, tea.Batch(cmds...)

	case composer.CancelMsg:
		m.composer = m.composer.Blur()
		m.right = rightNone
		if p := m.bufs.Active(); p != nil {
			*p = p.Focus()
		}
		m = m.resize()
		return m, nil

	case ghost.SuggestMsg:
		cmd := m.ghost.Update(msg)
		if p := m.bufs.Active(); p != nil {
			*p = p.SetGhostText(m.ghost.Proposal())
		}
		return m, cmd

	case lspStartedMsg:
		m.lspStarting = false
		if msg.err != nil {
			m.status = "lsp: " + msg.err.Error()
			return m, nil
		}
		m.lsp = msg.client
		if m.status == "starting gopls…" {
			m.status = "gopls ready"
		}
		var cmds []tea.Cmd
		cmds = append(cmds, m.lspPumpCmd(m.lsp.Diagnostics()))
		if p := m.bufs.Active(); p != nil && isGoFile(p.Path()) {
			m.lspVersions[p.Path()] = 1
			cmds = append(cmds, m.lspOpenCmd(p.Path(), p.Contents()))
		}
		return m, tea.Batch(cmds...)

	case lspDiagnosticsMsg:
		path := pathFromURI(msg.URI)
		if path != "" {
			m.diagnostics[path] = msg.Items
		}
		if m.activePath() == path {
			m = m.applyDiagnosticsToActive()
		}
		var cmd tea.Cmd
		if m.lsp != nil {
			cmd = m.lspPumpCmd(m.lsp.Diagnostics())
		}
		return m, cmd

	case lspOpenedMsg:
		// fire-and-forget; surface a status if the open failed
		if msg.err != nil {
			m.status = "lsp open " + filepath.Base(msg.path) + ": " + msg.err.Error()
		}
		return m, nil

	case lookup.HoverMsg:
		// Discard late responses for a stale cursor position so the
		// overlay always reflects the current focus.
		if p := m.bufs.Active(); p != nil {
			if p.Path() != msg.Path || p.CursorRow() != msg.Row || p.CursorCol() != msg.Col {
				return m, nil
			}
		}
		if msg.Err != nil {
			m.status = "hover: " + msg.Err.Error()
			return m, nil
		}
		if strings.TrimSpace(msg.Info.Contents) == "" {
			m.status = "no hover info"
			return m, nil
		}
		m.hoverContents = msg.Info.Contents
		m.hoverPath = msg.Path
		m.hoverRow = msg.Row
		m.hoverCol = msg.Col
		m.overlay = overlayHover
		return m, nil

	case lookup.DefinitionMsg:
		if msg.Err != nil {
			m.status = "definition: " + msg.Err.Error()
			return m, nil
		}
		if len(msg.Locations) == 0 {
			m.status = "no definition found"
			return m, nil
		}
		// Take the first definition. gopls usually returns exactly
		// one; multi-location results are interface methods and we
		// can build a chooser later.
		loc := msg.Locations[0]
		m.bufs.OpenOrSwitch(loc.Path)
		if p := m.bufs.Active(); p != nil {
			*p = p.JumpTo(loc.Line, loc.Col).Focus()
		}
		m = m.resize()
		if m.showTree {
			m.treePane.Reveal(loc.Path)
		}
		m = m.applyDiagnosticsToActive()
		m.status = fmt.Sprintf("jumped to %s:%d:%d", filepath.Base(loc.Path), loc.Line+1, loc.Col+1)
		return m, tea.Batch(m.ensureLSPForFile(loc.Path), m.refreshGutterCmd(), m.refreshInlayHintsCmd(), m.refreshBlameCmd())

	case lookup.CompletionMsg:
		// Discard late responses for a stale request: the user moved
		// the cursor or switched buffers before gopls answered.
		if msg.Path != m.completeReqPath || msg.Row != m.completeReqRow || msg.Col != m.completeReqCol {
			return m, nil
		}
		if msg.Err != nil {
			m.status = "completion: " + msg.Err.Error()
			return m, nil
		}
		if len(msg.Items) == 0 {
			m.status = "no completions"
			return m, nil
		}
		m.completePopup = m.completePopup.WithItems(msg.Items, msg.PrefixLen)
		m.overlay = overlayCompletion
		return m, nil

	case lookup.FormattingMsg:
		return m.handleFormattingMsg(msg)

	case lookup.CodeActionMsg:
		return m.handleCodeActionMsg(msg)

	case lookup.PrepareRenameMsg:
		return m.handlePrepareRenameMsg(msg)

	case lookup.RenameMsg:
		return m.handleRenameMsg(msg)

	case filetree.OpenMsg:
		_, action := m.bufs.OpenOrSwitch(msg.Path)
		m.treePane.Blur()
		rel, _ := filepath.Rel(m.root, msg.Path)
		switch action {
		case bufman.Switched:
			m.status = "switched to " + rel
		default:
			m.status = "opened " + rel
		}
		m = m.resize()
		m = m.applyDiagnosticsToActive()
		return m, tea.Batch(m.ensureLSPForFile(msg.Path), m.refreshGutterCmd(), m.refreshInlayHintsCmd(), m.refreshBlameCmd())

	case multibuffer.FragmentsMsg:
		m.multibufPane = m.multibufPane.SetFragments(msg.Fragments, msg.Err)
		return m, nil

	case multibuffer.OpenAtMsg:
		_, action := m.bufs.OpenOrSwitch(msg.Path)
		if p := m.bufs.Active(); p != nil {
			// editor.JumpTo takes 1-based line numbers; OpenAtMsg.Line is
			// already 1-based (mirrors search.OpenMsg's contract).
			*p = p.JumpTo(msg.Line, 1).Focus()
		}
		m.multibufPane = m.multibufPane.Blur()
		m.overlay = overlayNone
		m = m.resize()
		m = m.applyDiagnosticsToActive()
		rel, _ := filepath.Rel(m.root, msg.Path)
		switch action {
		case bufman.Switched:
			m.status = fmt.Sprintf("switched to %s:%d", rel, msg.Line)
		default:
			m.status = fmt.Sprintf("opened %s:%d", rel, msg.Line)
		}
		return m, tea.Batch(m.ensureLSPForFile(msg.Path), m.refreshGutterCmd(), m.refreshInlayHintsCmd(), m.refreshBlameCmd())

	case multibuffer.CancelMsg:
		m.multibufPane = m.multibufPane.Blur()
		m.overlay = overlayNone
		if p := m.bufs.Active(); p != nil {
			*p = p.Focus()
		}
		return m, nil

	case diagnostics.OpenAtMsg:
		_, action := m.bufs.OpenOrSwitch(msg.Path)
		if p := m.bufs.Active(); p != nil {
			// editor.JumpTo takes 1-based row/col; OpenAtMsg carries
			// 0-based values straight from the LSP diagnostic.
			*p = p.JumpTo(msg.Row+1, msg.Col+1).Focus()
		}
		m.diagPane = m.diagPane.Blur()
		m.overlay = overlayNone
		m = m.resize()
		m = m.applyDiagnosticsToActive()
		rel, _ := filepath.Rel(m.root, msg.Path)
		switch action {
		case bufman.Switched:
			m.status = fmt.Sprintf("switched to %s:%d:%d", rel, msg.Row+1, msg.Col+1)
		default:
			m.status = fmt.Sprintf("opened %s:%d:%d", rel, msg.Row+1, msg.Col+1)
		}
		return m, tea.Batch(m.ensureLSPForFile(msg.Path), m.refreshGutterCmd(), m.refreshInlayHintsCmd(), m.refreshBlameCmd())

	case diagnostics.CancelMsg:
		m.diagPane = m.diagPane.Blur()
		m.overlay = overlayNone
		if p := m.bufs.Active(); p != nil {
			*p = p.Focus()
		}
		return m, nil

	case tasks.RunTaskMsg:
		return m.spawnTask(msg.Task)

	case tasks.StartedMsg:
		var cmd tea.Cmd
		m.tasksPane, cmd = m.tasksPane.Update(msg)
		return m, cmd

	case tasks.LineMsg:
		var paneCmd tea.Cmd
		m.tasksPane, paneCmd = m.tasksPane.Update(msg)
		// Chain the next read so the stream keeps flowing until the
		// channel is closed (signaled by NextLineCmd returning nil).
		var nextCmd tea.Cmd
		if m.activeRunner != nil && msg.RunID == m.activeRunner.ID() {
			nextCmd = m.activeRunner.NextLineCmd()
		}
		return m, tea.Batch(paneCmd, nextCmd)

	case tasks.ExitMsg:
		var cmd tea.Cmd
		m.tasksPane, cmd = m.tasksPane.Update(msg)
		// Keep activeRunner non-nil so the pane can show the exit summary,
		// but mark it as exited; Esc-out (CancelMsg or BackToListMsg) is
		// what frees the slot.
		return m, cmd

	case tasks.KillMsg:
		// Ctrl+C in ModeOutput: kill the running process but keep the
		// overlay open so the exit summary is visible. The pane stays in
		// ModeOutput; Esc-after-exit returns to ModeList.
		if m.activeRunner != nil {
			m.activeRunner.Kill()
		}
		return m, nil

	case tasks.BackToListMsg:
		// Esc on an exited output view: clear the runner slot and flip the
		// pane back to its list state. The list keeps its cursor so the
		// user can re-run the same task with Enter.
		m.activeRunner = nil
		m.tasksPane = m.tasksPane.BackToList()
		return m, nil

	case tasks.CancelMsg:
		// Esc: close the overlay completely. If a task is running, kill it
		// first so the child process doesn't outlive the UI.
		if m.activeRunner != nil {
			m.activeRunner.Kill()
			m.activeRunner = nil
		}
		m.tasksPane = m.tasksPane.Blur()
		m.overlay = overlayNone
		if p := m.bufs.Active(); p != nil {
			*p = p.Focus()
		}
		return m, nil

	case errMsg:
		m.status = "error: " + msg.err.Error()
		return m, nil
	}
	// Ghost may also accept debounceMsg from its own Tick. We forward unknown
	// messages to the manager so it can handle internal lifecycle.
	if cmd := m.ghost.Update(msg); cmd != nil {
		return m, cmd
	}
	return m, nil
}

// applyComposerEdit writes one composer edit to disk and refreshes the editor
// if it's currently showing that file.
func (m model) applyComposerEdit(e composer.Edit) (tea.Model, tea.Cmd) {
	abs := filepath.Join(m.root, e.Path)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		m.status = "mkdir failed: " + err.Error()
		return m, nil
	}
	body := e.Proposed
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		m.status = "write failed: " + err.Error()
		return m, nil
	}
	m.bufs.RefreshIfOpen(abs)
	m.status = "wrote " + e.Path
	return m, nil
}

func (m model) routeKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help overlay swallows its own keys: ? or esc to dismiss, everything
	// else ignored so an accidental ctrl+f doesn't open search underneath
	// the help card.
	if m.overlay == overlayHelp {
		if km.Type == tea.KeyEsc {
			m.overlay = overlayNone
			return m, nil
		}
		if km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == '?' {
			m.overlay = overlayNone
			return m, nil
		}
		return m, nil
	}

	// Hover overlay also swallows its own keys: any keypress dismisses
	// (matches the modeless tooltip pattern in mainstream editors —
	// click/move/type away and the popup goes). Esc is reserved as the
	// explicit dismiss too so muscle memory works either way.
	if m.overlay == overlayHover {
		m.overlay = overlayNone
		m.hoverContents = ""
		if km.Type == tea.KeyEsc {
			return m, nil
		}
		// Fall through so the user's keypress reaches the editor (e.g.
		// pressing j to scroll down after closing hover doesn't get
		// eaten).
	}

	// Completion popup intercepts navigation keys: ↑/↓ move selection,
	// Enter accepts (replacing the trailing word-prefix with the chosen
	// item's InsertText), Esc dismisses silently. Any other key dismisses
	// AND falls through to the editor so the user can keep typing — this
	// matches VS Code's behavior where pressing 'a' after the popup is
	// open closes the menu and types 'a'.
	if m.overlay == overlayCompletion {
		switch km.Type {
		case tea.KeyUp:
			m.completePopup = m.completePopup.MoveUp()
			return m, nil
		case tea.KeyDown:
			m.completePopup = m.completePopup.MoveDown()
			return m, nil
		case tea.KeyEnter:
			return m.acceptCompletion()
		case tea.KeyEsc:
			m.dismissCompletion()
			return m, nil
		}
		// Any other key: dismiss the popup and fall through.
		m.dismissCompletion()
	}

	// Code-action popup swallows its own keys end-to-end: ↑/↓ navigate,
	// Enter applies the highlighted action, Esc cancels. Unlike the
	// completion popup, accidental typing should NOT fall through to the
	// editor — applying a refactor is a deliberate gesture, not a
	// keystroke-eat.
	if m.overlay == overlayCodeAction {
		switch km.Type {
		case tea.KeyUp:
			m.caPopup = m.caPopup.MoveUp()
			return m, nil
		case tea.KeyDown:
			m.caPopup = m.caPopup.MoveDown()
			return m, nil
		case tea.KeyEnter:
			return m.acceptCodeAction()
		case tea.KeyEsc:
			m.dismissCodeAction()
			return m, nil
		}
		return m, nil
	}

	// Rename prompt swallows its own keys: typing edits the input,
	// arrows/home/end move the cursor, Backspace deletes, Enter fires
	// the rename request, Esc cancels.
	if m.overlay == overlayRename {
		return m.routeRename(km)
	}

	// Global keys
	switch km.Type {
	case tea.KeyCtrlQ:
		return m, tea.Quit
	case tea.KeyCtrlP:
		m.overlay = overlayFilePicker
		m.picker = m.picker.WithFilter("")
		return m, nil
	case tea.KeyCtrlF:
		// Local find inside the active buffer. If no file is open, fall back
		// to project search so the key still does something useful.
		if m.bufs.Active() != nil {
			return m.openFinder(finder.ModeFind)
		}
		m.overlay = overlayProjectSearch
		m.search = m.search.Reset("")
		m.search = m.search.Focus()
		return m, nil
	case tea.KeyCtrlH:
		// In-buffer find and replace. Same fallback as ctrl+f.
		if m.bufs.Active() != nil {
			return m.openFinder(finder.ModeReplace)
		}
		return m, nil
	case tea.KeyCtrlG:
		if m.right == rightGit {
			m.right = rightNone
			m.gitPane = m.gitPane.Blur()
		} else {
			m.right = rightGit
			m.gitPane = m.gitPane.Focus()
		}
		m = m.resize()
		return m, m.refreshGitCmd()
	case tea.KeyCtrlK:
		return m.openInlineEdit()
	case tea.KeyCtrlL:
		return m.toggleComposer()
	case tea.KeyCtrlW:
		return m.closeActiveTab()
	case tea.KeyCtrlB:
		return m.toggleTree()
	case tea.KeyCtrlS:
		return m.saveActive(true)
	case tea.KeyCtrlCloseBracket:
		// Go-to-definition. Asks gopls where the symbol under the
		// cursor was declared and jumps to it (opening the target file
		// in a new buffer when it's outside the current one).
		if p := m.bufs.Active(); p != nil && p.Path() != "" {
			return m, lookup.DefinitionCmd(m.lsp, p.Path(), p.CursorRow(), p.CursorCol())
		}
	case tea.KeyCtrlAt:
		// Ctrl+Space is delivered as NUL (KeyCtrlAt) under standard
		// terminals because ASCII space has no separate control code.
		// Treat it as the completion trigger.
		return m.triggerCompletion()
	case tea.KeyF2:
		// LSP rename. F2 because Ctrl+R is already taken by
		// "replace current match" in the finder. We ask gopls to
		// prepareRename first so the prompt opens with the actual
		// identifier under the cursor, not whatever character the
		// user happened to be over.
		return m.triggerRename()
	}
	// Alt+Enter triggers code actions at the cursor. Ctrl+. is the VS
	// Code default but has no portable ASCII control code (only
	// kitty-protocol terminals send anything for it); alt+enter is the
	// IntelliJ default and works in every terminal bubbletea understands.
	if km.Alt && km.Type == tea.KeyEnter {
		return m.triggerCodeActions()
	}
	// Alt+] / Alt+[ cycle through open buffers. bubbletea surfaces Alt as
	// km.Alt with the modified key in km.Runes when KeyRunes; for "]" with
	// alt the type is KeyRunes and km.Alt is true.
	if km.Alt && km.Type == tea.KeyRunes && len(km.Runes) == 1 {
		switch km.Runes[0] {
		case ']':
			m.bufs.Next()
			m = m.resize()
			m = m.applyDiagnosticsToActive()
			if p := m.bufs.Active(); p != nil {
				if m.showTree {
					m.treePane.Reveal(p.Path())
				}
				m.status = "switched to " + filepath.Base(p.Path())
			}
			return m, nil
		case '[':
			m.bufs.Prev()
			m = m.resize()
			m = m.applyDiagnosticsToActive()
			if p := m.bufs.Active(); p != nil {
				if m.showTree {
					m.treePane.Reveal(p.Path())
				}
				m.status = "switched to " + filepath.Base(p.Path())
			}
			return m, nil
		case 'i':
			// LSP hover info on the cursor symbol. Alt+i instead of
			// Ctrl+. because Ctrl+. has no portable ASCII control code
			// — only Kitty-protocol terminals send anything for it.
			if p := m.bufs.Active(); p != nil && p.Path() != "" {
				return m, lookup.HoverCmd(m.lsp, p.Path(), p.CursorRow(), p.CursorCol())
			}
			return m, nil
		case 'f':
			// Alt+f opens project-wide search (ripgrep). Ctrl+F was reclaimed
			// by the in-buffer finder; project search keeps the same model
			// but moves to a less-conflicting modifier.
			m.overlay = overlayProjectSearch
			m.search = m.search.Reset("")
			m.search = m.search.Focus()
			return m, nil
		case 's':
			// Alt+s saves without formatting — the per-save escape hatch for
			// when gopls is grumpy or the user wants to commit a deliberately
			// "ugly" intermediate state without losing it.
			return m.saveActive(false)
		case 'S':
			// Alt+Shift+s toggles the global format-on-save preference.
			// Useful when working with a half-broken file where format
			// would just delete what you're writing.
			m.formatOnSave = !m.formatOnSave
			if m.formatOnSave {
				m.status = "format-on-save: on"
			} else {
				m.status = "format-on-save: off"
			}
			return m, nil
		case 'm':
			// Alt+m opens the multibuffer overlay populated from
			// `git diff HEAD` — every changed hunk in the working tree
			// rendered as one scrollable surface. Enter on a row jumps
			// to that file at that line.
			m.overlay = overlayMultibuffer
			m.multibufPane = m.multibufPane.WithSize(m.width-4, m.height-4).Reset("uncommitted changes").Focus()
			return m, multibuffer.LoadDiffCmd(m.root, "HEAD")
		case 'p':
			// Alt+p opens the workspace-wide diagnostics panel. The list
			// is rebuilt from m.diagnostics on each open so it reflects
			// the current LSP state; ctrl+shift+m (Cursor / VS Code's
			// keybinding) collides with Enter in most terminals, so
			// alt+p (mnemonic: "problems") is the portable surface.
			m.overlay = overlayDiagnostics
			m.diagPane = m.diagPane.WithSize(m.width-4, m.height-4).WithEntries(m.collectDiagnosticEntries()).Focus()
			return m, nil
		case 'v':
			// Alt+v toggles the markdown preview pane. Only opens on .md /
			// .markdown buffers — non-markdown buffers get a status hint so
			// the toggle never silently swallows the keystroke. Ctrl+Shift+V
			// is reserved by most terminal emulators for paste, so alt+v is
			// the portable surface.
			return m.toggleMdPreview()
		case 'y':
			// Alt+y toggles gopls inlay hints (type annotations + parameter
			// names) for every open buffer. Hints are visual-only; the
			// underlying buffer never changes. Default is on.
			m.inlayHintsOn = !m.inlayHintsOn
			if m.inlayHintsOn {
				m.status = "inlay hints: on"
				return m, m.refreshInlayHintsCmd()
			}
			m = m.clearInlayHints()
			m.status = "inlay hints: off"
			return m, nil
		case 'b':
			// Alt+b toggles GitLens-style inline blame on the cursor row of
			// the active editor pane. Blame data is fetched on file open
			// and save regardless of visibility, so toggling never re-shells
			// git for already-open buffers. Default is off.
			m.blameOn = !m.blameOn
			m = m.setBlameVisibility(m.blameOn)
			if m.blameOn {
				m.status = "inline blame: on"
				return m, m.refreshBlameCmd()
			}
			m.status = "inline blame: off"
			return m, nil
		case ',':
			// Alt+, reloads ~/.config/nook/config.toml. Editor toggles take
			// effect immediately; theme changes need a restart since deeply-
			// themed sub-panes aren't live-reskinned in v0.15.0.
			m = m.reloadConfig()
			if m.inlayHintsOn {
				return m, m.refreshInlayHintsCmd()
			}
			return m, nil
		case 'j':
			// Alt+j expands a VSCode-format snippet whose prefix the user has
			// just typed. Tab (collides with ghost-text accept), Ctrl+J (most
			// terminals collapse to Enter), and Ctrl+Shift+V (terminal paste)
			// are all unsuitable; alt+j is the portable surface. Mnemonic: "jot".
			return m.expandSnippetAtCursor()
		case 't':
			// Alt+t opens the tasks overlay (analog of VSCode's tasks.json
			// runner). Ctrl+Shift+B is the muscle-memory shortcut but every
			// terminal collapses Ctrl+Shift+B to Ctrl+B (file tree), so alt+t
			// is the portable surface. Mnemonic: "tasks".
			return m.openTasksOverlay()
		case 'u':
			// Alt+u asks the language server for every workspace site that
			// mentions the symbol under the cursor, then renders the hits
			// through the existing multibuffer overlay so Enter jumps and
			// Esc closes. Cursor / VS Code default to Shift+F12 but F12 is
			// not universally-reliable in terminals, so alt+u is the
			// portable surface. Mnemonic: "usages".
			return m.findReferencesAtCursor()
		}
	}
	// Ctrl+` toggles terminal — bubbletea expresses this as KeyRunes ` with ctrl.
	if km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == '`' {
		return m.toggleTerm()
	}
	// ? opens the help overlay. We only honor it when no file is open or
	// the editor isn't actively absorbing keystrokes — otherwise the user's
	// typing a question mark into their source file and that wins.
	if km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == '?' && m.activePath() == "" {
		m.overlay = overlayHelp
		return m, nil
	}

	// Overlay routing
	switch m.overlay {
	case overlayFilePicker:
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(km)
		return m, cmd
	case overlayProjectSearch:
		return m.routeProjectSearch(km)
	case overlayInlineEdit:
		var cmd tea.Cmd
		m.editPane, cmd = m.editPane.Update(km)
		return m, cmd
	case overlayFinder:
		return m.routeFinder(km)
	case overlayMultibuffer:
		var cmd tea.Cmd
		m.multibufPane, cmd = m.multibufPane.Update(km)
		return m, cmd
	case overlayDiagnostics:
		var cmd tea.Cmd
		m.diagPane, cmd = m.diagPane.Update(km)
		return m, cmd
	case overlayTasks:
		var cmd tea.Cmd
		m.tasksPane, cmd = m.tasksPane.Update(km)
		return m, cmd
	}

	// No overlay: composer takes keys when focused
	if m.right == rightComposer && m.composer.Focused() {
		var cmd tea.Cmd
		m.composer, cmd = m.composer.Update(km)
		return m, cmd
	}

	// File-tree gets keys when it's both visible and focused. Esc blurs
	// the tree but keeps it visible — matches the Cursor/VS Code muscle
	// memory where esc on the explorer focuses the editor without closing
	// the side panel.
	if m.showTree && m.treePane.Focused() {
		if km.Type == tea.KeyEsc {
			m.treePane.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.treePane, cmd = m.treePane.Update(km)
		return m, cmd
	}

	// Focused pane
	if m.gitPane.Focused() {
		var cmd tea.Cmd
		m.gitPane, cmd = m.gitPane.Update(km)
		return m, cmd
	}
	if m.termPane.Focused() {
		var cmd tea.Cmd
		m.termPane, cmd = m.termPane.Update(km)
		return m, cmd
	}
	if m.mdPane.Focused() {
		var cmd tea.Cmd
		m.mdPane, cmd = m.mdPane.Update(km)
		return m, cmd
	}
	// Default to editor — only routes if there's an active buffer.
	p := m.bufs.Active()
	if p == nil {
		return m, nil
	}
	if !p.Focused() {
		*p = p.Focus()
	}

	// Ghost-text key handling: Tab accepts a pending proposal; Esc dismisses
	// it (and doesn't propagate). Any other key invalidates the current
	// proposal but otherwise falls through.
	if p.Path() != "" && m.ghost.Enabled() {
		if km.Type == tea.KeyTab && p.GhostText() != "" {
			text := m.ghost.Accept()
			*p = p.InsertText(text).SetGhostText("")
			m.status = "ghost accepted"
			return m, nil
		}
		if km.Type == tea.KeyEsc && p.GhostText() != "" {
			m.ghost.Dismiss()
			*p = p.SetGhostText("")
			return m, nil
		}
		// Any other key clears the pending proposal — it's stale now.
		if p.GhostText() != "" {
			m.ghost.Dismiss()
			*p = p.SetGhostText("")
		}
	}

	var cmd tea.Cmd
	*p, cmd = p.Update(km)

	cmds := []tea.Cmd{}
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	// If the keypress mutated the buffer and gopls is running for a Go file,
	// publish a didChange so diagnostics stay live as the user types.
	if m.lsp != nil && isGoFile(p.Path()) && isMutatingKey(km.Type) {
		v := m.lspVersions[p.Path()] + 1
		m.lspVersions[p.Path()] = v
		cmds = append(cmds, m.lspChangeCmd(p.Path(), v, p.Contents()))
	}
	// After the editor state changes, ask ghost whether to schedule a request.
	if tcmd := m.scheduleGhost(); tcmd != nil {
		cmds = append(cmds, tcmd)
	}
	if len(cmds) == 0 {
		return m, nil
	}
	if len(cmds) == 1 {
		return m, cmds[0]
	}
	return m, tea.Batch(cmds...)
}

// isMutatingKey reports whether the key event would change the buffer when
// routed to the editor pane. Used by the LSP-change trigger so non-edit keys
// (arrows, page-up, etc.) don't churn didChange notifications.
func isMutatingKey(t tea.KeyType) bool {
	switch t {
	case tea.KeyRunes, tea.KeyEnter, tea.KeyTab, tea.KeySpace, tea.KeyBackspace, tea.KeyDelete:
		return true
	}
	return false
}

// scheduleGhost asks the ghost manager whether to schedule a new debounced
// completion request based on the editor's current cursor position. Returns
// the debounce cmd (or nil). m.ghost is a pointer so its state mutates
// regardless of value-vs-pointer receiver semantics here.
func (m model) scheduleGhost() tea.Cmd {
	if m.ghost == nil || !m.ghost.Enabled() {
		return nil
	}
	p := m.bufs.Active()
	if p == nil || p.Path() == "" {
		return nil
	}
	site := ghost.Site{
		Path:   p.Path(),
		Row:    p.CursorRow(),
		Col:    p.CursorCol(),
		Prefix: p.LinePrefix(),
	}
	// Suppress while an overlay is up.
	suppress := m.overlay != overlayNone || (m.right == rightComposer && m.composer.Focused())
	return m.ghost.Tick(site, false, suppress)
}

func (m model) openInlineEdit() (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil || p.Path() == "" {
		m.status = "open a file first (ctrl+p)"
		return m, nil
	}
	row := p.CursorRow()
	original := p.Line(row)
	m.editPane = m.editPane.Open(p.Path(), row, original)
	m.editPane = m.editPane.WithSize(min(70, m.width-4), 10)
	m.overlay = overlayInlineEdit
	*p = p.Blur()
	return m, nil
}

// saveActive saves the active buffer. When formatFirst is true (Ctrl+S),
// the buffer has a connected language server, and m.formatOnSave is on,
// the save runs textDocument/formatting first and applies the resulting
// edits before the disk write. When formatFirst is false (alt+s), the
// formatter is bypassed entirely — the buffer hits disk as-is.
func (m model) saveActive(formatFirst bool) (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil || p.Path() == "" {
		m.status = "no file to save"
		return m, nil
	}
	if !formatFirst || !m.formatOnSave || m.lsp == nil || !isGoFile(p.Path()) {
		// Plain save path: nothing to format against, or the user asked
		// for the escape hatch. Fire SaveCmd directly so the buffer
		// content hits disk exactly as displayed.
		m.status = "saving " + filepath.Base(p.Path())
		return m, p.SaveCmd()
	}
	// Format-then-save path. Pin the LSP version we're requesting against
	// so a stale response (the user typed more before gopls answered)
	// can be discarded in the message handler.
	version := m.lspVersions[p.Path()]
	tabW := m.tabWidth
	if tabW <= 0 {
		tabW = 4
	}
	m.status = "formatting " + filepath.Base(p.Path()) + "…"
	return m, lookup.FormattingCmd(m.lsp, p.Path(), version, tabW, false)
}

// handleFormattingMsg applies a textDocument/formatting response to the
// active buffer (when versions still match), publishes a didChange so
// gopls knows the new content, and fires SaveCmd. Errors and version
// drift both fall back to a plain save so the user's intent to save is
// always honored — formatting is an optimization, not a gate.
func (m model) handleFormattingMsg(msg lookup.FormattingMsg) (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil || p.Path() != msg.Path {
		// User switched buffers while gopls was thinking. Drop the
		// response on the floor.
		return m, nil
	}
	if msg.Err != nil {
		// errNoClient is the "feature unavailable" case — degrade silently
		// to a plain save. Any other error gets surfaced but still saves
		// so the user doesn't lose their changes to a formatter hiccup.
		if msg.Err.Error() != "no language server" {
			m.status = "format failed: " + msg.Err.Error()
		} else {
			m.status = "saving " + filepath.Base(msg.Path)
		}
		return m, p.SaveCmd()
	}
	if m.lspVersions[msg.Path] != msg.Version {
		// The buffer moved on while we were formatting. Don't apply the
		// stale edits — they'd corrupt what the user just typed. Save
		// the current buffer as-is instead.
		m.status = "save (buffer changed during format)"
		return m, p.SaveCmd()
	}
	if len(msg.Edits) == 0 {
		// Server thinks the file is already well-formatted. Skip the
		// rewrite and go straight to disk.
		m.status = "saving " + filepath.Base(msg.Path)
		return m, p.SaveCmd()
	}
	before := p.Contents()
	after := nooklsp.Apply(before, msg.Edits)
	if after == before {
		// Edits collapsed to a no-op (e.g. all idempotent). Skip the
		// rewrite path to avoid bumping bufVer for nothing.
		m.status = "saving " + filepath.Base(msg.Path)
		return m, p.SaveCmd()
	}
	*p = p.ReplaceAllFromString(after)
	cmds := []tea.Cmd{p.SaveCmd()}
	// Publish a fresh didChange so gopls's view of the document matches
	// what we just wrote. Bump the version monotonically the same way
	// every other mutating path does.
	nv := m.lspVersions[msg.Path] + 1
	m.lspVersions[msg.Path] = nv
	if c := m.lspChangeCmd(msg.Path, nv, after); c != nil {
		cmds = append(cmds, c)
	}
	m.status = "formatted " + filepath.Base(msg.Path)
	return m, tea.Batch(cmds...)
}

// triggerCompletion fires an LSP completion request at the cursor. The
// popup arms on the response message — there's nothing to show until the
// server answers. We stash the request inputs so a late response after
// the user moves on gets discarded in the CompletionMsg handler.
func (m model) triggerCompletion() (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil || p.Path() == "" {
		m.status = "open a file first (ctrl+p)"
		return m, nil
	}
	row := p.CursorRow()
	col := p.CursorCol()
	prefix := complete.WordPrefix(p.LinePrefix())
	m.completeReqPath = p.Path()
	m.completeReqRow = row
	m.completeReqCol = col
	m.status = "asking gopls for completions…"
	return m, lookup.CompletionCmd(m.lsp, p.Path(), row, col, len(prefix))
}

// acceptCompletion inserts the highlighted item's InsertText at the
// cursor, first deleting the word-prefix the user already typed so the
// resulting buffer reads correctly. Selecting an item is a buffer edit
// — we send a didChange to the language server if it's running.
func (m model) acceptCompletion() (tea.Model, tea.Cmd) {
	item, ok := m.completePopup.Selected()
	m.dismissCompletion()
	if !ok {
		return m, nil
	}
	p := m.bufs.Active()
	if p == nil {
		return m, nil
	}
	pl := len(complete.WordPrefix(p.LinePrefix()))
	if pl > 0 {
		row := p.CursorRow()
		col := p.CursorCol()
		line := p.Line(row)
		if col-pl >= 0 && col <= len(line) {
			newLine := line[:col-pl] + line[col:]
			*p = p.SetLine(row, newLine).JumpTo(row, col-pl)
		}
	}
	*p = p.InsertText(item.InsertText)
	m.status = "inserted " + item.Label
	var cmd tea.Cmd
	if m.lsp != nil && isGoFile(p.Path()) {
		v := m.lspVersions[p.Path()] + 1
		m.lspVersions[p.Path()] = v
		cmd = m.lspChangeCmd(p.Path(), v, p.Contents())
	}
	return m, cmd
}

// dismissCompletion clears the completion popup state. Safe to call
// even when no popup is active.
func (m *model) dismissCompletion() {
	m.completePopup = complete.New()
	m.completeReqPath = ""
	m.completeReqRow = 0
	m.completeReqCol = 0
	if m.overlay == overlayCompletion {
		m.overlay = overlayNone
	}
}

// triggerCodeActions fires a textDocument/codeAction request at the cursor
// and stashes the request site so a late response that arrived after the
// cursor moved gets discarded in the message handler. The popup arms on
// the response — there's nothing to show until the server answers.
func (m model) triggerCodeActions() (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil || p.Path() == "" {
		m.status = "open a file first (ctrl+p)"
		return m, nil
	}
	row := p.CursorRow()
	col := p.CursorCol()
	m.caReqPath = p.Path()
	m.caReqRow = row
	m.caReqCol = col
	m.status = "asking gopls for code actions…"
	return m, lookup.CodeActionCmd(m.lsp, p.Path(), row, col)
}

// handleCodeActionMsg either arms the popup with the server's response, or
// surfaces an empty-result / error status when there's nothing to show.
// Late responses (cursor already moved) are discarded so the popup always
// reflects the current focus.
func (m model) handleCodeActionMsg(msg lookup.CodeActionMsg) (tea.Model, tea.Cmd) {
	if msg.Path != m.caReqPath || msg.Row != m.caReqRow || msg.Col != m.caReqCol {
		return m, nil
	}
	if msg.Err != nil {
		if msg.Err.Error() == "no language server" {
			m.status = "code actions: no language server"
		} else {
			m.status = "code actions: " + msg.Err.Error()
		}
		return m, nil
	}
	if len(msg.Items) == 0 {
		m.status = "no code actions here"
		return m, nil
	}
	m.caPopup = m.caPopup.WithItems(msg.Items)
	m.overlay = overlayCodeAction
	m.status = fmt.Sprintf("%d code action(s)", len(msg.Items))
	return m, nil
}

// acceptCodeAction applies the selected workspace edit and dismisses the
// popup. Disabled rows refuse to apply (Popup.Selected returns ok=false);
// the status bar surfaces the disabled reason so the user knows why.
func (m model) acceptCodeAction() (tea.Model, tea.Cmd) {
	item, ok := m.caPopup.Selected()
	if !ok {
		if item.Disabled != "" {
			m.status = "action disabled: " + item.Disabled
		} else {
			m.status = "no action selected"
		}
		m.dismissCodeAction()
		return m, nil
	}
	m.dismissCodeAction()
	if item.Edit.Empty() {
		// Some servers attach a Command instead of an Edit; v1 doesn't
		// dispatch commands yet, so we report the no-op honestly rather
		// than pretending it worked.
		m.status = "action applied no edits"
		return m, nil
	}
	model, cmd, n := m.applyWorkspaceEdit(item.Edit)
	model.status = fmt.Sprintf("applied %q (%d file%s)", item.Title, n, pluralS(n))
	return model, cmd
}

// dismissCodeAction clears popup state and closes the overlay. Safe to
// call when no popup is up.
func (m *model) dismissCodeAction() {
	m.caPopup = codeaction.New()
	m.caReqPath = ""
	m.caReqRow = 0
	m.caReqCol = 0
	if m.overlay == overlayCodeAction {
		m.overlay = overlayNone
	}
}

// triggerRename starts the rename flow: ask prepareRename whether the
// cursor sits on a renamable token. The prompt opens on the response so
// it can pre-fill the identifier (read from source over the response's
// range, since gopls may return a default-behavior shape).
func (m model) triggerRename() (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil || p.Path() == "" {
		m.status = "open a file first (ctrl+p)"
		return m, nil
	}
	row := p.CursorRow()
	col := p.CursorCol()
	m.pendingRename = pendingRename{path: p.Path(), row: row, col: col}
	m.status = "asking gopls if rename is available…"
	return m, lookup.PrepareRenameCmd(m.lsp, p.Path(), row, col)
}

// findReferencesAtCursor is the alt+u handler. Resolves the identifier
// under the cursor via the same character-class walk gopls uses (no LSP
// call needed just to know what we're hovering), opens the multibuffer
// overlay with a "references to X" title, and fires the async
// findrefs.FindReferencesCmd. Errors come back through the standard
// multibuffer.FragmentsMsg pathway; empty results land as a Fragments
// slice of length zero, which the pane renders as "no fragments — press
// esc to close" — accurate UX for "the LSP returned no hits."
//
// Early exits:
//   - no active buffer: status hint, no overlay opened
//   - no LSP client attached yet: status hint, no overlay opened
//   - cursor not on an identifier: status hint, no overlay opened
func (m model) findReferencesAtCursor() (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil || p.Path() == "" {
		m.status = "open a file first (ctrl+p)"
		return m, nil
	}
	if m.lsp == nil {
		m.status = "references: no language server attached"
		return m, nil
	}
	row := p.CursorRow()
	col := p.CursorCol()
	sym := findrefs.Symbol(p.Contents(), row, col)
	if sym == "" {
		m.status = "references: no identifier under cursor"
		return m, nil
	}
	m.overlay = overlayMultibuffer
	m.multibufPane = m.multibufPane.
		WithSize(m.width-4, m.height-4).
		Reset("references to " + sym).
		Focus()
	if ap := m.bufs.Active(); ap != nil {
		*ap = ap.Blur()
	}
	return m, findrefs.FindReferencesCmd(m.lsp, p.Path(), row, col, findrefs.DefaultContextLines, nil)
}

// handlePrepareRenameMsg arms the rename prompt when the server says the
// position is renamable, or surfaces a status hint when it's not.
func (m model) handlePrepareRenameMsg(msg lookup.PrepareRenameMsg) (tea.Model, tea.Cmd) {
	if msg.Path != m.pendingRename.path || msg.Row != m.pendingRename.row || msg.Col != m.pendingRename.col {
		return m, nil
	}
	if msg.Err != nil {
		if msg.Err.Error() == "no language server" {
			m.status = "rename: no language server"
		} else {
			m.status = "rename: " + msg.Err.Error()
		}
		return m, nil
	}
	if !msg.Result.Available {
		m.status = "rename not available at cursor"
		return m, nil
	}
	current := m.identifierAtCursor(msg.Path, msg.Row, msg.Col, msg.Result)
	hint := filepath.Base(msg.Path)
	if rel, err := filepath.Rel(m.root, msg.Path); err == nil {
		hint = rel
	}
	m.renamePrompt = m.renamePrompt.WithCurrent(current, hint)
	m.overlay = overlayRename
	m.status = "rename: type new name, enter to apply"
	return m, nil
}

// routeRename forwards a keypress into the rename.Prompt. Enter fires the
// rename request; Esc cancels.
func (m model) routeRename(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch km.Type {
	case tea.KeyEsc:
		m.dismissRename()
		m.status = "rename cancelled"
		return m, nil
	case tea.KeyEnter:
		newName := m.renamePrompt.Value()
		current := m.renamePrompt.Current()
		if newName == "" {
			m.renamePrompt = m.renamePrompt.WithError("name is empty")
			return m, nil
		}
		if newName == current {
			m.dismissRename()
			m.status = "rename: no change"
			return m, nil
		}
		path := m.pendingRename.path
		row := m.pendingRename.row
		col := m.pendingRename.col
		m.status = fmt.Sprintf("renaming %s → %s…", current, newName)
		return m, lookup.RenameCmd(m.lsp, path, row, col, newName)
	case tea.KeyBackspace:
		m.renamePrompt = m.renamePrompt.Backspace()
		return m, nil
	case tea.KeyLeft:
		m.renamePrompt = m.renamePrompt.MoveLeft()
		return m, nil
	case tea.KeyRight:
		m.renamePrompt = m.renamePrompt.MoveRight()
		return m, nil
	case tea.KeyHome, tea.KeyCtrlA:
		m.renamePrompt = m.renamePrompt.MoveHome()
		return m, nil
	case tea.KeyEnd, tea.KeyCtrlE:
		m.renamePrompt = m.renamePrompt.MoveEnd()
		return m, nil
	case tea.KeyRunes:
		for _, r := range km.Runes {
			m.renamePrompt = m.renamePrompt.Type(r)
		}
		return m, nil
	}
	return m, nil
}

// handleRenameMsg applies the workspace edit returned by gopls. Empty
// edits and errors are surfaced as a status hint; the prompt itself was
// already closed by the Enter handler in routeRename.
func (m model) handleRenameMsg(msg lookup.RenameMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		// Surface the error in the prompt so the user can retype with a
		// different name (gopls rejects conflicts here).
		m.renamePrompt = m.renamePrompt.WithError(msg.Err.Error())
		if !m.renamePrompt.Open() {
			// Prompt was already dismissed; surface the failure on the
			// status bar instead.
			m.status = "rename failed: " + msg.Err.Error()
		}
		return m, nil
	}
	if msg.Edit.Empty() {
		m.dismissRename()
		m.status = "rename returned no edits"
		return m, nil
	}
	model, cmd, n := m.applyWorkspaceEdit(msg.Edit)
	model.dismissRename()
	model.status = fmt.Sprintf("renamed to %s (%d file%s)", msg.NewName, n, pluralS(n))
	return model, cmd
}

// dismissRename closes the rename prompt and clears its pending cursor.
func (m *model) dismissRename() {
	m.renamePrompt = rename.New()
	m.pendingRename = pendingRename{}
	if m.overlay == overlayRename {
		m.overlay = overlayNone
	}
}

// identifierAtCursor returns the identifier the rename will rewrite,
// reading from the active buffer when its path matches, or from disk
// otherwise. The PrepareRenameResult's range pins the span; gopls
// sometimes returns a zero range ({defaultBehavior:true}), in which case
// we walk the source from cursor to find the surrounding identifier.
func (m model) identifierAtCursor(path string, row, col int, res nooklsp.PrepareRenameResult) string {
	src := m.sourceFor(path)
	if src == "" {
		return ""
	}
	lines := strings.Split(src, "\n")
	if res.StartLine != res.EndLine || res.StartCol != res.EndCol {
		if res.StartLine >= 0 && res.StartLine < len(lines) {
			line := lines[res.StartLine]
			if res.StartCol >= 0 && res.EndCol <= len(line) && res.EndCol >= res.StartCol {
				return line[res.StartCol:res.EndCol]
			}
		}
	}
	if row < 0 || row >= len(lines) {
		return ""
	}
	line := lines[row]
	if col < 0 || col > len(line) {
		return ""
	}
	start := col
	for start > 0 && isIdentByte(line[start-1]) {
		start--
	}
	end := col
	for end < len(line) && isIdentByte(line[end]) {
		end++
	}
	if start >= end {
		return ""
	}
	return line[start:end]
}

// sourceFor returns the current contents for path: the open buffer if
// there is one, else the on-disk file. Returns "" on miss.
func (m model) sourceFor(path string) string {
	for i := 0; i < m.bufs.Count(); i++ {
		p := m.bufs.At(i)
		if p != nil && p.Path() == path {
			return p.Contents()
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// isIdentByte reports whether b is part of an ASCII identifier. The
// prepareRename fallback only walks ASCII — gopls returns proper ranges
// for Unicode identifiers, so the fallback is the rare case.
func isIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// applyWorkspaceEdit applies edit across both open buffers and on-disk
// files. For each affected path: if a buffer is open, the buffer's
// contents are rewritten and a fresh didChange is published; otherwise
// the file is read, edited, and written back to disk. Returns the
// updated model, the batched LSP cmds, and the number of files touched
// (so callers can build a status hint).
func (m model) applyWorkspaceEdit(edit nooklsp.WorkspaceEditChange) (model, tea.Cmd, int) {
	paths := edit.Paths()
	if len(paths) == 0 {
		return m, nil, 0
	}
	// Build the sources map and remember which paths are currently open
	// so we know whether to write back to disk or to the buffer.
	sources := make(map[string]string, len(paths))
	openIdx := make(map[string]int, len(paths))
	for _, path := range paths {
		idx := -1
		for i := 0; i < m.bufs.Count(); i++ {
			p := m.bufs.At(i)
			if p != nil && p.Path() == path {
				idx = i
				break
			}
		}
		if idx >= 0 {
			openIdx[path] = idx
			sources[path] = m.bufs.At(idx).Contents()
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			// Skip unreadable paths — we still apply the rest. The
			// status hint reflects only the files we actually touched.
			continue
		}
		sources[path] = string(b)
	}
	updated := nooklsp.ApplyWorkspaceEdit(sources, edit)

	var cmds []tea.Cmd
	written := 0
	for _, path := range paths {
		after, ok := updated[path]
		if !ok {
			continue
		}
		before := sources[path]
		if after == before {
			continue
		}
		if idx, isOpen := openIdx[path]; isOpen {
			p := m.bufs.At(idx)
			if p == nil {
				continue
			}
			*p = p.ReplaceAllFromString(after)
			if m.lsp != nil && isGoFile(path) {
				v := m.lspVersions[path] + 1
				m.lspVersions[path] = v
				if c := m.lspChangeCmd(path, v, after); c != nil {
					cmds = append(cmds, c)
				}
			}
			written++
			continue
		}
		if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
			// Surface but keep going.
			m.status = "write failed for " + filepath.Base(path) + ": " + err.Error()
			continue
		}
		// If the buffer manager is tracking this path under a different
		// active state (it shouldn't be — we already checked above), be
		// defensive and refresh anyway.
		m.bufs.RefreshIfOpen(path)
		written++
	}
	if cmd := m.refreshGitCmd(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return m, nil, written
	}
	if len(cmds) == 1 {
		return m, cmds[0], written
	}
	return m, tea.Batch(cmds...), written
}

// pluralS returns "" for 1, "s" otherwise. Saves a fmt branch at every
// "N file(s)" caller.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// openFinder opens the in-file finder over the active buffer in the requested
// mode. The pattern is preserved across opens so users can flip find↔replace
// without retyping.
func (m model) openFinder(mode finder.Mode) (tea.Model, tea.Cmd) {
	m.overlay = overlayFinder
	m.finder = m.finder.WithSize(m.width).Open(mode)
	m = m.resize()
	m.refreshFinderMatches()
	m.syncEditorToFinder()
	return m, nil
}

// closeFinder hides the bar and clears editor match highlights.
func (m *model) closeFinder() {
	m.finder = m.finder.Close()
	if p := m.bufs.Active(); p != nil {
		*p = p.ClearSearchMatches()
	}
	if m.overlay == overlayFinder {
		m.overlay = overlayNone
	}
	*m = m.resize()
}

// routeFinder dispatches a key event into the finder and reacts to the
// emitted event: re-running search, jumping the editor cursor, applying
// replacements.
func (m model) routeFinder(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	var ev finder.Event
	m.finder, ev = m.finder.Update(km)
	switch ev {
	case finder.EventClose:
		m.closeFinder()
		return m, nil
	case finder.EventPatternChanged:
		m.refreshFinderMatches()
		m.syncEditorToFinder()
		return m, nil
	case finder.EventJumpNext:
		m.finder = m.finder.Next()
		m.syncEditorToFinder()
		return m, nil
	case finder.EventJumpPrev:
		m.finder = m.finder.Prev()
		m.syncEditorToFinder()
		return m, nil
	case finder.EventReplaceCurrent:
		return m.replaceCurrentMatch()
	case finder.EventReplaceAll:
		return m.replaceAllMatches()
	}
	m.syncEditorToFinder()
	return m, nil
}

// refreshFinderMatches re-runs Search over the active buffer using the
// finder's current pattern/flags and feeds the result back into the finder.
// On pattern compile errors the error is stashed on the finder so View can
// surface it.
func (m *model) refreshFinderMatches() {
	p := m.bufs.Active()
	if p == nil {
		m.finder = m.finder.WithMatches(nil, false)
		return
	}
	lines := p.Lines()
	ms, err := finder.Search(lines, m.finder.Pattern(), m.finder.UseRegex(), m.finder.CaseSensitive())
	if err != nil {
		m.finder = m.finder.WithMatches(nil, false).SetPatternErr(err)
		return
	}
	m.finder = m.finder.SetPatternErr(nil).WithMatches(ms, false)
}

// syncEditorToFinder pushes the finder's match list to the active editor pane
// for highlight rendering, and jumps the cursor to the current match.
func (m *model) syncEditorToFinder() {
	p := m.bufs.Active()
	if p == nil {
		return
	}
	ms := m.finder.Matches()
	ranges := make([]editor.Range, len(ms))
	for i, mt := range ms {
		ranges[i] = editor.Range{Row: mt.Row, Start: mt.StartCol, End: mt.EndCol}
	}
	idx := m.finder.CurrentIndex()
	updated := p.WithSearchMatches(ranges, idx)
	if cur, ok := m.finder.CurrentMatch(); ok {
		updated = updated.JumpTo(cur.Row+1, cur.StartCol+1)
	}
	*p = updated
}

// replaceCurrentMatch substitutes the currently selected match in the active
// buffer with the finder's replacement text. After the edit, search is
// re-run so the match list reflects the new buffer.
func (m model) replaceCurrentMatch() (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil {
		return m, nil
	}
	cur, ok := m.finder.CurrentMatch()
	if !ok {
		return m, nil
	}
	var re *regexp.Regexp
	if m.finder.UseRegex() {
		compiled, err := finder.CompileRegex(m.finder.Pattern(), m.finder.CaseSensitive())
		if err != nil {
			m.finder = m.finder.SetPatternErr(err)
			return m, nil
		}
		re = compiled
	}
	line := p.Line(cur.Row)
	newLine := finder.ApplyReplacement(line, cur, m.finder.Replacement(), re)
	*p = p.SetLine(cur.Row, newLine)

	var cmd tea.Cmd
	if m.lsp != nil && isGoFile(p.Path()) {
		v := m.lspVersions[p.Path()] + 1
		m.lspVersions[p.Path()] = v
		cmd = m.lspChangeCmd(p.Path(), v, p.Contents())
	}

	m.refreshFinderMatches()
	m.finder = m.finder.SelectMatchAt(cur.Row, cur.StartCol)
	m.syncEditorToFinder()
	m.status = "replaced 1 match"
	return m, cmd
}

// replaceAllMatches walks every match in the buffer and substitutes them with
// the replacement, in reverse byte order per row so column indices stay valid
// during the rewrite.
func (m model) replaceAllMatches() (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil {
		return m, nil
	}
	matches := m.finder.Matches()
	if len(matches) == 0 {
		return m, nil
	}
	var re *regexp.Regexp
	if m.finder.UseRegex() {
		compiled, err := finder.CompileRegex(m.finder.Pattern(), m.finder.CaseSensitive())
		if err != nil {
			m.finder = m.finder.SetPatternErr(err)
			return m, nil
		}
		re = compiled
	}
	byRow := map[int][]finder.Match{}
	for _, mt := range matches {
		byRow[mt.Row] = append(byRow[mt.Row], mt)
	}
	count := 0
	updated := *p
	for row, rowMatches := range byRow {
		line := updated.Line(row)
		for i := len(rowMatches) - 1; i >= 0; i-- {
			line = finder.ApplyReplacement(line, rowMatches[i], m.finder.Replacement(), re)
			count++
		}
		updated = updated.SetLine(row, line)
	}
	*p = updated
	var cmd tea.Cmd
	if m.lsp != nil && isGoFile(p.Path()) {
		v := m.lspVersions[p.Path()] + 1
		m.lspVersions[p.Path()] = v
		cmd = m.lspChangeCmd(p.Path(), v, p.Contents())
	}
	m.refreshFinderMatches()
	m.syncEditorToFinder()
	m.status = fmt.Sprintf("replaced %d matches", count)
	return m, cmd
}

func (m model) toggleComposer() (tea.Model, tea.Cmd) {
	if m.right == rightComposer {
		m.right = rightNone
		m.composer = m.composer.Blur()
		if p := m.bufs.Active(); p != nil {
			*p = p.Focus()
		}
		m = m.resize()
		return m, nil
	}
	// Snap context onto the composer.
	ctx := composer.Context{
		Root:  m.root,
		Files: m.files,
	}
	var activePath string
	if p := m.bufs.Active(); p != nil && p.Path() != "" {
		ctx.OpenPath = p.Path()
		ctx.OpenContents = p.Contents()
		activePath = p.Path()
	}
	m.composer = m.composer.WithContext(ctx).WithActivePath(activePath).Focus()
	m.right = rightComposer
	m = m.resize()
	return m, nil
}

// closeActiveTab closes the active buffer (Ctrl+W). Dirty buffers stay
// open with a warning; the user must save first or hit Ctrl+S then Ctrl+W.
// We also send lsp.Close so gopls drops the document.
func (m model) closeActiveTab() (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil {
		return m, nil
	}
	if p.Dirty() {
		m.status = "buffer dirty — save first (ctrl+s)"
		return m, nil
	}
	path := m.bufs.CloseActive()
	delete(m.lspVersions, path)
	delete(m.diagnostics, path)
	var cmd tea.Cmd
	if m.lsp != nil && isGoFile(path) {
		cmd = m.lspCloseCmd(path)
	}
	if path != "" {
		m.status = "closed " + filepath.Base(path)
	}
	m = m.resize()
	m = m.applyDiagnosticsToActive()
	return m, cmd
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m model) routeProjectSearch(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If user is still typing the query, treat the search pane as "editing query"
	if m.search.Query() == "" || km.Type == tea.KeyRunes || km.Type == tea.KeyBackspace || km.Type == tea.KeySpace {
		// Only the query stage handles typing; once results stream, navigation keys win.
		// Simpler: accept Esc and Enter to start the search, otherwise build the query.
		switch km.Type {
		case tea.KeyEsc:
			m.overlay = overlayNone
			return m, nil
		case tea.KeyEnter:
			return m.startSearch()
		case tea.KeyBackspace:
			q := m.search.Query()
			if q != "" {
				m.search = m.search.Reset(q[:len(q)-1])
			}
			return m, nil
		case tea.KeyRunes:
			m.search = m.search.Reset(m.search.Query() + string(km.Runes))
			return m, nil
		case tea.KeySpace:
			m.search = m.search.Reset(m.search.Query() + " ")
			return m, nil
		}
	}
	// Once we've kicked off a search, route nav keys to the pane
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(km)
	return m, cmd
}

func (m model) startSearch() (tea.Model, tea.Cmd) {
	if strings.TrimSpace(m.search.Query()) == "" {
		return m, nil
	}
	if m.searchCancel != nil {
		m.searchCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.searchCancel = cancel
	query := m.search.Query()
	root := m.root
	out, done := search.Run(ctx, root, query)

	// Convert the channels into a recursive tea.Cmd.
	var pump func() tea.Msg
	pump = func() tea.Msg {
		select {
		case mch, ok := <-out:
			if !ok {
				err := <-done
				return search.DoneMsg{Err: err, Matches: 0}
			}
			return search.MatchMsg{Match: mch}
		case err := <-done:
			return search.DoneMsg{Err: err, Matches: 0}
		}
	}
	// Each MatchMsg arrival re-pumps via the Update handler in View+Init? Simpler:
	// wrap pump so that after one read we keep reading until the channel closes.
	// We achieve that by re-scheduling pump from the Update handler when we get a
	// MatchMsg. To keep the wiring local, here's the simplest version: schedule
	// pump now, and Update returns a fresh pump on each MatchMsg.
	_ = pump

	return m, m.searchPumpCmd(out, done)
}

// searchPumpCmd returns a tea.Cmd that drains one event from out/done. The
// Update handler re-schedules it on every MatchMsg until DoneMsg fires.
func (m model) searchPumpCmd(out <-chan search.Match, done <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case mch, ok := <-out:
			if !ok {
				err := <-done
				return search.DoneMsg{Err: err}
			}
			return searchPumpMsg{match: mch, more: true, out: out, done: done}
		case err := <-done:
			return search.DoneMsg{Err: err}
		}
	}
}

// searchPumpMsg carries one match plus the channels for the next pump.
type searchPumpMsg struct {
	match search.Match
	more  bool
	out   <-chan search.Match
	done  <-chan error
}

// toggleMdPreview opens or closes the markdown preview pane on the right
// column. When opening, requires the active buffer to be a markdown file
// (.md / .markdown) — non-markdown buffers get a status-bar hint and the
// pane stays closed. Closing also blurs the pane so the editor reclaims
// focus on the next keystroke.
//
// Preview is a right-column sibling of git / term / composer: opening
// one closes the others, since the right column only renders a single
// pane at a time.
func (m model) toggleMdPreview() (tea.Model, tea.Cmd) {
	if m.right == rightPreview {
		m.right = rightNone
		m.mdPane = m.mdPane.Blur()
		m = m.resize()
		return m, nil
	}
	p := m.bufs.Active()
	if p == nil || p.Path() == "" {
		m.status = "open a markdown file to preview"
		return m, nil
	}
	if !mdpreview.IsMarkdownPath(p.Path()) {
		m.status = "preview only works on .md / .markdown files"
		return m, nil
	}
	// Close any sibling right pane and feed the current buffer into the
	// preview so the open render reflects the buffer state, not a stale
	// snapshot from the last open.
	if m.right == rightGit {
		m.gitPane = m.gitPane.Blur()
	}
	if m.right == rightTerm {
		m.termPane = m.termPane.Blur()
	}
	if m.right == rightComposer {
		m.composer = m.composer.Blur()
	}
	m.mdPane = m.mdPane.WithSource(p.Path(), p.Contents()).Focus()
	m.right = rightPreview
	m = m.resize()
	m.status = "preview: " + filepath.Base(p.Path())
	return m, nil
}

// expandSnippetAtCursor looks at the word immediately before the cursor in
// the active buffer, looks it up against the snippet library under the
// buffer's language scope, and (if it matches) replaces the prefix with the
// expansion. Multiple matches pick the first; the rest are reachable via
// `nook snippets list` (future surface).
func (m model) expandSnippetAtCursor() (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil {
		m.status = "open a file first"
		return m, nil
	}
	row := p.CursorRow()
	col := p.CursorCol()
	line := p.Line(row)
	if col < 0 || col > len(line) {
		return m, nil
	}
	prefix := snippets.PrefixAt(line[:col])
	if prefix == "" {
		m.status = "snippet: place cursor after a word prefix"
		return m, nil
	}
	scope := snippets.ScopeFor(p.Path())
	hits := m.snipLib.Lookup(scope, prefix)
	if len(hits) == 0 {
		m.status = "snippet: no match for " + prefix
		return m, nil
	}
	vars := snippets.DefaultVariables()
	if path := p.Path(); path != "" {
		base := filepath.Base(path)
		vars.Filename = base
		vars.FilenameBase = strings.TrimSuffix(base, filepath.Ext(base))
	}
	exp, err := snippets.Expand(hits[0].Body, vars)
	if err != nil {
		m.status = "snippet: " + err.Error()
		return m, nil
	}
	prefixStart := col - len(prefix)
	*p = p.ExpandSnippet(prefixStart, exp)
	if p.InSnippetMode() {
		m.status = "snippet: " + hits[0].Name + " — Tab to advance, Esc to exit"
	} else {
		m.status = "snippet: " + hits[0].Name
	}
	return m, nil
}

// openTasksOverlay loads .nook/tasks.toml (or Go defaults when missing)
// and pops the picker. A parse error doesn't block the overlay: the pane
// shows the error in its header and falls through to the default tasks
// for the detected project type, so the user can still run something.
func (m model) openTasksOverlay() (tea.Model, tea.Cmd) {
	ts, loadErr := tasks.LoadOrDefaults(m.root)
	pane := m.tasksPane.WithTasks(ts)
	if loadErr != nil {
		pane = pane.WithLoadError(loadErr)
	}
	m.tasksPane = pane.WithSize(m.width-4, m.height-4).Focus()
	m.overlay = overlayTasks
	if p := m.bufs.Active(); p != nil {
		*p = p.Blur()
	}
	return m, nil
}

// spawnTask is the RunTaskMsg handler: it kills any prior runner (so the
// pane never juggles two live processes), starts a fresh one, flips the
// pane to ModeOutput, and batches the three streaming Cmds the pane
// needs to learn about the run.
func (m model) spawnTask(t tasks.Task) (tea.Model, tea.Cmd) {
	if m.activeRunner != nil {
		m.activeRunner.Kill()
		m.activeRunner = nil
	}
	r, err := tasks.Start(context.Background(), m.root, t)
	if err != nil {
		m.status = "task: " + err.Error()
		return m, nil
	}
	m.activeRunner = r
	m.tasksPane = m.tasksPane.SwitchToOutput(t, r.ID())
	return m, tea.Batch(r.StartedCmd(), r.NextLineCmd(), r.WaitCmd())
}

func (m model) toggleTerm() (tea.Model, tea.Cmd) {
	if m.right == rightTerm {
		m.right = rightNone
		m.termPane = m.termPane.Blur()
		m = m.resize()
		return m, nil
	}
	if m.termPane.Session() == nil {
		p, err := m.termPane.Start()
		if err != nil {
			m.status = "term start failed: " + err.Error()
			return m, nil
		}
		m.termPane = p
		// pump output -> tea via OutputMsg
		sess := p.Session()
		ch := make(chan []byte, 64)
		go func() {
			_ = sess.ReadLoop(context.Background(), ch)
			close(ch)
		}()
		// re-emit chunks as OutputMsg via a recursive Cmd
		m.right = rightTerm
		m.termPane = m.termPane.Focus()
		m = m.resize()
		return m, m.termPumpCmd(ch)
	}
	m.right = rightTerm
	m.termPane = m.termPane.Focus()
	m = m.resize()
	return m, nil
}

// toggleTree opens or closes the left-side file tree pane. Opening also
// refreshes the walk (so newly-created files appear) and gives the tree
// keyboard focus; closing blurs it. The tree is the most natural place
// to land focus when the user has no file open yet, but we leave the
// "where to focus next" decision to the editor's own focus logic.
func (m model) toggleTree() (tea.Model, tea.Cmd) {
	if m.showTree {
		m.showTree = false
		m.treePane.Blur()
		m = m.resize()
		m.status = "file tree closed"
		return m, nil
	}
	m.showTree = true
	m.treePane.Refresh()
	if p := m.bufs.Active(); p != nil && p.Path() != "" {
		m.treePane.Reveal(p.Path())
	}
	m.treePane.Focus()
	m = m.resize()
	m.status = "file tree open"
	return m, nil
}

func (m model) termPumpCmd(ch <-chan []byte) tea.Cmd {
	return func() tea.Msg {
		b, ok := <-ch
		if !ok {
			return term.ExitMsg{}
		}
		return termPumpMsg{data: b, ch: ch}
	}
}

type termPumpMsg struct {
	data []byte
	ch   <-chan []byte
}

func (m model) resize() model {
	if m.width < 60 || m.height < 12 {
		return m
	}
	// Layout:
	// - tab bar (1 row) when at least one buffer is open
	// - top: optional tree (left) + workspace (editor) + right (git/term/diff)
	// - bottom: status bar
	treeW := 0
	if m.showTree {
		treeW = m.width / 5
		if treeW < 22 {
			treeW = 22
		}
		if treeW > 40 {
			treeW = 40
		}
	}
	leftW := m.width - treeW
	if treeW > 0 {
		leftW-- // separator column
	}
	rightW := 0
	if m.right != rightNone {
		rightW = m.width / 3
		if rightW < 40 {
			rightW = 40
		}
		if rightW > m.width-40 {
			rightW = m.width - 40
		}
		leftW = m.width - rightW - 1 - treeW
		if treeW > 0 {
			leftW-- // separator between tree and editor
		}
	}
	if leftW < 20 {
		// Editor needs at least 20 cols to be useful; if the tree's
		// requested width would starve it, shrink the tree first.
		shrink := 20 - leftW
		treeW -= shrink
		if treeW < 0 {
			treeW = 0
		}
		leftW = 20
	}
	bodyH := m.height - 2
	if m.bufs.Count() > 0 {
		bodyH-- // reserve a row for the tab bar
	}
	if fh := m.finder.Height(); fh > 0 {
		bodyH -= fh // reserve rows for the find/replace bar
	}
	m.bufs.WithSize(leftW, bodyH)
	m.gitPane = m.gitPane.WithSize(rightW, bodyH)
	m.termPane = m.termPane.WithSize(rightW, bodyH)
	m.composer = m.composer.WithSize(rightW, bodyH)
	m.mdPane = m.mdPane.WithSize(rightW, bodyH)
	m.editPane = m.editPane.WithSize(min(70, m.width-4), 10)
	m.search = m.search.WithSize(m.width-4, m.height-6)
	m.picker = m.picker.WithSize(m.width-8, m.height-6)
	m.multibufPane = m.multibufPane.WithSize(m.width-4, m.height-4)
	m.tasksPane = m.tasksPane.WithSize(m.width-4, m.height-4)
	m.finder = m.finder.WithSize(m.width)
	if treeW > 0 {
		m.treePane.SetSize(treeW, bodyH)
	}
	return m
}

func (m model) View() string {
	t := m.theme

	// status bar — richer than the bare keymap. Format: <hint> · <project> · <position> · <diag>
	statusBar := m.renderStatusBar()

	// modal overlay
	if m.overlay == overlayFilePicker {
		return lipgloss.JoinVertical(lipgloss.Left, centerOverlay(m.width, m.height-1, m.picker.View()), statusBar)
	}
	if m.overlay == overlayProjectSearch {
		return lipgloss.JoinVertical(lipgloss.Left, centerOverlay(m.width, m.height-1, m.search.View()), statusBar)
	}
	if m.overlay == overlayInlineEdit {
		float := centerOverlay(m.width, m.height-1, m.editPane.View())
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlayHelp {
		float := centerOverlay(m.width, m.height-1, help.View(t, m.width))
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlayHover {
		// Hover floats over the editor rather than replacing it — the
		// user wants to see the symbol they're hovering on with the
		// info attached. We pick a width that's narrow enough to read
		// (~60ch) and clamp height to a third of the screen.
		boxW := 60
		if boxW > m.width-4 {
			boxW = m.width - 4
		}
		maxLines := (m.height - 2) / 3
		if maxLines < 4 {
			maxLines = 4
		}
		box := hover.View(t, m.hoverContents, boxW, maxLines)
		float := centerOverlay(m.width, m.height-1, box)
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlayCompletion {
		// Completion menu floats centered like hover. We cap width at
		// ~50ch so labels stay readable and rows at a third of the
		// screen so the menu doesn't dominate the editor.
		boxW := 50
		if boxW > m.width-4 {
			boxW = m.width - 4
		}
		maxRows := (m.height - 2) / 3
		if maxRows < 4 {
			maxRows = 4
		}
		box := m.completePopup.View(t, boxW, maxRows)
		float := centerOverlay(m.width, m.height-1, box)
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlayCodeAction {
		// Code-action menu uses the same float-centered shape as the
		// completion popup. The picker auto-fits its width within the
		// requested cap.
		boxW := 52
		if boxW > m.width-4 {
			boxW = m.width - 4
		}
		maxRows := (m.height - 2) / 3
		if maxRows < 4 {
			maxRows = 4
		}
		box := m.caPopup.View(t, boxW, maxRows)
		float := centerOverlay(m.width, m.height-1, box)
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlayRename {
		boxW := 56
		if boxW > m.width-4 {
			boxW = m.width - 4
		}
		box := m.renamePrompt.View(t, boxW)
		float := centerOverlay(m.width, m.height-1, box)
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlayMultibuffer {
		// Multibuffer takes the whole body area minus the status bar — it's a
		// scrollable surface, not a popup, so floating it small would defeat
		// the point. Width gets a small inset; height fills the rest.
		boxW := m.width - 4
		boxH := m.height - 4
		if boxH < 6 {
			boxH = 6
		}
		pane := m.multibufPane.WithSize(boxW, boxH)
		float := centerOverlay(m.width, m.height-1, pane.View())
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlayDiagnostics {
		// Diagnostics panel uses the same large-surface shape as multibuffer.
		// The list can grow long across a real workspace; floating it small
		// would force a tiny visible window.
		boxW := m.width - 4
		boxH := m.height - 4
		if boxH < 6 {
			boxH = 6
		}
		pane := m.diagPane.WithSize(boxW, boxH)
		float := centerOverlay(m.width, m.height-1, pane.View())
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlayTasks {
		// Tasks overlay: same large-surface shape so a long output buffer
		// from `go test ./...` doesn't fight a small float.
		boxW := m.width - 4
		boxH := m.height - 4
		if boxH < 6 {
			boxH = 6
		}
		pane := m.tasksPane.WithSize(boxW, boxH)
		float := centerOverlay(m.width, m.height-1, pane.View())
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}

	left := m.renderMainColumn()

	tabBar := m.renderTabBar()

	var finderBar string
	if m.finder.IsOpen() {
		finderBar = m.finder.View()
	}

	bodyH := m.height - 2
	if tabBar != "" {
		bodyH--
	}
	if fh := m.finder.Height(); fh > 0 {
		bodyH -= fh
	}
	verticalBar := func() string {
		return lipgloss.NewStyle().Foreground(t.Border).Render(strings.Repeat("│\n", bodyH))
	}

	// Body assembly: tree + main + (optional) right pane, joined with
	// thin border bars.
	tree := ""
	if m.showTree {
		tree = m.treePane.View()
	}

	pieces := []string{}
	if tree != "" {
		pieces = append(pieces, tree, verticalBar())
	}
	pieces = append(pieces, left)
	if m.right != rightNone {
		var right string
		switch m.right {
		case rightGit:
			right = m.gitPane.View()
		case rightTerm:
			right = m.termPane.View()
		case rightDiff:
			right = renderDiff(t, m.diffTitle, m.diffBody, m.width/3, m.height-2)
		case rightComposer:
			right = m.composer.View()
		case rightPreview:
			right = m.mdPane.View()
		}
		pieces = append(pieces, verticalBar(), right)
	}
	var body string
	if len(pieces) == 1 {
		body = pieces[0]
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, pieces...)
	}

	segments := make([]string, 0, 4)
	if tabBar != "" {
		segments = append(segments, tabBar)
	}
	segments = append(segments, body)
	if finderBar != "" {
		segments = append(segments, finderBar)
	}
	segments = append(segments, statusBar)
	return lipgloss.JoinVertical(lipgloss.Left, segments...)
}

// renderTabBar returns the styled tab strip or "" when no buffers are open.
func (m model) renderTabBar() string {
	if m.bufs.Count() == 0 {
		return ""
	}
	return tabbar.View(m.theme, m.bufs.Tabs(), m.bufs.ActiveIndex(), m.width)
}

// renderMainColumn returns the editor pane, OR the welcome card when no
// file has been opened yet. The welcome card replaces the empty editor's
// tilde-fill so first-run users see a usable surface instead of a blank
// vim-style buffer.
func (m model) renderMainColumn() string {
	if m.shouldShowWelcome() {
		w, h := m.editorSize()
		return welcome.View(m.theme, m.welcomeInfo(), w, h)
	}
	return m.bufs.Active().View()
}

// shouldShowWelcome is true when there are no open buffers. Closing the last
// buffer is symmetric with first-run: the welcome card returns. This drives
// the user back to ctrl+P naturally.
func (m model) shouldShowWelcome() bool {
	return m.bufs.Count() == 0
}

// editorSize returns the dimensions allocated to the editor pane. The
// welcome card needs to know this so it can self-clamp. Must mirror
// resize() exactly: tree on the left when visible, right pane on the
// right when active, and a minimum-20 editor floor that shrinks the
// tree when necessary.
func (m model) editorSize() (int, int) {
	treeW := 0
	if m.showTree {
		treeW = m.width / 5
		if treeW < 22 {
			treeW = 22
		}
		if treeW > 40 {
			treeW = 40
		}
	}
	leftW := m.width - treeW
	if treeW > 0 {
		leftW--
	}
	if m.right != rightNone {
		rightW := m.width / 3
		if rightW < 40 {
			rightW = 40
		}
		if rightW > m.width-40 {
			rightW = m.width - 40
		}
		leftW = m.width - rightW - 1 - treeW
		if treeW > 0 {
			leftW--
		}
	}
	if leftW < 20 {
		leftW = 20
	}
	bodyH := m.height - 2
	if m.bufs.Count() > 0 {
		bodyH--
	}
	return leftW, bodyH
}

// welcomeInfo gathers the everything-the-welcome-card-needs bundle.
// Capability probes (claude CLI, gopls) are cheap (LookPath stat) and
// fine to call every View; if that ever shows up in profiles, cache them
// at startup time.
func (m model) welcomeInfo() welcome.Info {
	return welcome.Info{
		Root:      m.root,
		FileCount: len(m.files),
		AI:        welcome.ProbeAI(m.aiClient != nil),
		LSP:       welcome.ProbeLSP(),
	}
}

// renderStatusBar builds the bottom-of-screen status line. Layout:
//
//	<status hint> · <project name> · <tab count> · <pos | dirty> · <diag>
//
// The project segment grounds the user in where they are; the tab counter
// (only shown when count > 1) signals other open buffers; the pos+dirty
// segment tells them which line they're on and whether they've saved;
// the diag segment surfaces LSP error/warning counts when present.
func (m model) renderStatusBar() string {
	t := m.theme
	muted := lipgloss.NewStyle().Foreground(t.TextMuted)
	dirty := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	dim := lipgloss.NewStyle().Foreground(t.TextMuted).Faint(true)

	parts := []string{m.status}

	if name := filepath.Base(m.root); name != "" && name != "." && name != "/" {
		parts = append(parts, muted.Render(name))
	}

	if n := m.bufs.Count(); n > 1 {
		parts = append(parts, muted.Render(fmt.Sprintf("%d/%d", m.bufs.ActiveIndex()+1, n)))
	}

	if p := m.bufs.Active(); p != nil && p.Path() != "" {
		pos := fmt.Sprintf("L%d:%d", p.CursorRow()+1, p.CursorCol()+1)
		seg := muted.Render(pos)
		if p.Dirty() {
			seg += " " + dirty.Render("●")
		}
		parts = append(parts, seg)
	}

	if errs, warns := m.diagCounts(); errs > 0 || warns > 0 {
		parts = append(parts, muted.Render(fmt.Sprintf("●%dE %dW", errs, warns)))
	}

	sep := dim.Render("  ·  ")
	statusText := strings.Join(parts, sep)

	return lipgloss.NewStyle().
		Background(t.Surface).
		Foreground(t.TextMuted).
		Width(m.width).
		Padding(0, 1).
		Render(statusText)
}

// activePath returns the path of the active buffer, or "" if no buffer is
// open. Convenience for the many sites that gate on path-presence.
func (m model) activePath() string {
	if p := m.bufs.Active(); p != nil {
		return p.Path()
	}
	return ""
}

func centerOverlay(w, h int, body string) string {
	bw, bh := lipgloss.Size(body)
	if bw >= w {
		bw = w
	}
	if bh >= h {
		bh = h
	}
	leftPad := (w - bw) / 2
	topPad := (h - bh) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	if topPad < 0 {
		topPad = 0
	}
	padded := lipgloss.NewStyle().Padding(topPad, leftPad).Render(body)
	return padded
}

// LSP wiring ----------------------------------------------------------------

type lspStartedMsg struct {
	client *nooklsp.Client
	err    error
}

type lspDiagnosticsMsg struct {
	URI   uri.URI
	Items []protocol.Diagnostic
}

type lspOpenedMsg struct {
	path string
	err  error
}

// isGoFile reports whether path has a .go extension. The LSP wiring only
// spawns gopls when this is true.
func isGoFile(path string) bool {
	return strings.HasSuffix(path, ".go")
}

// pathFromURI returns the local filesystem path for a file:// URI, or "" if
// the URI is not a file URI. uri.URI.Filename handles the Windows drive-letter
// prefix (file:///C:/foo → C:\foo) that a literal "file://" strip would leave
// as the un-openable "/C:/foo".
func pathFromURI(u uri.URI) string {
	if !strings.HasPrefix(string(u), "file://") {
		return ""
	}
	return u.Filename()
}

// ensureLSPForFile is invoked on every file open. If gopls is not yet running
// and the file is a Go source, kicks off Start. If gopls is already running,
// sends didOpen for the file.
func (m *model) ensureLSPForFile(path string) tea.Cmd {
	if !isGoFile(path) {
		return nil
	}
	if m.lsp != nil {
		contents := ""
		if p := m.bufs.Active(); p != nil && p.Path() == path {
			contents = p.Contents()
		}
		m.lspVersions[path] = 1
		return m.lspOpenCmd(path, contents)
	}
	if m.lspStarting {
		return nil
	}
	m.lspStarting = true
	m.status = "starting gopls…"
	return m.startLSPCmd()
}

// lspCloseCmd sends a didClose so gopls drops the document.
func (m model) lspCloseCmd(path string) tea.Cmd {
	cli := m.lsp
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cli.Close(ctx, path)
		return nil
	}
}

// startLSPCmd spawns gopls with the project root.
func (m model) startLSPCmd() tea.Cmd {
	root := m.root
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cli, err := nooklsp.Start(ctx, nooklsp.Options{RootDir: root})
		return lspStartedMsg{client: cli, err: err}
	}
}

// lspOpenCmd sends didOpen for a freshly opened file.
func (m model) lspOpenCmd(path, contents string) tea.Cmd {
	cli := m.lsp
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := cli.Open(ctx, path, "go", contents)
		return lspOpenedMsg{path: path, err: err}
	}
}

// lspChangeCmd sends a full-text didChange. version is the next monotonically
// increasing version for the document.
func (m model) lspChangeCmd(path string, version int32, contents string) tea.Cmd {
	cli := m.lsp
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cli.Change(ctx, path, version, contents)
		return nil
	}
}

// lspPumpCmd drains one diagnostics event from the channel and resurfaces it
// as a tea.Msg. The Update handler re-schedules pump on every receipt.
func (m model) lspPumpCmd(events <-chan nooklsp.DiagnosticsEvent) tea.Cmd {
	return func() tea.Msg {
		ev := <-events
		return lspDiagnosticsMsg{URI: ev.URI, Items: ev.Items}
	}
}

// applyDiagnosticsToActive pushes the row→severity map for the active buffer
// to its editor pane so gutter rendering branches by severity.
func (m model) applyDiagnosticsToActive() model {
	p := m.bufs.Active()
	if p == nil || p.Path() == "" {
		return m
	}
	items := m.diagnostics[p.Path()]
	if len(items) == 0 {
		*p = p.SetDiagnosticRows(nil)
		return m
	}
	rows := map[int]editor.Severity{}
	for _, d := range items {
		row := int(d.Range.Start.Line)
		sev := mapSeverity(d.Severity)
		if cur, ok := rows[row]; !ok || sev < cur {
			rows[row] = sev
		}
	}
	*p = p.SetDiagnosticRows(rows)
	return m
}

// mapSeverity converts the LSP enum to the editor's local severity. Lower
// values are worse (Error=1 wins over Warning=2 in the host's "minimum" merge).
func mapSeverity(s protocol.DiagnosticSeverity) editor.Severity {
	switch s {
	case protocol.DiagnosticSeverityError:
		return editor.SeverityError
	case protocol.DiagnosticSeverityWarning:
		return editor.SeverityWarning
	case protocol.DiagnosticSeverityInformation:
		return editor.SeverityInfo
	case protocol.DiagnosticSeverityHint:
		return editor.SeverityHint
	}
	return editor.SeverityError
}

// collectDiagnosticEntries walks the workspace-wide diagnostic store and
// produces one diagnostics.Entry per LSP diagnostic, keyed back to its
// source path. Used by the alt+p overlay to render every problem at once.
// Returns an unsorted slice; the diagnostics.Pane sorts on WithEntries.
func (m model) collectDiagnosticEntries() []diagnostics.Entry {
	var out []diagnostics.Entry
	for path, items := range m.diagnostics {
		for _, d := range items {
			out = append(out, diagnostics.Entry{
				Path:     path,
				Row:      int(d.Range.Start.Line),
				Col:      int(d.Range.Start.Character),
				Severity: diagnostics.Severity(d.Severity),
				Source:   d.Source,
				Message:  strings.ReplaceAll(d.Message, "\n", " "),
			})
		}
	}
	return out
}

// diagCounts summarizes the active buffer's diagnostics for the status bar.
func (m model) diagCounts() (errs, warns int) {
	path := m.activePath()
	if path == "" {
		return 0, 0
	}
	for _, d := range m.diagnostics[path] {
		switch d.Severity {
		case protocol.DiagnosticSeverityError:
			errs++
		case protocol.DiagnosticSeverityWarning:
			warns++
		}
	}
	return
}

func renderDiff(t theme.Theme, title, body string, w, h int) string {
	header := lipgloss.NewStyle().Foreground(t.TextMuted).Bold(true).Render("diff: " + title)
	addStyle := lipgloss.NewStyle().Foreground(t.Success)
	rmStyle := lipgloss.NewStyle().Foreground(t.Error)
	hunkStyle := lipgloss.NewStyle().Foreground(t.Info).Bold(true)

	lines := strings.Split(body, "\n")
	if len(lines) > h-1 {
		lines = lines[:h-1]
	}
	for i, ln := range lines {
		switch {
		case strings.HasPrefix(ln, "+"):
			lines[i] = addStyle.Render(ln)
		case strings.HasPrefix(ln, "-"):
			lines[i] = rmStyle.Render(ln)
		case strings.HasPrefix(ln, "@@"):
			lines[i] = hunkStyle.Render(ln)
		}
	}
	return header + "\n" + strings.Join(lines, "\n")
}
