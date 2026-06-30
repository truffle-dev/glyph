// Command nook is a terminal-native AI IDE built from glyph components.
//
// Usage:
//
//	nook                      # open the current working directory
//	nook some/dir             # open a project rooted at some/dir
//	nook some/file.go         # open one file; root is its parent
//	nook ~/.zshrc             # works for files outside any project
//	nook newfile.txt          # vim-style: opens an empty buffer; save
//	                          # writes the new file (mkdir -p first)
//	nook a.go b.go c.go       # open many files; first one is active,
//	                          # alt+] / alt+[ switch between buffers
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
//	ctrl+t     workspace symbol search (functions, types, vars across project)
//	ctrl+\     file outline (document symbols in current file)
//	alt+i      LSP hover info for symbol under cursor
//	alt+j      expand snippet at cursor (Tab cycles tabstops, Esc exits)
//	alt+y      toggle gopls inlay hints (type annotations, parameter names)
//	alt+b      toggle inline git blame on cursor row (GitLens-style)
//	alt+z      toggle soft wrap (wrap long lines onto multiple visual rows)
//	ctrl+]     LSP go to definition
//	ctrl+space LSP completion popup (↑/↓ to navigate, enter to accept)
//	alt+enter  LSP code actions at the cursor
//	f2         LSP rename symbol under cursor
//	f9         toggle breakpoint at cursor row
//	f5         launch debugger / continue when paused (alt+f5 terminates)
//	f6         pause a running debug session
//	f10/f11    step over / step in (alt+f11 steps out)
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
	"github.com/truffle-dev/glyph/cmd/nook/internal/airules"
	"github.com/truffle-dev/glyph/cmd/nook/internal/breakpoints"
	"github.com/truffle-dev/glyph/cmd/nook/internal/bufman"
	"github.com/truffle-dev/glyph/cmd/nook/internal/callhierarchy"
	"github.com/truffle-dev/glyph/cmd/nook/internal/codeaction"
	"github.com/truffle-dev/glyph/cmd/nook/internal/complete"
	"github.com/truffle-dev/glyph/cmd/nook/internal/completedoc"
	"github.com/truffle-dev/glyph/cmd/nook/internal/composer"
	"github.com/truffle-dev/glyph/cmd/nook/internal/config"
	"github.com/truffle-dev/glyph/cmd/nook/internal/configwatch"
	"github.com/truffle-dev/glyph/cmd/nook/internal/createprompt"
	"github.com/truffle-dev/glyph/cmd/nook/internal/dap"
	"github.com/truffle-dev/glyph/cmd/nook/internal/diagnostics"
	"github.com/truffle-dev/glyph/cmd/nook/internal/edit"
	"github.com/truffle-dev/glyph/cmd/nook/internal/editor"
	"github.com/truffle-dev/glyph/cmd/nook/internal/filetree"
	"github.com/truffle-dev/glyph/cmd/nook/internal/filetreeops"
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
	"github.com/truffle-dev/glyph/cmd/nook/internal/navhistory"
	"github.com/truffle-dev/glyph/cmd/nook/internal/outline"
	"github.com/truffle-dev/glyph/cmd/nook/internal/picker"
	"github.com/truffle-dev/glyph/cmd/nook/internal/rename"
	"github.com/truffle-dev/glyph/cmd/nook/internal/search"
	"github.com/truffle-dev/glyph/cmd/nook/internal/settings"
	"github.com/truffle-dev/glyph/cmd/nook/internal/signature"
	"github.com/truffle-dev/glyph/cmd/nook/internal/snippets"
	"github.com/truffle-dev/glyph/cmd/nook/internal/splitlayout"
	"github.com/truffle-dev/glyph/cmd/nook/internal/symbolsearch"
	"github.com/truffle-dev/glyph/cmd/nook/internal/tabbar"
	"github.com/truffle-dev/glyph/cmd/nook/internal/tasks"
	"github.com/truffle-dev/glyph/cmd/nook/internal/term"
	"github.com/truffle-dev/glyph/cmd/nook/internal/themepicker"
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
	overlaySymbolSearch
	overlayOutline
	overlayCreate
	overlaySettings
	overlayThemePicker
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

	// helpQuery is the live search string typed into the help overlay
	// (overlayHelp). Empty means the full keymap is shown; non-empty filters
	// the card to matching bindings. Reset to "" each time help opens.
	helpQuery string

	bufs *bufman.Manager

	// split is the view-split layout tree. Until a split gesture lands it
	// holds a single pane covering the whole editor region, so every size
	// and render path below behaves exactly as it did before splits
	// existed. paneBuf maps each pane to the bufman buffer index it shows;
	// the focused pane's buffer is kept as the active buffer, so the ~60
	// existing m.bufs.Active() editing paths operate on the focused pane
	// with no per-pane bookkeeping.
	split   *splitlayout.Tree
	paneBuf map[splitlayout.PaneID]int

	// awaitingWindowKey is the one-shot "window command" leader state. alt+w
	// arms it; the next key runs a window op (v/s split, c close pane) or
	// disarms. alt+w is used rather than the vim/tmux ctrl+w so the historical
	// ctrl+w = close-tab binding stays intact.
	awaitingWindowKey bool

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

	// rulesSource names which file the AI conventions came from
	// (".nookrules" / ".cursorrules" / none). The chip in the status
	// bar is derived from this. rulesText is the trimmed contents
	// folded into every AI wedge's system prompt.
	rulesSource airules.Source
	rulesText   string

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

	// LSP completion-doc side panel. Auto-fires completionItem/resolve as
	// the user navigates the popup so the highlighted item's documentation
	// renders beside the menu. docReqLabel pins the most recent request so
	// a late response for an item the user has scrolled past gets dropped.
	docPane     completedoc.Pane
	docReqLabel string

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

	// File-tree "new file or directory" prompt: armed by
	// filetree.CreatePromptMsg, displayed under overlayCreate. The prompt
	// carries the parent directory the new entry should land under; the
	// Enter handler runs filetreeops.CreatePath and on success refreshes
	// the tree and opens the file (if a file was created).
	createPrompt    createprompt.Prompt
	createParentDir string

	// Workspace symbol search modal (Ctrl+T). The prompt collects a query
	// string; on Enter the host fires lsp.WorkspaceSymbol and routes the
	// results through the multibuffer overlay as fragments. lastSymQuery
	// is the previous query so Ctrl+T re-opens with the last value (Zed
	// muscle memory).
	symbolPrompt symbolsearch.Prompt
	lastSymQuery string

	// File outline modal (Ctrl+\). outlinePane is the modal itself.
	// outlineCache holds previously-loaded symbol trees keyed by absolute
	// path so reopens are instant and the user's recent jumps don't
	// re-roundtrip the LSP. The cache is invalidated when the host saves
	// the file (didChange notifications would also be a fine trigger but
	// save is the simpler hook). outlineLoading prevents reopen-on-reopen
	// races: while a request is in flight, a second Ctrl+\ is a no-op.
	outlinePane    outline.Pane
	outlineCache   map[string][]nooklsp.DocSymbol
	outlineLoading map[string]bool

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

	// softWrapOn drives whether long logical lines wrap onto multiple
	// visual rows instead of horizontally scrolling. Alt+z toggles.
	// Default false (matches v0.44 behavior). Propagated to every open
	// pane through bufman.WithSoftWrap on toggle and on config reload.
	softWrapOn bool

	// cfgPath is the resolved ~/.config/nook/config.toml location. Stored
	// on the model so alt+, can re-read the same file without re-resolving
	// XDG_CONFIG_HOME each time. Empty when the host couldn't determine a
	// path (e.g. no home dir) — the reload key surfaces an error in that
	// case rather than crashing.
	cfgPath string
	// cfgFinger is the most recent fingerprint of cfgPath, used by the
	// configwatch poller to detect a save. Re-armed via WatchCmd on every
	// TickMsg so the loop runs without a long-lived goroutine.
	cfgFinger configwatch.Fingerprint
	// prjCfgPath is the per-project config path: <root>/.nook/config.toml
	// (v0.42.0). Loaded after the user config so project values win on
	// any field the project file explicitly sets. The host watches both
	// paths independently via configwatch.WatchCmd so editing either
	// file applies live.
	prjCfgPath string
	// prjCfgFinger is the most recent fingerprint of prjCfgPath. Tracked
	// alongside cfgFinger so each tick re-arms with the right finger.
	prjCfgFinger configwatch.Fingerprint
	// cfg is the effective merged configuration the host last applied
	// (user global overlaid with the per-project file). Kept on the model
	// so the read-only settings overlay (alt+.) can render exactly the
	// values in force without re-reading or re-merging. Refreshed on every
	// reloadConfig.
	cfg config.Config
	// cfgUserExists / cfgProjectExists record whether a file was actually
	// present at each scope when cfg was assembled. The settings overlay
	// shows them so a user can tell which scopes are live.
	cfgUserExists    bool
	cfgProjectExists bool
	// themeName is the name of the currently-applied theme. Live-reload
	// detects a change against this stored name and propagates SetTheme to
	// every pane in m. v0.38.0 dropped the "restart to apply" hint that
	// preceded live-reskin support.
	themeName string
	// themePicker is the live theme switcher overlay (alt+T). themePickerOrig
	// remembers the theme active when it opened, so esc restores it after the
	// cursor-move live previews. Session-only: the disk config is untouched.
	themePicker     themepicker.Picker
	themePickerOrig string
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

	// LSP signature help (parameter hints). Auto-fires on '(' inside a
	// buffer with an attached language server; auto-closes on ')' or Esc.
	// Unlike outline/symbolsearch this is a hint overlay — it never
	// captures input — so it lives off the overlay enum and renders only
	// when sigPane.IsOpen(). sigReqPath/Row/Col echo the request inputs
	// so a late response after the cursor moved gets discarded.
	sigPane    signature.Pane
	sigReqPath string
	sigReqRow  int
	sigReqCol  int

	// startupFiles are absolute paths opened from the CLI (`nook file …`).
	// Pre-opened in newModel via bufman.OpenOrSwitch so the first frame
	// shows the file rather than the welcome card. Init() dispatches LSP /
	// gutter / inlay / blame for the active one so a single-file launch
	// behaves like opening from the picker.
	startupFiles []string

	// DAP debug session state. bpStore holds per-file breakpoints across
	// sessions so toggling F9 stays sticky even when no debugger is
	// attached. dapClient is non-nil only while a session is live;
	// dapState drives the status-bar label ("" / "launching" / "running"
	// / "paused" / "terminated"). When paused, dapPausedPath+Row pin the
	// stop marker so we can clear it cleanly on continue / terminate.
	bpStore       *breakpoints.Store
	dapClient     *dap.Client
	dapState      string
	dapThreadID   int
	dapPausedPath string
	dapPausedRow  int

	// navHistory is the vim-style jump list. Each cross-file or
	// significant in-file jump records the cursor's from-position before
	// the jump fires. Alt+- walks back through prior positions; Alt+=
	// walks forward through entries previously revisited via Back. The
	// list never grows beyond navhistory.DefaultCap (100) entries.
	navHistory *navhistory.History

	// dochiGen is the cursor-settle generation counter for the LSP
	// document-highlight overlay. Every key event routed to the editor
	// bumps it and schedules a dochiSettleMsg via tea.Tick; only the
	// tick whose gen matches dochiGen at fire time issues the actual
	// textDocument/documentHighlight request. This is the standard
	// debounce shape used by the ghost manager — a counter is cheaper
	// than canceling in-flight Cmds and reads cleanly under test.
	dochiGen int
}

// pendingRename holds the cursor position the rename flow was launched from.
// Filled by openRenamePrompt and consumed when the user accepts the prompt.
type pendingRename struct {
	path string
	row  int
	col  int
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "nook:", err)
		os.Exit(1)
	}
	s, err := parseStartup(cwd, os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "nook:", err)
		os.Exit(1)
	}

	m := newModel(s.root, s.files...)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "nook:", err)
		os.Exit(1)
	}
}

// startup captures the workspace root and any files to pre-open from the
// CLI. Used by parseStartup so main() can stay one or two lines.
type startup struct {
	root  string
	files []string
}

