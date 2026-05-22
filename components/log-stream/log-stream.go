// Package logstream renders a bounded, scrollable log view. Entries are
// color-coded by level. The view sticks to the tail like `tail -f` unless
// the user scrolls up; new entries appended while scrolled up do not
// jump the viewport.
package logstream

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"

	"github.com/truffle-dev/glyph/components/theme"
)

// Level is the severity of a log entry. Higher values are more severe.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the four-char label rendered in the level column.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERRO"
	}
	return "????"
}

// Entry is one log line.
type Entry struct {
	Time    time.Time
	Level   Level
	Source  string // optional component / module name
	Message string
}

// Stream is a Bubble Tea model that renders a windowed view of entries.
type Stream struct {
	theme      theme.Theme
	entries    []Entry
	capacity   int
	width      int
	height     int
	offset     int // lines-from-bottom; 0 = at tail
	minLevel   Level
	showTime   bool
	timeFormat string
}

// New constructs a Stream with capacity 1000, INFO minimum level, and
// timestamps enabled.
func New(t theme.Theme) Stream {
	return Stream{
		theme:      t,
		capacity:   1000,
		width:      80,
		height:     12,
		minLevel:   LevelInfo,
		showTime:   true,
		timeFormat: "15:04:05",
	}
}

// WithCapacity caps the ring buffer. When exceeded, oldest entries drop.
// Minimum capacity is 1.
func (s Stream) WithCapacity(n int) Stream {
	if n < 1 {
		n = 1
	}
	s.capacity = n
	if len(s.entries) > n {
		s.entries = s.entries[len(s.entries)-n:]
	}
	return s
}

// WithSize sets the rendered width and visible-row height.
func (s Stream) WithSize(w, h int) Stream {
	if w < 30 {
		w = 30
	}
	if h < 3 {
		h = 3
	}
	s.width = w
	s.height = h
	return s
}

// WithMinLevel drops entries below the given severity from the view.
// The underlying ring is unchanged.
func (s Stream) WithMinLevel(l Level) Stream { s.minLevel = l; return s }

// WithTimestamps toggles the leading timestamp column.
func (s Stream) WithTimestamps(show bool) Stream { s.showTime = show; return s }

// WithTimeFormat overrides the timestamp format. Default is "15:04:05".
func (s Stream) WithTimeFormat(f string) Stream { s.timeFormat = f; return s }

// Append adds an entry. If the buffer exceeds capacity, the oldest drops.
// If the viewport is at the tail (offset == 0), it stays at the tail.
// Otherwise the offset is preserved so the user sees the same content.
func (s Stream) Append(e Entry) Stream {
	s.entries = append(s.entries, e)
	if len(s.entries) > s.capacity {
		s.entries = s.entries[len(s.entries)-s.capacity:]
	}
	return s
}

// Clear empties the buffer and resets the offset to the tail.
func (s Stream) Clear() Stream {
	s.entries = nil
	s.offset = 0
	return s
}

// Entries returns the current buffer (copy).
func (s Stream) Entries() []Entry {
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

// Offset returns the current scroll offset, in lines from the tail.
func (s Stream) Offset() int { return s.offset }

// Init implements tea.Model.
func (s Stream) Init() tea.Cmd { return nil }

// Update handles scroll keys. Up/Down moves one line, PgUp/PgDn moves a
// window, Home jumps to the top of buffered history, End returns to tail.
func (s Stream) Update(msg tea.Msg) (Stream, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	max := s.maxOffset()
	switch key.Type {
	case tea.KeyUp:
		s.offset++
	case tea.KeyDown:
		s.offset--
	case tea.KeyPgUp:
		s.offset += s.height
	case tea.KeyPgDown:
		s.offset -= s.height
	case tea.KeyHome:
		s.offset = max
	case tea.KeyEnd:
		s.offset = 0
	}
	if s.offset < 0 {
		s.offset = 0
	}
	if s.offset > max {
		s.offset = max
	}
	return s, nil
}

// View renders the visible window. Lines below the tail are blank-padded
// to preserve the configured height.
func (s Stream) View() string {
	lines := s.renderLines()
	if len(lines) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(s.theme.TextMuted).
			Italic(true).
			Render("No log entries.")
		out := []string{empty}
		for len(out) < s.height {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	end := len(lines) - s.offset
	if end < 0 {
		end = 0
	}
	start := end - s.height
	if start < 0 {
		start = 0
	}
	visible := lines[start:end]
	for len(visible) < s.height {
		visible = append([]string{""}, visible...)
	}
	return strings.Join(visible, "\n")
}

// TotalLines is the count of post-wrap lines currently in the buffer
// after level filtering.
func (s Stream) TotalLines() int { return len(s.renderLines()) }

func (s Stream) maxOffset() int {
	total := s.TotalLines()
	if total <= s.height {
		return 0
	}
	return total - s.height
}

func (s Stream) renderLines() []string {
	if len(s.entries) == 0 {
		return nil
	}
	var out []string
	timeCol := 0
	if s.showTime {
		timeCol = len(time.Time{}.Format(s.timeFormat)) + 1
	}
	levelCol := 5 // "DBUG " etc

	for _, e := range s.entries {
		if e.Level < s.minLevel {
			continue
		}
		levelStyle := s.styleFor(e.Level)
		var prefix strings.Builder
		if s.showTime {
			prefix.WriteString(lipgloss.NewStyle().
				Foreground(s.theme.TextMuted).
				Render(e.Time.Format(s.timeFormat)))
			prefix.WriteString(" ")
		}
		prefix.WriteString(levelStyle.Render(e.Level.String()))
		prefix.WriteString(" ")
		if e.Source != "" {
			prefix.WriteString(lipgloss.NewStyle().
				Foreground(s.theme.PrimaryStrong).
				Render(e.Source))
			prefix.WriteString(": ")
		}
		bodyWidth := s.width - timeCol - levelCol - len(e.Source)
		if bodyWidth < 10 {
			bodyWidth = 10
		}
		wrapped := wordwrap.String(e.Message, bodyWidth)
		bodyLines := strings.Split(wrapped, "\n")
		out = append(out, prefix.String()+bodyLines[0])
		// Continuation lines align under the body column.
		indent := strings.Repeat(" ", timeCol+levelCol+continuationIndent(e.Source))
		for _, bl := range bodyLines[1:] {
			out = append(out, indent+bl)
		}
	}
	return out
}

func continuationIndent(source string) int {
	if source == "" {
		return 0
	}
	return len(source) + 2 // "source: "
}

func (s Stream) styleFor(l Level) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true)
	switch l {
	case LevelDebug:
		return base.Foreground(s.theme.TextMuted)
	case LevelInfo:
		return base.Foreground(s.theme.Info)
	case LevelWarn:
		return base.Foreground(s.theme.Warning)
	case LevelError:
		return base.Foreground(s.theme.Error)
	}
	return base.Foreground(s.theme.Text)
}
