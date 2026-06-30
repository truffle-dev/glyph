// Package bufman owns the open-buffer collection. The editor.Pane stays a
// per-document leaf; bufman is the list. Pointer receiver so tab switches
// are atomic against the host's routeKey reads — copying the manager would
// risk "I switched tabs but my mutation went to the old copy" bugs.
package bufman

import (
	"github.com/truffle-dev/glyph/cmd/nook/internal/editor"
	"github.com/truffle-dev/glyph/cmd/nook/internal/highlight"
	"github.com/truffle-dev/glyph/components/theme"
)

// Manager holds the list of open buffers and the active index.
type Manager struct {
	theme  theme.Theme
	panes  []editor.Pane
	active int // -1 when empty
	width  int
	height int

	// highlighter is shared across panes. nil disables highlighting.
	highlighter highlight.Highlighter

	// tabWidth, lineNumbers, indentGuides, and softWrap are editor settings
	// forwarded to every pane. Storing them on the manager keeps newly-opened
	// buffers consistent with the current configuration even after a runtime
	// reload.
	tabWidth     int
	lineNumbers  bool
	indentGuides bool
	softWrap     bool
}

// New constructs an empty manager.
func New(t theme.Theme) *Manager {
	return &Manager{theme: t, active: -1, tabWidth: 4, lineNumbers: true, indentGuides: true}
}

// WithHighlighter sets the syntax highlighter applied to every new pane.
// Existing open panes are also rewired so theme/highlighter changes take
// effect on the next render.
func (m *Manager) WithHighlighter(h highlight.Highlighter) *Manager {
	m.highlighter = h
	for i := range m.panes {
		m.panes[i] = m.panes[i].WithHighlighter(h)
	}
	return m
}

// WithTabWidth sets the rendered tab expansion for every open pane and any
// pane opened later. Values <= 0 clamp to 4.
func (m *Manager) WithTabWidth(n int) *Manager {
	if n <= 0 {
		n = 4
	}
	m.tabWidth = n
	for i := range m.panes {
		m.panes[i] = m.panes[i].SetTabWidth(n)
	}
	return m
}

// WithLineNumbers toggles the row-number gutter for every open pane and any
// pane opened later.
func (m *Manager) WithLineNumbers(b bool) *Manager {
	m.lineNumbers = b
	for i := range m.panes {
		m.panes[i] = m.panes[i].SetLineNumbers(b)
	}
	return m
}

// WithIndentGuides toggles the indent-guide overlay for every open pane and
// any pane opened later.
func (m *Manager) WithIndentGuides(b bool) *Manager {
	m.indentGuides = b
	for i := range m.panes {
		m.panes[i] = m.panes[i].SetIndentGuides(b)
	}
	return m
}

// WithSoftWrap toggles soft wrap (wrap long lines onto multiple visual rows)
// for every open pane and any pane opened later.
func (m *Manager) WithSoftWrap(b bool) *Manager {
	m.softWrap = b
	for i := range m.panes {
		m.panes[i] = m.panes[i].SetSoftWrap(b)
	}
	return m
}

// SetTheme swaps the palette stored on the manager and propagates it to
// every open pane. Called from the host's live-reload path when the user
// changes the theme name in config.toml — the next render uses the new
// colors without restart.
func (m *Manager) SetTheme(t theme.Theme) *Manager {
	m.theme = t
	for i := range m.panes {
		m.panes[i] = m.panes[i].SetTheme(t)
	}
	return m
}

// Count returns the number of open buffers.
func (m *Manager) Count() int { return len(m.panes) }

// ActiveIndex returns the index of the active buffer, or -1 if none.
func (m *Manager) ActiveIndex() int { return m.active }

// Active returns a pointer to the active buffer for in-place replacement, or
// nil if there are no open buffers. Callers should not retain the pointer
// past the next Open/Close/Switch call — the underlying slice may move.
func (m *Manager) Active() *editor.Pane {
	if m.active < 0 || m.active >= len(m.panes) {
		return nil
	}
	return &m.panes[m.active]
}

// At returns a pointer to the buffer at idx, or nil if idx is out of range.
func (m *Manager) At(idx int) *editor.Pane {
	if idx < 0 || idx >= len(m.panes) {
		return nil
	}
	return &m.panes[idx]
}

// Find returns the index of the buffer whose path matches abs, or -1.
func (m *Manager) Find(abs string) int {
	for i, p := range m.panes {
		if p.Path() == abs {
			return i
		}
	}
	return -1
}

// OpenAction names what OpenOrSwitch did so the host can update status text
// and decide whether to fire LSP didOpen.
type OpenAction int

const (
	// Switched means a buffer for this path was already open; the active
	// index moved to it.
	Switched OpenAction = iota
	// ReplacedWelcome means the active buffer was a path-less placeholder
	// and was replaced in-place by the freshly opened file.
	ReplacedWelcome
	// OpenedNew means a brand-new buffer was appended and is now active.
	OpenedNew
)

