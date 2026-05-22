# Contributing to glyph

Thanks for considering a contribution. The fastest path to a merge is a new component or a fix to an existing one. Bigger changes (new framework adapter, CLI feature) are welcome but should open an issue first so we can align on the shape.

## Setup

```bash
git clone https://github.com/truffle-dev/glyph
cd glyph
go test ./components/... ./cmd/... ./tools/...
```

Go 1.22+. No other prerequisites for the test suite.

## Repo layout

```
cmd/glyph/                # The CLI
components/<name>/        # One component per directory
  <name>.go               # The component
  <name>_test.go          # Tests
  <name>.json             # Registry manifest
  story/main.go           # Story (//go:build glyph_story)
tools/build/              # Flattens components/* into a static registry
schema/                   # JSON Schema for registry-item.json
docs/                     # Long-form docs (architecture, registry)
examples/                 # Runnable end-to-end demos (showcase, reel)
visuals/                  # vhs tapes + recorded casts/GIFs for demos
research/                 # Design notes (Phase 0 artifacts)
r/                        # Built registry output (gitignored, regenerated)
```

## Registry contract

Each component ships a single `<name>.json` manifest. `tools/build` reads every manifest under `components/`, validates it, copies the listed source files into `r/<name>/`, and writes a flat `r/<name>.json` with every file's URL embedded. The CLI consumer (`glyph add <name>`) fetches `r/<name>.json`, walks `registryDependencies`, downloads each file from its URL, and writes it into the alias-resolved target path in the consumer's repo. The schema is `schema/registry-item.json`.

Required fields:

- `name` — kebab-case slug. Matches the directory name. The install command.
- `type` — one of `glyph:component`, `glyph:theme`, `glyph:lib`. Used by the consumer to pick the install alias.
- `version` — semver. Bump on breaking source-level changes.
- `frame` — currently always `bubbletea`. Reserved for ratatui/textual/ink in v0.2+.
- `files` — array of file descriptors. Each has `path` (where it lives in this repo), `type` (`glyph:component` or `glyph:test`), and `target` (where it lands in the consumer's repo, written as `@components/<file>` or `@lib/<file>`; the alias resolves via the consumer's `glyph.json`).

Optional fields:

- `dependencies` — Go module imports the component pulls in (`github.com/foo/bar@v1.0.0`). `glyph add` runs `go get` for each. Use exact-version pins so consumers reproduce.
- `registryDependencies` — other glyph components this one depends on. The CLI walks this graph and installs each first. Examples: `chat-thread` lists `["chat-bubble"]`, every component lists `["theme"]`.
- `title`, `description`, `docs`, `categories`, `meta` — used by the demo site and the README's component table.

The contract: a manifest is correct when `go run ./tools/build` succeeds, the produced `r/<name>.json` round-trips through `cmd/glyph`'s install logic (`go test ./cmd/glyph -run Integration`), and the produced `r/<name>/<file>` URLs each resolve when served. The build and integration tests verify all three.

## Story files

Every component ships a story file at `components/<name>/story/main.go` with the `//go:build glyph_story` build tag. The tag keeps the story out of the default Go build and lets `go test ./components/...` ignore it. Stories are runnable via `go run -tags glyph_story ./components/<name>/story/`.

A good story does three things:

1. Renders the component in three to five canonical states (idle, hover, error, empty, etc.). One state per "scene" if the component is stateful.
2. Uses only constructors and `With...` builders. No I/O, no clocks, no random sources. The output should be byte-identical across runs.
3. Reads from `theme.Default` (never hardcoded colors), so a future `theme.Light` swap retones everything.

Stories drive the visual pipeline: `visuals/tapes/<name>.tape` records each story into a GIF for the component card on the demo site. The story is also the screenshot test fixture and the README's worked example.

## Adding a component

1. Copy `components/chat-bubble/` to `components/<your-name>/`.
2. Rename the files. Update the package name and the JSON manifest.
3. Implement the component. Keep the API in the `With...` builder shape (`New`, `WithSomething`, `View`).
4. Write tests. At minimum: the component renders, it respects width, key bindings work if applicable.
5. Write a story file under `story/main.go` with the `//go:build glyph_story` tag. The story should showcase three to five states a user is likely to encounter.
6. Run `go test ./components/<your-name>/` and `go build -tags glyph_story ./components/<your-name>/story/`. Both must pass.
7. Open a PR. Describe the component in two sentences. Link the demo states.

## Voice

Component docs, comments, PR descriptions, and commit messages share one voice:

- No emojis in source files, docs, or PR descriptions.
- No marketing phrases. Cut "seamless", "robust", "unlock", "supercharge", "blazingly fast", "game-changing".
- Short sentences. Concrete nouns.
- Standard sentence case in prose. Lowercase branding (`glyph`) in body text is fine.

## Tests

`go test ./components/...` is the gate for component changes. `go test ./cmd/...` is the gate for CLI changes. Both must be green before merge.

We don't snapshot-test the rendered terminal output. Tests assert behavior (correct value after Submit, scroll clamps, etc.). Visual regression is caught by the story files re-rendered to SVG and reviewed in the PR.

## Style

`gofmt -d ./...` must be clean. `go vet ./...` must be clean.

Prefer values over pointers for the public API. Builders return new structs (`(b Bubble) WithText(s string) Bubble`).

Components reference theme tokens via `theme.Theme`. No hardcoded colors in components. If a token you need doesn't exist, propose adding it to `components/theme/`.

## Commits and PRs

One change per commit when reasonable. Subject in imperative mood, under 72 chars. Reference an issue if one exists.

```
chat-bubble: tighten role-label alignment for narrow widths

The role label drifted right by one column when width < 40.
Anchor it to the bubble's left edge instead of the wrapper.

Closes #42.
```

PR titles follow the same shape as commit subjects. PR bodies explain the why and link the demo: which states changed in the story, which screenshots were re-rendered. Keep bodies short.

## Code of conduct

Participation in this project is governed by [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Reports go to truffleagent@gmail.com.

## License

By contributing you agree your work is licensed under the [MIT License](LICENSE).
