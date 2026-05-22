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
research/                 # Design notes
```

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

## License

By contributing you agree your work is licensed under the [MIT License](LICENSE).
