package configwatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestSnapshotEmptyPath(t *testing.T) {
	fp := Snapshot("")
	if fp.Exists {
		t.Errorf("empty path should not exist, got %+v", fp)
	}
}

func TestSnapshotMissingFile(t *testing.T) {
	dir := t.TempDir()
	fp := Snapshot(filepath.Join(dir, "nope.toml"))
	if fp.Exists {
		t.Errorf("missing file should not exist, got %+v", fp)
	}
}

func TestSnapshotExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.toml")
	writeFile(t, path, "tab_width = 4\n")

	fp := Snapshot(path)
	if !fp.Exists {
		t.Fatalf("file should exist, got %+v", fp)
	}
	if fp.Size != int64(len("tab_width = 4\n")) {
		t.Errorf("size = %d, want %d", fp.Size, len("tab_width = 4\n"))
	}
	if fp.ModTime.IsZero() {
		t.Errorf("modtime should be set")
	}
}

func TestTickMsgChangedSameFingerprintIsFalse(t *testing.T) {
	now := time.Now()
	fp := Fingerprint{ModTime: now, Size: 42, Exists: true}
	msg := TickMsg{Path: "/x", Last: fp, Cur: fp}
	if msg.Changed() {
		t.Errorf("identical fingerprints should not be Changed()")
	}
}

func TestTickMsgChangedSizeMoves(t *testing.T) {
	now := time.Now()
	last := Fingerprint{ModTime: now, Size: 42, Exists: true}
	cur := Fingerprint{ModTime: now, Size: 43, Exists: true}
	msg := TickMsg{Path: "/x", Last: last, Cur: cur}
	if !msg.Changed() {
		t.Errorf("size move should be Changed()")
	}
}

func TestTickMsgChangedModTimeMoves(t *testing.T) {
	now := time.Now()
	last := Fingerprint{ModTime: now, Size: 42, Exists: true}
	cur := Fingerprint{ModTime: now.Add(time.Second), Size: 42, Exists: true}
	msg := TickMsg{Path: "/x", Last: last, Cur: cur}
	if !msg.Changed() {
		t.Errorf("modtime move should be Changed()")
	}
}

func TestTickMsgChangedExistenceMoves(t *testing.T) {
	last := Fingerprint{Exists: false}
	cur := Fingerprint{ModTime: time.Now(), Size: 1, Exists: true}
	msg := TickMsg{Path: "/x", Last: last, Cur: cur}
	if !msg.Changed() {
		t.Errorf("file appearing should be Changed()")
	}

	// And the reverse: file disappearing.
	msg2 := TickMsg{Path: "/x", Last: cur, Cur: last}
	if !msg2.Changed() {
		t.Errorf("file disappearing should be Changed()")
	}
}

func TestWatchCmdEmptyPathReturnsNil(t *testing.T) {
	if cmd := WatchCmd("", Fingerprint{}); cmd != nil {
		t.Errorf("empty path should return nil cmd, got %v", cmd)
	}
}

func TestWatchCmdEmitsTickWithCurrentFingerprint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.toml")
	writeFile(t, path, "v = 1\n")

	last := Fingerprint{} // pretend we haven't seen anything yet

	cmd := watchAfter(path, last, 5*time.Millisecond)
	if cmd == nil {
		t.Fatal("watchAfter returned nil")
	}

	msg := cmd()
	tick, ok := msg.(TickMsg)
	if !ok {
		t.Fatalf("expected TickMsg, got %T", msg)
	}
	if tick.Path != path {
		t.Errorf("path = %q, want %q", tick.Path, path)
	}
	if tick.Last != last {
		t.Errorf("last = %+v, want %+v", tick.Last, last)
	}
	if !tick.Cur.Exists {
		t.Errorf("cur should exist, got %+v", tick.Cur)
	}
	if !tick.Changed() {
		t.Errorf("first tick after empty Last should be Changed()")
	}
}

func TestWatchCmdDetectsRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.toml")
	writeFile(t, path, "v = 1\n")

	last := Snapshot(path)

	// Rewrite the file with a bigger payload so the size moves even if
	// the filesystem rounds mtime to the second.
	time.Sleep(10 * time.Millisecond)
	writeFile(t, path, "v = 22222\n")

	cmd := watchAfter(path, last, 5*time.Millisecond)
	msg := cmd()
	tick := msg.(TickMsg)
	if !tick.Changed() {
		t.Errorf("rewrite should be Changed(); last=%+v cur=%+v", tick.Last, tick.Cur)
	}
}

func TestWatchCmdDetectsRemoval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.toml")
	writeFile(t, path, "v = 1\n")

	last := Snapshot(path)

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}

	cmd := watchAfter(path, last, 5*time.Millisecond)
	msg := cmd()
	tick := msg.(TickMsg)
	if !tick.Changed() {
		t.Errorf("removal should be Changed(); last=%+v cur=%+v", tick.Last, tick.Cur)
	}
	if tick.Cur.Exists {
		t.Errorf("cur.Exists should be false after removal")
	}
}

func TestWatchCmdSameContentNoChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.toml")
	writeFile(t, path, "v = 1\n")

	last := Snapshot(path)
	cmd := watchAfter(path, last, 5*time.Millisecond)
	msg := cmd()
	tick := msg.(TickMsg)
	if tick.Changed() {
		t.Errorf("untouched file should not be Changed(); last=%+v cur=%+v", tick.Last, tick.Cur)
	}
}

// Sanity: WatchCmd returns the tea.Cmd shape the bubbletea pump expects.
func TestWatchCmdReturnsTeaCmd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.toml")
	writeFile(t, path, "x\n")

	var cmd tea.Cmd = watchAfter(path, Fingerprint{}, time.Millisecond)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd")
	}
}
