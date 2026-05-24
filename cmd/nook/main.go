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
//	ctrl+g     git pane
//	ctrl+k     inline AI edit on current line (Haiku)
//	ctrl+l     composer multi-file AI edit (Sonnet)
//	ctrl+`     terminal pane
//	ctrl+s     save current buffer
//	alt+i      LSP hover info for symbol under cursor
//	ctrl+]     LSP go to definition
//	ctrl+space LSP completion popup (↑/↓ to navigate, enter to accept)
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
	"github.com/truffle-dev/glyph/cmd/nook/internal/bufman"
	"github.com/truffle-dev/glyph/cmd/nook/internal/complete"
	"github.com/truffle-dev/glyph/cmd/nook/internal/composer"
	"github.com/truffle-dev/glyph/cmd/nook/internal/edit"
	"github.com/truffle-dev/glyph/cmd/nook/internal/editor"
	"github.com/truffle-dev/glyph/cmd/nook/internal/finder"
	"github.com/truffle-dev/glyph/cmd/nook/internal/ghost"
	"github.com/truffle-dev/glyph/cmd/nook/internal/git"
	"github.com/truffle-dev/glyph/cmd/nook/internal/help"
	"github.com/truffle-dev/glyph/cmd/nook/internal/highlight"
	"github.com/truffle-dev/glyph/cmd/nook/internal/hover"
	"github.com/truffle-dev/glyph/cmd/nook/internal/lookup"
	nooklsp "github.com/truffle-dev/glyph/cmd/nook/internal/lsp"
	"github.com/truffle-dev/glyph/cmd/nook/internal/picker"
	"github.com/truffle-dev/glyph/cmd/nook/internal/search"
	"github.com/truffle-dev/glyph/cmd/nook/internal/tabbar"
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
)

// rightPane is which of git/term/diff/composer occupies the lower-right slot.
type rightPane int

const (
	rightNone rightPane = iota
	rightGit
	rightTerm
	rightDiff
	rightComposer
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

	aiClient *ai.Client

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

func newModel(root string) model {
	t := theme.Default
	aiClient, _ := ai.NewClient() // tolerated nil; AI panes surface their own error
	// The welcome card carries the full keymap; the status bar just nudges
	// new users toward the help overlay. Once a file is open, callers
	// overwrite m.status with feedback for whatever they just did.
	status := "press ? for keymap"
	return model{
		theme:        t,
		root:         root,
		width:        80,
		height:       24,
		bufs:         bufman.New(t).WithHighlighter(highlight.New()),
		gitPane:      git.NewPane(t, root),
		termPane:     term.NewPane(t, root),
		picker:       picker.New(t).WithTitle("Open file").WithPlaceholder("type to filter…"),
		search:       search.NewPane(t, root),
		editPane:     edit.NewPane(t, aiClient),
		composer:     composer.NewPane(t, aiClient),
		ghost:        ghost.NewManager(aiClient),
		right:        rightNone,
		status:       status,
		aiClient:     aiClient,
		lspVersions:  map[string]int32{},
		diagnostics:  map[string][]protocol.Diagnostic{},
		finder:       finder.New(t),
		formatOnSave: true,
	}
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
		m = m.applyDiagnosticsToActive()
		return m, m.ensureLSPForFile(full)

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
		}
		return m, m.refreshGitCmd()

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
		m = m.applyDiagnosticsToActive()
		return m, m.ensureLSPForFile(msg.Path)

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
		m = m.applyDiagnosticsToActive()
		m.status = fmt.Sprintf("jumped to %s:%d:%d", filepath.Base(loc.Path), loc.Line+1, loc.Col+1)
		return m, m.ensureLSPForFile(loc.Path)

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
				m.status = "switched to " + filepath.Base(p.Path())
			}
			return m, nil
		case '[':
			m.bufs.Prev()
			m = m.resize()
			m = m.applyDiagnosticsToActive()
			if p := m.bufs.Active(); p != nil {
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
	}

	// No overlay: composer takes keys when focused
	if m.right == rightComposer && m.composer.Focused() {
		var cmd tea.Cmd
		m.composer, cmd = m.composer.Update(km)
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
	m.status = "formatting " + filepath.Base(p.Path()) + "…"
	return m, lookup.FormattingCmd(m.lsp, p.Path(), version, 4, false)
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
	if p := m.bufs.Active(); p != nil && p.Path() != "" {
		ctx.OpenPath = p.Path()
		ctx.OpenContents = p.Contents()
	}
	m.composer = m.composer.WithContext(ctx).Focus()
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
	// - top: workspace (editor) + right (git/term/diff if active)
	// - bottom: status bar
	leftW := m.width
	rightW := 0
	if m.right != rightNone {
		rightW = m.width / 3
		if rightW < 40 {
			rightW = 40
		}
		if rightW > m.width-40 {
			rightW = m.width - 40
		}
		leftW = m.width - rightW - 1
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
	m.editPane = m.editPane.WithSize(min(70, m.width-4), 10)
	m.search = m.search.WithSize(m.width-4, m.height-6)
	m.picker = m.picker.WithSize(m.width-8, m.height-6)
	m.finder = m.finder.WithSize(m.width)
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

	left := m.renderMainColumn()

	tabBar := m.renderTabBar()

	var finderBar string
	if m.finder.IsOpen() {
		finderBar = m.finder.View()
	}

	if m.right == rightNone {
		segments := make([]string, 0, 4)
		if tabBar != "" {
			segments = append(segments, tabBar)
		}
		segments = append(segments, left)
		if finderBar != "" {
			segments = append(segments, finderBar)
		}
		segments = append(segments, statusBar)
		return lipgloss.JoinVertical(lipgloss.Left, segments...)
	}

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
	}

	bodyH := m.height - 2
	if tabBar != "" {
		bodyH--
	}
	if fh := m.finder.Height(); fh > 0 {
		bodyH -= fh
	}
	bar := lipgloss.NewStyle().Foreground(t.Border).Render(strings.Repeat("│\n", bodyH))
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, bar, right)

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
// welcome card needs to know this so it can self-clamp.
func (m model) editorSize() (int, int) {
	leftW := m.width
	if m.right != rightNone {
		rightW := m.width / 3
		if rightW < 40 {
			rightW = 40
		}
		if rightW > m.width-40 {
			rightW = m.width - 40
		}
		leftW = m.width - rightW - 1
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
