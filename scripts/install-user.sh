#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
bin="${HIKARI_USER_BIN:-$HOME/.local/bin}"
data="${XDG_DATA_HOME:-$HOME/.local/share}"
mkdir -p "$bin"
(cd "$here" && go build -trimpath -o "$bin/hikarid" ./cmd/hikarid)
(cd "$here" && go build -trimpath -o "$bin/hikari" ./cmd/hikari)
"$here/scripts/build-backend.sh" "$data/hikari/amethyst_filegraph.abi3.so"
"$here/scripts/build-window.sh" "$bin/hikari-ui"
install -m755 "$here/scripts/hikari-app" "$bin/hikari-app"
install -m755 "$here/scripts/hikari-update" "$bin/hikari-update"
install -Dm644 "$here/packaging/hikari.svg" "$data/icons/hicolor/scalable/apps/hikari.svg"
install -Dm644 "$here/packaging/hikari.desktop" "$data/applications/hikari.desktop"
sed -i "s|^Exec=.*|Exec=$bin/hikari-app|" "$data/applications/hikari.desktop"
sed -i "s|^TryExec=.*|TryExec=$bin/hikari-app|" "$data/applications/hikari.desktop"
if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database "$data/applications" >/dev/null 2>&1 || true
fi
cat <<MSG
installed:
  $bin/hikarid, $bin/hikari, $bin/hikari-ui
  $bin/hikari-app, $bin/hikari-update
  $data/hikari/amethyst_filegraph.abi3.so (the mods catalog extension)
  $data/applications/hikari.desktop (Hikari appears in your app launcher)

start it from the launcher, or: $bin/hikari-app
MSG
