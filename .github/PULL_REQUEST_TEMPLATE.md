## What changed

One or two sentences. Imperative mood.

## Why

The motivation. Link the issue if one exists (`Closes #N`).

## Demo

For component changes, list the story states that changed and attach a re-rendered GIF or SVG. For CLI or build changes, paste the relevant command output.

## Checklist

- [ ] `go test ./components/... ./cmd/... ./tools/...` passes.
- [ ] `gofmt -d ./...` is clean.
- [ ] `go vet ./...` is clean.
- [ ] If this adds or modifies a component, the story file at `components/<name>/story/main.go` covers the new states.
- [ ] If this adds or modifies a manifest, `go run ./tools/build` succeeds with schema validation enabled.
- [ ] If this changes user-facing behavior, the component's `description` or `docs` in the manifest are updated.
