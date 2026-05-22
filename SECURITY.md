# Security Policy

## Supported versions

glyph is pre-1.0. The `main` branch is the only supported version. Released tags receive security fixes only as part of the next minor release; if you are on a tag, please update to the latest release for any security work.

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |
| <main   | :x:                |

## Reporting a vulnerability

Please report security vulnerabilities by emailing <security@truffleagent.com>. Do **not** open a public issue.

Include in your report:

- a description of the issue and the impact you believe it has
- the smallest reproducer you can produce (paths, inputs, commands, expected vs. actual behavior)
- the version or commit SHA you found it on
- any suggested fix or mitigation

You will get an acknowledgement within 72 hours and a more substantive reply within 7 days. If we accept the report, we will work with you on a coordinated disclosure timeline. Most fixes ship in the next release; the longest we will hold a fix is 90 days unless you ask for longer.

## Scope

In scope:

- the `glyph` CLI in `cmd/glyph/`
- the `glyph add` registry fetch path (manifest validation, file write paths, import rewrites)
- the registry artifacts under `r/`, including dependency resolution
- any component source in `components/<name>/` that is downloaded into a user's repo as part of `glyph add`

Out of scope:

- vulnerabilities in upstream Go modules (file those with the upstream project)
- vulnerabilities in [Bubble Tea](https://github.com/charmbracelet/bubbletea), [lipgloss](https://github.com/charmbracelet/lipgloss), or other Charm libraries (file those with Charm)
- the demo site at <https://truffleagent.com/glyph> — that ships from a separate repo

## What you can expect

We treat security reports as the highest-priority work in the queue. We will keep you in the loop until the fix is shipped, credit you in the release notes unless you ask not to be credited, and publish a brief postmortem if the issue is interesting enough that the community will learn from it.

Thank you for taking the time to make glyph safer.
