#!/usr/bin/env bash
# Builds the daemon and the window into frontend/build and opens the window
# against a runtime directory of its own, so a development run never touches
# the installed library. Point XDG_RUNTIME_DIR at the real one to use it.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo="$(cd "$here/.." && pwd)"
runtime="${HIKARI_UI_RUNTIME:-/tmp/hikari-ui-dev}"

mkdir -p "$runtime" "$here/build"
(cd "$repo" && go build -mod=vendor -o "$here/build/hikarid" ./cmd/hikarid)
bash "$here/scripts/build-host.sh"

export XDG_RUNTIME_DIR="$runtime"
if ! [ -S "$runtime/hikari.sock" ]; then
  "$here/build/hikarid" --demo &
  sleep 1
fi
exec "$here/build/hikari-ui"
