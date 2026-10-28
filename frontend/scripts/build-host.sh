#!/usr/bin/env bash
# Builds the web bundle, copies it into the host module and links the window.
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
exec "$here/../scripts/build-window.sh" "$here/build/hikari-ui"