// parseStartup turns CLI args into a (root, files) pair. Supported shapes:
//   - no args                  → root = cwd
//   - one dir arg              → root = dir
//   - one file arg             → root = file's parent, files = [file]
//   - one missing-path arg     → root = parent (must exist), files = [path]
//     (vim-style: creates a new buffer; Save will mkdir + write)
//   - N file/missing-path args → root = first arg's parent, files = all
//
// Trailing-slash paths are treated as directory references and rejected when
// they don't exist. Mixing a directory with other args is rejected.
func parseStartup(cwd string, args []string) (startup, error) {
	if len(args) == 0 {
		return startup{root: cwd}, nil
	}

	abs := make([]string, len(args))
	for i, a := range args {
		p, err := filepath.Abs(a)
		if err != nil {
			return startup{}, fmt.Errorf("%s: %w", a, err)
		}
		abs[i] = p
	}

	classify := func(i int) (info os.FileInfo, exists, dirRef bool, err error) {
		fi, statErr := os.Stat(abs[i])
		if statErr == nil {
			return fi, true, fi.IsDir(), nil
		}
		if !errors.Is(statErr, fs.ErrNotExist) {
			return nil, false, false, statErr
		}
		// Doesn't exist. A trailing slash means the user intended a
		// directory; refuse instead of silently treating it as a file.
		trail := strings.HasSuffix(args[i], "/") || strings.HasSuffix(args[i], string(filepath.Separator))
		return nil, false, trail, nil
	}

	if len(args) == 1 {
		_, exists, dirRef, err := classify(0)
		if err != nil {
			return startup{}, fmt.Errorf("%s: %w", args[0], err)
		}
		switch {
		case exists && dirRef:
			return startup{root: abs[0]}, nil
		case exists && !dirRef:
			return startup{root: filepath.Dir(abs[0]), files: []string{abs[0]}}, nil
		case !exists && dirRef:
			return startup{}, fmt.Errorf("%s: no such directory", args[0])
		}
		// Doesn't exist, no trailing slash → vim-style new file.
		dir := filepath.Dir(abs[0])
		if _, derr := os.Stat(dir); derr != nil {
			return startup{}, fmt.Errorf("%s: %w", args[0], derr)
		}
		return startup{root: dir, files: []string{abs[0]}}, nil
	}

	// Multi-arg: every arg must be a file (existing or new). A directory
	// argument is ambiguous here (which root wins?) so we reject.
	for i := range args {
		_, exists, dirRef, err := classify(i)
		if err != nil {
			return startup{}, fmt.Errorf("%s: %w", args[i], err)
		}
		if exists && dirRef {
			return startup{}, fmt.Errorf("%s: directory not allowed in multi-file mode", args[i])
		}
		if !exists && dirRef {
			return startup{}, fmt.Errorf("%s: no such directory", args[i])
		}
	}
	root := filepath.Dir(abs[0])
	if _, err := os.Stat(root); err != nil {
		root = cwd
	}
	return startup{root: root, files: abs}, nil
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

func newModel(root string, opens ...string) model {
	cfgPath, _ := config.Path()
	prjCfgPath := config.ProjectPath(root)
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
	// Layer the per-project config on top of the user config. Project
	// values for fields it explicitly sets win; absent fields fall through
	// to the user choice. A missing project file is silent (.nook/
	// config.toml is opt-in); a parse error surfaces the same way a user
	// parse error does. v0.42.0.
	prjLoadErr := error(nil)
	prjExists := false
	if pc, md, err := config.LoadRaw(prjCfgPath); err == nil {
		cfg = config.Merge(cfg, pc, md)
		prjExists = true
	} else if !errors.Is(err, config.ErrNotFound) {
		prjLoadErr = err
	}
	userExists := cfgPath != "" && !loadMissing && loadErr == nil

	t, themeOK := resolveTheme(cfg.Editor.Theme)
	aiClient, _ := ai.NewClient() // tolerated nil; AI panes surface their own error
	aiHistory := aihistory.NewStore()
	rulesSrc, rulesText, rulesErr := airules.Load(root)
	ghostMgr := ghost.NewManager(aiClient)
	ghostMgr.SetRules(rulesText)
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
	case prjLoadErr != nil:
		status = "project config: " + prjLoadErr.Error() + " (using user settings)"
	case rulesErr != nil:
		status = "rules: " + rulesErr.Error() + " (ai wedges unaffected)"
	case !themeOK && !loadMissing:
		status = "theme " + cfg.Editor.Theme + " not found; using default"
	}

	m := model{
		theme:            t,
		root:             root,
		width:            80,
		height:           24,
		bufs:             bufman.New(t).WithHighlighter(highlight.New()).WithTabWidth(cfg.Editor.TabWidth).WithLineNumbers(cfg.Editor.LineNumbers).WithIndentGuides(cfg.Editor.IndentGuides).WithSoftWrap(cfg.Editor.SoftWrap),
		gitPane:          git.NewPane(t, root),
		termPane:         term.NewPane(t, root),
		picker:           picker.New(t).WithTitle("Open file").WithPlaceholder("type to filter…"),
		search:           search.NewPane(t, root),
		editPane:         edit.NewPane(t, aiClient).WithRules(rulesText),
		composer:         composer.NewPane(t, aiClient).WithHistory(aiHistory).WithRules(rulesText),
		ghost:            ghostMgr,
		right:            rightNone,
		status:           status,
		aiClient:         aiClient,
		aiHistory:        aiHistory,
		rulesSource:      rulesSrc,
		rulesText:        rulesText,
		lspVersions:      map[string]int32{},
		diagnostics:      map[string][]protocol.Diagnostic{},
		finder:           finder.New(t),
		formatOnSave:     cfg.Editor.FormatOnSave,
		treePane:         filetree.New(t, root),
		showTree:         false,
		caPopup:          codeaction.New(),
		renamePrompt:     rename.New(),
		createPrompt:     createprompt.New(),
		symbolPrompt:     symbolsearch.New(),
		outlinePane:      outline.New(t),
		outlineCache:     map[string][]nooklsp.DocSymbol{},
		outlineLoading:   map[string]bool{},
		multibufPane:     multibuffer.NewPane(t, root),
		diagPane:         diagnostics.NewPane(t, root),
		mdPane:           mdpreview.NewPane(t),
		inlayHintsOn:     cfg.Editor.InlayHints,
		softWrapOn:       cfg.Editor.SoftWrap,
		cfgPath:          cfgPath,
		cfgFinger:        configwatch.Snapshot(cfgPath),
		prjCfgPath:       prjCfgPath,
		prjCfgFinger:     configwatch.Snapshot(prjCfgPath),
		cfg:              cfg,
		cfgUserExists:    userExists,
		cfgProjectExists: prjExists,
		themeName:        cfg.Editor.Theme,
		tabWidth:         cfg.Editor.TabWidth,
		snipLib:          snipLib,
		tasksPane:        tasks.NewPane(t, root),
		sigPane:          signature.New(),
		docPane:          completedoc.New(),
		bpStore:          breakpoints.New(),
		navHistory:       navhistory.New(0),
	}

	// Start with a single pane bound to buffer 0. splitlayout.New is
	// constant-time and does no I/O, so this stays inside the first-paint
	// rule. The pane stays single until a split gesture lands.
	sp, pid := splitlayout.New()
	m.split = sp
	m.paneBuf = map[splitlayout.PaneID]int{pid: 0}

	// Pre-open any files the user passed on the CLI. After OpenOrSwitch
	// the last one is active; switch back to index 0 so a multi-file
	// launch (`nook a b c`) lands on the first file the way vim does.
	if len(opens) > 0 {
		for _, abs := range opens {
			m.bufs.OpenOrSwitch(abs)
		}
		if len(opens) > 1 {
			m.bufs.Switch(0)
		}
		m.startupFiles = append(m.startupFiles, opens...)
		if len(opens) == 1 {
			if rel, err := filepath.Rel(root, opens[0]); err == nil && !strings.HasPrefix(rel, "..") {
				m.status = "opened " + rel
			} else {
				m.status = "opened " + filepath.Base(opens[0])
			}
		} else {
			m.status = fmt.Sprintf("opened %d files (alt+] / alt+[ to switch)", len(opens))
		}
		if m.showTree {
			m.treePane.Reveal(opens[0])
		}
	}
	return m
}

// reloadConfig re-reads m.cfgPath and m.prjCfgPath, merges them (project
// values win on any field the project file explicitly sets), and applies
// the runtime-mutable knobs. Editor toggles (format-on-save, inlay hints,
// tab width, line numbers) take effect immediately, and a theme change
// propagates live to every theme-holding pane via SetTheme so the user
// sees the new palette on the next render without a restart. Used by both
// the manual `alt+,` keybind and the configwatch tick loop. Returns the
// updated model.
func (m model) reloadConfig() model {
	if m.cfgPath == "" {
		m.status = "config: no path resolved"
		return m
	}
	cfg, userExists, projectExists, scopeErr := loadMergedConfig(m.cfgPath, m.prjCfgPath)
	if scopeErr != nil {
		m.status = scopeErr.Error() + " (kept current settings)"
		return m
	}
	prevTheme := m.themeName
	m.cfg = cfg
	m.cfgUserExists = userExists
	m.cfgProjectExists = projectExists
	m.formatOnSave = cfg.Editor.FormatOnSave
	m.inlayHintsOn = cfg.Editor.InlayHints
	m.tabWidth = cfg.Editor.TabWidth
	m.themeName = cfg.Editor.Theme
	m.bufs.WithTabWidth(cfg.Editor.TabWidth).WithLineNumbers(cfg.Editor.LineNumbers).WithIndentGuides(cfg.Editor.IndentGuides).WithSoftWrap(cfg.Editor.SoftWrap)
	m.softWrapOn = cfg.Editor.SoftWrap
	if !m.inlayHintsOn {
		m = m.clearInlayHints()
	}

	themeChanged := prevTheme != cfg.Editor.Theme
	themeFound := true
	if themeChanged {
		t, ok := resolveTheme(cfg.Editor.Theme)
		themeFound = ok
		m = m.applyTheme(t)
	}

	scope := scopeHint(userExists, projectExists)
	switch {
	case !userExists && !projectExists:
		m.status = "config: no file at " + m.cfgPath + " (using defaults)"
	case themeChanged && !themeFound:
		m.status = "settings reloaded — theme " + cfg.Editor.Theme + " not found; using default" + scope
	case themeChanged:
		m.status = "theme switched to " + cfg.Editor.Theme + scope
	default:
		m.status = "settings reloaded" + scope
	}
	return m
}

// loadMergedConfig reads the user config (with Default() seeding + safety
// reapply), then overlays a per-project config if one exists. Returns the
// merged Config plus presence flags for each scope so the caller can
// compose a precise status message. A parse error in either file returns
// the prefixed error so the caller surfaces "config:" vs "project config:"
// distinguishably. ErrNotFound in either path is silent: the user config
// falling back to Default() and the project layer being a no-op are both
// expected shapes.
func loadMergedConfig(userPath, projectPath string) (cfg config.Config, userExists, projectExists bool, err error) {
	cfg = config.Default()
	if userPath != "" {
		c, e := config.Load(userPath)
		switch {
		case e == nil:
			cfg = c
			userExists = true
		case errors.Is(e, config.ErrNotFound):
			// silent: stay on Default()
		default:
			return config.Default(), false, false, errors.New("config: " + e.Error())
		}
	}
	if projectPath != "" {
		pc, md, e := config.LoadRaw(projectPath)
		switch {
		case e == nil:
			cfg = config.Merge(cfg, pc, md)
			projectExists = true
		case errors.Is(e, config.ErrNotFound):
			// silent: project file is opt-in
		default:
			return config.Default(), false, false, errors.New("project config: " + e.Error())
		}
	}
	return cfg, userExists, projectExists, nil
}

// scopeHint composes the suffix that disambiguates which config scopes
// are active in a "settings reloaded" message. Returns the empty string
// when neither scope is active; that branch is handled separately by the
// caller so "settings reloaded" never appears without context.
func scopeHint(userExists, projectExists bool) string {
	switch {
	case userExists && projectExists:
		return " (user + project)"
	case userExists:
		return " (user)"
	case projectExists:
		return " (project)"
	default:
		return ""
	}
}

