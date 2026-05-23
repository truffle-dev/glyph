#!/usr/bin/env bash
# Render every story under components/*/story/ into out/<name>.gif.
# Pipeline: go build -tags glyph_story -> asciinema rec -> agg.
#
# Why not vhs? vhs needs a headless Chrome to render terminal frames,
# which isn't always installable. asciinema (Python) plus agg (Rust
# single-binary) cover the same ground without a browser dep.
#
# Interactive Bubble Tea stories (tea.NewProgram) need a snap helper
# at components/<name>/story/snap.go with build tag glyph_snap so the
# binary prints View() once and exits. See README in this directory.

set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p visuals/out

COLS=${GLYPH_COLS:-96}
ROWS=${GLYPH_ROWS:-32}
THEME=${GLYPH_THEME:-monokai}
FONT_SIZE=${GLYPH_FONT_SIZE:-18}

for story in components/*/story; do
  name=$(basename "$(dirname "$story")")

  # Prefer snap (static, build tag glyph_snap) when present; fall back
  # to the default story binary for non-interactive demos.
  if [ -f "$story/snap.go" ]; then
    tag=glyph_snap
  else
    tag=glyph_story
  fi

  bin="/tmp/glyph-snap-${name}"
  cast="/tmp/glyph-snap-${name}.cast"
  gif="visuals/out/${name}.gif"

  echo "==> ${name} (${tag})"
  go build -tags "${tag}" -o "${bin}" "./${story}/"

  # TERM=screen-256color: termenv skips the OSC 11 background-color query
  # for screen/tmux, which would otherwise add a 5s startup delay to any
  # binary that imports bubbletea (its init() calls lipgloss.HasDarkBackground).
  TERM=screen-256color asciinema rec \
    --overwrite \
    --cols "${COLS}" --rows "${ROWS}" \
    -c "${bin}" "${cast}" >/dev/null 2>&1

  agg --theme "${THEME}" --font-size "${FONT_SIZE}" "${cast}" "${gif}" >/dev/null

  rm -f "${bin}" "${cast}"
done

echo "done; gifs in visuals/out/"
