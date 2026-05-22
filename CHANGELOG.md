# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Nothing yet.

## [0.1.1] — 2026-05-22

Patch release. No CLI or component code changed; this release exercises the
new goreleaser pipeline so binaries are attached to the GitHub release for
the first time, and rounds out the docs surface.

### Added

- Pre-built binaries for linux, macos, and windows on amd64 and arm64,
  attached to the GitHub release as tarballs (or zip on windows), with a
  `checksums.txt` covering every archive.
- `goreleaser` config and a tag-triggered release workflow.
- Per-component `README.md` in every `components/<name>/` dir: preview GIF,
  install command, hello-world snippet matching the landing-page card, API
  surface, dependencies, and notes pulled from the JSON manifest.
- `glyph.schema.json` describing the consumer-side `glyph.json` config.
- `SECURITY.md` with a vulnerability disclosure policy.
- Dependabot config for weekly Go-module and GitHub-Actions updates.

### Changed

- `cmd/glyph`: `version` is now a mutable `var` so goreleaser can inject the
  release tag via `-ldflags "-X main.version=..."`. Source builds keep the
  `-dev` suffix.

## [0.1.0] — 2026-05-22

The first public release. Sixteen Bubble Tea components, a CLI, a static
registry, and a demo site.

### Added

- `cmd/glyph` CLI with `init`, `add`, `list`, and `version` subcommands.
- Static registry under `r/` describing every component as a JSON manifest
  with file list, dependencies, and import-rewrite rules.
- Sixteen v0.1 components, each with a runnable `story/` example and tests:
  `theme`, `chat-bubble`, `chat-input`, `chat-thread`, `command-palette`,
  `markdown-viewer`, `log-stream`, `diff-view`, `notification-toast`,
  `status-bar`, `spinner`, `tabs`, `panel`, `list`, `progress-bar`,
  `key-hints`.
- `examples/showcase`, a single-binary TUI demo composing the seven main
  surfaces into one application with tabs, a status bar, and a toast tray.
- `examples/reel`, a recorder-driven self-playing reel binary that produces
  `visuals/out/reel.gif` for the README and landing page.
- Test, vet, lint, and build CI across ubuntu, macos, and windows on every
  push and pull request.
- Issue templates for bug reports and component requests. PR template.
  `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`, `LICENSE` (MIT).
- Per-component animated GIFs under `visuals/out/<name>.gif` and a Gallery
  section in the README.
- Dependabot configuration for weekly Go-module and GitHub-Actions updates.

### Notes

- `glyph -version` and `glyph --version` both work, alongside `glyph -v` and
  `glyph version`. The CLI embeds VCS info via `runtime/debug.ReadBuildInfo`
  so `go install` builds carry a commit SHA in the version string.
- The registry contract is stable as of v0.1.0. The catalog grows; the
  shape of `r/<component>.json` does not break.

[Unreleased]: https://github.com/truffle-dev/glyph/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/truffle-dev/glyph/releases/tag/v0.1.1
[0.1.0]: https://github.com/truffle-dev/glyph/releases/tag/v0.1.0
