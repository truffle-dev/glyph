// Package spinner renders a small animated glyph next to an optional
// label, the way an agent UI signals "working on it". A spinner cycles
// through a fixed list of frames on a fixed interval and stops when
// the parent model removes it from the View.
//
// Spinners are stateless from the caller's perspective: each Update
// returns a new Spinner with the next frame, and Init returns the
// first tick command that drives the animation forward.
package spinner

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/truffle-dev/glyph/components/theme"
)

// Style enumerates the built-in animation styles.
type Style int

const (
	// StyleDots is a ten-frame braille rotator. The shadcn-ish default.
	StyleDots Style = iota
	// StyleLine is the classic - \ | / line spinner.
	StyleLine
	// StyleArc cycles a quarter-arc around a center point.
	StyleArc
	// StylePulse fades through four block characters.
	StylePulse
	// StyleBounce moves a single dot up and down.
	StyleBounce
)

// preset holds the frames and default interval for one Style.
type preset struct {
	frames   []string
	interval time.Duration
}

var presets = map[Style]preset{
	StyleDots:   {frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}, interval: 80 * time.Millisecond},
	StyleLine:   {frames: []string{"-", "\\", "|", "/"}, interval: 100 * time.Millisecond},
	StyleArc:    {frames: []string{"◜", "◠", "◝", "◞", "◡", "◟"}, interval: 100 * time.Millisecond},
	StylePulse:  {frames: []string{"█", "▓", "▒", "░"}, interval: 120 * time.Millisecond},
	StyleBounce: {frames: []string{"⠁", "⠂", "⠄", "⠂"}, interval: 100 * time.Millisecond},
}

// TickMsg is the per-frame advance message. Each Spinner produces its
// own TickMsg via Init and Update; parent models that compose multiple
// spinners can differentiate by checking the ID field.
type TickMsg struct {
	ID   string
	Time time.Time
}

// Spinner is an animated indicator with an optional label.
type Spinner struct {
	id       string
	theme    theme.Theme
	style    Style
	frames   []string
	interval time.Duration
	frame    int
	label    string
	color    lipgloss.Color
}

// New constructs a Spinner using StyleDots and the theme's Primary color.
func New(t theme.Theme) Spinner {
	p := presets[StyleDots]
	return Spinner{
		theme:    t,
		style:    StyleDots,
		frames:   p.frames,
		interval: p.interval,
		color:    t.Primary,
	}
}

// WithStyle picks one of the built-in frame sets and resets the
// interval to that style's default cadence.
func (s Spinner) WithStyle(style Style) Spinner {
	p, ok := presets[style]
	if !ok {
		return s
	}
	s.style = style
	s.frames = p.frames
	s.interval = p.interval
	if s.frame >= len(s.frames) {
		s.frame = 0
	}
	return s
}

// WithLabel sets text rendered after the spinner glyph. Pass "" to clear.
func (s Spinner) WithLabel(label string) Spinner { s.label = label; return s }

// WithInterval overrides the per-frame interval. Minimum 16ms.
func (s Spinner) WithInterval(d time.Duration) Spinner {
	if d < 16*time.Millisecond {
		d = 16 * time.Millisecond
	}
	s.interval = d
	return s
}

// WithColor overrides the foreground color of the glyph.
func (s Spinner) WithColor(c lipgloss.Color) Spinner { s.color = c; return s }

// WithID tags this spinner so a parent model with multiple spinners
// can route TickMsg messages by ID.
func (s Spinner) WithID(id string) Spinner { s.id = id; return s }

// Init returns the first tick command. Subsequent ticks are scheduled
// by Update as long as the parent forwards TickMsg.
func (s Spinner) Init() tea.Cmd { return s.tick() }

// Update advances the frame when it receives this Spinner's TickMsg.
// All other messages pass through unchanged.
func (s Spinner) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	tick, ok := msg.(TickMsg)
	if !ok {
		return s, nil
	}
	if tick.ID != s.id {
		return s, nil
	}
	s.frame = (s.frame + 1) % len(s.frames)
	return s, s.tick()
}

// View renders the current frame and optional label.
func (s Spinner) View() string {
	glyph := lipgloss.NewStyle().Foreground(s.color).Render(s.frames[s.frame])
	if s.label == "" {
		return glyph
	}
	label := lipgloss.NewStyle().Foreground(s.theme.Text).Render(s.label)
	return glyph + " " + label
}

// Frame returns the current frame index.
func (s Spinner) Frame() int { return s.frame }

func (s Spinner) tick() tea.Cmd {
	id := s.id
	return tea.Tick(s.interval, func(t time.Time) tea.Msg {
		return TickMsg{ID: id, Time: t}
	})
}
