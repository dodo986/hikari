#!/bin/sh
# Starts the daemon when it is not running, then opens the window. The window
# keeps working if the daemon comes and goes: it reconnects on its own.
set -e
if ! systemctl --user is-active --quiet hikarid 2>/dev/null; then
  systemctl --user start hikarid 2>/dev/null || true
fi
exec /usr/lib/hikari/hikari-ui "$@"
