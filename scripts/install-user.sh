#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
bin="${HIKARI_USER_BIN:-$HOME/.local/bin}"
data="${XDG_DATA_HOME:-$HOME/.local/share}"

# Escape sed replacement metacharacters: an unescaped & in the install path
# would splice in the whole matched line and a | would break the s|||
# delimiters (which aborts under set -e right after the binaries were copied).
desktop_key() {
	local key="$1" value="$2" file="$3" esc
	esc="${value//\\/\\\\}"
	esc="${esc//&/\\&}"
	esc="${esc//|/\\|}"
	sed -i "s|^$key=.*|$key=$esc|" "$file"
}

mkdir -p "$bin"
(cd "$here" && go build -trimpath -o "$bin/hikarid" ./cmd/hikarid)
(cd "$here" && go build -trimpath -o "$bin/hikari" ./cmd/hikari)
"$here/scripts/build-backend.sh" "$data/hikari/amethyst_filegraph.abi3.so"
"$here/scripts/build-window.sh" "$bin/hikari-ui"
install -m755 "$here/scripts/hikari-app" "$bin/hikari-app"
install -m755 "$here/scripts/hikari-update" "$bin/hikari-update"
install -Dm644 "$here/packaging/hikari.svg" "$data/icons/hicolor/scalable/apps/hikari.svg"
install -Dm644 "$here/packaging/hikari.desktop" "$data/applications/hikari.desktop"
desktop_key Exec "$bin/hikari-app" "$data/applications/hikari.desktop"
desktop_key TryExec "$bin/hikari-app" "$data/applications/hikari.desktop"
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