// applyTheme propagates a new palette to every pane on m that holds a
// theme. The host model itself caches the theme too (used directly for
// status row, tab bar, help overlay, welcome card, modals built around
// pane-style overlays). Returns the updated model. v0.38.0 made theme
// changes live-applicable — each new pane added since must take a copy
// of the theme via SetTheme here.
func (m model) applyTheme(t theme.Theme) model {
	m.theme = t
	m.bufs.SetTheme(t)
	m.gitPane = m.gitPane.SetTheme(t)
	m.termPane = m.termPane.SetTheme(t)
	m.picker = m.picker.SetTheme(t)
	m.search = m.search.SetTheme(t)
	m.editPane = m.editPane.SetTheme(t)
	m.composer = m.composer.SetTheme(t)
	m.finder = m.finder.SetTheme(t)
	m.treePane.SetTheme(t)
	m.outlinePane = m.outlinePane.SetTheme(t)
	m.multibufPane = m.multibufPane.SetTheme(t)
	m.diagPane = m.diagPane.SetTheme(t)
	m.mdPane = m.mdPane.SetTheme(t)
	m.tasksPane = m.tasksPane.SetTheme(t)
	return m
}

func (m model) Init() tea.Cmd {
	// filetree.BuildTreeCmd walks the file system in a goroutine; the
	// first paint is no longer gated on the walk (which can take >1s
	// for home-directory launches like `nook ~/.zshrc`). The pane
	// renders a "Scanning…" placeholder until the BuildTreeMsg lands.
	cmds := []tea.Cmd{m.loadFilesCmd(), m.refreshGitCmd(), filetree.BuildTreeCmd(m.root)}
	// configwatch polls cfgPath every Interval and emits configwatch.TickMsg.
	// The host's Update handler re-arms WatchCmd on every tick so the loop
	// runs without a long-lived goroutine. Nil when cfgPath is empty.
	if cmd := configwatch.WatchCmd(m.cfgPath, m.cfgFinger); cmd != nil {
		cmds = append(cmds, cmd)
	}
	// v0.42.0: parallel watch on the per-project config file. Each TickMsg
	// carries Path so the Update handler routes back to the right finger
	// and re-arms with the right path.
	if cmd := configwatch.WatchCmd(m.prjCfgPath, m.prjCfgFinger); cmd != nil {
		cmds = append(cmds, cmd)
	}
	// If files were pre-opened from the CLI, fire the same auxiliary
	// commands the picker/filetree open paths run so a single-file
	// launch (`nook foo.go`) gets LSP attach, gutter, inlay hints, and
	// blame on the first frame.
	if p := m.bufs.Active(); p != nil && p.Path() != "" {
		cmds = append(cmds, m.ensureLSPForFile(p.Path()))
		cmds = append(cmds, m.refreshGutterCmd())
		cmds = append(cmds, m.refreshInlayHintsCmd())
		cmds = append(cmds, m.refreshBlameCmd())
	}
	return tea.Batch(cmds...)
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

// refreshSemanticTokensCmd asks gopls for the current semanticTokens map of
// the active buffer's file and ships the result through a SemanticTokensMsg
// the editor overlays on top of its chroma highlight. Returns nil when no
// LSP is wired up, no active buffer, or no path; the wedge degrades to
// chroma-only when semantic tokens aren't available.
func (m model) refreshSemanticTokensCmd() tea.Cmd {
	if m.lsp == nil {
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
	return lookup.SemanticTokensCmd(m.lsp, path, p.BufVer())
}

// applySemanticTokens routes a semantic-tokens response into the pane whose
// path matches. The editor itself drops the overlay when the buffer has
// advanced past msg.PaneVer, so the host doesn't need a per-pane version
// table for this — it just forwards the response. Errors are swallowed
// (chroma-only fallback is invisible).
func (m model) applySemanticTokens(msg lookup.SemanticTokensMsg) model {
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
	*p = p.WithSemanticTokens(msg.Tokens, msg.PaneVer)
	return m
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

// dochiSettleDelay is the cursor-settle debounce window for LSP
// textDocument/documentHighlight. ~250ms is short enough to feel
// instant on a quick "settle to read" but long enough that a held
// arrow-key sweep across a file doesn't issue dozens of requests.
const dochiSettleDelay = 250 * time.Millisecond

// dochiSettleMsg fires after dochiSettleDelay of quiet. gen is the
// dochiGen value at the time the tick was scheduled; the handler
// drops the message when newer activity has bumped dochiGen further,
// so only the most recent settle survives to issue the LSP request.
type dochiSettleMsg struct {
	gen int
}

// scheduleDochiSettle bumps the cursor-settle generation and returns
// a tea.Tick that will re-enter Update with a dochiSettleMsg after
// the debounce window. Called after every key event that could move
// the cursor or mutate the buffer (which invalidates the previous
// highlight set either way).
func (m *model) scheduleDochiSettle() tea.Cmd {
	m.dochiGen++
	gen := m.dochiGen
	return tea.Tick(dochiSettleDelay, func(time.Time) tea.Msg {
		return dochiSettleMsg{gen: gen}
	})
}

// refreshDocumentHighlightsCmd issues a textDocument/documentHighlight
// request for the active buffer's current cursor. Returns nil when no
// LSP is running, no active path, or no buffer. The paneVer is pinned
// at request time so the editor can drop a stale response if the user
// typed in between.
func (m model) refreshDocumentHighlightsCmd() tea.Cmd {
	if m.lsp == nil {
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
	return lookup.DocumentHighlightCmd(m.lsp, path, p.CursorRow(), p.CursorCol(), p.BufVer())
}

// applyDocumentHighlights routes a documentHighlight response to the
// pane whose path matches and hands the response to the editor's
// staleness-checked SetDocumentHighlights accessor. Errors are
// swallowed silently — when the server can't resolve anything (cursor
// over whitespace, file not yet indexed) the overlay simply stays empty.
func (m model) applyDocumentHighlights(msg lookup.DocumentHighlightMsg) model {
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
	*p = p.SetDocumentHighlights(msg.Highlights, msg.PaneVer)
	return m
}

// clearAllDocumentHighlights drops the overlay from every open pane.
// Called when the LSP detaches or when the active buffer changes path
// — the prior pane's highlights are stale relative to the new view.
func (m model) clearAllDocumentHighlights() model {
	for i := 0; i < m.bufs.Count(); i++ {
		p := m.bufs.At(i)
		if p == nil {
			continue
		}
		*p = p.ClearDocumentHighlights()
	}
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
			// Invalidate the outline cache so a stale symbol tree from
			// before the edit doesn't surface on the next Ctrl+\ press.
			delete(m.outlineCache, msg.Path)
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

	case lookup.DocumentHighlightMsg:
		m = m.applyDocumentHighlights(msg)
		return m, nil

	case dochiSettleMsg:
		// Drop stale settles. Only the most recent generation issues
		// the actual textDocument/documentHighlight request.
		if msg.gen != m.dochiGen {
			return m, nil
		}
		if cmd := m.refreshDocumentHighlightsCmd(); cmd != nil {
			return m, cmd
		}
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
		m.pushNavCurrent()
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

	case search.ApplyMsg:
		// Project-wide replace. ApplyAll rewrites every recorded match
		// span on disk; we then reload any of those paths that happen
		// to be open in a buffer so the user sees the new bytes
		// immediately. Errors surface through the status hint without
		// aborting the rest of the pass (ApplyAll already returns a
		// partial Result on read/write failure).
		res, err := search.ApplyAll(m.search.Matches(), msg.Replacement)
		for _, p := range res.PathsTouched {
			m.bufs.RefreshIfOpen(p)
		}
		if err != nil {
			m.status = "replace error: " + err.Error()
		} else {
			m.status = fmt.Sprintf(
				"replaced %d occurrence(s) in %d file(s)",
				res.ReplacementsApplied, res.FilesChanged,
			)
		}
		m.search = m.search.ExitReplace()
		m.overlay = overlayNone
		if m.searchCancel != nil {
			m.searchCancel()
			m.searchCancel = nil
		}
		m = m.applyDiagnosticsToActive()
		return m, m.refreshGutterCmd()

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
			return m, nil
		}
		// On a successful open, request the first semantic-token batch so the
		// pane gets colored ranges as soon as gopls finishes parsing.
		return m, m.refreshSemanticTokensCmd()

	case lookup.SemanticTokensMsg:
		m = m.applySemanticTokens(msg)
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

	case lookup.SignatureHelpMsg:
		// Discard stale: only honor a response that matches the pin
		// captured when we fired the request. Avoids painting an
		// overlay for a paren the user has already closed.
		if msg.Path != m.sigReqPath || msg.Row != m.sigReqRow || msg.Col != m.sigReqCol {
			return m, nil
		}
		// Signature help is a hint, not a command surface; swallow
		// errors and empties silently instead of writing to status.
		if msg.Err != nil || len(msg.Info.Signatures) == 0 {
			m.sigPane = m.sigPane.Close()
			return m, nil
		}
		w := m.width - 4
		if w > 64 {
			w = 64
		}
		m.sigPane = m.sigPane.Open(msg.Info).WithSize(w)
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
		m.pushNavCurrent()
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
		return m, m.resolveSelectedCompletionCmd()

	case lookup.ResolveCompletionMsg:
		// Discard if the user has moved past the item this resolve was
		// fired for, or if the completion popup has closed entirely.
		if m.overlay != overlayCompletion || msg.ReqLabel != m.docReqLabel {
			return m, nil
		}
		if msg.Err != nil {
			// Resolve is best-effort: a server that doesn't support it
			// just returns the original item back to us. Don't write to
			// status, the popup is a hot path and noise would be worse
			// than no docs at all.
			return m, nil
		}
		m.docPane = m.docPane.Open(msg.Item)
		return m, nil

	case lookup.FormattingMsg:
		return m.handleFormattingMsg(msg)

	case lookup.CodeActionMsg:
		return m.handleCodeActionMsg(msg)

	case lookup.PrepareRenameMsg:
		return m.handlePrepareRenameMsg(msg)

	case lookup.RenameMsg:
		return m.handleRenameMsg(msg)

	case outlineRequestMsg:
		return m.handleOutlineRequestMsg(msg)

	case filetree.BuildTreeMsg:
		// Discard stale walks: if the project root changed between
		// kick-off and arrival (re-root happens via toggleTree refresh
		// or future re-root paths), drop the result and let the new
		// walk land.
		if msg.Root != m.root {
			return m, nil
		}
		m.treePane.SetNode(msg.Node)
		return m, nil

	case configwatch.TickMsg:
		// Two paths are watched in v0.42.0: the user config and the
		// per-project config. Each TickMsg carries Path, so we route to
		// the matching finger and re-arm with the same path. Either
		// file changing triggers a full reload, which merges both files
		// every time so the result reflects the union of both scopes.
		switch msg.Path {
		case m.cfgPath:
			if msg.Changed() {
				m = m.reloadConfig()
			}
			m.cfgFinger = msg.Cur
			return m, configwatch.WatchCmd(m.cfgPath, m.cfgFinger)
		case m.prjCfgPath:
			if msg.Changed() {
				m = m.reloadConfig()
			}
			m.prjCfgFinger = msg.Cur
			return m, configwatch.WatchCmd(m.prjCfgPath, m.prjCfgFinger)
		default:
			// Stale tick from a path the host no longer watches; drop.
			return m, nil
		}

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

	case filetree.CreatePromptMsg:
		return m.openCreatePrompt(msg.ParentDir), nil

	case createPathMsg:
		return m.handleCreatePathMsg(msg)

	case multibuffer.FragmentsMsg:
		m.multibufPane = m.multibufPane.SetFragments(msg.Fragments, msg.Err)
		return m, nil

	case multibuffer.OpenAtMsg:
		m.pushNavCurrent()
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
		m.pushNavCurrent()
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

	case dapStartedMsg:
		if msg.err != nil {
			m.dapState = ""
			m.status = "debug: " + msg.err.Error()
			return m, nil
		}
		m.dapClient = msg.client
		m.dapState = "running"
		m.status = "debug: running " + filepath.Base(msg.program)
		return m, m.dapPumpCmd(m.dapClient.Events())

	case dapPumpMsg:
		next := m.dapPumpCmd(msg.ch)
		switch msg.event.Kind {
		case "stopped":
			body := msg.event.Stopped
			if body == nil {
				return m, next
			}
			m.dapState = "paused"
			m.dapThreadID = body.ThreadID
			reason := body.Reason
			if reason == "" {
				reason = "stopped"
			}
			m.status = "debug: " + reason
			return m, tea.Batch(next, m.dapStackTraceCmd(body.ThreadID))
		case "continued":
			m.dapState = "running"
			m.dapPausedPath = ""
			m.dapPausedRow = 0
			m = m.clearAllStopMarkers()
			m.status = "debug: running"
			return m, next
		case "output":
			if msg.event.Output != nil {
				txt := strings.TrimRight(msg.event.Output.Output, "\r\n")
				if txt != "" {
					m.status = "debug> " + truncateForStatus(txt)
				}
			}
			return m, next
		case "terminated", "exited":
			m.dapState = "terminated"
			if msg.event.Exited != nil {
				m.status = fmt.Sprintf("debug: exited (%d)", msg.event.Exited.ExitCode)
			} else {
				m.status = "debug: terminated"
			}
			if m.dapClient != nil {
				_ = m.dapClient.Shutdown()
				m.dapClient = nil
			}
			m = m.clearAllStopMarkers()
			return m, nil
		}
		return m, next

	case dapStackFrameMsg:
		if msg.err != nil || len(msg.frames) == 0 {
			return m, nil
		}
		top := msg.frames[0]
		if top.Source.Path == "" || top.Line <= 0 {
			return m, nil
		}
		m = m.clearAllStopMarkers()
		m.dapPausedPath = top.Source.Path
		m.dapPausedRow = top.Line - 1
		m = m.applyStopMarker(top.Source.Path, top.Line-1)
		m = m.applyAllBreakpointMarkers()
		if m.showTree {
			m.treePane.Reveal(top.Source.Path)
		}
		m = m.applyDiagnosticsToActive()
		rel := top.Source.Path
		if r, err := filepath.Rel(m.root, top.Source.Path); err == nil && !strings.HasPrefix(r, "..") {
			rel = r
		}
		m.status = fmt.Sprintf("debug: paused at %s:%d", rel, top.Line)
		return m, tea.Batch(m.ensureLSPForFile(top.Source.Path), m.refreshGutterCmd(), m.refreshInlayHintsCmd(), m.refreshBlameCmd())

	case dapBreakpointsResultMsg:
		if msg.err != nil {
			m.status = "debug bp " + filepath.Base(msg.path) + ": " + msg.err.Error()
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

// truncateForStatus clamps a single-line debug message to keep the status
// bar single-line. Long stack traces or panic dumps would otherwise wrap
// and break the layout.
func truncateForStatus(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	const max = 80
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
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
		// esc is two-stage: clear an active search first, then close. This
		// matches the find bars, so a user who typed a query can back out of
		// it without losing the overlay.
		if km.Type == tea.KeyEsc {
			if m.helpQuery != "" {
				m.helpQuery = ""
				return m, nil
			}
			m.overlay = overlayNone
			return m, nil
		}
		// ? always closes, even mid-search. It is the toggle key and not a
		// useful search character, so it stays the fast way out.
		if km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == '?' {
			m.overlay = overlayNone
			m.helpQuery = ""
			return m, nil
		}
		// Backspace edits the query.
		if km.Type == tea.KeyBackspace {
			if r := []rune(m.helpQuery); len(r) > 0 {
				m.helpQuery = string(r[:len(r)-1])
			}
			return m, nil
		}
		// Space and printable runes build the query; everything else
		// (arrows, ctrl-combos) is swallowed so it can't act on the
		// workspace underneath the card.
		if km.Type == tea.KeySpace {
			m.helpQuery += " "
			return m, nil
		}
		if km.Type == tea.KeyRunes && !km.Alt {
			m.helpQuery += string(km.Runes)
			return m, nil
		}
		return m, nil
	}

	// Settings overlay swallows its own keys the same way help does: esc or
	// a second alt+. dismiss, everything else is ignored so a stray keystroke
	// doesn't act on the editor underneath the read-only card.
	if m.overlay == overlaySettings {
		if km.Type == tea.KeyEsc {
			m.overlay = overlayNone
			return m, nil
		}
		if km.Alt && km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == '.' {
			m.overlay = overlayNone
			return m, nil
		}
		return m, nil
	}

	// Theme picker swallows navigation: ↑/k and ↓/j move the highlight and
	// preview that theme live across every pane; enter keeps it for the
	// session; esc (or a second alt+T) restores the theme that was active
	// when the picker opened. The choice never touches config.toml on disk.
	if m.overlay == overlayThemePicker {
		preview := func() (model, tea.Cmd) {
			if t, ok := resolveTheme(m.themePicker.Selected()); ok {
				m = m.applyTheme(t)
			}
			return m, nil
		}
		switch {
		case km.Type == tea.KeyUp || (km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == 'k'):
			m.themePicker = m.themePicker.Up()
			return preview()
		case km.Type == tea.KeyDown || (km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == 'j'):
			m.themePicker = m.themePicker.Down()
			return preview()
		case km.Type == tea.KeyEnter:
			m.themeName = m.themePicker.Selected()
			m.overlay = overlayNone
			m.status = fmt.Sprintf("theme: %s (session only)", m.themeName)
			return m, nil
		case km.Type == tea.KeyEsc, km.Alt && km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == 'T':
			if t, ok := resolveTheme(m.themePickerOrig); ok {
				m = m.applyTheme(t)
			}
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
			return m, m.resolveSelectedCompletionCmd()
		case tea.KeyDown:
			m.completePopup = m.completePopup.MoveDown()
			return m, m.resolveSelectedCompletionCmd()
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

	// File-tree "new file or directory" prompt swallows its own keys:
	// typing edits the path, arrows/home/end move the cursor, Backspace
	// deletes, Enter fires the create command, Esc cancels.
	if m.overlay == overlayCreate {
		return m.routeCreate(km)
	}

	// Symbol-search prompt swallows its own keys: typing edits the
	// query, arrows/home/end move the cursor, Backspace deletes,
	// Enter fires workspace/symbol, Esc cancels.
	if m.overlay == overlaySymbolSearch {
		return m.routeSymbolSearch(km)
	}

	// Outline modal swallows its own keys: typing edits the filter,
	// arrows/home/end/pgup/pgdn move the cursor, Enter jumps to the
	// highlighted symbol, Esc cancels.
	if m.overlay == overlayOutline {
		return m.routeOutline(km)
	}

	// Window-command leader: alt+w arms awaitingWindowKey, and the very next
	// key either runs a window op or disarms. This sits ahead of the global
	// switch so the chord's second key (a plain rune like v/s/c) is consumed
	// here instead of being typed into the buffer.
	if m.awaitingWindowKey {
		m.awaitingWindowKey = false
		if km.Type == tea.KeyRunes && len(km.Runes) == 1 && !km.Alt {
			switch km.Runes[0] {
			case 'v':
				return m.splitPane(splitlayout.Columns)
			case 's':
				return m.splitPane(splitlayout.Rows)
			case 'c':
				return m.closePane()
			}
		}
		// Any other key cancels the leader without acting on the workspace.
		m.status = ""
		return m, nil
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
	case tea.KeyCtrlT:
		// Workspace symbol search. Opens a small modal prompt; the
		// query is sent to the language server's workspace/symbol on
		// Enter, and results land in the multibuffer overlay as
		// fragments (one window per hit, declaration line highlighted
		// Added, three context lines above/below). Matches Zed and VS
		// Code's Ctrl+T muscle memory. Re-opening preserves the last
		// query (cursor at end) so refining a search is one keystroke.
		return m.openSymbolSearch()
	case tea.KeyCtrlBackslash:
		// File outline. Pre-loads the active buffer's symbol tree from
		// the LSP and opens a filterable modal with the cursor parked
		// on the symbol enclosing the current row. Complementary to
		// Ctrl+T (workspace) — outline is scoped to one file. Cached
		// per path so reopens are instant; the cache is dropped on
		// save so post-edit reopens see the new symbols.
		return m.openOutline()
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
	case tea.KeyCtrlUnderscore:
		// Ctrl+/ is delivered as 0x1F (KeyCtrlUnderscore) under every
		// xterm-style emulator — the slash key and the underscore key
		// both fold into the US byte when Ctrl is held. Treat it as the
		// line-comment toggle.
		return m.toggleComment()
	case tea.KeyF2:
		// LSP rename. F2 because Ctrl+R is already taken by
		// "replace current match" in the finder. We ask gopls to
		// prepareRename first so the prompt opens with the actual
		// identifier under the cursor, not whatever character the
		// user happened to be over.
		return m.triggerRename()
	case tea.KeyF5:
		// F5 launches a debug session when none is live, or resumes
		// the paused thread when one is. Alt+F5 (handled further
		// down) terminates. The VS Code default uses Shift+F5 for
		// stop, but bubbletea doesn't surface a portable Shift+F5
		// constant — alt+F5 is the closest unambiguous fallback.
		if km.Alt {
			return m.shutdownDebugSession()
		}
		if m.dapClient != nil && m.dapState == "paused" {
			m.dapState = "running"
			m = m.clearAllStopMarkers()
			m.status = "debug: continuing"
			return m, m.dapContinueCmd(m.dapThreadID)
		}
		return m.startDebugSession()
	case tea.KeyF6:
		// F6 pauses a running session. No-op when idle or paused.
		if m.dapClient == nil || m.dapState != "running" {
			return m, nil
		}
		cli := m.dapClient
		tid := m.dapThreadID
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = cli.Pause(ctx, tid)
			return nil
		}
	case tea.KeyF9:
		// F9 toggles a breakpoint on the active buffer's cursor row.
		// The store keeps the set sticky across sessions; if a
		// debugger is attached, the new set is pushed to the adapter
		// immediately so the next continue honors the change.
		p := m.bufs.Active()
		if p == nil || p.Path() == "" {
			m.status = "no file open for breakpoint"
			return m, nil
		}
		path := p.Path()
		row1 := p.CursorRow() + 1
		set := m.bpStore.Toggle(path, row1)
		m = m.applyBreakpointMarkers(path)
		if set {
			m.status = fmt.Sprintf("breakpoint set %s:%d", filepath.Base(path), row1)
		} else {
			m.status = fmt.Sprintf("breakpoint cleared %s:%d", filepath.Base(path), row1)
		}
		if m.dapClient != nil {
			return m, m.dapSetBreakpointsCmd(path, m.bpStore.Rows(path))
		}
		return m, nil
	case tea.KeyF10:
		// F10 steps over the current line when paused.
		if m.dapClient == nil || m.dapState != "paused" {
			return m, nil
		}
		m.dapState = "running"
		m = m.clearAllStopMarkers()
		m.status = "debug: step over"
		return m, m.dapStepCmd(m.dapThreadID, "next")
	case tea.KeyF11:
		// F11 steps into the call when paused. Alt+F11 steps out.
		if m.dapClient == nil || m.dapState != "paused" {
			return m, nil
		}
		m.dapState = "running"
		m = m.clearAllStopMarkers()
		if km.Alt {
			m.status = "debug: step out"
			return m, m.dapStepCmd(m.dapThreadID, "stepOut")
		}
		m.status = "debug: step in"
		return m, m.dapStepCmd(m.dapThreadID, "stepIn")
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
		case '-':
			// Alt+- walks back one step in the jump list (vim's Ctrl-O).
			// Each push point captures the cursor's from-position before
			// a jump, so Back returns the user to where they triggered
			// the most-recent jump from. Subsequent Backs walk further
			// into the past.
			return m.navJumpBack()
		case '=':
			// Alt+= walks forward through the jump list (vim's Ctrl-I).
			// Only meaningful after at least one Alt+-; returns to the
			// next-newer pushed position.
			return m.navJumpForward()
		case ']':
			m.bufs.Next()
			m.sigPane = m.sigPane.Close()
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
			m.sigPane = m.sigPane.Close()
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
		case 'w':
			// Alt+w is the window-command leader (mnemonic: window). It arms a
			// one-shot state read at the top of routeKey: the next key runs a
			// window op. v splits the focused pane into side-by-side columns, s
			// splits it into stacked rows, c closes the focused pane. Kept off
			// ctrl+w so the historical close-tab binding survives unchanged.
			m.awaitingWindowKey = true
			m.status = "window: v split right · s split down · c close pane"
			return m, nil
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
		case 'z':
			// Alt+z toggles soft wrap on every open pane. When on, long
			// logical rows wrap onto multiple visual rows at the editor's
			// content width; when off, horizontal scrolling resumes. The
			// underlying buffer is never mutated — only the render layer.
			// Default off (matches v0.44 behavior). Mnemonic: "z" sits next
			// to "x" / "c" / "v" — close enough to muscle memory without
			// colliding with copy/paste.
			m.softWrapOn = !m.softWrapOn
			m.bufs.WithSoftWrap(m.softWrapOn)
			if m.softWrapOn {
				m.status = "soft wrap: on"
			} else {
				m.status = "soft wrap: off"
			}
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
		case '.':
			// Alt+. opens the read-only settings overlay — the companion to
			// alt+, (reload). Comma reloads the file, period shows the merged
			// result in force. The two keys sit next to each other so the
			// pairing reads off the keyboard.
			m.overlay = overlaySettings
			return m, nil
		case 'T':
			// Alt+shift+T opens the live theme switcher — the discoverable
			// counterpart to editing `theme` in config.toml and pressing
			// alt+, to reload. Lowercase alt+t is the task runner, so the
			// theme picker takes the shifted key (mirrors alt+k / alt+K).
			// Cursor moves preview each theme live; enter keeps it for the
			// session, esc restores the theme active when the picker opened.
			m.themePicker = themepicker.New(theme.Names(), m.themeName)
			m.themePickerOrig = m.themeName
			m.overlay = overlayThemePicker
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
		case 'k':
			// Alt+k opens the LSP call-hierarchy view in incoming direction —
			// "who calls this?" The two-step LSP graph walk (prepare then
			// callHierarchy/incomingCalls) renders each caller as a
			// multibuffer fragment with the caller's name in the suffix and
			// the call-site rows highlighted. Mnemonic: "k for callers."
			return m.callHierarchyAtCursor(callhierarchy.Incoming)
		case 'K':
			// Alt+Shift+k flips the direction to outgoing — "what does this
			// call?" Same pane, same fragment shape; the suffix now carries
			// the callee's name and the highlighted rows live in the source
			// file. Pairing alt+k / alt+K keeps the muscle memory close to
			// alt+u (find references) which sits one key over.
			return m.callHierarchyAtCursor(callhierarchy.Outgoing)
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
		m.helpQuery = ""
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

	// Signature-help dismissal on Esc takes precedence over the editor's
	// default handling so the user can clear a stale hint without losing
	// their place. Only swallows the Esc when the pane is actually open.
	if km.Type == tea.KeyEsc && m.sigPane.IsOpen() {
		m.sigPane = m.sigPane.Close()
		return m, nil
	}

	// Overload cycling while the signature-help overlay is open. Alt+Down
	// advances; Alt+Up steps back. Both wrap. Forwarding is short-
	// circuited so the editor doesn't also see the arrow as cursor motion.
	if m.sigPane.IsOpen() && km.Alt {
		switch km.Type {
		case tea.KeyDown:
			m.sigPane = m.sigPane.NextOverload()
			return m, nil
		case tea.KeyUp:
			m.sigPane = m.sigPane.PrevOverload()
			return m, nil
		}
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
	// Every key event routed to the editor invalidates the current
	// document-highlight overlay (the cursor may have moved off the
	// previously-highlighted identifier) and arms a fresh settle tick.
	// The clear is unconditional so the band never lags behind the
	// cursor; the new fetch runs only if the LSP is wired (the
	// schedule helper is cheap when no server is attached).
	*p = p.ClearDocumentHighlights()
	if m.lsp != nil && p.Path() != "" {
		if settleCmd := m.scheduleDochiSettle(); settleCmd != nil {
			cmds = append(cmds, settleCmd)
		}
	}
	// If the keypress mutated the buffer and gopls is running for a Go file,
	// publish a didChange so diagnostics stay live as the user types. Chain
	// a semanticTokens refresh after the change so the editor's overlay
	// catches up — semantic tokens are color-only, so a small lag is
	// invisible if a later edit lands before the response.
	if m.lsp != nil && isGoFile(p.Path()) && isMutatingKey(km.Type) {
		v := m.lspVersions[p.Path()] + 1
		m.lspVersions[p.Path()] = v
		cmds = append(cmds,
			tea.Sequence(
				m.lspChangeCmd(p.Path(), v, p.Contents()),
				m.refreshSemanticTokensCmd(),
			),
		)
	}
	// Signature-help trigger: '(' or ',' opens or refreshes the parameter
	// hint overlay; ')' closes it. The closure is intentionally narrow —
	// signature help is a hint, not a flow, so we only honor the explicit
	// open/close characters and an Esc swept up elsewhere.
	if newM, sigCmd := m.maybeUpdateSignatureHelp(km, p); sigCmd != nil {
		m = newM
		cmds = append(cmds, sigCmd)
	} else {
		m = newM
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

// maybeUpdateSignatureHelp reacts to the just-typed key by opening,
// refreshing, or closing the signature-help overlay. Returns the updated
// model (sigPane state and request-pin fields may change) and a tea.Cmd
// fetching fresh signature info when the keystroke warrants it. The
// trigger rules: '(' fires a new fetch (entered call expression);
// ',' fires a new fetch when the pane is already open (active parameter
// likely advanced); ')' closes the pane. Buffers with no path or no LSP
// are no-ops — the signature overlay can't have anything to show.
func (m model) maybeUpdateSignatureHelp(km tea.KeyMsg, p *editor.Pane) (model, tea.Cmd) {
	if p == nil || p.Path() == "" {
		if m.sigPane.IsOpen() {
			m.sigPane = m.sigPane.Close()
		}
		return m, nil
	}
	if km.Type != tea.KeyRunes || len(km.Runes) != 1 {
		return m, nil
	}
	r := km.Runes[0]
	switch r {
	case ')':
		if m.sigPane.IsOpen() {
			m.sigPane = m.sigPane.Close()
		}
		return m, nil
	case '(':
		// Open or refresh: fire a fresh request at the current cursor.
	case ',':
		// Only refresh if the overlay is already up — typing a comma in
		// prose shouldn't summon a signature box.
		if !m.sigPane.IsOpen() {
			return m, nil
		}
	default:
		return m, nil
	}
	if m.lsp == nil {
		return m, nil
	}
	row := p.CursorRow() - 1
	col := p.CursorCol() - 1
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}
	m.sigReqPath = p.Path()
	m.sigReqRow = row
	m.sigReqCol = col
	return m, lookup.SignatureHelpCmd(m.lsp, p.Path(), row, col)
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

// toggleComment toggles line comments on the active buffer's selection
// (or the cursor row when nothing is selected). The editor pane owns the
// transform; the host's job is to drive it, then publish a didChange so
// gopls picks up the edit live for diagnostics on Go files. For files
// with no canonical line-comment marker (HTML, CSS, unknown filetypes)
// editor.ToggleComment is a no-op and we leave the status bar alone so
// the user gets the same "nothing happened" feedback as e.g. ctrl+s on
// a clean buffer.
func (m model) toggleComment() (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil {
		return m, nil
	}
	before := p.Contents()
	*p = p.ToggleComment()
	if p.Contents() == before {
		return m, nil
	}
	var cmd tea.Cmd
	if m.lsp != nil && isGoFile(p.Path()) {
		v := m.lspVersions[p.Path()] + 1
		m.lspVersions[p.Path()] = v
		cmd = m.lspChangeCmd(p.Path(), v, p.Contents())
	}
	return m, cmd
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
// resulting buffer reads correctly. When the server marked the item as a
// snippet (InsertTextFormat == 2), the body is parsed through the nook
// snippet engine and the editor enters snippet mode so Tab/Shift+Tab walk
// the placeholders. Selecting an item is a buffer edit — we send a
// didChange to the language server if it's running.
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
	row := p.CursorRow()
	col := p.CursorCol()
	if item.InsertTextFormat == nooklsp.InsertTextFormatSnippet {
		vars := snippets.DefaultVariables()
		if path := p.Path(); path != "" {
			base := filepath.Base(path)
			vars.Filename = base
			vars.FilenameBase = strings.TrimSuffix(base, filepath.Ext(base))
		}
		exp, err := snippets.Expand(item.InsertText, vars)
		if err != nil {
			m.status = "completion: " + err.Error()
			return m, nil
		}
		prefixStart := col - pl
		if prefixStart < 0 {
			prefixStart = 0
		}
		*p = p.ExpandSnippet(prefixStart, exp)
		if p.InSnippetMode() {
			m.status = "inserted " + item.Label + " — Tab to advance, Esc to exit"
		} else {
			m.status = "inserted " + item.Label
		}
	} else {
		if pl > 0 {
			line := p.Line(row)
			if col-pl >= 0 && col <= len(line) {
				newLine := line[:col-pl] + line[col:]
				// JumpTo is 1-based; row/col are 0-based.
				*p = p.SetLine(row, newLine).JumpTo(row+1, col-pl+1)
			}
		}
		*p = p.InsertText(item.InsertText)
		m.status = "inserted " + item.Label
	}
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
	m.docPane = m.docPane.Close()
	m.docReqLabel = ""
	if m.overlay == overlayCompletion {
		m.overlay = overlayNone
	}
}

// resolveSelectedCompletionCmd fires completionItem/resolve for the
// currently-highlighted popup item so the doc side-panel can render its
// documentation. Updates docReqLabel to the new pin and returns nil when
// the popup is empty or has no selection. Tea will dispatch the resulting
// ResolveCompletionMsg back through Update where the staleness gate drops
// late responses for items the user has already navigated past.
func (m *model) resolveSelectedCompletionCmd() tea.Cmd {
	item, ok := m.completePopup.Selected()
	if !ok {
		m.docPane = m.docPane.Close()
		m.docReqLabel = ""
		return nil
	}
	m.docReqLabel = item.Label
	return lookup.ResolveCompletionCmd(m.lsp, item)
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

// callHierarchyAtCursor is the alt+k / alt+K handler. Resolves the
// identifier under the cursor (same character-class walk as find-references
// uses; no LSP call needed to know what we're hovering), opens the
// multibuffer overlay with a direction-tagged title, and fires the async
// callhierarchy.CallHierarchyCmd which internally does
// textDocument/prepareCallHierarchy then callHierarchy/incomingCalls or
// callHierarchy/outgoingCalls. Errors come back through the standard
// multibuffer.FragmentsMsg pathway; empty results land as a Fragments slice
// of length zero, which the pane renders as "no fragments — press esc to
// close" — accurate UX for "the LSP returned no edges."
//
// Early exits:
//   - no active buffer: status hint, no overlay opened
//   - no LSP client attached yet: status hint, no overlay opened
//   - cursor not on an identifier: status hint, no overlay opened
func (m model) callHierarchyAtCursor(direction callhierarchy.Direction) (tea.Model, tea.Cmd) {
	p := m.bufs.Active()
	if p == nil || p.Path() == "" {
		m.status = "open a file first (ctrl+p)"
		return m, nil
	}
	if m.lsp == nil {
		m.status = direction.Label() + ": no language server attached"
		return m, nil
	}
	row := p.CursorRow()
	col := p.CursorCol()
	sym := callhierarchy.Symbol(p.Contents(), row, col)
	if sym == "" {
		m.status = direction.Label() + ": no identifier under cursor"
		return m, nil
	}
	m.overlay = overlayMultibuffer
	m.multibufPane = m.multibufPane.
		WithSize(m.width-4, m.height-4).
		Reset(direction.Label() + " — " + sym).
		Focus()
	if ap := m.bufs.Active(); ap != nil {
		*ap = ap.Blur()
	}
	return m, callhierarchy.CallHierarchyCmd(
		m.lsp, p.Path(), row, col, direction,
		callhierarchy.DefaultContextLines, nil,
	)
}

// openSymbolSearch arms the workspace-symbol prompt. It re-uses the last
// query (Zed-style: refining a search is one keystroke) and surfaces a
// status hint when no LSP is attached so the user gets an early read on
// why the modal won't return results. The modal opens regardless so the
// user can still type and see the prompt UX even before the server
// finishes initializing.
func (m model) openSymbolSearch() (tea.Model, tea.Cmd) {
	label := filepath.Base(m.root)
	if label == "." || label == "" {
		label = "workspace"
	}
	if m.lastSymQuery != "" {
		m.symbolPrompt = m.symbolPrompt.OpenWith(label, m.lastSymQuery)
	} else {
		m.symbolPrompt = m.symbolPrompt.Open(label)
	}
	m.overlay = overlaySymbolSearch
	if m.lsp == nil {
		m.symbolPrompt = m.symbolPrompt.WithError("no language server attached yet")
	}
	if ap := m.bufs.Active(); ap != nil {
		*ap = ap.Blur()
	}
	m.status = "symbol search: type a query, enter to search, esc to cancel"
	return m, nil
}

// routeSymbolSearch forwards a keypress into the symbolsearch.Prompt.
// Enter fires the workspace/symbol request; Esc cancels. The result is
// rendered through the multibuffer overlay so the user-facing surface
// matches Alt+U (find references) and Alt+M (diff).
func (m model) routeSymbolSearch(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch km.Type {
	case tea.KeyEsc:
		m.symbolPrompt = m.symbolPrompt.Close()
		m.overlay = overlayNone
		m.status = "symbol search cancelled"
		return m, nil
	case tea.KeyEnter:
		query := m.symbolPrompt.Value()
		if query == "" {
			m.symbolPrompt = m.symbolPrompt.WithError("query is empty")
			return m, nil
		}
		if m.lsp == nil {
			m.symbolPrompt = m.symbolPrompt.WithError("no language server attached yet")
			return m, nil
		}
		m.lastSymQuery = query
		m.symbolPrompt = m.symbolPrompt.Close()
		m.overlay = overlayMultibuffer
		m.multibufPane = m.multibufPane.
			WithSize(m.width-4, m.height-4).
			Reset("symbols matching " + query).
			Focus()
		m.status = "asking gopls for workspace symbols…"
		return m, symbolsearch.FindSymbolsCmd(m.lsp, query, symbolsearch.DefaultContextLines, nil)
	case tea.KeyBackspace:
		m.symbolPrompt = m.symbolPrompt.Backspace()
		return m, nil
	case tea.KeyDelete, tea.KeyCtrlD:
		m.symbolPrompt = m.symbolPrompt.Delete()
		return m, nil
	case tea.KeyCtrlU:
		m.symbolPrompt = m.symbolPrompt.Clear()
		return m, nil
	case tea.KeyLeft:
		m.symbolPrompt = m.symbolPrompt.MoveLeft()
		return m, nil
	case tea.KeyRight:
		m.symbolPrompt = m.symbolPrompt.MoveRight()
		return m, nil
	case tea.KeyHome, tea.KeyCtrlA:
		m.symbolPrompt = m.symbolPrompt.MoveHome()
		return m, nil
	case tea.KeyEnd, tea.KeyCtrlE:
		m.symbolPrompt = m.symbolPrompt.MoveEnd()
		return m, nil
	case tea.KeyRunes, tea.KeySpace:
		runes := km.Runes
		if len(runes) == 0 && km.Type == tea.KeySpace {
			runes = []rune{' '}
		}
		for _, r := range runes {
			m.symbolPrompt = m.symbolPrompt.Type(r)
		}
		return m, nil
	}
	return m, nil
}

// outlineRequestMsg carries a single textDocument/documentSymbol result back
// to the host. The Path field lets the host discard a stale response
// when the user has switched buffers or saved-and-edited mid-flight.
type outlineRequestMsg struct {
	path string
	syms []nooklsp.DocSymbol
	err  error
}

// outlineDocumentSymbolCmd asks the LSP for the document symbol tree of
// path. nil-client → an "no language server attached" error so the
// modal still opens and surfaces the hint via outline.OpenError. The
// request runs under a 4s timeout to match the workspace-symbol
// command's window.
func outlineDocumentSymbolCmd(client *nooklsp.Client, path string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return outlineRequestMsg{path: path, err: errors.New("no language server attached")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		syms, err := client.DocumentSymbol(ctx, path)
		return outlineRequestMsg{path: path, syms: syms, err: err}
	}
}

// openOutline opens the file-outline modal. If the active buffer's
// symbol tree is already cached, the pane renders immediately; otherwise
// the pane opens in a loading state (empty list, status hint) and the
// LSP request fires. On reopen-while-loading we no-op so duplicate Ctrl+\
// taps don't stack multiple in-flight requests.
func (m model) openOutline() (tea.Model, tea.Cmd) {
	ap := m.bufs.Active()
	if ap == nil || ap.Path() == "" {
		m.status = "outline: no active file"
		return m, nil
	}
	path := ap.Path()
	row := ap.CursorRow()
	w, h := outlineDimensions(m.width, m.height)
	m.outlinePane = m.outlinePane.WithSize(w, h)
	if cached, ok := m.outlineCache[path]; ok {
		m.outlinePane = m.outlinePane.Open(path, cached, row)
		m.overlay = overlayOutline
		*ap = ap.Blur()
		m.status = "outline: " + filepath.Base(path)
		return m, nil
	}
	if m.outlineLoading[path] {
		return m, nil
	}
	if m.lsp == nil {
		m.outlinePane = m.outlinePane.OpenError(path, "no language server attached yet")
		m.overlay = overlayOutline
		*ap = ap.Blur()
		m.status = "outline: no language server"
		return m, nil
	}
	m.outlineLoading[path] = true
	m.outlinePane = m.outlinePane.Open(path, nil, row)
	m.overlay = overlayOutline
	*ap = ap.Blur()
	m.status = "outline: asking gopls for document symbols…"
	return m, outlineDocumentSymbolCmd(m.lsp, path)
}

// outlineDimensions picks the modal size from the workspace dimensions.
// The pane is at most 80 cols wide and 24 rows tall, clamped to leave
// breathing room around the edges (4 cols / 2 rows minimum). A 60×18
// floor keeps the pane usable even on a tiny TTY.
func outlineDimensions(workspaceW, workspaceH int) (int, int) {
	w := workspaceW - 6
	if w > 80 {
		w = 80
	}
	if w < 60 {
		w = 60
	}
	h := workspaceH - 6
	if h > 24 {
		h = 24
	}
	if h < 18 {
		h = 18
	}
	return w, h
}

// handleOutlineRequestMsg consumes the LSP response. Stale responses
// (the user switched files or closed the modal before the result
// arrived) are dropped silently.
func (m model) handleOutlineRequestMsg(msg outlineRequestMsg) (tea.Model, tea.Cmd) {
	delete(m.outlineLoading, msg.path)
	if msg.err != nil {
		if m.overlay == overlayOutline && m.outlinePane.Path() == msg.path {
			m.outlinePane = m.outlinePane.OpenError(msg.path, msg.err.Error())
			m.status = "outline: " + msg.err.Error()
		}
		return m, nil
	}
	m.outlineCache[msg.path] = msg.syms
	if m.overlay != overlayOutline || m.outlinePane.Path() != msg.path {
		// User moved on; cache update is the only side effect.
		return m, nil
	}
	row := 0
	if ap := m.bufs.Active(); ap != nil && ap.Path() == msg.path {
		row = ap.CursorRow()
	}
	m.outlinePane = m.outlinePane.Open(msg.path, msg.syms, row)
	m.status = "outline: " + filepath.Base(msg.path)
	return m, nil
}

// routeOutline forwards a keypress to the outline pane and reacts to the
// Jump / Cancel messages it emits.
func (m model) routeOutline(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.outlinePane, cmd = m.outlinePane.Update(km)
	if cmd == nil {
		if !m.outlinePane.IsOpen() {
			m.overlay = overlayNone
		}
		return m, nil
	}
	msg := cmd()
	switch v := msg.(type) {
	case outline.JumpMsg:
		m.overlay = overlayNone
		ap := m.bufs.Active()
		if ap == nil || ap.Path() != v.Path {
			m.status = "outline: target file no longer active"
			return m, nil
		}
		m.pushNavCurrent()
		*ap = ap.JumpTo(v.Row+1, v.Col+1).Focus()
		m.status = "outline: jumped to " + filepath.Base(v.Path)
		return m, nil
	case outline.CancelMsg:
		m.overlay = overlayNone
		if ap := m.bufs.Active(); ap != nil {
			*ap = ap.Focus()
		}
		m.status = "outline cancelled"
		return m, nil
	}
	return m, nil
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
	m.sigPane = m.sigPane.Close()
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

// splitPane divides the focused pane along o and binds the new pane to a
// different open buffer, so the split immediately shows two files. v1 caps the
// layout at two panes (one split) and needs at least two open buffers:
// splitting a lone buffer would show the same content twice, which the
// per-pane sizing can't honor. The fresh pane takes focus, and its buffer is
// made active so every existing editing path operates on the focused pane.
func (m model) splitPane(o splitlayout.Orientation) (tea.Model, tea.Cmd) {
	if m.split == nil {
		return m, nil
	}
	if m.split.Count() > 1 {
		m.status = "nook shows two panes in this version — close one with alt+w c"
		return m, nil
	}
	if m.bufs.Count() < 2 {
		m.status = "open another file to split (ctrl+P)"
		return m, nil
	}
	old := m.split.Focused()
	m.paneBuf[old] = m.bufs.ActiveIndex()
	other := (m.bufs.ActiveIndex() + 1) % m.bufs.Count()
	newID := m.split.SplitFocused(o)
	m.paneBuf[newID] = other
	m.bufs.Switch(other)
	m = m.resize()
	m = m.applyDiagnosticsToActive()
	m.status = "split — alt+w c to close the pane"
	return m, nil
}

// closePane closes the focused split pane; its space collapses into the
// sibling. The buffer the pane showed stays open as a tab, so only the pane
// binding is dropped. Closing the sole remaining pane is refused by the layout
// tree. After a close the surviving focused pane's buffer becomes active.
func (m model) closePane() (tea.Model, tea.Cmd) {
	if m.split == nil {
		return m, nil
	}
	closed := m.split.Focused()
	if !m.split.CloseFocused() {
		m.status = "only one pane open"
		return m, nil
	}
	delete(m.paneBuf, closed)
	if idx, ok := m.paneBuf[m.split.Focused()]; ok {
		m.bufs.Switch(idx)
	}
	m = m.resize()
	m = m.applyDiagnosticsToActive()
	m.status = "pane closed"
	return m, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m model) routeProjectSearch(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Replace mode wins over query editing. Rune input, space, and
	// backspace edit the replacement field; Enter/Esc/navigation are
	// handled by the pane's own Update which emits ApplyMsg on Enter
	// and collapses back to result navigation on Esc.
	if m.search.Replacing() {
		switch km.Type {
		case tea.KeyRunes:
			m.search = m.search.AppendReplacementRune(string(km.Runes))
			return m, nil
		case tea.KeySpace:
			m.search = m.search.AppendReplacementRune(" ")
			return m, nil
		case tea.KeyBackspace:
			m.search = m.search.BackspaceReplacement()
			return m, nil
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(km)
		return m, cmd
	}
	// Alt+R toggles replace mode. Lives outside the typing branch so it
	// fires whether the query field is empty or the result list has
	// already streamed in (it no-ops on either empty matches or while
	// ripgrep is still running, so the user doesn't enter a useless
	// "replace with nothing" state).
	if km.Alt && km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == 'r' {
		m.search = m.search.EnterReplace()
		return m, nil
	}
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
	refreshCmd := m.treePane.RefreshCmd()
	if p := m.bufs.Active(); p != nil && p.Path() != "" {
		m.treePane.Reveal(p.Path())
	}
	m.treePane.Focus()
	m = m.resize()
	m.status = "file tree open"
	return m, refreshCmd
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
	if m.split != nil && m.split.Count() > 1 {
		// A split is live: each on-screen buffer is sized to its own pane
		// rectangle, not the shared region. Buffers not bound to any pane
		// keep whatever size they last had; they are off-screen.
		rects := m.split.Rects(leftW, bodyH)
		for pid, idx := range m.paneBuf {
			if r, ok := rects[pid]; ok {
				m.bufs.SetSizeAt(idx, r.W, r.H)
			}
		}
	} else {
		m.bufs.WithSize(leftW, bodyH)
	}
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
		float := centerOverlay(m.width, m.height-1, help.ViewQuery(t, m.width, m.helpQuery))
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlaySettings {
		card := settings.View(t, m.width, m.cfg, m.cfgPath, m.prjCfgPath, m.cfgUserExists, m.cfgProjectExists)
		float := centerOverlay(m.width, m.height-1, card)
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlayThemePicker {
		card := themepicker.View(t, m.width, m.themePicker)
		float := centerOverlay(m.width, m.height-1, card)
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
		// Stitch the doc side panel beside the menu when the server has
		// resolved documentation for the highlighted item. Skip if the
		// screen is too narrow to fit menu + gap + min-doc-width without
		// crowding the editor underneath.
		if m.docPane.IsOpen() {
			docW := boxW
			if docW < 36 {
				docW = 36
			}
			needed := boxW + 1 + docW
			if needed <= m.width-2 {
				docH := maxRows + 2
				doc := m.docPane.WithSize(docW, docH).View(t)
				if doc != "" {
					box = lipgloss.JoinHorizontal(lipgloss.Top, box, " ", doc)
				}
			}
		}
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
	if m.overlay == overlayCreate {
		boxW := 60
		if boxW > m.width-4 {
			boxW = m.width - 4
		}
		box := m.createPrompt.View(t, boxW)
		float := centerOverlay(m.width, m.height-1, box)
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlaySymbolSearch {
		boxW := 64
		if boxW > m.width-4 {
			boxW = m.width - 4
		}
		box := m.symbolPrompt.View(t, boxW)
		float := centerOverlay(m.width, m.height-1, box)
		return lipgloss.JoinVertical(lipgloss.Left, float, statusBar)
	}
	if m.overlay == overlayOutline {
		box := m.outlinePane.View()
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

	var sigBar string
	var sigBarH int
	if m.sigPane.IsOpen() {
		sigBar = m.sigPane.View(t)
		sigBarH = lipgloss.Height(sigBar)
	}

	bodyH := m.height - 2
	if tabBar != "" {
		bodyH--
	}
	if fh := m.finder.Height(); fh > 0 {
		bodyH -= fh
	}
	if sigBarH > 0 {
		bodyH -= sigBarH
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

	segments := make([]string, 0, 5)
	if tabBar != "" {
		segments = append(segments, tabBar)
	}
	segments = append(segments, body)
	if sigBar != "" {
		segments = append(segments, sigBar)
	}
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
	if m.split != nil && m.split.Count() == 2 {
		return m.renderSplitPanes()
	}
	// One pane (or, defensively, a tree state v1 never produces) renders the
	// focused buffer exactly as before splits existed.
	return m.bufs.Active().View()
}

// renderSplitPanes composes the two-pane view. v1 caps at a single split, so
// exactly two panes partition the editor region: side by side with a vertical
// divider (Columns) or stacked with a horizontal one (Rows). Each pane's
// buffer is sized to its own rectangle by resize(); here each view is forced
// to that rectangle so the join stays clean, and the composed output fills the
// whole region. The divider orientation comes straight from the layout tree.
func (m model) renderSplitPanes() string {
	t := m.theme
	w, h := m.editorRegion()
	rects := m.split.Rects(w, h)

	// Order the two panes by screen position so left/top renders first.
	ids := m.split.Panes()
	if len(ids) != 2 {
		return m.bufs.Active().View()
	}
	divs := m.split.Dividers(w, h)
	orient := splitlayout.Columns
	if len(divs) > 0 {
		orient = divs[0].Orient
	}
	if orient == splitlayout.Columns {
		if rects[ids[0]].X > rects[ids[1]].X {
			ids[0], ids[1] = ids[1], ids[0]
		}
	} else {
		if rects[ids[0]].Y > rects[ids[1]].Y {
			ids[0], ids[1] = ids[1], ids[0]
		}
	}

	view := func(id splitlayout.PaneID) string {
		r := rects[id]
		body := ""
		if buf := m.bufs.At(m.paneBuf[id]); buf != nil {
			body = buf.View()
		}
		// Force the pane block to its exact rectangle so neighbouring panes
		// line up and the composite matches the region byte for byte.
		return lipgloss.NewStyle().
			Width(r.W).Height(r.H).
			MaxWidth(r.W).MaxHeight(r.H).
			Render(body)
	}

	a, b := view(ids[0]), view(ids[1])
	if orient == splitlayout.Columns {
		divider := lipgloss.NewStyle().Foreground(t.Border).
			Render(strings.TrimRight(strings.Repeat("│\n", h), "\n"))
		return lipgloss.JoinHorizontal(lipgloss.Top, a, divider, b)
	}
	divider := lipgloss.NewStyle().Foreground(t.Border).
		Render(strings.Repeat("─", w))
	return lipgloss.JoinVertical(lipgloss.Left, a, divider, b)
}

// shouldShowWelcome is true when there are no open buffers. Closing the last
// buffer is symmetric with first-run: the welcome card returns. This drives
// the user back to ctrl+P naturally.
func (m model) shouldShowWelcome() bool {
	return m.bufs.Count() == 0
}

// editorRegion returns the dimensions of the whole editor body: the area
// left after the tree (left) and the right pane claim their columns and the
// tab/status bars claim their rows. Split panes subdivide this region; with a
// single pane it IS the pane. Must mirror resize() exactly: tree on the left
// when visible, right pane on the right when active, and a minimum-20 editor
// floor that shrinks the tree when necessary.
func (m model) editorRegion() (int, int) {
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

// editorSize returns the dimensions allocated to the focused editor pane.
// It routes the whole editor region (editorRegion) through the split layout
// tree and reports the focused pane's rectangle. With a single pane that
// rectangle equals the region exactly, so every caller behaves as it did
// before splits existed; once a split lands, each pane gets its own slice.
func (m model) editorSize() (int, int) {
	w, h := m.editorRegion()
	if m.split == nil {
		return w, h
	}
	r := m.split.Rects(w, h)[m.split.Focused()]
	return r.W, r.H
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

	if m.navHistory != nil {
		if pos, total := m.navHistory.Position(); total > 0 && pos != total {
			parts = append(parts, muted.Render(fmt.Sprintf("nav %d/%d", pos, total)))
		}
	}

	if errs, warns := m.diagCounts(); errs > 0 || warns > 0 {
		parts = append(parts, muted.Render(fmt.Sprintf("●%dE %dW", errs, warns)))
	}

	if m.dapState != "" || (m.bpStore != nil && m.bpStore.Count() > 0) {
		var seg strings.Builder
		seg.WriteString("dbg")
		if m.dapState != "" {
			seg.WriteString(":")
			seg.WriteString(m.dapState)
		}
		if m.bpStore != nil {
			if n := m.bpStore.Count(); n > 0 {
				seg.WriteString(fmt.Sprintf(" ●%d", n))
			}
		}
		style := muted
		switch m.dapState {
		case "paused":
			style = lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
		case "running", "launching":
			style = lipgloss.NewStyle().Foreground(t.Success)
		case "terminated":
			style = lipgloss.NewStyle().Foreground(t.Error)
		}
		parts = append(parts, style.Render(seg.String()))
	}

	if chip := airules.StatusChip(m.rulesSource); chip != "" {
		parts = append(parts, muted.Render(chip))
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

// pushNavCurrent records the active buffer's cursor position into the
// jump list. Called immediately before any cross-file or significant
// in-file jump fires, so Alt+- can walk back to where the user came from.
// A no-op when no buffer is active.
func (m *model) pushNavCurrent() {
	p := m.bufs.Active()
	if p == nil || p.Path() == "" {
		return
	}
	m.navHistory.Push(navhistory.Entry{
		Path: p.Path(),
		Row:  p.CursorRow(),
		Col:  p.CursorCol(),
	})
}

// jumpToNavEntry switches to the entry's buffer and moves the cursor.
// Used by Alt+- (Back) and Alt+= (Forward). Returns whether the jump
// succeeded — false when the path can no longer be opened.
func (m *model) jumpToNavEntry(e navhistory.Entry) bool {
	if e.Path == "" {
		return false
	}
	m.bufs.OpenOrSwitch(e.Path)
	p := m.bufs.Active()
	if p == nil {
		return false
	}
	*p = p.JumpTo(e.Row+1, e.Col+1).Focus()
	if m.showTree {
		m.treePane.Reveal(e.Path)
	}
	return true
}

// navJumpBack walks one step back through the jump list.
func (m model) navJumpBack() (tea.Model, tea.Cmd) {
	e, ok := m.navHistory.Back()
	if !ok {
		m.status = "jump list: at oldest entry"
		return m, nil
	}
	if !m.jumpToNavEntry(e) {
		m.status = "jump list: target no longer reachable"
		return m, nil
	}
	m = m.resize()
	m = m.applyDiagnosticsToActive()
	pos, total := m.navHistory.Position()
	m.status = fmt.Sprintf("jump %d/%d ← %s:%d:%d", pos, total, filepath.Base(e.Path), e.Row+1, e.Col+1)
	return m, tea.Batch(m.ensureLSPForFile(e.Path), m.refreshGutterCmd(), m.refreshInlayHintsCmd(), m.refreshBlameCmd())
}

// navJumpForward walks one step forward through the jump list.
func (m model) navJumpForward() (tea.Model, tea.Cmd) {
	e, ok := m.navHistory.Forward()
	if !ok {
		m.status = "jump list: at newest entry"
		return m, nil
	}
	if !m.jumpToNavEntry(e) {
		m.status = "jump list: target no longer reachable"
		return m, nil
	}
	m = m.resize()
	m = m.applyDiagnosticsToActive()
	pos, total := m.navHistory.Position()
	m.status = fmt.Sprintf("jump %d/%d → %s:%d:%d", pos, total, filepath.Base(e.Path), e.Row+1, e.Col+1)
	return m, tea.Batch(m.ensureLSPForFile(e.Path), m.refreshGutterCmd(), m.refreshInlayHintsCmd(), m.refreshBlameCmd())
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
				Code:     diagnostics.CodeString(d.Code),
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

// DAP wiring: messages, commands, and helpers. The host owns the dap.Client
// lifecycle, drains its event channel via dapPumpCmd, and reflects stop/
// resume events into the editor pane's gutter (● for breakpoints, ▶ for
// the current stop). Delve is the only supported adapter in v0.30.0.

// dapPumpMsg carries one event from dap.Client.Events() plus the channel
// itself so the Update handler can re-schedule the read for the next event.
type dapPumpMsg struct {
	event dap.Event
	ch    <-chan dap.Event
}

// dapStartedMsg announces the result of spawning the adapter and finishing
// the initialize / launch / configurationDone handshake.
type dapStartedMsg struct {
	client  *dap.Client
	program string
	err     error
}

// dapStackFrameMsg carries the top frame after a stop, used to locate the
// editor pane to highlight.
type dapStackFrameMsg struct {
	frames []dap.StackFrame
	err    error
}

// dapBreakpointsResultMsg surfaces the adapter's confirmation that a
// setBreakpoints round-trip succeeded (or failed) for one file.
type dapBreakpointsResultMsg struct {
	path string
	err  error
}

// dapPumpCmd reads one event off the client's channel and resurfaces it
// as a tea.Msg. Re-armed on every receipt so the channel keeps flowing
// until the adapter terminates and closes it.
func (m model) dapPumpCmd(ch <-chan dap.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return dap.Event{Kind: "terminated", Terminated: &dap.TerminatedBody{}}
		}
		return dapPumpMsg{event: ev, ch: ch}
	}
}

// dapStartCmd spawns dlv, initializes, launches the program, replays the
// host's persisted breakpoints, and signals configurationDone. The result
// arrives as dapStartedMsg. All RPCs share a 10-second context — the launch
// handshake is fast on a healthy machine and a hang means the adapter is
// broken, not slow.
func (m model) dapStartCmd(program, workDir string, snap map[string][]int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client, err := dap.Start(ctx, dap.Options{WorkDir: workDir})
		if err != nil {
			return dapStartedMsg{err: err}
		}
		if err := client.Initialize(ctx, "nook"); err != nil {
			_ = client.Shutdown()
			return dapStartedMsg{err: fmt.Errorf("initialize: %w", err)}
		}
		if err := client.Launch(ctx, program, "debug"); err != nil {
			_ = client.Shutdown()
			return dapStartedMsg{err: fmt.Errorf("launch: %w", err)}
		}
		for path, rows := range snap {
			lines := make([]int, len(rows))
			copy(lines, rows)
			if _, err := client.SetBreakpoints(ctx, dap.Source{Path: path}, lines); err != nil {
				_ = client.Shutdown()
				return dapStartedMsg{err: fmt.Errorf("setBreakpoints %s: %w", filepath.Base(path), err)}
			}
		}
		if err := client.ConfigurationDone(ctx); err != nil {
			_ = client.Shutdown()
			return dapStartedMsg{err: fmt.Errorf("configurationDone: %w", err)}
		}
		return dapStartedMsg{client: client, program: program}
	}
}

// dapSetBreakpointsCmd pushes the current breakpoint set for one file to
// the adapter. Called when F9 toggles a breakpoint with a live session.
func (m model) dapSetBreakpointsCmd(path string, rows []int) tea.Cmd {
	cli := m.dapClient
	if cli == nil {
		return nil
	}
	lines := make([]int, len(rows))
	copy(lines, rows)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := cli.SetBreakpoints(ctx, dap.Source{Path: path}, lines)
		return dapBreakpointsResultMsg{path: path, err: err}
	}
}

// dapStackTraceCmd fetches the top frame for threadID so the host can
// open the source file and highlight the stop row. Fired on every
// "stopped" event.
func (m model) dapStackTraceCmd(threadID int) tea.Cmd {
	cli := m.dapClient
	if cli == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		frames, err := cli.StackTrace(ctx, threadID, 1)
		return dapStackFrameMsg{frames: frames, err: err}
	}
}

// dapContinueCmd resumes the paused thread.
func (m model) dapContinueCmd(threadID int) tea.Cmd {
	cli := m.dapClient
	if cli == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cli.Continue(ctx, threadID)
		return nil
	}
}

// dapStepCmd issues one of next/stepIn/stepOut on the paused thread.
// kind is "next" / "stepIn" / "stepOut"; unknown kinds are no-ops.
func (m model) dapStepCmd(threadID int, kind string) tea.Cmd {
	cli := m.dapClient
	if cli == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		switch kind {
		case "next":
			_ = cli.Next(ctx, threadID)
		case "stepIn":
			_ = cli.StepIn(ctx, threadID)
		case "stepOut":
			_ = cli.StepOut(ctx, threadID)
		}
		return nil
	}
}

// dapTerminateCmd asks the adapter to kill the debuggee and disconnect.
// Runs in a goroutine so the UI doesn't block on a slow adapter.
func (m model) dapTerminateCmd() tea.Cmd {
	cli := m.dapClient
	if cli == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = cli.Terminate(ctx)
		_ = cli.Disconnect(ctx, true)
		_ = cli.Shutdown()
		return nil
	}
}

// applyBreakpointMarkers pushes the current breakpoint set for path onto
// the matching editor pane. Rows are converted from 1-based (DAP / on-disk)
// to 0-based (editor.Pane convention). Idempotent and silent when the
// pane isn't open.
func (m model) applyBreakpointMarkers(path string) model {
	idx := m.bufs.Find(path)
	if idx < 0 {
		return m
	}
	p := m.bufs.At(idx)
	if p == nil {
		return m
	}
	rows1 := m.bpStore.Rows(path)
	if len(rows1) == 0 {
		*p = p.SetBreakpointRows(nil)
		return m
	}
	rows := make(map[int]bool, len(rows1))
	for _, r := range rows1 {
		rows[r-1] = true
	}
	*p = p.SetBreakpointRows(rows)
	return m
}

// applyAllBreakpointMarkers refreshes every open buffer's breakpoint set
// from the store. Used after a buffer switch or after the store changes
// behind the host's back (e.g. session start that replays the set).
func (m model) applyAllBreakpointMarkers() model {
	for i := 0; i < m.bufs.Count(); i++ {
		p := m.bufs.At(i)
		if p == nil {
			continue
		}
		if path := p.Path(); path != "" {
			m = m.applyBreakpointMarkers(path)
		}
	}
	return m
}

// clearAllStopMarkers wipes the ▶ marker from every open buffer. Called
// on continue/terminate so the old stop indicator doesn't linger.
func (m model) clearAllStopMarkers() model {
	for i := 0; i < m.bufs.Count(); i++ {
		p := m.bufs.At(i)
		if p == nil {
			continue
		}
		*p = p.SetStoppedAtRow(-1)
	}
	return m
}

// applyStopMarker sets the ▶ marker on path at the given 0-based row.
// If the buffer isn't open, the host opens it first so the user sees
// where execution paused.
func (m model) applyStopMarker(path string, row0 int) model {
	if m.bufs.Find(path) < 0 {
		m.bufs.OpenOrSwitch(path)
		m = m.resize()
	}
	p := m.bufs.At(m.bufs.Find(path))
	if p == nil {
		return m
	}
	*p = p.SetStoppedAtRow(row0)
	// Park the cursor on the stop row so paging up/down is intuitive.
	*p = p.JumpTo(row0+1, 1)
	return m
}

// startDebugSession decides what program to launch (the active file's
// directory, or m.root) and fires dapStartCmd. Called by F5 with no live
// session.
func (m model) startDebugSession() (model, tea.Cmd) {
	if m.dapClient != nil {
		return m, nil
	}
	program := m.root
	if p := m.bufs.Active(); p != nil && p.Path() != "" {
		if isGoFile(p.Path()) {
			program = filepath.Dir(p.Path())
		}
	}
	m.dapState = "launching"
	m.status = "debug: launching " + filepath.Base(program) + "…"
	return m, m.dapStartCmd(program, m.root, m.bpStore.Snapshot())
}

// shutdownDebugSession tears down a live session. Returns the cleared
// model and a fire-and-forget cmd that calls Terminate/Disconnect/Shutdown
// on the adapter off the UI goroutine.
func (m model) shutdownDebugSession() (model, tea.Cmd) {
	if m.dapClient == nil {
		return m, nil
	}
	cmd := m.dapTerminateCmd()
	m.dapClient = nil
	m.dapState = ""
	m.dapThreadID = 0
	m.dapPausedPath = ""
	m.dapPausedRow = 0
	m = m.clearAllStopMarkers()
	m.status = "debug: terminated"
	return m, cmd
}

// createPathMsg is the result of a filetreeops.CreatePath call. Result
// carries the absolute path of the new entry on success; Err is the
// error from filetreeops (collision, invalid name, fs failure). The host
// branches on Err.
type createPathMsg struct {
	Result filetreeops.CreateResult
	Err    error
}

// createPathCmd runs filetreeops.CreatePath in a goroutine and routes the
// outcome back as createPathMsg. The filesystem call is fast (single
// O_EXCL open or MkdirAll), but keeping it off the UI thread keeps the
// reveal/refresh latency-free and consistent with the rest of the host.
func createPathCmd(parentDir, input string) tea.Cmd {
	return func() tea.Msg {
		res, err := filetreeops.CreatePath(parentDir, input)
		return createPathMsg{Result: res, Err: err}
	}
}

// openCreatePrompt arms the create-prompt overlay with a parent label
// derived from the parent-directory path the filetree pane sent. The
// label shown is the path relative to the project root, falling back to
// "." when the parent IS the root.
func (m model) openCreatePrompt(parentDir string) model {
	rel, err := filepath.Rel(m.root, parentDir)
	if err != nil || rel == "" {
		rel = "."
	}
	rel = filepath.ToSlash(rel)
	m.createPrompt = m.createPrompt.WithParent(rel)
	m.createParentDir = parentDir
	m.overlay = overlayCreate
	m.status = "new: type path, / suffix for dir, enter to create"
	return m
}

// dismissCreatePrompt closes the create-prompt overlay and clears the
// parent-dir pin.
func (m *model) dismissCreatePrompt() {
	m.createPrompt = createprompt.New()
	m.createParentDir = ""
	if m.overlay == overlayCreate {
		m.overlay = overlayNone
	}
}

// routeCreate forwards a keypress into the create.Prompt. Enter fires the
// create command, Esc cancels.
func (m model) routeCreate(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch km.Type {
	case tea.KeyEsc:
		m.dismissCreatePrompt()
		m.status = "new: cancelled"
		return m, nil
	case tea.KeyEnter:
		value := m.createPrompt.Value()
		if value == "" {
			m.createPrompt = m.createPrompt.WithError("name is empty")
			return m, nil
		}
		parent := m.createParentDir
		return m, createPathCmd(parent, value)
	case tea.KeyBackspace:
		m.createPrompt = m.createPrompt.Backspace()
		return m, nil
	case tea.KeyLeft:
		m.createPrompt = m.createPrompt.MoveLeft()
		return m, nil
	case tea.KeyRight:
		m.createPrompt = m.createPrompt.MoveRight()
		return m, nil
	case tea.KeyHome, tea.KeyCtrlA:
		m.createPrompt = m.createPrompt.MoveHome()
		return m, nil
	case tea.KeyEnd, tea.KeyCtrlE:
		m.createPrompt = m.createPrompt.MoveEnd()
		return m, nil
	case tea.KeyRunes:
		for _, r := range km.Runes {
			m.createPrompt = m.createPrompt.Type(r)
		}
		return m, nil
	case tea.KeySpace:
		m.createPrompt = m.createPrompt.Type(' ')
		return m, nil
	}
	return m, nil
}

// handleCreatePathMsg dispatches the outcome of a createPathCmd. Errors
// keep the prompt open so the user can retype. Success closes the
// prompt, refreshes the file tree, reveals the new entry, and (for
// files) opens it in a buffer.
func (m model) handleCreatePathMsg(msg createPathMsg) (model, tea.Cmd) {
	if msg.Err != nil {
		m.createPrompt = m.createPrompt.WithError(createErrorString(msg.Err))
		return m, nil
	}
	res := msg.Result
	rel, _ := filepath.Rel(m.root, res.Path)
	rel = filepath.ToSlash(rel)

	m.dismissCreatePrompt()
	m.treePane.Reveal(res.Path)

	if res.IsDir {
		m.status = "created " + rel + "/"
		return m, m.treePane.RefreshCmd()
	}

	_, action := m.bufs.OpenOrSwitch(res.Path)
	switch action {
	case bufman.Switched:
		m.status = "created " + rel + " (already open, switched)"
	default:
		m.status = "created " + rel
	}
	m = m.resize()
	m = m.applyDiagnosticsToActive()
	return m, tea.Batch(
		m.treePane.RefreshCmd(),
		m.ensureLSPForFile(res.Path),
		m.refreshGutterCmd(),
		m.refreshInlayHintsCmd(),
		m.refreshBlameCmd(),
	)
}

// createErrorString turns a filetreeops error into a short user-facing
// label. The package-prefixed errors are accurate for logs but too noisy
// for a small prompt; we map the well-known sentinels to terse strings.
func createErrorString(err error) string {
	switch {
	case errors.Is(err, filetreeops.ErrEmptyName):
		return "name is empty"
	case errors.Is(err, filetreeops.ErrAbsolutePath):
		return "name must be relative"
	case errors.Is(err, filetreeops.ErrInvalidName):
		return "invalid path (no .. allowed)"
	case errors.Is(err, filetreeops.ErrPathExists):
		return "path already exists"
	default:
		return err.Error()
	}
}
