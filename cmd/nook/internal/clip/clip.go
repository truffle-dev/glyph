// Package clip is nook's minimal clipboard. It keeps an in-process register
// so cut/copy/paste always works regardless of the host environment, and
// best-effort syncs to the OS clipboard by shelling out to whichever helper
// is available on the host (xclip / xsel / wl-copy under Linux, pbcopy under
// macOS, clip.exe under WSL or Windows). No CGo, no extra modules.
//
// The package is deliberately small. Callers go through Set / Get and never
// have to know whether the underlying tool worked — Set always updates the
// in-process register, and Get falls back to it when the OS clipboard is
// empty or unreadable. The register-first design keeps cut/copy/paste fully
// functional in headless containers (and in tests), and the OS sync makes
// "copy out of nook, paste into VS Code or Slack" work whenever the host
// has a clipboard tool installed.
package clip

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// register is the process-local clipboard. Always set when Set runs, always
// consulted as a fallback when Get fails. Guards against the race between
// the OS-clipboard write (which may take time on slow xclip startups) and
// an immediate paste.
var (
	mu       sync.RWMutex
	register string
)

// Set stores text in the in-process register and best-effort writes to the
// OS clipboard. The OS write runs synchronously with a short timeout so a
// missing helper or slow Wayland socket doesn't hang the editor; the
// register is always updated regardless.
func Set(text string) {
	mu.Lock()
	register = text
	mu.Unlock()
	syncToOS(text)
}

// Get returns the current clipboard text. Prefers the OS clipboard so an
// "external paste" (text copied from another application) appears; falls
// back to the in-process register when no helper is available or the OS
// clipboard is empty.
func Get() string {
	if osText, ok := readFromOS(); ok && osText != "" {
		return osText
	}
	mu.RLock()
	defer mu.RUnlock()
	return register
}

// Register returns whatever is in the process-local register. Exported
// primarily for tests that want to verify Set/Get round-trips without
// depending on whether the host has a clipboard helper.
func Register() string {
	mu.RLock()
	defer mu.RUnlock()
	return register
}

// writeCmd describes one platform's clipboard-write helper: the command and
// the args. Tools run with stdin = text.
type writeCmd struct {
	name string
	args []string
}

// readCmd is the read-side counterpart of writeCmd.
type readCmd struct {
	name string
	args []string
}

func writeCandidates() []writeCmd {
	switch runtime.GOOS {
	case "darwin":
		return []writeCmd{{name: "pbcopy"}}
	case "windows":
		return []writeCmd{{name: "clip"}}
	default:
		// Order: wl-copy (Wayland), xclip (X11), xsel (X11), then clip.exe
		// in case nook is running under WSL with a Windows interop PATH.
		return []writeCmd{
			{name: "wl-copy"},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
			{name: "clip.exe"},
		}
	}
}

func readCandidates() []readCmd {
	switch runtime.GOOS {
	case "darwin":
		return []readCmd{{name: "pbpaste"}}
	case "windows":
		// PowerShell's Get-Clipboard is the only reliable read; clip itself
		// is write-only. Skipped on Windows non-WSL hosts.
		return []readCmd{{name: "powershell.exe", args: []string{"-NoProfile", "-Command", "Get-Clipboard"}}}
	default:
		return []readCmd{
			{name: "wl-paste", args: []string{"--no-newline"}},
			{name: "xclip", args: []string{"-selection", "clipboard", "-o"}},
			{name: "xsel", args: []string{"--clipboard", "--output"}},
			{name: "powershell.exe", args: []string{"-NoProfile", "-Command", "Get-Clipboard"}},
		}
	}
}

func syncToOS(text string) {
	for _, c := range writeCandidates() {
		if _, err := exec.LookPath(c.name); err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		cmd := exec.CommandContext(ctx, c.name, c.args...)
		cmd.Stdin = strings.NewReader(text)
		err := cmd.Run()
		cancel()
		if err == nil {
			return
		}
	}
}

func readFromOS() (string, bool) {
	for _, c := range readCandidates() {
		if _, err := exec.LookPath(c.name); err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		out, err := exec.CommandContext(ctx, c.name, c.args...).Output()
		cancel()
		if err != nil {
			continue
		}
		return normalizeOSText(out), true
	}
	return "", false
}

// normalizeOSText converts clipboard read bytes into the form callers want to
// paste: CRLF lowered to LF, then exactly one trailing newline stripped.
// PowerShell's Get-Clipboard always appends CRLF, so without the trim a
// "INS"-shaped Set on Windows pastes as "INS\n" and splits the line. xclip
// and pbpaste preserve content as-is, so a deliberately-newline-terminated
// copy loses its trailing newline here — that matches wl-paste --no-newline
// semantics already used on Wayland, and matches how every IDE paste behaves.
func normalizeOSText(out []byte) string {
	s := strings.ReplaceAll(string(out), "\r\n", "\n")
	return strings.TrimSuffix(s, "\n")
}
