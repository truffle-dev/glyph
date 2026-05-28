// Package configwatch polls a single file path for content changes and emits
// a tea.Msg every poll. The host inspects each message: when the file's
// fingerprint differs from the previously-seen one, the config has been
// rewritten and the host reloads.
//
// Polling instead of fsnotify is a deliberate choice. The config file is one
// small TOML file that changes at most a few times per session — usually
// zero. A 1s poll cadence is well below human reload-perception threshold
// ("save and alt-tab back" feels live) and sidesteps the inotify /
// FSEvents / ReadDirectoryChangesW platform surface, plus the
// atomic-rename-on-save dance (vim, helix, kakoune) that fsnotify watchers
// have to special-case. No new direct dependency.
//
// Fingerprint deliberately includes size as well as mtime: editors that
// rewrite within a single second can produce same-mtime revisions on
// filesystems with coarse mtime granularity (ext4 with relatime, NFS), and
// size moves catch those.
package configwatch

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Interval is the default poll cadence.
const Interval = 1 * time.Second

// Fingerprint identifies a file revision. Two fingerprints compare equal iff
// the file has not been rewritten since the older was taken.
type Fingerprint struct {
	ModTime time.Time
	Size    int64
	Exists  bool
}

// Snapshot returns the current fingerprint of path. A non-existent file
// returns Fingerprint{Exists: false} — that is itself a valid distinguishing
// state (config file appearing / disappearing is a reloadable change).
func Snapshot(path string) Fingerprint {
	if path == "" {
		return Fingerprint{}
	}
	fi, err := os.Stat(path)
	if err != nil {
		return Fingerprint{Exists: false}
	}
	return Fingerprint{ModTime: fi.ModTime(), Size: fi.Size(), Exists: true}
}

// TickMsg is delivered every Interval. The host compares Cur to Last via
// Changed(); whether or not the file moved, the host should fire WatchCmd
// again with Cur as the new Last to continue the poll loop.
type TickMsg struct {
	Path string
	Last Fingerprint
	Cur  Fingerprint
}

// Changed reports whether the file's fingerprint differs from the previous
// poll. The host uses this to gate config reload.
func (m TickMsg) Changed() bool { return m.Cur != m.Last }

// WatchCmd returns a tea.Cmd that sleeps for Interval and emits a TickMsg
// carrying the current fingerprint. The Cmd does not re-fire itself; the
// host wires WatchCmd(path, msg.Cur) on every received TickMsg, which keeps
// the loop alive without spawning a daemon goroutine on Init.
//
// A nil/empty path returns nil — the host treats "no config path" as
// "nothing to watch."
func WatchCmd(path string, last Fingerprint) tea.Cmd {
	return watchAfter(path, last, Interval)
}

// watchAfter is the testable form of WatchCmd. Exposed package-private so
// the tests can run with a millisecond interval.
func watchAfter(path string, last Fingerprint, interval time.Duration) tea.Cmd {
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		time.Sleep(interval)
		return TickMsg{Path: path, Last: last, Cur: Snapshot(path)}
	}
}
