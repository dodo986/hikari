#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
bin="${HIKARI_USER_BIN:-$HOME/.local/bin}"
data="${XDG_DATA_HOME:-$HOME/.local/share}"
config="${XDG_CONFIG_HOME:-$HOME/.config}"
mkdir -p "$bin" "$config/quickshell"
for f in hikarid hikari hikari-app hikari-update; do
	install -m755 "$here/$f" "$bin/$f"
done
install -Dm755 "$here/amethyst_filegraph.abi3.so" "$data/hikari/amethyst_filegraph.abi3.so"
rm -rf "$config/quickshell/hikari"
cp -a "$here/quickshell" "$config/quickshell/hikari"
install -Dm644 "$here/hikari.svg" "$data/icons/hicolor/scalable/apps/hikari.svg"
install -Dm644 "$here/hikari.desktop" "$data/applications/hikari.desktop"
sed -i "s|^Exec=.*|Exec=$bin/hikari-app|" "$data/applications/hikari.desktop"
sed -i "s|^TryExec=.*|TryExec=$bin/hikari-app|" "$data/applications/hikari.desktop"
if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database "$data/applications" >/dev/null 2>&1 || true
fi
cat <<MSG
installed $( "$bin/hikarid" --version )
  $bin/hikarid, $bin/hikari, $bin/hikari-app, $bin/hikari-update
  $data/hikari/amethyst_filegraph.abi3.so (the mods catalog extension)
  $config/quickshell/hikari
  $data/applications/hikari.desktop

start it from your app launcher, or: $bin/hikari-app
MSG
