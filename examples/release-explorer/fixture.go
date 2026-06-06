package main

// fixtureReleases returns a synthetic dataset modeled on glyph's own
// release history. The release tags, dates, and body shapes are realistic
// so the surfaces (list, tabs, markdown viewer, status bar) stay legible
// while the demo runs without network access or `gh` authentication.
func fixtureReleases() []release {
	return []release{
		{
			tag:         "v0.47.0",
			name:        "v0.47.0 - data-and-display tier",
			publishedAt: "2026-06-06T08:11:37Z",
			body: `## Highlights

Seven new components close the data-and-display tier.

- **sparkline-chart** - single-line unicode-block mini-chart, pin axes with WithMin/WithMax.
- **pagination-bar** - page x of y plus item count.
- **accordion** - vertical collapsible sections.
- **json-tree-view** - collapsible JSON browser.
- **tree-view** - generic recursive tree with cursor.
- **timeline** - vertical event log with relative time.
- **table-virtualized** - O(visible) table for large row sets.

## Notes

The pure-render primitives (sparkline) stay value-typed.
The keyboard-driven primitives (tree, accordion, table) are tea.Model
with explicit Msg types for selection and motion.

table-virtualized ships alongside the existing table component, so the
in-memory and O(visible) shapes are both available without either being
load-bearing for the other.`,
			prerelease: false,
			assets: []asset{
				{name: "glyph_0.47.0_linux_amd64.tar.gz", size: "3.1 MB"},
				{name: "glyph_0.47.0_linux_arm64.tar.gz", size: "3.0 MB"},
				{name: "glyph_0.47.0_darwin_amd64.tar.gz", size: "3.2 MB"},
				{name: "glyph_0.47.0_darwin_arm64.tar.gz", size: "3.1 MB"},
				{name: "glyph_0.47.0_windows_amd64.zip", size: "3.3 MB"},
				{name: "checksums.txt", size: "528 B"},
			},
		},
		{
			tag:         "v0.46.0",
			name:        "v0.46.0 - feedback tier",
			publishedAt: "2026-06-04T19:02:14Z",
			body: `## Highlights

Three feedback components ship together.

- **notification-toast** - timed dismissible message overlay.
- **progress-bar** - bordered determinate progress with label.
- **spinner** - braille-dotted indeterminate spinner.

## Notes

All three accept WithTheme for color consistency across the app.
The toast component uses a tea.Cmd timer for auto-dismiss so the
parent model never has to track the deadline.`,
			prerelease: false,
			assets: []asset{
				{name: "glyph_0.46.0_linux_amd64.tar.gz", size: "2.9 MB"},
				{name: "glyph_0.46.0_darwin_arm64.tar.gz", size: "3.0 MB"},
				{name: "checksums.txt", size: "402 B"},
			},
		},
		{
			tag:         "v0.45.0",
			name:        "v0.45.0 - chat composition",
			publishedAt: "2026-06-02T11:48:09Z",
			body: `## Highlights

Three chat components compose into a full conversation surface.

- **chat-bubble** - role-styled message bubble.
- **chat-input** - multi-line input with submit handling.
- **chat-thread** - scrolling history of bubbles.

## Notes

chat-bubble accepts a role string the theme maps to a color, so apps
extend roles without subclassing. chat-thread auto-scrolls to the
latest message; pgup pauses follow-mode until the cursor returns.`,
			prerelease: false,
			assets: []asset{
				{name: "glyph_0.45.0_linux_amd64.tar.gz", size: "2.7 MB"},
				{name: "glyph_0.45.0_darwin_arm64.tar.gz", size: "2.8 MB"},
				{name: "checksums.txt", size: "402 B"},
			},
		},
		{
			tag:         "v0.44.0",
			name:        "v0.44.0 - editor primitives",
			publishedAt: "2026-05-30T22:15:51Z",
			body: `## Highlights

Two editor primitives close the text-input tier.

- **editor** - multi-line text editor with line numbers.
- **code-view** - read-only code block with syntax highlighting.

## Notes

The editor is intentionally narrow: cursor motion, line-aware
backspace, and an OnChange hook. Syntax highlighting lives in
code-view, which is a separate primitive so apps don't pay the cost
when only an editable surface is wanted.`,
			prerelease: false,
			assets: []asset{
				{name: "glyph_0.44.0_linux_amd64.tar.gz", size: "2.5 MB"},
				{name: "checksums.txt", size: "268 B"},
			},
		},
		{
			tag:         "v0.43.0",
			name:        "v0.43.0 - navigation tier",
			publishedAt: "2026-05-27T17:30:22Z",
			body: `## Highlights

Three navigation primitives ship.

- **breadcrumb** - separator-joined path with the last segment highlighted.
- **command-palette** - filterable command list invoked from a single keybind.
- **kbd** - keystroke-styled inline labels.

## Notes

command-palette ships as a tea.Model and never owns the binding that
opens it; the app decides whether ctrl+k or f2 raises the palette.`,
			prerelease: false,
			assets: []asset{
				{name: "glyph_0.43.0_linux_amd64.tar.gz", size: "2.4 MB"},
				{name: "checksums.txt", size: "268 B"},
			},
		},
		{
			tag:         "v0.42.0",
			name:        "v0.42.0 - form tier",
			publishedAt: "2026-05-25T14:02:18Z",
			body: `## Highlights

Three form primitives close the input tier.

- **text-input** - single-line input with placeholder and focus.
- **select** - dropdown of labeled options.
- **confirmation** - modal yes/no with default focus.

## Notes

select returns its choice via a SelectedMsg the parent listens for; the
component doesn't carry parent state.`,
			prerelease: false,
			assets: []asset{
				{name: "glyph_0.42.0_linux_amd64.tar.gz", size: "2.2 MB"},
				{name: "checksums.txt", size: "268 B"},
			},
		},
		{
			tag:         "v0.41.0-beta.1",
			name:        "v0.41.0-beta.1 - modal scaffolding",
			publishedAt: "2026-05-23T09:14:33Z",
			body: `## Beta notes

Initial **modal** primitive lands behind an opt-in import. API may change
before v0.41.0 final. Reports welcome.`,
			prerelease: true,
			assets: []asset{
				{name: "glyph_0.41.0-beta.1_linux_amd64.tar.gz", size: "2.0 MB"},
			},
		},
	}
}
