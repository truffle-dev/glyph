#!/usr/bin/env bash
# Render every tape under tapes/ into out/<name>.gif. Requires vhs:
# https://github.com/charmbracelet/vhs
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p visuals/out

for tape in visuals/tapes/*.tape; do
  name=$(basename "$tape" .tape)
  echo "==> rendering $name"
  vhs "$tape"
done

echo "done; gifs in visuals/out/"
