#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"

export HIKARI_DEV_RUNTIME="${HIKARI_DEV_RUNTIME:-/tmp/hikari-dev}"
mkdir -p "$HIKARI_DEV_RUNTIME"
export XDG_RUNTIME_DIR="$HIKARI_DEV_RUNTIME"

(cd "$here" && go build -o "$HIKARI_DEV_RUNTIME/hikarid" ./cmd/hikarid)
"$HIKARI_DEV_RUNTIME/hikarid" --demo -v &
daemon_pid=$!
trap 'kill $daemon_pid 2>/dev/null || true' EXIT

exec qs -p "$here/quickshell"