// OpenOrSwitch is the picker/search routing primitive. Three behaviors:
//   - If a buffer with abs is already open, switch to it.
//   - Else if there's no active buffer or the active buffer is empty (no
//     path, no dirty), replace it in-place so the very first ctrl+P
//     doesn't leave a phantom welcome tab.
//   - Else append a new buffer and switch to it.
func (m *Manager) OpenOrSwitch(abs string) (int, OpenAction) {
	if i := m.Find(abs); i >= 0 {
		m.Switch(i)
		return i, Switched
	}
	p := editor.NewPane(m.theme).WithHighlighter(m.highlighter).WithSize(m.width, m.height).SetTabWidth(m.tabWidth).SetLineNumbers(m.lineNumbers).SetIndentGuides(m.indentGuides).SetSoftWrap(m.softWrap).Open(abs).Focus()
	if m.active >= 0 {
		cur := m.panes[m.active]
		if cur.Path() == "" && !cur.Dirty() {
			m.panes[m.active] = p
			return m.active, ReplacedWelcome
		}
	}
	if len(m.panes) == 0 {
		m.panes = append(m.panes, p)
		m.active = 0
		return 0, OpenedNew
	}
	m.panes = append(m.panes, p)
	m.active = len(m.panes) - 1
	// Blur the previously-active pane.
	if prev := m.active - 1; prev >= 0 {
		m.panes[prev] = m.panes[prev].Blur().SetGhostText("")
	}
	return m.active, OpenedNew
}

// Close removes the buffer at idx. Returns the path that was closed (so the
// caller can release LSP state) or "" if idx was out of range. The active
// index shifts left to stay in bounds; if the last buffer closes, active
// becomes -1.
func (m *Manager) Close(idx int) string {
	if idx < 0 || idx >= len(m.panes) {
		return ""
	}
	path := m.panes[idx].Path()
	m.panes = append(m.panes[:idx], m.panes[idx+1:]...)
	switch {
	case len(m.panes) == 0:
		m.active = -1
	case idx < m.active:
		m.active--
	case idx == m.active:
		if m.active >= len(m.panes) {
			m.active = len(m.panes) - 1
		}
	}
	if m.active >= 0 {
		m.panes[m.active] = m.panes[m.active].Focus()
	}
	return path
}

// CloseActive closes the active buffer.
func (m *Manager) CloseActive() string {
	return m.Close(m.active)
}

// Switch sets the active buffer to idx. Blurs the previously-active pane and
// clears its ghost-text so a stale completion doesn't linger when the user
// returns to that buffer.
func (m *Manager) Switch(idx int) {
	if idx < 0 || idx >= len(m.panes) || idx == m.active {
		// Idempotent. Focus the active pane in case it was blurred.
		if idx == m.active && m.active >= 0 {
			m.panes[m.active] = m.panes[m.active].Focus()
		}
		return
	}
	if m.active >= 0 {
		m.panes[m.active] = m.panes[m.active].Blur().SetGhostText("")
	}
	m.active = idx
	m.panes[idx] = m.panes[idx].Focus()
}

// Next switches to the buffer after the active one, wrapping at the end.
func (m *Manager) Next() {
	if len(m.panes) == 0 {
		return
	}
	m.Switch((m.active + 1) % len(m.panes))
}

// Prev switches to the buffer before the active one, wrapping at the start.
func (m *Manager) Prev() {
	if len(m.panes) == 0 {
		return
	}
	m.Switch((m.active - 1 + len(m.panes)) % len(m.panes))
}

// WithSize broadcasts the editor area dimensions to every pane.
func (m *Manager) WithSize(w, h int) {
	m.width = w
	m.height = h
	for i := range m.panes {
		m.panes[i] = m.panes[i].WithSize(w, h)
	}
}

// SetSizeAt sizes the single buffer at idx, leaving every other buffer
// untouched. Split panes need each on-screen buffer at its own pane
// rectangle, which the blanket WithSize cannot express. Out-of-range idx is
// a no-op so a stale pane binding can never panic.
func (m *Manager) SetSizeAt(idx, w, h int) {
	if idx < 0 || idx >= len(m.panes) {
		return
	}
	m.panes[idx] = m.panes[idx].WithSize(w, h)
}

// TabInfo is one entry in the tab bar.
type TabInfo struct {
	Path  string
	Dirty bool
}

// Tabs returns a snapshot of the open buffers for the tabbar renderer.
func (m *Manager) Tabs() []TabInfo {
	out := make([]TabInfo, len(m.panes))
	for i, p := range m.panes {
		out[i] = TabInfo{Path: p.Path(), Dirty: p.Dirty()}
	}
	return out
}

// RefreshIfOpen reloads the buffer at abs from disk if it's currently open.
// Used by the composer after writing an edit so the on-screen buffer reflects
// the new contents. Returns true if a buffer was refreshed.
func (m *Manager) RefreshIfOpen(abs string) bool {
	i := m.Find(abs)
	if i < 0 {
		return false
	}
	m.panes[i] = m.panes[i].Open(abs)
	if i == m.active {
		m.panes[i] = m.panes[i].Focus()
	}
	return true
}

// HasDirty reports whether any open buffer has unsaved changes.
func (m *Manager) HasDirty() bool {
	for _, p := range m.panes {
		if p.Dirty() {
			return true
		}
	}
	return false
}
